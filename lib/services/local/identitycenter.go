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
	"github.com/gravitational/teleport/lib/utils/pagination"
)

type IdentityServiceMode int

const (
	// IdentityCenterServiceModeStrict is the default service mode, with
	// strict validation enabled.
	IdentityCenterServiceModeStrict IdentityServiceMode = 0

	// IdentityCenterServiceModeRelaxed indicates that the service should do
	// no validation and just write to the provided backend. This is generally
	// for use with caches
	IdentityCenterServiceModeRelaxed IdentityServiceMode = 1

	identityCenterPageSize = 100
)

const (
	awsICPrefix                  = "identity_center"
	awsAccountPrefix             = "accounts"
	awsPermissionSetPrefix       = "permission_sets"
	awsPrincipalAssignmentPrefix = "principal_assignments"
	awsAccountAssignmentPrefix   = "account_assignments"
)

type IdentityCenterServiceConfig struct {
	Backend backend.Backend
	Mode    IdentityServiceMode
	Logger  *slog.Logger
}

func (cfg *IdentityCenterServiceConfig) CheckAndSetDefaults() error {
	if cfg.Backend == nil {
		return trace.BadParameter("must supply backend")
	}

	switch cfg.Mode {
	case IdentityCenterServiceModeStrict, IdentityCenterServiceModeRelaxed:
		break

	default:
		return trace.BadParameter("invalid service mode: %d", cfg.Mode)
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
	mode                 IdentityServiceMode
}

var _ services.IdentityCenter = (*IdentityCenterService)(nil)

// NewIdentityCenterService creates a new service for managing identity-center
// related resources
func NewIdentityCenterService(cfg IdentityCenterServiceConfig) (*IdentityCenterService, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	accountsSvc, err := generic.NewServiceWrapper(generic.ServiceWrapperConfig[*identitycenterv1.Account]{
		Backend:       cfg.Backend,
		ResourceKind:  types.KindIdentityCenterAccount,
		BackendPrefix: backend.NewKey(awsICPrefix, "accounts"),
		MarshalFunc:   services.MarshalProtoResource[*identitycenterv1.Account],
		UnmarshalFunc: services.UnmarshalProtoResource[*identitycenterv1.Account],
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating accounts service")
	}

	permissionSetSvc, err := generic.NewServiceWrapper(generic.ServiceWrapperConfig[*identitycenterv1.PermissionSet]{
		Backend:       cfg.Backend,
		ResourceKind:  types.KindIdentityCenterPermissionSet,
		BackendPrefix: backend.NewKey(awsICPrefix, "permission_sets"),
		MarshalFunc:   services.MarshalProtoResource[*identitycenterv1.PermissionSet],
		UnmarshalFunc: services.UnmarshalProtoResource[*identitycenterv1.PermissionSet],
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating permission sets service")
	}

	principalsSvc, err := generic.NewServiceWrapper(generic.ServiceWrapperConfig[*identitycenterv1.PrincipalAssignment]{
		Backend:       cfg.Backend,
		ResourceKind:  types.KindIdentityCenterPrincipalAssignment,
		BackendPrefix: backend.NewKey(awsICPrefix, "principal_assignments"),
		MarshalFunc:   services.MarshalProtoResource[*identitycenterv1.PrincipalAssignment],
		UnmarshalFunc: services.UnmarshalProtoResource[*identitycenterv1.PrincipalAssignment],
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating principal assignments service")
	}

	accountAssignmentsSvc, err := generic.NewServiceWrapper(generic.ServiceWrapperConfig[*identitycenterv1.AccountAssignment]{
		Backend:       cfg.Backend,
		ResourceKind:  types.KindIdentityCenterAccountAssignment,
		BackendPrefix: backend.NewKey(awsICPrefix, "account_assignments"),
		MarshalFunc:   services.MarshalProtoResource[*identitycenterv1.AccountAssignment],
		UnmarshalFunc: services.UnmarshalProtoResource[*identitycenterv1.AccountAssignment],
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating account assignments service")
	}

	svc := &IdentityCenterService{
		mode:                 cfg.Mode,
		accounts:             accountsSvc,
		permissionSets:       permissionSetSvc,
		principalAssignments: principalsSvc,
		accountAssignments:   accountAssignmentsSvc,
	}

	return svc, nil
}

func (svc *IdentityCenterService) ListIdentityCenterAccounts(ctx context.Context, page pagination.PageRequestToken) ([]services.IdentityCenterAccount, pagination.NextPageToken, error) {
	accounts, nextPage, err := svc.accounts.ListResources(ctx, identityCenterPageSize, string(page))
	if err != nil {
		return nil, "", trace.Wrap(err, "listing identity center assignment records")
	}

	result := make([]services.IdentityCenterAccount, len(accounts))
	for i, acct := range accounts {
		result[i] = services.IdentityCenterAccount{Account: acct}
	}

	return result, pagination.NextPageToken(nextPage), nil
}

func (svc *IdentityCenterService) CreateIdentityCenterAccount(ctx context.Context, acct *identitycenterv1.Account) (services.IdentityCenterAccount, error) {
	created, err := svc.accounts.CreateResource(ctx, acct)
	if err != nil {
		return services.IdentityCenterAccount{}, trace.Wrap(err, "creating identity center account")
	}
	return services.IdentityCenterAccount{Account: created}, nil
}

func (svc *IdentityCenterService) GetIdentityCenterAccount(ctx context.Context, name services.IdentityCenterAccountID) (services.IdentityCenterAccount, error) {
	acct, err := svc.accounts.GetResource(ctx, string(name))
	if err != nil {
		return services.IdentityCenterAccount{}, trace.Wrap(err, "fetching identity center account")
	}
	return services.IdentityCenterAccount{Account: acct}, nil
}

func (svc *IdentityCenterService) UpdateIdentityCenterAccount(ctx context.Context, acct *identitycenterv1.Account) (services.IdentityCenterAccount, error) {
	var updatedAccount *identitycenterv1.Account
	var err error

	switch svc.mode {
	case IdentityCenterServiceModeStrict:
		updatedAccount, err = svc.accounts.ConditionalUpdateResource(ctx, acct)

	case IdentityCenterServiceModeRelaxed:
		updatedAccount, err = svc.accounts.UpdateResource(ctx, acct)

	default:
		return services.IdentityCenterAccount{}, trace.BadParameter("invalid service mode: %v", svc.mode)
	}

	if err != nil {
		return services.IdentityCenterAccount{}, trace.Wrap(err, "updating identity center account record")
	}

	return services.IdentityCenterAccount{Account: updatedAccount}, nil
}

func (svc *IdentityCenterService) DeleteIdentityCenterAccount(ctx context.Context, name services.IdentityCenterAccountID) error {
	return trace.Wrap(svc.accounts.DeleteResource(ctx, string(name)))
}

func (svc *IdentityCenterService) DeleteAllIdentityCenterAccounts(ctx context.Context) error {
	return trace.Wrap(svc.accounts.DeleteAllResources(ctx))
}

func (svc *IdentityCenterService) ListPrincipalAssignments(ctx context.Context, page pagination.PageRequestToken) ([]*identitycenterv1.PrincipalAssignment, pagination.NextPageToken, error) {
	resp, nextPage, err := svc.principalAssignments.ListResources(ctx, identityCenterPageSize, string(page))
	if err != nil {
		return nil, "", trace.Wrap(err, "listing identity center assignment records")
	}
	return resp, pagination.NextPageToken(nextPage), nil
}

func (svc *IdentityCenterService) CreatePrincipalAssignment(ctx context.Context, asmt *identitycenterv1.PrincipalAssignment) (*identitycenterv1.PrincipalAssignment, error) {
	created, err := svc.principalAssignments.CreateResource(ctx, asmt)
	if err != nil {
		return nil, trace.Wrap(err, "creating principal assignment")
	}
	return created, nil
}

func (svc *IdentityCenterService) GetPrincipalAssignment(ctx context.Context, name services.PrincipalAssignmentID) (*identitycenterv1.PrincipalAssignment, error) {
	state, err := svc.principalAssignments.GetResource(ctx, string(name))
	if err != nil {
		return nil, trace.Wrap(err, "fetching principal assignment")
	}
	return state, nil
}

func (svc *IdentityCenterService) UpdatePrincipalAssignment(ctx context.Context, asmt *identitycenterv1.PrincipalAssignment) (*identitycenterv1.PrincipalAssignment, error) {
	var updatedAssignment *identitycenterv1.PrincipalAssignment
	var err error

	switch svc.mode {
	case IdentityCenterServiceModeStrict:
		updatedAssignment, err = svc.principalAssignments.ConditionalUpdateResource(ctx, asmt)

	case IdentityCenterServiceModeRelaxed:
		updatedAssignment, err = svc.principalAssignments.UpdateResource(ctx, asmt)

	default:
		return nil, trace.BadParameter("invalid service mode: %v", svc.mode)
	}

	if err != nil {
		return nil, trace.Wrap(err, "updating principal assignment record")
	}
	return updatedAssignment, nil
}

func (svc *IdentityCenterService) DeletePrincipalAssignment(ctx context.Context, name services.PrincipalAssignmentID) error {
	return trace.Wrap(svc.principalAssignments.DeleteResource(ctx, string(name)))
}

func (svc *IdentityCenterService) DeleteAllPrincipalAssignments(ctx context.Context) error {
	return trace.Wrap(svc.principalAssignments.DeleteAllResources(ctx))
}

func (svc *IdentityCenterService) ListPermissionSets(ctx context.Context, page pagination.PageRequestToken) ([]*identitycenterv1.PermissionSet, pagination.NextPageToken, error) {
	resp, nextPage, err := svc.permissionSets.ListResources(ctx, identityCenterPageSize, string(page))
	if err != nil {
		return nil, "", trace.Wrap(err, "listing identity center permission set records")
	}
	return resp, pagination.NextPageToken(nextPage), nil
}

func (svc *IdentityCenterService) CreatePermissionSet(ctx context.Context, asmt *identitycenterv1.PermissionSet) (*identitycenterv1.PermissionSet, error) {
	created, err := svc.permissionSets.CreateResource(ctx, asmt)
	if err != nil {
		return nil, trace.Wrap(err, "creating identity center permission set")
	}
	return created, nil
}

func (svc *IdentityCenterService) GetPermissionSet(ctx context.Context, name services.PermissionSetID) (*identitycenterv1.PermissionSet, error) {
	state, err := svc.permissionSets.GetResource(ctx, string(name))
	if err != nil {
		return nil, trace.Wrap(err, "fetching permission set")
	}
	return state, nil
}

func (svc *IdentityCenterService) UpdatePermissionSet(ctx context.Context, asmt *identitycenterv1.PermissionSet) (*identitycenterv1.PermissionSet, error) {
	var updatedAssignment *identitycenterv1.PermissionSet
	var err error

	switch svc.mode {
	case IdentityCenterServiceModeStrict:
		updatedAssignment, err = svc.permissionSets.ConditionalUpdateResource(ctx, asmt)

	case IdentityCenterServiceModeRelaxed:
		updatedAssignment, err = svc.permissionSets.UpdateResource(ctx, asmt)

	default:
		return nil, trace.BadParameter("invalid service mode: %v", svc.mode)
	}

	if err != nil {
		return nil, trace.Wrap(err, "updating permission set record")
	}
	return updatedAssignment, nil
}

func (svc *IdentityCenterService) DeletePermissionSet(ctx context.Context, name services.PermissionSetID) error {
	return trace.Wrap(svc.permissionSets.DeleteResource(ctx, string(name)))
}

func (svc *IdentityCenterService) ListAccountAssignments(ctx context.Context, page pagination.PageRequestToken) ([]services.IdentityCenterAccountAssignment, pagination.NextPageToken, error) {
	assignments, nextPage, err := svc.accountAssignments.ListResources(ctx, identityCenterPageSize, string(page))
	if err != nil {
		return nil, "", trace.Wrap(err, "listing identity center assignment records")
	}

	result := make([]services.IdentityCenterAccountAssignment, len(assignments))
	for i, asmt := range assignments {
		result[i] = services.IdentityCenterAccountAssignment{AccountAssignment: asmt}
	}

	return result, pagination.NextPageToken(nextPage), nil
}

func (svc *IdentityCenterService) CreateAccountAssignment(ctx context.Context, asmt services.IdentityCenterAccountAssignment) (services.IdentityCenterAccountAssignment, error) {
	created, err := svc.accountAssignments.CreateResource(ctx, asmt.AccountAssignment)
	if err != nil {
		return services.IdentityCenterAccountAssignment{}, trace.Wrap(err, "creating principal assignment")
	}
	return services.IdentityCenterAccountAssignment{AccountAssignment: created}, nil
}

func (svc *IdentityCenterService) GetAccountAssignment(ctx context.Context, name services.IdentityCenterAccountAssignmentID) (services.IdentityCenterAccountAssignment, error) {
	asmt, err := svc.accountAssignments.GetResource(ctx, string(name))
	if err != nil {
		return services.IdentityCenterAccountAssignment{}, trace.Wrap(err, "fetching principal assignment")
	}
	return services.IdentityCenterAccountAssignment{AccountAssignment: asmt}, nil
}

func (svc *IdentityCenterService) UpdateAccountAssignment(ctx context.Context, asmt services.IdentityCenterAccountAssignment) (services.IdentityCenterAccountAssignment, error) {
	var updatedAssignment *identitycenterv1.AccountAssignment
	var err error

	switch svc.mode {
	case IdentityCenterServiceModeStrict:
		updatedAssignment, err = svc.accountAssignments.ConditionalUpdateResource(ctx, asmt.AccountAssignment)

	case IdentityCenterServiceModeRelaxed:
		updatedAssignment, err = svc.accountAssignments.UpdateResource(ctx, asmt.AccountAssignment)

	default:
		return services.IdentityCenterAccountAssignment{}, trace.BadParameter("invalid service mode: %v", svc.mode)
	}

	if err != nil {
		return services.IdentityCenterAccountAssignment{}, trace.Wrap(err, "updating principal assignment record")
	}
	return services.IdentityCenterAccountAssignment{AccountAssignment: updatedAssignment}, nil
}

func (svc *IdentityCenterService) DeleteAccountAssignment(ctx context.Context, name services.IdentityCenterAccountAssignmentID) error {
	return trace.Wrap(svc.accountAssignments.DeleteResource(ctx, string(name)))
}

func (svc *IdentityCenterService) DeleteAllAccountAssignments(ctx context.Context) error {
	return trace.Wrap(svc.accountAssignments.DeleteAllResources(ctx))
}
