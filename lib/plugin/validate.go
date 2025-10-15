// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package plugin

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// Validate validates the plugin before writing it in the storage.
func Validate(plugin types.Plugin) error {
	if plugin == nil {
		return nil
	}
	switch plugin.GetType() {
	case types.PluginTypeOkta:
		pluginV1, err := toPluginV1(plugin)
		if err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(validateOkta(pluginV1))
	}
	return nil
}

func toPluginV1(plugin types.Plugin) (*types.PluginV1, error) {
	pluginV1, ok := plugin.(*types.PluginV1)
	if !ok {
		return nil, trace.BadParameter("plugin.(%T) %q is not of type PluginV1", plugin, plugin.GetName())
	}
	return pluginV1, nil
}

func validateOkta(plugin *types.PluginV1) error {
	oktaSettings := plugin.Spec.GetOkta()
	if oktaSettings == nil {
		return trace.BadParameter("plugin %q does not have Okta settings", plugin.GetName())
	}

	_, err := OktaParseTimeBetweenImports(oktaSettings.GetSyncSettings())
	return trace.Wrap(err)
}
