package app

import (
	"strings"
	"testing"

	"github.com/molnarroland/WorkLogger/internal/core"
)

func TestMaskSecrets(t *testing.T) {
	for in, want := range map[string]string{
		"GET https://x.hu/api?key=AIzaSyB0RaagJhe9&q=1 → 200":                "GET https://x.hu/api?key=•••&q=1 → 200",
		"POST /batchexecute?rpcids=SBNmJb&f.sid=-113131707&at=AJpMio:1794":   "POST /batchexecute?rpcids=SBNmJb&f.sid=•••&at=•••",
		"Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.e30.abc":                 "Authorization: Bearer •••",
		"git -C /Users/me/Workspaces/billing-api log --author=jane.d@x.hu": "git -C /Users/me/Workspaces/billing-api log --author=j•••@x.hu",
		"cookie TISCh-BGRN6On_JVS8nwsJxLY19N4ZmCkkdC4dKxw0x4IVvxhZ":          "cookie TISC•••",
		"Google Chat · Tue 3 Mar (2 of 10) · my messages · 12 read…":         "Google Chat · Tue 3 Mar (2 of 10) · my messages · 12 read…",
	} {
		if got := MaskSecrets(in); got != want {
			t.Errorf("MaskSecrets(%q) = %q, want %q", in, got, want)
		}
	}
}

// Progress of a running job is one live line; other lines are kept.
func TestActivityProgressIsOneLine(t *testing.T) {
	s, _ := newService(t)
	s.progress("git", core.Message{Text: "Reading 1 of 3…"})
	s.progress("git", core.Message{Text: "Reading 2 of 3…"})
	s.logActivity("git", "info", "git log → 10 bytes")
	s.progress("git", core.Message{Text: "Reading 3 of 3…"})
	_ = s.finish(t.Context(), core.Message{Text: "Collected 3 commits."}, nil)
	var got []string
	for _, l := range s.Activity() {
		got = append(got, l.Level+": "+l.Message)
	}
	want := "progress: Reading 2 of 3…|info: git log → 10 bytes|progress: Reading 3 of 3…|info: Collected 3 commits."
	if strings.Join(got, "|") != want {
		t.Errorf("got %v", got)
	}
}
