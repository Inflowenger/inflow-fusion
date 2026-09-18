package inflow

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Inflowenger/inflow-fusion/etc"
	"github.com/Inflowenger/inflow-fusion/models"
	roundrobin "github.com/thegeekyasian/round-robin-go"
)

type InflowResource struct {
	Name string
	Url  string
	Token string
	Tags []string
}

// PinResourceTag, when carried by a live resource, pins every process dispatch to
// that single resource and skips the round-robin. It is the "use just this one"
// switch surfaced in the flowmorphic-wapp settings dialog: an operator tags a
// resource with it (or adds one manually via AddResource) to force all traffic
// onto one engine instead of spreading it across the pool. If several live
// resources carry the tag, the last one seen wins.
const PinResourceTag = "pinned-resource"

// resourceMu guards the dispatch state below. Reads (GetResourceCandid,
// GetResourceCandidList) and writes (SetResourceCandid, AddResource, pin control)
// can race — a ReloadResources on one goroutine while another dispatches — so all
// of them take the lock.
var resourceMu sync.RWMutex

var resourceCandidate *roundrobin.RoundRobin[InflowResource]

// liveResources mirrors the resources currently in the round-robin pool. The
// round-robin type does not expose its members, so we keep this copy to answer
// GetResourceCandidList and to rebuild the pool when AddResource extends it.
var liveResources []InflowResource

// pinnedResource, when non-nil, is the single resource every dispatch uses; the
// round-robin is bypassed. It is set to whichever live resource carries
// PinResourceTag, or explicitly via PinResource.
var pinnedResource *InflowResource

// probeTimeout bounds one liveness probe. Resources are validated only once, at
// load, so this delays a resource reload at worst — never a process dispatch.
const probeTimeout = 3 * time.Second

func SetResourceCandid(list []models.RegisteredInflow) (*roundrobin.RoundRobin[InflowResource], error) {
	// Validate liveness once, here at load: only resources this inflow-fusion can
	// actually reach become dispatch candidates. A resource that was reinstalled,
	// moved host, or is unreachable from here is dropped, so GetResourceCandid
	// never hands one out. When none survive, roundrobin.New returns nil and
	// GetResourceCandid reports no resource — failing closed on purpose.
	live := filterLiveResources(toInflowResources(list), true)
	return rebuildCandidates(live)
}

// toInflowResources converts infra's registered-engine rows into pool entries,
// signing each one's dispatch token with the secret of the portal it enrolled
// through (empty secret ⇒ empty token ⇒ the infra bearer at dispatch time).
func toInflowResources(list []models.RegisteredInflow) []InflowResource {
	all := make([]InflowResource, 0, len(list))
	for _, el := range list {
		all = append(all, InflowResource{Token: makeTokenWithHs256(el.RegisterPortal.JwtSecret), Name: el.Name, Url: el.Url, Tags: el.Tags})
	}
	return all
}

// HasLiveResources reports whether the dispatch pool holds at least one resource
// — i.e. whether GetResourceCandid can hand anything out.
func HasLiveResources() bool {
	resourceMu.RLock()
	defer resourceMu.RUnlock()
	return len(liveResources) > 0
}

// adoptIfPoolEmpty installs live as the dispatch pool only if the pool is still
// empty, and reports whether it did. It is what the startup retry uses: by the
// time a retry has fetched and probed infra's list, an operator may already have
// added a resource by hand, and a background reload must not throw that away
// (the explicit /resource/reload is allowed to — this path is not). The check
// and the swap happen under one lock so nothing can slip in between.
func adoptIfPoolEmpty(live []InflowResource) bool {
	resourceMu.Lock()
	defer resourceMu.Unlock()
	if len(liveResources) > 0 {
		return false
	}
	rebuildCandidatesLocked(live)
	return true
}

// rebuildCandidates swaps the whole dispatch pool to live under the lock: it
// stores the mirror copy, rebuilds the round-robin, and re-derives the pin from
// the tags of the surviving resources. Callers pass only resources that have
// already passed the liveness probe.
func rebuildCandidates(live []InflowResource) (*roundrobin.RoundRobin[InflowResource], error) {
	resourceMu.Lock()
	defer resourceMu.Unlock()
	return rebuildCandidatesLocked(live)
}

