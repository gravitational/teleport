package monitor

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/common"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter/test"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// principalValidator is used to validate a principal when checking events
// emitted by the resource monitors. Any testify checks on the principal must be
// from `testify/assert`; anything from `require` will immediately panic the
// test on first failed assertion.
type principalValidator func(*assert.CollectT, types.Resource)

// nilPrincipal asserts that the event principal is nil
func nilPrincipal(t *assert.CollectT, principal types.Resource) {
	assert.Nil(t, principal, "expected principal to be nil")
}

// withPrincipal asserts that the principal attached to the event is of the same
// kind, and has the same name, as the `expected` resource.
func withPrincipal(expected types.Resource) principalValidator {
	return func(c *assert.CollectT, principal types.Resource) {
		// we may be testing against a delete event with a phoney principal
		// resource containing only metadata, so test by kind and name rather
		// than underlying type
		assert.NotNil(c, expected, "expected may not be nil")
		if principal != nil {
			assert.Equal(c, expected.GetKind(), principal.GetKind())
			assert.Equal(c, expected.GetName(), principal.GetName())
		}
	}
}

func requireEvent(ch <-chan *PrincipalEvent, verb Verb, validatePrincipal principalValidator) func(*assert.CollectT) {
	return func(c *assert.CollectT) {
		select {
		case event := <-ch:
			assert.Equal(c, verb, event.Verb)
			validatePrincipal(c, event.Principal)
		default:
			assert.Fail(c, "No event in queue")
		}
	}
}

