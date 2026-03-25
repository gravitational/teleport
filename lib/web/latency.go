/*
 * *
 *  * Teleport
 *  * Copyright (C) 2024 Gravitational, Inc.
 *  *
 *  * This program is free software: you can redistribute it and/or modify
 *  * it under the terms of the GNU Affero General Public License as published by
 *  * the Free Software Foundation, either version 3 of the License, or
 *  * (at your option) any later version.
 *  *
 *  * This program is distributed in the hope that it will be useful,
 *  * but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *  * GNU Affero General Public License for more details.
 *  *
 *  * You should have received a copy of the GNU Affero General Public License
 *  * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package web

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/lib/utils/diagnostics/latency"
)

// monitorLatency implements the Web UI's latency detector.
// It runs as long as the provided context has not expired.
//
// The latency of the provided websocket is monitored automatically,
// and the latency to the target endpoint is monitored with the provided pinger.
// The results of the latency calculation are reported to the web UI
// with the provided reporter.
func monitorLatency(
	ctx context.Context,
	clock clockwork.Clock,
	ws latency.WebSocket,
	endpointPinger latency.Pinger,
	reporter latency.Reporter,
) error {
	wsPinger, err := latency.NewWebsocketPinger(clock, ws)
	if err != nil {
		return trace.Wrap(err, "creating websocket pinger")
	}

	monitor, err := latency.NewMonitor(latency.MonitorConfig{
		ClientPinger: wsPinger,
		ServerPinger: endpointPinger,
		Reporter:     reporter,
		Clock:        clock,
	})
	if err != nil {
		return trace.Wrap(err, "creating latency monitor")
	}

	monitor.Run(ctx)
	return nil
}
