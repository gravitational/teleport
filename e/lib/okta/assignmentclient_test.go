package okta

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gravitational/trace"
	"github.com/okta/okta-sdk-golang/v2/okta"
	oktaquery "github.com/okta/okta-sdk-golang/v2/okta/query"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/utils/set"
)

// purgeCache clears the assignmentClient caches, forcing the client to reload
// everything from the upstream Okta service. Used only in tests.
func (a *assignmentClient) purgeCache() {
	clear(a.users)
	a.initUsersOnce = sync.Once{}
	a.apps.Clear()
	a.groups.Clear()
}

func firstVal[T, U any](t T, _ U) T {
	return t
}

func TestAssignmentClient(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	log := slog.With(teleport.ComponentKey, eteleport.ComponentOkta)
	testGroup := oktaGroupID("test-group")
	testApp := oktaAppID("test-app")
	testUser := userName("test-user@test.user")
	testOktaUserID := oktaUserID("okta-user-id")

	// Factory for creating an assignment client backed by a test Okta client
	// pre-configured with a user and some group and app memberships.
	testClientWithAssignments := func() (*testOktaClient, *assignmentClient) {
		oktaClient := newTestOktaClient()

		oktaClient.UsernamesToUserIDs.Store(testUser, testOktaUserID)
		oktaClient.AppsToUsers.Store(testApp, set.New(oktaapi.AppAssignment{UserID: string(testOktaUserID), Scope: oktaapi.UserScope}))
		oktaClient.GroupsToUsers.Store(testGroup, set.New(testOktaUserID))
		oktaClient.AppsToGroups = map[oktaAppID][]oktaGroupID{
			testApp: {testGroup},
		}
		assignmentClient := newAssignmentClient(log, oktaClient)

		return oktaClient, assignmentClient
	}

	t.Run("no such user is an error", func(t *testing.T) {
		// Given an Okta system with no users or groups...
		assignmentClient := newAssignmentClient(log, newTestOktaClient())

		// When I attempt to perform operations that require a given
		// user to exist, those operations will fail

		_, err := assignmentClient.userAssignedToApp(ctx, testUser, testApp)
		require.ErrorIs(t, err, trace.NotFound("unable to find ID for user %s", testUser))

		_, err = assignmentClient.userAssignedToGroup(ctx, testUser, testGroup)
		require.ErrorIs(t, err, trace.NotFound("unable to find ID for user %s", testUser))
	})

	t.Run("user with no apps no groups", func(t *testing.T) {
		// Given an assignmentClient backed by an Okta system with one user and
		// no apps or groups configured...
		oktaClient := newTestOktaClient()
		oktaClient.UsernamesToUserIDs.Store(testUser, testOktaUserID)
		assignmentClient := newAssignmentClient(log, oktaClient)

		// When I test that user's group and app memberships, the tests all
		// return NotFound because the Apps and Groups do not exist .

		_, err := assignmentClient.userAssignedToApp(ctx, testUser, testApp)
		require.ErrorIs(t, err, trace.NotFound("assignments for app %s not found", testApp))

		_, err = assignmentClient.userAssignedToGroup(ctx, testUser, testGroup)
		require.ErrorIs(t, err, trace.NotFound("assignments for group %s not found", testGroup))
	})

	t.Run("assignments and caching", func(t *testing.T) {
		// Given an assignmentClient backed by an Okta system with one user, one
		// group and one app configured, but the user is not assigned to either...
		oktaClient := newTestOktaClient()
		oktaClient.UsernamesToUserIDs.Store(testUser, testOktaUserID)
		oktaClient.AppsToUsers.Store(testApp, set.New[oktaapi.AppAssignment]())
		oktaClient.GroupsToUsers.Store(testGroup, set.New[oktaapi.OktaUserID]())

		assignmentClient := newAssignmentClient(log, oktaClient)

		// When we test for membership, the operations succeed and return `false`.
		isAssigned, err := assignmentClient.userAssignedToApp(ctx, testUser, testApp)
		require.NoError(t, err)
		require.False(t, isAssigned)

		isAssigned, err = assignmentClient.userAssignedToGroup(ctx, testUser, testGroup)
		require.NoError(t, err)
		require.False(t, isAssigned)

		// When we manually make the user and group assignments in the back-end
		// test client and re-test the membership, expect that the
		// assignmentClient uses cached data rather than re-querying the back
		// end, and so still reports `false`.
		oktaClient.AppsToUsers.Write(func(m map[oktaapi.OktaAppID]set.Set[oktaapi.AppAssignment]) {
			m[testApp].Add(oktaapi.AppAssignment{UserID: string(testOktaUserID), Scope: oktaapi.UserScope})
		})
		oktaClient.GroupsToUsers.Write(func(m map[oktaGroupID]set.Set[oktaUserID]) {
			m[testGroup].Add(testOktaUserID)
		})

		isAssigned, err = assignmentClient.userAssignedToApp(ctx, testUser, testApp)
		require.NoError(t, err)
		require.False(t, isAssigned)

		isAssigned, err = assignmentClient.userAssignedToGroup(ctx, testUser, testGroup)
		require.NoError(t, err)
		require.False(t, isAssigned)

		// When we purge the assignmentClient cache and retry the membership
		// tests, expect that the assignmentClient fetches new values from Okta
		// and so now reports `true`
		assignmentClient.purgeCache()

		isAssigned, err = assignmentClient.userAssignedToApp(ctx, testUser, testApp)
		require.NoError(t, err)
		require.True(t, isAssigned)

		isAssigned, err = assignmentClient.userAssignedToGroup(ctx, testUser, testGroup)
		require.NoError(t, err)
		require.True(t, isAssigned)
	})

	t.Run("reassignment", func(t *testing.T) {
		// Given an assignmentClient backed by an Okta system with one user, one
		// group and one app configured, and the user is assigned to both...
		oktaClient, assignmentClient := testClientWithAssignments()

		// When I attempt to disassociate a user from an app or group,
		// expect the operations to succeed and subsequent membership
		// tests to return `false`
		require.NoError(t, assignmentClient.unregisterUserFromApp(ctx, testUser, testApp))
		ok, err := assignmentClient.userAssignedToApp(ctx, testUser, testApp)
		require.NoError(t, err)
		require.False(t, ok)

		require.NoError(t, assignmentClient.unregisterUserFromGroup(ctx, testUser, testGroup))
		ok, err = assignmentClient.userAssignedToGroup(ctx, testUser, testGroup)
		require.NoError(t, err)
		require.False(t, ok)

		// Also expect that the change has been passed through to the okta
		// client
		require.Empty(t, firstVal(oktaClient.GroupsToUsers.Load(testGroup)))
		require.Empty(t, firstVal(oktaClient.AppsToUsers.Load(testApp)))

		// When I attempt to re-associate a user to an app or group,
		// expect the operations to succeed and subsequent membership
		// tests to return `true`

		require.NoError(t, assignmentClient.registerUserToApp(ctx, testUser, testApp))
		ok, err = assignmentClient.userAssignedToApp(ctx, testUser, testApp)
		require.NoError(t, err)
		require.True(t, ok)

		require.NoError(t, assignmentClient.registerUserToGroup(ctx, testUser, testGroup))
		ok, err = assignmentClient.userAssignedToGroup(ctx, testUser, testGroup)
		require.NoError(t, err)
		require.True(t, ok)
	})

	t.Run("validation errors are not propagated", func(t *testing.T) {
		// Given an assignment client with pre-existing memberships...
		oktaClient, assignmentClient := testClientWithAssignments()

		// When calls to the underlying client fail with a oktaAPIValidationError
		oktaClient.UnassignAppErr[testApp] = &oktaapi.OktaAPIValidationError{}
		oktaClient.UnassignGroupErr[testGroup] = &oktaapi.OktaAPIValidationError{}

		// Expect that the oktaAPIValidationError is treated as a success, the
		// error is *NOT* propagated from the underlying Okta client, and the
		// user assignments are updated as requested
		require.NoError(t, assignmentClient.unregisterUserFromApp(ctx, testUser, testApp))
		isAssigned, err := assignmentClient.userAssignedToApp(ctx, testUser, testApp)
		require.NoError(t, err)
		require.False(t, isAssigned)

		require.NoError(t, assignmentClient.unregisterUserFromGroup(ctx, testUser, testGroup))
		isAssigned, err = assignmentClient.userAssignedToGroup(ctx, testUser, testGroup)
		require.NoError(t, err)
		require.False(t, isAssigned)
	})

	t.Run("not found errors are not propagated", func(t *testing.T) {
		// Given an assignment client with pre-existing memberships...
		oktaClient, assignmentClient := testClientWithAssignments()

		// When calls to the underlying client fail with a NotFound error
		oktaClient.UnassignAppErr[testApp] = trace.WithField(trace.NotFound("summary"), oktaapi.OktaErrorID, oktaapi.OktaErrCodeNotFoundException)
		oktaClient.UnassignGroupErr[testGroup] = trace.WithField(trace.NotFound("summary"), oktaapi.OktaErrorID, oktaapi.OktaErrCodeNotFoundException)

		// Expect that the NotFound is treated as a success, the
		// error is *NOT* propagated from the underlying Okta client, and the
		// user assignments are updated as requested
		require.NoError(t, assignmentClient.unregisterUserFromApp(ctx, testUser, testApp))
		isAssigned, err := assignmentClient.userAssignedToApp(ctx, testUser, testApp)
		require.NoError(t, err)
		require.False(t, isAssigned)

		require.NoError(t, assignmentClient.unregisterUserFromGroup(ctx, testUser, testGroup))
		isAssigned, err = assignmentClient.userAssignedToGroup(ctx, testUser, testGroup)
		require.NoError(t, err)
		require.False(t, isAssigned)
	})

	t.Run("unassignment errors are propagated", func(t *testing.T) {
		// Given an assignment client with pre-existing memberships...
		oktaClient, assignmentClient := testClientWithAssignments()

		// When calls to the underlying client fail...
		oktaClient.UnassignAppErr[testApp] = trace.BadParameter("bad parameter")
		oktaClient.UnassignGroupErr[testGroup] = trace.BadParameter("bad parameter")

		// Expect that general errors are propagated from the underlying Okta
		// client, and that the assignments have not been affected.

		require.Error(t, assignmentClient.unregisterUserFromApp(ctx, testUser, testApp))
		isAssigned, err := assignmentClient.userAssignedToApp(ctx, testUser, testApp)
		require.NoError(t, err)
		require.True(t, isAssigned)

		require.Error(t, assignmentClient.unregisterUserFromGroup(ctx, testUser, testGroup))
		isAssigned, err = assignmentClient.userAssignedToGroup(ctx, testUser, testGroup)
		require.NoError(t, err)
		require.True(t, isAssigned)
	})

	t.Run("okta error during initialization", func(t *testing.T) {
		assignmentClient := newAssignmentClient(log, &badOktaUserLister{err: errors.New("nope")})
		require.NotPanics(t, func() {
			_, _ = assignmentClient.userID(context.Background(), "bob")
		})
	})

	t.Run("nil map returned by ListUsers", func(t *testing.T) {
		assignmentClient := newAssignmentClient(log, &badOktaUserLister{err: nil})
		require.NotPanics(t, func() {
			_, _ = assignmentClient.userID(context.Background(), "bob")
		})
	})
}

