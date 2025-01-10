/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package local

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	accessgraphsecretspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	"github.com/gravitational/teleport/api/types/accessgraph"
	"github.com/gravitational/teleport/lib/backend/memory"
)

func TestAccessGraphAuthorizedKeys(t *testing.T) {
	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)
	service, err := NewAccessGraphSecretsService(backend)
	require.NoError(t, err)

	ctx := context.TODO()
	pageSize := 10
	pageToken := ""

	// Test case 1: Empty list
	keys, nextToken, err := service.ListAllAuthorizedKeys(ctx, pageSize, pageToken)
	require.NoError(t, err)
	require.Empty(t, keys)
	require.Empty(t, nextToken)

	// Test case 2: Non-empty list
	authorizedKeys := []*accessgraphsecretspb.AuthorizedKeySpec{
		{
			HostId:         "host1",
			HostUser:       "user1",
			KeyFingerprint: "AAAAB3NzaC1yc2EAAAADAQABAAABAQC...",
			KeyType:        "ssh-rsa",
		},
		{
			HostId:         "host1",
			HostUser:       "user2",
			KeyFingerprint: "AAAAB3NzaC1yc2EAAAADAQABAAABAQC...",
			KeyType:        "ssh-rsa",
		},
		{
			HostId:         "host2",
			HostUser:       "user1",
			KeyFingerprint: "AAAAB3NzaC1yc2EAAAADAQABAAABAQC...",
			KeyType:        "ssh-rsa",
		},
		{
			HostId:         "host2",
			HostUser:       "user2",
			KeyFingerprint: "AAAAB3NzaC1yc2EAAAADAQABAAABAQC...",
			KeyType:        "ssh-rsa",
		},
	}
	var authKeys []*accessgraphsecretspb.AuthorizedKey
	for _, key := range authorizedKeys {
		authKey, err := accessgraph.NewAuthorizedKey(key)
		require.NoError(t, err)
		_, err = service.UpsertAuthorizedKey(ctx, authKey)
		require.NoError(t, err)
		authKeys = append(authKeys, authKey)
	}

	keys, nextToken, err = service.ListAllAuthorizedKeys(ctx, pageSize, pageToken)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(authKeys, keys,
		protocmp.Transform(),
		cmpopts.SortSlices(func(a, b *accessgraphsecretspb.AuthorizedKey) bool {
			return a.Metadata.Name < b.Metadata.Name
		})))
	require.Empty(t, nextToken)

	// Test case 3: Pagination
	pageSize = 2
	pageToken = ""
	keys, nextToken, err = service.ListAllAuthorizedKeys(ctx, pageSize, pageToken)
	require.NoError(t, err)
	require.Len(t, keys, pageSize)
	require.NotEmpty(t, nextToken)

	pageToken = nextToken
	keys, nextToken, err = service.ListAllAuthorizedKeys(ctx, pageSize, pageToken)
	require.NoError(t, err)
	require.Len(t, keys, pageSize)
	require.Empty(t, nextToken)

	// Test case 4: List authorized keys for server
	pageToken = ""
	keysHost1, nextToken, err := service.ListAuthorizedKeysForServer(ctx, "host1", pageSize, pageToken)
	require.NoError(t, err)
	require.Len(t, keys, 2)
	require.Empty(t, nextToken)
	keysHost2, nextToken, err := service.ListAuthorizedKeysForServer(ctx, "host2", pageSize, pageToken)
	require.NoError(t, err)
	require.Len(t, keys, 2)
	require.Empty(t, nextToken)
	require.NotEqual(t, keysHost1, keysHost2)

	// Test case 5: List authorized keys for server with pagination
	pageToken = ""
	pageSize = 1
	keys, nextToken, err = service.ListAuthorizedKeysForServer(ctx, "host1", pageSize, pageToken)
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.NotEmpty(t, nextToken)

	pageToken = nextToken
	keys, nextToken, err = service.ListAuthorizedKeysForServer(ctx, "host1", pageSize, pageToken)
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.Empty(t, nextToken)

	// Test case 6: Delete all
	err = service.DeleteAllAuthorizedKeys(ctx)
	require.NoError(t, err)
	keys, nextToken, err = service.ListAllAuthorizedKeys(ctx, pageSize, pageToken)
	require.NoError(t, err)
	require.Empty(t, keys)
	require.Empty(t, nextToken)
}

