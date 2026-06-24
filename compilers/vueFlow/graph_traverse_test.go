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
	vueFlowCompiler := NewVueFlowCompiler(WithEachNodeFunc(func(fnode VueFlowNode) (*models.Node,error) {
		node := &models.Node{
			ID:   fnode.ID,
			Type: models.CodeNodeType,
		}

		return node,nil

	}))

	graph , errs:= vueFlowCompiler.Compile("start_node", flow)
	if len(errs)>0{
		for nodeId,err:=range errs{
			fmt.Printf("there is error with %s : %s",nodeId,err.Error())
		}
	}
	b, _ := sonic.Marshal(graph)
	fmt.Println(string(b))
}
