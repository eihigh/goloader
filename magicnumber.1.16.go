//go:build go1.16 && !go1.18
// +build go1.16,!go1.18

package goloader

//golang 1.16 change magic number
var (
	x86moduleHead = []byte{0xFA, 0xFF, 0xFF, 0xFF, 0x0, 0x0, 0x1, PtrSize}
	armmoduleHead = []byte{0xFA, 0xFF, 0xFF, 0xFF, 0x0, 0x0, 0x4, PtrSize}
)