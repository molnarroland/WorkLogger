package web

import "testing"

func TestGoogleChatSignedIn(t *testing.T) {
	for u, want := range map[string]bool{
		"https://chat.google.com/u/0/app/home":                 true,
		"https://mail.google.com/chat/u/0/#chat/home":          true,
		"https://accounts.google.com/v3/signin/identifier?x=1": false,
		"https://workspace.google.com/products/chat/":          false,
		"https://www.google.com/intl/hu/chat/":                 false,
		"https://calendar.google.com/calendar/u/0/r":           true,
		"":                     false,
		"https://example.com/": false,
	} {
		if GoogleChatSignedIn(u) != want {
			t.Errorf("%q: want %v", u, want)
		}
	}
}
