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

package proxy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/net/http2"
	"k8s.io/client-go/transport"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// transportForRequest determines the transport to use for a request to a specific
// Kubernetes cluster. If the servers don't support impersonation, a single
// transport is used per request. Otherwise, a new transport is used for all
// requests in order to improve performance.
// TODO(tigrato): DELETE IN 15.0.0
// TODO(tigrato): Remove the check in Teleport 15, when all servers support impersonation.
func (f *Forwarder) transportForRequest(sess *clusterSession) (http.RoundTripper, error) {
	// allServersSupportImpersonation returns true if all servers support impersonation,
	// otherwise fallback to using a single transport per request with user's identity
	// embedded in the certificate.
	if f.allServersSupportImpersonation(sess) {
		// If all servers support impersonation, use a new transport for each
		// request. This will ensure that the client certificate is valid for the
		// server that the request is being sent to.
		return f.transportForRequestWithImpersonation(sess)
	}
	// Otherwise, use a single transport per request.
	return f.transportForRequestWithoutImpersonation(sess)
}

// allServersSupportImpersonation returns true if all servers support impersonation.
// If the cluster is remote, it checks if all remote proxies support impersonation, otherwise
// it checks if all kube_services support impersonation.
// TODO(tigrato): DELETE in 15.0.0
func (f *Forwarder) allServersSupportImpersonation(sess *clusterSession) bool {
	// If the cluster is remote, we need to check if all remote proxies support
	// impersonation. If it does, use a single transport per request. Otherwise,
	// fall back to using a new transport for each request.
	if sess.teleportCluster.isRemote {
		proxies, err := f.getRemoteClusterProxies(sess.teleportCluster.name)
		return err == nil && allServersSupportImpersonation(proxies)
	}
	// If the cluster is not remote, validate the kube services support of
	// impersonation.
	return allServersSupportImpersonation(sess.kubeServers)
}

