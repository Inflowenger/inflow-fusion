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

// AllSvcSubjects returns the registered extrinsic services as a
// topicKey -> subject-template map with plain string values. This is what an
// extrinsic extension binds to (the compiler's extension branch and the
// inspector's BindTo picker).
func AllSvcSubjects() map[string]string {
	out := map[string]string{}
	for key, topic := range GetAllSvcs() {
		out[key] = string(topic)
	}
	return out
}