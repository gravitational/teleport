/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ingress

import (
	"crypto/tls"
	"net"
	"net/http"
	"sync"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport/lib/observability/metrics"
)

// Constants for each ingress service.
const (
	// Web is the service name for traffic ingressing through a proxy's web listener.
	Web = "web"
	// SSH is the service name for traffic ingressing through a proxy's ssh listener.
	SSH = "ssh"
	// Kube is the service name for traffic ingressing through a proxy's kube listener.
	Kube = "kube"
	// Tunnel is the service name for traffic ingressing through a proxy's reverse tunnel listener.
	Tunnel = "tunnel"
	// MySQL is the service name for traffic ingressing through a proxy's mysql listener.
	MySQL = "mysql"
	// Postgres is the service name for traffic ingressing through a proxy's postgres listener.
	Postgres = "postgres"
	// DatabaseTLS is the service name for traffic ingressing through a proxy's database tls listener.
	DatabaseTLS = "database_tls"
)

const (
	// PathDirect is the ingress path for traffic that is accepted directly by a service listener.
	PathDirect = "direct"
	// PathALPN is the ingress path for traffic that is accepeted by the alpn listener.
	PathALPN = "alpn"
	// PathUnknown is the ingress path when no other path can be identified.
	PathUnknown = "unknown"
)

var (
	commonLabels = []string{"ingress_path", "ingress_service"}

	// acceptedConnections measures connections accepted by each listener type and ingress path.
	// This allows us to differentiate between connections going through alpn routing or directly
	// to the listener.
	acceptedConnections = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "teleport",
		Name:      "accepted_connections_total",
	}, commonLabels)

	// activeConnections measures the current number of active connections.
	activeConnections = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "teleport",
		Name:      "active_connections",
	}, commonLabels)

	// authenticatedConnectionsAccepted measures the number of connections that successfully authenticated.
	authenticatedConnectionsAccepted = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "teleport",
		Name:      "authenticated_accepted_connections_total",
	}, commonLabels)

	// authenticatedConnectionsActive measures the current number of active connections that
	// successfully authenticated.
	authenticatedConnectionsActive = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "teleport",
		Name:      "authenticated_active_connections",
	}, commonLabels)
)

// HTTPConnStateReporter returns a http connection event handler function to track
// connection metrics for an http server. This only tracks tls connections.
func HTTPConnStateReporter(service string, r *Reporter) func(net.Conn, http.ConnState) {
	tracker := &sync.Map{}
	return func(conn net.Conn, state http.ConnState) {
		if r == nil {
			return
		}

		switch state {
		case http.StateActive:
			// Use underlying *tls.Conn to ensure map key is comparable to avoid
			// panics. The down side is that we can only track tls connections.
			tlsConn, ok := getTLSConn(conn)
			if !ok {
				return
			}

			_, loaded := tracker.LoadOrStore(tlsConn, struct{}{})
			// Skip connections already added to the tracker.
			if loaded {
				return
			}

			r.ConnectionAccepted(service, conn)

			// Only connections with peer certs are considered authenticated.
			if len(tlsConn.ConnectionState().PeerCertificates) == 0 {
				return
			}
			r.ConnectionAuthenticated(service, conn)
		case http.StateClosed, http.StateHijacked:
			tlsConn, ok := getTLSConn(conn)
			if !ok {
				return
			}

			_, loaded := tracker.LoadAndDelete(tlsConn)
			// Skip connections that were not tracked or already removed.
			if !loaded {
				return
			}

			defer r.ConnectionClosed(service, conn)

			// Only connections with peer certs are considered authenticated.
			if len(tlsConn.ConnectionState().PeerCertificates) == 0 {
				return
			}
			r.AuthenticatedConnectionClosed(service, conn)
		}
	}
}

// NewReporter constructs a new ingress reporter.
func NewReporter(alpnAddr string) (*Reporter, error) {
	err := metrics.RegisterPrometheusCollectors(
		acceptedConnections,
		activeConnections,
		authenticatedConnectionsAccepted,
		authenticatedConnectionsActive,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var unspecifiedIP bool
	host, port, err := net.SplitHostPort(alpnAddr)
	if err == nil {
		ip := net.ParseIP(host)
		if ip != nil {
			unspecifiedIP = ip.IsUnspecified()
		}
	}

	return &Reporter{
		alpnAddr:      alpnAddr,
		alpnPort:      port,
		unspecifiedIP: unspecifiedIP,
	}, nil
}

// Reporter provides a simple interface for tracking connection ingress metrics.
type Reporter struct {
	// alpnAddr is the host string expected for a connection ingressing through ALPN routing.
	alpnAddr string
	// alpnPort is the port string expected for a connection ingressing through ALPN routing.
	alpnPort string
	// unspecifiedIP is true if the alpnAddr is an unspecified addr (0.0.0.0, [::]).
	unspecifiedIP bool
}

// ConnectionAccepted reports a new connection, ConnectionClosed must be called when the connection closes.
func (r *Reporter) ConnectionAccepted(service string, conn net.Conn) {
	path := r.getIngressPath(conn)
	acceptedConnections.WithLabelValues(path, service).Inc()
	activeConnections.WithLabelValues(path, service).Inc()
}

// ConnectionClosed reports a closed connection. This should only be called after ConnectionAccepted.
func (r *Reporter) ConnectionClosed(service string, conn net.Conn) {
	path := r.getIngressPath(conn)
	activeConnections.WithLabelValues(path, service).Dec()
}

// ConnectionAuthenticated reports a new authenticated connection, AuthenticatedConnectionClosed must
// be called when the connection is closed.
func (r *Reporter) ConnectionAuthenticated(service string, conn net.Conn) {
	path := r.getIngressPath(conn)
	authenticatedConnectionsAccepted.WithLabelValues(path, service).Inc()
	authenticatedConnectionsActive.WithLabelValues(path, service).Inc()
}

// AuthenticatedConnectionClosed reports a closed authenticated connection, this should only be called
// after ConnectionAuthenticated.
func (r *Reporter) AuthenticatedConnectionClosed(service string, conn net.Conn) {
	path := r.getIngressPath(conn)
	authenticatedConnectionsActive.WithLabelValues(path, service).Dec()
}

// getIngressPath determines the ingress path of a given connection.
func (r *Reporter) getIngressPath(conn net.Conn) string {
	// Unwrap a proxy protocol connection to compare against the local listener address.
	addr := getRealLocalAddr(conn)

	// An empty address indicates alpn routing is disabled.
	if r.alpnAddr == "" {
		return PathDirect
	}

	// If the IP is unspecified we only check if the ports match.
	if r.unspecifiedIP {
		_, port, err := net.SplitHostPort(addr.String())
		if err != nil {
			return PathUnknown
		}
		if port == r.alpnPort {
			return PathALPN
		}
		return PathDirect
	}
	if r.alpnAddr == addr.String() {
		return PathALPN
	}
	return PathDirect
}

// getRealLocalAddr returns the underlying local address if proxy protocol is being used.
func getRealLocalAddr(conn net.Conn) net.Addr {
	for {
		connGetter, ok := conn.(netConnGetter)
		if !ok {
			break
		}
		conn = connGetter.NetConn()
	}
	return conn.LocalAddr()
}

func getTLSConn(conn net.Conn) (*tls.Conn, bool) {
	for {
		if tlsConn, ok := conn.(*tls.Conn); ok {
			return tlsConn, true
		}
		connGetter, ok := conn.(netConnGetter)
		if !ok {
			return nil, false
		}
		conn = connGetter.NetConn()
	}
}

type netConnGetter interface {
	NetConn() net.Conn
}
