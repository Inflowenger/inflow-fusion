package inflow

import (
	"sync"
	"time"

	"github.com/Inflowenger/inflow-fusion/models"
	natsHandler "github.com/Inflowenger/inflow-fusion/nats"
	"github.com/bytedance/sonic"
	"github.com/nats-io/nats.go"
)

// resourceSignalWindow coalesces a burst of signals into one reload.
//
// A burst is the normal case, not the exception: on a host reboot every fractal
// container registers with infra at roughly the same moment, and each
// registration signals. Reloading once per signal would run the liveness probe
// over the whole pool N times in a row — and the earliest of those reloads would
// see a half-registered cluster. Waiting a moment and reloading once sees the
// settled list.
const resourceSignalWindow = 2 * time.Second

// SubscribeResourceSignals listens for infra's resource-reload announcements on
// the builtin inflow account and re-reads the engine list when one arrives.
//
// This is what makes a license lockout and a new engine take effect at once
// rather than at the next restart. The signal itself carries no engine data: the
// reload re-reads GET /inflow/resource, which is also where infra enforces the
// license, so a locked license serves an empty list and the dispatch pool
// empties through the ordinary reload path. One mechanism, two outcomes.
//
// It must be called after InitBackend has wired the backend — it needs the infra
// NATS connection that init() establishes, and a reload needs the instance to be
// published so the liveness probe can reach the bearer token. InitBackend calls
// it for you; it is exported for a backend that drives the wiring itself.
//
// Note for operators: a signalled reload replaces the dispatch pool from infra's
// list, exactly as an explicit ReloadResources does. A resource added by hand
// with AddResource does not survive it. Tag the resource on infra instead if it
// has to persist.
func (iw *InflowWire) SubscribeResourceSignals() error {
	con, err := natsHandler.GetInfraNats()
	if err != nil {
		return err
	}
	// Buffered: a burst must not block the NATS callback, and once the buffer is
	// full every further signal is redundant anyway — a reload is already coming
	// and it will read the latest list.
	trigger := make(chan models.ResourceSignal, 64)
	go iw.resourceReloadLoop(trigger)

	_, err = con.Subscribe(models.ResourceSignalSubject, func(msg *nats.Msg) {
		sig := models.ResourceSignal{}
		if err := sonic.Unmarshal(msg.Data, &sig); err != nil {
			iw.GetLogger().Warn("resource signal: unreadable payload", "subject", msg.Subject, "error", err.Error())
			return
		}
		select {
		case trigger <- sig:
		default:
			iw.GetLogger().Debug("resource signal dropped: a reload is already queued", "reason", sig.Reason)
		}
	})
	if err != nil {
		return err
	}
	iw.GetLogger().Info("listening for infra resource signals", "subject", models.ResourceSignalSubject)
	return nil
}

// resourceReloadLoop turns a stream of signals into at most one reload per
// window.
func (iw *InflowWire) resourceReloadLoop(trigger <-chan models.ResourceSignal) {
	for sig := range trigger {
		last, coalesced := coalesceSignals(trigger, sig, resourceSignalWindow)
		iw.applyResourceSignal(last, coalesced)
	}
}

// coalesceSignals absorbs every signal that arrives within window of first and
// returns the last one, plus how many were folded together.
//
// The window opens on the first signal and is deliberately NOT extended by the
// ones that follow. A sliding window would be the obvious choice and is the
// wrong one here: a cluster that registers an engine every second would keep
// pushing the deadline out and never reload at all. A fixed window converges
// within a bounded time however busy the cluster is.
func coalesceSignals(trigger <-chan models.ResourceSignal, first models.ResourceSignal, window time.Duration) (models.ResourceSignal, int) {
	last, count := first, 1
	deadline := time.After(window)
	for {
		select {
		case next := <-trigger:
			last, count = next, count+1
		case <-deadline:
			return last, count
		}
	}
}

func (iw *InflowWire) applyResourceSignal(sig models.ResourceSignal, coalesced int) {
	log := iw.GetLogger()
	list, err := iw.ReloadResources(iw.GetResourceLimit())
	if err != nil {
		log.Error("resource signal: reload failed", "reason", sig.Reason, "error", err.Error())
		return
	}
	live := len(GetResourceCandidList())

	// A locked license is not an error and not a misconfiguration, so say what it
	// is: the credit for the period is spent and infra is holding the engines
	// back. Read the reason off the served list rather than the signal, which may
	// be stale by the time the reload lands.
	if gate := iw.LicenseGate(); gate != nil && gate.Locked {
		log.Warn("process engines are inactive: license credit exhausted",
			"reason", gate.Reason,
			"consumed", gate.Consumed,
			"budget", gate.Budget,
			"products", gate.Products,
			"upgrade", gate.Upgrade)
		return
	}
	log.Info("resource pool reloaded on infra signal",
		"reason", sig.Reason,
		"signals_coalesced", coalesced,
		"registered", len(list),
		"dispatchable", live)
}

// licenseMu guards the gate last served with the resource list. The reload runs
// on the signal goroutine while dispatch and health checks read the gate, so the
// two must not race.
var licenseMu sync.RWMutex

// rememberLicenseGate stores the gate that came with the engine list.
func (iw *InflowWire) rememberLicenseGate(gate *models.LicenseGate) {
	licenseMu.Lock()
	defer licenseMu.Unlock()
	iw.license = gate
}

// LicenseGate returns infra's license gate as of the last resource read, or nil
// if no read has carried one (an older infra, or none yet). A caller that wants
// to explain to a user why a process cannot start should read it from here
// rather than calling infra again.
func (iw *InflowWire) LicenseGate() *models.LicenseGate {
	licenseMu.RLock()
	defer licenseMu.RUnlock()
	if iw.license == nil {
		return nil
	}
	gate := *iw.license
	return &gate
}

// LicenseLocked reports whether infra is currently refusing to serve process
// engines because the license credit is spent. It is false when the gate is
// unknown: an infra that does not report one is not a locked one.
func (iw *InflowWire) LicenseLocked() bool {
	gate := iw.LicenseGate()
	return gate != nil && gate.Locked
}
