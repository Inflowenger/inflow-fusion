package inflow

import (
	"github.com/Inflowenger/inflow-fusion/models"
	roundrobin "github.com/thegeekyasian/round-robin-go"
)

type InflowResource struct {
	Name string
	Url  string
	Tags []string
}

var resourceCandidate *roundrobin.RoundRobin[InflowResource]

func SetResourceCandid(list []models.RegisteredInflow) (*roundrobin.RoundRobin[InflowResource], error) {
	resourcesList := []*InflowResource{}
	for _, el := range list {
		resourcesList = append(resourcesList, &InflowResource{Name: el.Name, Url: el.Url, Tags: el.Tags})
	}
	var err error
	resourceCandidate, err = roundrobin.New(resourcesList...)
	return resourceCandidate, err

}

func GetResourceCandid() *InflowResource {
	if resourceCandidate == nil {
		return nil
	}
	return resourceCandidate.Next()
}