// rebuildCandidatesLocked is rebuildCandidates for callers that already hold
// resourceMu for writing.
func rebuildCandidatesLocked(live []InflowResource) (*roundrobin.RoundRobin[InflowResource], error) {
	liveResources = live
	resourcesList := make([]*InflowResource, 0, len(live))
	for i := range liveResources {
		resourcesList = append(resourcesList, &liveResources[i])
	}
	var err error
	resourceCandidate, err = roundrobin.New(resourcesList...)

	// Re-derive the pin from the fresh pool: a reload must never keep pinning a
	// resource that dropped out, and a resource newly tagged upstream must take
	// effect. Last tagged resource wins.
	pinnedResource = nil
	for i := range liveResources {
		if hasTag(liveResources[i].Tags, PinResourceTag) {
			pinnedResource = &liveResources[i]
		}
	}
	return resourceCandidate, err
}

func GetResourceCandid() *InflowResource {
	resourceMu.RLock()
	defer resourceMu.RUnlock()
	if pinnedResource != nil {
		return pinnedResource
	}
	if resourceCandidate == nil {
		return nil
	}
	return resourceCandidate.Next()
}

// GetResourceCandidList returns a snapshot of the live dispatch pool — the
// resources GetResourceCandid picks from — for the flowmorphic-wapp settings
// dialog to render. The pinned resource (if any) is flagged by its PinResourceTag
// among the returned Tags. The slice is a copy, safe for the caller to keep.
func GetResourceCandidList() []InflowResource {
	resourceMu.RLock()
	defer resourceMu.RUnlock()
	out := make([]InflowResource, len(liveResources))
	copy(out, liveResources)
	return out
}

// GetPinnedResource returns a copy of the resource every dispatch is currently
// pinned to, or nil when dispatch is round-robin across the whole pool. It lets a
// caller (the settings dialog) show which single resource is in use — whether the
// pin came from PinResourceTag or from an explicit PinResource call.
func GetPinnedResource() *InflowResource {
	resourceMu.RLock()
	defer resourceMu.RUnlock()
	if pinnedResource == nil {
		return nil
	}
	cp := *pinnedResource
	return &cp
}

// AddResource manually adds one resource to the dispatch pool, alongside those
// loaded from infra. It is the "add a resource by hand" path of the settings
// dialog: probe it for liveness exactly as a loaded resource, and on success fold
// it into the round-robin. If it carries PinResourceTag it becomes the pinned
// resource, so every subsequent dispatch uses only it. A resource already in the
// pool at the same Url is replaced rather than duplicated. Returns an error if the
// resource fails the liveness probe, saying which part of it failed.
func AddResource(res InflowResource) error {
	// A hand-added resource carries no credential of its own, but infra usually
	// already knows one: an engine enrolled through a portal is dispatched to with
	// the token that portal issued. Look that up by url before probing, so adding
	// "http://localhost:9001" from the settings dialog authenticates exactly the
	// way the same engine does when it is loaded from infra. Without it the probe
	// falls back to the infra bearer — a different key — and a portal-registered
	// engine answers 401: the resource is live and correctly addressed, yet never
	// joins the pool. An operator cannot paste a token they were never shown.
	if res.Token == "" {
		if b := GetInflowBackend(); b != nil {
			res.Token = b.GetResourceToken(res.Url)
		}
	}
	if err := probeResource(res); err != nil {
		return fmt.Errorf("inflow resource %q (%s) %w — not added", res.Name, res.Url, err)
	}
	resourceMu.RLock()
	next := make([]InflowResource, 0, len(liveResources)+1)
	for _, r := range liveResources {
		if r.Url != res.Url {
			next = append(next, r)
		}
	}
	resourceMu.RUnlock()
	next = append(next, res)
	_, err := rebuildCandidates(next)
	return err
}

// PinResource forces all dispatch onto the single pooled resource named or
// addressed by nameOrUrl, skipping the round-robin, and reports whether it was
// found. Use it for the dialog's "use just this one" toggle when the resource is
// already in the pool but not tagged. UnpinResource reverses it.
func PinResource(nameOrUrl string) bool {
	resourceMu.Lock()
	defer resourceMu.Unlock()
	for i := range liveResources {
		if liveResources[i].Name == nameOrUrl || liveResources[i].Url == nameOrUrl {
			pinnedResource = &liveResources[i]
			return true
		}
	}
	return false
}

// UnpinResource clears any pin so dispatch returns to round-robin across the whole
// pool. A following ReloadResources re-derives the pin from tags, so this is a
// runtime override rather than a permanent change.
func UnpinResource() {
	resourceMu.Lock()
	defer resourceMu.Unlock()
	pinnedResource = nil
}

