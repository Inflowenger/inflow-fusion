package inflow

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Inflowenger/inflow-fusion/etc"
	"github.com/Inflowenger/inflow-fusion/models"
	natsHandler "github.com/Inflowenger/inflow-fusion/nats"
	"github.com/Inflowenger/inflow-fusion/svcHandler"
	"github.com/golang-jwt/jwt/v5"
	"github.com/nats-io/nats.go"
)

type InflowWire struct {
	Infra              string
	hs                 string
	natsport           uint16
	token              string
	logger             *slog.Logger
	SvcImpl            IInflowService
	resources          []models.RegisteredInflow
	flowGetSvcTopic    svcHandler.SvcTopic
	contextGetSvcTopic svcHandler.SvcTopic
	contextSetSvcTopic svcHandler.SvcTopic
}

func (iw *InflowWire) getCred() (models.Cred, error) {
	cred, err := etc.SendHttpGet(context.Background(), map[string]string{"Authorization": iw.GetBearerToken()}, iw.Infra+"/account/inflow/cred", models.CredApiResponse{})
	if err != nil {
		return models.Cred{}, err
	}
	return cred.Data, nil

}
func (iw *InflowWire) GetInfraNatsUrl() string {

	return fmt.Sprintf("%s:%d", iw.hs, iw.natsport)

}

func (iw *InflowWire) GetBearerToken() string {

	return fmt.Sprintf("Bearer %s", iw.token)

}
// GetResourceToken returns the raw portal-signed token for the resource at rawUrl,
// or "" when no such resource is known or its portal carries no secret (callers
// then fall back to the infra bearer). URLs are compared on their normalized form:
// a process row stores the normalized url (p.GetResource()) while infra stores it
// bare, so a raw string match would miss and lose the resource's own credential —
// the root of the stop bug.
func (iw *InflowWire) GetResourceToken(rawUrl string) string {
	target, err := normalizeResourceUrl(rawUrl)
	if err != nil {
		target = rawUrl
	}
	for _, r := range iw.resources {
		norm, nerr := normalizeResourceUrl(r.Url)
		if nerr != nil {
			norm = r.Url
		}
		if norm == target && r.RegisterPortal.JwtSecret != "" {
			return makeTokenWithHs256(r.RegisterPortal.JwtSecret)
		}
	}
	return ""
}

// GetResourceBearerToken returns the Authorization header value for the resource at
// rawUrl: its portal token when it has one, otherwise the infra bearer.
func (iw *InflowWire) GetResourceBearerToken(rawUrl string) string {
	if t := iw.GetResourceToken(rawUrl); t != "" {
		return fmt.Sprintf("Bearer %s", t)
	}
	return fmt.Sprintf("Bearer %s", iw.token)

}
func makeTokenWithHs256(secret string)string{
		token := jwt.New(jwt.SigningMethodHS256)
		token.Claims = jwt.MapClaims{"admin": true}
		if secret == ""{
			return ""
		}
		encoded, err := token.SignedString([]byte(secret))
		if err!=nil{
			fmt.Println("in sign token with given secret error occurred ")
			return ""
		}
		return encoded
}
// Get all Inflow Instance (registered inflow instances) from infra and Add to Round-Robin struct to use by create new process function
func (iw *InflowWire) ReloadResources(limit int) ([]models.RegisteredInflow, error) {

	list, err := iw.fetchResources(limit)
	if err != nil {
		return nil, err
	}
	_, err = SetResourceCandid(list)
	if err != nil {
		iw.GetLogger().Error(fmt.Sprintf("error in load inflow resources list %s", err.Error()))
	}
	return list, nil

}

// fetchResources reads the registered engine list from infra and remembers it on
// the wire, without touching the dispatch pool.
func (iw *InflowWire) fetchResources(limit int) ([]models.RegisteredInflow, error) {
	list, err := etc.SendHttpGet(context.Background(), map[string]string{"Authorization": iw.GetBearerToken()}, fmt.Sprintf("%s/inflow/resource?per_page=%d", iw.Infra, limit), models.InflowResourcesList{})
	if err != nil {
		return nil, err
	}
	iw.resources = list.Data.List
	return list.Data.List, nil
}

