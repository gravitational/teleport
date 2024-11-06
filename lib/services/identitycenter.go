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

	"google.golang.org/protobuf/proto"

	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
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
func (a IdentityCenterAccount) CloneResource() IdentityCenterAccount {
	return IdentityCenterAccount{
		Account: proto.Clone(a.Account).(*identitycenterv1.Account),
	}
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
	DeleteAllIdentityCenterAccounts(context.Context) error
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
	DeleteAllPrincipalAssignments(context.Context) error
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
func (a IdentityCenterAccountAssignment) CloneResource() IdentityCenterAccountAssignment {
	return IdentityCenterAccountAssignment{
		AccountAssignment: proto.Clone(a.AccountAssignment).(*identitycenterv1.AccountAssignment),
	}
}

// IdentityCenterAccountAssignmentID is a strongly typed ID for an
// IdentityCenterAccountAssignment
type IdentityCenterAccountAssignmentID string

// IdentityCenterAccountAssignments defines the operations to create and maintain
// Identity Center account assignment records in the service.
type IdentityCenterAccountAssignments interface {
	// ListAccountAssignments lists all IdentityCenterAccountAssignment record
	// known to the service
	ListAccountAssignments(context.Context, int, *pagination.PageRequestToken) ([]IdentityCenterAccountAssignment, pagination.NextPageToken, error)

	// CreateAccountAssignment creates a new Account Assignment record in
	// the service from the supplied in-memory representation. Returns the
	// created record on success.
	CreateAccountAssignment(context.Context, IdentityCenterAccountAssignment) (IdentityCenterAccountAssignment, error)

	// GetAccountAssignment fetches a specific Account Assignment record.
	GetAccountAssignment(context.Context, IdentityCenterAccountAssignmentID) (IdentityCenterAccountAssignment, error)

	// UpdateAccountAssignment performs a conditional update on the supplied
	// Account Assignment, returning the updated record on success.
	UpdateAccountAssignment(context.Context, IdentityCenterAccountAssignment) (IdentityCenterAccountAssignment, error)

	// UpsertAccountAssignment performs an unconditional update on the supplied
	// Account Assignment, returning the updated record on success.
	UpsertAccountAssignment(context.Context, IdentityCenterAccountAssignment) (IdentityCenterAccountAssignment, error)

	// DeleteAccountAssignment deletes a specific account assignment
	DeleteAccountAssignment(context.Context, IdentityCenterAccountAssignmentID) error

	// DeleteAllAccountAssignments deletes all known account assignments
	DeleteAllAccountAssignments(context.Context) error
}

// IdentityCenter combines all the resource managers used by the Identity Center plugin
type IdentityCenter interface {
	IdentityCenterAccounts
	IdentityCenterPermissionSets
	IdentityCenterPrincipalAssignments
	IdentityCenterAccountAssignments
}
