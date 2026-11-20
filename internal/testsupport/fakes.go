package testsupport

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"sync"
	"time"
)

// InMemorySecrets is a SecretStore in a map.
type InMemorySecrets struct {
	mu sync.Mutex
	M  map[string]string
}

func NewSecrets(m map[string]string) *InMemorySecrets {
	if m == nil {
		m = map[string]string{}
	}
	return &InMemorySecrets{M: m}
}

func (s *InMemorySecrets) Get(k string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.M[k]
	return v, ok && v != ""
}

func (s *InMemorySecrets) Set(k, v string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v == "" {
		delete(s.M, k)
	} else {
		s.M[k] = v
	}
	return nil
}

// FakeHTTP records requests (with their bodies) and answers from a script.
type FakeHTTP struct {
	mu       sync.Mutex
	Handle   func(r *http.Request, body string) (int, string)
	Requests []*http.Request
	Bodies   []string
}

func (f *FakeHTTP) Do(r *http.Request) (*http.Response, error) {
	body := ""
	if r.Body != nil {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
	}
	f.mu.Lock()
	f.Requests = append(f.Requests, r)
	f.Bodies = append(f.Bodies, body)
	f.mu.Unlock()
	code, text := 200, "{}"
	if f.Handle != nil {
		code, text = f.Handle(r, body)
	}
	return &http.Response{StatusCode: code, Body: io.NopCloser(bytes.NewBufferString(text)), Header: http.Header{}, Request: r}, nil
}

type FixedClock struct{ T time.Time }

func (c FixedClock) Now() time.Time { return c.T }

// FakePage records what the collectors open and run, and answers scripts from Answer.
type FakePage struct {
	Signed   bool
	Redirect map[string]string
	Answer   func(url string, args map[string]any) any
	Progress []string
	Current  string
	Opened   []string
	Runs     []map[string]any
}

func (p *FakePage) SiteName() string                      { return "Fake site" }
func (p *FakePage) CurrentURL(context.Context) string     { return p.Current }
func (p *FakePage) WaitUntilLoaded(context.Context) error { return nil }
func (p *FakePage) IsSignedIn(context.Context) bool       { return p.Signed }

func (p *FakePage) Open(_ context.Context, url string) error {
	p.Opened = append(p.Opened, url)
	p.Current = url
	if r, ok := p.Redirect[url]; ok {
		p.Current = r
	}
	return nil
}

func (p *FakePage) Run(_ context.Context, _ string, args map[string]any, progress func(string)) (any, error) {
	if progress != nil {
		for _, j := range p.Progress {
			progress(j)
		}
	}
	p.Runs = append(p.Runs, args)
	if p.Answer == nil {
		if _, rpc := args["startMs"]; rpc { // Google Chat's search request
			return `{"messages":[]}`, nil
		}
		return "[]", nil
	}
	return p.Answer(p.Current, args), nil
}
