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

package services_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

// TestValidateHeadlessAuthentication tests headless authentication validation logic.
func TestValidateHeadlessAuthentication(t *testing.T) {
	t.Parallel()

	pubUUID := services.NewHeadlessAuthenticationID([]byte(sshPubKey))
	expires := time.Now().Add(time.Minute)

	newHA := func(modify func(*types.HeadlessAuthentication)) *types.HeadlessAuthentication {
		ha := &types.HeadlessAuthentication{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Name:    pubUUID,
					Expires: &expires,
				},
			},
			State:        types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_PENDING,
			User:         "user",
			SshPublicKey: []byte(sshPubKey),
			TlsPublicKey: []byte(tlsPubKey),
		}
		if modify != nil {
			modify(ha)
		}
		return ha
	}

	tests := []struct {
		name      string
		ha        *types.HeadlessAuthentication
		wantErr   string
		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "OK valid headless authentication",
			ha:   newHA(nil),
		}, {
			name: "NOK name missing",
			ha: newHA(func(ha *types.HeadlessAuthentication) {
				ha.SetName("")
			}),
			wantErr: "missing parameter Name",
		}, {
			name: "NOK name not derived from public key",
			ha: newHA(func(ha *types.HeadlessAuthentication) {
				// use a random UUID instead of the uuid.NewHash of the public key used above.
				ha.SetName(uuid.NewString())
			}),
			wantErr: "headless authentication authentication resource name must be derived from public key",
		}, {
			name: "NOK expires missing",
			ha: newHA(func(ha *types.HeadlessAuthentication) {
				ha.SetExpiry(time.Time{})
			}),
			wantErr: "headless authentication resource must have non-zero header.metadata.expires",
		}, {
			name: "NOK username missing",
			ha: newHA(func(ha *types.HeadlessAuthentication) {
				ha.User = ""
			}),
			wantErr: "headless authentication resource must have non-empty user",
		}, {
			name: "NOK state not specified",
			ha: newHA(func(ha *types.HeadlessAuthentication) {
				ha.State = types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_UNSPECIFIED
			}),
			wantErr: "headless authentication resource state must be specified",
		}, {
			name: "NOK SSH public key missing",
			ha: newHA(func(ha *types.HeadlessAuthentication) {
				ha.SshPublicKey = nil
			}),
			wantErr: "headless authentication resource must have non-empty SSH public key",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := services.ValidateHeadlessAuthentication(test.ha)
			if test.wantErr == "" {
				require.NoError(t, err, "ValidateHeadlessAuthentication errored unexpectedly")
				return
			}
			require.True(t, trace.IsBadParameter(err), "ValidateHeadlessAuthentication returned non-BadParameter error: %v", err)
			require.ErrorContains(t, err, test.wantErr, "ValidateHeadlessAuthentication error mismatch")
		})
	}
}

// sshPubKey is a randomly-generated public key used for login tests.
const sshPubKey = `ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBGv+gN2C23P08ieJRA9gU/Ik4bsOh3Kw193UYscJDw41mATj+Kqyf45Rmj8F8rs3i7mYKRXXu1IjNRBzNgpXxqc=`

// tlsPubKey is a randomly-generated public key used for login tests.
const tlsPubKey = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE/Jn3tYhc60M2IOen1yRht6r8xX3h
v7nNLYBIfxaKxXf+dAFVllYzVUrSzAQxi1LSAplOJVgOtHv0J69dRSUSzA==
-----END PUBLIC KEY-----`