func TestAccessGraphPrivateKeys(t *testing.T) {
	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)
	service, err := NewAccessGraphSecretsService(backend)
	require.NoError(t, err)

	ctx := context.TODO()
	pageSize := 10
	pageToken := ""

	// Test case 1: Empty list
	keys, nextToken, err := service.ListAllPrivateKeys(ctx, pageSize, pageToken)
	require.NoError(t, err)
	require.Empty(t, keys)
	require.Empty(t, nextToken)

	// Test case 2: Non-empty list
	privateKeysSpec := []*accessgraphsecretspb.PrivateKeySpec{
		{
			DeviceId:             "device1",
			PublicKeyMode:        accessgraphsecretspb.PublicKeyMode_PUBLIC_KEY_MODE_DERIVED,
			PublicKeyFingerprint: "AAAAB3NzaC1yc2EAAAADAQABAAABAQC...",
		},
		{
			DeviceId:             "device1",
			PublicKeyMode:        accessgraphsecretspb.PublicKeyMode_PUBLIC_KEY_MODE_PUB_FILE,
			PublicKeyFingerprint: "AAAAB3NzaC1yc2EAAAADAQABAAABAQC...",
		},
		{
			DeviceId:             "device2",
			PublicKeyMode:        accessgraphsecretspb.PublicKeyMode_PUBLIC_KEY_MODE_DERIVED,
			PublicKeyFingerprint: "AAAAB3NzaC1yc2EAAAADAQABAAABAQC...",
		},
		{
			DeviceId:             "device2",
			PublicKeyMode:        accessgraphsecretspb.PublicKeyMode_PUBLIC_KEY_MODE_PUB_FILE,
			PublicKeyFingerprint: "AAAAB3NzaC1yc2EAAAADAQABAAABAQC...",
		},
	}
	var authKeys []*accessgraphsecretspb.PrivateKey
	for _, key := range privateKeysSpec {
		name := uuid.New().String()
		prvKey, err := accessgraph.NewPrivateKeyWithName(name, key)
		require.NoError(t, err)
		_, err = service.UpsertPrivateKey(ctx, prvKey)
		require.NoError(t, err)
		authKeys = append(authKeys, prvKey)
	}

	keys, nextToken, err = service.ListAllPrivateKeys(ctx, pageSize, pageToken)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(authKeys, keys,
		protocmp.Transform(),
		cmpopts.SortSlices(func(a, b *accessgraphsecretspb.PrivateKey) bool {
			return a.Metadata.Name < b.Metadata.Name
		})))
	require.Empty(t, nextToken)

	// Test case 3: Pagination
	pageSize = 2
	pageToken = ""
	keys, nextToken, err = service.ListAllPrivateKeys(ctx, pageSize, pageToken)
	require.NoError(t, err)
	require.Len(t, keys, pageSize)
	require.NotEmpty(t, nextToken)

	pageToken = nextToken
	keys, nextToken, err = service.ListAllPrivateKeys(ctx, pageSize, pageToken)
	require.NoError(t, err)
	require.Len(t, keys, pageSize)
	require.Empty(t, nextToken)

	// Test case 4: List private keys for device
	pageToken = ""
	keysHost1, nextToken, err := service.ListPrivateKeysForDevice(ctx, "device1", pageSize, pageToken)
	require.NoError(t, err)
	require.Len(t, keys, 2)
	require.Empty(t, nextToken)
	keysHost2, nextToken, err := service.ListPrivateKeysForDevice(ctx, "device2", pageSize, pageToken)
	require.NoError(t, err)
	require.Len(t, keys, 2)
	require.Empty(t, nextToken)
	require.NotEqual(t, keysHost1, keysHost2)

	// Test case 5: List private keys for device with pagination
	pageToken = ""
	pageSize = 1
	keys, nextToken, err = service.ListPrivateKeysForDevice(ctx, "device1", pageSize, pageToken)
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.NotEmpty(t, nextToken)

	pageToken = nextToken
	keys, nextToken, err = service.ListPrivateKeysForDevice(ctx, "device1", pageSize, pageToken)
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.Empty(t, nextToken)

	// Test case 6: Delete all
	err = service.DeleteAllPrivateKeys(ctx)
	require.NoError(t, err)
	keys, nextToken, err = service.ListAllPrivateKeys(ctx, pageSize, pageToken)
	require.NoError(t, err)
	require.Empty(t, keys)
	require.Empty(t, nextToken)
}
