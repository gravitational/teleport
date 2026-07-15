// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package services

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	discoveryservicev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryservice/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

func TestMarshalDiscoveryServiceRejectsMissingMetadata(t *testing.T) {
	_, err := MarshalDiscoveryService(discoveryservicev1.DiscoveryService_builder{}.Build())
	require.True(t, trace.IsBadParameter(err), "expected BadParameter, got %v", err)
	require.ErrorContains(t, err, "missing metadata")
}

func TestUnmarshalDiscoveryServiceRejectsMissingMetadata(t *testing.T) {
	_, err := UnmarshalDiscoveryService([]byte(`{}`), WithRevision("revision"))
	require.True(t, trace.IsBadParameter(err), "expected BadParameter, got %v", err)
	require.ErrorContains(t, err, "missing metadata")
}

// TestStaticMatcherCountKeyListCannotMutateValidation pins the copy contract of
// [StaticMatcherCountKeyList]: writing through the returned slice must never
// reach the validator's allowed-key set. If the clone is ever replaced with a
// direct return of staticMatcherCountKeys, the overwrite below would alias the
// private slice and the injected key would pass validation.
func TestStaticMatcherCountKeyListCannotMutateValidation(t *testing.T) {
	keys := StaticMatcherCountKeyList()
	require.NotEmpty(t, keys)
	for i := range keys {
		keys[i] = "injected"
	}
	require.NotContains(t, staticMatcherCountKeys, "injected")

	err := ValidateDiscoveryService(discoveryservicev1.DiscoveryService_builder{
		Kind:     types.KindDiscoveryService,
		Version:  types.V1,
		Metadata: headerv1.Metadata_builder{Name: "host-id"}.Build(),
		Spec: discoveryservicev1.DiscoveryServiceSpec_builder{
			MatchersTruncated:   true,
			StaticMatcherCounts: map[string]int32{"injected": 1},
		}.Build(),
	}.Build())
	require.True(t, trace.IsBadParameter(err), "expected BadParameter, got %v", err)
	require.ErrorContains(t, err, `unknown static matcher count key "injected"`)
}

// TestMarshalDiscoveryServiceEmitsOpenStructFields pins the encoding/json codec
// to the hybrid (open struct) codegen for discoveryservice/v1. If a protoopaque
// migration removes the open struct fields, json.Marshal emits "{}" and every
// heartbeat write silently stores an empty record; that must fail here, next to
// the codec, not only in the lib/services/local round-trip tests.
func TestMarshalDiscoveryServiceEmitsOpenStructFields(t *testing.T) {
	data, err := MarshalDiscoveryService(discoveryservicev1.DiscoveryService_builder{
		Kind:     types.KindDiscoveryService,
		Version:  types.V1,
		Metadata: headerv1.Metadata_builder{Name: "host-id"}.Build(),
		Spec: discoveryservicev1.DiscoveryServiceSpec_builder{
			Hostname: "disc-1.example.com",
		}.Build(),
	}.Build())
	require.NoError(t, err)
	require.Contains(t, string(data), `"hostname":"disc-1.example.com"`,
		"encoding/json output is missing open struct spec fields (got %s)", data)
	require.Contains(t, string(data), `"name":"host-id"`,
		"encoding/json output is missing open struct metadata fields (got %s)", data)
}
