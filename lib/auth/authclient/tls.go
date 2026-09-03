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

	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/tlsca"
)

// AccessCacheWithEvents extends the [AccessCache] interface with [types.Events].
// Useful for trust-related components that need to watch for changes.
type AccessCacheWithEvents interface {
	AccessCache
	types.Events
}

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

// defaultClientCertPool returns default trusted x509 certificate authority pool.
// Use [WithClusterCAs] for setting up TLS client authentication on servers,
// or [ClientTLSConfigGenerator] for cached, event-driven TLS configs.
func defaultClientCertPool(ctx context.Context, client CAGetter, clusterName string) (*x509.CertPool, HostAndUserCAInfo, int64, error) {
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
		// onPoolErr decides what to do when the client CA pool can't be loaded.
		// If client certs are required, fail closed by aborting the handshake.
		// Otherwise keep no-cert handshakes working but reject any presented cert:
		// the empty pool must be non-nil, since a nil ClientCAs makes crypto/tls verify against the host's system roots.
		onPoolErr := func(err error) (*tls.Config, error) {
			if tlsConfig.ClientAuth == tls.RequireAndVerifyClientCert || tlsConfig.ClientAuth == tls.RequireAnyClientCert {
				return nil, trace.Wrap(err)
			}
			tlsCopy := tlsConfig.Clone()
			tlsCopy.ClientCAs = x509.NewCertPool()
			tlsCopy.VerifyPeerCertificate = nil
			return tlsCopy, nil
		}

		var clusterName string
		var err error
		if info.ServerName != "" {
			// Newer clients will set SNI that encodes the cluster name.
			clusterName, err = apiutils.DecodeClusterName(info.ServerName)
			if err != nil {
				if !trace.IsNotFound(err) {
					logger.DebugContext(info.Context(), "Ignoring unsupported cluster name", "cluster_name", info.ServerName)
					clusterName = ""
				}
			}
		}
		pool, caMap, totalSubjectsLen, err := defaultClientCertPool(info.Context(), ap, clusterName)
		if err != nil {
			logger.ErrorContext(info.Context(), "Failed to retrieve client pool for cluster", "error", err, "cluster", clusterName)
			return onPoolErr(err)
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

			pool, caMap, _, err = defaultClientCertPool(info.Context(), ap, currentClusterName)
			if err != nil {
				logger.ErrorContext(info.Context(), "Failed to retrieve client pool for cluster", "error", err, "cluster", currentClusterName)
				return onPoolErr(err)
			}
		}
		tlsCopy := tlsConfig.Clone()
		tlsCopy.ClientCAs = pool
		tlsCopy.VerifyPeerCertificate = VerifyPeerCertificate(caMap)
		return tlsCopy, nil
	}
}

const invalidCertErrMsg = "access denied: invalid client certificate"

// VerifyPeerCertificate returns a tls.Config.VerifyPeerCertificate callback
// that checks the client peer certificate's claimed cluster name matches the
// cluster name of the CA that issued it, and that the CA type (host vs user)
// matches the cert's role type.
func VerifyPeerCertificate(caMap HostAndUserCAInfo) func([][]byte, [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		if len(verifiedChains) == 0 || len(verifiedChains[0]) == 0 {
			return nil
		}

		peerCert := verifiedChains[0][0]
		identity, err := tlsca.FromSubject(peerCert.Subject, peerCert.NotAfter)
		if err != nil {
			slog.WarnContext(context.TODO(), "Failed to parse identity from client certificate subject", "error", err)
			return trace.Wrap(err)
		}

		certClusterName := identity.TeleportCluster
		issuerClusterName, err := tlsca.ClusterName(peerCert.Issuer)
		if err != nil {
			slog.WarnContext(context.TODO(), "Failed to parse issuer cluster name from client certificate issuer", "error", err)
			return trace.AccessDenied(invalidCertErrMsg)
		}
		if certClusterName != issuerClusterName {
			slog.WarnContext(context.TODO(), "Client peer certificate was issued by a CA from a different cluster than what the certificate claims to be from", "peer_cert_cluster_name", certClusterName, "issuer_cluster_name", issuerClusterName)
			return trace.AccessDenied(invalidCertErrMsg)
		}

		ca, ok := caMap[string(peerCert.RawIssuer)]
		if !ok {
			slog.WarnContext(context.TODO(), "Could not find issuer CA of client certificate")
			return trace.AccessDenied(invalidCertErrMsg)
		}

		systemRole, found := findPrimarySystemRole(identity)
		if found && !ca.IsHostCA {
			slog.WarnContext(context.TODO(), "Client peer certificate has a builtin role but was not issued by a host CA", "role", systemRole.String())
			return trace.AccessDenied(invalidCertErrMsg)
		} else if !found && !ca.IsUserCA {
			slog.WarnContext(context.TODO(), "Client peer certificate has a local role but was not issued by a user CA")
			return trace.AccessDenied(invalidCertErrMsg)
		}

		return nil
	}
}

func findPrimarySystemRole(i *tlsca.Identity) (types.SystemRole, bool) {
	if i.ScopePin.GetKind() == scopesv1.PinKind_PIN_KIND_AGENT {
		role := types.SystemRole(i.ScopePin.GetSystemRoles().GetPrimary())
		if err := role.Check(); err != nil {
			return "", false
		}
		return role, true
	}
	for _, role := range i.Groups {
		systemRole := types.SystemRole(role)
		if err := systemRole.Check(); err == nil {
			return systemRole, true
		}
	}
	return "", false
}
