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

package creds

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	tracehttp "github.com/gravitational/teleport/api/observability/tracing/http"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/net/http2"
	authzapi "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/kubernetes"
	authztypes "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
)

type KubeCreds interface {
	GetTLSConfig() *tls.Config
	GetTransportConfig() *transport.Config
	GetTargetAddr() string
	GetKubeRestConfig() *rest.Config
	GetKubeClient() kubernetes.Interface
	GetTransport() http.RoundTripper
	WrapTransport(http.RoundTripper) (http.RoundTripper, error)
	Close() error
}

// StaticKubeCreds contain authentication-related fields from kubeconfig.
//
// TODO(awly): make this an interface, one implementation for local k8s cluster
// and another for a remote teleport cluster.
type StaticKubeCreds struct {
	// tlsConfig contains (m)TLS configuration.
	tlsConfig *tls.Config
	// transportConfig contains HTTPS-related configuration.
	// Note: use wrapTransport method if working with http.RoundTrippers.
	transportConfig *transport.Config
	// targetAddr is a kubernetes API address.
	targetAddr string
	kubeClient kubernetes.Interface
	// clientRestCfg is the Kubernetes Rest config for the cluster.
	clientRestCfg *rest.Config
	transport     http.RoundTripper
}

func (s *StaticKubeCreds) GetTLSConfig() *tls.Config {
	return s.tlsConfig.Clone()
}

func (s *StaticKubeCreds) GetTransport() http.RoundTripper {
	return s.transport
}

func (s *StaticKubeCreds) GetTransportConfig() *transport.Config {
	return s.transportConfig
}

func (s *StaticKubeCreds) GetTargetAddr() string {
	return s.targetAddr
}

func (s *StaticKubeCreds) GetKubeClient() kubernetes.Interface {
	return s.kubeClient
}

func (s *StaticKubeCreds) GetKubeRestConfig() *rest.Config {
	return s.clientRestCfg
}

func (s *StaticKubeCreds) WrapTransport(rt http.RoundTripper) (http.RoundTripper, error) {
	if s == nil {
		return rt, nil
	}

	wrapped, err := transport.HTTPWrappersForConfig(s.transportConfig, rt)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return enforceCloseIdleConnections(wrapped, rt), nil
}

func (s *StaticKubeCreds) Close() error {
	return nil
}

// enforceCloseIdleConnections ensures that the returned [http.RoundTripper]
// has a CloseIdleConnections method. [transport.HTTPWrappersForConfig] returns
// a [http.RoundTripper] that does not implement it so any calls to [http.Client.CloseIdleConnections]
// will result in a noop instead of forwarding the request onto its wrapped [http.RoundTripper].
func enforceCloseIdleConnections(wrapper, wrapped http.RoundTripper) http.RoundTripper {
	type closeIdler interface {
		CloseIdleConnections()
	}

	type unwrapper struct {
		http.RoundTripper
		closeIdler
	}

	if _, ok := wrapper.(closeIdler); ok {
		return wrapper
	}

	if c, ok := wrapped.(closeIdler); ok {
		return &unwrapper{
			RoundTripper: wrapper,
			closeIdler:   c,
		}
	}

	return wrapper
}

// dynamicCredsClient defines the function signature used by `dynamicCreds`
// to generate and renew short-lived credentials to access the cluster.
type DynamicCredsClient func(ctx context.Context, cluster types.KubeCluster) (cfg *rest.Config, expirationTime time.Time, err error)

// dynamicKubeCreds contains short-lived credentials to access the cluster.
// Unlike `staticKubeCreds`, `dynamicKubeCreds` extracts access credentials using the `client`
// function and renews them whenever they are about to expire.
type DynamicKubeCreds struct {
	ctx         context.Context
	renewTicker clockwork.Ticker
	staticCreds *StaticKubeCreds
	log         *slog.Logger
	closeC      chan struct{}
	client      DynamicCredsClient
	checker     servicecfg.ImpersonationPermissionsChecker
	clock       clockwork.Clock
	component   string
	sync.RWMutex
	wg sync.WaitGroup
}

// dynamicCredsConfig contains configuration for dynamicKubeCreds.
type DynamicCredsConfig struct {
	KubeCluster          types.KubeCluster
	Log                  *slog.Logger
	Client               DynamicCredsClient
	Checker              servicecfg.ImpersonationPermissionsChecker
	Clock                clockwork.Clock
	InitialRenewInterval time.Duration
	ResourceMatchers     []services.ResourceMatcher
	Component            string
}

