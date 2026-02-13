// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	debugclient "github.com/gravitational/teleport/lib/client/debug"
)

func TestHandleProcessInfo(t *testing.T) {
	t.Parallel()

	s := &LocalSupervisor{
		clock: clockwork.NewRealClock(),
		services: []Service{
			NewLocalService("auth.tls", func() error { return nil }, WithDebugInfo(func(context.Context) (string, error) {
				return "auth_service:\n  enabled: true\n", nil
			})),
			NewLocalService("proxy.web", func() error { return nil }, WithDebugInfo(func(context.Context) (string, error) {
				return "", errors.New("unable to load service config")
			})),
			NewLocalService("ssh.node", func() error { return nil }),
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/process", nil)
	resp := httptest.NewRecorder()
	s.HandleProcessInfo(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)

	var got debugclient.ProcessInfo
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &got))
	require.Positive(t, got.PID)
	require.False(t, got.CollectedAt.IsZero())
	require.Len(t, got.ServiceDebugInfo, 3)

	require.True(t, got.ServiceDebugInfo["auth.tls"].HasInfo)
	require.Equal(t, "auth.tls", got.ServiceDebugInfo["auth.tls"].ServiceName)
	require.Equal(t, "auth_service:\n  enabled: true\n", got.ServiceDebugInfo["auth.tls"].ServiceConfig)

	require.False(t, got.ServiceDebugInfo["proxy.web"].HasInfo)
	require.Equal(t, "proxy.web", got.ServiceDebugInfo["proxy.web"].ServiceName)
	require.Equal(t, "unable to load service config", got.ServiceDebugInfo["proxy.web"].Error)

	require.False(t, got.ServiceDebugInfo["ssh.node"].HasInfo)
	require.Equal(t, "ssh.node", got.ServiceDebugInfo["ssh.node"].ServiceName)
}
