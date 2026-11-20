// Package calendar reads meetings from Google Calendar (OAuth loopback + PKCE) and from ICS feeds.
package calendar

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/molnarroland/WorkLogger/internal/core"
)

var (
	ErrNotConfigured = core.Err("google.notConfigured")
	ErrNotSignedIn   = core.Err("google.notConnected")
	ErrTimedOut      = core.Err("google.timedOut")
)

const googleScope = "https://www.googleapis.com/auth/calendar.events.readonly"

// Google endpoints; tests point them at a fake server.
var (
	GoogleAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	GoogleTokenURL = "https://oauth2.googleapis.com/token"
	GoogleAPI      = "https://www.googleapis.com/calendar/v3"
)

// Google is OAuth 2.0 for installed apps: PKCE + loopback redirect. The refresh token lives in the keyring.
type Google struct {
	ClientID, ClientSecret string
	Secrets                core.SecretStore
	HTTP                   core.HTTPDoer
	Clock                  core.Clock
	OpenURL                func(string) error // the system browser

	mu          sync.Mutex
	accessToken string
	expiry      time.Time
}

func randomURLSafe(n int) string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~"
	b := make([]byte, n)
	_, _ = rand.Read(b) // never fails since Go 1.24
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return string(b)
}

// SignIn opens Google's consent page in the browser and waits (up to 5 minutes) for the redirect.
func (g *Google) SignIn(ctx context.Context) error {
	if g.ClientID == "" || g.ClientSecret == "" {
		return ErrNotConfigured
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer ln.Close()
	redirect := fmt.Sprintf("http://127.0.0.1:%d", ln.Addr().(*net.TCPAddr).Port)
	verifier := randomURLSafe(64)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	state := randomURLSafe(24)

	got := make(chan url.Values, 1)
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if !q.Has("code") && !q.Has("error") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<html><body style="font-family:system-ui,sans-serif;padding:48px"><h2>WorkLogger received the Google response.</h2><p>You can close this tab and return to the app.</p></body></html>`)
		select {
		case got <- q:
		default:
		}
	})}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	q := url.Values{
		"client_id": {g.ClientID}, "redirect_uri": {redirect}, "response_type": {"code"}, "scope": {googleScope},
		"code_challenge": {challenge}, "code_challenge_method": {"S256"}, "state": {state},
		"access_type": {"offline"}, "prompt": {"consent"},
	}
	if err := g.OpenURL(GoogleAuthURL + "?" + q.Encode()); err != nil {
		return err
	}
	var items url.Values
	select {
	case items = <-got:
	case <-time.After(5 * time.Minute):
		return ErrTimedOut
	case <-ctx.Done():
		return ctx.Err()
	}
	if e := items.Get("error"); e != "" {
		return core.Err("google.cancelled", "reason", e)
	}
	if items.Get("state") != state || items.Get("code") == "" {
		return errors.New("Google returned an unexpected response: state mismatch")
	}
	tok, err := g.tokenRequest(ctx, url.Values{
		"code": {items.Get("code")}, "client_id": {g.ClientID}, "client_secret": {g.ClientSecret},
		"redirect_uri": {redirect}, "grant_type": {"authorization_code"}, "code_verifier": {verifier},
	})
	if err != nil {
		return err
	}
	if tok.RefreshToken == "" {
		return errors.New("Google returned an unexpected response: no refresh token")
	}
	if err := g.Secrets.Set(core.SecretGoogleRefresh, tok.RefreshToken); err != nil {
		return err
	}
	g.remember(tok)
	return nil
}

func (g *Google) SignOut() error {
	g.mu.Lock()
	g.accessToken = ""
	g.mu.Unlock()
	return g.Secrets.Set(core.SecretGoogleRefresh, "")
}

func (g *Google) Connected() bool {
	_, ok := g.Secrets.Get(core.SecretGoogleRefresh)
	return ok
}

type token struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

func (g *Google) remember(t token) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.accessToken = t.AccessToken
	g.expiry = g.Clock.Now().Add(time.Duration(t.ExpiresIn-60) * time.Second)
}

func (g *Google) validAccessToken(ctx context.Context) (string, error) {
	g.mu.Lock()
	if g.accessToken != "" && g.expiry.After(g.Clock.Now()) {
		defer g.mu.Unlock()
		return g.accessToken, nil
	}
	g.mu.Unlock()
	refresh, ok := g.Secrets.Get(core.SecretGoogleRefresh)
	if !ok {
		return "", ErrNotSignedIn
	}
	tok, err := g.tokenRequest(ctx, url.Values{
		"refresh_token": {refresh}, "client_id": {g.ClientID}, "client_secret": {g.ClientSecret}, "grant_type": {"refresh_token"},
	})
	if err != nil {
		return "", err
	}
	g.remember(tok)
	return tok.AccessToken, nil
}

func (g *Google) tokenRequest(ctx context.Context, form url.Values) (token, error) {
	req, _ := http.NewRequestWithContext(ctx, "POST", GoogleTokenURL, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := g.HTTP.Do(req)
	if err != nil {
		return token{}, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return token{}, fmt.Errorf("Google returned an unexpected response: %s", data)
	}
	var t token
	return t, json.Unmarshal(data, &t)
}

// Meetings is the timed events in [from, to). Skips all-day events, cancelled events, events you declined, and
// out-of-office / focus-time / working-location blocks.
func (g *Google) Meetings(ctx context.Context, calendarID string, from, to time.Time) ([]core.Meeting, error) {
	tok, err := g.validAccessToken(ctx)
	if err != nil {
		return nil, err
	}
	out := []core.Meeting{}
	pageToken := ""
	for {
		q := url.Values{
			"timeMin": {from.UTC().Format(time.RFC3339)}, "timeMax": {to.UTC().Format(time.RFC3339)},
			"singleEvents": {"true"}, "orderBy": {"startTime"}, "maxResults": {"2500"},
		}
		if pageToken != "" {
			q.Set("pageToken", pageToken)
		}
		req, _ := http.NewRequestWithContext(ctx, "GET", GoogleAPI+"/calendars/"+url.PathEscape(calendarID)+"/events?"+q.Encode(), nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err := g.HTTP.Do(req)
		if err != nil {
			return nil, err
		}
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("Google returned an unexpected response: %s", data)
		}
		var page struct {
			Items []struct {
				ID, Summary, Description, Status, EventType string
				Start, End                                  *struct {
					DateTime *time.Time `json:"dateTime"`
				}
				Attendees []struct {
					Self           bool   `json:"self"`
					ResponseStatus string `json:"responseStatus"`
				}
			} `json:"items"`
			NextPageToken string `json:"nextPageToken"`
		}
		if err := json.Unmarshal(data, &page); err != nil {
			return nil, err
		}
		for _, e := range page.Items {
			if e.Status == "cancelled" || e.Start == nil || e.End == nil || e.Start.DateTime == nil || e.End.DateTime == nil {
				continue
			}
			switch e.EventType {
			case "workingLocation", "outOfOffice", "focusTime", "birthday":
				continue
			}
			declined := false
			for _, a := range e.Attendees {
				if a.Self {
					declined = a.ResponseStatus == "declined"
					break
				}
			}
			if declined {
				continue
			}
			title := e.Summary
			if title == "" {
				title = "(no title)"
			}
			out = append(out, core.Meeting{
				ID: e.ID, CalendarID: calendarID, Title: title, Details: e.Description,
				Start: *e.Start.DateTime, End: *e.End.DateTime,
			})
		}
		if page.NextPageToken == "" {
			return out, nil
		}
		pageToken = page.NextPageToken
	}
}
