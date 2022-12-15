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

package types

import (
	"strings"

	"github.com/gogo/protobuf/proto"
	"github.com/google/go-cmp/cmp"
)

// protoMessage verifies *T is a proto.Message.
type protoMessage[T any] interface {
	proto.Message
	*T
}

// isProtoEmpty returns true if provided proto-generated struct is empty,
// ignoring XXX_* fields.
func isProtoEmpty[T any, PT protoMessage[T]](t T) bool {
	var empty T
	return equalProtoIngoreXXXFields[T, PT](t, empty)
}

// equalProtoIngoreXXXFields returns true if provided proto-generated structs
// are equal, ignoring XXX_* fields.
func equalProtoIngoreXXXFields[T any, PT protoMessage[T]](a, b T) bool {
	return cmp.Equal(a, b, cmp.FilterPath(isXXXField, cmp.Ignore()))
}

func isXXXField(path cmp.Path) bool {
	return strings.HasPrefix(path.Last().String(), ".XXX_")
}
