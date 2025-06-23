package identitycenter

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	identitycentercommon "github.com/gravitational/teleport/e/lib/aws/identitycenter/common"
	ictest "github.com/gravitational/teleport/e/lib/aws/identitycenter/test"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
)

// runService starts a new IC service instance using resources in the supplied
// test fixture, returning a function that will stop the service and wait until the
// service exits. The service will be automatically stopped when the test
// completes.
func runTestService(t *testing.T, ctx context.Context, fixture *ictest.Fixture) (*Service, context.CancelFunc) {
	waitCh := make(chan struct{})
	svcCtx, cancel := context.WithCancel(ctx)
	svc := newTestService(t, fixture)
	go func() {
		svc.Run(svcCtx)
		close(waitCh)
	}()

	stopAndWait := func() {
		cancel()
		<-waitCh
	}
	t.Cleanup(stopAndWait)

	return svc, stopAndWait
}

type testServiceOption func(*ServiceConfig)

func withRolesSyncMode(m RolesSyncMode) testServiceOption {
	return func(cfg *ServiceConfig) {
		cfg.RolesSyncMode = m
	}
}

// newTestService creates a new test instance from the supplied fixture.
func newTestService(t *testing.T, fixture *ictest.Fixture, options ...testServiceOption) *Service {
	// Ideally, newTestService wold be defined in the `identitycenter/test` sub-
	// package, but doing so would cause circular imports that it's probably not
	// worth rearranging things to avoid.

	plugin, err := fixture.GetPluginResource()
	if err != nil {
		fixture.CreatePluginResource(t)
		plugin = fixture.MustGetPluginResource(t)
	}

	details := plugin.Spec.GetAwsIc()

	cfg := ServiceConfig{
		Provisioning: ProvisioningConfig{
			SCIMClient:          fixture.SCIMClient,
			StateSvc:            fixture.Auth.Services,
			StateSvcCache:       fixture.Auth.Cache,
			UsersSvcCache:       fixture.Auth.Cache,
			AccessListsSvcCache: fixture.Auth.Cache,
			LocksSvc:            fixture.Auth.Services,
		},
		ICClient:                   fixture.ICClient,
		UsersSvc:                   fixture.Auth.Services,
		AccessListsSvc:             fixture.Auth.Services,
		AccessRequestsSvc:          fixture.Auth.Services,
		Clock:                      fixture.Clock,
		EventsClient:               fixture.Auth.Services,
		IdentityCenterDataSvc:      fixture.Auth.Services,
		IdentityCenterDataSvcCache: fixture.Auth.Cache,
		Log:                        slog.Default().With("test", t.Name(), teleport.ComponentKey, eteleport.ComponentAWSIC),
		RolesSvc:                   fixture.Auth.Services,
		ImportConfig: ImportConfig{
			AccessListDefaultOwners: []string{"user1", "user2"},
			GroupSyncFilter:         details.GroupSyncFilters,
		},
		PluginsService:   fixture.PluginService,
		PluginStatusSink: fixture.PluginStatusSink,
		UserPredicate:    identitycentercommon.UserPredicateFilter(nil),
		Emitter:          fixture.Emitter,
		RolesSyncMode:    RolesSyncModeAll,
	}

	for _, optionFn := range options {
		optionFn(&cfg)
	}

	svc, err := NewService(cfg)
	require.NoError(t, err, "creating Identity Center service")
	return svc
}
