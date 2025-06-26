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

package proxy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/gravitational/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/net/http2"
	"k8s.io/client-go/transport"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/kube/internal"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/utils"
)

// transportForRequest returns a transport that can be used to dial the next hop
// for the provided request using Impersonation.
func (f *Forwarder) transportForRequest(sess *clusterSession) (http.RoundTripper, error) {
	transport, _, err := f.transportForRequestWithImpersonation(sess)
	return transport, trace.Wrap(err)
}

// dialContextFunc is a context network dialer function that returns a network connection
type dialContextFunc func(context.Context, string, string) (net.Conn, error)

// transportForRequestWithImpersonation returns a transport that supports
// impersonation. This allows the client to reuse the same transport for all
// requests to the cluster in order to improve performance.
// The transport is cached in the forwarder so that it can be reused for future
// requests. If the transport is not cached, a new one is created and cached.
func (f *Forwarder) transportForRequestWithImpersonation(sess *clusterSession) (http.RoundTripper, *tls.Config, error) {
	// If the session has a kube API credentials, it means that the next hop is
	// a Kubernetes API server. In this case, we can use the provided credentials
	// to dial the next hop directly and never cache the transport.
	if sess.kubeAPICreds != nil {
		// If agent is running in agent mode, get the transport from the configured cluster
		// credentials.
		return sess.kubeAPICreds.getTransport(), sess.kubeAPICreds.getTLSConfig(), nil
	}

	// If the cluster is remote, the key is the teleport cluster name.
	// If the cluster is local, the key is the teleport cluster name and the kubernetes
	// cluster name: <teleport-cluster-name>/<kubernetes-cluster-name>.
	key := transportCacheKey(sess)

	t, err := utils.FnCacheGet(f.ctx, f.cachedTransport, key, func(ctx context.Context) (*cachedTransportEntry, error) {
		var (
			httpTransport http.RoundTripper
			tlsConfig     *tls.Config
			err           error
		)
		if sess.teleportCluster.isRemote {
			// If the cluster is remote, create a new transport for the remote cluster.
			httpTransport, tlsConfig, err = f.newRemoteClusterTransport(sess.teleportCluster.name)
		} else if f.cfg.ReverseTunnelSrv != nil {
			// If agent is running in proxy mode, create a new transport for the local cluster.
			httpTransport, tlsConfig, err = f.newLocalClusterTransport(sess.kubeClusterName)
		} else {
			return nil, trace.BadParameter("no reverse tunnel server or credentials provided")
		}

		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &cachedTransportEntry{
			transport: httpTransport,
			tlsConfig: tlsConfig,
		}, nil
	})

	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return t.transport, t.tlsConfig.Clone(), nil
}

// transportCacheKey returns a key used to cache transports.
// If the cluster is remote, the key is the teleport cluster name.
// If the cluster is local, the key is the teleport cluster name and the kubernetes
// cluster name.
// The key is used to cache transports so that they can be reused for future requests.
// Each transport contains a custom dialer that is valid for a specific Teleport
// remote proxy or Teleport Kubernetes Services that serves the target cluster.
func transportCacheKey(sess *clusterSession) string {
	if sess.teleportCluster.isRemote {
		return fmt.Sprintf("%x", sess.teleportCluster.name)
	}
	return fmt.Sprintf("%x/%x", sess.teleportCluster.name, sess.kubeClusterName)
}

// wrapTransport wraps the provided transport with the Kubernetes transport config
// if it is not nil.
func wrapTransport(rt http.RoundTripper, transportConfig *transport.Config) (http.RoundTripper, error) {
	if transportConfig == nil {
		return rt, nil
	}

	wrapped, err := transport.HTTPWrappersForConfig(transportConfig, rt)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return enforceCloseIdleConnections(wrapped, rt), nil
}

// newTransport creates a new [http.Transport] with the provided dialer and TLS
// config.
// The transport is configured to use a connection pool and to close idle
// connections after a timeout.
func newTransport(dial dialContextFunc, tlsConfig *tls.Config) *http.Transport {
	return &http.Transport{
		DialContext:     dial,
		TLSClientConfig: tlsConfig,
		// Increase the size of the connection pool. This substantially improves the
		// performance of Teleport under load as it reduces the number of TLS
		// handshakes performed.
		MaxIdleConns:        defaults.HTTPMaxIdleConns,
		MaxIdleConnsPerHost: defaults.HTTPMaxIdleConnsPerHost,
		// IdleConnTimeout defines the maximum amount of time before idle connections
		// are closed. Leaving this unset will lead to connections open forever and
		// will cause memory leaks in a long running process.
		IdleConnTimeout: defaults.HTTPIdleTimeout,
	}
}