// ReloadResourcesIfEmpty re-reads infra's engine list and installs the reachable
// ones as the dispatch pool ONLY if the pool is still empty — the check and the
// swap happen under one lock, so a resource an operator added by hand in the
// meantime is kept (only the explicit ReloadResources is allowed to drop it).
// It reports whether the pool was filled, plus how many of the registered
// resources answered, so a caller can log or decide to try again. Unreachable
// resources are not logged one by one here: a caller on a retry loop would
// repeat the same lines every tick, and the startup ReloadResources already
// said which ones dropped and why.
//
// It is the building block for a retry policy the SDK does not own: on a host
// reboot every container starts at once and the engine is usually still
// registering when InitBackend's one-shot reload probes it, and it is the
// application that decides how long and how often to keep looking.
func (iw *InflowWire) ReloadResourcesIfEmpty(limit int) (filled bool, reachable, registered int, err error) {
	if HasLiveResources() {
		return false, 0, 0, nil
	}
	list, err := iw.fetchResources(limit)
	if err != nil {
		return false, 0, 0, err
	}
	live := filterLiveResources(toInflowResources(list), false)
	if len(live) == 0 {
		return false, 0, len(list), nil
	}
	return adoptIfPoolEmpty(live), len(live), len(list), nil
}
func (iw *InflowWire) init() error {
	cred, err := iw.getCred()
	if err != nil {
		return err
	}
	cred.ServerUrl = iw.GetInfraNatsUrl()
	_, err = natsHandler.NewInfraNats(cred, iw.logger)
	if err != nil {
		return err
	}

	return iw.connectAndListen()
}
func (iw *InflowWire) GetLogger() *slog.Logger {

	return iw.logger
}
func (iw *InflowWire) GetAccountByKey(key string) (*models.Account,error) {
	response, err := etc.SendHttpGet(context.Background(), map[string]string{"Authorization": iw.GetBearerToken()},
		fmt.Sprintf("%s/account/id/%s", iw.Infra, key),
		struct {
			Data  *models.Account `json:"data"`
			Error any             `json:"error"`
		}{},
	)
	if err != nil {
		return nil, err
	}
	if response.Data == nil || response.Error != nil {
		return nil, fmt.Errorf("given account not found or any internal error occurred")
	}

	return response.Data, nil
}
func (iw *InflowWire) connectAndListen() error {
	con, err := natsHandler.GetInfraNats()
	if err != nil {
		return err
	}

	_, err = con.Subscribe(iw.flowGetSvcTopic.ConvertToSubscribe(), func(msg *nats.Msg) {
		if iw.SvcImpl == nil {
			fmt.Printf("New Request Recieved On Subscription Channel : %s\n", iw.flowGetSvcTopic.ConvertToSubscribe())
			fmt.Printf("Subject : %s\n", msg.Subject)
			fmt.Printf("Data : %s\n", string(msg.Data))
			msg.Respond([]byte(`not implemented`))
			return
		}
		iw.SvcImpl.RetrieveFlow(msg)
	})
	if err != nil {
		return err
	}
	iw.logger.Info(fmt.Sprintf("Subscription Registered On  : %s\n", iw.flowGetSvcTopic.ConvertToSubscribe()))

	_, err = con.Subscribe(iw.contextGetSvcTopic.ConvertToSubscribe(), func(msg *nats.Msg) {
		if iw.SvcImpl == nil {
			fmt.Printf("New Request Recieved On Subscription Channel : %s\n", iw.contextGetSvcTopic.ConvertToSubscribe())
			fmt.Printf("Subject : %s\n", msg.Subject)
			fmt.Printf("Data : %s\n", string(msg.Data))
			msg.Respond([]byte(`not implemented`))
			return
		}
		iw.SvcImpl.RetrieveContext(msg)
	})

	if err != nil {
		return err
	}
	iw.logger.Info(fmt.Sprintf("Subscription Registered On  : %s\n", iw.contextGetSvcTopic.ConvertToSubscribe()))
	_, err = con.Subscribe(iw.contextSetSvcTopic.ConvertToSubscribe(), func(msg *nats.Msg) {
		if iw.SvcImpl == nil {
			fmt.Printf("New Request Recieved On Subscription Channel : %s\n", iw.contextSetSvcTopic.ConvertToSubscribe())
			fmt.Printf("Subject : %s\n", msg.Subject)
			fmt.Printf("Data : %s\n", string(msg.Data))
			msg.Respond([]byte(`not implemented`))
			return
		}
		iw.SvcImpl.UpdateContext(msg)

	})

	if err != nil {
		return err
	}
	iw.logger.Info(fmt.Sprintf("Subscription Registered On  : %s\n", iw.contextSetSvcTopic.ConvertToSubscribe()))

	return err
}


func (iw *InflowWire) GetInflowEventsPipe() (*nats.Conn, error) {
	return natsHandler.GetInfraNats()
}