package identitycenter

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	identitycentercommon "github.com/gravitational/teleport/e/lib/aws/identitycenter/common"
	ictest "github.com/gravitational/teleport/e/lib/aws/identitycenter/test"
	"github.com/gravitational/teleport/e/lib/provisioning"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/integrations/access/common"
	icfilters "github.com/gravitational/teleport/lib/aws/identitycenter/filters"
)

// runNewTestService starts a new IC service instance using resources in the supplied
// test fixture, returning a function that will stop the service and wait until the
// service exits. The service will be automatically stopped when the test
// completes. You MUST manually invoke the shutdown function if the test service
// is started inside a `synctest` bubble, otherwise the test will deadlock when
// the bubble is destroyed.
// It is considered a test failure if the service exits with error.
func runNewTestService(t *testing.T, ctx context.Context, fixture *ictest.Fixture, options ...testServiceOption) (*Service, context.CancelFunc) {
	svc := newTestService(t, fixture, options...)
	waitCh := make(chan error)
	svcCtx, cancel := context.WithCancel(ctx)
	go func() {
		waitCh <- svc.Run(svcCtx)
	}()

	stopAndWait := sync.OnceFunc(func() {
		cancel()
		select {
		case err := <-waitCh:
			require.NoError(t, err, "IC service must exit cleanly")
		case <-time.After(1 * time.Minute):
			require.Fail(t, "IC service shutdown timed out")
		}
	})
	t.Cleanup(stopAndWait)

	return svc, stopAndWait
}

type testServiceOption func(*ServiceConfig)

func withRolesSyncMode(m RolesSyncMode) testServiceOption {
	return func(cfg *ServiceConfig) {
		cfg.RolesSyncMode = m
	}
}

func withLogger(logger *slog.Logger) testServiceOption {
	return func(cfg *ServiceConfig) {
		cfg.Log = logger
	}
}

func withStatusSink(s common.StatusSink) testServiceOption {
	return func(cfg *ServiceConfig) {
		cfg.PluginStatusSink = s
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

	groupSyncFilter, err := icfilters.New(details.GroupSyncFilters)
	require.NoError(t, err, "compiling group sync filters")

	cfg := ServiceConfig{
		Provisioning: ProvisioningConfig{
			SCIMClient:           fixture.SCIMClient,
			StateSvc:             fixture.Auth.Services,
			StateSvcCache:        fixture.Auth.Services,
			UsersSvcCache:        fixture.Auth.Services,
			AccessListsSvcCache:  fixture.Auth.Services,
			LocksSvc:             fixture.Auth.Services,
			StateRefreshInterval: 500 * time.Millisecond,
			UserProvisioningMode: provisioning.UserProvisioningModeInternal,
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
			GroupSyncFilter:         groupSyncFilter,
		},
		PluginsService:     fixture.PluginService,
		PluginStatusSink:   fixture.PluginStatusSink,
		UserPredicate:      identitycentercommon.UserPredicateFilter(nil),
		Emitter:            fixture.Emitter,
		RolesSyncMode:      RolesSyncModeAll,
		EventBatchDuration: 50 * time.Millisecond,
	}

	for _, optionFn := range options {
		optionFn(&cfg)
	}

	svc, err := NewService(cfg)
	require.NoError(t, err, "creating Identity Center service")
	return svc
}