// hasTag reports whether tags contains want.
func hasTag(tags []string, want string) bool {
	for _, t := range tags {
		if t == want {
			return true
		}
	}
	return false
}

// normalizeResourceUrl brings a raw resource URL (as infra stored it, or as an
// operator typed it into the settings dialog) to the address the HTTP client
// dials: an http scheme when none is given and the default REST port when none
// is given. Exec and the liveness probe share it so a resource is probed at
// exactly the address a dispatch would use.
//
// The scheme is fixed up *before* parsing, not after. url.Parse reads a
// scheme-less "localhost:9001" as the scheme "localhost" with opaque "9001", so
// u.Port() comes back empty and a default port gets appended to a string that
// already has one ("localhost:9001:9001"); a bare "127.0.0.1:9001" fails to
// parse outright ("first path segment in URL cannot contain colon"). Either way
// the resource is live but unreachable at the address we built. Prefixing the
// scheme first makes both parse as a real host:port.
func normalizeResourceUrl(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("resource url is empty")
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Host == "" {
		return "", fmt.Errorf("resource url %q has no host", raw)
	}
	if u.Port() == "" {
		// Hostname() drops any IPv6 brackets, JoinHostPort puts them back, so a
		// "[::1]" host does not end up double-bracketed.
		u.Host = net.JoinHostPort(u.Hostname(), models.INFLOW_REST_PORT)
	}
	return u.String(), nil
}

// filterLiveResources probes every resource from *this* inflow-fusion, in
// parallel, and returns only those that answer. With logExcluded set, each one
// dropped is logged so an operator can see which resource fell out and why; the
// startup retry passes false, since repeating the same warnings every backoff
// tick for the same stale entries would only bury the line that matters.
func filterLiveResources(resources []InflowResource, logExcluded bool) []InflowResource {
	if len(resources) == 0 {
		return resources
	}
	probeErr := make([]error, len(resources))
	var wg sync.WaitGroup
	for i := range resources {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			probeErr[i] = probeResource(resources[i])
		}(i)
	}
	wg.Wait()

	live := make([]InflowResource, 0, len(resources))
	for i, res := range resources {
		if probeErr[i] == nil {
			live = append(live, res)
			continue
		}
		if b := GetInflowBackend(); b != nil && logExcluded {
			b.GetLogger().Warn(fmt.Sprintf("inflow resource %q (%s) %v — excluded from dispatch", res.Name, res.Url, probeErr[i]))
		}
	}
	return live
}

// probeResource reports whether a resource is reachable and healthy from here,
// returning nil when it is and a describing error when it is not. It calls the
// resource's authenticated process-list endpoint, so a 200 proves three things
// at once: the address resolves, the fractal instance is up, and the token is
// accepted. A transport error or any non-200 means the resource must not be
// handed out. The token fallback matches Exec: a portal with no secret
// authenticates with the infra bearer.
//
// The error says *which* of those three failed. They need different fixes — a
// wrong address, a stopped engine and a rejected credential are not the same
// problem — and collapsing them into one "failed liveness probe" line leaves an
// operator with a live engine, a correct URL and no idea why it will not join.
func probeResource(res InflowResource) error {
	addr, err := normalizeResourceUrl(res.Url)
	if err != nil {
		return fmt.Errorf("has an unusable url: %w", err)
	}
	token := "Bearer " + res.Token
	if res.Token == "" {
		b := GetInflowBackend()
		if b == nil {
			return errors.New("cannot be probed: the inflow backend is not connected yet")
		}
		token = b.GetBearerToken()
	}
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	resp, err := etc.SendHttpGetRaw(ctx, map[string]string{"Authorization": token}, addr+"/engine/ps", probeTimeout)
	if err != nil {
		return fmt.Errorf("is unreachable at %s: %w", addr, err)
	}
	switch code := resp.Status(); {
	case code == 200:
		return nil
	case code == 400 || code == 401 || code == 403:
		// The engine answered, so it is up and the address is right — it refused
		// the credential. A portal-registered engine signs with the secret its
		// portal issued, which is not the infra bearer probeResource falls back
		// to, so this is what a hand-added resource hits when no token is given.
		return fmt.Errorf("rejected the credential at %s (HTTP %d): it is up, but the token sent is not the one it accepts — supply the token its portal issued", addr, code)
	default:
		return fmt.Errorf("is unhealthy at %s: answered HTTP %d", addr, code)
	}
}
