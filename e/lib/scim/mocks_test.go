package scim

import (
	"context"
	"fmt"

	"github.com/stretchr/testify/mock"

	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/api/types"
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

func (m *mockUserService) ListUsers(ctx context.Context, pageSize int, nextToken string, withSecrets bool) ([]types.User, string, error) {
	result := m.Called(ctx, pageSize, nextToken, withSecrets)
	return getResultAs[[]types.User](result, 0), result.String(1), result.Error(2)
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

func (m *mockProviderShim) updateUser(ctx context.Context, u types.User, r *scimpb.Resource) (*scimpb.Resource, error) {
	result := m.Called(ctx, u, r)
	fn, isDelegate := result.Get(0).(func(context.Context, types.User, *scimpb.Resource) (*scimpb.Resource, error))
	if isDelegate {
		return fn(ctx, u, r)
	}
	return getResultAs[*scimpb.Resource](result, 0), result.Error(1)
}

func (m *mockProviderShim) authorizeRequest(ctx context.Context, hdr string) error {
	result := m.Called(ctx, hdr)
	return result.Error(0)
}
