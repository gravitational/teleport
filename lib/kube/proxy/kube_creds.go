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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
)

type kubeCreds interface {
	getTLSConfig() *tls.Config
	getTransportConfig() *transport.Config
	getTargetAddr() string
	getKubeClient() *kubernetes.Clientset
	wrapTransport(http.RoundTripper) (http.RoundTripper, error)
	close() error
}

var (
	_ kubeCreds = &staticKubeCreds{}
	_ kubeCreds = &dynamicCreds{}
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

func (s *staticKubeCreds) wrapTransport(rt http.RoundTripper) (http.RoundTripper, error) {
	if s == nil {
		return rt, nil
	}
	return transport.HTTPWrappersForConfig(s.transportConfig, rt)
}

func (s *staticKubeCreds) close() error {
	return nil
}

type dynamicCredsClient func(context.Context, types.KubeCluster) (*rest.Config, time.Time, error)

type dynamicCreds struct {
	ctx         context.Context
	renewTicker *time.Ticker
	st          *staticKubeCreds
	log         logrus.FieldLogger
	closeC      chan struct{}
	client      dynamicCredsClient
	checker     ImpersonationPermissionsChecker
	sync.RWMutex
}

func newDynamicCreds(ctx context.Context, kubeCluster types.KubeCluster, log logrus.FieldLogger, client dynamicCredsClient, checker ImpersonationPermissionsChecker) (*dynamicCreds, error) {
	dyn := &dynamicCreds{
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
		select {
		case <-dyn.closeC:
			return
		case <-dyn.renewTicker.C:
			if err := dyn.renewClientset(kubeCluster); err != nil {
				log.WithError(err).Warnf("unable to renew cluster %q credentials", kubeCluster.GetName())
			}
		}
	}()

	return dyn, nil
}

func (d *dynamicCreds) getTLSConfig() *tls.Config {
	d.RLock()
	defer d.RUnlock()
	return d.st.tlsConfig
}
func (d *dynamicCreds) getTransportConfig() *transport.Config {
	d.RLock()
	defer d.RUnlock()
	return d.st.transportConfig
}
func (d *dynamicCreds) getTargetAddr() string {
	d.RLock()
	defer d.RUnlock()
	return d.st.targetAddr
}
func (d *dynamicCreds) getKubeClient() *kubernetes.Clientset {
	d.RLock()
	defer d.RUnlock()
	return d.st.kubeClient
}
func (d *dynamicCreds) wrapTransport(rt http.RoundTripper) (http.RoundTripper, error) {
	d.RLock()
	defer d.RUnlock()
	return d.st.wrapTransport(rt)
}

func (d *dynamicCreds) close() error {
	close(d.closeC)
	return nil
}

// renewClientset generates the credentials required for accessing the cluster using the GetAuthConfig function provided by watcher.
func (d *dynamicCreds) renewClientset(cluster types.KubeCluster) error {
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
	d.st = creds
	// prepares the next renew cycle
	if !exp.IsZero() {
		d.renewTicker.Reset(time.Until(exp) / 2)
	}
	return nil
}
