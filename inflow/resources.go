package inflow

import (
	"context"
	"fmt"
	"net/url"
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
	all := make([]InflowResource, 0, len(list))
	for _, el := range list {
		all = append(all, InflowResource{Token: makeTokenWithHs256(el.RegisterPortal.JwtSecret), Name: el.Name, Url: el.Url, Tags: el.Tags})
	}
	// Validate liveness once, here at load: only resources this inflow-fusion can
	// actually reach become dispatch candidates. A resource that was reinstalled,
	// moved host, or is unreachable from here is dropped, so GetResourceCandid
	// never hands one out. When none survive, roundrobin.New returns nil and
	// GetResourceCandid reports no resource — failing closed on purpose.
	live := filterLiveResources(all)
	return rebuildCandidates(live)
}

// rebuildCandidates swaps the whole dispatch pool to live under the lock: it
// stores the mirror copy, rebuilds the round-robin, and re-derives the pin from
// the tags of the surviving resources. Callers pass only resources that have
// already passed the liveness probe.
func rebuildCandidates(live []InflowResource) (*roundrobin.RoundRobin[InflowResource], error) {
	resourceMu.Lock()
	defer resourceMu.Unlock()

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

// AddResource manually adds one resource to the dispatch pool, alongside those
// loaded from infra. It is the "add a resource by hand" path of the settings
// dialog: probe it for liveness exactly as a loaded resource, and on success fold
// it into the round-robin. If it carries PinResourceTag it becomes the pinned
// resource, so every subsequent dispatch uses only it. A resource already in the
// pool at the same Url is replaced rather than duplicated. Returns an error if the
// resource fails the liveness probe.
func AddResource(res InflowResource) error {
	if !probeResource(res) {
		return fmt.Errorf("inflow resource %q (%s) failed liveness probe — not added", res.Name, res.Url)
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

// normalizeResourceUrl brings a raw resource URL (as infra stored it) to the
// address the HTTP client dials: a default REST port when none is given and an
// http scheme when none is given. Exec and the liveness probe share it so a
// resource is probed at exactly the address a dispatch would use.
func normalizeResourceUrl(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Port() == "" {
		raw = fmt.Sprintf("%s:%s", raw, models.INFLOW_REST_PORT)
	}
	if u.Scheme == "" {
		raw = fmt.Sprintf("http://%s", raw)
	}
	return raw, nil
}

// filterLiveResources probes every resource from *this* inflow-fusion, in
// parallel, and returns only those that answer. Excluded ones are logged so an
// operator can see which resource dropped out and why.
func filterLiveResources(resources []InflowResource) []InflowResource {
	if len(resources) == 0 {
		return resources
	}
	alive := make([]bool, len(resources))
	var wg sync.WaitGroup
	for i := range resources {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			alive[i] = probeResource(resources[i])
		}(i)
	}
	wg.Wait()

	live := make([]InflowResource, 0, len(resources))
	for i, res := range resources {
		if alive[i] {
			live = append(live, res)
			continue
		}
		if b := GetInflowBackend(); b != nil {
			b.GetLogger().Warn(fmt.Sprintf("inflow resource %q (%s) failed liveness probe — excluded from dispatch", res.Name, res.Url))
		}
	}
	return live
}

// probeResource reports whether a resource is reachable and healthy from here.
// It calls the resource's authenticated process-list endpoint, so a 200 proves
// three things at once: the address resolves, the fractal instance is up, and
// the token is accepted. A transport error or any non-200 means the resource
// must not be handed out. The token fallback matches Exec: a portal with no
// secret authenticates with the infra bearer.
func probeResource(res InflowResource) bool {
	addr, err := normalizeResourceUrl(res.Url)
	if err != nil {
		return false
	}
	token := "Bearer " + res.Token
	if res.Token == "" {
		b := GetInflowBackend()
		if b == nil {
			return false
		}
		token = b.GetBearerToken()
	}
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	resp, err := etc.SendHttpGetRaw(ctx, map[string]string{"Authorization": token}, addr+"/engine/ps", probeTimeout)
	if err != nil {
		return false
	}
	return resp.Status() == 200
}
