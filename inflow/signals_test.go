package inflow

import (
	"testing"
	"time"

	"github.com/Inflowenger/inflow-fusion/models"
)

func TestCoalesceSignalsFoldsABurstIntoOneReload(t *testing.T) {
	// What a host reboot looks like: every fractal registers at once.
	trigger := make(chan models.ResourceSignal, 16)
	for i := 0; i < 5; i++ {
		trigger <- models.ResourceSignal{Reason: models.ResourceSignalRegistered}
	}
	trigger <- models.ResourceSignal{Reason: models.ResourceSignalLicenseLocked, Locked: true}

	last, count := coalesceSignals(trigger, models.ResourceSignal{Reason: models.ResourceSignalBoot}, 30*time.Millisecond)

	if count != 7 {
		t.Errorf("coalesced %d signals, want 7 (the first plus the six queued)", count)
	}
	if last.Reason != models.ResourceSignalLicenseLocked || !last.Locked {
		t.Errorf("last = %+v, want the most recent signal to win", last)
	}
}

func TestCoalesceSignalsReturnsASingleSignalAfterTheWindow(t *testing.T) {
	trigger := make(chan models.ResourceSignal)
	start := time.Now()
	last, count := coalesceSignals(trigger, models.ResourceSignal{Reason: models.ResourceSignalBoot}, 40*time.Millisecond)

	if count != 1 || last.Reason != models.ResourceSignalBoot {
		t.Errorf("got (%+v, %d), want the lone boot signal", last, count)
	}
	if elapsed := time.Since(start); elapsed < 40*time.Millisecond {
		t.Errorf("returned after %s, want it to wait out the %s window so a burst can gather", elapsed, 40*time.Millisecond)
	}
}

// The window must not slide: a cluster that registers an engine every tick would
// otherwise push the deadline out forever and never reload at all.
func TestCoalesceSignalsWindowDoesNotSlide(t *testing.T) {
	trigger := make(chan models.ResourceSignal, 1)
	done := make(chan struct{})
	stop := make(chan struct{})
	go func() {
		defer close(done)
		coalesceSignals(trigger, models.ResourceSignal{Reason: models.ResourceSignalBoot}, 60*time.Millisecond)
	}()
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			select {
			case trigger <- models.ResourceSignal{Reason: models.ResourceSignalRegistered}:
			default:
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()
	defer close(stop)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("a continuous stream of signals starved the reload; the window is sliding")
	}
}

func TestLicenseGateIsNilUntilInfraReportsOne(t *testing.T) {
	iw := &InflowWire{}
	if iw.LicenseGate() != nil {
		t.Error("gate must be nil before any resource read")
	}
	// An infra that reports no gate is not a locked one — failing closed here
	// would take every engine out of service against an older infra.
	if iw.LicenseLocked() {
		t.Error("an unknown gate must not read as locked")
	}
}

func TestLicenseGateIsRememberedAndCopied(t *testing.T) {
	iw := &InflowWire{}
	iw.rememberLicenseGate(&models.LicenseGate{
		Locked: true, Metered: true, Reason: "trial credit for this period is finished",
		Budget: 2000, Credit: 0, Consumed: 2000, Percent: 100,
		Products: []string{"venapce"},
	})
	if !iw.LicenseLocked() {
		t.Fatal("a locked gate was not remembered")
	}
	gate := iw.LicenseGate()
	if gate.Budget != 2000 || gate.Percent != 100 || len(gate.Products) != 1 {
		t.Errorf("gate = %+v, want it preserved", gate)
	}
	// Callers get a copy: mutating what they were handed must not change the
	// state the dispatcher reads.
	gate.Locked = false
	if !iw.LicenseLocked() {
		t.Error("LicenseGate handed out a reference to internal state")
	}
}
