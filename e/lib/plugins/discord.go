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
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/discord"
	"github.com/gravitational/teleport/integrations/lib/logger"
)

func discordInstanceFactory(ctx context.Context, plugin *types.PluginV1, deps instanceDependencies) (func() error, error) {
	discordSpec := plugin.Spec.GetDiscord()
	if discordSpec == nil {
		return nil, trace.BadParameter("field Spec.Discord must be present")
	}

	if len(deps.staticCredentials) == 0 {
		return nil, trace.BadParameter("missing plugin static credentials")
	}

	recipients := make(common.RawRecipientsMap)
	for role, channels := range discordSpec.RoleToRecipients {
		recipients[role] = channels.ChannelIds
	}

	cfg := &discord.Config{
		BaseConfig: common.BaseConfig{
			Recipients: recipients,
		},
		Discord: common.GenericAPIConfig{
			Token: deps.staticCredentials[0].GetAPIToken(),
		},
		Client:     deps.client,
		StatusSink: deps.statusSink,
	}

	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err, "set discord defaults")
	}

	app := discord.NewApp(cfg)
	appCtx := logger.WithLogger(deps.lifetime, deps.log)
	return func() error {
		err := app.Run(appCtx)
		return trace.Wrap(err)
	}, nil
}
