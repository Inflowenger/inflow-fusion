package compilers

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Inflowenger/inflow-fusion/models"
)

// buildFlow turns a list of "source>target" pairs into a VueFlow graph, naming
// every edge the way a frontend would.
func buildFlow(edges ...string) VueFlow {
	flow := VueFlow{}
	seen := make(map[string]bool)
	addNode := func(id string) {
		if seen[id] {
			return
		}
		seen[id] = true
		flow.Nodes = append(flow.Nodes, VueFlowNode{ID: id, Type: "code"})
	}
	for _, e := range edges {
		source, target, found := strings.Cut(e, ">")
		if !found {
			panic("edge must be written as source>target: " + e)
		}
		addNode(source)
		addNode(target)
		flow.Edges = append(flow.Edges, Edges{
			ID:           fmt.Sprintf("e-%s-%s", source, target),
			Source:       source,
			Target:       target,
			SourceHandle: "output",
			TargetHandle: "input",
		})
	}
	return flow
}

// dependsFromInbound is the hook pattern a frontend drives: a node's blocking
// input port lists every node wired into it.
func dependsFromInbound(joins map[string][]string) func(VueFlowNode) (*models.Node, error) {
	return func(fnode VueFlowNode) (*models.Node, error) {
		node := &models.Node{ID: fnode.ID, Type: models.CodeNodeType}
		if depends, ok := joins[fnode.ID]; ok {
			node.Depends = depends
		}
		return node, nil
	}
}

// start -> A ->> B,C ; B,C -> D ; D -> E ; E -> D, with D waiting on [B,C,E].
// D can never start: E only exists downstream of D.
func TestDependsOnDownstreamNodeDeadlocks(t *testing.T) {
	flow := buildFlow("start>A", "A>B", "A>C", "B>D", "C>D", "D>E", "E>D")
	compiler := NewVueFlowCompiler(WithDependsGuard())
	compiler.SetHookFunc(dependsFromInbound(map[string][]string{
		"D": {"B", "C", "E"},
	}))

	_, cerror := compiler.Compile("start", flow)
	if _, ok := cerror["D"]; !ok {
		t.Fatalf("expected Compile to report an error for node D, got %v", cerror)
	}

	report := compiler.Report()
	if !report.HasErrors() {
		t.Fatal("expected the guard to reject the flow")
	}
	errs := report.Errors()
	if len(errs) != 1 {
		t.Fatalf("expected exactly one error, got %d: %v", len(errs), errs)
	}
	issue := errs[0]
	if issue.Code != CodeDependDeadlock {
		t.Errorf("code = %q, want %q", issue.Code, CodeDependDeadlock)
	}
	if issue.NodeId != "D" || issue.DependsOn != "E" {
		t.Errorf("issue = %s -> %s, want D -> E", issue.NodeId, issue.DependsOn)
	}
	if issue.EdgeId != "e-E-D" {
		t.Errorf("edgeId = %q, want %q", issue.EdgeId, "e-E-D")
	}
	if got := issue.SuggestedTargets; len(got) != 2 || got[0] != "B" || got[1] != "C" {
		t.Errorf("suggestedTargets = %v, want [B C]", got)
	}
	t.Log(report.Err())
}

// The same loop, re-entered through an upstream dependency of the join: B stays
// reachable from start without D, so the topology is valid.
func TestBackEdgeToUpstreamDependencyIsValid(t *testing.T) {
	flow := buildFlow("start>A", "A>B", "A>C", "B>D", "C>D", "D>E", "E>B")
	compiler := NewVueFlowCompiler(WithDependsGuard())
	compiler.SetHookFunc(dependsFromInbound(map[string][]string{
		"D": {"B", "C"},
	}))

	_, cerror := compiler.Compile("start", flow)
	if len(cerror) != 0 {
		t.Fatalf("expected no compile errors, got %v", cerror)
	}
	if report := compiler.Report(); report.HasErrors() {
		t.Fatalf("expected a valid flow, got %v", report.Err())
	}
}

// A back edge may target the join itself as long as the join does not wait for
// the node it comes from: only the depends entry is invalid, not the loop.
func TestBackEdgeToJoinWithoutDependIsValid(t *testing.T) {
	flow := buildFlow("start>A", "A>B", "A>C", "B>D", "C>D", "D>E", "E>D")
	compiler := NewVueFlowCompiler(WithDependsGuard())
	compiler.SetHookFunc(dependsFromInbound(map[string][]string{
		"D": {"B", "C"},
	}))

	if _, cerror := compiler.Compile("start", flow); len(cerror) != 0 {
		t.Fatalf("expected no compile errors, got %v", cerror)
	}
}

