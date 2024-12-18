// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package internal

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/utils/interval"
)

var (
	clientPingInitOnce sync.Once

	clientPingsTotal       *prometheus.CounterVec
	clientFailedPingsTotal *prometheus.CounterVec
)

func clientPingInit() {
	clientPingsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: teleport.MetricNamespace,
		Subsystem: "proxy_peer_client",
		Name:      "pings_total",
		Help:      "Total number of proxy peering client pings per peer proxy, both successful and failed.",
	}, []string{"local_id", "peer_id", "peer_host", "peer_group"})

	clientFailedPingsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: teleport.MetricNamespace,
		Subsystem: "proxy_peer_client",
		Name:      "failed_pings_total",
		Help:      "Total number of failed proxy peering client pings per peer proxy.",
	}, []string{"local_id", "peer_id", "peer_host", "peer_group"})
}

// ClientPingsMetricsParams contains the parameters for [ClientPingsMetrics].
type ClientPingsMetricsParams struct {
	// LocalID is the host ID of the current proxy.
	LocalID string
	// PeerID is the host ID of the peer proxy.
	PeerID string
	// PeerHost is the hostname of the peer proxy.
	PeerHost string
	// PeerGroup is the peer group ID of the peer proxy. Can be blank.
	PeerGroup string
}

// ClientPingsMetrics returns the Prometheus metrics for a given peer proxy,
// given host ID, hostname and (optionally) peer group.
func ClientPingsMetrics(params ClientPingsMetricsParams) (pings, failedPings prometheus.Counter) {
	clientPingInitOnce.Do(clientPingInit)

	pings = clientPingsTotal.WithLabelValues(params.LocalID, params.PeerID, params.PeerHost, params.PeerGroup)
	failedPings = clientFailedPingsTotal.WithLabelValues(params.LocalID, params.PeerID, params.PeerHost, params.PeerGroup)

	return pings, failedPings
}

// RunClientPing periodically pings the peer proxy reachable through the given
// [ClientConn], accumulating counts in the given Prometheus metrics. Returns
// when the context is canceled.
func RunClientPing(ctx context.Context, cc ClientConn, pings, failedPings prometheus.Counter) {
	const pingInterval = time.Minute
	ivl := interval.New(interval.Config{
		Duration:      pingInterval * 14 / 13,
		FirstDuration: retryutils.HalfJitter(pingInterval),
		Jitter:        retryutils.SeventhJitter,
	})
	defer ivl.Stop()

	for ctx.Err() == nil {
		select {
		case <-ivl.Next():
			func() {
				timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()

				err := cc.Ping(timeoutCtx)
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					failedPings.Inc()
				}
				pings.Inc()
			}()
		case <-ctx.Done():
		}
	}
}
