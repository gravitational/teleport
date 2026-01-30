package plugins

import (
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services/local"
)

// RegisterPluginManager starts and registers the plugin manager.
func RegisterPluginManager(oauthProviders servicecfg.PluginOAuthProviders, process *service.TeleportProcess) error {
	// Start plugin manager
	log := slog.With(teleport.ComponentKey, eteleport.ComponentPluginManager)
	authorizers := NewAuthorizerSetFromConfig(oauthProviders, log)
	pluginStaticCredentialsService, err := local.NewPluginStaticCredentialsService(process.GetBackend())
	if err != nil {
		return trace.Wrap(err)
	}
	pluginManager, err := NewManager(ManagerConfig{
		Authorizers:             authorizers,
		Plugins:                 process.GetAuthServer().Services,
		PluginStaticCredentials: pluginStaticCredentialsService,
		Events:                  process.GetAuthServer().Services,
		TeleportClient:          process.GetAuthServer(),
		ParentProcess:           process,
		Logger:                  log,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	process.Supervisor.RegisterFunc(eteleport.ComponentPluginManager, func() error {
		return trace.Wrap(pluginManager.Run(process.GracefulExitContext()))
	})
	return nil
}
