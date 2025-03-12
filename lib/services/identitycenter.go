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

package services

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/pagination"
)

// IdentityCenterAccount wraps a raw identity center record in a new type to
// allow it to implement the interfaces required for use with the Unified
// Resource listing.
//
// IdentityCenterAccount simply wraps a pointer to the underlying
// identitycenterv1.Account record, and can be treated as a reference-like type.
// Copies of an IdentityCenterAccount will point to the same record.
type IdentityCenterAccount struct {
	// This wrapper needs to:
	//  - implement the interfaces required for use with the Unified Resource
	//    service.
	//  - expose the existing interfaces & methods on the underlying
	//    identitycenterv1.Account
	//  - avoid copying the underlying identitycenterv1.Account due to embedded
	//    mutexes in the protobuf-generated code
	//
	// Given those requirements, storing an embedded pointer seems to be the
	// least-bad approach.

	*identitycenterv1.Account
}

// CloneResource creates a deep copy of the underlying account resource
func (a IdentityCenterAccount) CloneResource() types.ClonableResource153 {
	return IdentityCenterAccount{
		Account: apiutils.CloneProtoMsg(a.Account),
	}
}

// GetDisplayName returns a human-readable name for the account for UI display.
func (a IdentityCenterAccount) GetDisplayName() string {
	return a.Account.GetSpec().GetName()
}

// IdentityCenterAccountID is a strongly-typed Identity Center account ID.
type IdentityCenterAccountID string

// IdentityCenterAccountGetter provides read-only access to Identity Center
// Account records
type IdentityCenterAccountGetter interface {
	// ListIdentityCenterAccounts provides a paged list of all known identity
	// center accounts
	ListIdentityCenterAccounts(context.Context, int, *pagination.PageRequestToken) ([]IdentityCenterAccount, pagination.NextPageToken, error)

	// GetIdentityCenterAccount fetches a specific Identity Center Account
	GetIdentityCenterAccount(context.Context, IdentityCenterAccountID) (IdentityCenterAccount, error)
}

// IdentityCenterAccounts defines read/write access to Identity Center account
// resources
type IdentityCenterAccounts interface {
	IdentityCenterAccountGetter

	// CreateIdentityCenterAccount creates a new Identity Center Account record
	CreateIdentityCenterAccount(context.Context, IdentityCenterAccount) (IdentityCenterAccount, error)

	// UpdateIdentityCenterAccount performs a conditional update on an Identity
	// Center Account record, returning the updated record on success.
	UpdateIdentityCenterAccount(context.Context, IdentityCenterAccount) (IdentityCenterAccount, error)

	// UpsertIdentityCenterAccount performs an *unconditional* upsert on an
	// Identity Center Account record, returning the updated record on success.
	// Be careful when mixing UpsertIdentityCenterAccount() with resources
	// protected by optimistic locking
	UpsertIdentityCenterAccount(context.Context, IdentityCenterAccount) (IdentityCenterAccount, error)

	// DeleteIdentityCenterAccount deletes an Identity Center Account record
	DeleteIdentityCenterAccount(context.Context, IdentityCenterAccountID) error

	// DeleteAllIdentityCenterAccounts deletes all Identity Center Account records
	DeleteAllIdentityCenterAccounts(context.Context, *identitycenterv1.DeleteAllIdentityCenterAccountsRequest) (*emptypb.Empty, error)
}

// PrincipalAssignmentID is a strongly-typed ID for Identity Center Principal
// Assignments
type PrincipalAssignmentID string

// IdentityCenterPrincipalAssignments defines operations on an Identity Center
// principal assignment database
type IdentityCenterPrincipalAssignments interface {
	// ListPrincipalAssignments lists all PrincipalAssignment records in the
	// service
	ListPrincipalAssignments(context.Context, int, *pagination.PageRequestToken) ([]*identitycenterv1.PrincipalAssignment, pagination.NextPageToken, error)

	// CreatePrincipalAssignment creates a new Principal Assignment record in
	// the service from the supplied in-memory representation. Returns the
	// created record on success.
	CreatePrincipalAssignment(context.Context, *identitycenterv1.PrincipalAssignment) (*identitycenterv1.PrincipalAssignment, error)

	// GetPrincipalAssignment fetches a specific Principal Assignment record.
	GetPrincipalAssignment(context.Context, PrincipalAssignmentID) (*identitycenterv1.PrincipalAssignment, error)

	// UpdatePrincipalAssignment performs a conditional update on a Principal
	// Assignment record
	UpdatePrincipalAssignment(context.Context, *identitycenterv1.PrincipalAssignment) (*identitycenterv1.PrincipalAssignment, error)

	// UpsertPrincipalAssignment performs an unconditional update on a Principal
	// Assignment record
	UpsertPrincipalAssignment(context.Context, *identitycenterv1.PrincipalAssignment) (*identitycenterv1.PrincipalAssignment, error)

	// DeletePrincipalAssignment deletes a specific principal assignment record
	DeletePrincipalAssignment(context.Context, PrincipalAssignmentID) error

	// DeleteAllPrincipalAssignments deletes all assignment record
	DeleteAllPrincipalAssignments(context.Context, *identitycenterv1.DeleteAllPrincipalAssignmentsRequest) (*emptypb.Empty, error)
}

