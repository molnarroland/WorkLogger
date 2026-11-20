package webview

import (
	"log"
	"net/http"
	"net/url"
	"strings"
)

// SiteWindowPrefix starts the window name of every profile's site window (Google, HRmaster). Those windows show
// pages from other origins, so they must never reach the app's runtime (docs/ARCHITECTURE.md, Trust boundaries).
const SiteWindowPrefix = "web-"

// Wails tags every request a webview makes with the window it came from. It sets these itself, overwriting
// anything the page sends, so a page can't pass for another window.
const (
	headerWindowName = "x-wails-window-name"
	headerWindowID   = "x-wails-window-id"
)

// GuardRuntime is the asset server's outermost middleware. Wails' desktop builds don't check where a runtime call
// comes from (only its server mode does), and a page can send the call as a plain POST that needs no CORS
// preflight. So any page in any app window could call the bound methods (SaveSettings, SetToken, PushToTempo…).
// This refuses every /wails/ request (runtime calls, the event stream, event payloads, the runtime script) from a
// site window, and from any page whose origin isn't the app's own.
// Refusals are logged to stderr and handed to report (the activity console), when given.
func GuardRuntime(report func(message string)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/wails/") && !runtimeAllowed(r) {
				msg := "Refused " + r.Method + " " + r.URL.Path + " from window " + quoteOr(r.Header.Get(headerWindowName), "?") +
					" (origin " + quoteOr(r.Header.Get("Origin"), "none") + ")"
				log.Print("worklogger: ", msg)
				if report != nil {
					report(msg)
				}
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func quoteOr(v, empty string) string {
	if v == "" {
		return empty
	}
	return `"` + v + `"`
}

func runtimeAllowed(r *http.Request) bool {
	if strings.HasPrefix(r.Header.Get(headerWindowName), SiteWindowPrefix) {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		// Browsers send Origin on every cross-origin POST; Sec-Fetch-Site covers any that don't.
		switch r.Header.Get("Sec-Fetch-Site") {
		case "cross-site", "same-site":
			return false
		}
		return true
	}
	return appOrigin(origin)
}

// appOrigin: the asset server's own origin, wails://localhost on macOS and Linux, http://wails.localhost on Windows
// (with a port in `wails3 dev`).
func appOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	switch u.Scheme {
	case "wails", "http", "https":
	default:
		return false
	}
	switch strings.ToLower(u.Hostname()) {
	case "localhost", "wails.localhost":
		return true
	}
	return false
}
