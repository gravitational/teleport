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
