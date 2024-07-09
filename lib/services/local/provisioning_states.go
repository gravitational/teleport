package local

import (
	"context"

	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
	"github.com/gravitational/trace"
)

const (
	provisioningUserStatePrefix   = "provisioning_user_status"
	provisioningUserStatePageSize = 100
)

// ProvisioningStateService handles low-level CRUD operations for the provisioning status
type ProvisioningStateService struct {
	service *generic.ServiceWrapper[*provisioningv1.UserState]
}

var _ services.ProvisioningStates = (*ProvisioningStateService)(nil)

func NewProvisioningStateService(backend backend.Backend) (*ProvisioningStateService, error) {
	userStatusSvc, err := generic.NewServiceWrapper(
		backend,
		types.KindProvisioningUserState,
		provisioningUserStatePrefix,
		services.MarshalProvisioningUserState,
		services.UnmarshalProvisioningUserState)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	svc := &ProvisioningStateService{
		service: userStatusSvc,
	}

	return svc, nil
}

func (ss *ProvisioningStateService) CreateUserProvisioningState(ctx context.Context, state *provisioningv1.UserState) (*provisioningv1.UserState, error) {
	createdState, err := ss.service.CreateResource(ctx, state)
	if err != nil {
		return nil, trace.Wrap(err, "creating new user state record")
	}
	return createdState, nil
}

func (ss *ProvisioningStateService) UpdateUserProvisioningState(ctx context.Context, state *provisioningv1.UserState) (*provisioningv1.UserState, error) {
	updatedState, err := ss.service.UpdateResource(ctx, state)
	if err != nil {
		return nil, trace.Wrap(err, "updating new user state record")
	}
	return updatedState, nil
}

func (ss *ProvisioningStateService) GetUserProvisioningState(ctx context.Context, name string) (*provisioningv1.UserState, error) {
	state, err := ss.service.GetResource(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err, "fetching user provisioning state")
	}
	return state, nil
}

func (ss *ProvisioningStateService) ListUserProvisioningStates(ctx context.Context, page services.PageToken) ([]*provisioningv1.UserState, services.PageToken, error) {
	resp, nextPage, err := ss.service.ListResources(ctx, provisioningUserStatePageSize, string(page))
	if err != nil {
		return nil, "", trace.Wrap(err, "listing user provisioning states")
	}
	return resp, services.PageToken(nextPage), nil
}

func (ss *ProvisioningStateService) DeleteUserProvisioningState(ctx context.Context, name string) error {
	return trace.Wrap(ss.service.DeleteResource(ctx, name))
}

func (ss *ProvisioningStateService) DeleteAllUserProvisioningStates(ctx context.Context) error {
	return trace.Wrap(ss.service.DeleteAllResources(ctx))
}
