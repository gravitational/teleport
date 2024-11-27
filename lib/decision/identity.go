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

package decision

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	decision "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/tlsca"
)

func IdentityToProto(identity tlsca.Identity) *decision.Identity {
	return &decision.Identity{
		Username:                identity.Username,
		Impersonator:            identity.Impersonator,
		Groups:                  identity.Groups,
		SystemRoles:             identity.SystemRoles,
		Usage:                   identity.Usage,
		Principals:              identity.Principals,
		KubernetesGroups:        identity.KubernetesGroups,
		KubernetesUsers:         identity.KubernetesUsers,
		Expires:                 timeToProto(identity.Expires),
		RouteToCluster:          identity.RouteToCluster,
		KubernetesCluster:       identity.KubernetesCluster,
		Traits:                  traitsToProto(identity.Traits),
		RouteToApp:              routeToAppToProto(identity.RouteToApp),
		TeleportCluster:         identity.TeleportCluster,
		RouteToDatabase:         routeToDatabaseToProto(identity.RouteToDatabase),
		DatabaseNames:           identity.DatabaseNames,
		DatabaseUsers:           identity.DatabaseUsers,
		MfaVerified:             identity.MFAVerified,
		PreviousIdentityExpires: timeToProto(identity.PreviousIdentityExpires),
		LoginIp:                 identity.LoginIP,
		PinnedIp:                identity.PinnedIP,
		AwsRoleArns:             identity.AWSRoleARNs,
		AzureIdentities:         identity.AzureIdentities,
		GcpServiceAccounts:      identity.GCPServiceAccounts,
		ActiveRequests:          identity.ActiveRequests,
		DisallowReissue:         identity.DisallowReissue,
		Renewable:               identity.Renewable,
		Generation:              identity.Generation,
		BotName:                 identity.BotName,
		BotInstanceId:           identity.BotInstanceID,
		AllowedResourceIds:      resourceIDsToProto(identity.AllowedResourceIDs),
		PrivateKeyPolicy:        string(identity.PrivateKeyPolicy),
		ConnectionDiagnosticId:  identity.ConnectionDiagnosticID,
		DeviceExtensions:        deviceExtensionsToProto(identity.DeviceExtensions),
		UserType:                string(identity.UserType),
	}
}

func IdentityFromProto(identity *decision.Identity) tlsca.Identity {
	return tlsca.Identity{
		Username:                identity.Username,
		Impersonator:            identity.Impersonator,
		Groups:                  identity.Groups,
		SystemRoles:             identity.SystemRoles,
		Usage:                   identity.Usage,
		Principals:              identity.Principals,
		KubernetesGroups:        identity.KubernetesGroups,
		KubernetesUsers:         identity.KubernetesUsers,
		Expires:                 timeFromProto(identity.Expires),
		RouteToCluster:          identity.RouteToCluster,
		KubernetesCluster:       identity.KubernetesCluster,
		Traits:                  traitsFromProto(identity.Traits),
		RouteToApp:              routeToAppFromProto(identity.RouteToApp),
		TeleportCluster:         identity.TeleportCluster,
		RouteToDatabase:         routeToDatabaseFromProto(identity.RouteToDatabase),
		DatabaseNames:           identity.DatabaseNames,
		DatabaseUsers:           identity.DatabaseUsers,
		MFAVerified:             identity.MfaVerified,
		PreviousIdentityExpires: timeFromProto(identity.PreviousIdentityExpires),
		LoginIP:                 identity.LoginIp,
		PinnedIP:                identity.PinnedIp,
		AWSRoleARNs:             identity.AwsRoleArns,
		AzureIdentities:         identity.AzureIdentities,
		GCPServiceAccounts:      identity.GcpServiceAccounts,
		ActiveRequests:          identity.ActiveRequests,
		DisallowReissue:         identity.DisallowReissue,
		Renewable:               identity.Renewable,
		Generation:              identity.Generation,
		BotName:                 identity.BotName,
		BotInstanceID:           identity.BotInstanceId,
		AllowedResourceIDs:      resourceIDsFromProto(identity.AllowedResourceIds),
		PrivateKeyPolicy:        keys.PrivateKeyPolicy(identity.PrivateKeyPolicy),
		ConnectionDiagnosticID:  identity.ConnectionDiagnosticId,
		DeviceExtensions:        deviceExtensionsFromProto(identity.DeviceExtensions),
		UserType:                types.UserType(identity.UserType),
	}
}

