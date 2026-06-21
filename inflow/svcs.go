package inflow

import (
	"github.com/nats-io/nats.go"
)



type IInflowService interface {
	RetrieveContext(msg *nats.Msg)
	UpdateContext(msg *nats.Msg)
	// UpdateContextHeader(msg *nats.Msg) 
	RetrieveFlow(msg *nats.Msg) 
}




