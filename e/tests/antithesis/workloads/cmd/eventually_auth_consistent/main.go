// eventually_auth_consistent is an Antithesis command that checks the items as served by the Auth match the backend.
// Note that we assume the cache is healthy, if not another assertion will fire. In timelines in which the cache is unhealthy
// this assertion still verifies the auth correctly serves resources as seen by the backend. There is a per item allowance of time
// for propagation of events. This method is not perfect, if we were running in process we could assert that the collection matches
// the local service but Antithesis commands run out of process as standalone binaries.
//
// TODO(okraport): In the future it would be handy if the event service provided a way to seed all watched
// items within the same stream to avoid the initialization dance. It would also guarantee that all items are
// fetched from the same auth in the case of client side loadbalancing techniques which we currently cannot do.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/antithesishq/antithesis-sdk-go/assert"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/e/tests/antithesis/workloads/lib/crud"
	"github.com/gravitational/teleport/e/tests/antithesis/workloads/lib/eventually"
	"github.com/gravitational/teleport/lib/backend"
	_ "github.com/gravitational/teleport/lib/backend/pgbk"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils"
	stacksignal "github.com/gravitational/teleport/lib/utils/signal"
	tctlclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

type localServiceClient struct {
	*local.AccessService
	*local.AppService
	*local.AccessMonitoringRulesService
	*local.CrownJewelsService
	*local.DatabaseService
	*local.DiscoveryConfigService
	*local.DynamicWindowsDesktopService
	*local.GitServerService
	*local.HealthCheckConfigService
	*local.IntegrationsService
	*local.IdentityService
	*local.KubernetesService
	*local.LinuxDesktopService
	*local.SAMLIdPServiceProviderService
	*local.StaticHostUserService
	*local.UserGroupService
	*local.WindowsDesktopService
}