func (d *DynamicCredsConfig) checkAndSetDefaults() error {
	if d.KubeCluster == nil {
		return trace.BadParameter("missing kubeCluster")
	}
	if d.Log == nil {
		return trace.BadParameter("missing log")
	}
	if d.Client == nil {
		return trace.BadParameter("missing client")
	}
	if d.Checker == nil {
		return trace.BadParameter("missing checker")
	}
	if d.Clock == nil {
		d.Clock = clockwork.NewRealClock()
	}
	if d.InitialRenewInterval == 0 {
		d.InitialRenewInterval = time.Hour
	}
	return nil
}

// NewDynamicKubeCreds creates a new dynamicKubeCreds refresher and starts the
// credentials refresher mechanism to renew them once they are about to expire.
func NewDynamicKubeCreds(ctx context.Context, cfg DynamicCredsConfig) (*DynamicKubeCreds, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	dyn := &DynamicKubeCreds{
		ctx:         ctx,
		log:         cfg.Log,
		closeC:      make(chan struct{}),
		client:      cfg.Client,
		renewTicker: cfg.Clock.NewTicker(cfg.InitialRenewInterval),
		checker:     cfg.Checker,
		clock:       cfg.Clock,
		component:   cfg.Component,
	}

	if err := dyn.renewClientset(cfg.KubeCluster); err != nil {
		return nil, trace.Wrap(err)
	}
	dyn.wg.Add(1)
	go func() {
		defer dyn.wg.Done()
		for {
			select {
			case <-dyn.closeC:
				return
			case <-dyn.renewTicker.Chan():
				if err := dyn.renewClientset(cfg.KubeCluster); err != nil {
					cfg.Log.WarnContext(ctx, "Unable to renew cluster credentials", "cluster", cfg.KubeCluster.GetName(), "error", err)
				}
			}
		}
	}()

	return dyn, nil
}

func (d *DynamicKubeCreds) GetTLSConfig() *tls.Config {
	d.RLock()
	defer d.RUnlock()
	return d.staticCreds.GetTLSConfig()
}

func (d *DynamicKubeCreds) GetTransportConfig() *transport.Config {
	d.RLock()
	defer d.RUnlock()
	return d.staticCreds.transportConfig
}

func (d *DynamicKubeCreds) GetKubeRestConfig() *rest.Config {
	d.RLock()
	defer d.RUnlock()
	return d.staticCreds.clientRestCfg
}

func (d *DynamicKubeCreds) GetTargetAddr() string {
	d.RLock()
	defer d.RUnlock()
	return d.staticCreds.targetAddr
}

func (d *DynamicKubeCreds) GetKubeClient() kubernetes.Interface {
	d.RLock()
	defer d.RUnlock()
	return d.staticCreds.kubeClient
}

func (d *DynamicKubeCreds) WrapTransport(rt http.RoundTripper) (http.RoundTripper, error) {
	d.RLock()
	defer d.RUnlock()
	return d.staticCreds.WrapTransport(rt)
}

func (d *DynamicKubeCreds) Close() error {
	close(d.closeC)
	d.wg.Wait()
	d.renewTicker.Stop()
	return nil
}

func (d *DynamicKubeCreds) GetTransport() http.RoundTripper {
	d.RLock()
	defer d.RUnlock()
	return d.staticCreds.GetTransport()
}

// renewClientset generates the credentials required for accessing the cluster using the client function.
func (d *DynamicKubeCreds) renewClientset(cluster types.KubeCluster) error {
	// get auth config
	restConfig, exp, err := d.client(d.ctx, cluster)
	if err != nil {
		return trace.Wrap(err)
	}
	creds, err := ExtractKubeCreds(d.ctx, d.component, cluster.GetName(), restConfig, d.log, d.checker)
	if err != nil {
		return trace.Wrap(err)
	}

	d.Lock()
	defer d.Unlock()
	d.staticCreds = creds
	// prepares the next renew cycle
	if !exp.IsZero() {
		reset := exp.Sub(d.clock.Now()) / 2
		d.renewTicker.Reset(reset)
	}
	return nil
}

