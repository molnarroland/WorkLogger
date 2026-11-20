package app

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/molnarroland/WorkLogger/internal/core"
)

// The activity console: everything the background did this session (job progress and results, git commands,
// HTTP requests, signed-in pages), masked, in memory only.

// EventActivity carries one ActivityLine as it is logged.
const EventActivity = "activity:line"

// ActivityLine is one line of the console. A progress line with Replace set takes the place of the line before
// it (the same job's previous progress), so a running job is one live line instead of hundreds.
type ActivityLine struct {
	Time    string `json:"time"`
	Source  string `json:"source"`
	Level   string `json:"level"` // "info", "progress", "error"
	Message string `json:"message"`
	Replace bool   `json:"replace"`
}

// Known limit: keeps the last 2000 lines of the session; write to a file if a log must outlive the app.
const activityLimit = 2000

// Activity is this session's console lines, oldest first.
func (s *Service) Activity() []ActivityLine {
	s.actMu.Lock()
	defer s.actMu.Unlock()
	return append([]ActivityLine{}, s.activity...)
}

// logFailure puts an error the app works around (a side step that failed) in the activity console, instead of
// dropping it.
func (s *Service) logFailure(source, what string, err error) {
	if err != nil {
		s.logActivity(source, "error", what+": "+err.Error())
	}
}

func (s *Service) logActivity(source, level, message string) {
	l := ActivityLine{Time: rfc(s.Clock.Now()), Source: source, Level: level, Message: MaskSecrets(message)}
	s.actMu.Lock()
	if n := len(s.activity); level == "progress" && n > 0 && s.activity[n-1].Level == "progress" && s.activity[n-1].Source == source {
		l.Replace = true
		s.activity[n-1] = l
	} else {
		s.activity = append(s.activity, l)
		if len(s.activity) > activityLimit {
			s.activity = s.activity[len(s.activity)-activityLimit:]
		}
	}
	s.actMu.Unlock()
	s.emit(EventActivity, l)
}

var (
	// key=value in URLs and command lines whose value is a credential.
	secretParamRe = regexp.MustCompile(`(?i)\b(token|access_token|refresh_token|api[_-]?key|key|at|f\.sid|sid|password|passwd|pass|secret|client_secret|code|auth|authorization|session|signature|sig|xsrf)=([^&\s"']+)`)
	bearerRe      = regexp.MustCompile(`(?i)\b(bearer|basic|token)\s+[A-Za-z0-9._~+/=-]{8,}`)
	emailRe       = regexp.MustCompile(`([A-Za-z0-9._%+-])[A-Za-z0-9._%+-]*@([A-Za-z0-9.-]+\.[A-Za-z]{2,})`)
	// Long opaque strings (tokens, cookies, keys); paths and words have / . or spaces and stay readable.
	opaqueRe = regexp.MustCompile(`[A-Za-z0-9_+=-]{32,}`)
)

// MaskSecrets hides credentials and shortens email addresses (r•••@x.hu) in a console line.
func MaskSecrets(s string) string {
	s = secretParamRe.ReplaceAllString(s, "$1=•••")
	s = bearerRe.ReplaceAllString(s, "$1 •••")
	s = emailRe.ReplaceAllString(s, "$1•••@$2")
	return opaqueRe.ReplaceAllStringFunc(s, func(t string) string { return t[:4] + "•••" })
}

// loggedRunner logs each command line (git) with its time and outcome.
type loggedRunner struct {
	next core.CommandRunner
	s    *Service
}

func (r loggedRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	start := r.s.Clock.Now()
	out, err := r.next.Run(ctx, name, args...)
	line := name + " " + strings.Join(args, " ")
	if err != nil {
		r.s.logActivity(name, "error", fmt.Sprintf("%s → %v (%s)", line, err, r.s.since(start)))
	} else {
		r.s.logActivity(name, "info", fmt.Sprintf("%s → %d bytes (%s)", line, len(out), r.s.since(start)))
	}
	return out, err
}

// loggedHTTP logs each request (Tempo, Jira, calendar feeds) with its status; never headers or bodies.
type loggedHTTP struct {
	next core.HTTPDoer
	s    *Service
}

func (h loggedHTTP) Do(req *http.Request) (*http.Response, error) {
	start := h.s.Clock.Now()
	res, err := h.next.Do(req)
	line := req.Method + " " + req.URL.Redacted()
	if err != nil {
		h.s.logActivity("http", "error", fmt.Sprintf("%s → %v (%s)", line, err, h.s.since(start)))
	} else {
		level := "info"
		if res.StatusCode >= 400 {
			level = "error"
		}
		h.s.logActivity("http", level, fmt.Sprintf("%s → %d (%s)", line, res.StatusCode, h.s.since(start)))
	}
	return res, err
}

// loggedSite logs the pages a signed-in site opens and the scripts that fail there.
type loggedSite struct {
	Site
	s      *Service
	source string
}

func (l loggedSite) Open(ctx context.Context, url string) error {
	l.s.logActivity(l.source, "info", "open "+url)
	err := l.Site.Open(ctx, url)
	if err != nil {
		l.s.logActivity(l.source, "error", "open "+url+" → "+err.Error())
	}
	return err
}

func (l loggedSite) Run(ctx context.Context, script string, args map[string]any, progress func(string)) (any, error) {
	start := l.s.Clock.Now()
	v, err := l.Site.Run(ctx, script, args, progress)
	if err != nil {
		l.s.logActivity(l.source, "error", fmt.Sprintf("page script failed after %s: %v", l.s.since(start), err))
	} else {
		l.s.logActivity(l.source, "info", fmt.Sprintf("page script done (%s)", l.s.since(start)))
	}
	return v, err
}

func (s *Service) since(t time.Time) string {
	return s.Clock.Now().Sub(t).Round(time.Millisecond).String()
}
