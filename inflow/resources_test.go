package inflow

import (
	"testing"

	"github.com/Inflowenger/inflow-fusion/models"
)

// TestNormalizeResourceUrl covers the forms an operator types into the settings
// dialog and the forms infra stores. The scheme-less "host:port" cases are the
// regression: url.Parse reads "localhost" as a scheme and "9001" as opaque, so
// the old code appended a second port ("localhost:9001:9001") and never added
// the http prefix, leaving a live engine unreachable at the address we built.
func TestNormalizeResourceUrl(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{"scheme-less host and port", "localhost:9001", "http://localhost:9001"},
		{"scheme-less ip and port", "127.0.0.1:9001", "http://127.0.0.1:9001"},
		{"bare host takes default port", "localhost", "http://localhost:" + models.INFLOW_REST_PORT},
		{"full url is left alone", "http://mate-Predator:9001", "http://mate-Predator:9001"},
		{"https is preserved", "https://engine.example:8443", "https://engine.example:8443"},
		{"https without port takes default", "https://engine.example", "https://engine.example:" + models.INFLOW_REST_PORT},
		{"surrounding space is trimmed", "  localhost:9001  ", "http://localhost:9001"},
		{"ipv6 keeps single brackets", "[::1]:9001", "http://[::1]:9001"},
		{"ipv6 without port takes default", "[::1]", "http://[::1]:" + models.INFLOW_REST_PORT},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeResourceUrl(tc.in)
			if err != nil {
				t.Fatalf("normalizeResourceUrl(%q) unexpected error: %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("normalizeResourceUrl(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// A url with nothing to dial must fail rather than produce a plausible-looking
// address: probeResource turns the error into "has an unusable url", which is a
// different fix from an unreachable host.
func TestNormalizeResourceUrlRejectsEmpty(t *testing.T) {
	for _, in := range []string{"", "   "} {
		if got, err := normalizeResourceUrl(in); err == nil {
			t.Errorf("normalizeResourceUrl(%q) = %q, want an error", in, got)
		}
	}
}

// Normalizing twice must be a no-op — Exec and the liveness probe both call it,
// and a process row stores the already-normalized url.
func TestNormalizeResourceUrlIsIdempotent(t *testing.T) {
	for _, in := range []string{"localhost:9001", "localhost", "http://mate-Predator:9001", "[::1]"} {
		once, err := normalizeResourceUrl(in)
		if err != nil {
			t.Fatalf("normalizeResourceUrl(%q): %v", in, err)
		}
		twice, err := normalizeResourceUrl(once)
		if err != nil {
			t.Fatalf("normalizeResourceUrl(%q): %v", once, err)
		}
		if once != twice {
			t.Errorf("not idempotent for %q: %q then %q", in, once, twice)
		}
	}
}

// resetPool empties the dispatch pool so a test starts from the "nothing loaded
// yet" state the startup retry reasons about.
func resetPool(t *testing.T) {
	t.Helper()
	rebuildCandidates(nil)
	if HasLiveResources() {
		t.Fatal("pool not empty after reset")
	}
}

// The startup retry may only fill an EMPTY pool. If an operator added a resource
// by hand while a retry tick was still probing, that resource must survive: the
// retry's late result is dropped, not the operator's.
func TestAdoptIfPoolEmpty(t *testing.T) {
	resetPool(t)
	t.Cleanup(func() { rebuildCandidates(nil) })

	fromRetry := []InflowResource{{Name: "fractal-1", Url: "http://fractal-1:9001"}}
	if !adoptIfPoolEmpty(fromRetry) {
		t.Fatal("empty pool: adoptIfPoolEmpty should adopt and report true")
	}
	if got := GetResourceCandidList(); len(got) != 1 || got[0].Url != fromRetry[0].Url {
		t.Fatalf("pool after adopt = %+v, want %+v", got, fromRetry)
	}
	if GetResourceCandid() == nil {
		t.Fatal("GetResourceCandid should hand out the adopted resource")
	}

	byHand := []InflowResource{{Name: "manual", Url: "http://manual:9001", Tags: []string{PinResourceTag}}}
	rebuildCandidates(byHand) // stands in for AddResource, minus the network probe
	if adoptIfPoolEmpty(fromRetry) {
		t.Fatal("non-empty pool: adoptIfPoolEmpty must not replace it")
	}
	if got := GetResourceCandidList(); len(got) != 1 || got[0].Url != byHand[0].Url {
		t.Fatalf("pool after refused adopt = %+v, want the hand-added %+v", got, byHand)
	}
	if p := GetPinnedResource(); p == nil || p.Url != byHand[0].Url {
		t.Fatalf("pin lost: got %+v, want %+v", p, byHand[0])
	}
}

// An empty adopt must leave the pool empty and GetResourceCandid failing closed,
// so a retry tick that found nothing reachable cannot install a nil round-robin
// that later dispatches to nothing.
func TestAdoptIfPoolEmptyWithNothingKeepsFailingClosed(t *testing.T) {
	resetPool(t)
	if !adoptIfPoolEmpty(nil) {
		t.Fatal("adopting nil into an empty pool should still report true (pool was empty)")
	}
	if HasLiveResources() || GetResourceCandid() != nil {
		t.Fatal("pool should stay empty and GetResourceCandid should return nil")
	}
}
