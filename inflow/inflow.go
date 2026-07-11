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
func (iw *InflowWire) GetResourceBearerToken(url string) string {
	for _,r:=range iw.resources{
		if r.Url ==  url{
			if r.RegisterPortal.JwtSecret == ""{
				break // to return infra bearer token
			}
			return makeTokenWithHs256(r.RegisterPortal.JwtSecret)
		}
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

	list, err := etc.SendHttpGet(context.Background(), map[string]string{"Authorization": iw.GetBearerToken()}, fmt.Sprintf("%s/inflow/resource?per_page=%d", iw.Infra, limit), models.InflowResourcesList{})
	if err != nil {
		return nil, err
	}
	iw.resources = list.Data.List
	_, err = SetResourceCandid(iw.resources)
	if err != nil {
		iw.GetLogger().Error(fmt.Sprintf("error in load inflow resources list %s", err.Error()))
	}
	return list.Data.List, nil

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