// newRemoteClusterTransport returns a new [http.Transport] (https://golang.org/pkg/net/http/#Transport)
// that can be used to dial Kubernetes Proxy in a remote Teleport cluster.
// The transport is configured to use a connection pool and to close idle
// connections after a timeout.
func (f *Forwarder) newRemoteClusterTransport(clusterName string) (http.RoundTripper, *tls.Config, error) {
	// Tunnel is nil for a teleport process with "kubernetes_service" but
	// not "proxy_service".
	if f.cfg.ReverseTunnelSrv == nil {
		return nil, nil, trace.BadParameter("this Teleport process can not dial Kubernetes endpoints in remote Teleport clusters; only proxy_service supports this, make sure a Teleport proxy is first in the request path")
	}
	// Dialer that will be used to dial the remote cluster via the reverse tunnel.
	dialFn := f.remoteClusterDialer(clusterName)
	tlsConfig, err := f.getTLSConfigForLeafCluster(clusterName)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	// Create a new HTTP/2 transport that will be used to dial the remote cluster.
	h2Transport, err := newH2Transport(tlsConfig, dialFn)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return instrumentedRoundtripper(
		f.cfg.KubeServiceType,
		internal.NewImpersonatorRoundTripper(h2Transport),
	), tlsConfig.Clone(), nil
}

// getTLSConfigForLeafCluster returns a TLS config with the Proxy certificate
// and the root CAs for the leaf cluster. Root proxy uses its own certificate
// to connect to the leaf proxy.
func (f *Forwarder) getTLSConfigForLeafCluster(clusterName string) (*tls.Config, error) {
	ctx, cancel := context.WithTimeout(f.ctx, 5*time.Second)
	defer cancel()
	// Get the host CA for the target cluster from Auth to ensure we trust the
	// leaf proxy certificate at the current time.
	_, err := f.cfg.CachingAuthClient.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.HostCA,
		DomainName: clusterName,
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsConfig := utils.TLSConfig(f.cfg.ConnTLSCipherSuites)
	tlsConfig.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
		tlsCert, err := f.cfg.GetConnTLSCertificate()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return tlsCert, nil
	}
	tlsConfig.InsecureSkipVerify = true
	tlsConfig.VerifyConnection = utils.VerifyConnectionWithRoots(func() (*x509.CertPool, error) {
		pool, _, err := authclient.ClientCertPool(f.ctx, f.cfg.CachingAuthClient, clusterName, types.HostCA)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return pool, nil
	})

	return tlsConfig, nil
}

// remoteClusterDialer returns a dialer that can be used to dial Kubernetes Proxy
// in a remote Teleport cluster via the reverse tunnel.
func (f *Forwarder) remoteClusterDialer(clusterName string) dialContextFunc {
	return func(ctx context.Context, _, _ string) (net.Conn, error) {
		_, span := f.cfg.tracer.Start(
			ctx,
			"kube.Forwarder/remoteClusterDiater",
			oteltrace.WithSpanKind(oteltrace.SpanKindClient),
			oteltrace.WithAttributes(
				semconv.RPCServiceKey.String(f.cfg.KubeServiceType),
				semconv.RPCMethodKey.String("reverse_tunnel.Dial"),
				semconv.RPCSystemKey.String("kube"),
			),
		)
		defer span.End()

		targetCluster, err := f.cfg.ReverseTunnelSrv.GetSite(clusterName)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return targetCluster.DialTCP(reversetunnelclient.DialParams{
			// Send a sentinel value to the remote cluster because this connection
			// will be used to forward multiple requests to the remote cluster from
			// different users.
			// IP Pinning is based on the source IP address of the connection that
			// we transport over HTTP headers so it's not affected.
			From: &utils.NetAddr{AddrNetwork: "tcp", Addr: "0.0.0.0:0"},
			// Proxy uses reverse tunnel dialer to connect to Kubernetes in a leaf cluster
			// and the targetKubernetes cluster endpoint is determined from the identity
			// encoded in the TLS certificate. We're setting the dial endpoint to a hardcoded
			// `kube.teleport.cluster.local` value to indicate this is a Kubernetes proxy request
			To:       &utils.NetAddr{AddrNetwork: "tcp", Addr: reversetunnelclient.LocalKubernetes},
			ConnType: types.KubeTunnel,
		})
	}
}

