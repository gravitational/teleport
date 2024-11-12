package identitycenter

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	ictest "github.com/gravitational/teleport/e/lib/aws/identitycenter/test"
)

// newTestService creates a new test instance from the supplied fixture.
func newTestService(t *testing.T, fixture *ictest.Fixture) *Service {
	// Ideally, newTestService wold be defined in the `identitycenter/test` sub-
	// package, but doing so would cause circular imports that it's probably not
	// worth rearranging things to avoid.

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
		Log:                        slog.Default().With("test", t.Name()),
		RolesSvc:                   fixture.Auth.Services,
		ImportConfig: ImportConfig{
			AccessListDefaultOwners: []string{"user1", "user2"},
		},
		PluginsService:   fixture.PluginService,
		PluginStatusSink: fixture.PluginStatusSink,
	}

	svc, err := NewService(cfg)
	require.NoError(t, err, "creating Identity Center service")
	return svc
}
