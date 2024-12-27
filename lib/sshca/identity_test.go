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

// Package sshca specifies interfaces for SSH certificate authorities
package sshca

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/services"
	"github.com/stretchr/testify/require"
)

func TestIdentityConversion(t *testing.T) {
	ident := &Identity{
		ValidAfter:            1,
		ValidBefore:           2,
		Username:              "user",
		Impersonator:          "impersonator",
		AllowedLogins:         []string{"login1", "login2"},
		PermitX11Forwarding:   true,
		PermitAgentForwarding: true,
		PermitPortForwarding:  true,
		Roles:                 []string{"role1", "role2"},
		RouteToCluster:        "cluster",
		Traits:                wrappers.Traits{"trait1": []string{"value1"}, "trait2": []string{"value2"}},
		ActiveRequests: services.RequestIDs{
			AccessRequests: []string{uuid.NewString()},
		},
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
		Renewable:              true,
		Generation:             3,
		BotName:                "bot",
		BotInstanceID:          "instance",
		AllowedResourceIDs:     "resource",
		ConnectionDiagnosticID: "diag",
		PrivateKeyPolicy:       keys.PrivateKeyPolicy("policy"),
		DeviceID:               "device",
		DeviceAssetTag:         "asset",
		DeviceCredentialID:     "cred",
		GitHubUserID:           "github",
		GitHubUsername:         "ghuser",
	}

	cert, err := ident.Encode(constants.CertificateFormatStandard)
	require.NoError(t, err)

	ident2, err := DecodeIdentity(cert)
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(ident, ident2))
}
