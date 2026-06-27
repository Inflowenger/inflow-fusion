package models

type Flow struct {
	UUID  string         `json:"uuid"`
	Info  map[string]any `json:"info"`
	Nodes []Node         `json:"nodes"`
}

func (f *Flow) ValidateNext() {
	for i, el := range f.Nodes {
		sanitizedNexts := []Next{}

		for _, nextNode := range el.Next {
			if nextNode.FlowId == "" {
				nextNode.FlowId = f.UUID
			}
			sanitizedNexts = append(sanitizedNexts, nextNode)

		}
		f.Nodes[i].Next = sanitizedNexts
	}
}
func (f *Flow) GetFlowInfo(key string) any {
	return f.Info[key]
}

func (f *Flow) FindNodeById(id string) *Node {
	for _, node := range f.Nodes {
		if node.ID == id {
			return &node
		}
	}
	return nil
}

type Node struct {
	ID    string    `json:"uuid"`
	Type  NodeType  `json:"type"`
	Title string    `json:"title"`
	Key   string    `json:"key"`
	Scope string    `json:"scope"` // json path
	Code  *CodeRule `json:"code,omitempty"`
	GoTo  *GoToRule `json:"goto,omitempty"`
	Extrinsic  *ExtrinsicRule `json:"extrinsic,omitempty"`
	Plugin     *PluginRule    `json:"plugin,omitempty"`
	Contract   *ContractRule  `json:"contract,omitempty"`
	Meta       map[string]any `json:"meta"`
	Tags       string         `json:"tags"`
	Next       []Next         `json:"next"`
	Depends    []string       `json:"depends"`      // wait to complete all inbounds nodes finished
	NextFilter []string       `json:"next_filters"` // filter next nodes by edges tags
}

type Next struct {
	FlowId string         `json:"flowId"`
	NodeId string         `json:"nodeId"`
	Tags   []string       `json:"tags"`
	Meta   map[string]any `json:"meta"`
}
