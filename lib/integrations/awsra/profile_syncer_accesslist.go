/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package awsra

import (
	"context"
	"fmt"
	"path"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/services"
)

// AccessListManager is an interface that defines methods for managing access lists.
// This is used to create access lists for each AWS Roles Anywhere Profile.
// Access Lists are also removed when the Profile is deleted or disabled.
type AccessListManager interface {
	ListAccessLists(context.Context, int, string) ([]*accesslist.AccessList, string, error)
	UpsertAccessList(context.Context, *accesslist.AccessList) (*accesslist.AccessList, error)
	GetAccessList(ctx context.Context, name string) (*accesslist.AccessList, error)
	DeleteAccessList(context.Context, string) error

	// GetRole returns role by name
	GetRole(ctx context.Context, name string) (types.Role, error)
	// UpsertRole creates or updates a role.
	UpsertRole(ctx context.Context, role types.Role) (types.Role, error)
}

// uuidNamespace is the namespace used for generating UUIDs for access lists.
// It is a UUID derived from the string "aws-iam-roles-anywhere-profile-arn".
var uuidNamespace = uuid.NewSHA1(uuid.Nil, []byte("aws-iam-roles-anywhere-profile-arn"))

// accessListName generates a unique name for the access list based on the integration name and profile ARN.
func accessListName(integration, profileARN string) string {
	accessListName := path.Join(integration, profileARN)
	// generate a UUID from the path to ensure uniqueness
	// and to avoid collisions with other access lists.
	// This is necessary because access list names are used as keys in the backend
	// and must be unique and deterministic.
	return uuid.NewSHA1(uuidNamespace, []byte(accessListName)).String()
}

const accessListAuditInTwoWeeks = 14 * 24 * time.Hour

// createAccessListForProfile creates an access list for the given AWS Roles Anywhere Profile.
// It has no members, but grants the generic role and the following trait:
// - iam-roles-anywhere-profile-arn: <profile ARN>
// - iam-roles: [<profile roles>]
//
// This trait is used by the generic role to allow access to the AWS Roles Anywhere Profile
func createAccessListForProfile(ctx context.Context, req processProfileRequest) error {
	if !req.Params.AccessListsEnabled {
		return nil
	}

	accessListName := accessListName(req.Integration.GetName(), req.Profile.GetArn())

	accessList, err := accessListForProfile(accessListName, req)
	if err != nil {
		return trace.Wrap(err)
	}

	existingAccessList, err := req.Params.AccessListManager.GetAccessList(ctx, accessListName)
	if err != nil {
		if trace.IsNotFound(err) {
			_, err = req.Params.AccessListManager.UpsertAccessList(ctx, accessList)
			return trace.Wrap(err)
		}

		return trace.Wrap(err)
	}

	mergedAccessList, updated := mergedNewAccessListSpec(existingAccessList, accessList)
	if !updated {
		// No changes to the access list spec, so we can skip the upsert.
		return nil
	}

	_, err = req.Params.AccessListManager.UpsertAccessList(ctx, mergedAccessList)
	return trace.Wrap(err)
}

func mergedNewAccessListSpec(old, new *accesslist.AccessList) (merged *accesslist.AccessList, changed bool) {
	if old.Spec.Title != new.Spec.Title {
		return overrideAccessListWithNewSpec(old, new), true
	}

	if old.Spec.Description != new.Spec.Description {
		return overrideAccessListWithNewSpec(old, new), true
	}

	if !slices.Equal(old.Spec.Grants.Roles, new.Spec.Grants.Roles) {
		return overrideAccessListWithNewSpec(old, new), true
	}

	if len(old.Spec.Grants.Traits) != len(new.Spec.Grants.Traits) {
		return overrideAccessListWithNewSpec(old, new), true
	} else {
		for key, newValues := range new.Spec.Grants.Traits {
			oldValues, ok := old.Spec.Grants.Traits[key]
			if !ok || !slices.Equal(oldValues, newValues) {
				return overrideAccessListWithNewSpec(old, new), true
			}
		}
	}

	return nil, false
}

func overrideAccessListWithNewSpec(old, new *accesslist.AccessList) *accesslist.AccessList {
	// This function is used to override the spec of an existing access list with a new one.
	// It is used to ensure that the access list has the correct spec, even if it already exists.
	// This is necessary because the access list spec may change over time, and we want to ensure
	// that the access list has the latest spec.
	//
	// Only the fields coming from the Roles Anywhere Profile are updated.
	// This ensures that Audit and Owners are not changed.
	old.Spec.Title = new.Spec.Title
	old.Spec.Description = new.Spec.Description
	old.Spec.Grants = new.Spec.Grants
	return old
}

func accessListForProfile(accessListName string, req processProfileRequest) (*accesslist.AccessList, error) {
	const listOwnerSystem = "system"
	applicationName := applicationNameFromProfile(req.Profile, req.Integration.GetName())

	accessList, err := accesslist.NewAccessList(
		header.Metadata{
			Name: accessListName,
			Labels: map[string]string{
				types.OriginLabel:                     types.OriginIntegrationAWSRolesAnywhere,
				types.AWSRolesAnywhereProfileARNLabel: req.Profile.GetArn(),
				types.IntegrationLabel:                req.Integration.GetName(),
			},
		},
		accesslist.Spec{
			Title:       "AWS " + applicationName + " Access",
			Description: fmt.Sprintf("Access Roles allowed by the IAM Roles Anywhere Profile %q", req.Profile.Arn),
			Owners: []accesslist.Owner{
				{Name: listOwnerSystem, MembershipKind: accesslist.MembershipKindUser},
			},
			Audit: accesslist.Audit{
				NextAuditDate: req.Params.Clock.Now().Add(accessListAuditInTwoWeeks),
			},
			Grants: services.AWSIAMRolesAnywhereAccessListGrants(req.Profile.Arn, req.Profile.Roles),
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return accessList, nil
}

// removeOutdatedAccessLists removes access lists that are no longer matching an existing AWS Roles Anywhere Profile.
// This can happen when
// - a Profile is deleted or disabled
// - when the Profile is no longer matched because of the filters
// - or the integration was deleted or its profile sync was disabled
func removeOutdatedAccessLists(ctx context.Context, accessListManager AccessListManager, syncedProfileARNs []string) error {
	startKey := ""
	for {
		accessLists, nextKey, err := accessListManager.ListAccessLists(ctx, 0, startKey)
		if err != nil {
			return trace.Wrap(err)
		}

		for _, accessList := range accessLists {
			accessListLabels := accessList.GetAllLabels()

			if accessListLabels[types.OriginLabel] != types.OriginIntegrationAWSRolesAnywhere {
				continue
			}

			profileARN := accessListLabels[types.AWSRolesAnywhereProfileARNLabel]

			if !slices.Contains(syncedProfileARNs, profileARN) {
				if err := accessListManager.DeleteAccessList(ctx, accessList.GetName()); err != nil {
					return trace.Wrap(err)
				}
			}
		}
		if nextKey == "" {
			break
		}
		startKey = nextKey
	}

	return nil
}