// newLocalClusterTransport returns a new [http.Transport] (https://golang.org/pkg/net/http/#Transport)
// that can be used to dial Kubernetes Service in a local Teleport cluster.
func (f *Forwarder) newLocalClusterTransport(kubeClusterName string) (http.RoundTripper, *tls.Config, error) {
	tlsConfig := utils.TLSConfig(f.cfg.ConnTLSCipherSuites)
	tlsConfig.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
		tlsCert, err := f.cfg.GetConnTLSCertificate()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return tlsCert, nil
	}
	tlsConfig.InsecureSkipVerify = true
	tlsConfig.VerifyConnection = utils.VerifyConnectionWithRoots(f.cfg.GetConnTLSRoots)

	dialFn := f.localClusterDialer(kubeClusterName)
	// Create a new HTTP/2 transport that will be used to dial the remote cluster.
	h2Transport, err := newH2Transport(tlsConfig, dialFn)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return instrumentedRoundtripper(
		f.cfg.KubeServiceType,
		internal.NewImpersonatorRoundTripper(h2Transport),
	), tlsConfig.Clone(), nil
}

// localClusterDialer returns a dialer that can be used to dial Kubernetes Service
// in a local Teleport cluster using the reverse tunnel.
// The endpoints are fetched from the cached auth client and are shuffled
// to avoid hotspots.
func (f *Forwarder) localClusterDialer(kubeClusterName string, opts ...contextDialerOption) dialContextFunc {
	opt := contextDialerOptions{}
	for _, o := range opts {
		o(&opt)
	}
	return func(ctx context.Context, _, _ string) (net.Conn, error) {
		_, span := f.cfg.tracer.Start(
			ctx,
			"kube.Forwarder/localClusterDiater",
			oteltrace.WithSpanKind(oteltrace.SpanKindClient),
			oteltrace.WithAttributes(
				semconv.RPCServiceKey.String(f.cfg.KubeServiceType),
				semconv.RPCMethodKey.String("reverse_tunnel.Dial"),
				semconv.RPCSystemKey.String("kube"),
			),
		)
		defer span.End()

		// Not a remote cluster and we have a reverse tunnel server.
		// Use the local reversetunnel.Site which knows how to dial by serverID
		// (for "kubernetes_service" connected over a tunnel) and falls back to
		// direct dial if needed.
		localCluster, err := f.cfg.ReverseTunnelSrv.GetSite(f.cfg.ClusterName)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		kubeServers, err := f.getKubernetesServersForKubeCluster(ctx, kubeClusterName)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		var errs []error
		// Shuffle the list of servers to avoid always connecting to the same
		// server.
		for _, s := range utils.ShuffleVisit(kubeServers) {
			// Validate that the requested kube cluster is registered.
			kubeCluster := s.GetCluster()
			if kubeCluster.GetName() != kubeClusterName || !opt.matches(s.GetHostID()) {
				continue
			}
			// serverID is a unique identifier of the server in the cluster.
			// It is a combination of the server's hostname and the cluster name.
			// <host_id>.<cluster_name>
			serverID := fmt.Sprintf("%s.%s", s.GetHostID(), f.cfg.ClusterName)
			conn, err := localCluster.DialTCP(reversetunnelclient.DialParams{
				// Send a sentinel value to the remote cluster because this connection
				// will be used to forward multiple requests to the remote cluster from
				// different users.
				// IP Pinning is based on the source IP address of the connection that
				// we transport over HTTP headers so it's not affected.
				From:     &utils.NetAddr{AddrNetwork: "tcp", Addr: "0.0.0.0:0"},
				To:       &utils.NetAddr{AddrNetwork: "tcp", Addr: s.GetHostname()},
				ConnType: types.KubeTunnel,
				ServerID: serverID,
				ProxyIDs: s.GetProxyIDs(),
			})
			if err == nil {
				opt.collect(s.GetHostID())
				return conn, nil
			}
			errs = append(errs, trace.Wrap(err))
		}

		if len(errs) > 0 {
			return nil, trace.NewAggregate(errs...)
		}

		return nil, trace.NotFound("kubernetes cluster %q is not found in teleport cluster %q", kubeClusterName, f.cfg.ClusterName)
	}
}

