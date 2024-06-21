package scim

import (
	"context"
	"fmt"

	"github.com/stretchr/testify/mock"

	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/lib/authz"
)

// getResultAs extracts a value from a testify mock argument collection and
// casts it to the desired type. Safely handles untyped `nil` result values
// by returning the zero value for type T.
//
// Any attempt to cast an argument value to an incompatible type will still
// panic.
func getResultAs[T any](result mock.Arguments, index int) T {
	untypedValue := result.Get(index)
	if untypedValue == nil {
		var zero T
		return zero
	}

	typedValue, ok := untypedValue.(T)
	if !ok {
		panic(fmt.Sprintf("getResultAs[%T](%d) failed because object %v wasn't correct type", typedValue, index, untypedValue))
	}

	return typedValue
}

type mockUserService struct {
	mock.Mock
}

var _ UsersService = (*mockUserService)(nil)

func (m *mockUserService) ListUsers(ctx context.Context, req *userspb.ListUsersRequest) (*userspb.ListUsersResponse, error) {
	result := m.Called(ctx, req)
	if fn, isDelegate := result.Get(0).(func(context.Context, *userspb.ListUsersRequest) (*userspb.ListUsersResponse, error)); isDelegate {
		return fn(ctx, req)
	}
	return getResultAs[*userspb.ListUsersResponse](result, 0), result.Error(1)
}

func (m *mockUserService) GetUser(ctx context.Context, user string, withSecrets bool) (types.User, error) {
	result := m.Called(ctx, user, withSecrets)
	fn, isDelegate := result.Get(0).(func(context.Context, string, bool) (types.User, error))
	if isDelegate {
		return fn(ctx, user, withSecrets)
	}
	return getResultAs[types.User](result, 0), result.Error(1)
}

func (m *mockUserService) CreateUser(ctx context.Context, user types.User) (types.User, error) {
	result := m.Called(ctx, user)
	fn, ok := result.Get(0).(func(ctx context.Context, user types.User) (types.User, error))
	if ok {
		return fn(ctx, user)
	}
	return getResultAs[types.User](result, 0), result.Error(1)
}

func (m *mockUserService) UpdateUser(ctx context.Context, user types.User) (types.User, error) {
	result := m.Called(ctx, user)
	fn, isDelegate := result.Get(0).(func(context.Context, types.User) (types.User, error))
	if isDelegate {
		return fn(ctx, user)
	}
	return getResultAs[types.User](result, 0), result.Error(1)
}

func (m *mockUserService) DeleteUser(ctx context.Context, user string) error {
	result := m.Called(ctx, user)
	return result.Error(0)
}

type mockLocksService struct {
	mock.Mock
}

var _ LocksService = (*mockLocksService)(nil)

func (m *mockLocksService) GetLocks(ctx context.Context, inForceOnly bool, targets ...types.LockTarget) ([]types.Lock, error) {
	result := m.Called(ctx, inForceOnly, targets)
	return getResultAs[[]types.Lock](result, 0), result.Error(1)
}

func (m *mockLocksService) UpsertLock(ctx context.Context, lock types.Lock) error {
	result := m.Called(ctx, lock)
	return result.Error(0)
}

func (m *mockLocksService) DeleteLock(ctx context.Context, name string) error {
	result := m.Called(ctx, name)
	return result.Error(0)
}

type mockPluginsService struct {
	mock.Mock
}

var _ PluginsService = (*mockPluginsService)(nil)

func (m *mockPluginsService) GetPlugin(ctx context.Context, name string, withSecrets bool) (types.Plugin, error) {
	result := m.Called(ctx, name, withSecrets)
	return getResultAs[types.Plugin](result, 0), result.Error(1)
}

type mockCredentialsService struct {
	mock.Mock
}

func (m *mockCredentialsService) GetPluginStaticCredentialsByLabels(ctx context.Context, labels map[string]string) ([]types.PluginStaticCredentials, error) {
	result := m.Called(ctx, labels)
	return getResultAs[[]types.PluginStaticCredentials](result, 0), result.Error(1)
}

