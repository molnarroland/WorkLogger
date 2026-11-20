package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	ts "github.com/molnarroland/WorkLogger/internal/testsupport"
)

const key = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"

// A plain database is encrypted in place on the first encrypted open, keeps its data, and needs the key after.
func TestOpenEncrypted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Application Support", "worklogger.sqlite")
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	plain, err := Open(path, ts.Days)
	if err != nil {
		t.Fatal(err)
	}
	st := plain.LoadSettings(func() string { return "id" })
	st.AuthorEmails = []string{"me@example.com"}
	if err := plain.SaveSettings(st); err != nil {
		t.Fatal(err)
	}
	plain.Close()
	if Encrypted(path) {
		t.Fatal("a plain database reads as encrypted")
	}

	enc, err := OpenEncrypted(path, key, ts.Days)
	if err != nil {
		t.Fatal(err)
	}
	if got := enc.LoadSettings(func() string { return "id" }).AuthorEmails; len(got) != 1 || got[0] != "me@example.com" {
		t.Errorf("the data moved over: %v", got)
	}
	enc.Close()
	if !Encrypted(path) {
		t.Error("the file is still plain SQLite")
	}
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "me@example.com") {
		t.Error("the data is readable in the file")
	}

	again, err := OpenEncrypted(path, key, ts.Days)
	if err != nil {
		t.Fatalf("reopening with the key: %v", err)
	}
	again.Close()
	if _, err := OpenEncrypted(path, strings.Repeat("ab", 32), ts.Days); err == nil {
		t.Error("a wrong key opened the database")
	}
}
