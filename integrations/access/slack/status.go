/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package slack

import (
	"github.com/gravitational/teleport/api/types"
)

// statusFromResponse tries to map a Slack API error string
// to a PluginStatus.
//
// Ref: https://github.com/slackapi/slack-api-specs/blob/bc08db49625630e3585bf2f1322128ea04f2a7f3/web-api/slack_web_openapi_v2.json
func statusFromResponse(resp *APIResponse) types.PluginStatus {
	if resp.Ok {
		return &types.PluginStatusV1{Code: types.PluginStatusCode_RUNNING}
	}

	code := types.PluginStatusCode_OTHER_ERROR
	switch resp.Error {
	case "channel_not_found", "not_in_channel":
		code = types.PluginStatusCode_SLACK_NOT_IN_CHANNEL
	case "token_expired", "not_authed", "invalid_auth", "account_inactive", "token_revoked", "no_permission", "org_login_required":
		code = types.PluginStatusCode_UNAUTHORIZED
	}
	return &types.PluginStatusV1{Code: code}
}
