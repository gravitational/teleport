// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	traitpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/trait/v1"
	"github.com/gravitational/teleport/api/types"
	apitrait "github.com/gravitational/teleport/api/types/trait"
	apitraitconvert "github.com/gravitational/teleport/api/types/trait/convert/v1"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/tlsca"
)

// TLSIdentityToTLSCA transforms a [decisionpb.TLSIdentity] into its
// equivalent [tlsca.Identity].
// Note that certain types, like slices, are not deep-copied.
func TLSIdentityToTLSCA(id *decisionpb.TLSIdentity) *tlsca.Identity {
	if id == nil {
		return nil
	}

	return &tlsca.Identity{
		Username:                id.Username,
		Impersonator:            id.Impersonator,
		Groups:                  id.Groups,
		SystemRoles:             id.SystemRoles,
		Usage:                   id.Usage,
		Principals:              id.Principals,
		KubernetesGroups:        id.KubernetesGroups,
		KubernetesUsers:         id.KubernetesUsers,
		Expires:                 timestampToGoTime(id.Expires),
		RouteToCluster:          id.RouteToCluster,
		KubernetesCluster:       id.KubernetesCluster,
		Traits:                  traitToWrappers(id.Traits),
		RouteToApp:              routeToAppFromProto(id.RouteToApp),
		TeleportCluster:         id.TeleportCluster,
		RouteToDatabase:         routeToDatabaseFromProto(id.RouteToDatabase),
		DatabaseNames:           id.DatabaseNames,
		DatabaseUsers:           id.DatabaseUsers,
		MFAVerified:             id.MfaVerified,
		PreviousIdentityExpires: timestampToGoTime(id.PreviousIdentityExpires),
		LoginIP:                 id.LoginIp,
		PinnedIP:                id.PinnedIp,
		AWSRoleARNs:             id.AwsRoleArns,
		AzureIdentities:         id.AzureIdentities,
		GCPServiceAccounts:      id.GcpServiceAccounts,
		ActiveRequests:          id.ActiveRequests,
		DisallowReissue:         id.DisallowReissue,
		Renewable:               id.Renewable,
		Generation:              id.Generation,
		BotName:                 id.BotName,
		BotInstanceID:           id.BotInstanceId,
		AllowedResourceIDs:      resourceIDsToTypes(id.AllowedResourceIds),
		PrivateKeyPolicy:        keys.PrivateKeyPolicy(id.PrivateKeyPolicy),
		ConnectionDiagnosticID:  id.ConnectionDiagnosticId,
		DeviceExtensions:        deviceExtensionsFromProto(id.DeviceExtensions),
		UserType:                types.UserType(id.UserType),
	}
}

// TLSIdentityFromTLSCA transforms a [tlsca.Identity] into its equivalent
// [decisionpb.TLSIdentity].
// Note that certain types, like slices, are not deep-copied.
func TLSIdentityFromTLSCA(id *tlsca.Identity) *decisionpb.TLSIdentity {
	if id == nil {
		return nil
	}

	return &decisionpb.TLSIdentity{
		Username:                id.Username,
		Impersonator:            id.Impersonator,
		Groups:                  id.Groups,
		SystemRoles:             id.SystemRoles,
		Usage:                   id.Usage,
		Principals:              id.Principals,
		KubernetesGroups:        id.KubernetesGroups,
		KubernetesUsers:         id.KubernetesUsers,
		Expires:                 timestampFromGoTime(id.Expires),
		RouteToCluster:          id.RouteToCluster,
		KubernetesCluster:       id.KubernetesCluster,
		Traits:                  traitFromWrappers(id.Traits),
		RouteToApp:              routeToAppToProto(&id.RouteToApp),
		TeleportCluster:         id.TeleportCluster,
		RouteToDatabase:         routeToDatabaseToProto(&id.RouteToDatabase),
		DatabaseNames:           id.DatabaseNames,
		DatabaseUsers:           id.DatabaseUsers,
		MfaVerified:             id.MFAVerified,
		PreviousIdentityExpires: timestampFromGoTime(id.PreviousIdentityExpires),
		LoginIp:                 id.LoginIP,
		PinnedIp:                id.PinnedIP,
		AwsRoleArns:             id.AWSRoleARNs,
		AzureIdentities:         id.AzureIdentities,
		GcpServiceAccounts:      id.GCPServiceAccounts,
		ActiveRequests:          id.ActiveRequests,
		DisallowReissue:         id.DisallowReissue,
		Renewable:               id.Renewable,
		Generation:              id.Generation,
		BotName:                 id.BotName,
		BotInstanceId:           id.BotInstanceID,
		AllowedResourceIds:      resourceIDsFromTypes(id.AllowedResourceIDs),
		PrivateKeyPolicy:        string(id.PrivateKeyPolicy),
		ConnectionDiagnosticId:  id.ConnectionDiagnosticID,
		DeviceExtensions:        deviceExtensionsToProto(&id.DeviceExtensions),
		UserType:                string(id.UserType),
	}
}

