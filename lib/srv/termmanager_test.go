/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package srv

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
)

func TestAtomicAlign(t *testing.T) {
	m := NewTermManager()

	verifyAlign := func(field *uint64) {
		addr := uintptr(unsafe.Pointer(field))
		require.True(t, addr%8 == 0, "field %v is not 8-byte aligned", field)
	}

	verifyAlign(&m.countWritten)
	verifyAlign(&m.countRead)
}
