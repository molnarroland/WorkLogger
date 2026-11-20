package core

import (
	"context"
	"net/http"
	"time"
)

// The testability seam: every edge the services touch. internal/platform has the real ones,
// internal/testsupport the fakes.

type Clock interface{ Now() time.Time }

// SecretStore keeps API tokens (the keyring). Set with an empty value deletes.
type SecretStore interface {
	Get(account string) (string, bool)
	Set(account, value string) error
}

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) (stdout string, err error)
}

// PageDriver is a signed-in web page the collectors drive.
type PageDriver interface {
	SiteName() string
	CurrentURL(ctx context.Context) string
	Open(ctx context.Context, url string) error
	WaitUntilLoaded(ctx context.Context) error
	// Run runs script as the body of an async function with args as its named parameters and returns its result.
	// progress, when not nil, gets the script's window.__wlProgress (a JSON string) while it runs.
	Run(ctx context.Context, script string, args map[string]any, progress func(string)) (any, error)
	IsSignedIn(ctx context.Context) bool
}
