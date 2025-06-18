// Copyright 2025 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"log"
	"log/slog"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/trait"
)

func main() {
	ctx := context.Background()

	if err := do(ctx); err != nil {
		log.Fatalf("failed to run: %v", err)
	}
}

func buildRequest() (*SyncRequest, error) {
	// Here you would typically use a flag package to parse command line arguments
	// For simplicity, we are returning a hardcoded request
	return &SyncRequest{
		RoleForEKSAccessWithTrait: "eks-access",

		AWSAccountID: "123456789012",

		EntraTeam: "de24889e-2cea-4ac5-b43a-5d86ce394816", // replace with your Entra Team ID

		AccessListUniqueID:  "eks-access-123456789012",
		AccessListTitle:     "EKS Access for 123456789012",
		AccessListOwnerUser: "system",
	}, nil
}

func do(ctx context.Context) error {
	// Load request from flags or environment variables.
	request, err := buildRequest()
	if err != nil {
		return trace.Wrap(err)
	}

	// Set up Connection to Teleport
	clt, err := loadTeleportClient(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer clt.Close()

	resp, err := clt.Ping(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	slog.DebugContext(ctx, "Connected to Teleport",
		"cluster_name", resp.ClusterName,
		"server_version", resp.ServerVersion,
	)

	// Create or use existing Teleport Role for EKS access.
	err = createRoleForEKSAccess(ctx, clt, request.RoleForEKSAccessWithTrait)
	switch {
	case err == nil:
		slog.InfoContext(ctx, "Role for EKS access created.",
			"role_name", request.RoleForEKSAccessWithTrait,
		)
	case trace.IsAlreadyExists(err):
		slog.InfoContext(ctx, "Using existing Teleport Role.",
			"role_name", request.RoleForEKSAccessWithTrait,
		)
	default:
		return trace.Wrap(err)
	}

	// Find the auto-discovered EKS Cluster for the specified AWS account ID.
	kubeCluster, err := findAutoDiscoveredEKSCluster(ctx, clt, request.AWSAccountID)
	if err != nil {
		return trace.Wrap(err)
	}

	slog.InfoContext(ctx, "Found Kubernetes cluster",
		"account_id", request.AWSAccountID,
		"cluster_name", kubeCluster.GetName(),
	)

	// Ensure the Access List exists or create it if it doesn't.
	accessList, err := ensureAccessList(ctx, clt, request)
	if err != nil {
		return trace.Wrap(err)
	}

	slog.InfoContext(ctx, "Using Access List",
		"access_list_name", accessList.GetName(),
		"access_list_title", accessList.Spec.Title,
		"account_id", request.AWSAccountID,
	)

	// Add a member to the Access List.
	err = upsertEntraTeamMemberToAccessList(ctx, clt, accessList, request.EntraTeam)
	if err != nil {
		return trace.Wrap(err)
	}

	slog.InfoContext(ctx, "Entra Team ID added as a sub-list member",
		"member_name", request.EntraTeam,
		"access_list_name", accessList.GetName(),
		"account_id", request.AWSAccountID,
	)

	return nil
}

type SyncRequest struct {
	AWSAccountID              string
	EntraTeam                 string
	AccessListUniqueID        string
	AccessListTitle           string
	AccessListNextAuditDate   time.Time
	AccessListOwnerUser       string
	RoleForEKSAccessWithTrait string
}

func loadTeleportClient(ctx context.Context) (*client.Client, error) {
	clt, err := client.New(ctx, client.Config{
		Addrs: []string{
			"dinis17.cloud.gravitational.io:443",
		},
		Credentials: []client.Credentials{
			client.LoadProfile("", ""),
		},
	})

	if err != nil {
		return nil, trace.Wrap(err)
	}

	return clt, nil
}

// Single Role for accessing EKS Clusters coming from auto-discovery.
// `account-id` trait is used to filter access to specific AWS account ID.
// This is injected by the Access List's trait.
func createRoleForEKSAccess(ctx context.Context, clt *client.Client, roleName string) error {
	role, err := types.NewRole(roleName, types.RoleSpecV6{
		Allow: types.RoleConditions{
			KubernetesLabels: types.Labels{
				"account-id":                  []string{`{{external["account-id"]}}`},
				"teleport.dev/discovery-type": []string{"eks"},
			},
			KubeGroups: []string{"system:masters"},
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = clt.CreateRole(ctx, role)
	return trace.Wrap(err)
}

func findAutoDiscoveredEKSCluster(ctx context.Context, clt *client.Client, awsAccountID string) (types.KubeCluster, error) {
	allKubeClusters, err := clt.GetKubernetesClusters(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, cluster := range allKubeClusters {
		// Auto discovered EKS Clusters have a specific label.
		if val, _ := cluster.GetLabel("teleport.dev/discovery-type"); val != "eks" {
			continue
		}

		// Check if the cluster is for the specified AWS account ID.
		if cluster.GetAWSConfig().AccountID == awsAccountID {
			return cluster, nil
		}
	}

	return nil, trace.NotFound("no auto-discovered EKS Clusters found for AWS account ID: %s", awsAccountID)
}

func ensureAccessList(ctx context.Context, clt *client.Client, request *SyncRequest) (*accesslist.AccessList, error) {
	existingAccessList, err := clt.AccessListClient().GetAccessList(ctx, request.AccessListUniqueID)
	if err == nil || !trace.IsNotFound(err) {
		return existingAccessList, trace.Wrap(err)
	}

	const twoWeeks = 14 * 24 * time.Hour
	nextAuditDate := request.AccessListNextAuditDate
	if nextAuditDate.IsZero() {
		nextAuditDate = time.Now().Add(twoWeeks)
	}
	accessList, err := accesslist.NewAccessList(
		header.Metadata{
			Name: request.AccessListUniqueID,
		},
		accesslist.Spec{
			Title:       request.AccessListTitle,
			Description: "Access auto discovered EKS Clusters in aws:" + request.AWSAccountID,
			Owners:      []accesslist.Owner{{Name: request.AccessListOwnerUser, MembershipKind: accesslist.MembershipKindUser}},
			Audit: accesslist.Audit{
				NextAuditDate: nextAuditDate,
			},
			Grants: accesslist.Grants{
				Roles: []string{request.RoleForEKSAccessWithTrait},
				Traits: trait.Traits{
					"account-id": []string{request.AWSAccountID},
				},
			},
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	createdAccessList, err := clt.AccessListClient().UpsertAccessList(ctx, accessList)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return createdAccessList, nil
}

func upsertEntraTeamMemberToAccessList(ctx context.Context, clt *client.Client, accessList *accesslist.AccessList, entraTeam string) error {
	member, err := accesslist.NewAccessListMember(
		header.Metadata{
			Name: entraTeam,
		},
		accesslist.AccessListMemberSpec{
			AccessList:     accessList.GetName(),
			Name:           entraTeam,
			Joined:         time.Now().UTC(),
			Expires:        time.Now().UTC().Add(14 * 24 * time.Hour), // Two weeks from now.
			MembershipKind: accesslist.MembershipKindList,
			AddedBy:        "system",
		},
	)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = clt.AccessListClient().UpsertAccessListMember(ctx, member)
	return trace.Wrap(err)
}
