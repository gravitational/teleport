// Copyright 2026 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