// newH2Transport creates a new HTTP/2 transport with ALPN support.
func newH2Transport(tlsConfig *tls.Config, dial dialContextFunc) (*http.Transport, error) {
	tlsConfig = tlsConfig.Clone()
	if tlsConfig == nil {
		tlsConfig = &tls.Config{}
	}
	tlsConfig.NextProtos = []string{http2.NextProtoTLS, teleport.HTTPNextProtoTLS}
	h2HTTPTransport := newTransport(dial, tlsConfig)
	// Upgrade transport to h2 where HTTP_PROXY and HTTPS_PROXY
	// envs are not take into account purposely.
	if err := http2.ConfigureTransport(h2HTTPTransport); err != nil {
		return nil, trace.Wrap(err)
	}
	return h2HTTPTransport, nil
}

// getTLSConfig returns TLS config required to connect to the next hop.
// If the current Kubernetes service serves the target cluster, it returns the
// Kubernetes API tls configuration.
// If the current service is a proxy and the next hop supports impersonation,
// it returns the proxy's TLS config.
// Otherwise, it requests a certificate from the auth server with the identity
// of the user that is requesting the connection embedded in the certificate.
// The boolean returned indicates whether the upstream server supports
// impersonation.
func (f *Forwarder) getTLSConfig(sess *clusterSession) (*tls.Config, bool, error) {
	if sess.kubeAPICreds != nil {
		return sess.kubeAPICreds.getTLSConfig(), false, nil
	}

	_, tlsConfig, err := f.transportForRequestWithImpersonation(sess)
	return tlsConfig, err == nil, trace.Wrap(err)
}

// getContextDialerFunc returns a dialer function that can be used to connect
// to the next hop.
// If the next hop is a remote cluster, it returns a dialer that connects to
// the remote cluster proxy using the reverse tunnel server.
// If the next hop is a kubernetes service, it returns a dialer that connects
// to the first available kubernetes service.
// If the next hop is a local cluster, it returns a dialer that directly dials
// to the next hop.
func (f *Forwarder) getContextDialerFunc(s *clusterSession, opts ...contextDialerOption) dialContextFunc {
	if s.kubeAPICreds != nil {
		// If this is a kubernetes service, we need to connect to the kubernetes
		// API server using a direct dialer.
		return new(net.Dialer).DialContext
	} else if s.teleportCluster.isRemote {
		// If this is a remote cluster, we need to connect to the local proxy
		// and then forward the connection to the remote cluster.
		return f.remoteClusterDialer(s.teleportCluster.name)
	} else if f.cfg.ReverseTunnelSrv != nil {
		// If this is a local cluster, we need to connect to the remote proxy
		// and then forward the connection to the local cluster.
		return f.localClusterDialer(s.kubeClusterName, opts...)
	}

	return new(net.Dialer).DialContext
}

// contextDialerOptions is a set of options that can be used to filter
// the hosts that the dialer connects to.
type contextDialerOptions struct {
	hostIDFilter  string
	collectHostID *string
}

// matches returns true if the host matches the hostID of the dialer options or
// if the dialer hostID is empty.
func (c *contextDialerOptions) matches(hostID string) bool {
	return c.hostIDFilter == "" || c.hostIDFilter == hostID
}

// collect sets the hostID that the dialer connected to if collectHostID is not nil.
func (c *contextDialerOptions) collect(hostID string) {
	if c.collectHostID != nil {
		*c.collectHostID = hostID
	}
}

// contextDialerOption is a functional option for the contextDialerOptions.
type contextDialerOption func(*contextDialerOptions)

// withTargetHostID is a functional option that sets the hostID of the dialer.
// If the hostID is empty, the dialer will connect to the first available host.
// If the hostID is not empty, the dialer will connect to the host with the
// specified hostID. If that host is not available, the dialer will return an
// error.
func withTargetHostID(hostID string) contextDialerOption {
	return func(o *contextDialerOptions) {
		o.hostIDFilter = hostID
	}
}

// withHostIDCollection is a functional option that sets the hostID of the dialer
// to the provided pointer.
func withHostIDCollection(hostID *string) contextDialerOption {
	return func(o *contextDialerOptions) {
		o.collectHostID = hostID
	}
}
