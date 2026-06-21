package compilers

import (
	"fmt"
	"os"
	"testing"

	"github.com/Inflowenger/inflow-fusion/models"
	"github.com/bytedance/sonic"
)

func TestVueFlowCompile(t *testing.T) {
	data, err := os.ReadFile("vueFlow.json")
	if err != nil {
		panic(err)
	}
	flow := VueFlow{}
	sonic.Unmarshal(data, &flow)
	vueFlowCompiler := NewVueFlowCompiler(WithEachNodeFunc(func(fnode VueFlowNode) *models.Node {
		node := &models.Node{
			ID:   fnode.ID,
			Type: models.CodeNodeType,
		}

		return node

	}))

	graph := vueFlowCompiler.Compile("start_node", flow)
	b, _ := sonic.Marshal(graph)
	fmt.Println(string(b))
}
