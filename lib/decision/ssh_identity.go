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
	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/sshca"
)

// SSHIdentityToSSHCA transforms a [decisionpb.SSHIdentity] into its
// equivalent [sshca.Identity].
// Note that certain types, like slices, are not deep-copied.
func SSHIdentityToSSHCA(id *decisionpb.SSHIdentity) *sshca.Identity {
	if id == nil {
		return nil
	}

	return &sshca.Identity{
		ValidAfter:              id.ValidAfter,
		ValidBefore:             id.ValidBefore,
		CertType:                id.CertType,
		ClusterName:             id.ClusterName,
		SystemRole:              types.SystemRole(id.SystemRole),
		Username:                id.Username,
		Impersonator:            id.Impersonator,
		Principals:              id.Principals,
		PermitX11Forwarding:     id.PermitX11Forwarding,
		PermitAgentForwarding:   id.PermitAgentForwarding,
		PermitPortForwarding:    id.PermitPortForwarding,
		Roles:                   id.Roles,
		RouteToCluster:          id.RouteToCluster,
		Traits:                  traitToWrappers(id.Traits),
		ActiveRequests:          id.ActiveRequests,
		MFAVerified:             id.MfaVerified,
		PreviousIdentityExpires: timestampToGoTime(id.PreviousIdentityExpires),
		LoginIP:                 id.LoginIp,
		PinnedIP:                id.PinnedIp,
		DisallowReissue:         id.DisallowReissue,
		CertificateExtensions:   certExtensionsFromProto(id.CertificateExtensions),
		Renewable:               id.Renewable,
		Generation:              id.Generation,
		BotName:                 id.BotName,
		BotInstanceID:           id.BotInstanceId,
		AllowedResourceIDs:      resourceIDsToTypes(id.AllowedResourceIds),
		ConnectionDiagnosticID:  id.ConnectionDiagnosticId,
		PrivateKeyPolicy:        keys.PrivateKeyPolicy(id.PrivateKeyPolicy),
		DeviceID:                id.DeviceId,
		DeviceAssetTag:          id.DeviceAssetTag,
		DeviceCredentialID:      id.DeviceCredentialId,
		GitHubUserID:            id.GithubUserId,
		GitHubUsername:          id.GithubUsername,
	}
}

func SSHIdentityFromSSHCA(id *sshca.Identity) *decisionpb.SSHIdentity {
	if id == nil {
		return nil
	}

	return &decisionpb.SSHIdentity{
		ValidAfter:              id.ValidAfter,
		ValidBefore:             id.ValidBefore,
		CertType:                id.CertType,
		ClusterName:             id.ClusterName,
		SystemRole:              string(id.SystemRole),
		Username:                id.Username,
		Impersonator:            id.Impersonator,
		Principals:              id.Principals,
		PermitX11Forwarding:     id.PermitX11Forwarding,
		PermitAgentForwarding:   id.PermitAgentForwarding,
		PermitPortForwarding:    id.PermitPortForwarding,
		Roles:                   id.Roles,
		RouteToCluster:          id.RouteToCluster,
		Traits:                  traitFromWrappers(id.Traits),
		ActiveRequests:          id.ActiveRequests,
		MfaVerified:             id.MFAVerified,
		PreviousIdentityExpires: timestampFromGoTime(id.PreviousIdentityExpires),
		LoginIp:                 id.LoginIP,
		PinnedIp:                id.PinnedIP,
		DisallowReissue:         id.DisallowReissue,
		CertificateExtensions:   certExtensionsToProto(id.CertificateExtensions),
		Renewable:               id.Renewable,
		Generation:              id.Generation,
		BotName:                 id.BotName,
		BotInstanceId:           id.BotInstanceID,
		AllowedResourceIds:      resourceIDsFromTypes(id.AllowedResourceIDs),
		ConnectionDiagnosticId:  id.ConnectionDiagnosticID,
		PrivateKeyPolicy:        string(id.PrivateKeyPolicy),
		DeviceId:                id.DeviceID,
		DeviceAssetTag:          id.DeviceAssetTag,
		DeviceCredentialId:      id.DeviceCredentialID,
		GithubUserId:            id.GitHubUserID,
		GithubUsername:          id.GitHubUsername,
	}
}

func certExtensionsFromProto(extensions []*decisionpb.CertExtension) []*types.CertExtension {
	if len(extensions) == 0 {
		return nil
	}
	out := make([]*types.CertExtension, 0, len(extensions))
	for _, extension := range extensions {
		out = append(out, &types.CertExtension{
			Mode:  types.CertExtensionMode(int32(extension.Mode) - 1), // enum is equivalent but off by 1
			Type:  types.CertExtensionType(int32(extension.Type) - 1), // enum is equivalent but off by 1
			Name:  extension.Name,
			Value: extension.Value,
		})
	}
	return out
}

func certExtensionsToProto(extensions []*types.CertExtension) []*decisionpb.CertExtension {
	if len(extensions) == 0 {
		return nil
	}
	out := make([]*decisionpb.CertExtension, 0, len(extensions))
	for _, extension := range extensions {
		out = append(out, &decisionpb.CertExtension{
			Mode:  decisionpb.CertExtensionMode(int32(extension.Mode) + 1), // enum is equivalent but off by 1
			Type:  decisionpb.CertExtensionType(int32(extension.Type) + 1), // enum is equivalent but off by 1
			Name:  extension.Name,
			Value: extension.Value,
		})
	}
	return out
}
