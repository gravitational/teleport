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

package mcp

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/mcptest"
)

func Test_handleStdioToSSE(t *testing.T) {
	testServerSSEEndpoint := mcptest.MustStartSSETestServer(t)

	app, err := types.NewAppV3(types.Metadata{
		Name: "test-sse",
	}, types.AppSpecV3{
		URI: fmt.Sprintf("mcp+sse+%s", testServerSSEEndpoint),
	})
	require.NoError(t, err)

	ctx := t.Context()
	emitter := eventstest.MockRecorderEmitter{}
	s, err := NewServer(ServerConfig{
		Emitter:       &emitter,
		ParentContext: ctx,
		HostID:        "my-host-id",
		AccessPoint:   fakeAccessPoint{},
		CipherSuites:  utils.DefaultCipherSuites(),
		AuthClient:    mockAuthClient{},
	})
	require.NoError(t, err)

	testCtx := setupTestContext(t, withAdminRole(t), withApp(app))
	handleDoneCh := make(chan struct{}, 1)
	go func() {
		handlerErr := s.HandleSession(ctx, testCtx.SessionCtx)
		handleDoneCh <- struct{}{}
		assert.NoError(t, handlerErr)
	}()

	// Use a real client. Double check start event has the external MCP session
	// ID.
	stdioClient := mcptest.NewStdioClientFromConn(t, testCtx.clientSourceConn)
	var startEvent *apievents.MCPSessionStart
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		var ok bool
		event := emitter.LastEvent()
		startEvent, ok = event.(*apievents.MCPSessionStart)
		require.True(t, ok)
	}, time.Second*5, time.Millisecond*100, "expect session start")
	require.NotEmpty(t, startEvent.McpSessionId)

	resp := mcptest.MustInitializeClient(t, stdioClient)
	require.Equal(t, "test-server", resp.ServerInfo.Name)

	// Make a tools call.
	mcptest.MustCallServerTool(t, stdioClient)

	// Now close the client.
	stdioClient.Close()
	select {
	case <-time.After(time.Second * 5):
		require.Fail(t, "timed out waiting for handler")
	case <-handleDoneCh:
	}
}
