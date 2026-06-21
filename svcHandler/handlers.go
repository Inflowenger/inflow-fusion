package svcHandler

import (
	"fmt"

	natsHandler "github.com/Inflowenger/inflow-fusion/nats"
	// "github.com/Inflowenger/inflow-fusion/nodes"
	"github.com/nats-io/nats.go"
)

// func ImplHandlerForSvcNode(ev nodes.EventSvcNode, handler func(header nats.Header, data []byte) ([]byte, error)) (*nats.Subscription, error) {
// 	natsBox, err := natsHandler.GetNatsByInfraIsolate(ev.InfraIsolated)
// 	if err != nil {
// 		return nil,err
// 	}
// 	con := natsBox.GetConnection()
// 	if con == nil {
// 		return nil,fmt.Errorf("no nats connection found for account %s", ev.InfraIsolated.Account)
// 	}
// 	return  con.Subscribe(ev.Subject, func(msg *nats.Msg) {
// 		if msg.Header == nil {
// 			msg.Header = nats.Header{}
// 		}
// 		msg.Header.Set("recv_subject", msg.Subject)
// 		resp, err := handler(msg.Header, msg.Data)
// 		if err != nil {
// 			msg.Respond([]byte(fmt.Sprintf(`{"error": "%v"}`, err)))
// 			return
// 		}
// 		msg.Respond(resp)
// 	})

// }

func ImplHandlerOnSubject(subject SvcTopic, handler func(header nats.Header, data []byte) ([]byte, error)) error {

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

	return nil
}
