/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
		return types.ResourceHeader{}, trace.BadParameter("%s", err)
	}

	return h.ResourceHeader, nil
}
