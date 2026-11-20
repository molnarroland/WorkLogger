package store

import (
	"bytes"
	"database/sql"
	"encoding/hex"
	"errors"
	"io"
	"net/url"
	"os"

	"github.com/ncruces/go-sqlite3"
	"github.com/ncruces/go-sqlite3/driver"

	"github.com/molnarroland/WorkLogger/internal/core"
)

// plainHeader starts every unencrypted SQLite file.
var plainHeader = []byte("SQLite format 3\x00")

// Encrypted says whether a database file exists at path and is not plain SQLite (so it needs its key).
func Encrypted(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	head := make([]byte, len(plainHeader))
	n, _ := io.ReadFull(f, head)
	return n > 0 && !bytes.Equal(head[:n], plainHeader[:n])
}

// OpenEncrypted opens (or creates) the database at path encrypted at rest (Adiantum, 4 KiB blocks) with a
// 32-byte key given as 64 hex digits. A plain database already at path is encrypted first, in place.
func OpenEncrypted(path, hexKey string, days core.Days) (*Store, error) {
	if k, err := hex.DecodeString(hexKey); err != nil || len(k) != 32 {
		return nil, errors.New("the database key must be 32 bytes as 64 hex digits")
	}
	if err := encryptPlain(path, hexKey); err != nil {
		return nil, err
	}
	return openDSN(path, uri(path, ""), hexKey, days)
}

// uri is the encrypted file's SQLite URI, with the key only when given (VACUUM INTO has no other way to take it).
func uri(path, hexKey string) string {
	q := url.Values{"vfs": {"adiantum"}}
	if hexKey != "" {
		q.Set("hexkey", hexKey)
	}
	return (&url.URL{Scheme: "file", Path: path, RawQuery: q.Encode()}).String()
}

// encryptPlain rewrites a plain database at path as an encrypted one: copied with VACUUM INTO to a new file, that
// file opened with the key to check it, then moved over the old one. Nothing happens unless path is plain SQLite.
func encryptPlain(path, hexKey string) error {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	head := make([]byte, len(plainHeader))
	_, err = io.ReadFull(f, head)
	f.Close()
	if err != nil || !bytes.Equal(head, plainHeader) {
		return nil // empty, or already encrypted
	}
	tmp := path + ".encrypting"
	_ = os.Remove(tmp)
	plain, err := sql.Open("sqlite3", (&url.URL{Scheme: "file", Path: path}).String())
	if err != nil {
		return err
	}
	_, err = plain.Exec(`VACUUM INTO ?`, uri(tmp, hexKey))
	if cerr := plain.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = verify(tmp, hexKey)
	}
	if err != nil {
		_ = os.Remove(tmp)
		return errors.New("encrypting the database failed: " + err.Error())
	}
	// The plain database's WAL and index are checkpointed and gone once it closed; any left over belong to it.
	_ = os.Remove(path + "-wal")
	_ = os.Remove(path + "-shm")
	return os.Rename(tmp, path)
}

// verify opens the encrypted copy with the key and reads its schema.
func verify(path, hexKey string) error {
	db, err := driver.Open(uri(path, ""), func(c *sqlite3.Conn) error { return c.Exec("PRAGMA hexkey='" + hexKey + "'") })
	if err != nil {
		return err
	}
	defer db.Close()
	var n int
	return db.QueryRow(`SELECT count(*) FROM sqlite_master`).Scan(&n)
}
