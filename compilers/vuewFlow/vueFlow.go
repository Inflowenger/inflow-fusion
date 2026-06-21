package compilers

import "github.com/Inflowenger/inflow-fusion/models"

type VueFlowCompiler struct {
	nodes   map[string]*models.Node
	hook    func(VueFlowNode) *models.Node
	vueFlow VueFlow
}

func (v *VueFlowCompiler) SetHookFunc(f func(VueFlowNode) *models.Node) {
	v.hook = f
}
func NewVueFlowCompiler(opts ...func(*VueFlowCompiler)) *VueFlowCompiler {
	comp := &VueFlowCompiler{nodes: make(map[string]*models.Node), hook: func(fn VueFlowNode) *models.Node {
		return &models.Node{Type: models.VoidNodeType, Title: fn.Type, ID: fn.ID}

	}}
	for _, o := range opts {
		o(comp)
	}
	return comp
}
func WithEachNodeFunc(f func(VueFlowNode) *models.Node) func(*VueFlowCompiler) {
	return func(vfc *VueFlowCompiler) {
		vfc.SetHookFunc(f)
	}
}

func (v *VueFlowCompiler) Compile(startNodeId string, flow VueFlow) map[string]*models.Node {
	v.vueFlow = flow
	// create nodes
	startNode := v.getNode(startNodeId)
	if startNode != nil {
		v.nodes[startNode.ID] = v.hook(*startNode)
		v.walk(startNode)
	}
	return v.nodes
}

func (v *VueFlowCompiler) walk(flowNode *VueFlowNode) error {

	inflowNode := v.hook(*flowNode)

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
			if _, ok := v.nodes[e.Target]; ok {
				continue
			}
			next := v.getNode(e.Target)

			err := v.walk(next)
			if err != nil {
				return err
			}
		}

	}

	v.nodes[flowNode.ID] = inflowNode
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