func main() {
	level := slog.LevelDebug
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: &level})))

	err := run(context.Background())
	// Signal to the fuzzer this checker has to succeed at least some times. Error here means the setup
	// for the assertion itself has failed. We want to build confidence that the assertions themselves have ran
	// at least sometimes.
	assert.Sometimes(err == nil, "sometimes eventually auth consistent setup succeeds", map[string]any{})
	if err != nil {
		fmt.Fprint(os.Stdout, utils.UserMessageFromError(err))
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	ctx, cancel := stacksignal.GetSignalHandler().NotifyContext(ctx)
	defer cancel()

	var ccf tctlcfg.GlobalCLIFlags
	cfg := servicecfg.MakeDefaultConfig()

	if configFileEnv, ok := os.LookupEnv(defaults.ConfigFileEnvar); ok {
		ccf.ConfigFile = configFileEnv
	} else if utils.FileExists(defaults.ConfigFilePath) {
		ccf.ConfigFile = defaults.ConfigFilePath
	}

	if ccf.ConfigFile == "" {
		return trace.BadParameter("config file is missing")
	}

	// We borrow the tctl client since we run in the same container reusing the auth local identity.
	clt, close, err := tctlclient.GetInitFunc(ccf, cfg)(ctx)
	if err != nil {
		return trace.Wrap(err, "creating auth client")
	}
	defer close(ctx)

	bc := cfg.Auth.StorageConfig // reuse local config
	bk, err := backend.New(ctx, bc.Type, bc.Params)
	if err != nil {
		return trace.Wrap(err, "creating backend")
	}
	defer bk.Close()

	backendClient, err := newLocalServiceClient(bk)
	if err != nil {
		return trace.Wrap(err, "creating backend CRUD client")
	}

	wg, ctx := errgroup.WithContext(ctx)
	for _, kind := range crud.DefaultKinds() {
		authOps, err := crud.OpsForKind(clt, kind)
		if err != nil {
			return trace.Wrap(err, "getting auth ops for %q", kind)
		}

		backendOps, err := crud.OpsForKind(backendClient, kind)
		if err != nil {
			return trace.Wrap(err, "getting backend ops for %q", kind)
		}

		const cacheConsistencyTimeout = 5 * time.Minute
		wg.Go(func() error {
			return trace.Wrap(eventually.Assert(ctx, eventually.AssertParams{
				Message: "auth resources are eventually consistent with the backend",
				Timeout: cacheConsistencyTimeout,
				Details: map[string]any{"kind": kind},
				Condition: func(ctx context.Context, _ eventually.AddDetailFunc) (bool, error) {
					authItems, err := crud.ListAll(ctx, authOps, 0)
					if err != nil {
						return false, trace.Wrap(err, "listing auth resources")
					}

					backendItems, err := crud.ListAll(ctx, backendOps, 0)
					if err != nil {
						return false, trace.Wrap(err, "listing backend resources")
					}

					equal := crud.EqualResourceSlices(backendItems, authItems, backendOps.Equal)
					return equal, nil
				},
			}))
		})
	}

	if err = wg.Wait(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func newLocalServiceClient(bk backend.Backend) (localServiceClient, error) {
	accessMonitoringRules, err := local.NewAccessMonitoringRulesService(bk)
	if err != nil {
		return localServiceClient{}, trace.Wrap(err, "creating access monitoring rules service")
	}

	crownJewels, err := local.NewCrownJewelsService(bk)
	if err != nil {
		return localServiceClient{}, trace.Wrap(err, "creating crown jewels service")
	}

	discoveryConfigs, err := local.NewDiscoveryConfigService(bk)
	if err != nil {
		return localServiceClient{}, trace.Wrap(err, "creating discovery config service")
	}

	dynamicWindowsDesktops, err := local.NewDynamicWindowsDesktopService(bk)
	if err != nil {
		return localServiceClient{}, trace.Wrap(err, "creating dynamic windows desktop service")
	}

	gitServers, err := local.NewGitServerService(bk)
	if err != nil {
		return localServiceClient{}, trace.Wrap(err, "creating git server service")
	}

	healthCheckConfigs, err := local.NewHealthCheckConfigService(bk)
	if err != nil {
		return localServiceClient{}, trace.Wrap(err, "creating health check config service")
	}

	integrations, err := local.NewIntegrationsService(bk)
	if err != nil {
		return localServiceClient{}, trace.Wrap(err, "creating integrations service")
	}

	identity, err := local.NewIdentityService(bk)
	if err != nil {
		return localServiceClient{}, trace.Wrap(err, "creating identity service")
	}

	linuxDesktops, err := local.NewLinuxDesktopService(bk)
	if err != nil {
		return localServiceClient{}, trace.Wrap(err, "creating linux desktop service")
	}

	samlIdPServiceProviders, err := local.NewSAMLIdPServiceProviderService(bk)
	if err != nil {
		return localServiceClient{}, trace.Wrap(err, "creating SAML IdP service provider service")
	}

	staticHostUsers, err := local.NewStaticHostUserService(bk)
	if err != nil {
		return localServiceClient{}, trace.Wrap(err, "creating static host user service")
	}

	userGroups, err := local.NewUserGroupService(bk)
	if err != nil {
		return localServiceClient{}, trace.Wrap(err, "creating user group service")
	}

	kubeService, err := local.NewKubernetesService(bk)
	if err != nil {
		return localServiceClient{}, trace.Wrap(err, "creating kube service")
	}

	return localServiceClient{
		AccessService:                 local.NewAccessService(bk),
		AppService:                    local.NewAppService(bk),
		AccessMonitoringRulesService:  accessMonitoringRules,
		CrownJewelsService:            crownJewels,
		DatabaseService:               local.NewDatabasesService(bk),
		DiscoveryConfigService:        discoveryConfigs,
		DynamicWindowsDesktopService:  dynamicWindowsDesktops,
		GitServerService:              gitServers,
		HealthCheckConfigService:      healthCheckConfigs,
		IntegrationsService:           integrations,
		IdentityService:               identity,
		KubernetesService:             kubeService,
		LinuxDesktopService:           linuxDesktops,
		SAMLIdPServiceProviderService: samlIdPServiceProviders,
		StaticHostUserService:         staticHostUsers,
		UserGroupService:              userGroups,
		WindowsDesktopService:         local.NewWindowsDesktopService(bk),
	}, nil
}
