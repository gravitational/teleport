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

package decision_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	traitpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/trait/v1"
	"github.com/gravitational/teleport/lib/decision"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestTLSIdentity_roundtrip(t *testing.T) {
	t.Parallel()

	minimalTLSIdentity := &decisionpb.TLSIdentity{
		// tlsca.Identity has no pointer fields, so these are always non-nil after
		// copying.
		RouteToApp:       &decisionpb.RouteToApp{},
		RouteToDatabase:  &decisionpb.RouteToDatabase{},
		DeviceExtensions: &decisionpb.DeviceExtensions{},
	}

	fullIdentity := &decisionpb.TLSIdentity{
		Username:          "user",
		Impersonator:      "impersonator",
		Groups:            []string{"role1", "role2"},
		SystemRoles:       []string{"system1", "system2"},
		Usage:             []string{"usage1", "usage2"},
		Principals:        []string{"login1", "login2"},
		KubernetesGroups:  []string{"kgroup1", "kgroup2"},
		KubernetesUsers:   []string{"kuser1", "kuser2"},
		Expires:           timestamppb.Now(),
		RouteToCluster:    "route-to-cluster",
		KubernetesCluster: "k8s-cluster",
		Traits: []*traitpb.Trait{
			// Note: sorted by key on conversion.
			{Key: "", Values: []string{"missingkey"}},
			{Key: "missingvalues", Values: nil},
			{Key: "trait1", Values: []string{"val1"}},
			{Key: "trait2", Values: []string{"val1", "val2"}},
		},
		RouteToApp: &decisionpb.RouteToApp{
			SessionId:         "session-id",
			PublicAddr:        "public-addr",
			ClusterName:       "cluster-name",
			Name:              "name",
			AwsRoleArn:        "aws-role-arn",
			AzureIdentity:     "azure-id",
			GcpServiceAccount: "gcp-service-account",
			Uri:               "uri",
			TargetPort:        111,
		},
		TeleportCluster: "teleport-cluster",
		RouteToDatabase: &decisionpb.RouteToDatabase{
			ServiceName: "service-name",
			Protocol:    "protocol",
			Username:    "username",
			Database:    "database",
			Roles:       []string{"role1", "role2"},
		},
		DatabaseNames:           []string{"db1", "db2"},
		DatabaseUsers:           []string{"dbuser1", "dbuser2"},
		MfaVerified:             "mfa-device-id",
		PreviousIdentityExpires: timestamppb.Now(),
		LoginIp:                 "login-ip",
		PinnedIp:                "pinned-ip",
		AwsRoleArns:             []string{"arn1", "arn2"},
		AzureIdentities:         []string{"azure-id-1", "azure-id-2"},
		GcpServiceAccounts:      []string{"gcp-account-1", "gcp-account-2"},
		ActiveRequests:          []string{"accessrequest1", "accessrequest2"},
		DisallowReissue:         true,
		Renewable:               true,
		Generation:              112,
		BotName:                 "bot-name",
		BotInstanceId:           "bot-instance-id",
		AllowedResourceIds: []*decisionpb.ResourceId{
			{
				ClusterName:     "cluster1",
				Kind:            "kind1",
				Name:            "name1",
				SubResourceName: "sub-resource1",
			},
			{
				ClusterName:     "cluster2",
				Kind:            "kind2",
				Name:            "name2",
				SubResourceName: "sub-resource2",
			},
		},
		PrivateKeyPolicy:       "private-key-policy",
		ConnectionDiagnosticId: "connection-diag-id",
		DeviceExtensions: &decisionpb.DeviceExtensions{
			DeviceId:     "device-id",
			AssetTag:     "asset-tag",
			CredentialId: "credential-id",
		},
		UserType: "user-type",
	}

	tests := []struct {
		name        string
		start, want *decisionpb.TLSIdentity
	}{
		{
			name:  "nil-to-nil",
			start: nil,
			want:  nil,
		},
		{
			name:  "zero-to-zero",
			start: &decisionpb.TLSIdentity{},
			want:  minimalTLSIdentity,
		},
		{
			name:  "full identity",
			start: fullIdentity,
			want:  fullIdentity,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := decision.TLSIdentityFromTLSCA(
				decision.TLSIdentityToTLSCA(test.start),
			)
			if diff := cmp.Diff(test.want, got, protocmp.Transform()); diff != "" {
				t.Errorf("TLSIdentity conversion mismatch (-want +got)\n%s", diff)
			}
		})
	}

	t.Run("zero tlsca.Identity", func(t *testing.T) {
		var id tlsca.Identity
		got := decision.TLSIdentityFromTLSCA(&id)
		want := minimalTLSIdentity
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Errorf("TLSIdentity conversion mismatch (-want +got)\n%s", diff)
		}
	})
}

func TestTLSIdentityToTLSCA_zeroTimestamp(t *testing.T) {
	t.Parallel()

	id := decision.TLSIdentityToTLSCA(&decisionpb.TLSIdentity{
		Expires:                 &timestamppb.Timestamp{},
		PreviousIdentityExpires: &timestamppb.Timestamp{},
	})
	assert.Zero(t, id.Expires, "id.Expires")
	assert.Zero(t, id.PreviousIdentityExpires, "id.PreviousIdentityExpires")
}
