/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
		trace.Wrap(err)
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
		trace.Wrap(err)
	}

	process.Supervisor.RegisterFunc(teleport.ComponentPluginManager, func() error {
		return trace.Wrap(pluginManager.Run(process.GracefulExitContext()))
	})
	return nil
}