type badOktaUserLister struct {
	oktaapi.Interface
	err error
}

func (b *badOktaUserLister) ListUsers(ctx context.Context, paramOpts ...oktaquery.ParamOptions) (map[oktaapi.UserName]oktaapi.OktaUserID, error) {
	if len(paramOpts) > 0 {
		// listing deactivated users
		return map[oktaapi.UserName]oktaapi.OktaUserID{"alice": "al1ce"}, nil
	}
	return nil, b.err
}

// testAssignmentOktaServer is a fixture for testing parallel calls to the Okta
// server. Creating
type testAssignmentOktaServer struct {
	t                *testing.T
	httpServer       *httptest.Server
	appsCallsCount   atomic.Int64
	group1CallsCount atomic.Int64
	group2CallsCount atomic.Int64
}

func newTestAssignmentOktaServer(t *testing.T) *testAssignmentOktaServer {
	fixture := &testAssignmentOktaServer{t: t}
	fixture.httpServer = httptest.NewTLSServer(fixture)
	t.Cleanup(func() { fixture.httpServer.Close() })

	return fixture
}

func (ts *testAssignmentOktaServer) client(t *testing.T, ctx context.Context) oktaapi.Interface {
	_, oktaClient, err := okta.NewClient(ctx,
		okta.WithHttpClientPtr(ts.httpServer.Client()),
		okta.WithCache(false),
		okta.WithOrgUrl(ts.httpServer.URL),
		okta.WithToken("test"),
	)
	require.NoError(t, err)

	return oktaapi.NewForAPIClient(oktaapi.NewAPIClient(oktaClient))
}

