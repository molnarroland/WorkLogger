package webview

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/molnarroland/WorkLogger/internal/core"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Session is a signed-in website in its own app window (the system webview, sharing the app's cookie store),
// sign in there once; fetching drives the same page, hidden unless it needs you.
//
// Results come back over the webview's message bridge (webkit.messageHandlers.external / chrome.webview), which
// Wails hands to RawMessageHandler for any page. Wails only runs ExecJS once a page has said
// "wails:runtime:ready", so the window first opens web-session.html (our asset) and then goes to the site.
type Session struct {
	App        func() *application.App
	Bridge     *Bridge             // where the page's answers come back
	Source     string              // the profile id: names the window and the activity lines
	Kind       core.ProfileKind    // what the profile connects to
	Name       string              // the profile's title, in messages
	Home       func() string       // the page that proves a sign-in, "" when not configured
	SignInURL  func() string       // where Sign in starts (the site\'s own login page); Home when nil
	SignedInAt func(u string) bool // whether a loaded page at u means signed in
	StateFile  string              // remembers the last sign-in check

	mu    sync.Mutex
	win   *application.WebviewWindow
	ready chan struct{}
	shown bool
}

// Google refuses sign-in in webviews it doesn't recognise; Safari's user agent is accepted.
const safariUA = "Version/26.0 Safari/605.1.15"

// sameWindowJS keeps the site in its window. WebKit on macOS and Linux can't open new windows for Wails pages, so
// "Sign in" buttons that open one (target="_blank", window.open, as on Google's product pages) did nothing.
// It runs after every page load; WebView2 on Windows opens such windows itself.
const sameWindowJS = `(() => {
  if (window.__wlSameWindow) return;
  window.__wlSameWindow = true;
  window.open = (u) => { if (u) location.assign(new URL(String(u), location.href).href); return window; };
  document.addEventListener('click', (e) => {
    const a = e.target instanceof Element && e.target.closest('a[target]');
    if (!a || !a.href || a.target === '_self' || a.target === '_top' || a.target === '_parent') return;
    e.preventDefault();
    location.assign(a.href);
  }, true);
  document.addEventListener('submit', (e) => { if (e.target instanceof HTMLFormElement) e.target.target = '_self'; }, true);
})();`

// Bridge carries the answers of scripts run in site windows back to the Session that ran them. Results arrive as
// "wl:<id>:<json>"; the id is random and a result is only taken from the window that ran the script, so another
// window's page can't answer for it.
type Bridge struct {
	mu      sync.Mutex
	waiters map[string]waiter
}

type waiter struct {
	window string
	ch     chan string
}

// NewBridge makes the bridge main.go hands to Wails as Options.RawMessageHandler (Bridge.RawMessage).
func NewBridge() *Bridge { return &Bridge{waiters: map[string]waiter{}} }

// RawMessage routes a message from a site window.
func (b *Bridge) RawMessage(window application.Window, message string, _ *application.OriginInfo) {
	if window != nil {
		b.deliver(window.Name(), message)
	}
}

func (b *Bridge) deliver(window, message string) {
	rest, ok := strings.CutPrefix(message, "wl:")
	if !ok {
		return
	}
	id, body, ok := strings.Cut(rest, ":")
	if !ok {
		return
	}
	b.mu.Lock()
	wt, ok := b.waiters[id]
	b.mu.Unlock()
	if ok && wt.window == window {
		select {
		case wt.ch <- body:
		default:
		}
	}
}

func (b *Bridge) wait(id, window string) chan string {
	ch := make(chan string, 1)
	b.mu.Lock()
	b.waiters[id] = waiter{window: window, ch: ch}
	b.mu.Unlock()
	return ch
}

func (b *Bridge) forget(id string) {
	b.mu.Lock()
	delete(b.waiters, id)
	b.mu.Unlock()
}

func randomID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b) // never fails since Go 1.24
	return hex.EncodeToString(b)
}

func (s *Session) SiteName() string { return s.Name }

