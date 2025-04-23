// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package authclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"log/slog"
	"math"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/tlsca"
)

// CAGetter is an interface for retrieving certificate authorities.
type CAGetter interface {
	// GetCertAuthority returns a single cert authority by id.
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)

	// GetCertAuthorities returns all cert authorities of a specific type.
	GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadKeys bool) ([]types.CertAuthority, error)
}

// HostAndUserCAInfo is a map of CA raw subjects and type info for Host
// and User CAs. The key is the RawSubject of the X.509 certificate authority
// (so it's ASN.1 data, not printable).
type HostAndUserCAInfo = map[string]CATypeInfo

// CATypeInfo indicates whether the CA is a host or user CA, or both.
type CATypeInfo struct {
	IsHostCA bool
	IsUserCA bool
}

// ClientCertPool returns trusted x509 certificate authority pool with CAs provided as caType.
// In addition, it returns the total length of all subjects added to the cert pool, allowing
// the caller to validate that the pool doesn't exceed the maximum 2-byte length prefix before
// using it.
func ClientCertPool(ctx context.Context, client CAGetter, clusterName string, caType types.CertAuthType) (*x509.CertPool, int64, error) {
	authorities, err := getCACerts(ctx, client, clusterName, caType)
	if err != nil {
		return nil, 0, trace.Wrap(err)
	}

	pool := x509.NewCertPool()
	var totalSubjectsLen int64
	for _, auth := range authorities {
		for _, keyPair := range auth.GetTrustedTLSKeyPairs() {
			cert, err := tlsca.ParseCertificatePEM(keyPair.Cert)
			if err != nil {
				return nil, 0, trace.Wrap(err)
			}
			pool.AddCert(cert)

			// Each subject in the list gets a separate 2-byte length prefix.
			totalSubjectsLen += 2
			totalSubjectsLen += int64(len(cert.RawSubject))
		}
	}
	return pool, totalSubjectsLen, nil
}

// DefaultClientCertPool returns default trusted x509 certificate authority pool.
func DefaultClientCertPool(ctx context.Context, client CAGetter, clusterName string) (*x509.CertPool, HostAndUserCAInfo, int64, error) {
	authorities, err := getCACerts(ctx, client, clusterName, types.HostCA, types.UserCA)
	if err != nil {
		return nil, nil, 0, trace.Wrap(err)
	}

	pool := x509.NewCertPool()
	caInfos := make(HostAndUserCAInfo, len(authorities))
	var totalSubjectsLen int64
	for _, auth := range authorities {
		for _, keyPair := range auth.GetTrustedTLSKeyPairs() {
			cert, err := tlsca.ParseCertificatePEM(keyPair.Cert)
			if err != nil {
				return nil, nil, 0, trace.Wrap(err)
			}
			pool.AddCert(cert)

			caType := auth.GetType()
			caInfo := caInfos[string(cert.RawSubject)]
			switch caType {
			case types.HostCA:
				caInfo.IsHostCA = true
			case types.UserCA:
				caInfo.IsUserCA = true
			default:
				return nil, nil, 0, trace.BadParameter("unexpected CA type %q", caType)
			}
			caInfos[string(cert.RawSubject)] = caInfo

			// Each subject in the list gets a separate 2-byte length prefix.
			totalSubjectsLen += 2
			totalSubjectsLen += int64(len(cert.RawSubject))
		}
	}

	return pool, caInfos, totalSubjectsLen, nil
}

func getCACerts(ctx context.Context, client CAGetter, clusterName string, caTypes ...types.CertAuthType) ([]types.CertAuthority, error) {
	if len(caTypes) == 0 {
		return nil, trace.BadParameter("at least one CA type is required")
	}

	var authorities []types.CertAuthority
	if clusterName == "" {
		for _, caType := range caTypes {
			cas, err := client.GetCertAuthorities(ctx, caType, false)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			authorities = append(authorities, cas...)
		}
	} else {
		for _, caType := range caTypes {
			ca, err := client.GetCertAuthority(
				ctx,
				types.CertAuthID{Type: caType, DomainName: clusterName},
				false)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			authorities = append(authorities, ca)
		}
	}

	return authorities, nil
}

// WithClusterCAs returns a TLS hello callback that returns a copy of the provided
// TLS config with client CAs pool of the specified cluster.
func WithClusterCAs(tlsConfig *tls.Config, ap CAGetter, currentClusterName string, logger *slog.Logger) func(*tls.ClientHelloInfo) (*tls.Config, error) {
	return func(info *tls.ClientHelloInfo) (*tls.Config, error) {
		var clusterName string
		var err error
		if info.ServerName != "" {
			// Newer clients will set SNI that encodes the cluster name.
			clusterName, err = apiutils.DecodeClusterName(info.ServerName)
			if err != nil {
				if !trace.IsNotFound(err) {
					logger.DebugContext(info.Context(), "Ignoring unsupported cluster name name", "cluster_name", info.ServerName)
					clusterName = ""
				}
			}
		}
		pool, _, totalSubjectsLen, err := DefaultClientCertPool(info.Context(), ap, clusterName)
		if err != nil {
			logger.ErrorContext(info.Context(), "Failed to retrieve client pool for cluster", "error", err, "cluster", clusterName)
			// this falls back to the default config
			return nil, nil
		}

		// Per https://tools.ietf.org/html/rfc5246#section-7.4.4 the total size of
		// the known CA subjects sent to the client can't exceed 2^16-1 (due to
		// 2-byte length encoding). The crypto/tls stack will panic if this
		// happens.
		//
		// This usually happens on the root cluster with a very large (>500) number
		// of leaf clusters. In these cases, the client cert will be signed by the
		// current (root) cluster.
		//
		// If the number of CAs turns out too large for the handshake, drop all but
		// the current cluster CA. In the unlikely case where it's wrong, the
		// client will be rejected.
		if totalSubjectsLen >= int64(math.MaxUint16) {
			logger.DebugContext(info.Context(), "Number of CAs in client cert pool is too large and cannot be encoded in a TLS handshake; this is due to a large number of trusted clusters; will use only the CA of the current cluster to validate")

			pool, _, _, err = DefaultClientCertPool(info.Context(), ap, currentClusterName)
			if err != nil {
				logger.ErrorContext(info.Context(), "Failed to retrieve client pool for cluster", "error", err, "cluster", currentClusterName)
				// this falls back to the default config
				return nil, nil
			}
		}
		tlsCopy := tlsConfig.Clone()
		tlsCopy.ClientCAs = pool
		return tlsCopy, nil
	}
}
