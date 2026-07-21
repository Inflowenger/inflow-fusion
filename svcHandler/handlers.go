package svcHandler

import (
	"fmt"

	"github.com/Inflowenger/inflow-fusion/models"
	natsHandler "github.com/Inflowenger/inflow-fusion/nats"
	"github.com/bytedance/sonic"
	"github.com/nats-io/nats.go"
)

func ImplHandlerOnSubject(name string,subject SvcTopic, handler func(header nats.Header, data []byte) ([]byte, error)) error {

	con, err := natsHandler.GetInfraNats()
	if err != nil {
		return err
	}

	_, err = con.Subscribe(subject.ConvertToSubscribe(), func(msg *nats.Msg) {
		if msg.Header == nil {
			msg.Header = nats.Header{}
		}
		msg.Header.Set("recv_subject", msg.Subject)
		resp, err := handler(msg.Header, msg.Data)
		if err != nil {
			msg.Respond([]byte(fmt.Sprintf(`{"error": "%v"}`, err)))
			return
		}
		msg.Respond(resp)
	})
	if err != nil {
		return err
	}
	GetExtrinsicSvcs().set(name,subject)
	return nil
}

func StopHereResponse(data map[string]any)([]byte , error){
	return MakeCmdResponse(models.CmdStop,data)

}
func FilterNextResponse(data map[string]any, nextTags []string)([]byte , error){
	data[string(models.SvcCmdResponseNextFilterKey)] = nextTags 
	return MakeCmdResponse(models.CmdNextFilter,data)

}
func MakeCmdResponse(cmd models.SvcReturnCommand,data map[string]any)([]byte,error){
	data[string(models.SvcCmdResposeKey)] = cmd
	return sonic.Marshal(data)
}