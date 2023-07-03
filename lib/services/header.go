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
	"sync"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/protoadapt"

	"github.com/gravitational/teleport/api/types"
)

var (
	initHeaderJsonpbUnmarshaler sync.Once
	protojsonOptions            *protojson.UnmarshalOptions
)

// getHeaderProtoJSONOptions will return a singleton protojson options for unmarshaling.
func getHeaderProtoJSONOptions() *protojson.UnmarshalOptions {
	initHeaderJsonpbUnmarshaler.Do(func() {
		protojsonOptions = &protojson.UnmarshalOptions{
			DiscardUnknown: true,
		}
	})
	return protojsonOptions
}

// unmarshalHeaderWithProtoJSON will unmarshal the resource header using raw protojson.
// This is primarily of use for grpc messages that contain oneof and use the ResourceHeader,
// as utils.FastUnmarshal does not work for messages that use oneof.
func unmarshalHeaderWithProtoJSON(data []byte) (types.ResourceHeader, error) {
	var h types.MessageWithHeader
	if err := getHeaderProtoJSONOptions().Unmarshal(data, protoadapt.MessageV2Of(&h)); err != nil {
		return types.ResourceHeader{}, trace.BadParameter(err.Error())
	}

	return h.ResourceHeader, nil
}
