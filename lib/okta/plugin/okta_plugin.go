/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package oktaplugin

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// Get fetches the Okta plugin if it exists and does proper type assertions.
func Get(ctx context.Context, plugins PluginGetter, withSecrets bool) (*types.PluginV1, error) {
	plugin, err := plugins.GetPlugin(ctx, types.PluginTypeOkta, withSecrets)
	if err != nil {
		return nil, trace.Wrap(err, "getting Okta plugin")
	}
	pluginV1, ok := plugin.(*types.PluginV1)
	if !ok {
		return nil, trace.BadParameter("plugin.(%T) is not of type PluginV1", plugin)
	}

	oktaSettings := pluginV1.Spec.GetOkta()
	if oktaSettings == nil {
		return nil, trace.BadParameter("plugin %q does not have Okta settings", plugin.GetName())
	}
	if oktaSettings.SyncSettings == nil {
		oktaSettings.SyncSettings = &types.PluginOktaSyncSettings{}
	}

	return pluginV1, nil
}