func TestDependsGuardEdgeCases(t *testing.T) {
	cases := []struct {
		name   string
		flow   VueFlow
		joins  map[string][]string
		code   ValidationCode
		nodeId string
	}{
		{
			name:   "self depend",
			flow:   buildFlow("start>A"),
			joins:  map[string][]string{"A": {"A"}},
			code:   CodeDependSelf,
			nodeId: "A",
		},
		{
			name:   "unknown node",
			flow:   buildFlow("start>A"),
			joins:  map[string][]string{"A": {"ghost"}},
			code:   CodeDependUnknownNode,
			nodeId: "A",
		},
		{
			name:   "start node waits for a successor",
			flow:   buildFlow("start>A"),
			joins:  map[string][]string{"start": {"A"}},
			code:   CodeDependDeadlock,
			nodeId: "start",
		},
		{
			name:   "join waits for a transitive successor",
			flow:   buildFlow("start>A", "A>B", "B>C", "C>D", "D>A"),
			joins:  map[string][]string{"A": {"start", "C"}},
			code:   CodeDependDeadlock,
			nodeId: "A",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			compiler := NewVueFlowCompiler(WithDependsGuard())
			compiler.SetHookFunc(dependsFromInbound(tc.joins))
			compiler.Compile("start", tc.flow)

			errs := compiler.Report().Errors()
			if len(errs) == 0 {
				t.Fatal("expected the guard to reject the flow")
			}
			if errs[0].Code != tc.code || errs[0].NodeId != tc.nodeId {
				t.Fatalf("got %s on %q, want %s on %q",
					errs[0].Code, errs[0].NodeId, tc.code, tc.nodeId)
			}
			t.Log(errs[0].Error())
		})
	}
}

// A depends entry with no edge into the node is satisfiable, so it is a warning
// the app can surface without refusing the flow.
func TestDependWithoutEdgeIsWarning(t *testing.T) {
	flow := buildFlow("start>A", "A>B", "A>C", "C>D")
	compiler := NewVueFlowCompiler(WithDependsGuard())
	compiler.SetHookFunc(dependsFromInbound(map[string][]string{
		"D": {"B", "C"},
	}))

	if _, cerror := compiler.Compile("start", flow); len(cerror) != 0 {
		t.Fatalf("expected no compile errors, got %v", cerror)
	}
	report := compiler.Report()
	warnings := report.Warnings()
	if len(warnings) != 1 || warnings[0].Code != CodeDependWithoutEdge || warnings[0].DependsOn != "B" {
		t.Fatalf("expected one depend_without_edge warning on B, got %v", warnings)
	}
}

func TestValidateDependsWithoutCompiler(t *testing.T) {
	nodes := map[string]*models.Node{
		"start": {ID: "start", Next: []models.Next{{NodeId: "A"}}},
		"A":     {ID: "A", Next: []models.Next{{NodeId: "B"}}},
		"B":     {ID: "B", Depends: []string{"A"}},
	}
	if report := ValidateDepends("start", nodes); report.HasErrors() {
		t.Fatalf("expected a valid flow, got %v", report.Err())
	}
	if report := ValidateDepends("missing", nodes); !report.HasErrors() {
		t.Fatal("expected a missing start node to be reported")
	}
}

// The guard is opt-in: a plain compiler lowers the same deadlocking graph
// without a word, because a caller compiling a graph to RUN it may be entering
// it somewhere the guard's start-node walk says nothing about (see
// WithDependsGuard). The findings stay available through ValidateDepends.
func TestDependsGuardIsOffByDefault(t *testing.T) {
	flow := buildFlow("start>A", "A>B", "A>C", "B>D", "C>D", "D>E", "E>D")
	joins := map[string][]string{"D": {"B", "C", "E"}}

	compiler := NewVueFlowCompiler(WithEachNodeFunc(dependsFromInbound(joins)))
	nodes, cerror := compiler.Compile("start", flow)
	if len(cerror) != 0 {
		t.Fatalf("expected no errors without the guard, got %v", cerror)
	}
	if report := compiler.Report(); len(report.Issues) != 0 {
		t.Fatalf("expected no report without the guard, got %v", report.Issues)
	}
	// The whole graph is still lowered, loop included.
	for _, id := range []string{"start", "A", "B", "C", "D", "E"} {
		if nodes[id] == nil {
			t.Fatalf("node %q was not lowered", id)
		}
	}
	// And the same verdict is one call away for a caller that wants it.
	if !ValidateDepends("start", nodes).HasErrors() {
		t.Fatal("ValidateDepends over the same map should still reject it")
	}
}
