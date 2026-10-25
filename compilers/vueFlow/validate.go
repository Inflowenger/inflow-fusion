package compilers

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Inflowenger/inflow-fusion/models"
)

type ValidationSeverity string

const (
	SeverityError   ValidationSeverity = "error"
	SeverityWarning ValidationSeverity = "warning"
)

type ValidationCode string

const (
	// CodeDependDeadlock is raised when a node waits for a node that can only ever
	// run after it. The dependency is unreachable from the start node unless the
	// waiting node runs first, so the first crossing of the flow never completes.
	CodeDependDeadlock ValidationCode = "depend_deadlock"
	// CodeDependUnreachable is raised when a dependency exists in the graph but is
	// not reachable from the start node at all, so it never gets queued.
	CodeDependUnreachable ValidationCode = "depend_unreachable"
	// CodeDependUnknownNode is raised when depends references an id that is not a
	// node of the compiled graph.
	CodeDependUnknownNode ValidationCode = "depend_unknown_node"
	// CodeDependSelf is raised when a node waits for itself.
	CodeDependSelf ValidationCode = "depend_self"
	// CodeDependWithoutEdge is raised when a node waits for a node that has no
	// transition into it. Satisfiable, but usually a stale depends entry.
	CodeDependWithoutEdge ValidationCode = "depend_without_edge"
	// CodeStartNodeMissing is raised when the start node is absent from the graph,
	// which makes every reachability answer meaningless.
	CodeStartNodeMissing ValidationCode = "start_node_missing"
)

// ValidationIssue is one guard finding, shaped so an app (or a frontend canvas)
// can point at the exact node and edge that has to change.
type ValidationIssue struct {
	Code      ValidationCode     `json:"code"`
	Severity  ValidationSeverity `json:"severity"`
	NodeId    string             `json:"nodeId"`           // the node that blocks (the join)
	DependsOn string             `json:"dependsOn"`        // the offending entry of NodeId's depends
	EdgeId    string             `json:"edgeId,omitempty"` // edge DependsOn -> NodeId, when one exists
	Message   string             `json:"message"`
	// Suggestion is a human readable fix, SuggestedTargets the node ids the
	// offending edge may legally target instead.
	Suggestion       string   `json:"suggestion,omitempty"`
	SuggestedTargets []string `json:"suggestedTargets,omitempty"`
}

func (i ValidationIssue) Error() string {
	if i.Suggestion == "" {
		return fmt.Sprintf("%s: %s", i.Code, i.Message)
	}
	return fmt.Sprintf("%s: %s. %s", i.Code, i.Message, i.Suggestion)
}

// ValidationReport is the full guard outcome for one compiled graph.
type ValidationReport struct {
	StartNodeId string            `json:"startNodeId"`
	Issues      []ValidationIssue `json:"issues"`
}

func (r ValidationReport) HasErrors() bool {
	return len(r.Errors()) > 0
}

func (r ValidationReport) Errors() (issues []ValidationIssue) {
	for _, i := range r.Issues {
		if i.Severity == SeverityError {
			issues = append(issues, i)
		}
	}
	return
}

func (r ValidationReport) Warnings() (issues []ValidationIssue) {
	for _, i := range r.Issues {
		if i.Severity == SeverityWarning {
			issues = append(issues, i)
		}
	}
	return
}

// Err collapses every error severity issue into a single error, nil when the
// graph is valid. Use it when a caller only wants to refuse the flow.
func (r ValidationReport) Err() error {
	errs := r.Errors()
	if len(errs) == 0 {
		return nil
	}
	msgs := make([]string, 0, len(errs))
	for _, e := range errs {
		msgs = append(msgs, e.Error())
	}
	return fmt.Errorf("invalid flow: %s", strings.Join(msgs, "; "))
}

// ErrorsByNode groups error severity issues per blocking node id, matching the
// map[nodeId]error channel Compile already returns.
func (r ValidationReport) ErrorsByNode() map[string]error {
	out := make(map[string]error)
	for _, i := range r.Errors() {
		if prev, ok := out[i.NodeId]; ok {
			out[i.NodeId] = fmt.Errorf("%s; %s", prev.Error(), i.Error())
			continue
		}
		out[i.NodeId] = fmt.Errorf("%s", i.Error())
	}
	return out
}