func deviceExtensionsToProto(deviceExtensions tlsca.DeviceExtensions) *decision.DeviceExtensions {
	if deviceExtensions.IsZero() {
		return nil
	}
	return &decision.DeviceExtensions{
		DeviceId:     deviceExtensions.DeviceID,
		AssetTag:     deviceExtensions.AssetTag,
		CredentialId: deviceExtensions.CredentialID,
	}
}

func deviceExtensionsFromProto(deviceExtensions *decision.DeviceExtensions) tlsca.DeviceExtensions {
	if deviceExtensions == nil {
		return tlsca.DeviceExtensions{}
	}
	return tlsca.DeviceExtensions{
		DeviceID:     deviceExtensions.DeviceId,
		AssetTag:     deviceExtensions.AssetTag,
		CredentialID: deviceExtensions.CredentialId,
	}
}

func resourceIDsToProto(resourceIDs []types.ResourceID) []*decision.ResourceID {
	if len(resourceIDs) == 0 {
		return nil
	}

	out := make([]*decision.ResourceID, len(resourceIDs))
	for i, resourceID := range resourceIDs {
		out[i] = &decision.ResourceID{
			ClusterName:     resourceID.ClusterName,
			Kind:            resourceID.Kind,
			Name:            resourceID.Name,
			SubResourceName: resourceID.SubResourceName,
		}
	}
	return out
}

func resourceIDsFromProto(resourceIDs []*decision.ResourceID) []types.ResourceID {
	if len(resourceIDs) == 0 {
		return nil
	}

	out := make([]types.ResourceID, len(resourceIDs))
	for i, resourceID := range resourceIDs {
		out[i] = types.ResourceID{
			ClusterName:     resourceID.ClusterName,
			Kind:            resourceID.Kind,
			Name:            resourceID.Name,
			SubResourceName: resourceID.SubResourceName,
		}
	}
	return out
}

func routeToDatabaseToProto(routeToDatabase tlsca.RouteToDatabase) *decision.RouteToDatabase {
	if routeToDatabase.Empty() {
		return nil
	}
	return &decision.RouteToDatabase{
		ServiceName: routeToDatabase.ServiceName,
		Protocol:    routeToDatabase.Protocol,
		Username:    routeToDatabase.Username,
		Database:    routeToDatabase.Database,
		Roles:       routeToDatabase.Roles,
	}
}

func routeToDatabaseFromProto(routeToDatabase *decision.RouteToDatabase) tlsca.RouteToDatabase {
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

func routeToAppToProto(routeToApp tlsca.RouteToApp) *decision.RouteToApp {
	if routeToApp.IsZero() {
		return nil
	}
	return &decision.RouteToApp{
		SessionId:         routeToApp.SessionID,
		PublicAddr:        routeToApp.PublicAddr,
		ClusterName:       routeToApp.ClusterName,
		Name:              routeToApp.Name,
		AwsRoleArn:        routeToApp.AWSRoleARN,
		AzureIdentity:     routeToApp.AzureIdentity,
		GcpServiceAccount: routeToApp.GCPServiceAccount,
		Uri:               routeToApp.URI,
	}
}

func routeToAppFromProto(routeToApp *decision.RouteToApp) tlsca.RouteToApp {
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
	}
}

func traitsToProto(traits wrappers.Traits) []*decision.Trait {
	if traits == nil {
		return nil
	}

	out := make([]*decision.Trait, 0, len(traits))
	for key, values := range traits {
		out = append(out, &decision.Trait{
			Name:   key,
			Values: values,
		})
	}
	return out
}

func traitsFromProto(traits []*decision.Trait) wrappers.Traits {
	if traits == nil {
		return nil
	}

	out := make(wrappers.Traits, len(traits))
	for _, trait := range traits {
		out[trait.Name] = trait.Values
	}
	return out
}

func timeToProto(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}

	return timestamppb.New(t)
}

func timeFromProto(t *timestamppb.Timestamp) time.Time {
	if t == nil {
		return time.Time{}
	}

	return t.AsTime()
}
