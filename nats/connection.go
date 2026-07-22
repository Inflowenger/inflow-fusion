package nats

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Inflowenger/inflow-fusion/models"
	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats.go"
)

type natsConSpec struct {
	inboxPrefix string
	account     string
}
type Nats struct {
	cred        models.Cred
	topicPrefix string
	con         *nats.Conn
	spec        natsConSpec
	logger      *slog.Logger
}

func New(cred models.Cred) (*Nats, error) {
	n := Nats{cred: cred, spec: natsConSpec{inboxPrefix: NATS_DEFAULT_INBOX}}
	if err := n.extractToken(); err != nil {
		return nil, err
	}
	err := n.Connect()
	if err != nil {
		return nil, err
	}
	if n.logger == nil {
		n.logger = slog.Default()
	}
	return &n, nil
}

func (n *Nats) extractToken() error {
	var credBytes []byte
	var err error
	if len(n.cred.Raw) > 0 {
		credBytes = []byte(n.cred.Raw)
	} else {
		credBytes, err = base64.StdEncoding.DecodeString(n.cred.Base64Cred)
		if err != nil {
			return err
		}
		n.cred.Raw = string(credBytes)
	}
	token, err := jwt.ParseDecoratedJWT(credBytes)
	if err != nil {
		return err
	}
	userClaim, err := jwt.DecodeUserClaims(token)
	if err != nil {
		return err
	}
	n.spec.account = userClaim.Issuer
	n.spec.inboxPrefix = GetInboxPrefix(userClaim)
	return nil
}
func (n *Nats) Connect() error {
	var err error
	n.con, err = nats.Connect(fmt.Sprintf("nats://%s", n.cred.ServerUrl),
		nats.RetryOnFailedConnect(true),
		nats.CustomInboxPrefix(n.spec.inboxPrefix),
		nats.UserCredentialBytes([]byte(n.cred.Raw)),
		nats.PingInterval(30*time.Second),
		nats.ReconnectHandler(func(c *nats.Conn) {
			// Reconnect logic
			n.logger.Info(fmt.Sprintf("Reconnected to NATS server: %s", c.ConnectedUrl()))
		}), nats.ReconnectErrHandler(func(c *nats.Conn, err error) {
			n.logger.Error(fmt.Sprintf("Reconnection error: %v", err))
		}), nats.DisconnectErrHandler(func(c *nats.Conn, err error) {
			n.logger.Error(fmt.Sprintf("Disconnected from NATS server: %v", err))
		}), nats.ClosedHandler(func(c *nats.Conn) {
			n.logger.Info("Connection to NATS server closed")
		}))
	if err != nil {
		return err
	}
	return nil
}

func (n *Nats) GetConnection() *nats.Conn {
	if n.con == nil {
		n.Connect()
	}
	if n.con.IsClosed() {
		n.Connect()
	}
	return n.con
}
func (n *Nats) SetTopicPrefix(prefix string) {
	n.topicPrefix = prefix
}
func GetNatsByInfraIsolate(infraIsolate models.InfraIsolated) (*Nats, error) {
	account := infraIsolate.Account
	nat, ok := GetNatsBox().Read(account)
	if ok {
		return nat, nil
	}
	credBytes, err := base64.StdEncoding.DecodeString(infraIsolate.Cred)
	if err != nil {
		return nil, err
	}
	nat, err = New(models.Cred{Base64Cred: infraIsolate.Cred, ServerUrl: infraIsolate.Url, Raw: string(credBytes)})
	if err != nil {
		return nil, errors.New("make nats connection get failed")
	}
	GetNatsBox().write(account, nat)
	return nat, nil
}

func NewInfraNats(cred models.Cred, logger *slog.Logger) (*nats.Conn, error) {

	nats, err := New(cred)
	if err != nil {
		return nil, err
	}
	nats.logger = logger
	nats.logger.Info(fmt.Sprintf("new connection established , with profile :%s", models.DEFAULT_ISOLATED_INFRA))
	if nats.con.IsConnected() {
		GetNatsBox().write(models.DEFAULT_ISOLATED_INFRA, nats)
		return nats.con, nil
	}
	return nats.con, err
}

func GetInfraNats() (*nats.Conn, error) {

	if infranat := GetNatsBox().GetConnection(models.DEFAULT_ISOLATED_INFRA); infranat != nil {
		return infranat, nil
	}
	return nil, errors.New("infra connection not found , firstly need to make it")
}
