package inflowport

import (
	"fmt"
	"strings"

	"github.com/Inflowenger/inflow-fusion/models"
	"github.com/bytedance/sonic"
	"github.com/nats-io/nats.go"
)

type ImplSvcExample struct{}

func (isvc *ImplSvcExample) RetrieveContext(msg *nats.Msg) {
	fmt.Println(msg.Header)
	// get context data from db
	msg.Respond([]byte(`{"header":{},"data":"{\"node1\":{\"b\":2,\"sum\":3,\"a\":1},\"node2\":{\"a\":1,\"b\":2,\"sum\":3}}"}`))
}

func (isvc *ImplSvcExample) UpdateContext(msg *nats.Msg) {
	fmt.Println(string(msg.Data)) // save to db
	msg.Respond([]byte(`accepted`))
}

func (isvc *ImplSvcExample) RetrieveFlow(msg *nats.Msg) {
	fmt.Println(msg.Header.Get("spaceId"))
	fmt.Println("Get Flow With ID : ", msg.Subject)
	subpart := strings.Split(msg.Subject, ".")
	flow := models.Flow{
		UUID: subpart[len(subpart)-1],
		Info: map[string]any{"title": "FlowNo.1"},
		Nodes: []models.Node{

			{
				ID:   "node0",
				Type: models.VoidNodeType,
				Next: []models.Next{{NodeId: "node1"}, {NodeId: "node2"}, {NodeId: "node3"}},
			},
			{
				ID:    "node1",
				Type:  models.CodeNodeType,
				Scope: "$.node1",
				Code: &models.CodeRule{
					Lang: "js",
					LogicRule: `
						input.a = input.b*  input.b
						input
					`,
				},
				Next: []models.Next{},
			},
			{
				ID:    "node2",
				Type:  models.CodeNodeType,
				Scope: "$.node2",
				Code: &models.CodeRule{
					Lang: "js",
					LogicRule: `

					x = {"sumx": input.a + input.b}
					x
		
					`,
				},
				Next: []models.Next{{NodeId: "node4"}, {NodeId: "node5"}},
			},
			{
				ID:    "node3",
				Type:  models.CodeNodeType,
				Scope: "$.node3",
				Code: &models.CodeRule{
					Lang: "js",
					LogicRule: `
						let init = {a:1,b:2}
						init.sum = init.a + init.b
						input.result = init.sum=21
						input

					`,
				},
				Next: []models.Next{
					{NodeId: "node6"},
				},
			},
			{
				ID:    "node4",
				Type:  models.CodeNodeType,
				Scope: "$.node4",
				Code: &models.CodeRule{
					Lang:      "opa",
					OpaResult: "result",
					LogicRule: `
					c= 5
					allow if {
							c < 10
							}
					result = {"f":c*12,"allow": "unknownnnn"}
					`,
				},
				Next: []models.Next{},
			},
			{
				ID:    "node5",
				Type:  models.CodeNodeType,
				Scope: "$.node5",
				Key:   "calc_result",
				Code: &models.CodeRule{
					Lang:    "opa",
					OpaData: map[string]any{"threshold": 10},
					LogicRule: `
							c= 8
				allow if {
							c < 10
						}
					myc = [c*2,data.threshold*2]
					`,
				},
				Next: []models.Next{},
			},
			{
				ID:    "node6",
				Type:  models.RuleNodeType,
				Scope: "$.node6",
				Contract: &models.ContractRule{
					Lang:       "opa",
					Conditions: map[string]any{"threshold": 10},
					OpaResult:  "next",
					LogicRule: `
							c= 8
						default next:=["else"]
						next := ["a", "b"] if {
						c < data.threshold
								}
					`,
				},
				Next: []models.Next{{NodeId: "node7", Tags: []string{"a"}}},
			},
			{
				ID:    "node7",
				Type:  models.RuleNodeType,
				Scope: "$.node7",
				Contract: &models.ContractRule{
					Lang:       "js",
					Conditions: map[string]any{"threshold": 10},
					LogicRule: `
							c= 8
					if (c < data.threshold) {
						next = ["a"]
					} else {
						next = ["else"]
					}
					next
					`,
				},
				Next: []models.Next{{NodeId: "node8", Tags: []string{"a"}}},
			},
			{
				ID:    "node8",
				Type:  models.ExtrinsicNodeType,
				Scope: "$.node8",
				Key:   "result",
				Extrinsic: &models.ExtrinsicRule{
					Subject: "my.internal.svc.persist.task",
					// `op` is not a static payload: the engine resolves runtime
					// variables in every root-level string value against the flow
					// context just before publishing, so the handler receives values
					// rather than templates. See docs/nodes/extrinsic.md.
					OperationData: map[string]any{
						"taskId": 123,    // plain value, passed through as-is
						"status": "done", // plain value

						// A value that is exactly one placeholder keeps the context
						// value's JSON type: `sum` arrives as the number 3, not "3".
						// (node1 emits its input back, so $.node1.sum is 3 whether or
						// not that branch has run yet.)
						"sum": "{{$.node1.sum}}",

						// A placeholder inside longer text is interpolated instead, so
						// `note` is always a string: "node1 squared b into 4".
						"note": "node1 squared b into {{$.node1.a}}",

						// `$this` is this node's own location — whatever `Scope` above
						// selects. Nothing writes $.node8 in this flow, so here it
						// arrives as {}; it earns its keep when a scope selects many
						// (`$.orders[*]`), where the node runs once per location and
						// `$this` follows each pass instead of pinning an index.
						"current": "{{$this}}",
					},
				},
				Next: []models.Next{},
			},
		},
	}
	flow.ValidateNext()
	b, _ := sonic.Marshal(flow)
	msg.Respond(b)

}
