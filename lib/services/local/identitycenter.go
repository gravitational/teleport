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

package local

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const (
	identityCenterPageSize = 100
)

const (
	awsResourcePrefix            = "identity_center"
	awsAccountPrefix             = "accounts"
	awsPermissionSetPrefix       = "permission_sets"
	awsPrincipalAssignmentPrefix = "principal_assignments"
	awsAccountAssignmentPrefix   = "account_assignments"
)

// IdentityCenterServiceConfig provides configuration parameters for an
// IdentityCenterService
type IdentityCenterServiceConfig struct {
	// Backend is the storage backend to use for the service
	Backend backend.Backend

	// Logger is the logger for the service to use. A default will be supplied
	// if not specified.
	Logger *slog.Logger
}

// CheckAndSetDefaults validates the cfg and supplies defaults where
// appropriate.
func (cfg *IdentityCenterServiceConfig) CheckAndSetDefaults() error {
	if cfg.Backend == nil {
		return trace.BadParameter("must supply backend")
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.Default().With(teleport.ComponentKey, "AWS-IC-LOCAL")
	}

	return nil
}

// IdentityCenterService handles low-level CRUD operations for the identity-
// center related resources
type IdentityCenterService struct {
	accounts             *generic.ServiceWrapper[*identitycenterv1.Account]
	permissionSets       *generic.ServiceWrapper[*identitycenterv1.PermissionSet]
	principalAssignments *generic.ServiceWrapper[*identitycenterv1.PrincipalAssignment]
	accountAssignments   *generic.ServiceWrapper[*identitycenterv1.AccountAssignment]
}

// compile-time assertion that the IdentityCenterService implements the
// services.IdentityCenter interface
var _ services.IdentityCenter = (*IdentityCenterService)(nil)