// PermissionSetID is a strongly typed ID for an identitycenterv1.PermissionSet
type PermissionSetID string

// IdentityCenterPermissionSets defines the operations to create and maintain
// identitycenterv1.PermissionSet records in the service.
type IdentityCenterPermissionSets interface {
	// ListPermissionSets list the known Permission Sets
	ListPermissionSets(context.Context, int, *pagination.PageRequestToken) ([]*identitycenterv1.PermissionSet, pagination.NextPageToken, error)

	// CreatePermissionSet creates a new PermissionSet record based on the
	// supplied in-memory representation, returning the created record on
	// success
	CreatePermissionSet(context.Context, *identitycenterv1.PermissionSet) (*identitycenterv1.PermissionSet, error)

	// GetPermissionSet fetches a specific PermissionSet record
	GetPermissionSet(context.Context, PermissionSetID) (*identitycenterv1.PermissionSet, error)

	// UpdatePermissionSet performs a conditional update on the supplied Identity
	// Center Permission Set
	UpdatePermissionSet(context.Context, *identitycenterv1.PermissionSet) (*identitycenterv1.PermissionSet, error)

	// DeletePermissionSet deletes a specific Identity Center PermissionSet
	DeletePermissionSet(context.Context, PermissionSetID) error

	// DeleteAllPermissionSets deletes all Identity Center PermissionSets.
	DeleteAllPermissionSets(context.Context, *identitycenterv1.DeleteAllPermissionSetsRequest) (*emptypb.Empty, error)
}

// IdentityCenterAccountAssignment wraps a raw identitycenterv1.AccountAssignment
// record in a new type to allow it to implement the interfaces required for use
// with the Unified Resource listing. IdentityCenterAccountAssignment simply
// wraps a pointer to the underlying account record, and can be treated as a
// reference-like type.
//
// Copies of an IdentityCenterAccountAssignment will point to the same record.
type IdentityCenterAccountAssignment struct {
	// This wrapper needs to:
	//  - implement the interfaces required for use with the Unified Resource
	//    service.
	//  - expose the existing interfaces & methods on the underlying
	//    identitycenterv1.AccountAssignment
	//  - avoid copying the underlying identitycenterv1.AccountAssignment due to
	//    embedded mutexes in the protobuf-generated code
	//
	// Given those requirements, storing an embedded pointer seems to be the
	// least-bad approach.

	*identitycenterv1.AccountAssignment
}

// CloneResource creates a deep copy of the underlying account resource
func (a IdentityCenterAccountAssignment) CloneResource() types.ClonableResource153 {
	return IdentityCenterAccountAssignment{
		AccountAssignment: apiutils.CloneProtoMsg(a.AccountAssignment),
	}
}

// IdentityCenterAccountAssignmentID is a strongly typed ID for an
// IdentityCenterAccountAssignment
type IdentityCenterAccountAssignmentID string

// IdentityCenterAccountAssignmentGetter provides read-only access to Identity
// Center Account Assignment records
type IdentityCenterAccountAssignmentGetter interface {
	// GetAccountAssignment fetches a specific Account Assignment record.
	GetAccountAssignment(context.Context, IdentityCenterAccountAssignmentID) (IdentityCenterAccountAssignment, error)

	// ListAccountAssignments lists all IdentityCenterAccountAssignment record
	// known to the service
	ListAccountAssignments(context.Context, int, *pagination.PageRequestToken) ([]IdentityCenterAccountAssignment, pagination.NextPageToken, error)
}

