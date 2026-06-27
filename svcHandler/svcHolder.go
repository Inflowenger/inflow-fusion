package svcHandler

import (
	"maps"
	"sync"
)

var extHolder *ExtSvcHolder

type ExtSvcHolder struct {
	svc   map[string]SvcTopic
	mutex sync.RWMutex
}

func (es *ExtSvcHolder) get(name string) (SvcTopic, bool) {
	es.mutex.Lock()
	defer es.mutex.Unlock()
	svc, ok := es.svc[name]
	return svc, ok
}
func (es *ExtSvcHolder) getAll() map[string]SvcTopic {
	es.mutex.RLock()
	defer es.mutex.RUnlock()
	return maps.Clone(es.svc)
}
func (es *ExtSvcHolder) set(name string, topic SvcTopic) {
	es.mutex.Lock()
	defer es.mutex.Unlock()
	es.svc[name] = topic
}
func GetExtrinsicSvcs() *ExtSvcHolder {
	if extHolder == nil {
		extHolder = &ExtSvcHolder{svc: map[string]SvcTopic{}}
	}
	return extHolder
}

func GetSvc(name string)SvcTopic{
	svc,ok:=GetExtrinsicSvcs().get(name)
	if ok{
		return svc
	}
	return ""
}
func GetAllSvcs()map[string]SvcTopic{
	return GetExtrinsicSvcs().getAll()
}