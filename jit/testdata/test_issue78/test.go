package test_issue78

import (
	"context"
	"fmt"
	"github.com/eihigh/goloader/jit/testdata/common"
	"unsafe"
)

var val = 1
var val2 = 1

func Test() (output int, output2 int) {
	val++
	val2++
	return val, val2
}

type eface struct {
	typ  uintptr
	data uintptr
}

func Test2() int {
	var ctx interface{}
	ctx = context.Background()
	fmt.Printf("Ctx addr: %p\n", ctx)

	select {
	case <-ctx.(context.Context).Done():
	default:
		return int((*eface)(unsafe.Pointer(&ctx)).data)
	}
	return 99
}

func Test3() (a int) {
	a = common.Val + 1
	a = common.Val - 1
	common.Inc()
	fmt.Println(common.Val)
	return common.Val
}
