/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package app

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport/lib/observability/metrics"
	appauthz "github.com/gravitational/teleport/lib/srv/app/authz"
	"github.com/gravitational/teleport/lib/srv/app/common"
)

// metricsSubsystem names this package's metrics under the registry's subsystem.
const metricsSubsystem = "proxy_app"

const (
	sniRouteAccepted = "accepted"
	// sniRouteError is the result of an SNI-route connection refused without an
	// app error code.
	sniRouteError = "admission-error"

	// Results that a client also sees as an app error code use the code, so
	// the same failure has one name across metrics and responses. The
	// exception is agents-unreachable, which an SNI-route client sees as
	// no-capable-replica.
	refusedMisdirected       = string(common.AppErrorCodeAuthorityMismatch)
	refusedNotImplemented    = "not-implemented"
	forwardedResult          = "forwarded"
	noAgentResult            = string(common.AppErrorCodeNoCapableReplica)
	agentsUnreachableResult  = "agents-unreachable"
	unsupportedAppResult     = string(common.AppErrorCodeUnsupportedApplicationType)
	sessionInvalidResult     = string(common.AppErrorCodeSessionInvalid)
	crossSiteWebSocketResult = "cross-site-websocket"
	redirectedResult         = "redirected"
	canceledResult           = "canceled"
	forwardErrorResult       = "error"

	dialSuccess     = "success"
	dialUnreachable = "unreachable"
	dialFailure     = "failure"
)

// accessMetrics counts the App connections the Proxy accepts or refuses, and
// how long it takes to decide.
type accessMetrics struct {
	sniRouteConnections       *prometheus.CounterVec
	sniRouteActiveConnections *prometheus.GaugeVec
	sniRouteAdmissionDuration prometheus.Histogram
}

func newAccessMetrics(reg *metrics.Registry) (*accessMetrics, error) {
	reg = reg.Wrap(metricsSubsystem)
	m := &accessMetrics{
		sniRouteConnections: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: reg.Namespace(),
			Subsystem: reg.Subsystem(),
			Name:      "sni_route_connections_total",
			Help:      "Number of SNI-route App connections the Proxy accepted or refused, by result (accepted, the app error code of a refusal, or admission-error) and by the protocol the client negotiated (h2 or http/1.1).",
		}, []string{"result", "protocol"}),
		sniRouteActiveConnections: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: reg.Namespace(),
			Subsystem: reg.Subsystem(),
			Name:      "sni_route_active_connections",
			Help:      "Number of open SNI-route App connections that the Proxy accepted, by the protocol the client negotiated (h2 or http/1.1).",
		}, []string{"protocol"}),
		sniRouteAdmissionDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: reg.Namespace(),
			Subsystem: reg.Subsystem(),
			Name:      "sni_route_admission_duration_seconds",
			Help:      "Time the Proxy took to resolve an SNI-route App connection's route and authorize it, whether it accepted or refused the connection.",
			// 1ms to about 16s, past the 10s resolution timeout, since reads
			// that miss the Proxy's caches can take seconds under load.
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 15),
		}),
	}
	err := errors.Join(
		reg.Register(m.sniRouteConnections),
		reg.Register(m.sniRouteActiveConnections),
		reg.Register(m.sniRouteAdmissionDuration),
	)
	return m, trace.Wrap(err, "registering App access metrics")
}

// handlerMetrics counts the app requests the Handler serves on both paths,
// and the connections the Proxy's transports open to Application Service
// agents.
type handlerMetrics struct {
	requests    *prometheus.CounterVec
	sharedDial  dialMetrics
	sessionDial dialMetrics
}

func newHandlerMetrics(reg *metrics.Registry) (*handlerMetrics, error) {
	reg = reg.Wrap(metricsSubsystem)
	m := &handlerMetrics{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: reg.Namespace(),
			Subsystem: reg.Subsystem(),
			Name:      "requests_total",
			Help:      "Number of App requests the Proxy served, by route (sni or app-session) and result: forwarded, canceled, error, agents-unreachable, or why the Proxy answered the request itself, which is an app error code where one exists.",
		}, []string{"route", "result"}),
		sharedDial:  newDialMetrics(reg, "shared_transport", "the shared transport", "A request that fails over dials each agent it tries."),
		sessionDial: newDialMetrics(reg, "session_transport", "app session transports for HTTP and TCP apps", "A dial that fails over dials each agent it tries."),
	}
	err := errors.Join(
		reg.Register(m.requests),
		m.sharedDial.register(reg),
		m.sessionDial.register(reg),
	)
	return m, trace.Wrap(err, "registering App handler metrics")
}

// The route label of a request: where its app route came from.
const (
	routeSNI        = "sni"
	routeAppSession = "app-session"
)

// countRequest counts an app request on route with result. Every app request
// is counted once, where its result is decided.
func (m *handlerMetrics) countRequest(route, result string) {
	m.requestsWith(route, result).Inc()
}

