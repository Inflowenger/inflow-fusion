package inflow

import (
	"fmt"
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
	if u.Scheme==""{
		inflow.Infra = fmt.Sprintf("http://%s",inflow.Infra)
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
	// Published before ReloadResources so the liveness probe run there can reach
	// the backend — for its logger and the infra-bearer token fallback used by
	// resources whose portal carries no secret. init() has already finished, so
	// exposing the instance now is safe; the resource pool is simply still empty
	// until the reload fills it.
	inflowWireInstance = &inflow
	inflow.ReloadResources(inflow.GetResourceLimit())
	// The backend is wired and the pool is loaded: start listening for infra's
	// resource signals so a new engine, or a license lockout, takes effect
	// immediately instead of at the next restart. A failure here is not fatal —
	// without the subscription the backend still works, it just converges on its
	// own schedule rather than on infra's announcement.
	if serr := inflow.SubscribeResourceSignals(); serr != nil {
		inflow.GetLogger().Warn("infra resource signals unavailable; resource changes will not be picked up automatically", "error", serr.Error())
	}
	return err
}

// DefaultResourceLimit is the page size used when reading infra's engine list.
const DefaultResourceLimit = 100

// WithResourceLimit sets the page size for every engine-list read, including the
// reloads triggered by infra's resource signals.
func WithResourceLimit(limit int) func(*InflowWire) {
	return func(iw *InflowWire) {
		if limit > 0 {
			iw.resourceLimit = limit
		}
	}
}

func WithInfraApi(infraurl string) func(*InflowWire) {
	return func(iw *InflowWire) {
		iw.Infra = infraurl
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