type mockProviderShim struct {
	mock.Mock
}

var _ providerShim = (*mockProviderShim)(nil)

// accessListPredicate checks if the access list is "owned" by this provider
func (m *mockProviderShim) accessListPredicate(ctx context.Context, acl *accesslist.AccessList) bool {
	result := m.Called(ctx, acl)

	if fn, isDelegate := result.Get(0).(func(context.Context, *accesslist.AccessList) bool); isDelegate {
		return fn(ctx, acl)
	}

	return result.Bool(0)
}

func (m *mockProviderShim) userPredicate(ctx context.Context, u types.User) bool {
	// Mock doesn't give us an easy way to execute an arbitrary function and
	// return that function's result as the mocked call's result. We emulate
	// that behavior by passing a delegate function through the Result() method
	// when setting up the expectation, and invoking that delegate if we detect
	// it here.
	result := m.Called(ctx, u)
	if fn, isDelegate := result.Get(0).(func(context.Context, types.User) bool); isDelegate {
		return fn(ctx, u)
	}

	return result.Bool(0)
}

func (m *mockProviderShim) userToResource(ctx context.Context, user types.User) (*scimpb.Resource, error) {
	result := m.Called(ctx, user)

	if fn, isDelegate := result.Get(0).(func(context.Context, types.User) (*scimpb.Resource, error)); isDelegate {
		return fn(ctx, user)
	}

	return getResultAs[*scimpb.Resource](result, 0), result.Error(1)
}

func (m *mockProviderShim) resourceToUser(ctx context.Context, r *scimpb.Resource) (types.User, error) {
	result := m.Called(ctx, r)

	if fn, isDelegate := result.Get(0).(func(context.Context, *scimpb.Resource) (types.User, error)); isDelegate {
		return fn(ctx, r)
	}

	return getResultAs[types.User](result, 0), result.Error(1)
}

func (m *mockProviderShim) authorizeRequest(ctx context.Context, hdr string) error {
	result := m.Called(ctx, hdr)
	return result.Error(0)
}

func (m *mockProviderShim) onCreatingAccessList(ctx context.Context, accessList *accesslist.AccessList) error {
	result := m.Called(ctx, accessList)
	return result.Error(0)
}

func (m *mockProviderShim) onCreatingAccessListMember(ctx context.Context, member *accesslist.AccessListMember) error {
	result := m.Called(ctx, member)
	if fn, isDelegate := result.Get(0).(func(context.Context, *accesslist.AccessListMember) error); isDelegate {
		return fn(ctx, member)
	}
	return result.Error(0)
}

func (m *mockProviderShim) onCreatingUser(ctx context.Context, u types.User, r *scimpb.Resource) error {
	result := m.Called(ctx, u, r)
	return result.Error(0)
}

func (m *mockProviderShim) onCreatedUser(ctx context.Context, u types.User, r *scimpb.Resource) error {
	result := m.Called(ctx, u, r)
	return result.Error(0)
}

func (m *mockProviderShim) onUpdatingUser(ctx context.Context, u types.User, r *scimpb.Resource) (types.User, bool, error) {
	result := m.Called(ctx, u, r)
	fn, isDelegate := result.Get(0).(func(context.Context, types.User, *scimpb.Resource) (types.User, bool, error))
	if isDelegate {
		return fn(ctx, u, r)
	}
	return getResultAs[types.User](result, 0), result.Bool(1), result.Error(2)
}

func (m *mockProviderShim) getResourceLabels() map[string]string {
	result := m.Called()
	return getResultAs[map[string]string](result, 0)
}

type mockAuthorizer struct {
	mock.Mock
}

var _ authz.Authorizer = (*mockAuthorizer)(nil)

func (m *mockAuthorizer) Authorize(ctx context.Context) (*authz.Context, error) {
	result := m.Called(ctx)
	return getResultAs[*authz.Context](result, 0), result.Error(1)
}

type mockRoleService struct {
	mock.Mock
}

