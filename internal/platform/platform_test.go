package platform

import (
	"errors"
	"testing"

	"github.com/zalando/go-keyring"
)

// RELEASE_REVIEW C12: the keyring wrapper starts locked, round-trips its JSON item, and never overwrites an item
// it can't read.
func TestKeyring(t *testing.T) {
	keyring.MockInit()

	t.Run("starts locked and refuses writes", func(t *testing.T) {
		k := &Keyring{Service: "WorkLogger-test-locked"}
		if !k.Locked() {
			t.Fatal("not locked")
		}
		if err := k.Set("jira.apiToken.j1", "t"); err == nil {
			t.Error("wrote while locked")
		}
		if _, ok := k.Get("jira.apiToken.j1"); ok {
			t.Error("read while locked")
		}
	})
	t.Run("an empty keyring unlocks, and values survive a new unlock", func(t *testing.T) {
		k := &Keyring{Service: "WorkLogger-test-roundtrip"}
		if !k.Unlock() || k.Locked() {
			t.Fatal("did not unlock")
		}
		if err := k.Set("jira.apiToken.j1", "secret"); err != nil {
			t.Fatal(err)
		}
		again := &Keyring{Service: "WorkLogger-test-roundtrip"}
		if !again.Unlock() {
			t.Fatal("second unlock")
		}
		if v, ok := again.Get("jira.apiToken.j1"); !ok || v != "secret" {
			t.Errorf("got %q %v", v, ok)
		}
		if err := again.Set("jira.apiToken.j1", ""); err != nil {
			t.Fatal(err)
		}
		if _, ok := again.Get("jira.apiToken.j1"); ok {
			t.Error("an empty value didn't delete")
		}
	})
	t.Run("an unreadable item stays locked and untouched", func(t *testing.T) {
		if err := keyring.Set("WorkLogger-test-garbled", keyringItem, "not json"); err != nil {
			t.Fatal(err)
		}
		k := &Keyring{Service: "WorkLogger-test-garbled"}
		if k.Unlock() || !k.Locked() {
			t.Error("unlocked a garbled item")
		}
		if raw, _ := keyring.Get("WorkLogger-test-garbled", keyringItem); raw != "not json" {
			t.Errorf("item changed to %q", raw)
		}
	})
	t.Run("a keyring that refuses leaves it locked", func(t *testing.T) {
		keyring.MockInitWithError(errors.New("user cancelled"))
		defer keyring.MockInit()
		k := &Keyring{Service: "WorkLogger-test-refused"}
		if k.Unlock() || !k.Locked() {
			t.Error("unlocked after a refusal")
		}
	})
}
