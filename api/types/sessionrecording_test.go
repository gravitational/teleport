// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package types_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestSetEncryptionKeys(t *testing.T) {
	cases := []struct {
		name         string
		initialKeys  []*types.AgeEncryptionKey
		newKeys      []*types.AgeEncryptionKey
		expectChange bool
	}{
		{
			name:         "adding new keys to empty list",
			expectChange: true,
			newKeys: []*types.AgeEncryptionKey{
				{
					PublicKey: []byte("123"),
				},
				{
					PublicKey: []byte("456"),
				},
			},
		}, {
			name:         "adding new keys to existing list",
			expectChange: true,
			initialKeys: []*types.AgeEncryptionKey{
				{
					PublicKey: []byte("123"),
				},
				{
					PublicKey: []byte("456"),
				},
			},
			newKeys: []*types.AgeEncryptionKey{
				{
					PublicKey: []byte("123"),
				},
				{
					PublicKey: []byte("456"),
				},
				{
					PublicKey: []byte("789"),
				},
			},
		}, {
			name:         "replacing existing keys",
			expectChange: true,
			initialKeys: []*types.AgeEncryptionKey{
				{
					PublicKey: []byte("123"),
				},
				{
					PublicKey: []byte("456"),
				},
			},
			newKeys: []*types.AgeEncryptionKey{
				{
					PublicKey: []byte("321"),
				},
				{
					PublicKey: []byte("654"),
				},
			},
		}, {
			name:         "removing from existing keys",
			expectChange: true,
			initialKeys: []*types.AgeEncryptionKey{
				{
					PublicKey: []byte("123"),
				},
				{
					PublicKey: []byte("456"),
				},
				{
					PublicKey: []byte("789"),
				},
			},
			newKeys: []*types.AgeEncryptionKey{
				{
					PublicKey: []byte("123"),
				},
				{
					PublicKey: []byte("456"),
				},
			},
		}, {
			name:         "try to remove all keys",
			expectChange: false,
			initialKeys: []*types.AgeEncryptionKey{
				{
					PublicKey: []byte("123"),
				},
				{
					PublicKey: []byte("456"),
				},
				{
					PublicKey: []byte("789"),
				},
			},
		}, {
			name:         "no change",
			expectChange: false,
			initialKeys: []*types.AgeEncryptionKey{
				{
					PublicKey: []byte("123"),
				},
				{
					PublicKey: []byte("456"),
				},
			},
			newKeys: []*types.AgeEncryptionKey{
				{
					PublicKey: []byte("123"),
				},
				{
					PublicKey: []byte("456"),
				},
			},
		}, {
			name:         "adding duplicates",
			expectChange: false,
			initialKeys: []*types.AgeEncryptionKey{
				{
					PublicKey: []byte("123"),
				},
				{
					PublicKey: []byte("456"),
				},
			},
			newKeys: []*types.AgeEncryptionKey{
				{
					PublicKey: []byte("123"),
				},
				{
					PublicKey: []byte("123"),
				},
				{
					PublicKey: []byte("456"),
				},
				{
					PublicKey: []byte("456"),
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := &types.SessionRecordingConfigV2{
				Status: &types.SessionRecordingConfigStatus{
					EncryptionKeys: c.initialKeys,
				},
			}

			keysChanged := src.SetEncryptionKeys(slices.Values(c.newKeys))
			require.Equal(t, c.expectChange, keysChanged)
			if keysChanged {
				require.Equal(t, c.newKeys, src.Status.EncryptionKeys)
			} else {
				require.Equal(t, c.initialKeys, src.Status.EncryptionKeys)
			}
		})
	}
}
