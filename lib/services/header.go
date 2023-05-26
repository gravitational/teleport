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

package services

import (
	"bytes"
	"sync"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

var (
	initHeaderJsonpbUnmarshaler sync.Once
	headerJsonpbUnmarshaler     *jsonpb.Unmarshaler
)

// getHeaderJsonpbUnmarshaler will return a singleton jsonpb unmarshaler.
func getHeaderJsonpbUnmarshaler() *jsonpb.Unmarshaler {
	initHeaderJsonpbUnmarshaler.Do(func() {
		headerJsonpbUnmarshaler = &jsonpb.Unmarshaler{}
		headerJsonpbUnmarshaler.AllowUnknownFields = true
	})
	return headerJsonpbUnmarshaler
}

// unmarshalHeaderWithJsonpb will unmarshal the resource header using raw jsonpb.
// This is primarily of use for grpc messages that contain oneof and use the ResourceHeader,
// as utils.FastUnmarshal does not work for messages that use oneof.
func unmarshalHeaderWithJsonpb(data []byte) (types.ResourceHeader, error) {
	var h types.MessageWithHeader
	if err := getHeaderJsonpbUnmarshaler().Unmarshal(bytes.NewReader(data), &h); err != nil {
		return types.ResourceHeader{}, trace.BadParameter(err.Error())
	}

	return h.ResourceHeader, nil
}
