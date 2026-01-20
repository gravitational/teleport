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
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"github.com/gravitational/trace"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/client/identityfile"
	"github.com/gravitational/teleport/lib/utils"
)

func runClaudeMCP(cf *CLIConf) error {
	_, err := initLogger(cf, utils.LoggingForMCP, getLoggingOptsForMCPServer(cf))
	if err != nil {
		return trace.Wrap(err)
	}
	logger.InfoContext(cf.Context, "== TSH_CLAUDE_SESSION", "TSH_CLAUDE_SESSION", os.Getenv("TSH_CLAUDE_SESSION"))

	mcpServer := mcpserver.NewMCPServer(
		"tsh",
		teleport.Version,
		mcpserver.WithInstructions(`This MCP server is intended to be used for AI coding tools
like claude and codex. It replaces the need to run "tsh" commands in most
situations. The MCP server is also scoped to a subset of resources the user can
access via "tsh". The scoped profile can be updated manually or adjusted with
"tsh code approve".`),
	)

	mcpServer.AddTool(
		mcp.NewTool(
			"login",
			mcp.WithDescription("Initiate login for Teleport tsh sessions."),
			mcp.WithString("proxy_addr", mcp.Description("Teleport proxy address, e.g. teleport.example.com:443"), mcp.Required()),
			mcp.WithString("user", mcp.Description("Teleport user name. not required for SSO login.")),
			mcp.WithBoolean("sso", mcp.Description("True for SSO login")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			proxyAddr := request.GetString("proxy_addr", "")
			if proxyAddr == "" {
				return nil, trace.BadParameter("proxy_addr is required")
			}

			// TODO(greedy52) fix the default
			user := request.GetString("user", "admin")
			cf.Username = user
			cf.Proxy = proxyAddr
			cf.IdentityFormat = identityfile.DefaultFormat

			sso := request.GetBool("sso", false)
			if sso {
				cfClone := *cf
				go func() {
					if err := onLogin(&cfClone); err != nil {
						slog.DebugContext(ctx, trace.DebugReport(err))
					}
				}()
				result := "Initiating login via Browser. Once completed, run the status tool to verify login is successful."
				return &mcp.CallToolResult{
					Content: []mcp.Content{mcp.NewTextContent(result)},
				}, nil
			}

			//TODO(greedy52) use vnet for now. we need to support login
			//TODO(greedy52) RFD 0233
			cmd := exec.CommandContext(ctx, "open", fmt.Sprintf("teleport://%s@%s/vnet", user, proxyAddr))
			if err := cmd.Run(); err != nil {
				return nil, trace.Wrap(err, "failed to open Teleport Connect login")
			}
			result := "Initiating login via Teleport Connect. Once completed, run the status tool to verify login is successful."
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.NewTextContent(result)},
			}, nil
		},
	)
	mcpServer.AddTool(
		mcp.NewTool(
			"status",
			mcp.WithDescription("Show Teleport/tsh login status"),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// TODO(greedy52) support multiple profiles
			profile, profiles, err := cf.FullProfileStatus()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if profile == nil {
				return &mcp.CallToolResult{
					Content: []mcp.Content{mcp.NewTextContent("not logged in")},
				}, nil
			}
			env := getTshEnv()

			active, others := makeAllProfileInfo(profile, profiles, env)
			// Reduce some info
			if active != nil {
				active.Traits = nil
				active.Roles = nil
			}
			for _, p := range others {
				p.Traits = nil
				p.Roles = nil
			}

			result, err := serializeProfiles(active, others, env, teleport.JSON)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.NewTextContent(result)},
			}, nil
		},
	)
	mcpServer.AddTool(
		mcp.NewTool(
			"curl",
			mcp.WithDescription("run curl against HTTP apps"),
			mcp.WithString("app_name", mcp.Description("app name to curl against"), mcp.Required()),
			mcp.WithString("url_path", mcp.Description("Url path to curl without the domain. Example: /path/to/curl.")),
			mcp.WithArray("curl_args", mcp.Description("all other args that will be passed to curl")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			cfClone := *cf
			cfClone.AppName = request.GetString("app_name", "")
			if cfClone.AppName == "" {
				return nil, trace.BadParameter("app_name is required")
			}
			cfClone.DatabaseUser = request.GetString("url_path", "")
			cfClone.AWSCommandArgs = request.GetStringSlice("curl_args", nil)
			output := new(bytes.Buffer)
			outputSynced := utils.NewSyncWriter(output)
			cfClone.overrideStdin = strings.NewReader("")
			cfClone.OverrideStdout = outputSynced
			cfClone.overrideStderr = outputSynced
			if err := onAppCurl(&cfClone); err != nil {
				return nil, trace.Wrap(err)
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.NewTextContent(output.String())},
			}, nil
		},
	)

	return mcpserver.NewStdioServer(mcpServer).Listen(cf.Context, cf.Stdin(), cf.Stdout())
}
