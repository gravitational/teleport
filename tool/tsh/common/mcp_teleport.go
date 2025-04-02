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
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	libevents "github.com/gravitational/teleport/lib/events"
)

func onMCPStartTeleport(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	var clusterClient *client.ClusterClient
	if err := client.RetryWithRelogin(cf.Context, tc, func() error {
		clusterClient, err = tc.ConnectToCluster(cf.Context)
		return trace.Wrap(err)
	}); err != nil {
		return trace.Wrap(err)
	}
	defer clusterClient.Close()

	authClient := clusterClient.AuthClient

	mcpServer := server.NewMCPServer("teleport_tools", teleport.Version)
	mcpServer.AddTool(
		mcp.NewTool(
			"teleport_search_events",
			mcp.WithDescription(`Search Teleport audit events.

Teleport is the easiest, most secure way to access and protect all your infrastructure.

Teleport logs cluster activity by emitting various events into its audit log. 

The tool takes in two mandatory parameters "from" and "to" which are the
searching time range. The time must be in RFC3339 formats. 
An optional "start_key"" param can be used to perform pagination where returned
by previous call.

The response is a list of audit events found in that time period, maximum 100
per call. If more events are available, it will return a "next_key"" to be used
as "start_key"" in the next call for pagination.
`),
			mcp.WithString("from", mcp.Required(), mcp.Description("oldest date of returned events, in RFC3339 format")),
			mcp.WithString("to", mcp.Required(), mcp.Description("newest date of returned events, in RFC3339 format")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			fromStr, ok := request.Params.Arguments["from"].(string)
			if !ok {
				return nil, trace.BadParameter("missing string parameter 'from'")
			}
			toStr, ok := request.Params.Arguments["to"].(string)
			if !ok {
				return nil, trace.BadParameter("missing string parameter 'to'")
			}
			from, err := time.Parse(time.RFC3339, fromStr)
			if err != nil {
				return nil, trace.Wrap(err, "failed to parse 'from' as RFC3339 format")
			}
			to, err := time.Parse(time.RFC3339, toStr)
			if err != nil {
				return nil, trace.Wrap(err, "failed to parse 'to' as RFC3339 format")
			}
			req := libevents.SearchEventsRequest{
				From:  from,
				To:    to,
				Limit: 100,
			}
			startKey, ok := request.Params.Arguments["start_key"].(string)
			if ok {
				req.StartKey = startKey
			}

			events, nextKey, err := authClient.SearchEvents(cf.Context, req)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			result, err := json.Marshal(map[string]any{
				"events":   events,
				"next_key": nextKey,
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return mcp.NewToolResultText(string(result)), nil
		},
	)

	mcpServer.AddTool(
		mcp.NewTool(
			"teleport_access_request",
			mcp.WithDescription(`Create Teleport access request.

The tool takes a mandatory "role" parameter that indicates a Teleport role
an access request should be submitted for.
`),
			mcp.WithString("role", mcp.Required(), mcp.Description("role name to request")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			role, ok := request.Params.Arguments["role"].(string)
			if !ok {
				return nil, trace.BadParameter("missing string parameter 'role'")
			}

			accessRequest, err := types.NewAccessRequest(
				uuid.NewString(),
				tc.Username,
				role)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			createdRequest, err := authClient.CreateAccessRequestV2(cf.Context, accessRequest)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			result, err := json.Marshal(createdRequest)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			return mcp.NewToolResultText(string(result)), nil
		},
	)

	return trace.Wrap(
		server.NewStdioServer(mcpServer).Listen(cf.Context, cf.Stdin(), cf.Stdout()),
	)
}
