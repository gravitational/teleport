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
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/trait"
)

const twoWeeks = 14 * 24 * time.Hour

func main() {
	ctx := context.Background()

	if err := do(ctx); err != nil {
		slog.ErrorContext(ctx, "failed to run", "error", err)
	}
}

func fromStringFlag(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func parseRequest() (*SyncRequest, error) {
	flag.Usage = func() {
		flag.CommandLine.Output().Write([]byte(`Usage of sync:
Create an Access List which allows users from an Entra ID Group to access EKS Clusters in a given AWS Account.

You can provide the Entra ID Group using one of:
- the Teleport's Access List ID as seen in Teleport
- the Entra Group Object ID as seen in Azure/Entra
- the Entra Group Name

Ensure you have valid Teleport credentials (eg, tsh login) before running this command.
The following Teleport RBAC rules are required:
    - resources:
      - roles
      verbs:
      - read
      - list
      - create
    - resources:
      - access_list
      verbs:
      - read
      - list
      - create
      - update

Full list of arguments:
`))
		flag.PrintDefaults()
	}

	awsAccountID := flag.String(
		"aws-account-id",
		os.Getenv("AWS_ACCOUNT_ID"),
		"AWS Account ID to allow access to (required).",
	)

	teleportAddress := flag.String(
		"proxy",
		os.Getenv("TELEPORT_PROXY"),
		"Teleport Proxy's address, eg. tenant.teleport.sh:443 (required).",
	)

	teleportEntraGroupAccessListID := flag.String(
		"group-by-teleport-id",
		os.Getenv("GROUP_BY_TELEPORT_ID"),
		"Teleport's ID for the Access List.",
	)

	microsoftEntraGroupObjectID := flag.String(
		"group-by-entra-object-id",
		os.Getenv("GROUP_BY_ENTRA_OBJECT_ID"),
		"Microsoft Entra Group Object ID.",
	)

	microsoftEntraGroupName := flag.String(
		"group-by-name",
		os.Getenv("GROUP_BY_ENTRA_NAME"),
		"Teleport Teleport Entra Group Access List ID as synced into Teleport",
	)

	flag.Parse()

	if fromStringFlag(awsAccountID) == "" {
		return nil, trace.BadParameter("aws-account-id is required")
	}

	if fromStringFlag(teleportEntraGroupAccessListID) == "" &&
		fromStringFlag(microsoftEntraGroupObjectID) == "" &&
		fromStringFlag(microsoftEntraGroupName) == "" {
		return nil, trace.BadParameter("at least one of group-by-teleport-id, group-by-entra-object-id or group-by-name is required")
	}

	// Here you would typically use a flag package to parse command line arguments
	// For simplicity, we are returning a hardcoded request
	return &SyncRequest{

		AWSAccountID: fromStringFlag(awsAccountID),

		TeleportAddress: fromStringFlag(teleportAddress),

		TeleportEntraGroupAccessListID: fromStringFlag(teleportEntraGroupAccessListID),
		MicrosoftEntraGroupObjectID:    fromStringFlag(microsoftEntraGroupObjectID),
		MicrosoftEntraGroupName:        fromStringFlag(microsoftEntraGroupName),

		RoleForEKSAccessWithTrait: "eks-access",
		AccessListUniqueID:        "eks-access-" + fromStringFlag(awsAccountID),
		AccessListTitle:           "EKS Access for account id " + fromStringFlag(awsAccountID),
	}, nil
}

func do(ctx context.Context) error {
	// Load request from flags or environment variables.
	request, err := parseRequest()
	if err != nil {
		return trace.Wrap(err)
	}

	// Set up Connection to Teleport
	clt, err := loadTeleportClient(ctx, request.TeleportAddress)
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

	// Ensure the Access List exists or create it if it doesn't.
	accessList, err := ensureAccessList(ctx, clt, request)
	if err != nil {
		return trace.Wrap(err)
	}

	slog.InfoContext(ctx, "Using Access List",
		"access_list_name", accessList.GetName(),
		"access_list_title", accessList.Spec.Title,
	)

	entraGroup, err := guessAccessListIDFromEntraIdentifiers(ctx, clt, request)
	if err != nil {
		return trace.Wrap(err)
	}

	slog.InfoContext(ctx, "Entra Group ID found",
		"teleport_access_list_name", entraGroup,
		"entra_group_name", request.MicrosoftEntraGroupName,
		"entra_group_object_id", request.MicrosoftEntraGroupObjectID,
	)

	// Add a Entra Group as member to the Access List.
	err = upsertEntraGroupMemberToAccessList(ctx, clt, accessList, entraGroup)
	if err != nil {
		return trace.Wrap(err)
	}

	slog.InfoContext(ctx, "Entra Group ID added as a group member",
		"access_list", accessList.GetName(),
		"member_group_name", entraGroup,
	)

	return nil
}

type SyncRequest struct {
	AWSAccountID    string
	TeleportAddress string

	// Microsoft Entra Group can be referenced by one of the following, ordered by preference and priority.
	// 1. Teleport's Access List name that corresponds to the Entra Group as synced into Teleport.
	TeleportEntraGroupAccessListID string
	// 2. Microsoft Entra Group Object ID.
	MicrosoftEntraGroupObjectID string
	// 3. Microsoft Entra Group Display Name.
	MicrosoftEntraGroupName string

	AccessListUniqueID        string
	AccessListTitle           string
	RoleForEKSAccessWithTrait string
}

func loadTeleportClient(ctx context.Context, teleportAddress string) (*client.Client, error) {
	clt, err := client.New(ctx, client.Config{
		Addrs: []string{
			teleportAddress,
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

func ensureAccessList(ctx context.Context, clt *client.Client, request *SyncRequest) (*accesslist.AccessList, error) {
	existingAccessList, err := clt.AccessListClient().GetAccessList(ctx, request.AccessListUniqueID)
	if err == nil || !trace.IsNotFound(err) {
		return existingAccessList, trace.Wrap(err)
	}

	const listOwnerSystem = "system"

	accessList, err := accesslist.NewAccessList(
		header.Metadata{
			Name: request.AccessListUniqueID,
		},
		accesslist.Spec{
			Title:       request.AccessListTitle,
			Description: "Access auto discovered EKS Clusters in aws:" + request.AWSAccountID,
			Owners:      []accesslist.Owner{{Name: listOwnerSystem, MembershipKind: accesslist.MembershipKindUser}},
			Audit: accesslist.Audit{
				NextAuditDate: time.Now().Add(twoWeeks),
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

func upsertEntraGroupMemberToAccessList(ctx context.Context, clt *client.Client, accessList *accesslist.AccessList, entraGroup string) error {
	member, err := accesslist.NewAccessListMember(
		header.Metadata{
			Name: entraGroup,
		},
		accesslist.AccessListMemberSpec{
			AccessList:     accessList.GetName(),
			Name:           entraGroup,
			Joined:         time.Now().UTC(),
			Expires:        time.Now().UTC().Add(twoWeeks),
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

// guessAccessListIDFromEntraIdentifiers iterates through the Access Lists
// and tries to find the one that matches the Entra Group ID.
// This ID might be the Teleport's Access List name or the Microsoft Entra Group Object ID.
// The function returns the Teleport's Access List name that corresponds to the Entra Group ID.
func guessAccessListIDFromEntraIdentifiers(ctx context.Context, clt *client.Client, request *SyncRequest) (string, error) {
	if request.TeleportEntraGroupAccessListID != "" {
		return request.TeleportEntraGroupAccessListID, nil
	}

	nextToken := ""
	for {
		// List all Access Lists.
		existingAccessList, respNextToken, err := clt.AccessListClient().ListAccessLists(ctx, 0, nextToken)
		if err != nil {
			return "", trace.Wrap(err)
		}

		for _, accessList := range existingAccessList {
			accessListLabels := accessList.GetAllLabels()

			// Only consider Access Lists that are from Entra ID.
			if accessListLabels[types.OriginLabel] != types.OriginEntraID {
				continue
			}

			// Use Microsoft ObjectID if provided.
			if request.MicrosoftEntraGroupObjectID != "" {
				if accessListLabels[types.EntraUniqueIDLabel] != request.MicrosoftEntraGroupObjectID {
					continue
				}
				return accessList.GetName(), nil
			}

			// Otherwise, use the Group's Display Name.
			if request.MicrosoftEntraGroupName != "" && accessListLabels[types.EntraDisplayNameLabel] == request.MicrosoftEntraGroupName {
				return accessList.GetName(), nil
			}
		}
		if respNextToken == "" {
			break
		}
		nextToken = respNextToken
	}

	return "", trace.NotFound("no Access List found for Entra Group ID: %s", request.MicrosoftEntraGroupObjectID)
}