func timestampToGoTime(t *timestamppb.Timestamp) time.Time {
	// nil or "zero" Timestamps are mapped to Go's zero time (0-0-0 0:0.0) instead
	// of unix epoch. The latter avoids problems with tooling (eg, Terraform) that
	// sets structs to their defaults instead of using nil.
	if t == nil || (t.Seconds == 0 && t.Nanos == 0) {
		return time.Time{}
	}
	return t.AsTime()
}

func timestampFromGoTime(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

func traitToWrappers(traits []*traitpb.Trait) wrappers.Traits {
	apiTraits := apitraitconvert.FromProto(traits)
	return wrappers.Traits(apiTraits)
}

func traitFromWrappers(traits wrappers.Traits) []*traitpb.Trait {
	if len(traits) == 0 {
		return nil
	}
	apiTraits := apitrait.Traits(traits)
	return apitraitconvert.ToProto(apiTraits)
}

func routeToAppFromProto(routeToApp *decisionpb.RouteToApp) tlsca.RouteToApp {
	if routeToApp == nil {
		return tlsca.RouteToApp{}
	}

	return tlsca.RouteToApp{
		SessionID:         routeToApp.SessionId,
		PublicAddr:        routeToApp.PublicAddr,
		ClusterName:       routeToApp.ClusterName,
		Name:              routeToApp.Name,
		AWSRoleARN:        routeToApp.AwsRoleArn,
		AzureIdentity:     routeToApp.AzureIdentity,
		GCPServiceAccount: routeToApp.GcpServiceAccount,
		URI:               routeToApp.Uri,
		TargetPort:        int(routeToApp.TargetPort),
	}
}

func routeToAppToProto(routeToApp *tlsca.RouteToApp) *decisionpb.RouteToApp {
	if routeToApp == nil {
		return nil
	}

	return &decisionpb.RouteToApp{
		SessionId:         routeToApp.SessionID,
		PublicAddr:        routeToApp.PublicAddr,
		ClusterName:       routeToApp.ClusterName,
		Name:              routeToApp.Name,
		AwsRoleArn:        routeToApp.AWSRoleARN,
		AzureIdentity:     routeToApp.AzureIdentity,
		GcpServiceAccount: routeToApp.GCPServiceAccount,
		Uri:               routeToApp.URI,
		TargetPort:        int32(routeToApp.TargetPort),
	}
}

func routeToDatabaseFromProto(routeToDatabase *decisionpb.RouteToDatabase) tlsca.RouteToDatabase {
	if routeToDatabase == nil {
		return tlsca.RouteToDatabase{}
	}

	return tlsca.RouteToDatabase{
		ServiceName: routeToDatabase.ServiceName,
		Protocol:    routeToDatabase.Protocol,
		Username:    routeToDatabase.Username,
		Database:    routeToDatabase.Database,
		Roles:       routeToDatabase.Roles,
	}
}

func routeToDatabaseToProto(routeToDatabase *tlsca.RouteToDatabase) *decisionpb.RouteToDatabase {
	if routeToDatabase == nil {
		return nil
	}

	return &decisionpb.RouteToDatabase{
		ServiceName: routeToDatabase.ServiceName,
		Protocol:    routeToDatabase.Protocol,
		Username:    routeToDatabase.Username,
		Database:    routeToDatabase.Database,
		Roles:       routeToDatabase.Roles,
	}
}

func resourceIDsToTypes(resourceIDs []*decisionpb.ResourceId) []types.ResourceID {
	if len(resourceIDs) == 0 {
		return nil
	}

	ret := make([]types.ResourceID, len(resourceIDs))
	for i, r := range resourceIDs {
		ret[i] = types.ResourceID{
			ClusterName:     r.ClusterName,
			Kind:            r.Kind,
			Name:            r.Name,
			SubResourceName: r.SubResourceName,
		}
	}
	return ret
}

func resourceIDsFromTypes(resourceIDs []types.ResourceID) []*decisionpb.ResourceId {
	if len(resourceIDs) == 0 {
		return nil
	}

	ret := make([]*decisionpb.ResourceId, len(resourceIDs))
	for i, r := range resourceIDs {
		ret[i] = &decisionpb.ResourceId{
			ClusterName:     r.ClusterName,
			Kind:            r.Kind,
			Name:            r.Name,
			SubResourceName: r.SubResourceName,
		}
	}
	return ret
}

func deviceExtensionsFromProto(exts *decisionpb.DeviceExtensions) tlsca.DeviceExtensions {
	if exts == nil {
		return tlsca.DeviceExtensions{}
	}

	return tlsca.DeviceExtensions{
		DeviceID:     exts.DeviceId,
		AssetTag:     exts.AssetTag,
		CredentialID: exts.CredentialId,
	}
}

func deviceExtensionsToProto(exts *tlsca.DeviceExtensions) *decisionpb.DeviceExtensions {
	if exts == nil {
		return nil
	}

	return &decisionpb.DeviceExtensions{
		DeviceId:     exts.DeviceID,
		AssetTag:     exts.AssetTag,
		CredentialId: exts.CredentialID,
	}
}
