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

	"github.com/gravitational/teleport"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
	"github.com/gravitational/teleport/lib/utils/pagination"
	"github.com/gravitational/trace"
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
	awsICPrefix                   = "identity_center"
	awsAccountPrefix              = awsICPrefix + "/accounts"
	awsPermissionSetPrefix        = awsICPrefix + "/permission_sets"
	awsPermissionSetBindingPrefix = awsICPrefix + "/permission_set_bindings"
	awsPrincipalAssignmentPrefix  = awsICPrefix + "/principal_assignments"
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
	accounts              *generic.ServiceWrapper[*identitycenterv1.Account]
	permissionSets        *generic.ServiceWrapper[*identitycenterv1.PermissionSet]
	permissionSetBindings *generic.ServiceWrapper[*identitycenterv1.PermissionSetBinding]
	assignments           *generic.ServiceWrapper[*identitycenterv1.PrincipalAssignment]
	mode                  IdentityServiceMode
}

var _ services.IdentityCenter = (*IdentityCenterService)(nil)

// NewIdentityCenterService creates a new service for managing identity-center
// related resources
func NewIdentityCenterService(cfg IdentityCenterServiceConfig) (*IdentityCenterService, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	accountsSvc, err := generic.NewServiceWrapper(
		cfg.Backend,
		types.KindIdentityCenterAccount,
		awsAccountPrefix,
		services.MarshalIdentityCenterAccount,
		services.UnmarshalIdentityCenterAccount)
	if err != nil {
		return nil, trace.Wrap(err, "creating accounts service")
	}

	permissionSetSvc, err := generic.NewServiceWrapper(
		cfg.Backend,
		types.KindIdentityCenterPermissionSet,
		awsPermissionSetPrefix,
		services.MarshalPermissionSet,
		services.UnmarshalPermissionSet)
	if err != nil {
		return nil, trace.Wrap(err, "creating permission sets service")
	}

	permissionSetBindingsSvc, err := generic.NewServiceWrapper(
		cfg.Backend,
		types.KindIdentityCenterPermissionSetBinding,
		awsPermissionSetBindingPrefix,
		services.MarshalPermissionSetBinding,
		services.UnmarshalPermissionSetBinding)
	if err != nil {
		return nil, trace.Wrap(err, "creating permission set bindings service")
	}

	assignmentsSvc, err := generic.NewServiceWrapper(
		cfg.Backend,
		types.KindIdentityCenterPrincipalAssignment,
		awsPrincipalAssignmentPrefix,
		services.MarshalPrincipalAssignment,
		services.UnmarshalPrincipalAssignment)
	if err != nil {
		return nil, trace.Wrap(err, "creating assignments service")
	}

	svc := &IdentityCenterService{
		mode:                  cfg.Mode,
		accounts:              accountsSvc,
		permissionSets:        permissionSetSvc,
		permissionSetBindings: permissionSetBindingsSvc,
		assignments:           assignmentsSvc,
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
	resp, nextPage, err := svc.assignments.ListResources(ctx, identityCenterPageSize, string(page))
	if err != nil {
		return nil, "", trace.Wrap(err, "listing identity center assignment records")
	}
	return resp, pagination.NextPageToken(nextPage), nil
}

func (svc *IdentityCenterService) CreatePrincipalAssignment(ctx context.Context, asmt *identitycenterv1.PrincipalAssignment) (*identitycenterv1.PrincipalAssignment, error) {
	created, err := svc.assignments.CreateResource(ctx, asmt)
	if err != nil {
		return nil, trace.Wrap(err, "creating principal assignment")
	}
	return created, nil
}

func (svc *IdentityCenterService) GetPrincipalAssignment(ctx context.Context, name services.PrincipalAssignmentID) (*identitycenterv1.PrincipalAssignment, error) {
	state, err := svc.assignments.GetResource(ctx, string(name))
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
		updatedAssignment, err = svc.assignments.ConditionalUpdateResource(ctx, asmt)

	case IdentityCenterServiceModeRelaxed:
		updatedAssignment, err = svc.assignments.UpdateResource(ctx, asmt)

	default:
		return nil, trace.BadParameter("invalid service mode: %v", svc.mode)
	}

	if err != nil {
		return nil, trace.Wrap(err, "updating principal assignment record")
	}
	return updatedAssignment, nil
}

func (svc *IdentityCenterService) DeletePrincipalAssignment(ctx context.Context, name services.PrincipalAssignmentID) error {
	return trace.Wrap(svc.assignments.DeleteResource(ctx, string(name)))
}

func (svc *IdentityCenterService) DeleteAllPrincipalAssignments(ctx context.Context) error {
	return trace.Wrap(svc.assignments.DeleteAllResources(ctx))
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

func (svc *IdentityCenterService) ListPermissionSetBindings(ctx context.Context, page pagination.PageRequestToken) ([]*identitycenterv1.PermissionSetBinding, pagination.NextPageToken, error) {
	resp, nextPage, err := svc.permissionSetBindings.ListResources(ctx, identityCenterPageSize, string(page))
	if err != nil {
		return nil, "", trace.Wrap(err, "listing identity center permission set binding records")
	}
	return resp, pagination.NextPageToken(nextPage), nil
}

func (svc *IdentityCenterService) CreatePermissionSetBinding(ctx context.Context, binding *identitycenterv1.PermissionSetBinding) (*identitycenterv1.PermissionSetBinding, error) {
	created, err := svc.permissionSetBindings.CreateResource(ctx, binding)
	if err != nil {
		return nil, trace.Wrap(err, "creating identity center permission set binding")
	}
	return created, nil
}

func (svc *IdentityCenterService) GetPermissionSetBinding(ctx context.Context, name services.PermissionSetBindingID) (*identitycenterv1.PermissionSetBinding, error) {
	binding, err := svc.permissionSetBindings.GetResource(ctx, string(name))
	if err != nil {
		return nil, trace.Wrap(err, "fetching permission set")
	}
	return binding, nil
}

func (svc *IdentityCenterService) UpdatePermissionSetBinding(ctx context.Context, binding *identitycenterv1.PermissionSetBinding) (*identitycenterv1.PermissionSetBinding, error) {
	var updatedBinding *identitycenterv1.PermissionSetBinding
	var err error

	switch svc.mode {
	case IdentityCenterServiceModeStrict:
		updatedBinding, err = svc.permissionSetBindings.ConditionalUpdateResource(ctx, binding)

	case IdentityCenterServiceModeRelaxed:
		updatedBinding, err = svc.permissionSetBindings.UpdateResource(ctx, binding)

	default:
		return nil, trace.BadParameter("invalid service mode: %v", svc.mode)
	}

	if err != nil {
		return nil, trace.Wrap(err, "updating permission set record")
	}
	return updatedBinding, nil
}

func (svc *IdentityCenterService) DeletePermissionSetBinding(ctx context.Context, name services.PermissionSetBindingID) error {
	return trace.Wrap(svc.permissionSetBindings.DeleteResource(ctx, string(name)))
}
