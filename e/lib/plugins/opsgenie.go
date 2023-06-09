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
	"github.com/gravitational/teleport/integrations/access/opsgenie"
)

func opsgenieInstanceFactory(ctx context.Context, plugin *types.PluginV1, deps instanceDependencies) (func() error, error) {
	opsgenieSpec := plugin.Spec.GetOpsgenie()
	if opsgenieSpec == nil {
		return nil, trace.BadParameter("field Spec.Opsgenie must be present")
	}

	if len(deps.staticCredentials) == 0 {
		return nil, trace.BadParameter("static credentials must be present")
	}

	// For now, we'll just choose the first static credential until we have a need for rotation or other complexity.
	staticToken := deps.staticCredentials[0].GetAPIToken()
	if staticToken == "" {
		return nil, trace.BadParameter("api token is empty")
	}

	opsgenieConfig := plugin.Spec.GetOpsgenie()
	pc := &pluginConfiguration{
		client: deps.client,
		pluginConfig: &opsgenie.Config{
			ClientConfig: opsgenie.ClientConfig{
				APIKey:           staticToken,
				APIEndpoint:      opsgenieConfig.ApiEndpoint,
				DefaultSchedules: opsgenieConfig.DefaultSchedules,
				Priority:         opsgenieConfig.Priority,
			},
			StatusSink: deps.statusSink,
		},
	}

	app := common.NewApp(pc, plugin.GetName())
	return func() error {
		err := app.Run(ctx)
		return trace.Wrap(err)
	}, nil
}
