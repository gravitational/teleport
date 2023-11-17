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
	"encoding/base64"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
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
		config.ALPNConnUpgradeRequired = client.IsALPNConnUpgradeRequired(ctx, config.RemoteProxyAddr, config.InsecureSkipVerify)
		return trace.Wrap(WithClusterCAsIfConnUpgrade(ctx, getClusterCertPool)(config))
	}
}

// WithClusterCAsIfConnUpgrade is a LocalProxyConfigOpt that fetches the
// cluster CAs when ALPN connection upgrades are required.
func WithClusterCAsIfConnUpgrade(ctx context.Context, getClusterCertPool GetClusterCACertPoolFunc) LocalProxyConfigOpt {
	return func(config *LocalProxyConfig) error {
		if !config.ALPNConnUpgradeRequired {
			return nil
		}

		// If ALPN connection upgrade is required, explicitly use the cluster
		// CAs since the tunneled TLS routing connection serves the Host cert.
		return trace.Wrap(WithClusterCAs(ctx, getClusterCertPool)(config))
	}
}

// WithClusterCAs is a LocalProxyConfigOpt that fetches the cluster CAs.
func WithClusterCAs(ctx context.Context, getClusterCertPool GetClusterCACertPoolFunc) LocalProxyConfigOpt {
	return func(config *LocalProxyConfig) error {
		clusterCAs, err := getClusterCertPool(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		config.RootCAs = clusterCAs
		return nil
	}
}

// WithClientCerts is a LocalProxyConfigOpt that sets the client certs used to
// connect to the remote Teleport Proxy.
func WithClientCerts(certs ...tls.Certificate) LocalProxyConfigOpt {
	return func(config *LocalProxyConfig) error {
		config.Certs = certs
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

// WithDatabaseProtocol is a LocalProxyConfigOpt that sets the ALPN protocol
// based on the provided database protocol.
func WithDatabaseProtocol(dbProtocol string) LocalProxyConfigOpt {
	return func(config *LocalProxyConfig) error {
		alpnProtocol, err := common.ToALPNProtocol(dbProtocol)
		if err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(WithALPNProtocol(alpnProtocol)(config))
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

// WithMiddleware is a LocalProxyConfigOpt that sets Middleware.
func WithMiddleware(middleware LocalProxyMiddleware) LocalProxyConfigOpt {
	return func(config *LocalProxyConfig) error {
		config.Middleware = middleware
		return nil
	}
}

// WithCheckCertsNeeded is a LocalProxyConfigOpt that enables check certs on
// demand.
func WithCheckCertsNeeded() LocalProxyConfigOpt {
	return func(config *LocalProxyConfig) error {
		config.CheckCertsNeeded = true
		return nil
	}
}

// WithSNI is a LocalProxyConfigOpt that sets the SNI.
func WithSNI(sni string) LocalProxyConfigOpt {
	return func(config *LocalProxyConfig) error {
		config.SNI = sni
		return nil
	}
}

// WithMySQLVersionProto is a LocalProxyConfigOpt that encodes MySQL version in
// the ALPN protocol.
func WithMySQLVersionProto(db types.Database) LocalProxyConfigOpt {
	return func(config *LocalProxyConfig) error {
		mysqlServerVersionProto := mySQLVersionToProto(db)
		if mysqlServerVersionProto != "" {
			config.Protocols = append(config.Protocols, common.Protocol(mysqlServerVersionProto))
		}
		return nil
	}
}

// mySQLVersionToProto returns base64 encoded MySQL server version with MySQL protocol prefix.
// If version is not set in the past database an empty string is returned.
func mySQLVersionToProto(database types.Database) string {
	version := database.GetMySQLServerVersion()
	if version == "" {
		return ""
	}

	versionBase64 := base64.StdEncoding.EncodeToString([]byte(version))

	// Include MySQL server version
	return string(common.ProtocolMySQLWithVerPrefix) + versionBase64
}