func TestResourceMonitor(t *testing.T) {
	slog.SetLogLoggerLevel(slog.LevelDebug)
	slog.SetDefault(
		slog.New(logutils.NewSlogTextHandler(
			os.Stderr, logutils.SlogTextHandlerConfig{Level: slog.LevelDebug})))

	modulestest.SetTestModules(t, modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.AccessLists: {Enabled: true},
			},
		},
	})

	logger := slog.Default().With("test", t.Name())

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// GIVEN a test cluster
	fixture := test.NewFixture(t, test.WithStartedCache)
	eventCh := make(chan *PrincipalEvent)
	defer close(eventCh)

	// These cluster resources must be created before the watcher starts, so
	// the have to be created up here rather than the more logical place closer
	// to the actual tests.

	// GIVEN some pre-existing users
	admin, err := types.NewUser("admin")
	require.NoError(t, err)
	admin, err = fixture.Auth.Services.CreateUser(ctx, admin)
	require.NoError(t, err)

	leto, err := types.NewUser("leto@atreides.duchy.ar")
	require.NoError(t, err)
	leto, err = fixture.Auth.Services.CreateUser(ctx, leto)
	require.NoError(t, err)

	// GIVEN some pre-existing roles
	adminRole, err := types.NewRole("IdentityCenterAdmin", types.RoleSpecV6{})
	require.NoError(t, err)
	adminRole, err = fixture.Auth.Services.CreateRole(ctx, adminRole)
	require.NoError(t, err)

	readOnlyRole, err := types.NewRole("ReadOnly", types.RoleSpecV6{})
	require.NoError(t, err)
	readOnlyRole, err = fixture.Auth.Services.CreateRole(ctx, readOnlyRole)
	require.NoError(t, err)

	// GIVEN and Access List
	adminACL, err := fixture.Auth.Services.UpsertAccessList(ctx, test.AccessList{
		Name:          "administrators",
		Title:         "Anyone",
		Owners:        []types.User{admin},
		GrantsOwners:  []types.Role{adminRole},
		GrantsMembers: []types.Role{adminRole},
	}.Build(t))
	require.NoError(t, err)

	// GIVEN a monitor on the cluster
	monitorUnderTest, err := New(Config{
		Events:              fixture.Auth,
		AccessListsSvcCache: fixture.Auth.Cache,
		UsersSvcCache:       fixture.Auth.Cache,
		Logger:              logger,
		Clock:               fixture.Clock,
		OnEvent: func(ctx context.Context, event *PrincipalEvent) {
			select {
			case <-ctx.Done():
			case eventCh <- event:
			}
		},
	})
	require.NoError(t, err, "creating resource monitor")
	require.NotNil(t, monitorUnderTest, "resource monitor must not be nil")

	go monitorUnderTest.Watch(ctx)

	require.EventuallyWithT(t,
		requireEvent(eventCh, VerbCalculateAll, nilPrincipal),
		10*time.Second, 100*time.Millisecond,
		"monitor must issue targeted recalc event on user creation")

	t.Run("Users", func(t *testing.T) {
		// WHEN I Create a new user...
		paul, err := types.NewUser("paul@atreides.duchy.ar")
		require.NoError(t, err)
		paul, err = fixture.Auth.Services.CreateUser(ctx, paul)
		require.NoError(t, err)

		// EXPECT that the  monitor will issue a recalculate event for that user
		require.EventuallyWithT(t,
			requireEvent(eventCh, VerbCalculate, withPrincipal(paul)),
			10*time.Second, 100*time.Millisecond,
			"monitor must issue targeted recalc event on user creation")

		// WHEN I modify that user
		paul.SetOrigin(common.OriginAWSIdentityCenter)
		paul, err = fixture.Auth.Services.UpdateUser(ctx, paul)
		require.NoError(t, err)

		// EXPECT that the  monitor will issue another recalculate event for that user
		require.EventuallyWithT(t,
			requireEvent(eventCh, VerbCalculate, withPrincipal(paul)),
			10*time.Second, 100*time.Millisecond,
			"monitor must issue targeted recalc event on user creation")

		// WHEN I delete that user
		err = fixture.Auth.Services.DeleteUser(ctx, paul.GetName())
		require.NoError(t, err)

		// EXPECT that the  monitor will issue a delete request for that user
		require.EventuallyWithT(t,
			requireEvent(eventCh, VerbDelete, withPrincipal(paul)),
			10*time.Second, 100*time.Millisecond,
			"monitor must issue targeted delete event on user deletion")
	})

	t.Run("AccessLists", func(t *testing.T) {
		// WHEN I Create a new Access List...
		acl, err := fixture.Auth.Services.UpsertAccessList(ctx, test.AccessList{
			Name:          "test-access-list",
			Title:         "Test Access List",
			Owners:        []types.User{admin},
			GrantsOwners:  []types.Role{adminRole},
			GrantsMembers: []types.Role{readOnlyRole},
		}.Build(t))
		require.NoError(t, err)

		// EXPECT that the  monitor will issue a recalculate event for that
		// access list
		require.EventuallyWithT(t,
			requireEvent(eventCh, VerbCalculate, withPrincipal(acl)),
			10*time.Second, 100*time.Millisecond,
			"monitor must issue targeted recalc event on Access List creation")

		// WHEN I edit the access list...
		acl.Metadata.Labels = map[string]string{"a": "alpha", "b": "beta"}
		acl, err = fixture.Auth.Services.UpsertAccessList(ctx, acl)
		require.NoError(t, err)

		// EXPECT that the  monitor will issue a recalculate event for that
		// access list
		require.EventuallyWithT(t,
			requireEvent(eventCh, VerbCalculate, withPrincipal(acl)),
			10*time.Second, 100*time.Millisecond,
			"monitor must issue targeted recalc event on Access List edit")

		// WHEN I delete the Access List...
		err = fixture.Auth.Services.DeleteAccessList(ctx, acl.GetName())
		require.NoError(t, err)

		// EXPECT that the  monitor will issue *another* recalculate event for the
		// owning access list
		require.EventuallyWithT(t,
			requireEvent(eventCh, VerbDelete, withPrincipal(acl)),
			10*time.Second, 100*time.Millisecond,
			"monitor must issue delete event on Access List deletion")
	})

	t.Run("AccessListMembers", func(t *testing.T) {
		// WHEN I add a member to an access list...
		member, err := fixture.Auth.Services.UpsertAccessListMember(ctx, test.AccessListMember{
			Member:     leto,
			AccessList: adminACL,
			AddedBy:    admin,
		}.Build(t))
		require.NoError(t, err)

		// EXPECT that the  monitor will issue a recalculate event for the
		// owning access list
		require.EventuallyWithT(t,
			requireEvent(eventCh, VerbCalculate, withPrincipal(adminACL)),
			10*time.Second, 100*time.Millisecond,
			"monitor must issue targeted recalc event on Access List Member creation")

		// WHEN I update the Access List member
		member.Spec.Reason = "He's just this guy, you know?"
		member, err = fixture.Auth.Services.UpsertAccessListMember(ctx, member)
		require.NoError(t, err)

		// EXPECT that the  monitor will issue *another* recalculate event for
		// the owning access list
		require.EventuallyWithT(t,
			requireEvent(eventCh, VerbCalculate, withPrincipal(adminACL)),
			10*time.Second, 100*time.Millisecond,
			"monitor must issue targeted recalc event on Access List member edit")

		// WHEN I delete the member from the access list...
		err = fixture.Auth.Services.DeleteAccessListMember(ctx, adminACL.GetName(), member.GetName())
		require.NoError(t, err)

		// EXPECT that the  monitor will issue a final recalculate event for
		// the owning access list
		require.EventuallyWithT(t,
			requireEvent(eventCh, VerbCalculate, withPrincipal(adminACL)),
			10*time.Second, 100*time.Millisecond,
			"monitor must issue targeted recalc event on Access List member edit")
	})

	t.Run("Roles", func(t *testing.T) {
		// WHEN I create a role...
		auditor, err := types.NewRole("IdentityCenterAuditor", types.RoleSpecV6{})
		require.NoError(t, err)
		auditor, err = fixture.Auth.Services.CreateRole(ctx, auditor)
		require.NoError(t, err)

		// EXPECT that the  monitor will issue a full recalculate event
		require.EventuallyWithT(t,
			requireEvent(eventCh, VerbCalculateAll, nilPrincipal),
			10*time.Second, 100*time.Millisecond,
			"monitor must issue a full recalc event on role creation")

		// WHEN I edit a role...
		auditor.SetSubKind("some-subkind")
		// auditor, err = fixture.Auth.GetRole(ctx, auditor.GetName())
		// require.NoError()
		auditor, err = fixture.Auth.Services.UpdateRole(ctx, auditor)
		require.NoError(t, err)

		// EXPECT that the  monitor will issue a full recalculate event
		require.EventuallyWithT(t,
			requireEvent(eventCh, VerbCalculateAll, nilPrincipal),
			10*time.Second, 100*time.Millisecond,
			"monitor must issue a full recalc event on role update")

		// WHEN I delete a role...
		err = fixture.Auth.Services.DeleteRole(ctx, auditor.GetName())
		require.NoError(t, err)

		// EXPECT that the  monitor will issue a full recalculate event
		require.EventuallyWithT(t,
			requireEvent(eventCh, VerbCalculateAll, nilPrincipal),
			10*time.Second, 100*time.Millisecond,
			"monitor must issue a full recalc event on role deletion")
	})
}