// NewIdentityCenterService creates a new service for managing identity-center
// related resources
func NewIdentityCenterService(cfg IdentityCenterServiceConfig) (*IdentityCenterService, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	accountsSvc, err := generic.NewServiceWrapper(generic.ServiceConfig[*identitycenterv1.Account]{
		Backend:       cfg.Backend,
		ResourceKind:  types.KindIdentityCenterAccount,
		BackendPrefix: backend.NewKey(awsResourcePrefix, awsAccountPrefix),
		MarshalFunc:   services.MarshalProtoResource[*identitycenterv1.Account],
		UnmarshalFunc: services.UnmarshalProtoResource[*identitycenterv1.Account],
		PageLimit:     identityCenterPageSize,
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating accounts service")
	}

	permissionSetSvc, err := generic.NewServiceWrapper(generic.ServiceConfig[*identitycenterv1.PermissionSet]{
		Backend:       cfg.Backend,
		ResourceKind:  types.KindIdentityCenterPermissionSet,
		BackendPrefix: backend.NewKey(awsResourcePrefix, awsPermissionSetPrefix),
		MarshalFunc:   services.MarshalProtoResource[*identitycenterv1.PermissionSet],
		UnmarshalFunc: services.UnmarshalProtoResource[*identitycenterv1.PermissionSet],
		PageLimit:     identityCenterPageSize,
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating permission sets service")
	}

	principalsSvc, err := generic.NewServiceWrapper(generic.ServiceConfig[*identitycenterv1.PrincipalAssignment]{
		Backend:       cfg.Backend,
		ResourceKind:  types.KindIdentityCenterPrincipalAssignment,
		BackendPrefix: backend.NewKey(awsResourcePrefix, awsPrincipalAssignmentPrefix),
		MarshalFunc:   services.MarshalProtoResource[*identitycenterv1.PrincipalAssignment],
		UnmarshalFunc: services.UnmarshalProtoResource[*identitycenterv1.PrincipalAssignment],
		PageLimit:     identityCenterPageSize,
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating principal assignments service")
	}

	accountAssignmentsSvc, err := generic.NewServiceWrapper(generic.ServiceConfig[*identitycenterv1.AccountAssignment]{
		Backend:       cfg.Backend,
		ResourceKind:  types.KindIdentityCenterAccountAssignment,
		BackendPrefix: backend.NewKey(awsResourcePrefix, awsAccountAssignmentPrefix),
		MarshalFunc:   services.MarshalProtoResource[*identitycenterv1.AccountAssignment],
		UnmarshalFunc: services.UnmarshalProtoResource[*identitycenterv1.AccountAssignment],
		PageLimit:     identityCenterPageSize,
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating account assignments service")
	}

	svc := &IdentityCenterService{
		accounts:             accountsSvc,
		permissionSets:       permissionSetSvc,
		principalAssignments: principalsSvc,
		accountAssignments:   accountAssignmentsSvc,
	}

	return svc, nil
}

// ListIdentityCenterAccounts provides a paged list of all AWS accounts known
// to the Identity Center integration
func (svc *IdentityCenterService) ListIdentityCenterAccounts(ctx context.Context, pageSize int, pageToken string) ([]*identitycenterv1.Account, string, error) {
	accounts, nextPage, err := svc.ListIdentityCenterAccounts2(ctx, pageSize, pageToken)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return accounts, nextPage, nil
}

// ListIdentityCenterAccounts2 provides a paged list of all AWS accounts known
// to the Identity Center integration
func (svc *IdentityCenterService) ListIdentityCenterAccounts2(ctx context.Context, pageSize int, pageToken string) ([]*identitycenterv1.Account, string, error) {
	accounts, nextPage, err := svc.accounts.ListResources(ctx, pageSize, pageToken)
	if err != nil {
		return nil, "", trace.Wrap(err, "listing identity center assignment records")
	}

	return accounts, nextPage, nil
}

// CreateIdentityCenterAccount creates a new Identity Center Account record
func (svc *IdentityCenterService) CreateIdentityCenterAccount(ctx context.Context, acct *identitycenterv1.Account) (*identitycenterv1.Account, error) {
	created, err := svc.CreateIdentityCenterAccount2(ctx, acct)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return created, nil
}

// CreateIdentityCenterAccount2 creates a new Identity Center Account record
func (svc *IdentityCenterService) CreateIdentityCenterAccount2(ctx context.Context, acct *identitycenterv1.Account) (*identitycenterv1.Account, error) {
	created, err := svc.accounts.CreateResource(ctx, acct)
	if err != nil {
		return nil, trace.Wrap(err, "creating identity center account")
	}
	return created, nil
}

// GetIdentityCenterAccount fetches a specific Identity Center Account
func (svc *IdentityCenterService) GetIdentityCenterAccount(ctx context.Context, name string) (*identitycenterv1.Account, error) {
	acct, err := svc.accounts.GetResource(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err, "fetching identity center account")
	}
	return acct, nil
}

// UpdateIdentityCenterAccount performs a conditional update on an Identity
// Center Account record, returning the updated record on success.
func (svc *IdentityCenterService) UpdateIdentityCenterAccount(ctx context.Context, acct *identitycenterv1.Account) (*identitycenterv1.Account, error) {
	updated, err := svc.UpdateIdentityCenterAccount2(ctx, acct)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return updated, nil
}

// UpdateIdentityCenterAccount2 performs a conditional update on an Identity
// Center Account record, returning the updated record on success.
func (svc *IdentityCenterService) UpdateIdentityCenterAccount2(ctx context.Context, acct *identitycenterv1.Account) (*identitycenterv1.Account, error) {
	updated, err := svc.accounts.ConditionalUpdateResource(ctx, acct)
	if err != nil {
		return nil, trace.Wrap(err, "updating identity center account record")
	}
	return updated, nil
}

// UpsertIdentityCenterAccount performs an *unconditional* upsert on an
// Identity Center Account record, returning the updated record on success.
// Be careful when mixing UpsertIdentityCenterAccount() with resources
// protected by optimistic locking
func (svc *IdentityCenterService) UpsertIdentityCenterAccount(ctx context.Context, acct *identitycenterv1.Account) (*identitycenterv1.Account, error) {
	updated, err := svc.accounts.UpsertResource(ctx, acct)
	if err != nil {
		return nil, trace.Wrap(err, "upserting identity center account record")
	}
	return updated, nil
}

// DeleteIdentityCenterAccount deletes an Identity Center Account record
func (svc *IdentityCenterService) DeleteIdentityCenterAccount(ctx context.Context, name services.IdentityCenterAccountID) error {
	return trace.Wrap(svc.accounts.DeleteResource(ctx, string(name)))
}

// DeleteAllIdentityCenterAccounts deletes all Identity Center Account records
func (svc *IdentityCenterService) DeleteAllIdentityCenterAccounts(ctx context.Context) error {
	return trace.Wrap(svc.accounts.DeleteAllResources(ctx))
}

// ListPrincipalAssignments lists a page of PrincipalAssignment records in the service.
func (svc *IdentityCenterService) ListPrincipalAssignments(ctx context.Context, pageSize int, pageToken string) ([]*identitycenterv1.PrincipalAssignment, string, error) {
	resp, nextPage, err := svc.ListPrincipalAssignments2(ctx, pageSize, pageToken)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return resp, nextPage, nil
}

// ListPrincipalAssignments2 lists a page of PrincipalAssignment records in the service.
func (svc *IdentityCenterService) ListPrincipalAssignments2(ctx context.Context, pageSize int, pageToken string) ([]*identitycenterv1.PrincipalAssignment, string, error) {
	resp, nextPage, err := svc.principalAssignments.ListResources(ctx, pageSize, pageToken)
	if err != nil {
		return nil, "", trace.Wrap(err, "listing identity center assignment records")
	}
	return resp, nextPage, nil
}

// CreatePrincipalAssignment creates a new Principal Assignment record in the
// service from the supplied in-memory representation. Returns the created
// record on success.
func (svc *IdentityCenterService) CreatePrincipalAssignment(ctx context.Context, asmt *identitycenterv1.PrincipalAssignment) (*identitycenterv1.PrincipalAssignment, error) {
	created, err := svc.principalAssignments.CreateResource(ctx, asmt)
	if err != nil {
		return nil, trace.Wrap(err, "creating principal assignment")
	}
	return created, nil
}

// GetPrincipalAssignment fetches a specific Principal Assignment record.
func (svc *IdentityCenterService) GetPrincipalAssignment(ctx context.Context, name services.PrincipalAssignmentID) (*identitycenterv1.PrincipalAssignment, error) {
	state, err := svc.principalAssignments.GetResource(ctx, string(name))
	if err != nil {
		return nil, trace.Wrap(err, "fetching principal assignment")
	}
	return state, nil
}

// UpdatePrincipalAssignment performs a conditional update on a Principal
// Assignment record
func (svc *IdentityCenterService) UpdatePrincipalAssignment(ctx context.Context, asmt *identitycenterv1.PrincipalAssignment) (*identitycenterv1.PrincipalAssignment, error) {
	updated, err := svc.principalAssignments.ConditionalUpdateResource(ctx, asmt)
	if err != nil {
		return nil, trace.Wrap(err, "updating principal assignment record")
	}
	return updated, nil
}

// UpsertPrincipalAssignment performs an unconditional update on a Principal
// Assignment record
func (svc *IdentityCenterService) UpsertPrincipalAssignment(ctx context.Context, asmt *identitycenterv1.PrincipalAssignment) (*identitycenterv1.PrincipalAssignment, error) {
	updated, err := svc.principalAssignments.UpsertResource(ctx, asmt)
	if err != nil {
		return nil, trace.Wrap(err, "upserting principal assignment record")
	}
	return updated, nil
}

// DeletePrincipalAssignment deletes a specific principal assignment record
func (svc *IdentityCenterService) DeletePrincipalAssignment(ctx context.Context, name services.PrincipalAssignmentID) error {
	return trace.Wrap(svc.principalAssignments.DeleteResource(ctx, string(name)))
}

// DeleteAllPrincipalAssignments deletes all assignment record
func (svc *IdentityCenterService) DeleteAllPrincipalAssignments(ctx context.Context) error {
	return trace.Wrap(svc.principalAssignments.DeleteAllResources(ctx))
}

// ListPermissionSets returns a page of known Permission Sets in the managed Identity Center
func (svc *IdentityCenterService) ListPermissionSets(ctx context.Context, pageSize int, pageToken string) ([]*identitycenterv1.PermissionSet, string, error) {
	resp, nextPage, err := svc.ListPermissionSets2(ctx, pageSize, pageToken)
	if err != nil {
		return nil, "", trace.Wrap(err, "listing identity center permission set records")
	}
	return resp, nextPage, nil
}

// ListPermissionSets2 returns a page of known Permission Sets in the managed Identity Center
func (svc *IdentityCenterService) ListPermissionSets2(ctx context.Context, pageSize int, pageToken string) ([]*identitycenterv1.PermissionSet, string, error) {

	resp, nextPage, err := svc.permissionSets.ListResources(ctx, pageSize, pageToken)
	if err != nil {
		return nil, "", trace.Wrap(err, "listing identity center permission set records")
	}
	return resp, nextPage, nil
}

// CreatePermissionSet creates a new PermissionSet record based on the supplied
// in-memory representation, returning the created record on success.
func (svc *IdentityCenterService) CreatePermissionSet(ctx context.Context, asmt *identitycenterv1.PermissionSet) (*identitycenterv1.PermissionSet, error) {
	created, err := svc.permissionSets.CreateResource(ctx, asmt)
	if err != nil {
		return nil, trace.Wrap(err, "creating identity center permission set")
	}
	return created, nil
}

// GetPermissionSet fetches a specific PermissionSet record
func (svc *IdentityCenterService) GetPermissionSet(ctx context.Context, name services.PermissionSetID) (*identitycenterv1.PermissionSet, error) {
	state, err := svc.permissionSets.GetResource(ctx, string(name))
	if err != nil {
		return nil, trace.Wrap(err, "fetching permission set")
	}
	return state, nil
}

// UpdatePermissionSet performs a conditional update on the supplied Identity
// Center Permission Set
func (svc *IdentityCenterService) UpdatePermissionSet(ctx context.Context, asmt *identitycenterv1.PermissionSet) (*identitycenterv1.PermissionSet, error) {
	updated, err := svc.permissionSets.ConditionalUpdateResource(ctx, asmt)
	if err != nil {
		return nil, trace.Wrap(err, "updating permission set record")
	}
	return updated, nil
}

// DeletePermissionSet deletes a specific Identity Center PermissionSet
func (svc *IdentityCenterService) DeletePermissionSet(ctx context.Context, name services.PermissionSetID) error {
	return trace.Wrap(svc.permissionSets.DeleteResource(ctx, string(name)))
}

// DeleteAllPermissionSets deletes all Identity Center PermissionSet
func (svc *IdentityCenterService) DeleteAllPermissionSets(ctx context.Context) error {
	return trace.Wrap(svc.permissionSets.DeleteAllResources(ctx))
}

// ListIdentityCenterAccountAssignments returns a page of IdentityCenterAccountAssignment records.
func (svc *IdentityCenterService) ListIdentityCenterAccountAssignments(ctx context.Context, pageSize int, pageToken string) ([]*identitycenterv1.AccountAssignment, string, error) {
	assignments, nextPage, err := svc.accountAssignments.ListResources(ctx, pageSize, pageToken)
	if err != nil {
		return nil, "", trace.Wrap(err, "listing identity center assignment records")
	}

	return assignments, nextPage, nil
}

// CreateAccountAssignment creates a new Account Assignment record in
// the service from the supplied in-memory representation. Returns the
// created record on success.
func (svc *IdentityCenterService) CreateAccountAssignment(ctx context.Context, asmt services.IdentityCenterAccountAssignment) (services.IdentityCenterAccountAssignment, error) {
	created, err := svc.CreateIdentityCenterAccountAssignment(ctx, asmt.AccountAssignment)
	if err != nil {
		return services.IdentityCenterAccountAssignment{}, trace.Wrap(err)
	}
	return services.IdentityCenterAccountAssignment{AccountAssignment: created}, nil
}

// CreateIdentityCenterAccountAssignment creates a new Account Assignment record in
// the service from the supplied in-memory representation. Returns the
// created record on success.
func (svc *IdentityCenterService) CreateIdentityCenterAccountAssignment(ctx context.Context, asmt *identitycenterv1.AccountAssignment) (*identitycenterv1.AccountAssignment, error) {
	created, err := svc.accountAssignments.CreateResource(ctx, asmt)
	if err != nil {
		return nil, trace.Wrap(err, "creating principal assignment")
	}
	return created, nil
}

// GetAccountAssignment fetches a specific Account Assignment record.
func (svc *IdentityCenterService) GetAccountAssignment(ctx context.Context, name services.IdentityCenterAccountAssignmentID) (services.IdentityCenterAccountAssignment, error) {
	asmt, err := svc.GetIdentityCenterAccountAssignment(ctx, string(name))
	if err != nil {
		return services.IdentityCenterAccountAssignment{}, trace.Wrap(err)
	}
	return services.IdentityCenterAccountAssignment{AccountAssignment: asmt}, nil
}

// GetIdentityCenterAccountAssignment fetches a specific Account Assignment record.
func (svc *IdentityCenterService) GetIdentityCenterAccountAssignment(ctx context.Context, name string) (*identitycenterv1.AccountAssignment, error) {
	asmt, err := svc.accountAssignments.GetResource(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err, "fetching principal assignment")
	}
	return asmt, nil
}

// UpdateAccountAssignment performs a conditional update on the supplied
// Account Assignment, returning the updated record on success.
func (svc *IdentityCenterService) UpdateAccountAssignment(ctx context.Context, asmt services.IdentityCenterAccountAssignment) (services.IdentityCenterAccountAssignment, error) {
	updated, err := svc.UpdateIdentityCenterAccountAssignment(ctx, asmt.AccountAssignment)
	if err != nil {
		return services.IdentityCenterAccountAssignment{}, trace.Wrap(err)
	}
	return services.IdentityCenterAccountAssignment{AccountAssignment: updated}, nil
}

// UpdateIdentityCenterAccountAssignment performs a conditional update on the supplied
// Account Assignment, returning the updated record on success.
func (svc *IdentityCenterService) UpdateIdentityCenterAccountAssignment(ctx context.Context, asmt *identitycenterv1.AccountAssignment) (*identitycenterv1.AccountAssignment, error) {
	updated, err := svc.accountAssignments.ConditionalUpdateResource(ctx, asmt)
	if err != nil {
		return nil, trace.Wrap(err, "updating principal assignment record")
	}
	return updated, nil
}

// UpsertAccountAssignment performs an unconditional upsert on the supplied
// Account Assignment, returning the updated record on success.
func (svc *IdentityCenterService) UpsertAccountAssignment(ctx context.Context, asmt *identitycenterv1.AccountAssignment) (*identitycenterv1.AccountAssignment, error) {
	updated, err := svc.accountAssignments.UpsertResource(ctx, asmt)
	if err != nil {
		return nil, trace.Wrap(err, "upserting principal assignment record")
	}
	return updated, nil
}

// DeleteAccountAssignment deletes a specific account assignment
func (svc *IdentityCenterService) DeleteAccountAssignment(ctx context.Context, name services.IdentityCenterAccountAssignmentID) error {
	return trace.Wrap(svc.accountAssignments.DeleteResource(ctx, string(name)))
}

// DeleteAllAccountAssignments deletes all known account assignments
func (svc *IdentityCenterService) DeleteAllAccountAssignments(ctx context.Context) error {
	return trace.Wrap(svc.accountAssignments.DeleteAllResources(ctx))
}
