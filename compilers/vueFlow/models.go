package compilers

type VueFlow struct {
	Nodes    []VueFlowNode `json:"nodes"`
	Edges    []Edges       `json:"edges"`
	Position FlowPosition  `json:"position"`
}
type FlowPosition struct {
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Zoom float64 `json:"zoom"`
}
type Dimensions struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}
type ComputedPosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Source struct {
	ID       string  `json:"id"`
	Type     string  `json:"type"`
	NodeID   string  `json:"nodeId"`
	Position string  `json:"position"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Width    float64 `json:"width"`
	Height   float64 `json:"height"`
}
type Target struct {
	ID       string  `json:"id"`
	Type     string  `json:"type"`
	NodeID   string  `json:"nodeId"`
	Position string  `json:"position"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Width    float64 `json:"width"`
	Height   float64 `json:"height"`
}
type HandleBounds struct {
	Source []Source `json:"source"`
	Target []Target `json:"target"`
}
type VueFlowNode struct {
	ID               string           `json:"id"`
	Type             string           `json:"type"`
	Dimensions       Dimensions       `json:"dimensions"`
	ComputedPosition ComputedPosition `json:"computedPosition"`
	Selected         bool             `json:"selected"`
	Dragging         bool             `json:"dragging"`
	Resizing         bool             `json:"resizing"`
	Initialized      bool             `json:"initialized"`
	IsParent         bool             `json:"isParent"`
	Position         Position         `json:"position"`
	Data             any              `json:"data"`
	Events           any              `json:"events"`
	HandleBounds     HandleBounds     `json:"handleBounds,omitempty"`
}
type EdgePayload struct {
	Tags     []string `json:"tags"`
	EdgeType string   `json:"edgeType"`
}

type Edges struct {
	ID           string      `json:"id"`
	Type         string      `json:"type"`
	Source       string      `json:"source"`
	Target       string      `json:"target"`
	SourceHandle string      `json:"sourceHandle"`
	TargetHandle string      `json:"targetHandle"`
	Data         EdgePayload `json:"data"`
	Events       any         `json:"events"`
	Label        string      `json:"label"`
	MarkerEnd    string      `json:"markerEnd"`
	Animated     bool        `json:"animated"`
	SourceX      float64     `json:"sourceX"`
	SourceY      float64     `json:"sourceY"`
	TargetX      float64     `json:"targetX"`
	TargetY      float64     `json:"targetY"`
	SourceNode   VueFlowNode `json:"sourceNode"`
	TargetNode   VueFlowNode `json:"targetNode"`
}
