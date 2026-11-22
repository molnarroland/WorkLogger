package platform

import (
	"crypto/sha1"
	"encoding/binary"
)

// guid is a COM GUID in its in-memory layout. Here, not in the Windows file, so its tests run everywhere.
type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

// winrtNamespace is the namespace WinRT hashes parameterized interface signatures in.
var winrtNamespace = [16]byte{0x11, 0xf4, 0x7a, 0xd5, 0x7b, 0x73, 0x42, 0xc0, 0xab, 0xae, 0x87, 0x8b, 0x1e, 0x16, 0xad, 0xee}

// parameterizedIID is the IID of a WinRT generic interface instance, from its signature, such as
// "pinterface({9fc2b0bb-e446-44e2-aa61-9cab8f636af2};b1)" for IAsyncOperation<Boolean>: a version 5 UUID of the
// signature's SHA-1 (what C++/WinRT's __uuidof and the .NET projection compute).
func parameterizedIID(signature string) guid {
	h := sha1.New()
	h.Write(winrtNamespace[:])
	h.Write([]byte(signature))
	sum := h.Sum(nil)
	g := guid{
		Data1: binary.BigEndian.Uint32(sum[0:4]),
		Data2: binary.BigEndian.Uint16(sum[4:6]),
		Data3: binary.BigEndian.Uint16(sum[6:8])&0x0fff | 0x5000,
	}
	copy(g.Data4[:], sum[8:16])
	g.Data4[0] = g.Data4[0]&0x3f | 0x80
	return g
}
