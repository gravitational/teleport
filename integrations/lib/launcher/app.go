package launcher

import (
	"context"
	"golang.org/x/sync/errgroup"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// minServerVersion is the minimal teleport version the plugin supports.
	minServerVersion = "6.1.0-beta.1"
	// InitTimeout is used to bound execution time of health check and teleport version check.
	initTimeout = time.Second * 10
)

// Launcher is responsible for obtaining a teleport client and launching one or multiple services.
type Launcher struct {
	PluginName     string
	teleportClient teleport.Client
	Conf           Config
	Clock          clockwork.Clock
	Services       []Service
}

// New creates a new Launcher and initialize its main job
func New(conf Config, pluginName string) *Launcher {
	baseApp := Launcher{
		PluginName: pluginName,
		Conf:       conf,
	}
	return &baseApp
}

// Run initializes the launcher, gets a teleport client, check connectivity, and start all the configured services.
func (a *Launcher) Run(ctx context.Context) error {
	// TODO: cancel context on signal
	err := a.init(ctx)
	if err != nil {
		return trace.Wrap(err, "failed initialization")
	}

	group, ctx := errgroup.WithContext(ctx)
	for _, service := range a.Services {
		group.Go(func() error {
			return service.Run(ctx, a.teleportClient, a.PluginName)
		})
	}

	return group.Wait()
}

func (a *Launcher) init(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, initTimeout)
	defer cancel()

	err := a.initTeleport(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	a.PluginName = a.Conf.PluginName()

	// Check which service the plugin supports and run their preflight checks
	return nil
}

func (a *Launcher) checkTeleportVersion(ctx context.Context) (proto.PingResponse, error) {
	pong, err := a.teleportClient.Ping(ctx)
	if err != nil {
		if trace.IsNotImplemented(err) {
			return pong, trace.Wrap(err, "server version must be at least %s", minServerVersion)
		}
		return pong, trace.Wrap(err, "Unable to get Teleport server version")
	}
	err = utils.CheckMinVersion(pong.ServerVersion, minServerVersion)
	return pong, trace.Wrap(err)
}

// initTeleport creates a Teleport client and validates Teleport connectivity.
func (a *Launcher) initTeleport(ctx context.Context) (err error) {
	clt, err := a.Conf.GetTeleportClient(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	a.teleportClient = clt
	_, err = a.checkTeleportVersion(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
