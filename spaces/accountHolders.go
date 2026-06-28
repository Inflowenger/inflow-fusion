package InfraSpaces

import (
	"maps"
	"sync"

	"github.com/Inflowenger/inflow-fusion/models"
)

var spaceHolder *InfraSpaceholder

type InfraSpaceholder struct {
	svc   map[string]*models.Account
	mutex sync.RWMutex
}

func (s *InfraSpaceholder) get(name string) (*models.Account, bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	svc, ok := s.svc[name]
	return svc, ok
}
func (s *InfraSpaceholder) getAll() map[string]*models.Account {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return maps.Clone(s.svc)
}
func (s *InfraSpaceholder) set(name string, topic *models.Account) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.svc[name] = topic
}
func GetInfraSpaces() *InfraSpaceholder {
	if spaceHolder == nil {
		spaceHolder = &InfraSpaceholder{svc: map[string]*models.Account{}}
	}
	return spaceHolder
}
