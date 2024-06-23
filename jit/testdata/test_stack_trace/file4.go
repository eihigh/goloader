package test_stack_trace

import (
	"github.com/eihigh/goloader/jit/testdata/common"
)

//go:noinline
func (m *SomeType) callSite4(msg common.SomeStruct) {

	// ARSE

	// ARSE

	// ARSE

	panic(msg.Val1)
}
