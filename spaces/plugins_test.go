package InfraSpaces

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// A pluginId that is not a single subject token must not reach a grant: it is
// what the subject-as-identity scheme rests on. "*" is the dangerous one — it
// is a legal NATS permission subject, so it mints a credential that may publish
// under any plugin's id.
func TestStrictPermissionRejectsSubjectInjection(t *testing.T) {
	for _, id := range []string{"*", ">", "a.b", "inflow.plugin.*", "", "a b", "a*"} {
		if _, err := PluginCredentialStrictPermission("u", id, "ACC"); !errors.Is(err, ErrInvalidPluginId) {
			t.Errorf("pluginId %q: err = %v, want ErrInvalidPluginId", id, err)
		}
	}
}

// Not every real pluginId is a uuid: the builtin llm/mcp/cast nodes carry
// hard-coded seed ids, and imported flowfiles carry their own.
func TestStrictPermissionAcceptsRealPluginIds(t *testing.T) {
	for _, id := range []string{
		"aaaa-bbbb-cccc-llm0",
		"venapce-1234",
		"id-a-b-c",
		"4d7f1c2e-9b3a-4f58-8e21-0c6d5a9b7e14",
	} {
		if _, err := PluginCredentialStrictPermission("u", id, "ACC"); err != nil {
			t.Errorf("pluginId %q: unexpected error %v", id, err)
		}
	}
}

// The plugin publishes upstream on its own leaf and nowhere else in the signal
// channel. If a grant ever covers the whole channel again, a plugin can forge
// the runtime's ".proc" stop to its own replicas.
func TestStrictPermissionCannotPublishRuntimeSignals(t *testing.T) {
	perm, err := PluginCredentialStrictPermission("u", "plug-1", "ACC")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(perm.Pub.Allow, "inflow.plugin.plug-1.up.>") {
		t.Errorf("Pub.Allow = %v, want the upstream leaf", perm.Pub.Allow)
	}
	for _, s := range perm.Pub.Allow {
		if s == "inflow.plugin.plug-1.>" || s == "inflow.plugin.plug-1.proc" {
			t.Errorf("Pub.Allow contains the runtime signal channel: %q", s)
		}
	}
	// the plugin still receives the whole channel
	if !slices.Contains(perm.Sub.Allow, "inflow.plugin.plug-1.>") {
		t.Errorf("Sub.Allow = %v, want the signal channel", perm.Sub.Allow)
	}
}

// An empty Pub allow-list is "allow all" to NATS, so the collector's inability
// to publish has to be stated, not implied.
func TestCollectorCredentialIsSubscribeOnly(t *testing.T) {
	perm := CollectorCredentialPermission("collector", "ACC")
	if len(perm.Pub.Allow) != 0 || !slices.Contains(perm.Pub.Deny, ">") {
		t.Errorf("Pub = %+v, want no allow and a deny on >", perm.Pub)
	}
	if !slices.Contains(perm.Sub.Allow, "inflow.plugin.*.up.>") {
		t.Errorf("Sub.Allow = %v, want the upstream wildcard", perm.Sub.Allow)
	}
}

// What the collector subscribes must match what a plugin is allowed to publish,
// and the id must read back out of it.
func TestUpstreamSubjectRoundTrip(t *testing.T) {
	const id = "aaaa-bbbb-cccc-llm0"
	subject := PluginUpstreamSubject(id, "logs")
	if subject != "inflow.plugin.aaaa-bbbb-cccc-llm0.up.logs" {
		t.Fatalf("subject = %q", subject)
	}
	got, ok := PluginIdFromUpstreamSubject(subject)
	if !ok || got != id {
		t.Errorf("PluginIdFromUpstreamSubject(%q) = %q, %v; want %q, true", subject, got, ok, id)
	}
	// the grant pattern the credential carries must cover it
	grant := PluginUpstreamSubject(id, ">")
	if !strings.HasPrefix(subject, strings.TrimSuffix(grant, ">")) {
		t.Errorf("grant %q does not cover %q", grant, subject)
	}
}

// Anything that is not upstream traffic must not be attributed as if it were —
// above all the runtime's own signals, which share the channel.
func TestPluginIdFromUpstreamSubjectRejectsNonUpstream(t *testing.T) {
	for _, s := range []string{
		"inflow.plugin.plug-1.proc", // the runtime's stop signal
		"inflow.plugin.plug-1.up",   // no kind token
		"inflow.plugin.plug-1",      // bare id
		"inflow.cpu.plug-1.up.logs", // another channel
		"inflow.plugin.a.b.up.logs", // id is not one token
		"inflow.plugin.*.up.logs",   // a wildcard as an id
		"",
	} {
		if id, ok := PluginIdFromUpstreamSubject(s); ok {
			t.Errorf("PluginIdFromUpstreamSubject(%q) = %q, true; want false", s, id)
		}
	}
}
