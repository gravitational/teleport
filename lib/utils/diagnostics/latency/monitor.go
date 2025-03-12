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

package latency

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/utils/retryutils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var logger = logutils.NewPackageLogger(teleport.ComponentKey, "latency")

// Statistics contain latency measurements for both
// legs of a proxied connection.
type Statistics struct {
	// Client measures the round trip time between the client and the Proxy.
	Client int64
	// Server measures the round trip time the Proxy and the target host.
	Server int64
}

// Reporter is an abstraction over how to provide the latency statistics to
// the consumer. Used by the Monitor to provide periodic latency updates.
type Reporter interface {
	Report(ctx context.Context, statistics Statistics) error
}

// ReporterFunc type is an adapter to allow the use of
// ordinary functions as a Reporter. If f is a function
// with the appropriate signature, Reporter(f) is a
// Reporter that calls f.
type ReporterFunc func(ctx context.Context, stats Statistics) error

// Report calls f(ctx, stats).
func (f ReporterFunc) Report(ctx context.Context, stats Statistics) error {
	return f(ctx, stats)
}

// Pinger abstracts the mechanism used to measure the round trip time of
// a connection. All "ping" messages should be responded to before returning
// from [Pinger.Ping].
type Pinger interface {
	Ping(ctx context.Context) error
}

// Monitor periodically pings both legs of a proxied connection and records
// the round trip times so that they may be emitted to consumers.
type Monitor struct {
	clientPinger Pinger
	serverPinger Pinger
	reporter     Reporter
	clock        clockwork.Clock

	pingInterval   time.Duration
	reportInterval time.Duration

	clientTimer clockwork.Timer
	serverTimer clockwork.Timer
	reportTimer clockwork.Timer

	clientLatency atomic.Int64
	serverLatency atomic.Int64
}

// MonitorConfig provides required dependencies for the [Monitor].
type MonitorConfig struct {
	// ClientPinger measure the round trip time for client half of the connection.
	ClientPinger Pinger
	// ServerPinger measure the round trip time for server half of the connection.
	ServerPinger Pinger
	// Reporter periodically emits statistics to consumers.
	Reporter Reporter
	// Clock used to measure time.
	Clock clockwork.Clock
	// InitialPingInterval an optional duration to use for the first PingInterval.
	InitialPingInterval time.Duration
	// PingInterval is the frequency at which both legs of the connection are pinged for
	// latency calculations.
	PingInterval time.Duration
	// InitialReportInterval an optional duration to use for the first ReportInterval.
	InitialReportInterval time.Duration
	// ReportInterval is the frequency at which the latency information is reported.
	ReportInterval time.Duration
}

// CheckAndSetDefaults ensures required fields are provided and sets
// default values for any omitted optional fields.
func (c *MonitorConfig) CheckAndSetDefaults() error {
	if c.ClientPinger == nil {
		return trace.BadParameter("client pinger not provided to MonitorConfig")
	}

	if c.ServerPinger == nil {
		return trace.BadParameter("server pinger not provided to MonitorConfig")
	}

	if c.Reporter == nil {
		return trace.BadParameter("reporter not provided to MonitorConfig")
	}

	if c.PingInterval <= 0 {
		c.PingInterval = 3 * time.Second
	}

	if c.InitialPingInterval <= 0 {
		c.InitialReportInterval = retryutils.FullJitter(500 * time.Millisecond)
	}

	if c.ReportInterval <= 0 {
		c.ReportInterval = 5 * time.Second
	}

	if c.InitialReportInterval <= 0 {
		c.InitialReportInterval = retryutils.HalfJitter(1500 * time.Millisecond)
	}

	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}

	return nil
}

// NewMonitor creates an unstarted [Monitor] with the provided configuration. To
// begin sampling connection latencies [Monitor.Run] must be called.
func NewMonitor(cfg MonitorConfig) (*Monitor, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Monitor{
		clientPinger:   cfg.ClientPinger,
		serverPinger:   cfg.ServerPinger,
		clientTimer:    cfg.Clock.NewTimer(cfg.InitialPingInterval),
		serverTimer:    cfg.Clock.NewTimer(cfg.InitialPingInterval),
		reportTimer:    cfg.Clock.NewTimer(cfg.InitialReportInterval),
		reportInterval: cfg.ReportInterval,
		pingInterval:   cfg.PingInterval,
		reporter:       cfg.Reporter,
		clock:          cfg.Clock,
	}, nil
}

// GetStats returns a copy of the last known latency measurements.
func (m *Monitor) GetStats() Statistics {
	return Statistics{
		Client: m.clientLatency.Load(),
		Server: m.serverLatency.Load(),
	}
}

