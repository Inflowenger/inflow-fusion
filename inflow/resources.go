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

var resourceCandidate *roundrobin.RoundRobin[InflowResource]

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
	resourcesList := make([]*InflowResource, 0, len(live))
	for i := range live {
		resourcesList = append(resourcesList, &live[i])
	}
	var err error
	resourceCandidate, err = roundrobin.New(resourcesList...)
	return resourceCandidate, err

}

func GetResourceCandid() *InflowResource {
	if resourceCandidate == nil {
		return nil
	}
	return resourceCandidate.Next()
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
