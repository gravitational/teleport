package plugins

import (
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services/local"
)

// RegisterPluginManager starts and registers the plugin manager.
func RegisterPluginManager(oauthProviders servicecfg.PluginOAuthProviders, process *service.TeleportProcess) error {
	// Start plugin manager
	authorizers := NewAuthorizerSetFromConfig(oauthProviders)
	pluginsService := local.NewPluginsService(process.GetBackend())
	pluginStaticCredentialsService, err := local.NewPluginStaticCredentialsService(process.GetBackend())
	if err != nil {
		return trace.Wrap(err)
	}
	pluginManager, err := NewManager(ManagerConfig{
		Authorizers:             authorizers,
		Plugins:                 pluginsService,
		PluginStaticCredentials: pluginStaticCredentialsService,
		Events:                  process.GetAuthServer().Services,
		TeleportClient:          process.GetAuthServer(),
		ParentProcess:           process,

		Log: logrus.WithFields(logrus.Fields{
			trace.Component: teleport.ComponentPluginManager,
		}),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	process.Supervisor.RegisterFunc(teleport.ComponentPluginManager, func() error {
		return trace.Wrap(pluginManager.Run(process.GracefulExitContext()))
	})
	return nil
}
