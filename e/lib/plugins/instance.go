package plugins

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/common/auth/oauth"
	"github.com/gravitational/teleport/integrations/access/common/auth/storage"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
)

// instanceDependencies is a container for dependencies of a plugin instance.
type instanceDependencies struct {
	lifetime          context.Context
	authorizer        oauth.Authorizer
	client            teleport.Client
	store             storage.Store
	statusSink        common.StatusSink
	parentProcess     *service.TeleportProcess
	staticCredentials []types.PluginStaticCredentials
	logger            *slog.Logger
	pluginsService    services.Plugins
	// HTTP client to be used by Plugin.
	// Leave as `nil` for the plugins to use their own defaults.
	HTTPClient *http.Client
}

// instanceFactory takes plugin spec and its dependencies,
// and returns a function that runs the plugin instance.
//
// The ctx argument is intended to be only used for managing
// cancellation within the life of the factory call. Lifetime
// management of the resulting plugin should use the context
// supplied by `deps.lifetime`
type instanceFactory func(ctx context.Context, plugin *types.PluginV1, deps instanceDependencies) (func() error, error)

// instance represents a single plugin instance.
type instance struct {
	// cancel is a function that closes the instance's context
	cancel func()

	// plugin is a copy of the resource that was used to configure the running plugin instance
	plugin *types.PluginV1

	// staticCredentials is the list of known credential at startup time.
	staticCredentials []*types.PluginStaticCredentialsV1
}

func (i *instance) isUpToDate(p *types.PluginV1) bool {
	return i.plugin.Spec.Equal(p.Spec) && i.plugin.Credentials.Equal(p.Credentials)
}

func (i *instance) findCredentialByName(name string) *types.PluginStaticCredentialsV1 {
	for _, c := range i.staticCredentials {
		if c.GetName() == name {
			return c
		}
	}
	return nil
}
