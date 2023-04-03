/*
Copyright 2018-2019 Gravitational, Inc.

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
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/http2"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/utils"
)

// TLSServerConfig is a configuration for TLS server
type TLSServerConfig struct {
	// ForwarderConfig is a config of a forwarder
	ForwarderConfig
	// TLS is a base TLS configuration
	TLS *tls.Config
	// LimiterConfig is limiter config
	LimiterConfig limiter.Config
	// AccessPoint is caching access point
	AccessPoint auth.ReadKubernetesAccessPoint
	// OnHeartbeat is a callback for kubernetes_service heartbeats.
	OnHeartbeat func(error)
	// ConnectedProxyGetter gets the proxies teleport is connected to.
	ConnectedProxyGetter *reversetunnel.ConnectedProxyGetter
	// Log is the logger.
	Log log.FieldLogger
}

// CheckAndSetDefaults checks and sets default values
func (c *TLSServerConfig) CheckAndSetDefaults() error {
	if err := c.ForwarderConfig.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if c.TLS == nil {
		return trace.BadParameter("missing parameter TLS")
	}
	c.TLS.ClientAuth = tls.RequireAndVerifyClientCert
	if c.TLS.ClientCAs == nil {
		return trace.BadParameter("missing parameter TLS.ClientCAs")
	}
	if c.TLS.RootCAs == nil {
		return trace.BadParameter("missing parameter TLS.RootCAs")
	}
	if len(c.TLS.Certificates) == 0 {
		return trace.BadParameter("missing parameter TLS.Certificates")
	}
	if c.AccessPoint == nil {
		return trace.BadParameter("missing parameter AccessPoint")
	}

	if err := c.validateLabelKeys(); err != nil {
		return trace.Wrap(err)
	}

	if c.Log == nil {
		c.Log = log.New()
	}
	if c.ConnectedProxyGetter == nil {
		c.ConnectedProxyGetter = reversetunnel.NewConnectedProxyGetter()
	}

	return nil
}

// validateLabelKeys checks that all labels keys are valid.
// Dynamic labels are validated in labels.NewDynamicLabels.
func (c *TLSServerConfig) validateLabelKeys() error {
	for name := range c.StaticLabels {
		if !types.IsValidLabelKey(name) {
			return trace.BadParameter("invalid label key: %q", name)
		}
	}
	return nil
}

// TLSServer is TLS auth server
type TLSServer struct {
	*http.Server
	// TLSServerConfig is TLS server configuration used for auth server
	TLSServerConfig
	fwd       *Forwarder
	mu        sync.Mutex
	listener  net.Listener
	heartbeat *srv.Heartbeat
}

// NewTLSServer returns new unstarted TLS server
func NewTLSServer(cfg TLSServerConfig) (*TLSServer, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	// limiter limits requests by frequency and amount of simultaneous
	// connections per client
	limiter, err := limiter.NewLimiter(cfg.LimiterConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fwd, err := NewForwarder(cfg.ForwarderConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// authMiddleware authenticates request assuming TLS client authentication
	// adds authentication information to the context
	// and passes it to the API server
	authMiddleware := &auth.Middleware{
		AccessPoint:   cfg.AccessPoint,
		AcceptedUsage: []string{teleport.UsageKubeOnly},
	}
	authMiddleware.Wrap(fwd)
	// Wrap sets the next middleware in chain to the authMiddleware
	limiter.WrapHandle(authMiddleware)
	// force client auth if given
	cfg.TLS.ClientAuth = tls.VerifyClientCertIfGiven

	server := &TLSServer{
		fwd:             fwd,
		TLSServerConfig: cfg,
		Server: &http.Server{
			Handler:           httplib.MakeTracingHandler(limiter, teleport.ComponentKube),
			ReadHeaderTimeout: apidefaults.DefaultIOTimeout * 2,
			IdleTimeout:       apidefaults.DefaultIdleTimeout,
			TLSConfig:         cfg.TLS,
		},
	}
	server.TLS.GetConfigForClient = server.GetConfigForClient

	// Start the heartbeat to announce kubernetes_service presence.
	//
	// Only announce when running in an actual kubernetes_service, or when
	// running in proxy_service with local kube credentials. This means that
	// proxy_service will pretend to also be kubernetes_service.
	if cfg.KubeServiceType == KubeService ||
		(cfg.KubeServiceType == LegacyProxyService && len(fwd.kubeClusters()) > 0) {
		log.Debugf("Starting kubernetes_service heartbeats for %q", cfg.Component)
		server.heartbeat, err = srv.NewHeartbeat(srv.HeartbeatConfig{
			Mode:            srv.HeartbeatModeKube,
			Context:         cfg.Context,
			Component:       cfg.Component,
			Announcer:       cfg.AuthClient,
			GetServerInfo:   server.GetServerInfo,
			KeepAlivePeriod: apidefaults.ServerKeepAliveTTL(),
			AnnouncePeriod:  apidefaults.ServerAnnounceTTL/2 + utils.RandomDuration(apidefaults.ServerAnnounceTTL/10),
			ServerTTL:       apidefaults.ServerAnnounceTTL,
			CheckPeriod:     defaults.HeartbeatCheckPeriod,
			Clock:           cfg.Clock,
			OnHeartbeat:     cfg.OnHeartbeat,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		log.Debug("No local kube credentials on proxy, will not start kubernetes_service heartbeats")
	}

	fwd.getKubernetesServersForKubeCluster, err = server.getKubernetesServiceFunc()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return server, nil
}

// Serve takes TCP listener, upgrades to TLS using config and starts serving
func (t *TLSServer) Serve(listener net.Listener) error {
	// Wrap listener with a multiplexer to get Proxy Protocol support.
	mux, err := multiplexer.New(multiplexer.Config{
		Context:             t.Context,
		Listener:            listener,
		Clock:               t.Clock,
		EnableProxyProtocol: true,
		ID:                  t.Component,
		// Increases deadline until the agent receives the first byte to 10s.
		// It's required to accommodate setups with high latency and where the time
		// between the TCP being accepted and the time for the first byte is longer
		// than the default value -  1s.
		ReadDeadline: 10 * time.Second,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	go mux.Serve()
	defer mux.Close()

	t.mu.Lock()
	t.listener = mux.TLS()
	if err = http2.ConfigureServer(t.Server, &http2.Server{}); err != nil {
		return trace.Wrap(err)
	}
	t.mu.Unlock()

	if t.heartbeat != nil {
		go t.heartbeat.Run()
	}

	return t.Server.Serve(tls.NewListener(mux.TLS(), t.TLS))
}

// Close closes the server and cleans up all resources.
func (t *TLSServer) Close() error {
	errs := []error{t.Server.Close()}
	if t.heartbeat != nil {
		errs = append(errs, t.heartbeat.Close())
	}
	return trace.NewAggregate(errs...)
}

// GetConfigForClient is getting called on every connection
// and server's GetConfigForClient reloads the list of trusted
// local and remote certificate authorities
func (t *TLSServer) GetConfigForClient(info *tls.ClientHelloInfo) (*tls.Config, error) {
	return auth.WithClusterCAs(t.TLS, t.AccessPoint, t.ClusterName, t.Log)(info)
}

// GetServerInfo returns a services.Server object for heartbeats (aka
// presence).
func (t *TLSServer) GetServerInfo() (types.Resource, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	var addr string
	if t.TLSServerConfig.ForwarderConfig.PublicAddr != "" {
		addr = t.TLSServerConfig.ForwarderConfig.PublicAddr
	} else if t.listener != nil {
		addr = t.listener.Addr().String()
	}

	// Both proxy and kubernetes services can run in the same instance (same
	// ServerID). Add a name suffix to make them distinct.
	//
	// Note: we *don't* want to add suffix for kubernetes_service!
	// This breaks reverse tunnel routing, which uses server.Name.
	name := t.ServerID
	if t.KubeServiceType != KubeService {
		name += "-proxy_service"
	}

	srv := &types.ServerV2{
		Kind:    types.KindKubeService,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      name,
			Namespace: t.Namespace,
		},
		Spec: types.ServerSpecV2{
			Addr:               addr,
			Version:            teleport.Version,
			KubernetesClusters: t.fwd.kubeClusters(),
			ProxyIDs:           t.ConnectedProxyGetter.GetProxyIDs(),
		},
	}
	srv.SetExpiry(t.Clock.Now().UTC().Add(apidefaults.ServerAnnounceTTL))

	return srv, nil
}

// getKubernetesServiceFunc returns a function that returns the kubernetes services
func (t *TLSServer) getKubernetesServiceFunc() (getKubeServicesByNameFunc, error) {
	switch t.KubeServiceType {
	case KubeService:
		return func(_ context.Context, name string) ([]types.Server, error) {
			resource, err := t.GetServerInfo()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			srv, ok := resource.(types.Server)
			if !ok {
				return nil, trace.BadParameter("unexpected type %T", resource)
			}
			return []types.Server{srv}, nil
		}, nil
	case ProxyService:
		return t.getAuthKubeServices, nil
	case LegacyProxyService:
		return func(ctx context.Context, name string) ([]types.Server, error) {
			servers, err := t.getLocalKubeServiceForCluster(name)
			if err != nil {
				servers, err := t.getAuthKubeServices(ctx, name)
				return servers, trace.Wrap(err)
			}
			return servers, nil
		}, nil
	default:
		return nil, trace.BadParameter("unknown kubernetes service type %q", t.KubeServiceType)
	}
}

// getAuthKubeServers returns the kubernetes servers for a given kube cluster
// using the Auth server client.
func (t *TLSServer) getAuthKubeServices(ctx context.Context, name string) ([]types.Server, error) {
	servers, err := t.CachingAuthClient.GetKubeServices(ctx)
	return servers, trace.Wrap(err)
}

// getLocalKubeServiceForCluster returns the local kubernetes service if it
// includes the given cluster.
func (t *TLSServer) getLocalKubeServiceForCluster(clusterName string) ([]types.Server, error) {
	resource, err := t.GetServerInfo()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	srv, ok := resource.(types.Server)
	if !ok {
		return nil, trace.BadParameter("unexpected type %T", resource)
	}
	for _, cluster := range srv.GetKubernetesClusters() {
		if cluster.Name == clusterName {
			return []types.Server{srv}, nil
		}
	}
	return nil, trace.NotFound("kubernetes cluster %q not found", clusterName)
}
