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

package local

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	mfav2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v2"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
)

func TestValidatedMFAChallengeParser_FilterByTargetCluster(t *testing.T) {
	t.Parallel()

	const (
		chalName      = "test-chal"
		sessionID     = "session-id"
		sourceCluster = "root"
		testUsername  = "alice"
		leafA         = "leaf-a"
		leafB         = "leaf-b"
		filterKey     = "target_cluster"
	)

	var filterLeafA = map[string]string{filterKey: leafA}

	marshalChal := func(targetCluster string) []byte {
		chal := mfav2.ValidatedMFAChallenge_builder{
			Kind:    types.KindValidatedMFAChallenge,
			Version: types.V1,
			Metadata: headerv1.Metadata_builder{
				Name:    chalName,
				Expires: timestamppb.Now(),
			}.Build(),
			Spec: mfav2.ValidatedMFAChallengeSpec_builder{
				Payload: mfav2.SessionIdentifyingPayload_builder{
					SshSessionId: []byte(sessionID),
				}.Build(),
				SourceCluster: sourceCluster,
				TargetCluster: targetCluster,
				Username:      testUsername,
			}.Build(),
		}.Build()

		data, err := MarshalValidatedMFAChallenge(chal)
		require.NoError(t, err)

		return data
	}

	makeEvent := func(targetCluster string, isDelete bool) backend.Event {
		key := backend.NewKey(types.KindValidatedMFAChallenge, targetCluster, chalName)
		if isDelete {
			return backend.Event{Type: types.OpDelete, Item: backend.Item{Key: key}}
		}

		return backend.Event{
			Type: types.OpPut,
			Item: backend.Item{
				Key:     key,
				Value:   marshalChal(targetCluster),
				Expires: timestamppb.Now().AsTime(),
			},
		}
	}

	for _, tc := range []struct {
		name          string
		filter        map[string]string
		targetCluster string
		isDelete      bool
		wantResource  bool
	}{
		{
			name:          "empty filter passes challenge",
			filter:        nil,
			targetCluster: leafA,
			isDelete:      false,
			wantResource:  true,
		},
		{
			name:          "filter matches target cluster",
			filter:        filterLeafA,
			targetCluster: leafA,
			isDelete:      false,
			wantResource:  true,
		},
		{
			name:          "filter excludes non-matching target cluster",
			filter:        filterLeafA,
			targetCluster: leafB,
			isDelete:      false,
			wantResource:  false,
		},
		{
			name:          "delete passes through regardless of filter",
			filter:        filterLeafA,
			targetCluster: leafB,
			isDelete:      true,
			wantResource:  true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parser := newValidatedMFAChallengeParser(tc.filter)
			event := makeEvent(tc.targetCluster, tc.isDelete)

			resource, err := parser.parse(event)
			require.NoError(t, err)
			require.Equal(t, tc.wantResource, resource != nil)
		})
	}
}
