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

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

// TestIdentityService_HeadlessAuthenticationBackend tests headless authentication
// backend methods for functionality and validation.
func TestIdentityService_HeadlessAuthenticationBackend(t *testing.T) {
	t.Parallel()
	identity := newIdentityService(t, clockwork.NewFakeClock())

	ctx := context.Background()
	pubUUID := services.NewHeadlessAuthenticationID([]byte(sshPubKey))
	expires := time.Now().Add(time.Minute)

	expectBadParameter := func(tt require.TestingT, err error, i ...interface{}) {
		require.True(t, trace.IsBadParameter(err), "expected bad parameter error but got: %v", err)
	}

	tests := []struct {
		name              string
		ha                *types.HeadlessAuthentication
		createStubErr     require.ErrorAssertionFunc
		compareAndSwapErr require.ErrorAssertionFunc
	}{
		{
			name: "OK headless authentication",
			ha: &types.HeadlessAuthentication{
				ResourceHeader: types.ResourceHeader{
					Metadata: types.Metadata{
						Name:    pubUUID,
						Expires: &expires,
					},
				},
				User:      "user",
				PublicKey: []byte(sshPubKey),
			},
		}, {
			name: "NOK name missing",
			ha: &types.HeadlessAuthentication{
				ResourceHeader: types.ResourceHeader{
					Metadata: types.Metadata{
						Expires: &expires,
					},
				},
				User:      "user",
				PublicKey: []byte(sshPubKey),
			},
			createStubErr: expectBadParameter,
		}, {
			name: "NOK expires missing",
			ha: &types.HeadlessAuthentication{
				ResourceHeader: types.ResourceHeader{
					Metadata: types.Metadata{
						Name: pubUUID,
					},
				},
				User:      "user",
				PublicKey: []byte(sshPubKey),
			},
			compareAndSwapErr: expectBadParameter,
		}, {
			name: "NOK username missing",
			ha: &types.HeadlessAuthentication{
				ResourceHeader: types.ResourceHeader{
					Metadata: types.Metadata{
						Name:    pubUUID,
						Expires: &expires,
					},
				},
				PublicKey: []byte(sshPubKey),
			},
			compareAndSwapErr: expectBadParameter,
		}, {
			name: "NOK name not derived from public key",
			ha: &types.HeadlessAuthentication{
				ResourceHeader: types.ResourceHeader{
					Metadata: types.Metadata{
						Name:    uuid.NewString(),
						Expires: &expires,
					},
				},
				User:      "user",
				PublicKey: []byte(sshPubKey),
			},
			compareAndSwapErr: expectBadParameter,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			expires := identity.Clock().Now().Add(time.Minute)
			stub, err := types.NewHeadlessAuthenticationStub(test.ha.Metadata.Name, expires)
			if test.createStubErr != nil {
				test.createStubErr(t, err)
				return
			}

			err = identity.UpsertHeadlessAuthentication(ctx, stub)
			require.NoError(t, err, "UpsertHeadlessAuthentication returned non-nil error")

			t.Cleanup(func() {
				err = identity.DeleteHeadlessAuthentication(ctx, test.ha.Metadata.Name)
				require.NoError(t, err)

				_, err = identity.GetHeadlessAuthentication(ctx, test.ha.Metadata.Name)
				require.True(t, trace.IsNotFound(err), "expected not found error but got: %v", err)
			})

			swapped, err := identity.CompareAndSwapHeadlessAuthentication(ctx, stub, test.ha)
			if test.compareAndSwapErr != nil {
				test.compareAndSwapErr(t, err)
				return
			}
			require.NoError(t, err, "CompareAndSwapHeadlessAuthentication returned non-nil error")

			retrieved, err := identity.GetHeadlessAuthentication(ctx, test.ha.Metadata.Name)
			require.NoError(t, err, "GetHeadlessAuthentication returned non-nil error")
			require.Equal(t, swapped, retrieved)

			retrievedList, err := identity.GetHeadlessAuthentications(ctx)
			require.NoError(t, err, "GetHeadlessAuthentications returned non-nil error")
			require.Equal(t, []*types.HeadlessAuthentication{swapped}, retrievedList)
		})
	}
}

// sshPubKey is a randomly-generated public key used for login tests.
const sshPubKey = `ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBGv+gN2C23P08ieJRA9gU/Ik4bsOh3Kw193UYscJDw41mATj+Kqyf45Rmj8F8rs3i7mYKRXXu1IjNRBzNgpXxqc=`
