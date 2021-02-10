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
	"crypto/tls"
	"net"
	"net/http"
	"sync"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/server"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
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
	AccessPoint auth.AccessPoint
	// OnHeartbeat is a callback for kubernetes_service heartbeats.
	OnHeartbeat func(error)
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
	authMiddleware := &server.Middleware{
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
			Handler:           limiter,
			ReadHeaderTimeout: defaults.DefaultDialTimeout * 2,
		},
	}
	server.TLS.GetConfigForClient = server.GetConfigForClient

	// Start the heartbeat to announce kubernetes_service presence.
	//
	// Only announce when running in an actual kubernetes_service, or when
	// running in proxy_service with local kube credentials. This means that
	// proxy_service will pretend to also be kubernetes_service.
	if cfg.NewKubeService || len(fwd.kubeClusters()) > 0 {
		log.Debugf("Starting kubernetes_service heartbeats for %q", cfg.Component)
		server.heartbeat, err = srv.NewHeartbeat(srv.HeartbeatConfig{
			Mode:            srv.HeartbeatModeKube,
			Context:         cfg.Context,
			Component:       cfg.Component,
			Announcer:       cfg.AuthClient,
			GetServerInfo:   server.GetServerInfo,
			KeepAlivePeriod: defaults.ServerKeepAliveTTL,
			AnnouncePeriod:  defaults.ServerAnnounceTTL/2 + utils.RandomDuration(defaults.ServerAnnounceTTL/10),
			ServerTTL:       defaults.ServerAnnounceTTL,
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
		DisableSSH:          true,
		ID:                  t.Component,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	go mux.Serve()
	defer mux.Close()

	t.mu.Lock()
	t.listener = mux.TLS()
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
	var clusterName string
	var err error
	if info.ServerName != "" {
		clusterName, err = auth.DecodeClusterName(info.ServerName)
		if err != nil {
			if !trace.IsNotFound(err) {
				log.Debugf("Ignoring unsupported cluster name name %q.", info.ServerName)
				clusterName = ""
			}
		}
	}
	pool, err := server.ClientCertPool(t.AccessPoint, clusterName)
	if err != nil {
		log.Errorf("failed to retrieve client pool: %v", trace.DebugReport(err))
		// this falls back to the default config
		return nil, nil
	}
	tlsCopy := t.TLS.Clone()
	tlsCopy.ClientCAs = pool
	return tlsCopy, nil
}

// GetServerInfo returns a services.Server object for heartbeats (aka
// presence).
func (t *TLSServer) GetServerInfo() (services.Resource, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	var addr string
	if t.listener != nil {
		addr = t.listener.Addr().String()
	}

	// Both proxy and kubernetes services can run in the same instance (same
	// ServerID). Add a name suffix to make them distinct.
	//
	// Note: we *don't* want to add suffix for kubernetes_service!
	// This breaks reverse tunnel routing, which uses server.Name.
	name := t.ServerID
	if !t.NewKubeService {
		name += "-proxy_service"
	}

	srv := &services.ServerV2{
		Kind:    services.KindKubeService,
		Version: services.V2,
		Metadata: services.Metadata{
			Name:      name,
			Namespace: t.Namespace,
		},
		Spec: services.ServerSpecV2{
			Addr:               addr,
			Version:            teleport.Version,
			KubernetesClusters: t.fwd.kubeClusters(),
		},
	}
	srv.SetExpiry(t.Clock.Now().UTC().Add(defaults.ServerAnnounceTTL))
	return srv, nil
}
