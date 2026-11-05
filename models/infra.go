package models

import (
	"strconv"

	"github.com/nats-io/jwt/v2"
)

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

// ResourceWeight is whether infra is currently serving a registered process
// engine (fractal) as dispatchable, and how much work it should attract.
// Mirrors infra's models.ResourceWeight.
//
// Negative is out of service; zero and above are active, and the magnitude is a
// dispatch weight. Only the sign is honoured here today — the pool is still an
// even round-robin — so a weight above zero behaves exactly like the default
// until weighted selection lands.
//
// The zero value is active, which is what makes the field safe to add: an infra
// that sends no weight at all, or one that predates the field, decodes to a
// working engine instead of a parked one.
type ResourceWeight int8

const (
	// ResourceWeightInactive is a parked engine: infra still holds the
	// registration, but no new work may go there. Any lower value is inactive
	// too — infra uses the sign, not this exact constant.
	ResourceWeightInactive ResourceWeight = -1
	// ResourceWeightDefault is the zero value: active, no weighting preference.
	ResourceWeightDefault ResourceWeight = 0
)

// Active reports whether work may be dispatched to a resource at this weight.
// Ask this rather than comparing against a constant — every negative value is
// out of service.
func (w ResourceWeight) Active() bool { return w >= ResourceWeightDefault }

// Share is the relative pull a balancer should give this resource: 0 when
// inactive, otherwise the weight plus one, so an unweighted cluster stays an
// even round-robin and a weight of n means n+1 times a default resource's share.
func (w ResourceWeight) Share() int {
	if !w.Active() {
		return 0
	}
	return int(w) + 1
}

func (w ResourceWeight) String() string {
	if !w.Active() {
		return "inactive"
	}
	if w == ResourceWeightDefault {
		return "active"
	}
	return "active weight " + strconv.Itoa(int(w))
}

type RegisteredInflow struct {
	ID                  string `json:"id"`
	RegisterRequestBody `json:",inline"`
	RegisterPortal      Portal `json:"portal"`
	CreatedAt           int64  `json:"createdAt"`
	Last                int64  `json:"last_login"`
	Count               uint64 `json:"count"`
	// Weight is what infra serves for this engine right now: an operator's
	// stored weight, or inactive while infra's license gate is closed.
	Weight ResourceWeight `json:"weight"`
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
		// List is every registered engine, each carrying its own Weight. A
		// locked license does not shorten this list — it serves every row at a
		// negative weight — so anything building a dispatch pool from it has to
		// check Weight.Active().
		List []RegisteredInflow `json:"list"`
		// License is infra's gate at the moment the list was served. Read it to
		// explain a cluster that has engines but accepts no work, instead of
		// reporting "no engines registered".
		License *LicenseGate `json:"license,omitempty"`
	} `json:"data"`
	Error any `json:"error"`
}

// ResourceSignalSubject is where infra announces that the registered process
// engine (fractal) list has changed and must be re-read. It lives on the builtin
// inflow account — the same account this backend already connects with — beside
// `_infra.trace.>`. Mirrors models.ResourceSignalSubject in infra.
const ResourceSignalSubject = "_infra.signal.resources"

// Reasons infra signals a resource reload. Mirrors infra's constants.
const (
	ResourceSignalBoot            = "boot"
	ResourceSignalRegistered      = "registered"
	ResourceSignalUpdated         = "updated"
	ResourceSignalRemoved         = "removed"
	ResourceSignalLicenseLocked   = "license.locked"
	ResourceSignalLicenseUnlocked = "license.unlocked"
)

// ResourceSignal is the payload on ResourceSignalSubject.
//
// It deliberately carries no resource data: it says "what you hold is stale",
// and the reload re-reads GET /inflow/resource. That endpoint is also where
// infra enforces the license, so a locked license simply serves an empty list —
// which means "reload" and "take every engine out of service" are one code path
// here rather than two that could disagree.
type ResourceSignal struct {
	Reason string `json:"reason"`
	// Locked mirrors infra's license gate when the signal was sent. It is for
	// logging only; the served resource list is the authority.
	Locked    bool         `json:"locked"`
	Resources int          `json:"resources"`
	License   *LicenseGate `json:"license,omitempty"`
	Cluster   string       `json:"cluster"`
	At        int64        `json:"at"`
}

// LicenseGate is infra's answer to "may process engines be used right now",
// served on GET /inflow/resource and GET /license/gate and carried on a
// ResourceSignal. Mirrors models.LicenseGate in infra.
type LicenseGate struct {
	Locked   bool     `json:"locked"`
	Metered  bool     `json:"metered"`
	Code     string   `json:"code,omitempty"`
	Reason   string   `json:"reason,omitempty"`
	Products []string `json:"products,omitempty"`
	Action   string   `json:"action,omitempty"`
	Budget   int64    `json:"budget"`
	Credit   int64    `json:"credit"`
	Consumed int64    `json:"consumed"`
	Percent  int      `json:"percent"`
	Expire   int64    `json:"expire,omitempty"`
	Upgrade  string   `json:"upgrade,omitempty"`
}