// IdentityCenterAccountAssignments defines the operations to create and maintain
// Identity Center account assignment records in the service.
type IdentityCenterAccountAssignments interface {
	IdentityCenterAccountAssignmentGetter

	// CreateAccountAssignment creates a new Account Assignment record in
	// the service from the supplied in-memory representation. Returns the
	// created record on success.
	CreateAccountAssignment(context.Context, IdentityCenterAccountAssignment) (IdentityCenterAccountAssignment, error)

	// UpdateAccountAssignment performs a conditional update on the supplied
	// Account Assignment, returning the updated record on success.
	UpdateAccountAssignment(context.Context, IdentityCenterAccountAssignment) (IdentityCenterAccountAssignment, error)

	// UpsertAccountAssignment performs an unconditional update on the supplied
	// Account Assignment, returning the updated record on success.
	UpsertAccountAssignment(context.Context, IdentityCenterAccountAssignment) (IdentityCenterAccountAssignment, error)

	// DeleteAccountAssignment deletes a specific account assignment
	DeleteAccountAssignment(context.Context, IdentityCenterAccountAssignmentID) error

	// DeleteAllAccountAssignments deletes all known account assignments
	DeleteAllAccountAssignments(context.Context, *identitycenterv1.DeleteAllAccountAssignmentsRequest) (*emptypb.Empty, error)
}

// IdentityCenter combines all the resource managers used by the Identity Center plugin
type IdentityCenter interface {
	IdentityCenterAccounts
	IdentityCenterPermissionSets
	IdentityCenterPrincipalAssignments
	IdentityCenterAccountAssignments
}

// NewIdentityCenterAccountMatcher creates a new [RoleMatcher] configured to
// match the supplied [IdentityCenterAccount].
func NewIdentityCenterAccountMatcher(account IdentityCenterAccount) *IdentityCenterAccountMatcher {
	return &IdentityCenterAccountMatcher{
		accountID: account.GetSpec().GetId(),
	}
}

// IdentityCenterMatcher implements a [RoleMatcher] for comparing Identity Center
// Account resources against the AccountAssignments specified in a Role condition.
type IdentityCenterAccountMatcher struct {
	accountID string
}

// Match implements Role Matching for Identity Center Account resources. It
// attempts to match the Account Assignments in a Role Condition against a
// known Account ID.
func (m *IdentityCenterAccountMatcher) Match(role types.Role, condition types.RoleConditionType) (bool, error) {
	// TODO(tcsc): Expand to cover role template expansion (e.g. {{external.account_assignments}})
	for _, asmt := range role.GetIdentityCenterAccountAssignments(condition) {
		accountMatches, err := matchExpression(m.accountID, asmt.Account)
		if err != nil {
			return false, trace.Wrap(err)
		}

		if accountMatches {
			return true, nil
		}
	}
	return false, nil
}

func (m *IdentityCenterAccountMatcher) String() string {
	return fmt.Sprintf("IdentityCenterAccountMatcher(account=%v)", m.accountID)
}

// NewIdentityCenterAccountAssignmentMatcher creates a new [IdentityCenterAccountAssignmentMatcher]
// configured to match the supplied [IdentityCenterAccountAssignment].
func NewIdentityCenterAccountAssignmentMatcher(assignment IdentityCenterAccountAssignment) *IdentityCenterAccountAssignmentMatcher {
	return &IdentityCenterAccountAssignmentMatcher{
		accountID:        assignment.GetSpec().GetAccountId(),
		permissionSetARN: assignment.GetSpec().GetPermissionSet().Arn,
	}
}

// IdentityCenterMatcher implements a [RoleMatcher] for comparing Identity Center
// Account Assignment resources against the AccountAssignments specified in a
// Role condition.
type IdentityCenterAccountAssignmentMatcher struct {
	accountID        string
	permissionSetARN string
}

// Match implements Role Matching for Identity Center Account resources. It
// attempts to match the Account Assignments in a Role Condition against a
// known Account ID.
func (m *IdentityCenterAccountAssignmentMatcher) Match(role types.Role, condition types.RoleConditionType) (bool, error) {
	// TODO(tcsc): Expand to cover role template expansion (e.g. {{external.account_assignments}})
	for _, asmt := range role.GetIdentityCenterAccountAssignments(condition) {
		accountMatches, err := matchExpression(m.accountID, asmt.Account)
		if err != nil {
			return false, trace.Wrap(err)
		}

		if !accountMatches {
			continue
		}

		permissionSetMatches, err := matchExpression(m.permissionSetARN, asmt.PermissionSet)
		if err != nil {
			return false, trace.Wrap(err)
		}

		if permissionSetMatches {
			return true, nil
		}
	}
	return false, nil
}

func (m *IdentityCenterAccountAssignmentMatcher) String() string {
	return fmt.Sprintf("IdentityCenterAccountMatcher(account=%v, permissionSet=%v)",
		m.accountID, m.permissionSetARN)
}

func matchExpression(target, expression string) (bool, error) {
	if expression == types.Wildcard {
		return true, nil
	}
	matches, err := utils.MatchString(target, expression)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return matches, nil
}
