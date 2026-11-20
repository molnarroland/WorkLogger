package tempo

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"

	"github.com/molnarroland/WorkLogger/internal/core"
)

// An Atlassian API token with scopes (fine-grained) is refused on the site's own address (401): it works only
// through Atlassian's gateway, https://api.atlassian.com/ex/<product>/<cloud id>. A classic token works on both.
const gatewayBase = "https://api.atlassian.com/ex/"

// gateways remembers each site's gateway address per product once a scoped token needed it ("jira|site" → address).
var gateways sync.Map

// ForgetGateway drops what was learnt about the site's gateways (a token changed: a classic one may work on the
// site again).
func ForgetGateway(site string) {
	gateways.Delete("jira|" + site)
	gateways.Delete("confluence|" + site)
}

// gateway is the site's gateway address for a product ("jira", "confluence"): its cloud id read from
// <site>/_edge/tenant_info (no sign-in needed).
func gateway(ctx context.Context, h core.HTTPDoer, site, product string) (string, error) {
	if g, ok := gateways.Load(product + "|" + site); ok {
		return g.(string), nil
	}
	req, err := http.NewRequestWithContext(ctx, "GET", site+"/_edge/tenant_info", nil)
	if err != nil {
		return "", err
	}
	resp, err := h.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var info struct {
		CloudID string `json:"cloudId"`
	}
	if resp.StatusCode != 200 || json.NewDecoder(resp.Body).Decode(&info) != nil || info.CloudID == "" {
		return "", errors.New("no cloud id at " + site)
	}
	g := gatewayBase + product + "/" + info.CloudID
	gateways.Store(product+"|"+site, g)
	return g, nil
}

// jiraSend is a Jira REST call (path from /rest/…): on the site, or through the gateway when the site refuses the
// token (or refused it before).
func (c *Client) jiraSend(ctx context.Context, method, path string, body any, what string) ([]byte, error) {
	auth := basicAuth(c.JiraEmail, c.JiraToken)
	return siteSend(ctx, c.HTTP, c.send, c.JiraBaseURL, "jira", method, path, auth, body, what)
}

type sender func(ctx context.Context, method, rawURL, auth string, body any, what string) ([]byte, error)

// siteSend is an Atlassian REST call: on the site, or through the product's gateway when the site refuses the token
// (or refused it before).
func siteSend(ctx context.Context, h core.HTTPDoer, send sender, site, product, method, path, auth string, body any, what string) ([]byte, error) {
	if g, ok := gateways.Load(product + "|" + site); ok {
		return send(ctx, method, g.(string)+path, auth, body, what)
	}
	data, err := send(ctx, method, site+path, auth, body, what)
	var he HTTPError
	// A scoped token gets 401 on most calls, but 404 on an issue ("does not exist or you do not have permission").
	if errors.As(err, &he) && (he.Code == http.StatusUnauthorized || he.Code == http.StatusNotFound) {
		if g, gerr := gateway(ctx, h, site, product); gerr == nil {
			return send(ctx, method, g+path, auth, body, what)
		}
	}
	return data, err
}
