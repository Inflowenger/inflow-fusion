package inflow

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"time"

	"github.com/Inflowenger/inflow-fusion/etc"
	"github.com/Inflowenger/inflow-fusion/models"
	"github.com/bytedance/sonic"
)

type Process struct {
	req         models.ProcessRequest 
	resourceUrl string	
}

func NewProcess(startNodeId string, opts ...func(*Process)) (*Process, error) {

	p := &Process{
		resourceUrl: "",
		req: models.ProcessRequest{StartNodeId: startNodeId,
			Settings: models.Settings{
				RequestTimeOut:   5,
				ProcessNodeLimit: 1000,
				ExecuteTimeOut:   int64(time.Hour.Seconds())}}}
	for _, o := range opts {
		o(p)
	}
	if p.req.PID == "" {
		p.req.PID = etc.UUID()
	}
	backend := GetInflowBackend()
	if backend == nil {
		return nil, errors.New("inflow backend init is required before any request")
	}
	if p.resourceUrl == "" {
		candidInflow := GetResourceCandid()
		if candidInflow == nil {
			return nil, errors.New("no any inflow resource found.")
		} else {
			p.resourceUrl = candidInflow.Url
		}
	}
	resurl, err := url.Parse(p.resourceUrl)
	if err != nil {
		return nil, errors.New("invalid value of inflow  resource url")

	}
	if resurl.Port() == "" {
		p.resourceUrl = fmt.Sprintf("%s:%s", p.resourceUrl, models.INFLOW_REST_PORT)
	}
	if resurl.Scheme == "" {
		p.resourceUrl = fmt.Sprintf("http://%s", p.resourceUrl)

	}

	if p.req.Context.ContextId == "" {
		return nil, errors.New("contextId is required")
	}
	if p.req.Flow.FlowId == "" {
		return nil, errors.New("flowId is required")

	}
	vars := map[string]any{"contextId": p.req.Context.ContextId, "flowId": p.req.Flow.FlowId}
	for k, el := range p.req.Meta {
		vars[k] = el
	}
	if p.req.Context.Getter == "" {
		p.req.Context.Getter = backend.contextGetSvcTopic.MakeReqSubjectWithParams(vars)
	}
	if p.req.Context.Setter == "" {
		p.req.Context.Setter = backend.contextSetSvcTopic.MakeReqSubjectWithParams(vars)
	}

	if p.req.Flow.GetFlow == "" {
		p.req.Flow.GetFlow = string(backend.flowGetSvcTopic)
	}

	return p, nil
}

func WithProcessTimeout(t time.Duration) func(*Process) {
	return func(p *Process) {
		p.req.Settings.ExecuteTimeOut = int64(t.Seconds())
	}
}

func WithFlowId(flowId string) func(*Process) {
	return func(p *Process) {
		p.req.Flow.FlowId = flowId
	}
}
func WithContextDocument(docId string) func(*Process) {
	return func(p *Process) {
		p.req.Context.ContextId = docId
	}
}
func WithPID(processId string) func(*Process) {
	return func(p *Process) {
		p.req.PID = processId
	}
}
func WithInflowInstanceUrl(url string) func(*Process) {
	return func(p *Process) {
		p.resourceUrl = url
	}
}

/*
meta data ship with headers in all registered services as index and query use
also meta key value contribute for replace value in service subject pattern
*/
func WithMeta(meta map[string]string) func(*Process) {
	return func(p *Process) {
		p.req.Meta = meta
	}
}
func (p *Process)GetRequest()models.ProcessRequest{
	return p.req
}
func (p *Process)GetResource()string{
	return p.resourceUrl
}
func (p *Process) Exec(ctx context.Context) (*models.ProcessResponse,error) {
	backend := GetInflowBackend()
	if backend == nil {
		return nil,errors.New("inflow backend init is required before any request")
	}
	url:=fmt.Sprintf("%s/engine",p.resourceUrl)
	response, err := etc.SendHttpPost(ctx, map[string]string{"Authorization": backend.GetBearerToken()},url, p.req)
	if err!=nil{
		return nil,err
	}
	if !slices.Contains([]int{200,202},response.Status()){
		return nil,fmt.Errorf("%s",response.Body())
	}
	newProcRes:=&models.ProcessResponse{}
	err=sonic.Unmarshal(response.Body(),newProcRes)
	return newProcRes,err
}
