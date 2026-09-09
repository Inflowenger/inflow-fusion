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