// window makes the site's window (hidden) and waits until scripts can run in it.
func (s *Session) window(ctx context.Context) (*application.WebviewWindow, error) {
	s.mu.Lock()
	if s.win == nil {
		s.ready = make(chan struct{})
		ready := s.ready
		w := s.App().Window.NewWithOptions(application.WebviewWindowOptions{
			Name: SiteWindowPrefix + s.Source, Title: s.Name, Width: 1100, Height: 820, Hidden: true,
			URL: "/web-session.html",
			Mac: application.MacWindow{WebviewPreferences: application.MacWebviewPreferences{ApplicationNameForUserAgent: safariUA}},
		})
		w.OnWindowEvent(events.Common.WindowRuntimeReady, func(*application.WindowEvent) {
			select {
			case <-ready:
			default:
				close(ready)
			}
		})
		keep := func(*application.WindowEvent) { w.ExecJS(sameWindowJS) }
		w.OnWindowEvent(events.Mac.WebViewDidFinishNavigation, keep)
		w.OnWindowEvent(events.Linux.WindowLoadFinished, keep)
		// Closing only hides it: the page stays signed in and fetching can keep using it.
		w.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
			e.Cancel()
			w.Hide()
			s.mu.Lock()
			s.shown = false
			s.mu.Unlock()
		})
		s.win = w
	}
	w, ready := s.win, s.ready
	s.mu.Unlock()
	select {
	case <-ready:
		return w, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(20 * time.Second):
		return nil, core.Err("site.notStarted", "name", s.Name)
	}
}

// SignIn shows the site's login page.
func (s *Session) SignIn(ctx context.Context) error {
	u := s.Home()
	if s.SignInURL != nil {
		u = s.SignInURL()
	}
	if u == "" {
		return errors.New(s.Name + ": the address is not set up yet.")
	}
	return s.Show(ctx, u)
}

// Show brings the window up at url, for signing in.
func (s *Session) Show(ctx context.Context, u string) error {
	w, err := s.window(ctx)
	if err != nil {
		return err
	}
	w.SetURL(u)
	w.Show()
	w.Focus()
	s.mu.Lock()
	s.shown = true
	s.mu.Unlock()
	return nil
}

func (s *Session) Open(ctx context.Context, u string) error {
	w, err := s.window(ctx)
	if err != nil {
		return err
	}
	w.SetURL(u)
	return nil
}

// eval runs expr (a JS expression, may be a promise) and returns its JSON result. Gives up after timeout, e.g.
// when a navigation replaces the page before it answers.
func (s *Session) eval(ctx context.Context, expr string, timeout time.Duration) (json.RawMessage, error) {
	w, err := s.window(ctx)
	if err != nil {
		return nil, err
	}
	id := randomID()
	ch := s.Bridge.wait(id, w.Name())
	defer s.Bridge.forget(id)
	w.ExecJS(`(async () => {
  const post = m => (window.webkit?.messageHandlers?.external ?? window.chrome?.webview)?.postMessage(m);
  let r = null, e = null;
  try { r = await (` + expr + `); } catch (x) { e = String(x && x.message || x); }
  post(` + "`wl:" + id + ":`" + ` + JSON.stringify({ r: r === undefined ? null : r, e }));
})();`)
	select {
	case body := <-ch:
		var out struct {
			R json.RawMessage `json:"r"`
			E *string         `json:"e"`
		}
		if err := json.Unmarshal([]byte(body), &out); err != nil {
			return nil, err
		}
		if out.E != nil {
			return nil, errors.New(*out.E)
		}
		return out.R, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(timeout):
		return nil, errNoAnswer
	}
}

var errNoAnswer = errors.New("the page did not answer")

func (s *Session) evalString(ctx context.Context, expr string) string {
	raw, err := s.eval(ctx, expr, 3*time.Second)
	var v string
	if err == nil {
		_ = json.Unmarshal(raw, &v)
	}
	return v
}

func (s *Session) CurrentURL(ctx context.Context) string {
	return s.evalString(ctx, "location.href")
}

// WaitUntilLoaded waits for the current navigation to finish (the page may redirect a few times), up to 45 s.
func (s *Session) WaitUntilLoaded(ctx context.Context) error {
	deadline := time.Now().Add(45 * time.Second)
	complete := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
		// Two complete answers in a row: a redirect started by the first page would have replaced it by then.
		if s.evalString(ctx, "document.readyState") == "complete" && !strings.HasSuffix(s.CurrentURL(ctx), "/web-session.html") {
			if complete++; complete >= 2 {
				return nil
			}
		} else {
			complete = 0
		}
		if time.Now().After(deadline) {
			return core.Err("site.notLoaded", "name", s.Name)
		}
	}
}