func (m *mockRoleService) CreateRole(ctx context.Context, role types.Role) (types.Role, error) {
	result := m.Called(ctx, role)
	if fn, ok := result.Get(0).(func(context.Context, types.Role) (types.Role, error)); ok {
		return fn(ctx, role)
	}
	return getResultAs[types.Role](result, 0), result.Error(1)
}

func (m *mockRoleService) DeleteRole(ctx context.Context, roleName string) error {
	result := m.Called(ctx, roleName)
	if fn, ok := result.Get(0).(func(ctx context.Context, roleName string) error); ok {
		return fn(ctx, roleName)
	}
	return result.Error(0)
}

type mockAccessListService struct {
	mock.Mock
}

func (m *mockAccessListService) DeleteAccessList(ctx context.Context, accessList string) error {
	result := m.Called(ctx, accessList)
	return result.Error(0)
}

func (m *mockAccessListService) DeleteAccessListMember(ctx context.Context, accessList string, memberName string) error {
	result := m.Called(ctx, accessList, memberName)
	return result.Error(0)
}

func (m *mockAccessListService) GetAccessList(ctx context.Context, aclName string) (*accesslist.AccessList, error) {
	result := m.Called(ctx, aclName)
	return getResultAs[*accesslist.AccessList](result, 0), result.Error(1)
}

func (m *mockAccessListService) ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, pageToken string) ([]*accesslist.AccessListMember, string, error) {
	result := m.Called(ctx, accessListName, pageSize, pageToken)
	return getResultAs[[]*accesslist.AccessListMember](result, 0), result.String(1), result.Error(2)
}

func (m *mockAccessListService) ListAccessLists(ctx context.Context, pageSize int, nextToken string) ([]*accesslist.AccessList, string, error) {
	result := m.Called(ctx, pageSize, nextToken)
	return getResultAs[[]*accesslist.AccessList](result, 0), result.String(1), result.Error(2)
}

func (m *mockAccessListService) UpsertAccessList(ctx context.Context, al *accesslist.AccessList) (*accesslist.AccessList, error) {
	result := m.Called(ctx, al)
	if fn, ok := result.Get(0).(func(context.Context, *accesslist.AccessList) (*accesslist.AccessList, error)); ok {
		return fn(ctx, al)
	}
	return getResultAs[*accesslist.AccessList](result, 0), result.Error(1)
}

func (m *mockAccessListService) UpsertAccessListWithMembers(ctx context.Context, al *accesslist.AccessList, ms []*accesslist.AccessListMember) (*accesslist.AccessList, []*accesslist.AccessListMember, error) {
	result := m.Called(ctx, al, ms)
	if fn, ok := result.Get(0).(func(context.Context, *accesslist.AccessList, []*accesslist.AccessListMember) (*accesslist.AccessList, []*accesslist.AccessListMember, error)); ok {
		return fn(ctx, al, ms)
	}
	return getResultAs[*accesslist.AccessList](result, 0), getResultAs[[]*accesslist.AccessListMember](result, 1), result.Error(2)
}

func (m *mockAccessListService) UpsertAccessListMember(ctx context.Context, alm *accesslist.AccessListMember) (*accesslist.AccessListMember, error) {
	result := m.Called(ctx, alm)
	if fn, ok := result.Get(0).(func(context.Context, *accesslist.AccessListMember) (*accesslist.AccessListMember, error)); ok {
		return fn(ctx, alm)
	}
	return getResultAs[*accesslist.AccessListMember](result, 0), result.Error(1)
}

type mockIdentityService struct {
	mock.Mock
}

func (m *mockIdentityService) GetSAMLConnector(ctx context.Context, id string, withSecrets bool) (types.SAMLConnector, error) {
	result := m.Called(ctx, id, withSecrets)
	if fn, ok := result.Get(0).(func(context.Context, string, bool) (types.SAMLConnector, error)); ok {
		return fn(ctx, id, withSecrets)
	}
	return getResultAs[types.SAMLConnector](result, 0), result.Error(1)
}
