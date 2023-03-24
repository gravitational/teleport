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
	"net/http"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

type kubeCreds interface {
	getTLSConfig() *tls.Config
	getTransportConfig() *transport.Config
	getTargetAddr() string
	getKubeRestConfig() *rest.Config
	getKubeClient() *kubernetes.Clientset
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
	kubeClient *kubernetes.Clientset
	// clientRestCfg is the Kubernetes Rest config for the cluster.
	clientRestCfg *rest.Config
}

func (s *staticKubeCreds) getTLSConfig() *tls.Config {
	return s.tlsConfig
}

func (s *staticKubeCreds) getTransportConfig() *transport.Config {
	return s.transportConfig
}

func (s *staticKubeCreds) getTargetAddr() string {
	return s.targetAddr
}

func (s *staticKubeCreds) getKubeClient() *kubernetes.Clientset {
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
	renewTicker *time.Ticker
	staticCreds *staticKubeCreds
	log         logrus.FieldLogger
	closeC      chan struct{}
	client      dynamicCredsClient
	checker     servicecfg.ImpersonationPermissionsChecker
	sync.RWMutex
}

// newDynamicKubeCreds creates a new dynamicKubeCreds refresher and starts the
// credentials refresher mechanism to renew them once they are about to expire.
func newDynamicKubeCreds(ctx context.Context, kubeCluster types.KubeCluster, log logrus.FieldLogger, client dynamicCredsClient, checker servicecfg.ImpersonationPermissionsChecker) (*dynamicKubeCreds, error) {
	dyn := &dynamicKubeCreds{
		ctx:         ctx,
		log:         log,
		closeC:      make(chan struct{}),
		client:      client,
		renewTicker: time.NewTicker(time.Hour),
		checker:     checker,
	}

	if err := dyn.renewClientset(kubeCluster); err != nil {
		return nil, trace.Wrap(err)
	}

	go func() {
		for {
			select {
			case <-dyn.closeC:
				return
			case <-dyn.renewTicker.C:
				if err := dyn.renewClientset(kubeCluster); err != nil {
					log.WithError(err).Warnf("Unable to renew cluster %q credentials.", kubeCluster.GetName())
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

func (d *dynamicKubeCreds) getKubeClient() *kubernetes.Clientset {
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
	return nil
}

// renewClientset generates the credentials required for accessing the cluster using the client function.
func (d *dynamicKubeCreds) renewClientset(cluster types.KubeCluster) error {
	// get auth config
	restConfig, exp, err := d.client(d.ctx, cluster)
	if err != nil {
		return trace.Wrap(err)
	}
	creds, err := extractKubeCreds(d.ctx, cluster.GetName(), restConfig, d.log, d.checker)
	if err != nil {
		return trace.Wrap(err)
	}

	d.Lock()
	defer d.Unlock()
	d.staticCreds = creds
	// prepares the next renew cycle
	if !exp.IsZero() {
		d.renewTicker.Reset(time.Until(exp) / 2)
	}
	return nil
}
