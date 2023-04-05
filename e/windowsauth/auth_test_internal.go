package main

/*
#include <stdlib.h>
#include "auth.h"

extern PLSA_DISPATCH_TABLE ptab;
*/
import "C"

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/windows"
)

func setupDispatchTable() {
	DispatchTable = C.ptab
}
func testToLSAString(t *testing.T) {
	for _, s := range []string{"", "abcd", "xyz"} {
		us, err := toLSAString(s)
		assert.Equal(t, len(s)*2, int(us.Length))
		assert.Equal(t, len(s)*2+2, int(us.MaximumLength))
		assert.Equal(t, s, windows.UTF16PtrToString((*uint16)(unsafe.Pointer(us.Buffer))))
		if assert.NoError(t, err) {
			C.free(unsafe.Pointer(us.Buffer))
			C.free(unsafe.Pointer(us))
		}
	}
}
