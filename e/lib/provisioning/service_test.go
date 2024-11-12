package provisioning

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

func TestUpstreamProvisioning(t *testing.T) {
	ctx := context.Background()
	pack := newPack(t)

	const (
		aliceUser = "alice"
		aclID     = "test-access-list-1"
		aclTitle  = "Test Access List 1 Title"
	)

	t.Run("should provision teleport user to scim upstream", func(t *testing.T) {
		pack.mustCreateTeleportUser(t, aliceUser)
		assertSCIMUserExists(t, pack.scimMock, aliceUser)
	})

	t.Run("should provision access list to scim upstream", func(t *testing.T) {
		pack.mustCreateAccessList(t, aclID, aclTitle)
		pack.mustUpsertAccessListMember(t, aclID, aliceUser)
		assertSCIMGroupExitsWithMembersLength(t, pack.scimMock, aclTitle, 1)
	})

	t.Run("should de-provision scim group membership", func(t *testing.T) {
		require.NoError(t, pack.depsMock.DeleteAccessListMember(ctx, aclID, aliceUser))
		assertSCIMGroupExitsWithMembersLength(t, pack.scimMock, aclTitle, 0)
	})

	t.Run("should de-provision scim group", func(t *testing.T) {
		require.NoError(t, pack.depsMock.DeleteAccessList(ctx, aclID))
		assertSCIMGroupDoestExist(t, pack.scimMock, aclTitle)
	})

	t.Run("should de-provision scim user", func(t *testing.T) {
		require.NoError(t, pack.depsMock.DeleteUser(ctx, aliceUser))
		assertSCIMUserDoesntExist(t, pack.scimMock, aliceUser)
	})
}

func TestUpstreamProvisioningUserActivationDeactivation(t *testing.T) {
	ctx := context.Background()
	pack := newPack(t)

	const (
		aliceUser = "alice"
		lockName  = "lock-1"
	)

	pack.mustCreateTeleportUser(t, aliceUser)
	assertSCIMUserExistAndIsActive(t, pack.scimMock, aliceUser)

	t.Run("should de-activate scim user in upstream when a user is locked in teleport", func(t *testing.T) {
		l := &types.LockV2{
			Metadata: types.Metadata{Name: lockName},
			Spec:     types.LockSpecV2{Target: types.LockTarget{User: aliceUser}},
		}
		require.NoError(t, pack.depsMock.UpsertLock(ctx, l))
		assertSCIMUserExistAndIsNotActive(t, pack.scimMock, aliceUser)
	})

	t.Run("should re-activate scim user in upstream when teleport user lock is deleted", func(t *testing.T) {
		require.NoError(t, pack.depsMock.DeleteLock(ctx, lockName))
		assertSCIMUserExistAndIsActive(t, pack.scimMock, aliceUser)
	})
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

	identitySvc, err := local.NewIdentityServiceV2(b)
	require.NoError(t, err)

	aclSvc, err := local.NewAccessListService(b, clock)
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

type testPack struct {
	depsMock *mockDeps
	scimMock *scimsdk.ClientMock
	clock    clockwork.FakeClock
}

type sutOptions struct {
	scimClient *scimsdk.ClientMock
}

type sutOption func(*sutOptions)

func newPack(t *testing.T, options ...sutOption) *testPack {
	defaultOpts := &sutOptions{
		scimClient: scimsdk.NewSCIMClientMock(),
	}
	for _, opt := range options {
		opt(defaultOpts)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	clock := clockwork.NewFakeClock()

	depsMock := newDepsMock(t, clock)
	svc, err := NewService(ServiceConfig{
		SCIMClient:       defaultOpts.scimClient,
		UsersCache:       depsMock,
		AccessListsCache: depsMock,
		Locks:            depsMock,
		StateSvc:         depsMock,
		StateSvcCache:    depsMock,
		EventsClient:     depsMock,
		Clock:            clock,
		DownstreamID:     "downstreamID",
		AccessListPredicate: func(context.Context, *accesslist.AccessList) (bool, error) {
			return true, nil
		},
	})
	require.NoError(t, err)

	go func() {
		err = svc.Run(ctx)
		require.NoError(t, err)
	}()

	return &testPack{
		depsMock: depsMock,
		scimMock: defaultOpts.scimClient,
		clock:    clock,
	}
}

func eventualWithT(t *testing.T, condition func(collect *assert.CollectT)) {
	const (
		defaultWaitFor = time.Second * 2
		defaultTick    = time.Millisecond * 50
	)
	require.EventuallyWithT(t, condition, defaultWaitFor, defaultTick)
}

func assertSCIMGroupExitsWithMembersLength(t *testing.T, client scimsdk.Client, groupDisplayName string, wantMembersLength int) {
	eventualWithT(t, func(collect *assert.CollectT) {
		acl, err := client.GetGroupByDisplayName(context.Background(), groupDisplayName)
		if !assert.NoError(collect, err) {
			return
		}
		assert.Len(collect, acl.Members, wantMembersLength)
	})
}

func assertSCIMUserExists(t *testing.T, client scimsdk.Client, userName string) {
	eventualWithT(t, func(collect *assert.CollectT) {
		_, err := client.GetUserByUserName(context.Background(), userName)
		assert.NoError(collect, err)
	})
}

func assertSCIMUserDoesntExist(t *testing.T, client scimsdk.Client, userName string) {
	eventualWithT(t, func(collect *assert.CollectT) {
		_, err := client.GetUserByUserName(context.Background(), userName)
		assert.True(collect, trace.IsNotFound(err))
	})
}

func assertSCIMUserExistAndIsNotActive(t *testing.T, client scimsdk.Client, userName string) {
	eventualWithT(t, func(collect *assert.CollectT) {
		out, err := client.GetUserByUserName(context.Background(), userName)
		if !assert.NoError(collect, err) {
			return
		}
		assert.False(collect, out.Active)
	})
}

func assertSCIMUserExistAndIsActive(t *testing.T, client scimsdk.Client, userName string) {
	eventualWithT(t, func(collect *assert.CollectT) {
		out, err := client.GetUserByUserName(context.Background(), userName)
		if !assert.NoError(collect, err) {
			return
		}
		assert.True(collect, out.Active)
	})
}

func assertSCIMGroupDoestExist(t *testing.T, scimClient scimsdk.Client, displayName string) {
	eventualWithT(t, func(collect *assert.CollectT) {
		_, err := scimClient.GetGroupByDisplayName(context.Background(), displayName)
		assert.True(collect, trace.IsNotFound(err))
	})
}

func (s *testPack) mustCreateTeleportUser(t *testing.T, name string) {
	_, err := s.depsMock.CreateUser(context.Background(), &types.UserV2{
		Metadata: types.Metadata{
			Name: name,
		},
	})
	require.NoError(t, err)
}

func (s *testPack) mustCreateAccessList(t *testing.T, name, title string) {
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
	_, err := s.depsMock.UpsertAccessList(context.Background(), acl)
	require.NoError(t, err)
}

func (s *testPack) mustUpsertAccessListMember(t *testing.T, accessList, memberName string) {
	aclMember := &accesslist.AccessListMember{
		ResourceHeader: header.ResourceHeader{
			Metadata: header.Metadata{Name: memberName},
		},
		Spec: accesslist.AccessListMemberSpec{
			AccessList: accessList,
			Name:       memberName,
			Joined:     s.clock.Now(),
			AddedBy:    "ut-test",
		},
	}
	_, err := s.depsMock.UpsertAccessListMember(context.Background(), aclMember)
	require.NoError(t, err)
}
