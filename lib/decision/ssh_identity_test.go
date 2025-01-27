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

package decision

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/utils/testutils"
)

func TestSSHIdentityConversion(t *testing.T) {
	ident := &sshca.Identity{
		ValidAfter:              1,
		ValidBefore:             2,
		CertType:                ssh.UserCert,
		ClusterName:             "some-cluster",
		SystemRole:              types.RoleNode,
		Username:                "user",
		Impersonator:            "impersonator",
		Principals:              []string{"login1", "login2"},
		PermitX11Forwarding:     true,
		PermitAgentForwarding:   true,
		PermitPortForwarding:    true,
		Roles:                   []string{"role1", "role2"},
		RouteToCluster:          "cluster",
		Traits:                  wrappers.Traits{"trait1": []string{"value1"}, "trait2": []string{"value2"}},
		ActiveRequests:          []string{uuid.NewString()},
		MFAVerified:             "mfa",
		PreviousIdentityExpires: time.Unix(12345, 0),
		LoginIP:                 "127.0.0.1",
		PinnedIP:                "127.0.0.1",
		DisallowReissue:         true,
		CertificateExtensions: []*types.CertExtension{&types.CertExtension{
			Name:  "extname",
			Value: "extvalue",
			Type:  types.CertExtensionType_SSH,
			Mode:  types.CertExtensionMode_EXTENSION,
		}},
		Renewable:  true,
		Generation: 3,
		BotName:    "bot",
		AllowedResourceIDs: []types.ResourceID{{
			ClusterName:     "cluster",
			Kind:            types.KindKubePod, // must use a kube resource kind for parsing of sub-resource to work correctly
			Name:            "name",
			SubResourceName: "sub/sub",
		}},
		ConnectionDiagnosticID: "diag",
		PrivateKeyPolicy:       keys.PrivateKeyPolicy("policy"),
		DeviceID:               "device",
		DeviceAssetTag:         "asset",
		DeviceCredentialID:     "cred",
	}

	ignores := []string{
		"CertExtension.Type", // only currently defined enum variant is a zero value
		"CertExtension.Mode", // only currently defined enum variant is a zero value
		// TODO(fspmarshall): figure out a mechanism for making ignore of grpc fields more convenient
		"CertExtension.XXX_NoUnkeyedLiteral",
		"CertExtension.XXX_unrecognized",
		"CertExtension.XXX_sizecache",
		"ResourceID.XXX_NoUnkeyedLiteral",
		"ResourceID.XXX_unrecognized",
		"ResourceID.XXX_sizecache",
	}

	require.True(t, testutils.ExhaustiveNonEmpty(ident, ignores...), "empty=%+v", testutils.FindAllEmpty(ident, ignores...))

	proto := SSHIdentityFromSSHCA(ident)

	ident2 := SSHIdentityToSSHCA(proto)

	require.Empty(t, cmp.Diff(ident, ident2))
}
