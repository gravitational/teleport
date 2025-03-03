/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

// TestTimeReconciliation launches two instances with clock differences in system clock,
// to verify that global notification is created about time drifting.
func TestTimeReconciliation(t *testing.T) {
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)
	helpers.SetTestTimeouts(2 * time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start Teleport Auth and Proxy services
	authProcess, proxyProcess, provisionToken := helpers.MakeTestServers(t)
	authService := authProcess.GetAuthServer()
	proxyAddr, err := proxyProcess.ProxyWebAddr()
	require.NoError(t, err)

	cfg := servicecfg.MakeDefaultConfig()
	agentClock := clockwork.NewFakeClockAt(time.Now().Add(24 * time.Hour))
	cfg.Clock = agentClock
	agent := helpers.MakeAgentServer(t, cfg, *proxyAddr, provisionToken)
	require.NotNil(t, agent)

	err = retryutils.RetryStaticFor(30*time.Second, time.Second, func() error {
		agentClock.Advance(time.Minute)
		notifications, _, err := authService.ListGlobalNotifications(ctx, 100, "")
		if err != nil {
			return trace.Wrap(err)
		}

		var found bool
		for _, notification := range notifications {
			found = found || notification.GetMetadata().GetName() == "cluster-monitor-system-clock-warning"
		}
		if !found {
			return trace.BadParameter("expected notification is not found")
		}
		return nil
	})
	require.NoError(t, err)
}