func ExtractKubeCreds(ctx context.Context, component string, cluster string, clientCfg *rest.Config, log *slog.Logger, checkPermissions servicecfg.ImpersonationPermissionsChecker) (*StaticKubeCreds, error) {
	log = log.With("cluster", cluster)

	log.DebugContext(ctx, "Checking Kubernetes impersonation permissions")
	client, err := kubernetes.NewForConfig(clientCfg)
	if err != nil {
		return nil, trace.Wrap(err, "failed to generate Kubernetes client for cluster %q", cluster)
	}

	// For each loaded cluster, check impersonation permissions. This
	// check only logs when permissions are not configured, but does not fail startup.
	if err := checkPermissions(ctx, cluster, client.AuthorizationV1().SelfSubjectAccessReviews()); err != nil {
		log.WarnContext(ctx, "Failed to test the necessary Kubernetes permissions. The target Kubernetes cluster may be down or have misconfigured RBAC. This teleport instance will still handle Kubernetes requests towards this Kubernetes cluster.",
			"error", err,
		)
	} else {
		log.DebugContext(ctx, "Have all necessary Kubernetes impersonation permissions")
	}

	targetAddr, err := parseKubeHost(clientCfg.Host)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// tlsConfig can be nil and still no error is returned.
	// This happens when no `certificate-authority-data` is provided in kubeconfig because one is expected to use
	// the system default CA pool.
	tlsConfig, err := rest.TLSConfigFor(clientCfg)
	if err != nil {
		return nil, trace.Wrap(err, "failed to generate TLS config from kubeconfig: %v", err)
	}
	transportConfig, err := clientCfg.TransportConfig()
	if err != nil {
		return nil, trace.Wrap(err, "failed to generate transport config from kubeconfig: %v", err)
	}

	transport, err := newDirectTransport(component, tlsConfig, transportConfig)
	if err != nil {
		return nil, trace.Wrap(err, "failed to generate transport from kubeconfig: %v", err)
	}

	log.DebugContext(ctx, "Initialized Kubernetes credentials")
	return &StaticKubeCreds{
		tlsConfig:       tlsConfig,
		transportConfig: transportConfig,
		targetAddr:      targetAddr,
		kubeClient:      client,
		clientRestCfg:   clientCfg,
		transport:       transport,
	}, nil
}

