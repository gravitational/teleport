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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types/header"
)

func TestResourceHeaderRoundtrip(t *testing.T) {
	resourceHeader := header.ResourceHeader{
		Kind:    "kind",
		SubKind: "subkind",
		Version: "version",
		Metadata: header.Metadata{
			Name:        "name",
			Description: "description",
			Labels:      map[string]string{"label": "value"},
			Expires:     time.Now(),
			ID:          12345,
		},
	}

	converted := FromResourceHeaderProto(ToResourceHeaderProto(resourceHeader))

	require.Empty(t, cmp.Diff(resourceHeader, converted))
}

func TestMetadataRoundtrip(t *testing.T) {
	metadata := header.Metadata{
		Name:        "name",
		Description: "description",
		Labels:      map[string]string{"label": "value"},
		Expires:     time.Now(),
		ID:          12345,
	}

	converted := FromMetadataProto(ToMetadataProto(metadata))

	require.Empty(t, cmp.Diff(metadata, converted))
}

// TestMetadataZeroTime checks that go's zero time is mapped to the protobuf's
// zero time and vice-versa.
func TestMetadataZeroTime(t *testing.T) {
	// When a proto message without expiration is converted to metadata
	metadata := &headerv1.Metadata{
		Expires: nil,
	}
	converted := FromMetadataProto(metadata)
	// IsZero() must be true (as this is how we check if the resource expires
	// in most places).
	require.True(t, converted.Expires.IsZero())

	// When a metadata without an expiration is converted to protobuf
	convertedTwice := ToMetadataProto(converted)

	// The protobuf expiration field must be unset
	require.Empty(t, cmp.Diff(metadata.Expires, convertedTwice.Expires))
}
