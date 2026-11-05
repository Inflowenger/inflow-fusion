package inflow

import (
	"testing"

	"github.com/Inflowenger/inflow-fusion/models"
	"github.com/bytedance/sonic"
)

func registered(name string, weight models.ResourceWeight) models.RegisteredInflow {
	return models.RegisteredInflow{
		RegisterRequestBody: models.RegisterRequestBody{Name: name, Url: "http://" + name + ":8033"},
		Weight:              weight,
	}
}

func TestNegativeWeightsNeverBecomeDispatchCandidates(t *testing.T) {
	// toInflowResources is the only way a row from infra turns into a pool
	// entry, so an out-of-service engine has to be stopped exactly here.
	out := toInflowResources([]models.RegisteredInflow{
		registered("one", models.ResourceWeightDefault),
		registered("two", models.ResourceWeightInactive),
		registered("three", 4),
		// Not the -1 constant: infra reads the sign, so anything below zero is
		// out of service and must be dropped just the same.
		registered("four", -7),
	})
	if len(out) != 2 {
		t.Fatalf("expected the two negative-weight engines to be dropped, got %d candidates", len(out))
	}
	for _, r := range out {
		if r.Name == "two" || r.Name == "four" {
			t.Fatalf("engine %q was out of service but entered the dispatch pool", r.Name)
		}
	}
}

func TestPoolEntryKeepsTheWeightForBalancing(t *testing.T) {
	// The magnitude is not honoured by the round-robin yet, but it has to reach
	// the pool entry, or weighted selection would need another read from infra.
	out := toInflowResources([]models.RegisteredInflow{registered("heavy", 3)})
	if len(out) != 1 {
		t.Fatalf("expected one candidate, got %d", len(out))
	}
	if out[0].Weight != 3 {
		t.Fatalf("pool entry carries weight %d, want 3", out[0].Weight)
	}
	if got := out[0].Weight.Share(); got != 4 {
		t.Fatalf("weight 3 has share %d, want 4 (n+1, so default is an even split)", got)
	}
}

func TestAWholeLockedListLeavesTheDispatchPoolEmpty(t *testing.T) {
	// What infra serves when the trial credit is finished: every registration is
	// still there, all of them at a negative weight. The pool must end up empty
	// so GetResourceCandid hands nothing out and dispatch fails closed.
	locked := []models.RegisteredInflow{
		registered("one", models.ResourceWeightInactive),
		registered("two", models.ResourceWeightInactive),
	}
	if out := toInflowResources(locked); len(out) != 0 {
		t.Fatalf("a fully locked list produced %d candidates, want 0", len(out))
	}
}

func TestAResourceWithoutAWeightFieldIsActive(t *testing.T) {
	// An older infra does not send `weight` at all. The zero value has to mean
	// active, or upgrading fusion alone would park an otherwise healthy cluster.
	raw := `{"data":{"list":[{"id":"r1","name":"one","url":"http://one:8033"}]},"error":null}`
	list := models.InflowResourcesList{}
	if err := sonic.Unmarshal([]byte(raw), &list); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(list.Data.List) != 1 {
		t.Fatalf("expected one row, got %d", len(list.Data.List))
	}
	if !list.Data.List[0].Weight.Active() {
		t.Fatalf("a missing weight decoded as %q, want active", list.Data.List[0].Weight)
	}
	if len(toInflowResources(list.Data.List)) != 1 {
		t.Fatal("a resource from an infra that sends no weight was dropped from the pool")
	}
}

func TestResourceWeightSemantics(t *testing.T) {
	cases := []struct {
		weight models.ResourceWeight
		active bool
		share  int
		name   string
	}{
		{-128, false, 0, "inactive"},
		{models.ResourceWeightInactive, false, 0, "inactive"},
		{models.ResourceWeightDefault, true, 1, "active"},
		{1, true, 2, "active weight 1"},
		{127, true, 128, "active weight 127"},
	}
	for _, c := range cases {
		if got := c.weight.Active(); got != c.active {
			t.Errorf("weight %d: active %v, want %v", c.weight, got, c.active)
		}
		if got := c.weight.Share(); got != c.share {
			t.Errorf("weight %d: share %d, want %d", c.weight, got, c.share)
		}
		if got := c.weight.String(); got != c.name {
			t.Errorf("weight %d: printed %q, want %q", c.weight, got, c.name)
		}
	}
}
