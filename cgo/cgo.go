package cgo

/*
#include "sub/bar.c"
#include "foo.h"
*/
import "C"

func Test() int {
	return int(C.myTest())
}
