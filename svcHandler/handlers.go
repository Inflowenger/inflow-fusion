package svcHandler

import (
	"fmt"

	natsHandler "github.com/Inflowenger/inflow-fusion/nats"
	"github.com/nats-io/nats.go"
)

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
