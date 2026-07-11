package models

import "github.com/nats-io/jwt/v2"

const (
	DEFAULT_ISOLATED_INFRA = "inflow_infra"
)

type Cred struct {
	Base64Cred string `json:"cred"`
	Raw        string `json:"raw"`
	Jwt        string `json:"jwt"`
	USeed      string `json:"user_seed"`
	UPub       string `json:"user_pub"`
	ServerUrl  string `json:"server"`
}
type UserCredGenInput struct {
	Name    string         `json:"name"`
	Account string         `json:"account"`
	Pub     jwt.Permission `json:"pub"`
	Sub     jwt.Permission `json:"sub"`
	Tags    jwt.TagList    `json:"tags"`
}
type RegisterationResponse struct {
	Cred       `json:",inline"`
	ApiSecret  string `json:"secret"`
	SubsPrefix string `json:"subscribe_prefix"`
}

type RegisterRequestBody struct {
	Name string   `json:"name"`
	Url  string   `json:"url"`
	Tags []string `json:"tags"`
}

type CredApiResponse struct {
	Data  Cred `json:"data"`
	Error any  `json:"error"`
}

type RegisteredInflow struct {
	ID                  string `json:"id"`
	RegisterRequestBody `json:",inline"`
	RegisterPortal      Portal `json:"portal"`
	CreatedAt           int64  `json:"createdAt"`
	Last                int64  `json:"last_login"`
	Count               uint64 `json:"count"`
}

type Portal struct {
	Title      string         `json:"title"`
	SubsPrefix string         `json:"subscribe_prefix"`
	Path       string         `json:"path"`
	JwtSecret  string         `json:"jwt_secret"`
	Config     map[string]any `json:"config"`
}

type InflowResourcesList struct {
	Data struct {
		List []RegisteredInflow `json:"list"`
	} `json:"data"`
	Error any `json:"error"`
}
