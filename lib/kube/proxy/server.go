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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/limiter"

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
	LimiterConfig limiter.LimiterConfig
	// AccessPoint is caching access point
	AccessPoint auth.AccessPoint
	// Component is used for debugging purposes
	Component string
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
		TLSServerConfig: cfg,
		Server: &http.Server{
			Handler:           limiter,
			ReadHeaderTimeout: defaults.DefaultDialTimeout * 2,
		},
	}
	server.TLS.GetConfigForClient = server.GetConfigForClient
	return server, nil
}

// Serve takes TCP listener, upgrades to TLS using config and starts serving
func (t *TLSServer) Serve(listener net.Listener) error {
	return t.Server.Serve(tls.NewListener(listener, t.TLS))
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
	pool, err := auth.ClientCertPool(t.AccessPoint, clusterName)
	if err != nil {
		log.Errorf("failed to retrieve client pool: %v", trace.DebugReport(err))
		// this falls back to the default config
		return nil, nil
	}
	tlsCopy := t.TLS.Clone()
	tlsCopy.ClientCAs = pool
	return tlsCopy, nil
}
