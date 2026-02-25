package provisioning

import (
	"context"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	identitycentercommon "github.com/gravitational/teleport/e/lib/aws/identitycenter/common"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils/clocki"
)

type testPack struct {
	depsMock     *mockDeps
	scimMock     *scimsdk.ClientMock
	clock        clocki.FakeClock
	downstreamID services.DownstreamID
}

type sutOptions struct {
	downstreamID        services.DownstreamID
	scimClient          *scimsdk.ClientMock
	accessListPredicate AccessListPredicate
	userPredicate       identitycentercommon.UserFilterFunc
	onProvisioning      EventHandlerWithError
	onProvisioned       EventHandler
	onDeprovisioning    EventHandlerWithError
}

type sutOption func(*sutOptions)

func withAccessListPredicate(p AccessListPredicate) sutOption {
	return func(opts *sutOptions) {
		opts.accessListPredicate = p
	}
}

func withUserPredicate(fn func(types.User) bool) sutOption {
	return func(opts *sutOptions) {
		opts.userPredicate = fn
	}
}

func withOnProvisioningCallback(fn EventHandlerWithError) sutOption {
	return func(opts *sutOptions) {
		opts.onProvisioning = fn
	}
}

func withOnProvisionedCallback(fn EventHandler) sutOption {
	return func(opts *sutOptions) {
		opts.onProvisioned = fn
	}
}

func withOnDeprovisioningCallback(fn EventHandlerWithError) sutOption {
	return func(opts *sutOptions) {
		opts.onDeprovisioning = fn
	}
}

