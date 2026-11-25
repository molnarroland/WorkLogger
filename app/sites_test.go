package app

import (
	"testing"

	"github.com/molnarroland/WorkLogger/internal/core"

	ts "github.com/molnarroland/WorkLogger/internal/testsupport"
)

// windowSite counts how often its window was let go.
type windowSite struct {
	fakeSite
	spec     SiteSpec
	released int
	closed   int
}

func (w *windowSite) Release() { w.released++ }
func (w *windowSite) Close()   { w.closed++ }

func newSiteService(t *testing.T) (*Service, map[string]*windowSite) {
	t.Helper()
	s, _ := newService(t)
	made := map[string]*windowSite{}
	s.StateDir = t.TempDir()
	s.NewSite = func(sp SiteSpec) Site {
		w := &windowSite{fakeSite: fakeSite{FakePage: &ts.FakePage{}}, spec: sp}
		made[sp.ID] = w
		return w
	}
	st := s.Settings()
	st.Profiles = []core.Profile{{ID: "g1", Kind: core.ProfileGoogleChat, Email: "me@x.hu"}}
	_, err := s.SaveSettings(st)
	must(t, err)
	return s, made
}

// A job's end lets go of the site windows it left behind; they'd otherwise run hidden until the app quits.
func TestJobEndReleasesSiteWindows(t *testing.T) {
	s, made := newSiteService(t)
	_, err := s.siteFor(s.Settings().Profiles[0])
	must(t, err)
	ctx, done := s.startJob()
	if ctx == nil {
		t.Fatal("no job")
	}
	if made["g1"].released != 0 {
		t.Fatal("released while the job runs")
	}
	done()
	if made["g1"].released != 1 {
		t.Fatalf("released %d times after the job, want 1", made["g1"].released)
	}
}

// Closing a site window by hand releases it when no job runs, and leaves it to the job's end when one does.
func TestHiddenSiteWindowWaitsForTheJob(t *testing.T) {
	s, made := newSiteService(t)
	_, err := s.siteFor(s.Settings().Profiles[0])
	must(t, err)
	w := made["g1"]
	if w.spec.Hidden == nil {
		t.Fatal("the window has no Hidden callback")
	}
	_, done := s.startJob()
	w.spec.Hidden()
	if w.released != 0 {
		t.Fatal("released while the job runs")
	}
	done()
	w.spec.Hidden()
	if w.released != 2 { // once at the job's end, once for the close after it
		t.Fatalf("released %d times, want 2", w.released)
	}
}

// Removing a profile closes its window and forgets it, so a profile made later gets a fresh one.
func TestRemoveProfileClosesItsWindow(t *testing.T) {
	s, made := newSiteService(t)
	_, err := s.siteFor(s.Settings().Profiles[0])
	must(t, err)
	old := made["g1"]
	_, err = s.RemoveProfile("g1")
	must(t, err)
	if old.closed != 1 {
		t.Fatalf("closed %d times, want 1", old.closed)
	}
	if _, ok := s.webSites["g1"]; ok {
		t.Fatal("the removed profile's window is still kept")
	}
}
