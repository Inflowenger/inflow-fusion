package inflowport

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/Inflowenger/inflow-fusion/etc"
	"github.com/Inflowenger/inflow-fusion/inflow"
	"github.com/Inflowenger/inflow-fusion/models"
	"github.com/Inflowenger/inflow-fusion/nodes"
	InfraSpaces "github.com/Inflowenger/inflow-fusion/spaces"
	"github.com/Inflowenger/inflow-fusion/svcHandler"
	"github.com/bytedance/sonic"
	"github.com/nats-io/nats.go"
)

func TestBackend(t *testing.T) {
	os.Setenv("INFLOW_INFRA_API", "http://localhost:8022")
	os.Setenv("INFLOW_INFRA_JWT_SECRET", "PAykm7EqmSlmJ0NmU2t6WlNYOGRbviMnZMvEOe6RBw5s3tI6ZXlVxMJ9os57rCCC")
	implInflowBackendInterface := ImplSvcExample{}
	err := inflow.InitBackend(inflow.WithImplementedBackendBy(&implInflowBackendInterface))
	if err != nil {
		panic(err)
	}
	slog.Default().Info("inflowenger backend initialized successfully")

	// define a service node to save entity data on different tables based on subject
	saveEntitynode := nodes.NewExtrinsicSvcNode("my.internal.svc.persist.*") // wildcards can be used as tables or collections
	err = svcHandler.ImplHandlerOnSubject("db_handler", svcHandler.SvcTopic(saveEntitynode.Subject), func(header nats.Header, data []byte) ([]byte, error) {
		subject := header.Get("recv_subject")
		fmt.Printf("recieved Message On Subject %s with data %s\n", subject, string(data))
		table := strings.Split(subject, ".")[4]
		saveOnTable(table, data)
		return []byte(`{"status":"saved successfully"}`), nil
	})
	if err != nil {
		panic(fmt.Errorf("failed to create service node : %v", err))
	}
	fmt.Println("New SVC handler registered On  ", svcHandler.SvcTopic(saveEntitynode.Subject).ConvertToSubscribe())
	///
	//Insert saveEntitynode to Nodes to show in frontend side
	nodeInmydb := struct {
		nodes.ExtrinsicSvcNode `json:",inline" bson:",inline"`
		UniqID                 string `json:"id" bson:"_id"`
	}{}
	InsertNodeTodb(nodeInmydb)
	select {}
}

func saveOnTable(table string, data []byte) {
	fmt.Printf("data saved to table %s: %s\n", table, string(data))
}

func InsertNodeTodb[T any](inflowNode T) {
	// this is just a sample function to show how to insert the service node to db and show in frontend side
	// in real scenario, you can use any db and any orm to save the node data

}

func TestPlugin(t *testing.T) {
	os.Setenv("INFLOW_INFRA_API", "http://localhost:8022")
	os.Setenv("INFLOW_INFRA_JWT_SECRET", "PAykm7EqmSlmJ0NmU2t6WlNYOGRbviMnZMvEOe6RBw5s3tI6ZXlVxMJ9os57rCCC")
	implInflowBackendInterface := ImplSvcExample{}
	err := inflow.InitBackend(inflow.WithImplementedBackendBy(&implInflowBackendInterface))
	if err != nil {
		panic(err)
	}
	slog.Default().Info("inflowenger backend initialized successfully")

	plugin, err := nodes.NewPluginNode("jira")
	if err != nil {
		panic(err)
	}
	fmt.Println(plugin)
	select {}
}

func TestSafePluginPermission(t *testing.T) {
	os.Setenv("INFLOW_INFRA_API", fmt.Sprintf("http://%s:8022", etc.MyHostname()))
	os.Setenv("INFLOW_INFRA_JWT_SECRET", "PAykm7EqmSlmJ0NmU2t6WlNYOGRbviMnZMvEOe6RBw5s3tI6ZXlVxMJ9os57rCCC")
	implInflowBackendInterface := ImplSvcExample{}
	err := inflow.InitBackend(inflow.WithImplementedBackendBy(&implInflowBackendInterface))
	if err != nil {
		panic(err)
	}
	slog.Default().Info("inflowenger backend initialized successfully")

	plugin, err := nodes.NewPluginNode("jira", nodes.WithUniqId[*nodes.PluginNode]("id-a-b-c"))
	if err != nil {
		panic(err)
	}
	perm := InfraSpaces.PluginCredentialStrictPermission("remote-client-user", plugin.UniqId, plugin.InfraIsolated.Account)
	cred, err := InfraSpaces.CreateUserCredential(plugin.InfraIsolated.Seed, perm)
	if err != nil {
		panic(err)
	}
	fmt.Println(cred)
	fmt.Println(plugin)
	select {}
}

func TestUUID(t *testing.T) {
	fmt.Println(etc.UuidLastPart("d8c6bf11-483b-4e0d-abb8-9ded50e28dbb"))
}

func TestResources(t *testing.T) {
	os.Setenv("INFLOW_INFRA_API", fmt.Sprintf("http://%s:8022", etc.MyHostname()))
	os.Setenv("INFLOW_INFRA_JWT_SECRET", "PAykm7EqmSlmJ0NmU2t6WlNYOGRbviMnZMvEOe6RBw5s3tI6ZXlVxMJ9os57rCCC")
	implInflowBackendInterface := ImplSvcExample{}
	err := inflow.InitBackend(inflow.WithImplementedBackendBy(&implInflowBackendInterface))
	if err != nil {
		panic(err)
	}
	//get
	l, err := inflow.GetInflowBackend().ReloadResources(100)
	b, err := sonic.Marshal(l)
	fmt.Println(string(b), err)

	pid, err := inflow.NewProcess("node0" /*inflow.WithInflowInstanceUrl("https://cpux:3002"),*/, inflow.WithContextDocument("ctx-123"), inflow.WithFlowId("f-a-122"))
	fmt.Println("New Process Error ", err)
	b, err = sonic.Marshal(pid)
	fmt.Println(string(b), err)

	candid := inflow.GetResourceCandid()
	fmt.Println(candid)
}

func TestSoundRobinResource(t *testing.T) {
	inflow.SetResourceCandid([]models.RegisteredInflow{
		{RegisterRequestBody: models.RegisterRequestBody{Name: "cpu1", Url: "cpu1"}},
		{RegisterRequestBody: models.RegisterRequestBody{Name: "cpu2", Url: "cpu2"}},
	})
	fmt.Println(inflow.GetResourceCandid())
	fmt.Println(inflow.GetResourceCandid())
	fmt.Println(inflow.GetResourceCandid())

}
