//go:build darwin
// +build darwin

package mmap

import (
	"github.com/eihigh/goloader/mmap/mapping"
	"github.com/eihigh/goloader/mmap/vmmap"
)

// https://developer.apple.com/library/archive/documentation/Performance/Conceptual/ManagingMemory/Articles/VMPages.html
func getCurrentProcMaps() ([]mapping.Mapping, error) {
	return vmmap.Vmmap()
}