func newPack(t *testing.T, options ...sutOption) *testPack {
	defaultOpts := &sutOptions{
		downstreamID: "test-downstream",
		scimClient:   scimsdk.NewSCIMClientMock(),
		accessListPredicate: func(context.Context, *accesslist.AccessList) (bool, error) {
			return true, nil
		},
		userPredicate: identitycentercommon.UserPredicateFilter(nil),
	}
	for _, opt := range options {
		opt(defaultOpts)
	}

	// We can only legally call testify assertions from the main test thread, so
	// set up a mechanism for us to marshal the service goroutine's exit code
	// back here for asserting that it exited cleanly.
	errCh := make(chan error)
	t.Cleanup(func() {
		// If we have already failed, then there's no point in waiting around
		if t.Failed() {
			return
		}

		select {
		case err := <-errCh:
			require.NoError(t, err, "Service shutdown")
		case <-time.After(10 * time.Second):
			require.Fail(t, "Test cleanup timed out")
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	clock := clockwork.NewFakeClock()

	depsMock := newDepsMock(t, clock)
	svc, err := NewService(ServiceConfig{
		SCIMClient:                defaultOpts.scimClient,
		UsersCache:                depsMock,
		AccessListsCache:          depsMock,
		Locks:                     depsMock,
		StateSvc:                  depsMock,
		StateSvcCache:             depsMock,
		EventsClient:              depsMock,
		Clock:                     clock,
		DownstreamID:              defaultOpts.downstreamID,
		AccessListPredicate:       defaultOpts.accessListPredicate,
		UserPredicate:             defaultOpts.userPredicate,
		OnPrincipalProvisioning:   defaultOpts.onProvisioning,
		OnPrincipalProvisioned:    defaultOpts.onProvisioned,
		OnPrincipalDeprovisioning: defaultOpts.onDeprovisioning,
		UserProvisioningMode:      UserProvisioningModeInternal,
	})
	require.NoError(t, err)

	go func() {
		// ensure we marshal the error value back to the main test goroutine for
		// assertion
		errCh <- svc.Run(ctx)
	}()

	return &testPack{
		downstreamID: defaultOpts.downstreamID,
		depsMock:     depsMock,
		scimMock:     defaultOpts.scimClient,
		clock:        clock,
	}
}

type userOption struct {
	labels map[string]string
}

type userOptionFn func(*userOption)

func withUserLabels(labels map[string]string) userOptionFn {
	return func(opts *userOption) {
		opts.labels = labels
	}
}

func (s *testPack) mustCreateTeleportUser(t *testing.T, name string, options ...userOptionFn) {
	opts := &userOption{}
	for _, opt := range options {
		opt(opts)
	}
	_, err := s.depsMock.CreateUser(context.Background(), &types.UserV2{
		Metadata: types.Metadata{
			Labels: opts.labels,
			Name:   name,
		},
	})
	require.NoError(t, err)
}

func (s *testPack) mustUpdateTeleportUser(t *testing.T, name string, mutate func(u types.User)) types.User {
	user, err := s.depsMock.GetUser(t.Context(), name, false /* no secrets */)
	require.NoError(t, err)
	mutate(user)
	updatedUser, err := s.depsMock.UpdateUser(t.Context(), user)
	require.NoError(t, err)
	return updatedUser
}

func (s *testPack) mustCreateAccessList(t *testing.T, name, title string) *accesslist.AccessList {
	acl := &accesslist.AccessList{
		ResourceHeader: header.ResourceHeader{
			Metadata: header.Metadata{Name: name},
		},
		Spec: accesslist.Spec{
			Owners: []accesslist.Owner{{Name: "access-list-owner"}},
			Grants: accesslist.Grants{Roles: []string{"role1"}},
			Title:  title,
		},
	}
	return s.mustUpsertAccessList(t, acl)
}

func (s *testPack) mustCreateAccessListWithCleanup(t *testing.T, name, title string) *accesslist.AccessList {
	acl := s.mustCreateAccessList(t, name, title)
	t.Cleanup(func() {
		err := s.depsMock.DeleteAccessList(context.Background(), acl.GetName())
		require.NoError(t, err)
	})
	return acl
}

func (s *testPack) mustUpsertAccessList(t *testing.T, acl *accesslist.AccessList) *accesslist.AccessList {
	acl, err := s.depsMock.UpsertAccessList(context.Background(), acl)
	require.NoError(t, err)
	return acl
}

func (s *testPack) mustUpsertAccessListMember(t *testing.T, accessList, memberName, memberKind string) {
	s.aclMember(t, accessList, memberName, memberKind, "" /* withOrigin */)
}

func (s *testPack) mustUpsertAccessListMemberWithNonExistentUserAccount(t *testing.T, accessList, memberName, origin string) {
	s.aclMember(t, accessList, memberName, accesslist.MembershipKindUser, origin)
}

func (s *testPack) aclMember(t *testing.T, accessList, memberName string, memberKind string, withOrigin string) {
	aclMember := &accesslist.AccessListMember{
		ResourceHeader: header.ResourceHeader{
			Metadata: header.Metadata{Name: memberName},
		},
		Spec: accesslist.AccessListMemberSpec{
			AccessList:     accessList,
			Name:           memberName,
			Joined:         s.clock.Now(),
			AddedBy:        "ut-test",
			MembershipKind: memberKind,
		},
	}
	if withOrigin != "" {
		aclMember.SetOrigin(withOrigin)
		aclMember.Metadata.Labels[ExternalIDLabel.String()] = memberName
	}
	_, err := s.depsMock.UpsertAccessListMember(context.Background(), aclMember)
	require.NoError(t, err)
}

func (s *testPack) mustDeleteAccessListMember(t *testing.T, accessList, memberName string) {
	require.NoError(t, s.depsMock.DeleteAccessListMember(t.Context(), accessList, memberName))
}

func (s *testPack) getAccessListProvisioningState(t require.TestingT, aclID string) *provisioningv1.PrincipalState {
	ctx := context.Background()
	if ctxer, ok := t.(interface{ Context() context.Context }); ok {
		ctx = ctxer.Context()
	}
	pps, err := s.depsMock.GetProvisioningState(ctx, s.downstreamID, getIDForAccessListName(aclID))
	require.NoError(t, err, "Principal Provisioning State for Access List %q must exist", aclID)
	return pps
}

func (s *testPack) getUserProvisioningState(t require.TestingT, username string) *provisioningv1.PrincipalState {
	ctx := context.Background()
	if ctxer, ok := t.(interface{ Context() context.Context }); ok {
		ctx = ctxer.Context()
	}
	pps, err := s.depsMock.GetProvisioningState(ctx, s.downstreamID, GetIDForUserName(username))
	require.NoError(t, err, "Principal Provisioning State for user %q must exist", username)
	return pps
}

func (s *testPack) getProvisioningStates(t *testing.T) []*provisioningv1.PrincipalState {
	var result []*provisioningv1.PrincipalState
	for pps, err := range allProvisioningStates(t.Context(), s.depsMock, s.downstreamID) {
		require.NoError(t, err)
		result = append(result, pps)
	}
	return result
}

type mockDeps struct {
	*local.AccessService
	*local.IdentityService
	*local.AccessListService
	services.DownstreamProvisioningStates
	types.Events
}

func newDepsMock(t *testing.T, clock clockwork.Clock) *mockDeps {
	b, err := memory.New(memory.Config{})
	require.NoError(t, err)

	identitySvc, err := local.NewIdentityService(b)
	require.NoError(t, err)

	aclSvc, err := local.NewAccessListServiceV2(local.AccessListServiceConfig{
		Backend: b,
		Modules: modulestest.EnterpriseModules(),
	})
	require.NoError(t, err)

	provStateSvc, err := local.NewProvisioningStateService(b)
	require.NoError(t, err)

	return &mockDeps{
		AccessService:                local.NewAccessService(b),
		Events:                       local.NewEventsService(b),
		IdentityService:              identitySvc,
		AccessListService:            aclSvc,
		DownstreamProvisioningStates: provStateSvc,
	}
}