func (ts *testAssignmentOktaServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var payload any
	switch r.URL.Path {
	case "/api/v1/users":
		payload = []*okta.User{
			{Id: "userid1", Profile: &okta.UserProfile{oktaapi.OktaUserProfileLogin: "username1"}},
			{Id: "userid2", Profile: &okta.UserProfile{oktaapi.OktaUserProfileLogin: "username2"}},
		}
	case "/api/v1/apps/testApp/users":
		ts.appsCallsCount.Add(1)
		payload = []*okta.AppUser{
			{Id: "userid1", Scope: "GROUP"},
		}
	case "/api/v1/groups/testGroup1/users":
		ts.group1CallsCount.Add(1)
		payload = []*okta.User{
			{Id: "userid1"},
		}
	case "/api/v1/groups/testGroup2/users":
		ts.group2CallsCount.Add(1)
		payload = []*okta.User{
			{Id: "userid1"},
		}

	default:
		ts.t.Fatalf("unexpected URL %s", r.URL.Path)
	}

	w.Header().Add("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func TestClientGetAssignedAppsGroups(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	const numOfParallelCalls = 20

	log := slog.With(teleport.ComponentKey, eteleport.ComponentOkta)

	t.Run("get assigned app for user concurrent calls", func(t *testing.T) {
		testServer := newTestAssignmentOktaServer(t)
		assignmentClient := newAssignmentClient(log, testServer.client(t, ctx))

		// Test the Okta API is called only once for the same user and app
		// when multiple concurrent calls are made to the assignmentClient
		// for the same user and app.
		var wg sync.WaitGroup
		for range numOfParallelCalls {
			wg.Go(func() {
				ok, err := assignmentClient.userAssignedToApp(ctx, "username1", "testApp")
				assert.NoError(t, err)
				assert.True(t, ok)
			})
		}
		wg.Wait()

		require.Equal(t, int64(1), testServer.appsCallsCount.Load())
		require.Equal(t, int64(0), testServer.group1CallsCount.Load())
		require.Equal(t, int64(0), testServer.group2CallsCount.Load())
	})

	t.Run("get assigned groups for user concurrent calls", func(t *testing.T) {
		testServer := newTestAssignmentOktaServer(t)
		assignmentClient := newAssignmentClient(log, testServer.client(t, ctx))

		// Test the Okta API is called only once for the same user and group
		// when multiple concurrent calls are made to the assignmentClient
		// for the same user and app.
		var wg sync.WaitGroup
		for range numOfParallelCalls {
			wg.Add(2)
			go func() {
				defer wg.Done()
				ok, err := assignmentClient.userAssignedToGroup(ctx, "username1", "testGroup1")
				require.NoError(t, err)
				require.True(t, ok)
			}()
			go func() {
				defer wg.Done()
				ok, err := assignmentClient.userAssignedToGroup(ctx, "username1", "testGroup2")
				require.NoError(t, err)
				require.True(t, ok)
			}()
		}
		wg.Wait()

		require.Equal(t, int64(0), testServer.appsCallsCount.Load())
		require.Equal(t, int64(1), testServer.group1CallsCount.Load())
		require.Equal(t, int64(1), testServer.group2CallsCount.Load())
	})

	t.Run("get assigned for username2", func(t *testing.T) {
		testServer := newTestAssignmentOktaServer(t)
		assignmentClient := newAssignmentClient(log, testServer.client(t, ctx))

		// Check if for other user the Okta API will not be called again and cached value will be used.
		ok, err := assignmentClient.userAssignedToGroup(ctx, "username2", "testGroup1")
		require.NoError(t, err)
		require.False(t, ok)

		ok, err = assignmentClient.userAssignedToApp(ctx, "username2", "testApp")
		require.NoError(t, err)
		require.False(t, ok)

		require.Equal(t, int64(1), testServer.group1CallsCount.Load())
		require.Equal(t, int64(0), testServer.group2CallsCount.Load())
		require.Equal(t, int64(1), testServer.appsCallsCount.Load())
	})
}
