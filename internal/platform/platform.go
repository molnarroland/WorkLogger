// Package platform holds the real adapters behind core's ports: the clock, the git runner and the keyring.
// It is the only package that calls time.Now() or os/exec.
package platform

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/zalando/go-keyring"
)

type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }

// ExecRunner runs a command line tool found on PATH (plus the usual Homebrew/Unix folders on macOS and Linux).
type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	env := os.Environ()
	env = append(env, "GIT_TERMINAL_PROMPT=0")
	if path := os.Getenv("PATH"); path != "" && os.PathSeparator == '/' {
		env = append(env, "PATH="+path+":/usr/bin:/usr/local/bin:/opt/homebrew/bin")
	}
	cmd.Env = env
	hideWindow(cmd)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			label := strings.Join(append([]string{name}, args[:min(2, len(args))]...), " ")
			return "", fmt.Errorf("%s exited with %d: %s", label, cmd.ProcessState.ExitCode(), strings.TrimSpace(errOut.String()))
		}
		return "", err
	}
	return out.String(), nil
}

// Keyring keeps every API token in one keyring item as JSON, so reading them asks for access
// once. It starts locked: nothing is read or written until Unlock.
type Keyring struct {
	Service string // "WorkLogger"
	mu      sync.Mutex
	values  map[string]string // nil while locked
}

const keyringItem = "tokens"

// Unlock reads the item. False when the keyring refused (an empty keyring is fine).
func (k *Keyring) Unlock() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	raw, err := keyring.Get(k.Service, keyringItem)
	switch {
	case err == keyring.ErrNotFound:
		k.values = map[string]string{}
	case err != nil:
		return false
	default:
		values := map[string]string{}
		if json.Unmarshal([]byte(raw), &values) != nil {
			return false // don't overwrite an item we can't read
		}
		k.values = values
	}
	return true
}

func (k *Keyring) Locked() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.values == nil
}

func (k *Keyring) Get(account string) (string, bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	v, ok := k.values[account]
	return v, ok && v != ""
}

// Set stores a token (empty deletes) and writes the whole item back.
func (k *Keyring) Set(account, value string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.values == nil {
		return fmt.Errorf("the keyring is locked")
	}
	if value == "" {
		delete(k.values, account)
	} else {
		k.values[account] = value
	}
	data, _ := json.Marshal(k.values)
	return keyring.Set(k.Service, keyringItem, string(data))
}

// databaseKeyItem is the keyring item holding the database's encryption key: its own item, read at start-up
// (the tokens wait for Unlock).
const databaseKeyItem = "database-key"

// DatabaseKey is the database's encryption key (64 hex digits) from the OS keyring, made on the first run.
// existing says the database is already encrypted: then a missing key is an error, never a new key (the old one
// is the only way to read it).
func DatabaseKey(service string, existing bool) (string, error) {
	key, err := keyring.Get(service, databaseKeyItem)
	switch {
	case err == nil:
		if b, err := hex.DecodeString(key); err != nil || len(b) != 32 {
			return "", fmt.Errorf("the keyring's database key is damaged")
		}
		return key, nil
	case err != keyring.ErrNotFound:
		return "", fmt.Errorf("reading the database key from the keyring: %w", err)
	case existing:
		return "", fmt.Errorf("the database is encrypted, but its key is missing from the keyring")
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	key = hex.EncodeToString(b)
	if err := keyring.Set(service, databaseKeyItem, key); err != nil {
		return "", fmt.Errorf("saving the database key in the keyring: %w", err)
	}
	// Encrypt only with a key the keyring gives back.
	if back, err := keyring.Get(service, databaseKeyItem); err != nil || back != key {
		return "", fmt.Errorf("the keyring didn't keep the database key")
	}
	return key, nil
}