// newDirectTransport creates a new http.Transport that will be used to connect to the Kubernetes API server.
// It is a direct connection, not going through a Teleport proxy.
// The transport used respects HTTP_PROXY, HTTPS_PROXY, and NO_PROXY environment variables.
func newDirectTransport(component string, tlsConfig *tls.Config, transportConfig *transport.Config) (http.RoundTripper, error) {
	h2HTTPTransport, err := NewH2Transport(tlsConfig, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// SetTransportDefaults sets the default values for the transport including
	// support for HTTP_PROXY, HTTPS_PROXY, NO_PROXY, and the default user agent.
	h2HTTPTransport = utilnet.SetTransportDefaults(h2HTTPTransport)
	h2Transport, err := wrapTransport(h2HTTPTransport, transportConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return InstrumentedRoundtripper(component, h2Transport), nil
}

// InstrumentedRoundtripper instruments the provided RoundTripper with
// Prometheus metrics and OpenTelemetry tracing.
func InstrumentedRoundtripper(component string, tr http.RoundTripper) http.RoundTripper {
	// Define functions for the available httptrace.ClientTrace hook
	// functions that we want to instrument.
	httpTrace := &promhttp.InstrumentTrace{
		GotConn: func(t float64) {
			clietGotConnLatencyVec.WithLabelValues(component).Observe(t)
		},
		GotFirstResponseByte: func(t float64) {
			clientFirstByteLatencyVec.WithLabelValues(component).Observe(t)
		},
		TLSHandshakeStart: func(t float64) {
			clientTLSLatencyVec.WithLabelValues(component, "tls_handshake_start").Observe(t)
		},
		TLSHandshakeDone: func(t float64) {
			clientTLSLatencyVec.WithLabelValues(component, "tls_handshake_done").Observe(t)
		},
	}
	curryWith := prometheus.Labels{"component": component}
	return tracehttp.NewTransportWithInner(
		promhttp.InstrumentRoundTripperInFlight(
			clientInFlightGauge.WithLabelValues(component),
			promhttp.InstrumentRoundTripperCounter(
				clientRequestCounter.MustCurryWith(curryWith),
				promhttp.InstrumentRoundTripperTrace(
					httpTrace,
					promhttp.InstrumentRoundTripperDuration(clientRequestDurationHistVec.MustCurryWith(curryWith), tr),
				),
			),
		),
		// Pass the original RoundTripper to the inner transport so that it can
		// be used to close idle connections because promhttp roundtrippers don't
		// implement CloseIdleConnections.
		tr,
	)
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

// DialContextFunc is a context network dialer function that returns a network connection
type DialContextFunc func(context.Context, string, string) (net.Conn, error)

// newH2Transport creates a new HTTP/2 transport with ALPN support.
func NewH2Transport(tlsConfig *tls.Config, dial DialContextFunc) (*http.Transport, error) {
	tlsConfig = tlsConfig.Clone()
	if tlsConfig == nil {
		tlsConfig = &tls.Config{}
	}
	tlsConfig.NextProtos = []string{http2.NextProtoTLS, teleport.HTTPNextProtoTLS}
	h2HTTPTransport := NewTransport(dial, tlsConfig)
	// Upgrade transport to h2 where HTTP_PROXY and HTTPS_PROXY
	// envs are not take into account purposely.
	if err := http2.ConfigureTransport(h2HTTPTransport); err != nil {
		return nil, trace.Wrap(err)
	}
	return h2HTTPTransport, nil
}

// newTransport creates a new [http.Transport] with the provided dialer and TLS
// config.
// The transport is configured to use a connection pool and to close idle
// connections after a timeout.
func NewTransport(dial DialContextFunc, tlsConfig *tls.Config) *http.Transport {
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

// parseKubeHost parses and formats kubernetes hostname
// to host:port format, if no port it set,
// it assumes default HTTPS port
func parseKubeHost(host string) (string, error) {
	u, err := url.Parse(host)
	if err != nil {
		return "", trace.Wrap(err, "failed to parse Kubernetes host: %v", err)
	}
	if _, _, err := net.SplitHostPort(u.Host); err != nil {
		// add default HTTPS port
		return fmt.Sprintf("%v:443", u.Host), nil
	}
	return u.Host, nil
}

func checkImpersonationPermissions(ctx context.Context, cluster string, sarClient authztypes.SelfSubjectAccessReviewInterface) error {
	for _, resource := range []string{"users", "groups", "serviceaccounts"} {
		resp, err := sarClient.Create(ctx, &authzapi.SelfSubjectAccessReview{
			Spec: authzapi.SelfSubjectAccessReviewSpec{
				ResourceAttributes: &authzapi.ResourceAttributes{
					Verb:     "impersonate",
					Resource: resource,
				},
			},
		}, metav1.CreateOptions{})
		if err != nil {
			return trace.Wrap(err, "failed to verify impersonation permissions for Kubernetes: %v; this may be due to missing the SelfSubjectAccessReview permission on the ClusterRole used by the proxy; please make sure that proxy has all the necessary permissions: https://goteleport.com/teleport/docs/kubernetes-ssh/#impersonation", err)
		}
		if !resp.Status.Allowed {
			return trace.AccessDenied("proxy can't impersonate Kubernetes %s at the cluster level; please make sure that proxy has all the necessary permissions: https://goteleport.com/teleport/docs/kubernetes-ssh/#impersonation", resource)
		}
	}
	return nil
}


func CheckImpersonationPermissions(ctx context.Context, cluster string, sarClient authztypes.SelfSubjectAccessReviewInterface) error {
	for _, resource := range []string{"users", "groups", "serviceaccounts"} {
		resp, err := sarClient.Create(ctx, &authzapi.SelfSubjectAccessReview{
			Spec: authzapi.SelfSubjectAccessReviewSpec{
				ResourceAttributes: &authzapi.ResourceAttributes{
					Verb:     "impersonate",
					Resource: resource,
				},
			},
		}, metav1.CreateOptions{})
		if err != nil {
			return trace.Wrap(err, "failed to verify impersonation permissions for Kubernetes: %v; this may be due to missing the SelfSubjectAccessReview permission on the ClusterRole used by the proxy; please make sure that proxy has all the necessary permissions: https://goteleport.com/teleport/docs/kubernetes-ssh/#impersonation", err)
		}
		if !resp.Status.Allowed {
			return trace.AccessDenied("proxy can't impersonate Kubernetes %s at the cluster level; please make sure that proxy has all the necessary permissions: https://goteleport.com/teleport/docs/kubernetes-ssh/#impersonation", resource)
		}
	}
	return nil
}
