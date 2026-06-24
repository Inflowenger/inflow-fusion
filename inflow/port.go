package inflow

import (
	"log/slog"
	"net/url"
	"os"

	"github.com/Inflowenger/inflow-fusion/svcHandler"
	"github.com/golang-jwt/jwt/v5"
)

var inflowWireInstance *InflowWire

func InitBackend(opts ...func(*InflowWire)) error {

	inflow := InflowWire{natsport: 4222,
		Infra:              os.Getenv("INFLOW_INFRA_API"),
		flowGetSvcTopic:    svcHandler.DefaultGetFlowSvc,
		contextGetSvcTopic: svcHandler.DefaultGetContextSvc,
		contextSetSvcTopic: svcHandler.DefaultSetContextSvc,
	}
	for _, opt := range opts {
		opt(&inflow)
	}
	if inflow.token == "" {
		WithJwtSecretKey(os.Getenv("INFLOW_INFRA_JWT_SECRET"))(&inflow)
	}
	u, err := url.Parse(inflow.Infra)
	if err != nil {
		return err
	}
	if inflow.hs == "" {
		inflow.hs = u.Hostname()
	}
	if inflow.logger == nil {
		inflow.logger = slog.Default()
	}
	err = inflow.init()
	if err != nil {
		return err
	}
	inflow.ReloadResources(100)
	inflowWireInstance = &inflow
	return err
}

func WithInfraApi(url string) func(*InflowWire) {
	return func(iw *InflowWire) {
		iw.Infra = url
	}
}

func WithInfraHostname(hs string) func(*InflowWire) {
	return func(iw *InflowWire) {
		iw.hs = hs
	}
}

func WithNatsPort(port uint16) func(*InflowWire) {
	return func(iw *InflowWire) {
		iw.natsport = port
	}
}
func WithLogger(logger *slog.Logger) func(*InflowWire) {
	return func(iw *InflowWire) {
		iw.logger = logger
	}
}
func WithJwtSecretKey(seckey string) func(*InflowWire) {
	return func(iw *InflowWire) {
		token := jwt.New(jwt.SigningMethodHS256)
		token.Claims = jwt.MapClaims{"admin": true}
		encoded, _ := token.SignedString([]byte(seckey))
		iw.token = encoded
	}
}

func WithImplementedBackendBy(cls IInflowService) func(*InflowWire) {
	return func(iw *InflowWire) {
		iw.SvcImpl = cls
	}
}

func GetInflowBackend() *InflowWire {
	return inflowWireInstance
}
