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

package reversetunnel

import (
	"net"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
)

type dialType string

const (
	// dialTypeDirect is a direct dialed connection.
	dialTypeDirect dialType = "direct"
	// dialTypePeer is a connection established through a peer proxy.
	dialTypePeer dialType = "peer"
	// dialTypeTunnel is a connection established over a local reverse tunnel initiated
	// by a client.
	dialTypeTunnel dialType = "tunnel"
	// dialTypePeerTunnel is a connection established over a local reverse tunnel
	// initiated by a peer proxy.
	dialTypePeerTunnel dialType = "peer-tunnel"
)

// metricConn reports metrics for reversetunnel connections.
type metricConn struct {
	net.Conn
	clock clockwork.Clock

	// start is the time since the last state was reported.
	start    time.Time
	firstUse sync.Once
	dialType dialType
}

// newMetricConn returns a new metricConn
func newMetricConn(conn net.Conn, dt dialType, start time.Time, clock clockwork.Clock) *metricConn {
	c := &metricConn{
		Conn:     conn,
		dialType: dt,
		start:    start,
		clock:    clock,
	}

	connLatency.WithLabelValues(string(c.dialType), "established").Observe(c.duration().Seconds())
	return c
}

// duration returns the duration since c.start and updates c.start to now.
func (c *metricConn) duration() time.Duration {
	now := c.clock.Now()
	d := now.Sub(c.start)
	c.start = now
	return d
}

// Read wraps a net.Conn.Read to report the time between the connection being established
// and the connection being used.
func (c *metricConn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	c.firstUse.Do(func() {
		connLatency.WithLabelValues(string(c.dialType), "first_read").Observe(c.duration().Seconds())
	})
	return n, err
}

var (
	connLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "reversetunnel_conn_latency",
			Help: "Latency metrics for reverse tunnel connections",
			// lowest bucket start of upper bound 0.001 sec (1 ms) with factor 2
			// highest bucket start of 0.001 sec * 2^15 == 32.768 sec
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
		},
		[]string{"dial_type", "state"},
	)
)
