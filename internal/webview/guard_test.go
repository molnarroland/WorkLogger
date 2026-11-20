package webview

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// RELEASE_REVIEW A11: a page in a site window, or any foreign page, can't reach the app's runtime.
func TestGuardRuntime(t *testing.T) {
	served := false
	h := GuardRuntime(nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { served = true }))
	call := func(method, path string, headers map[string]string) int {
		served = false
		r := httptest.NewRequest(method, path, strings.NewReader(`{"object":0,"method":1}`))
		for k, v := range headers {
			r.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code == http.StatusOK && !served {
			t.Fatalf("%s %s: OK without reaching the handler", method, path)
		}
		return w.Code
	}
	cases := []struct {
		name    string
		method  string
		path    string
		headers map[string]string
		want    int
	}{
		{"the main window on macOS and Linux", "POST", "/wails/runtime", map[string]string{headerWindowName: "", "Origin": "wails://localhost"}, 200},
		{"the main window on Windows", "POST", "/wails/runtime", map[string]string{"Origin": "http://wails.localhost"}, 200},
		{"the main window under wails3 dev", "POST", "/wails/runtime", map[string]string{"Origin": "wails://localhost:9245"}, 200},
		{"a same-origin request without Origin", "POST", "/wails/runtime", map[string]string{"Sec-Fetch-Site": "same-origin"}, 200},
		{"Google's page in a site window", "POST", "/wails/runtime", map[string]string{headerWindowName: "web-googleChat", "Origin": "https://chat.google.com"}, 403},
		{"a site window, even with the app's origin", "POST", "/wails/runtime", map[string]string{headerWindowName: "web-hrmaster", "Origin": "wails://localhost"}, 403},
		{"a site window polling the event stream", "GET", "/wails/stream/poll", map[string]string{headerWindowName: "web-x"}, 403},
		{"a site window fetching an event payload", "GET", "/wails/event/0123456789abcdef", map[string]string{headerWindowName: "web-x"}, 403},
		{"a foreign origin in any window", "POST", "/wails/runtime", map[string]string{"Origin": "https://evil.example"}, 403},
		{"a lookalike host", "POST", "/wails/runtime", map[string]string{"Origin": "https://localhost.evil.example"}, 403},
		{"a cross-site request without Origin", "POST", "/wails/runtime", map[string]string{"Sec-Fetch-Site": "cross-site"}, 403},
		{"a garbled origin", "POST", "/wails/runtime", map[string]string{"Origin": "::"}, 403},
		{"a site window loading its own start page", "GET", "/web-session.html", map[string]string{headerWindowName: "web-googleChat"}, 200},
		{"the app's assets", "GET", "/index.html", nil, 200},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := call(c.method, c.path, c.headers); got != c.want {
				t.Errorf("got %d, want %d", got, c.want)
			}
		})
	}
}

// A script result is only taken from the window that is waiting for it.
func TestBridgeOnlyFromTheWaitingWindow(t *testing.T) {
	b := NewBridge()
	ch := b.wait("abc", "web-hrmaster")
	defer b.forget("abc")

	b.deliver("web-googleChat", `wl:abc:{"r":"forged"}`)
	select {
	case got := <-ch:
		t.Fatalf("took %q from another window", got)
	default:
	}
	b.deliver("web-hrmaster", `wl:abc:{"r":1}`)
	if got := <-ch; got != `{"r":1}` {
		t.Errorf("got %q", got)
	}
}
