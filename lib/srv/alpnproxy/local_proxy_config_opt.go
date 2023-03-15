/*
Copyright 2023 Gravitational, Inc.

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

package alpnproxy

import (
	"context"
	"crypto/tls"
	"crypto/x509"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
)

// LocalProxyConfigOpt is an option func to update LocalProxyConfig.
type LocalProxyConfigOpt func(*LocalProxyConfig) error

// GetClusterCACertPoolFunc is a function to fetch cluster CAs.
type GetClusterCACertPoolFunc func(ctx context.Context) (*x509.CertPool, error)

// WithALPNConnUpgradeTest performs the test to see if ALPN connection upgrade
// is required and update other configs if necessary.
//
// This LocalProxyConfigOpt assumes RemoteProxyAddr and InsecureSkipVerify has
// already been set.
func WithALPNConnUpgradeTest(ctx context.Context, getClusterCertPool GetClusterCACertPoolFunc) LocalProxyConfigOpt {
	return func(config *LocalProxyConfig) error {
		config.ALPNConnUpgradeRequired = IsALPNConnUpgradeRequired(config.RemoteProxyAddr, config.InsecureSkipVerify)
		if !config.ALPNConnUpgradeRequired {
			return nil
		}

		// If ALPN connection upgrade is required, explicitly use the cluster
		// CAs since the tunneled TLS routing connection serves the Host cert.
		clusterCAs, err := getClusterCertPool(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		config.RootCAs = clusterCAs
		return nil
	}
}

// WithClientCert is a LocalProxyConfigOpt that sets the client certs used to
// connect to the remote Teleport Proxy.
func WithClientCert(certs tls.Certificate) LocalProxyConfigOpt {
	return func(config *LocalProxyConfig) error {
		config.Certs = []tls.Certificate{certs}
		return nil
	}
}

// WithALPNProtocol is a LocalProxyConfigOpt that sets the ALPN protocol used
// for TLS Routing.
func WithALPNProtocol(protocol common.Protocol) LocalProxyConfigOpt {
	return func(config *LocalProxyConfig) error {
		config.Protocols = []common.Protocol{protocol}
		return nil
	}
}

// WithHTTPMiddleware is a LocalProxyConfigOpt that sets HTTPMiddleware.
func WithHTTPMiddleware(middleware LocalProxyHTTPMiddleware) LocalProxyConfigOpt {
	return func(config *LocalProxyConfig) error {
		config.HTTPMiddleware = middleware

		// Set protocol to ProtocolHTTP if not set yet.
		if len(config.Protocols) == 0 {
			config.Protocols = []common.Protocol{common.ProtocolHTTP}
		}
		return nil
	}
}
