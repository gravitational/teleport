/*
Copyright 2023 Gravitational, Inc.

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

package utils

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/protoadapt"
)

// CloneProtoMsg returns a deep copy of msg. Modifying the returned
// protobuf message will not affect msg. If msg contains any empty
// slices, the returned copy will have nil slices instead.
func CloneProtoMsg[T protoadapt.MessageV1](msg T) T {
	// github.com/golang/protobuf/proto.Clone panics when trying to
	// copy a map[K]V where the type of V is a slice of anything
	// other than byte. See https://github.com/gogo/protobuf/issues/14
	msgV2 := protoadapt.MessageV2Of(msg)
	msgV2 = proto.Clone(msgV2)
	// this is safe as protoadapt.MessageV2Of will simply wrap the message
	// with a type that implements the protobuf v2 API, and
	// protoadapt.MessageV1Of will return the unwrapped message
	return protoadapt.MessageV1Of(msgV2).(T)
}
