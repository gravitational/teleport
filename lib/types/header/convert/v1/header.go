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

package headerv1

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/lib/types/header"
)

// FromResourceHeaderProto converts the resource header protobuf message into an internal resource header object.
// This function does not use the builder due to the generics for the builder object.
func FromResourceHeaderProto(msg *headerv1.ResourceHeader) header.ResourceHeader {
	return header.ResourceHeader{
		Kind:     msg.Kind,
		SubKind:  msg.SubKind,
		Version:  msg.Version,
		Metadata: FromMetadataProto(msg.Metadata),
	}
}

// ToResourceHeaderProto converts an internal resource header object into a v1 resource header protobuf message.
func ToResourceHeaderProto(resourceHeader header.ResourceHeader) *headerv1.ResourceHeader {
	return &headerv1.ResourceHeader{
		Kind:     resourceHeader.Kind,
		SubKind:  resourceHeader.SubKind,
		Version:  resourceHeader.Version,
		Metadata: ToMetadataProto(resourceHeader.Metadata),
	}
}

// FromMetadataProto converts v1 metadata into an internal metadata object.
func FromMetadataProto(msg *headerv1.Metadata) header.Metadata {
	return header.Metadata{
		Name:        msg.Name,
		Description: msg.Description,
		Labels:      msg.Labels,
		Expires:     msg.Expires.AsTime(),
		ID:          msg.Id,
	}
}

// ToMetadataProto converts an internal metadata object into a v1 metadata protobuf message.
func ToMetadataProto(metadata header.Metadata) *headerv1.Metadata {
	return &headerv1.Metadata{
		Name:        metadata.Name,
		Description: metadata.Description,
		Labels:      metadata.Labels,
		Expires:     timestamppb.New(metadata.Expires),
		Id:          metadata.ID,
	}
}
