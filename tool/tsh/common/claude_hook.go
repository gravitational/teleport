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

package common

import (
	"encoding/json"
	"os"

	"github.com/gravitational/trace"
)

type HookInput struct {
	ToolName  string    `json:"tool_name"`
	ToolInput CurlInput `json:"tool_input"`
}

type CurlInput struct {
	AppName  string `json:"app_name"`
	CurlArgs string `json:"curl_args,omitempty"`
	UrlPath  string `json:"url_path"`
}

type HookOutput struct {
	HookSpecificOutput HookDecision `json:"hookSpecificOutput"`
}

type HookDecision struct {
	HookEventName            string                 `json:"hookEventName"`
	PermissionDecision       string                 `json:"permissionDecision"` // "allow", "ask", "deny"
	PermissionDecisionReason string                 `json:"permissionDecisionReason"`
	UpdatedInput             map[string]interface{} `json:"updatedInput,omitempty"` // Optional: modify tool input
}

func runClaudeHook(cf *CLIConf) error {
	if os.Getenv(envTSHClaudeSession) == "" {
		return trace.BadParameter("not in a tsh claude session")
	}

	var hookInput HookInput
	decoder := json.NewDecoder(cf.Stdin())
	if err := decoder.Decode(&hookInput); err != nil {
		return trace.BadParameter(err.Error(), "failed to decode hook input")
	}

	// TODO(greedy52) this hardcoded for now. should make decision based on tsh-ai-profile.yaml
	decision := "deny"
	switch hookInput.ToolInput.AppName {
	case "grafana-prod-onprem":
		decision = "ask"
	case "grafana-staging-onprem":
		decision = "allow"
	}

	return trace.Wrap(json.NewEncoder(cf.Stdout()).Encode(HookOutput{
		HookSpecificOutput: HookDecision{
			HookEventName:            hookInput.ToolName,
			PermissionDecision:       decision,
			PermissionDecisionReason: "bound by tsh AI profile configured for this session",
		},
	}))
}