// Run runs script as the body of an async function whose named parameters are args. While it runs,
// window.__wlProgress is reported every second; when a redirect replaces the page, it is run again (up to 4 times).
func (s *Session) Run(ctx context.Context, script string, args map[string]any, progress func(string)) (any, error) {
	names := make([]string, 0, len(args))
	for k := range args {
		names = append(names, k)
	}
	sort.Strings(names)
	data, _ := json.Marshal(args)
	if args == nil {
		data = []byte("{}")
	}
	for attempt := 1; ; attempt++ {
		run := randomID()
		expr := "(window.__wlRun = '" + run + "', (async ({" + strings.Join(names, ",") + "}) => {\n" + script + "\n})(" + string(data) + "))"
		type result struct {
			raw json.RawMessage
			err error
		}
		done := make(chan result, 1)
		attemptCtx, stop := context.WithCancel(ctx)
		go func() {
			raw, err := s.eval(attemptCtx, expr, 30*time.Minute)
			done <- result{raw, err}
		}()
		replaced := false
		for !replaced {
			select {
			case r := <-done:
				stop()
				if r.err != nil {
					return nil, r.err
				}
				var v any
				err := json.Unmarshal(r.raw, &v)
				return v, err
			case <-ctx.Done():
				stop()
				return nil, ctx.Err()
			case <-time.After(time.Second):
			}
			raw, err := s.eval(ctx, "({ run: window.__wlRun ?? null, p: window.__wlProgress ?? null })", 3*time.Second)
			var st struct {
				Run *string `json:"run"`
				P   *string `json:"p"`
			}
			if err == nil && json.Unmarshal(raw, &st) == nil {
				if st.Run == nil || *st.Run != run {
					replaced = true // a navigation replaced the page; the script is gone
				} else if progress != nil && st.P != nil {
					progress(*st.P)
				}
			}
		}
		stop() // the replaced page will never answer
		if attempt >= 4 {
			return nil, core.Err("site.reloading", "name", s.Name)
		}
		time.Sleep(3 * time.Second)
		if err := s.WaitUntilLoaded(ctx); err != nil {
			return nil, err
		}
	}
}

// IsSignedIn opens the site's home page (when the window isn't on the site already) and asks whether it shows
// the signed-in app; the answer is remembered for LastSignedIn.
func (s *Session) IsSignedIn(ctx context.Context) bool {
	home := s.Home()
	if home == "" {
		return false
	}
	cur := s.CurrentURL(ctx)
	if !s.SignedInAt(cur) && host(cur) != host(home) {
		if s.Open(ctx, home) != nil || s.WaitUntilLoaded(ctx) != nil {
			return false
		}
		cur = s.CurrentURL(ctx)
	}
	ok := s.SignedInAt(cur)
	s.remember(ok)
	return ok
}

func host(u string) string {
	p, err := url.Parse(u)
	if err != nil {
		return ""
	}
	return strings.ToLower(p.Hostname())
}

func (s *Session) remember(ok bool) {
	v := "0"
	if ok {
		v = "1"
	}
	_ = os.WriteFile(s.StateFile, []byte(v), 0o600) // only a hint for the next start; a failed write asks again
}

// LastSignedIn is the result of the last check, without loading anything; nil before the first one.
func (s *Session) LastSignedIn() *bool {
	data, err := os.ReadFile(s.StateFile)
	if err != nil {
		return nil
	}
	ok := strings.TrimSpace(string(data)) == "1"
	return &ok
}

// SignOut shows the site's sign-out page. Wails has no API for the webview's cookie store, so the site itself
// ends the session.
// Known limit: HRmaster's own Kijelentkezés button does the work there; add its logout URL when it's known.
func (s *Session) SignOut(ctx context.Context) error {
	u := s.Home()
	if s.Kind == core.ProfileGoogleChat {
		u = "https://accounts.google.com/Logout"
	}
	if u == "" {
		return nil
	}
	s.remember(false)
	return s.Show(ctx, u)
}
