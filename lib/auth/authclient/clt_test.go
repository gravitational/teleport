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

package authclient

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/fixtures"
)

func fakeCA(t *testing.T, caType types.CertAuthType) types.CertAuthority {
	t.Helper()
	ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        caType,
		ClusterName: "fizz-buzz",
		ActiveKeys: types.CAKeySet{
			TLS: []*types.TLSKeyPair{
				{
					Cert: []byte(fixtures.TLSCACertPEM),
					Key:  []byte(fixtures.TLSCAKeyPEM),
				},
			},
			SSH: []*types.SSHKeyPair{
				// Two of these to ensure that both are written to known hosts
				{
					PrivateKey: []byte(fixtures.SSHCAPrivateKey),
					PublicKey:  []byte(fixtures.SSHCAPublicKey),
				},
				{
					PrivateKey: []byte(fixtures.SSHCAPrivateKey),
					PublicKey:  []byte(fixtures.SSHCAPublicKey),
				},
			},
		},
	})
	require.NoError(t, err)
	return ca
}

func TestValidateTrustedClusterRequestProto(t *testing.T) {
	native := &ValidateTrustedClusterRequest{
		Token:           "fizz-buzz",
		TeleportVersion: "v19.0.0",
		CAs: []types.CertAuthority{
			fakeCA(t, types.HostCA),
			fakeCA(t, types.UserCA),
		},
	}
	proto, err := native.ToProto()
	require.NoError(t, err)
	backToNative := ValidateTrustedClusterRequestFromProto(proto)
	require.Empty(t, cmp.Diff(native, backToNative))
}

func TestValidateTrustedClusterResponseProto(t *testing.T) {
	native := &ValidateTrustedClusterResponse{
		CAs: []types.CertAuthority{
			fakeCA(t, types.HostCA),
			fakeCA(t, types.UserCA),
		},
	}
	proto, err := native.ToProto()
	require.NoError(t, err)
	backToNative := ValidateTrustedClusterResponseFromProto(proto)
	require.Empty(t, cmp.Diff(native, backToNative))
}
