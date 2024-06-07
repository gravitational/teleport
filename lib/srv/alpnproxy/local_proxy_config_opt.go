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

// WithClientCert is a LocalProxyConfigOpt that sets the client certs used to
// connect to the remote Teleport Proxy.
func WithClientCert(cert tls.Certificate) LocalProxyConfigOpt {
	return func(config *LocalProxyConfig) error {
		config.Cert = cert
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

// WithCheckCertNeeded is a LocalProxyConfigOpt that enables check certs on
// demand.
func WithCheckCertNeeded() LocalProxyConfigOpt {
	return func(config *LocalProxyConfig) error {
		config.CheckCertNeeded = true
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