// ValidateDepends is the guard rule for node depends sets.
//
// A node with depends blocks until every id it lists has completed. That is only
// ever satisfiable if each dependency can complete *without* the waiting node
// running first - in graph terms, the waiting node must not dominate its own
// dependencies. The check is therefore: remove the waiting node from the graph
// and ask whether the dependency is still reachable from the start node.
//
// This deliberately accepts loops. A back edge E -> B, where B is an upstream
// dependency of the join D, keeps B reachable from start without D, so the
// topology stays valid. A back edge E -> D, where E only exists downstream of D,
// makes D wait for its own future and is rejected.
//
// It operates on a compiled node map, so it is reusable by any compiler, or on a
// flow built by hand with the nodes builders.
func ValidateDepends(startNodeId string, nodes map[string]*models.Node) ValidationReport {
	report := ValidationReport{StartNodeId: startNodeId}
	if len(nodes) == 0 {
		return report
	}
	if n, ok := nodes[startNodeId]; !ok || n == nil {
		report.Issues = append(report.Issues, ValidationIssue{
			Code:     CodeStartNodeMissing,
			Severity: SeverityError,
			NodeId:   startNodeId,
			Message: fmt.Sprintf("start node %q is not part of the graph, depends cannot be validated",
				startNodeId),
		})
		return report
	}

	reachable := reachableFrom(startNodeId, nodes, "")
	inbound := inboundIndex(nodes)

	for _, id := range sortedNodeIds(nodes) {
		node := nodes[id]
		if node == nil || len(node.Depends) == 0 || !reachable[id] {
			continue
		}
		// reachability with this node cut out of the graph, computed once per
		// node that actually has depends.
		var without map[string]bool
		cut := func() map[string]bool {
			if without == nil {
				without = reachableFrom(startNodeId, nodes, id)
			}
			return without
		}

		for _, dep := range node.Depends {
			switch {
			case dep == id:
				report.Issues = append(report.Issues, ValidationIssue{
					Code:      CodeDependSelf,
					Severity:  SeverityError,
					NodeId:    id,
					DependsOn: dep,
					Message:   fmt.Sprintf("node %q depends on itself, so it can never start", id),
					Suggestion: fmt.Sprintf("remove %q from the depends set of %q.",
						dep, id),
				})
			case nodes[dep] == nil:
				report.Issues = append(report.Issues, ValidationIssue{
					Code:      CodeDependUnknownNode,
					Severity:  SeverityError,
					NodeId:    id,
					DependsOn: dep,
					Message: fmt.Sprintf("node %q depends on %q, which is not a node of this graph",
						id, dep),
					Suggestion: fmt.Sprintf("remove %q from the depends set of %q, or add the missing node.",
						dep, id),
				})
			case !reachable[dep]:
				report.Issues = append(report.Issues, ValidationIssue{
					Code:      CodeDependUnreachable,
					Severity:  SeverityError,
					NodeId:    id,
					DependsOn: dep,
					EdgeId:    edgeIdBetween(nodes, dep, id),
					Message: fmt.Sprintf("node %q depends on %q, which is not reachable from start node %q, so it is never queued and %q blocks forever",
						id, dep, startNodeId, id),
					Suggestion: fmt.Sprintf("connect %q to the part of the flow that runs before %q, or remove it from the depends set of %q.",
						dep, id, id),
				})
			case !cut()[dep]:
				targets := suggestedTargets(node, inbound[id], cut())
				issue := ValidationIssue{
					Code:      CodeDependDeadlock,
					Severity:  SeverityError,
					NodeId:    id,
					DependsOn: dep,
					EdgeId:    edgeIdBetween(nodes, dep, id),
					Message: fmt.Sprintf("node %q depends on %q, but %q can only be reached by running %q first: on the first crossing %q waits for %q while %q is still queued behind it, so the flow deadlocks at %q. A node may not depend on a step that comes after it",
						id, dep, dep, id, id, dep, dep, id),
					SuggestedTargets: targets,
				}
				issue.Suggestion = deadlockSuggestion(issue, targets)
				report.Issues = append(report.Issues, issue)
			}

			if nodes[dep] != nil && dep != id {
				if _, ok := inbound[id][dep]; !ok {
					report.Issues = append(report.Issues, ValidationIssue{
						Code:      CodeDependWithoutEdge,
						Severity:  SeverityWarning,
						NodeId:    id,
						DependsOn: dep,
						Message: fmt.Sprintf("node %q depends on %q, but %q has no transition into %q",
							id, dep, dep, id),
						Suggestion: fmt.Sprintf("connect %q to %q, or drop the stale depends entry.",
							dep, id),
					})
				}
			}
		}
	}
	return report
}

