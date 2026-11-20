package web

import (
	"net/url"
	"strings"
)

// GoogleSignInURL is Google's own sign-in page, returning to Chat afterwards.
const GoogleSignInURL = "https://accounts.google.com/ServiceLogin?service=dynamite&continue=https%3A%2F%2Fchat.google.com%2F"

// GoogleSignInFor is the sign-in page for one account: Google fills in the address, and adds the account next to
// the ones already signed in (the profiles' windows share one cookie store, as Google's multi-sign-in expects).
func GoogleSignInFor(email string) string {
	if email == "" {
		return GoogleSignInURL
	}
	return GoogleSignInURL + "&Email=" + url.QueryEscape(email)
}

// GoogleChatSignedIn: signed out, chat.google.com redirects to the sign-in (accounts.google.com) or the product
// page (workspace.google.com, www.google.com). Signed in it stays, or goes on to Chat in Gmail
// (mail.google.com/chat/…), as many Workspace accounts do.
func GoogleChatSignedIn(u string) bool {
	switch h := host(u); h {
	case "accounts.google.com", "workspace.google.com", "www.google.com", "":
		return false
	default:
		return h == "google.com" || strings.HasSuffix(h, ".google.com")
	}
}

// HRmasterSignedIn: signed out, HRmaster shows its /Account/Login page.
func HRmasterSignedIn(u string) bool {
	p, err := url.Parse(u)
	return err == nil && !strings.Contains(strings.ToLower(p.Path), "/account/login") && strings.Contains(p.Path, "/Datacenter")
}

func host(u string) string {
	p, err := url.Parse(u)
	if err != nil {
		return ""
	}
	return strings.ToLower(p.Hostname())
}
