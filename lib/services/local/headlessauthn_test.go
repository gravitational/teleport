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

package local_test

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

// TestIdentityService_HeadlessAuthenticationBackend tests headless authentication backend methods.
func TestIdentityService_HeadlessAuthenticationBackend(t *testing.T) {
	t.Parallel()
	identity := newIdentityService(t, clockwork.NewFakeClock())

	ctx := context.Background()
	pubUUID := services.NewHeadlessAuthenticationID([]byte(sshPubKey))
	expires := identity.Clock().Now().Add(time.Minute)
	headlessAuthn := &types.HeadlessAuthentication{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name:    pubUUID,
				Expires: &expires,
			},
		},
		User:         "user",
		SshPublicKey: []byte(sshPubKey),
		TlsPublicKey: []byte(tlsPubKey),
		State:        types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_PENDING,
	}

	// Create a headless authentication with minimal fields set (stub)
	stub, err := types.NewHeadlessAuthentication(headlessAuthn.User, headlessAuthn.Metadata.Name, headlessAuthn.Expiry())
	require.NoError(t, err)
	err = identity.UpsertHeadlessAuthentication(ctx, stub)
	require.NoError(t, err, "UpsertHeadlessAuthentication returned non-nil error")

	swapped, err := identity.CompareAndSwapHeadlessAuthentication(ctx, stub, headlessAuthn)
	require.NoError(t, err, "CompareAndSwapHeadlessAuthentication returned non-nil error")

	// Compare and swap should fail if the new headless authn fails validation.
	_, err = identity.CompareAndSwapHeadlessAuthentication(ctx, swapped, stub)
	require.True(t, trace.IsBadParameter(err), "CompareAndSwapHeadlessAuthentication expected bad parameter error but got: %v", err)

	retrieved, err := identity.GetHeadlessAuthentication(ctx, headlessAuthn.User, headlessAuthn.Metadata.Name)
	require.NoError(t, err, "GetHeadlessAuthentication returned non-nil error")
	require.Equal(t, swapped, retrieved)

	retrievedList, err := identity.GetHeadlessAuthentications(ctx)
	require.NoError(t, err, "GetHeadlessAuthentications returned non-nil error")
	require.Equal(t, []*types.HeadlessAuthentication{swapped}, retrievedList)

	err = identity.DeleteHeadlessAuthentication(ctx, headlessAuthn.User, headlessAuthn.Metadata.Name)
	require.NoError(t, err)

	_, err = identity.GetHeadlessAuthentication(ctx, headlessAuthn.User, headlessAuthn.Metadata.Name)
	require.True(t, trace.IsNotFound(err), "expected not found error but got: %v", err)
}

// sshPubKey is a randomly-generated public key used for login tests.
const sshPubKey = `ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBGv+gN2C23P08ieJRA9gU/Ik4bsOh3Kw193UYscJDw41mATj+Kqyf45Rmj8F8rs3i7mYKRXXu1IjNRBzNgpXxqc=`

// tlsPubKey is a randomly-generated public key used for login tests.
const tlsPubKey = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE/Jn3tYhc60M2IOen1yRht6r8xX3h
v7nNLYBIfxaKxXf+dAFVllYzVUrSzAQxi1LSAplOJVgOtHv0J69dRSUSzA==
-----END PUBLIC KEY-----`