func deadlockSuggestion(issue ValidationIssue, targets []string) string {
	edge := "the back edge"
	if issue.EdgeId != "" {
		edge = fmt.Sprintf("edge %q", issue.EdgeId)
	}
	if len(targets) == 0 {
		return fmt.Sprintf("re-point %s so it targets a node that already runs before %q, or remove %q from the depends set of %q.",
			edge, issue.NodeId, issue.DependsOn, issue.NodeId)
	}
	return fmt.Sprintf("re-point %s from %q so it targets a node connected to %q that already runs before it (one of: %s). The loop then re-enters %q through its existing dependencies and stays valid. Otherwise remove %q from the depends set of %q.",
		edge, issue.DependsOn, issue.NodeId, strings.Join(quoted(targets), ", "),
		issue.NodeId, issue.DependsOn, issue.NodeId)
}

// suggestedTargets lists the nodes the offending edge may legally target: the
// waiting node's own dependencies and inbound sources that stay reachable when
// the waiting node is cut out of the graph.
func suggestedTargets(node *models.Node, inbound map[string]string, without map[string]bool) []string {
	candidates := make(map[string]bool)
	for _, dep := range node.Depends {
		if without[dep] {
			candidates[dep] = true
		}
	}
	for src := range inbound {
		if without[src] {
			candidates[src] = true
		}
	}
	out := make([]string, 0, len(candidates))
	for id := range candidates {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// reachableFrom walks Next transitions from startNodeId, optionally cutting one
// node out of the graph. Inactive transitions are followed on purpose: the guard
// only reports a deadlock it is certain about, and Next.Active may be flipped by
// the app after compilation.
func reachableFrom(startNodeId string, nodes map[string]*models.Node, skip string) map[string]bool {
	seen := make(map[string]bool)
	if startNodeId == skip {
		return seen
	}
	seen[startNodeId] = true
	stack := []string{startNodeId}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		node := nodes[cur]
		if node == nil {
			continue
		}
		for _, next := range node.Next {
			if next.NodeId == skip || seen[next.NodeId] {
				continue
			}
			seen[next.NodeId] = true
			stack = append(stack, next.NodeId)
		}
	}
	return seen
}

// inboundIndex maps every node id to its inbound sources, keeping the edge id
// carried in Next.Meta so an issue can name the edge to re-point.
func inboundIndex(nodes map[string]*models.Node) map[string]map[string]string {
	index := make(map[string]map[string]string)
	for id, node := range nodes {
		if node == nil {
			continue
		}
		for _, next := range node.Next {
			if _, ok := index[next.NodeId]; !ok {
				index[next.NodeId] = make(map[string]string)
			}
			index[next.NodeId][id] = edgeIdOf(next)
		}
	}
	return index
}

func edgeIdBetween(nodes map[string]*models.Node, source, target string) string {
	node := nodes[source]
	if node == nil {
		return ""
	}
	for _, next := range node.Next {
		if next.NodeId == target {
			return edgeIdOf(next)
		}
	}
	return ""
}

func edgeIdOf(next models.Next) string {
	if id, ok := next.Meta["edgeId"].(string); ok {
		return id
	}
	return ""
}

func sortedNodeIds(nodes map[string]*models.Node) []string {
	ids := make([]string, 0, len(nodes))
	for id := range nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func quoted(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, fmt.Sprintf("%q", v))
	}
	return out
}