// requestsWith returns the counter of route's requests with result.
func (m *handlerMetrics) requestsWith(route, result string) prometheus.Counter {
	return m.requests.WithLabelValues(route, result)
}

// dialMetrics counts a transport's dials to agents, and the connections they
// return until each is closed.
type dialMetrics struct {
	dials *prometheus.CounterVec
	open  prometheus.Gauge
}

// newDialMetrics returns the dial metrics of the transport whose metric names
// start with name. who names the transport in their help, and failover says
// how a dial that fails over counts.
func newDialMetrics(reg *metrics.Registry, name, who, failover string) dialMetrics {
	return dialMetrics{
		dials: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: reg.Namespace(),
			Subsystem: reg.Subsystem(),
			Name:      name + "_dials_total",
			Help:      fmt.Sprintf("Number of reverse tunnel connections %s dialed to Application Service agents, by result: success, unreachable when the Proxy couldn't reach the agent, so it tries another agent if there is one, or failure. A dial succeeds before the TLS handshake. %s", who, failover),
		}, []string{"result"}),
		open: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: reg.Namespace(),
			Subsystem: reg.Subsystem(),
			Name:      name + "_open_connections",
			Help:      fmt.Sprintf("Number of open connections from %s to Application Service agents.", who),
		}),
	}
}

func (m dialMetrics) register(reg *metrics.Registry) error {
	return errors.Join(reg.Register(m.dials), reg.Register(m.open))
}

// dial runs dial and counts its result. It counts the connection dial returns
// in open until the connection is closed.
func (m dialMetrics) dial(dial func() (net.Conn, error)) (net.Conn, error) {
	conn, err := dial()
	m.dials.WithLabelValues(dialResult(err)).Inc()
	if err != nil {
		return nil, err
	}
	return newGaugedConn(conn, m.open), nil
}

// dialResult returns the result label of a dial to an agent that failed with
// err, or succeeded if err is nil. A dial doesn't take the request's context,
// so a client that gives up doesn't end it.
func dialResult(err error) string {
	switch {
	case err == nil:
		return dialSuccess
	case isReverseTunnelDownError(err):
		return dialUnreachable
	default:
		return dialFailure
	}
}

// sniRouteResult returns the result label of an SNI-route admission that
// returned err.
func sniRouteResult(err error) string {
	if err == nil {
		return sniRouteAccepted
	}
	if code, ok := appauthz.ForwardedErrorCode(err); ok {
		return string(code)
	}
	return sniRouteError
}

// sniRouteProtocol returns the protocol label of an SNI-route connection: h2,
// or http/1.1 for any other protocol.
func sniRouteProtocol(state tls.ConnectionState) string {
	if state.NegotiatedProtocol == "h2" {
		return "h2"
	}
	return "http/1.1"
}

// sharedForwardResult returns the result label of a request that the shared
// transport handled with err.
func sharedForwardResult(err error) string {
	switch {
	case err == nil:
		return forwardedResult
	// Checked first, since a canceled listing can also wrap errNoCapableAgent.
	// An agent that hangs usually shows here too, since the client gives up
	// first.
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return canceledResult
	// Checked before errNoCapableAgent, which it wraps.
	case errors.Is(err, errAgentsUnreachable):
		return agentsUnreachableResult
	case errors.Is(err, errNoCapableAgent):
		return noAgentResult
	case errors.Is(err, appauthz.ErrUnsupportedApplicationType):
		return unsupportedAppResult
	default:
		return forwardErrorResult
	}
}

// sessionForwardResult returns the result label of a request on the app
// session route that an app session's transport handled with err.
func sessionForwardResult(err error) string {
	switch {
	case err == nil:
		return forwardedResult
	case errors.Is(err, ErrInvalidAppSession):
		return sessionInvalidResult
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return canceledResult
	case errors.Is(err, errAgentsUnreachable):
		return agentsUnreachableResult
	// The transport reports an app with no app server, or none it could list,
	// this way.
	case trace.IsConnectionProblem(err):
		return noAgentResult
	default:
		return forwardErrorResult
	}
}

// gaugedConn counts a connection in a gauge until it is first closed.
type gaugedConn struct {
	net.Conn
	gauge prometheus.Gauge
	once  sync.Once
}

func newGaugedConn(conn net.Conn, gauge prometheus.Gauge) *gaugedConn {
	gauge.Inc()
	return &gaugedConn{Conn: conn, gauge: gauge}
}

// Close closes the connection, and takes it out of the gauge the first time,
// since net/http may close a connection more than once.
func (c *gaugedConn) Close() error {
	c.once.Do(c.gauge.Dec)
	return c.Conn.Close()
}

// NetConn returns the connection c wraps, so that findConn and the ingress
// metrics see through it.
func (c *gaugedConn) NetConn() net.Conn {
	return c.Conn
}
