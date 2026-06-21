package svcHandler

import (
	"github.com/Inflowenger/inflow-fusion/etc"
)

type SvcTopic string

const (
	DefaultGetFlowSvc    SvcTopic = "inflow.req.flow.get.{flowId}"
	DefaultGetContextSvc SvcTopic = "inflow.req.context.get.{contextId}"
	DefaultSetContextSvc SvcTopic = "inflow.req.context.set.{contextId}"
)

func (st SvcTopic) ConvertToSubscribe() string {
	return etc.ReplaceAllWith(string(st), "*")
}

func (st SvcTopic) MakeReqSubjectWithParams(args map[string]any) string {
	return etc.ReplaceByMapString(string(st), args)
}
