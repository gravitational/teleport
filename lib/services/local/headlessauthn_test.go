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
		User:      "user",
		PublicKey: []byte(sshPubKey),
		State:     types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_PENDING,
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