// getRemoteClusterProxies returns a list of proxies registered at the remote cluster.
// It's used to determine whether the remote cluster supports identity forwarding.
func (f *Forwarder) getRemoteClusterProxies(clusterName string) ([]types.Server, error) {
	targetCluster, err := f.cfg.ReverseTunnelSrv.GetSite(clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Get the remote cluster's cache.
	caching, err := targetCluster.CachingAccessPoint()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	proxies, err := caching.GetProxies()
	return proxies, trace.Wrap(err)
}

// dialContextFunc is a context network dialer function that returns a network connection
type dialContextFunc func(context.Context, string, string) (net.Conn, error)

// transportForRequestWithoutImpersonation returns a transport that does not
// support impersonation. This is used when the at least one kube_server or proxy
// don't support impersonation in order to ensure that the client request
// can be routed correctly.
//
// DELETE IN 15.0.0
// TODO(tigrato): Remove this once all servers support impersonation.
func (f *Forwarder) transportForRequestWithoutImpersonation(sess *clusterSession) (http.RoundTripper, error) {
	if sess.kubeAPICreds != nil {
		return sess.kubeAPICreds.getTransport(), nil
	}
	// Get the TLS config for the next hop that does not support impersonation.
	tlsConfig, _, err := f.getTLSConfig(sess)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	transport := newTransport(sess.DialWithContext(), tlsConfig)
	if !sess.upgradeToHTTP2 {
		return instrumentedRoundtripper(f.cfg.KubeServiceType, transport), nil
	}
	if err := http2.ConfigureTransport(transport); err != nil {
		return nil, trace.Wrap(err)
	}
	return instrumentedRoundtripper(f.cfg.KubeServiceType, transport), nil
}

// transportForRequestWithImpersonation returns a transport that supports
// impersonation. This allows the client to reuse the same transport for all
// requests to the cluster in order to improve performance.
// The transport is cached in the forwarder so that it can be reused for future
// requests. If the transport is not cached, a new one is created and cached.
func (f *Forwarder) transportForRequestWithImpersonation(sess *clusterSession) (http.RoundTripper, error) {
	// transportCacheTTL is the TTL for the transport cache.
	const transportCacheTTL = 5 * time.Hour
	// If the cluster is remote, the key is the teleport cluster name.
	// If the cluster is local, the key is the teleport cluster name and the kubernetes
	// cluster name: <teleport-cluster-name>/<kubernetes-cluster-name>.
	key := transportCacheKey(sess)

	// Check if the transport is cached.
	f.cachedTransportMu.Lock()
	cachedI, ok := f.cachedTransport.Get(key)
	f.cachedTransportMu.Unlock()
	if ok {
		if cached, ok := cachedI.(http.RoundTripper); ok {
			return cached, nil
		}
	}

	var httpTransport http.RoundTripper
	var err error
	if sess.teleportCluster.isRemote {
		// If the cluster is remote, create a new transport for the remote cluster.
		httpTransport, err = f.newRemoteClusterTransport(sess.teleportCluster.name)
	} else if sess.kubeAPICreds != nil {
		// If agent is running in agent mode, get the transport from the configured cluster
		// credentials.
		return sess.kubeAPICreds.getTransport(), nil
	} else if f.cfg.ReverseTunnelSrv != nil {
		// If agent is running in proxy mode, create a new transport for the local cluster.
		httpTransport, err = f.newLocalClusterTransport(sess.kubeClusterName)
	} else {
		return nil, trace.BadParameter("no reverse tunnel server or credentials provided")
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Cache the transport.
	f.cachedTransportMu.Lock()
	f.cachedTransport.Set(key, httpTransport, transportCacheTTL)
	f.cachedTransportMu.Unlock()

	return httpTransport, nil
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

// versionWithoutImpersonation is the version of Teleport that starts supporting
// impersonation. Before this version, the client will not use impersonation.
var versionWithoutImpersonation = semver.New(utils.VersionBeforeAlpha("13.0.0"))

// teleportVersionInterface is an interface that allows to get the Teleport version of
// a server.
// DELETE IN 15.0.0
type teleportVersionInterface interface {
	GetTeleportVersion() string
}

// allServersSupportImpersonation returns true if all servers in the list
// support impersonation. This is used to determine if the client should
// create a new client certificate and use a different [http.Transport]
// (https://golang.org/pkg/net/http/#Transport) for each request.
// Only returns true if all servers in the list support impersonation.
// DELETE IN 15.0.0
func allServersSupportImpersonation[T teleportVersionInterface](servers []T) bool {
	if len(servers) == 0 {
		return false
	}
	for _, server := range servers {
		serverVersion := server.GetTeleportVersion()
		semVer, err := semver.NewVersion(serverVersion)
		if err != nil || semVer.LessThan(*versionWithoutImpersonation) {
			return false
		}
	}
	return true
}

// getOrRequestClientCreds returns the client credentials for the provided auth context.
// If the credentials are not cached, they will be requested from the auth server.
// DELETE IN 15.0.0
func (f *Forwarder) getOrRequestClientCreds(tracingCtx context.Context, authCtx authContext) (*tls.Config, error) {
	c := f.getClientCreds(authCtx)
	if c == nil {
		return f.serializedRequestClientCreds(tracingCtx, authCtx)
	}
	return c, nil
}

// getClientCreds returns the client credentials for the provided auth context.
// DELETE IN 15.0.0
func (f *Forwarder) getClientCreds(ctx authContext) *tls.Config {
	f.mu.Lock()
	defer f.mu.Unlock()
	creds, ok := f.clientCredentials.Get(ctx.key())
	if !ok {
		return nil
	}
	c := creds.(*tls.Config)
	if !validClientCreds(f.cfg.Clock, c) {
		return nil
	}
	return c
}

// saveClientCreds saves the client credentials for the provided auth context.
// DELETE IN 15.0.0
func (f *Forwarder) saveClientCreds(ctx authContext, c *tls.Config) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.clientCredentials.Set(ctx.key(), c, ctx.sessionTTL)
}

// validClientCreds returns true if the provided client credentials are valid.
// DELETE IN 15.0.0
func validClientCreds(clock clockwork.Clock, c *tls.Config) bool {
	if len(c.Certificates) == 0 || len(c.Certificates[0].Certificate) == 0 {
		return false
	}
	crt, err := x509.ParseCertificate(c.Certificates[0].Certificate[0])
	if err != nil {
		return false
	}
	// Make sure that the returned cert will be valid for at least 1 more
	// minute.
	return clock.Now().After(crt.NotBefore) && clock.Now().Add(time.Minute).Before(crt.NotAfter)
}

// newRemoteClusterTransport returns a new [http.Transport] (https://golang.org/pkg/net/http/#Transport)
// that can be used to dial Kubernetes Proxy in a remote Teleport cluster.
// The transport is configured to use a connection pool and to close idle
// connections after a timeout.
func (f *Forwarder) newRemoteClusterTransport(clusterName string) (http.RoundTripper, error) {
	// Tunnel is nil for a teleport process with "kubernetes_service" but
	// not "proxy_service".
	if f.cfg.ReverseTunnelSrv == nil {
		return nil, trace.BadParameter("this Teleport process can not dial Kubernetes endpoints in remote Teleport clusters; only proxy_service supports this, make sure a Teleport proxy is first in the request path")
	}
	// Dialer that will be used to dial the remote cluster via the reverse tunnel.
	dialFn := f.remoteClusterDialer(clusterName)
	tlsConfig, err := f.getTLSConfigForLeafCluster(clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Create a new HTTP/2 transport that will be used to dial the remote cluster.
	h2Transport, err := newH2Transport(tlsConfig, dialFn)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return instrumentedRoundtripper(f.cfg.KubeServiceType, auth.NewImpersonatorRoundTripper(h2Transport)), nil
}

// getTLSConfigForLeafCluster returns a TLS config with the Proxy certificate
// and the root CAs for the leaf cluster. Root proxy uses its own certificate
// to connect to the leaf proxy.
func (f *Forwarder) getTLSConfigForLeafCluster(clusterName string) (*tls.Config, error) {
	ctx, cancel := context.WithTimeout(f.ctx, 5*time.Second)
	defer cancel()
	// Get the host CA for the target cluster from Auth to ensure we trust the
	// leaf proxy certificate.
	hostCA, err := f.cfg.CachingAuthClient.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.HostCA,
		DomainName: clusterName,
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pool := x509.NewCertPool()
	for _, certAuthority := range services.GetTLSCerts(hostCA) {
		if ok := pool.AppendCertsFromPEM(certAuthority); !ok {
			return nil, trace.BadParameter("failed to append certificates, check that kubeconfig has correctly encoded certificate authority data")
		}
	}
	// Clone the TLS config and set the root CAs to the leaf host CA pool.
	tlsConfig := f.cfg.ConnTLSConfig.Clone()
	tlsConfig.RootCAs = pool

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
func (f *Forwarder) newLocalClusterTransport(kubeClusterName string) (http.RoundTripper, error) {
	dialFn := f.localClusterDialer(kubeClusterName)
	// Create a new HTTP/2 transport that will be used to dial the remote cluster.
	h2Transport, err := newH2Transport(f.cfg.ConnTLSConfig, dialFn)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return instrumentedRoundtripper(f.cfg.KubeServiceType, auth.NewImpersonatorRoundTripper(h2Transport)), nil
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
		// Shuffle the list of servers to avoid always connecting to the same
		// server.
		rand.Shuffle(
			len(kubeServers),
			func(i, j int) {
				kubeServers[i], kubeServers[j] = kubeServers[j], kubeServers[i]
			},
		)

		var errs []error
		// Validate that the requested kube cluster is registered.
		for _, s := range kubeServers {
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

	// if the next hop supports impersonation, we can use the TLS config
	// of the proxy to connect to it.
	if f.allServersSupportImpersonation(sess) {
		return f.cfg.ConnTLSConfig.Clone(), true, nil
	}

	// If the next hop does not support impersonation, we need to get a
	// certificate from the auth server with the identity of the user
	// that is requesting the connection.
	// TODO(tigrato): DELETE in 15.0.0
	tlsConfig, err := f.getOrRequestClientCreds(sess.requestContext, sess.authContext)
	if err != nil {
		f.log.Warningf("Failed to get certificate for %v: %v.", sess.authContext, err)
		return nil, false, trace.AccessDenied("access denied: failed to authenticate with auth server")
	}
	return tlsConfig, false, nil
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
	hostID string
}

// matches returns true if the host matches the hostID of the dialer options or
// if the dialer hostID is empty.
func (c *contextDialerOptions) matches(hostID string) bool {
	return c.hostID == "" || c.hostID == hostID
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
		o.hostID = hostID
	}
}
