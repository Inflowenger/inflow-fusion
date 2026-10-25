package compilers

import (
	"fmt"

	"github.com/Inflowenger/inflow-fusion/models"
)

type VueFlowCompiler struct {
	nodes   map[string]*models.Node
	hook    func(VueFlowNode) (*models.Node, error)
	vueFlow VueFlow
	guard   bool
	report  ValidationReport
}

func (v *VueFlowCompiler) SetHookFunc(f func(VueFlowNode) (*models.Node, error)) {
	v.hook = f
}
func NewVueFlowCompiler(opts ...func(*VueFlowCompiler)) *VueFlowCompiler {
	comp := &VueFlowCompiler{nodes: make(map[string]*models.Node), hook: func(fn VueFlowNode) (*models.Node, error) {
		return &models.Node{Type: models.VoidNodeType, Title: fn.Type, ID: fn.ID}, nil

	}}
	for _, o := range opts {
		o(comp)
	}
	return comp
}
func WithEachNodeFunc(f func(VueFlowNode) (*models.Node, error)) func(*VueFlowCompiler) {
	return func(vfc *VueFlowCompiler) {
		vfc.SetHookFunc(f)
	}
}

// WithDependsGuard runs the depends validation (see ValidateDepends) over the
// compiled map after the walk, merging its error-severity findings into the
// map[nodeId]error Compile returns and leaving the full report on Report().
//
// It is OFF by default, deliberately. The guard answers a question about ONE
// traversal — the one starting at the startNodeId it was given — and a caller
// compiling a graph in order to RUN it is often not making that traversal: a
// run resumed after a human task, or after a timer, enters at the nodes the
// parked node handed back, several of them, in the middle of the graph, with
// the engine seeding the earlier run's traversal state, so a join the guard
// calls deadlocked may be exactly the join whose dependencies are already
// satisfied. Refusing those compiles would stall correct work.
//
// So the guard belongs where a graph is EDITED — an editor saving a flow, a
// tool checking a draft — with an author in front of it, and its findings are
// help: the node, the edge to re-point and the targets it may point at
// instead. Opt in there, and read Report() for the warnings too.
func WithDependsGuard() func(*VueFlowCompiler) {
	return func(vfc *VueFlowCompiler) {
		vfc.guard = true
	}
}

// Report returns the guard outcome of the last Compile, including warnings,
// suggested fixes and the edge ids an app should point its user at. It is the
// zero report when the compiler was built without WithDependsGuard — a caller
// that wants the findings without the guard merging errors into Compile's map
// can always run ValidateDepends over the returned map itself.
func (v *VueFlowCompiler) Report() ValidationReport {
	return v.report
}

func (v *VueFlowCompiler) Compile(startNodeId string, flow VueFlow) (map[string]*models.Node, map[string]error) {
	v.vueFlow = flow
	v.nodes = make(map[string]*models.Node)
	v.report = ValidationReport{StartNodeId: startNodeId}
	cerror := make(map[string]error)
	// create nodes
	startNode := v.getNode(startNodeId)
	if startNode != nil {
		n, err := v.hook(*startNode)
		if err != nil {
			cerror[startNode.ID] = err
		}
		v.nodes[startNode.ID] = n
		if err := v.walk(startNode); err != nil {
			cerror[startNode.ID] = err
		}
	}
	if v.guard {
		v.report = ValidateDepends(startNodeId, v.nodes)
		for nodeId, err := range v.report.ErrorsByNode() {
			if prev, ok := cerror[nodeId]; ok {
				cerror[nodeId] = fmt.Errorf("%s; %s", prev.Error(), err.Error())
				continue
			}
			cerror[nodeId] = err
		}
	}
	return v.nodes, cerror
}

func (v *VueFlowCompiler) walk(flowNode *VueFlowNode) error {

	inflowNode, err := v.hook(*flowNode)
	if err != nil {
		return err
	}

	// connect VueFlowNode
	for _, e := range v.vueFlow.Edges {
		if e.Source == flowNode.ID {
			inflowNode.Next = append(inflowNode.Next, models.Next{
				NodeId: e.Target,
				Tags:   e.Data.Tags,
				Meta: map[string]any{
					"edgeId":     e.ID,
					"label":      e.Label,
					"edgeHandle": e.SourceHandle,
				}})

		}

	}
	v.nodes[flowNode.ID] = inflowNode
	for _, e := range inflowNode.Next {
		if _, ok := v.nodes[e.NodeId]; ok {
			continue
		}
		next := v.getNode(e.NodeId)

		err := v.walk(next)
		if err != nil {
			return err
		}
	}
	return nil
}

func (v *VueFlowCompiler) getNode(nodeId string) *VueFlowNode {
	for _, n := range v.vueFlow.Nodes {
		if n.ID == nodeId {

			return &n
		}
	}
	return nil
}

func (v *VueFlowCompiler) GetOutboundsEdges(nodeId string) (edges []Edges) {
	for _, e := range v.vueFlow.Edges {
		if e.Source == nodeId {
			edges = append(edges, e)
		}
	}
	return
}

func (v *VueFlowCompiler) GetInboundsEdges(nodeId string) (edges []Edges) {
	for _, e := range v.vueFlow.Edges {
		if e.Target == nodeId {
			edges = append(edges, e)
		}
	}
	return
}
