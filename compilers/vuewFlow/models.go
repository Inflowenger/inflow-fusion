package compilers

type VueFlow struct {
	Nodes []VueFlowNode `json:"nodes"`
	Edges []Edges       `json:"edges"`
}
type Dimensions struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}
type ComputedPosition struct {
	X int `json:"x"`
	Y int `json:"y"`
	Z int `json:"z"`
}
type Position struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type Source struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	NodeID   string `json:"nodeId"`
	Position string `json:"position"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
}
type Target struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	NodeID   string `json:"nodeId"`
	Position string `json:"position"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
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
	SourceX      int         `json:"sourceX"`
	SourceY      int         `json:"sourceY"`
	TargetX      int         `json:"targetX"`
	TargetY      int         `json:"targetY"`
	SourceNode   VueFlowNode `json:"sourceNode"`
	TargetNode   VueFlowNode `json:"targetNode"`
}
