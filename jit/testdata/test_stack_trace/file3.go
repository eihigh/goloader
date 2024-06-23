package test_stack_trace

import (
	"github.com/eihigh/goloader/jit/testdata/common"
)

//go:noinline
func (m *SomeType) callSite3(msg common.SomeStruct) {

	// ARSE

	// ARSE
	m.callSite4(msg)
}
