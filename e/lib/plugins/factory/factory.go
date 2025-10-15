package factory

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

// Dependencies is a container for dependencies of a plugin instance.
type Dependencies struct {
	Authorizer oauth.Authorizer
	// Client is a cached Teleport client (for resources that support it).
	// If used in conjunction with an event watcher, the watcher must also
	// feed from cache (as opposed to the auth.Services backend changefeed).
	Client            teleport.Client
	Store             storage.Store
	StatusSink        common.StatusSink
	ParentProcess     *service.TeleportProcess
	StaticCredentials []types.PluginStaticCredentials
	Logger            *slog.Logger
	PluginsService    services.Plugins
	// HTTP client to be used by Plugin.
	// Leave as `nil` for the plugins to use their own defaults.
	HTTPClient *http.Client
}

// Delegate describes a function that runs a plugin instance.
type Delegate func(context.Context) error

// Factory takes plugin spec and its dependencies, and returns a function that
// runs the plugin instance.
//
// The [ctx] argument is intended to be only used for managing cancellation within
// the life of the factory call. Lifetime management of the resulting plugin should
// use the context supplied to the resulting [Delegate].
type Factory func(ctx context.Context, plugin *types.PluginV1, deps Dependencies) (Delegate, error)
