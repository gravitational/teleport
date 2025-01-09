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
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
)

type kubeCreds interface {
	getTLSConfig() *tls.Config
	getTransportConfig() *transport.Config
	getTargetAddr() string
	getKubeRestConfig() *rest.Config
	getKubeClient() kubernetes.Interface
	getTransport() http.RoundTripper
	wrapTransport(http.RoundTripper) (http.RoundTripper, error)
	close() error
}

var (
	_ kubeCreds = &staticKubeCreds{}
	_ kubeCreds = &dynamicKubeCreds{}
)

// staticKubeCreds contain authentication-related fields from kubeconfig.
//
// TODO(awly): make this an interface, one implementation for local k8s cluster
// and another for a remote teleport cluster.
type staticKubeCreds struct {
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

func (s *staticKubeCreds) getTLSConfig() *tls.Config {
	return s.tlsConfig.Clone()
}

func (s *staticKubeCreds) getTransport() http.RoundTripper {
	return s.transport
}

func (s *staticKubeCreds) getTransportConfig() *transport.Config {
	return s.transportConfig
}

func (s *staticKubeCreds) getTargetAddr() string {
	return s.targetAddr
}

func (s *staticKubeCreds) getKubeClient() kubernetes.Interface {
	return s.kubeClient
}

func (s *staticKubeCreds) getKubeRestConfig() *rest.Config {
	return s.clientRestCfg
}

func (s *staticKubeCreds) wrapTransport(rt http.RoundTripper) (http.RoundTripper, error) {
	if s == nil {
		return rt, nil
	}

	wrapped, err := transport.HTTPWrappersForConfig(s.transportConfig, rt)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return enforceCloseIdleConnections(wrapped, rt), nil
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

func (s *staticKubeCreds) close() error {
	return nil
}

// dynamicCredsClient defines the function signature used by `dynamicCreds`
// to generate and renew short-lived credentials to access the cluster.
type dynamicCredsClient func(ctx context.Context, cluster types.KubeCluster) (cfg *rest.Config, expirationTime time.Time, err error)

// dynamicKubeCreds contains short-lived credentials to access the cluster.
// Unlike `staticKubeCreds`, `dynamicKubeCreds` extracts access credentials using the `client`
// function and renews them whenever they are about to expire.
type dynamicKubeCreds struct {
	ctx         context.Context
	renewTicker clockwork.Ticker
	staticCreds *staticKubeCreds
	log         *slog.Logger
	closeC      chan struct{}
	client      dynamicCredsClient
	checker     servicecfg.ImpersonationPermissionsChecker
	clock       clockwork.Clock
	component   KubeServiceType
	sync.RWMutex
	wg sync.WaitGroup
}

// dynamicCredsConfig contains configuration for dynamicKubeCreds.
type dynamicCredsConfig struct {
	kubeCluster          types.KubeCluster
	log                  *slog.Logger
	client               dynamicCredsClient
	checker              servicecfg.ImpersonationPermissionsChecker
	clock                clockwork.Clock
	initialRenewInterval time.Duration
	resourceMatchers     []services.ResourceMatcher
	component            KubeServiceType
}

func (d *dynamicCredsConfig) checkAndSetDefaults() error {
	if d.kubeCluster == nil {
		return trace.BadParameter("missing kubeCluster")
	}
	if d.log == nil {
		return trace.BadParameter("missing log")
	}
	if d.client == nil {
		return trace.BadParameter("missing client")
	}
	if d.checker == nil {
		return trace.BadParameter("missing checker")
	}
	if d.clock == nil {
		d.clock = clockwork.NewRealClock()
	}
	if d.initialRenewInterval == 0 {
		d.initialRenewInterval = time.Hour
	}
	return nil
}

// newDynamicKubeCreds creates a new dynamicKubeCreds refresher and starts the
// credentials refresher mechanism to renew them once they are about to expire.
func newDynamicKubeCreds(ctx context.Context, cfg dynamicCredsConfig) (*dynamicKubeCreds, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	dyn := &dynamicKubeCreds{
		ctx:         ctx,
		log:         cfg.log,
		closeC:      make(chan struct{}),
		client:      cfg.client,
		renewTicker: cfg.clock.NewTicker(cfg.initialRenewInterval),
		checker:     cfg.checker,
		clock:       cfg.clock,
		component:   cfg.component,
	}

	if err := dyn.renewClientset(cfg.kubeCluster); err != nil {
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
				if err := dyn.renewClientset(cfg.kubeCluster); err != nil {
					cfg.log.WarnContext(ctx, "Unable to renew cluster credentials", "cluster", cfg.kubeCluster.GetName(), "error", err)
				}
			}
		}
	}()

	return dyn, nil
}

func (d *dynamicKubeCreds) getTLSConfig() *tls.Config {
	d.RLock()
	defer d.RUnlock()
	return d.staticCreds.tlsConfig
}

func (d *dynamicKubeCreds) getTransportConfig() *transport.Config {
	d.RLock()
	defer d.RUnlock()
	return d.staticCreds.transportConfig
}

func (d *dynamicKubeCreds) getKubeRestConfig() *rest.Config {
	d.RLock()
	defer d.RUnlock()
	return d.staticCreds.clientRestCfg
}

func (d *dynamicKubeCreds) getTargetAddr() string {
	d.RLock()
	defer d.RUnlock()
	return d.staticCreds.targetAddr
}

func (d *dynamicKubeCreds) getKubeClient() kubernetes.Interface {
	d.RLock()
	defer d.RUnlock()
	return d.staticCreds.kubeClient
}

func (d *dynamicKubeCreds) wrapTransport(rt http.RoundTripper) (http.RoundTripper, error) {
	d.RLock()
	defer d.RUnlock()
	return d.staticCreds.wrapTransport(rt)
}

func (d *dynamicKubeCreds) close() error {
	close(d.closeC)
	d.wg.Wait()
	d.renewTicker.Stop()
	return nil
}

func (d *dynamicKubeCreds) getTransport() http.RoundTripper {
	d.RLock()
	defer d.RUnlock()
	return d.staticCreds.getTransport()
}

// renewClientset generates the credentials required for accessing the cluster using the client function.
func (d *dynamicKubeCreds) renewClientset(cluster types.KubeCluster) error {
	// get auth config
	restConfig, exp, err := d.client(d.ctx, cluster)
	if err != nil {
		return trace.Wrap(err)
	}
	creds, err := extractKubeCreds(d.ctx, d.component, cluster.GetName(), restConfig, d.log, d.checker)
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