// Run periodically records round trip times. It should not be called
// more than once.
func (m *Monitor) Run(ctx context.Context) {
	defer func() {
		m.clientTimer.Stop()
		m.serverTimer.Stop()
		m.reportTimer.Stop()
	}()

	go m.pingLoop(ctx, m.clientPinger, m.clientTimer, &m.clientLatency)
	go m.pingLoop(ctx, m.serverPinger, m.serverTimer, &m.serverLatency)

	for {
		select {
		case <-m.reportTimer.Chan():
			if err := m.reporter.Report(ctx, m.GetStats()); err != nil && !errors.Is(err, context.Canceled) {
				logger.WarnContext(ctx, "failed to report latency stats", "error", err)
			}
			m.reportTimer.Reset(retryutils.SeventhJitter(m.reportInterval))
		case <-ctx.Done():
			return
		}
	}
}

func (m *Monitor) pingLoop(ctx context.Context, pinger Pinger, timer clockwork.Timer, latency *atomic.Int64) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.Chan():
			then := m.clock.Now()
			if err := pinger.Ping(ctx); err != nil && !errors.Is(err, context.Canceled) {
				logger.WarnContext(ctx, "unexpected failure sending ping", "error", err)
			} else {
				latency.Store(m.clock.Now().Sub(then).Milliseconds())
			}
			timer.Reset(retryutils.SeventhJitter(m.pingInterval))
		}
	}
}

// SSHClient is the subset of the [ssh.Client] required by the [SSHPinger].
type SSHClient interface {
	SendRequest(ctx context.Context, name string, wantReply bool, payload []byte) (bool, []byte, error)
}

// SSHPinger is a [Pinger] implementation that measures the latency of an
// SSH connection. To calculate round trip time, a keepalive@openssh.com request
// is sent.
type SSHPinger struct {
	clt SSHClient
}

// NewSSHPinger creates a new [SSHPinger] with the provided configuration.
func NewSSHPinger(clt SSHClient) (*SSHPinger, error) {
	if clt == nil {
		return nil, trace.BadParameter("ssh client not provided to SSHPinger")
	}

	return &SSHPinger{
		clt: clt,
	}, nil
}

// Ping sends a keepalive@openssh.com request via the provided [SSHClient].
func (s *SSHPinger) Ping(ctx context.Context) error {
	_, _, err := s.clt.SendRequest(ctx, teleport.KeepAliveReqType, true, nil)
	return trace.Wrap(err, "sending request %s", teleport.KeepAliveReqType)
}

// WebSocket is the subset of [websocket.Conn] required by the [WebSocketPinger].
type WebSocket interface {
	WriteControl(messageType int, data []byte, deadline time.Time) error
	PongHandler() func(appData string) error
	SetPongHandler(h func(appData string) error)
}

// WebSocketPinger is a [Pinger] implementation that measures the latency of a
// websocket connection.
type WebSocketPinger struct {
	ws    WebSocket
	pongC chan string
	clock clockwork.Clock
}

// NewWebsocketPinger creates a [WebSocketPinger] with the provided configuration.
func NewWebsocketPinger(clock clockwork.Clock, ws WebSocket) (*WebSocketPinger, error) {
	if ws == nil {
		return nil, trace.BadParameter("web socket not provided to WebSocketPinger")
	}

	if clock == nil {
		clock = clockwork.NewRealClock()
	}

	pinger := &WebSocketPinger{
		ws:    ws,
		clock: clock,
		pongC: make(chan string, 1),
	}

	handler := ws.PongHandler()
	ws.SetPongHandler(func(payload string) error {
		select {
		case pinger.pongC <- payload:
		default:
		}

		if handler == nil {
			return nil
		}

		return trace.Wrap(handler(payload))
	})

	return pinger, nil
}

// Ping writes a ping control message and waits for the corresponding pong control message
// to be received before returning. The random identifier in the ping message is expected
// to be returned in the pong payload so that we can determine the true round trip time for
// a ping/pong message pair.
func (s *WebSocketPinger) Ping(ctx context.Context) error {
	// websocketPingMessage denotes a ping control message.
	const websocketPingMessage = 9

	payload := uuid.NewString()
	deadline := s.clock.Now().Add(2 * time.Second)
	if err := s.ws.WriteControl(websocketPingMessage, []byte(payload), deadline); err != nil {
		return trace.Wrap(err, "sending ping message")
	}

	for {
		select {
		case pong := <-s.pongC:
			if pong == payload {
				return nil
			}
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		}
	}
}
