//go:build !windows && !(darwin && cgo)

package platform

import "unsafe"

// VerifyUser passes without asking: Unlock here only reads the keyring, which the Secret Service may guard with
// its own password prompt.
//
// TODO(linux): ask the user to prove it's them before Unlock, as on macOS (Touch ID / password) and Windows
// (Windows Hello). Candidates: polkit (`pkcheck --allow-user-interaction` with an action of our own, which shows
// the desktop's password dialog), or an app password that also wraps the database key. macOS builds without cgo
// (-tags purego) land here too.
func VerifyUser(string, unsafe.Pointer) error { return nil }
