// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package upstreamtls

import (
	"cmp"
	"context"
	"crypto/tls"
	"crypto/x509"
	"iter"
	"log/slog"
	"net/url"
	"slices"

	"github.com/gravitational/trace"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// CertificateAuthorityGetter used to retrieve CA public information.
type CertificateAuthorityGetter interface {
	// GetCertAuthority returns cert authority by id.
	GetCertAuthority(context.Context, types.CertAuthID, bool) (types.CertAuthority, error)
}

// Options are the options used to configure the TLS.
type Options struct {
	// Logger is the slog.Logger used by the configurator.
	Logger *slog.Logger
	// CAGetter is the interface used to retrieve certificate authorities.
	CAGetter CertificateAuthorityGetter
	// ClusterName is the current cluster name.
	ClusterName string
	// App is the app being configured.
	App types.Application
	// CipherSuites are the TLS cipher suites.
	CipherSuites []uint16
	// InsecureMode indicates the service is running in insecure mode.
	InsecureMode bool
}

// CheckAndSetDefaults validates required fields and sets defaults for optional
// ones.
func (o *Options) CheckAndSetDefaults() error {
	if o.CAGetter == nil {
		return trace.BadParameter("CAGetter is required")
	}
	if o.ClusterName == "" {
		return trace.BadParameter("ClusterName is required")
	}
	if o.App == nil {
		return trace.BadParameter("App is required")
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	return nil
}

// Configure creates and configures a *tls.Config that will be used to verify
// upstream TLS servers. This function assumes a validated AppTLS, meaning it
// won't perform validation checks for fields and their contents.
//
// Note that [types.AppTLS] can be nil since it is not required for supported
// protocols. This function must take that into account.
func Configure(ctx context.Context, opts Options) (*tls.Config, error) {
	if err := opts.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	// Service-level insecure mode takes precedence over app configuration.
	if opts.InsecureMode {
		return configureTLSInsecure(opts.CipherSuites)
	}

	appTLS := cmp.Or(opts.App.GetTLS(), &types.AppTLS{})
	logger := opts.Logger.With("app", opts.App.GetName())

	caPool, err := newTLSCertPool(ctx, logger, opts.CAGetter, opts.ClusterName, appTLS.AllowedCas)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch opts.App.GetTLSMode() {
	case types.AppTLSModeVerifyFull:
		spiffeID, err := spiffeid.FromString(appTLS.ServerSpiffeId)
		if err != nil {
			return nil, trace.BadParameter("app contains an invalid tls.server_spiffe_id field: %v", err)
		}
		return configureTLSVerifyFull(opts.CipherSuites, caPool, opts.App.GetURI(), spiffeID, appTLS.ServerName)
	case types.AppTLSModeVerifySpiffeID:
		spiffeID, err := spiffeid.FromString(appTLS.ServerSpiffeId)
		if err != nil {
			return nil, trace.BadParameter("app contains an invalid tls.server_spiffe_id field: %v", err)
		}
		return configureTLSSpiffeIDVerify(opts.CipherSuites, caPool, spiffeID)
	case types.AppTLSModeVerifyServerName:
		return configureTLSVerifyServerName(opts.CipherSuites, caPool, appTLS.ServerName)
	case types.AppTLSModeInsecure:
		return configureTLSInsecure(opts.CipherSuites)
	case "":
		// Empty mode is returned by GetTLSMode() for app protocols that don't
		// support TLS configuration. Return baseline tls.Config so callers
		// still get configured cipher suites.
		return utils.TLSConfig(opts.CipherSuites), nil
	default:
		// This should be unreachable. In case it does, it is probably caused by
		// a new TLS mode being added to the config but not here. Ensure this
		// switch-case handles all valid options.
		return nil, trace.BadParameter("unsupported TLS mode %q", opts.App.GetTLSMode())
	}
}

func configureTLSSpiffeIDVerify(cipherSuites []uint16, caPool *x509.CertPool, spiffeID spiffeid.ID) (*tls.Config, error) {
	tlsConfig := utils.TLSConfig(cipherSuites)
	tlsConfig.RootCAs = caPool
	tlsConfig.InsecureSkipVerify = true
	tlsConfig.ServerName = ""
	// Skips server name verification.
	tlsConfig.VerifyConnection = tlsVerifyPeerCertificateWithSPIFFE(tlsConfig.RootCAs, spiffeID, "")
	return tlsConfig, nil
}

func configureTLSVerifyFull(cipherSuites []uint16, caPool *x509.CertPool, appURI string, spiffeID spiffeid.ID, serverName string) (*tls.Config, error) {
	tlsConfig := utils.TLSConfig(cipherSuites)
	tlsConfig.RootCAs = caPool
	tlsConfig.InsecureSkipVerify = true

	// Since we're providing a custom VerifyConnection we must ensure that
	// server name is not empty, otherwise it would just skip the server name
	// verification.
	//
	// To do this we parse the app URI and lookup for the hostname.
	if serverName == "" {
		u, err := url.Parse(appURI)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		serverName = u.Hostname()
		if serverName == "" {
			return nil, trace.BadParameter("cannot derive server name from app URI %q", appURI)
		}
	}

	tlsConfig.ServerName = serverName
	tlsConfig.VerifyConnection = tlsVerifyPeerCertificateWithSPIFFE(tlsConfig.RootCAs, spiffeID, serverName)
	return tlsConfig, nil
}

// configureTLSVerifyServerName setups a TLS config with standard hostname
// certificate match verification.
func configureTLSVerifyServerName(cipherSuites []uint16, caPool *x509.CertPool, serverName string) (*tls.Config, error) {
	tlsConfig := utils.TLSConfig(cipherSuites)
	tlsConfig.RootCAs = caPool
	// In case the property is empty (default value), the standard library
	// will pickup the appropriate hostname value.
	tlsConfig.ServerName = serverName
	return tlsConfig, nil
}

func configureTLSInsecure(cipherSuites []uint16) (*tls.Config, error) {
	tlsConfig := utils.TLSConfig(cipherSuites)
	tlsConfig.InsecureSkipVerify = true
	return tlsConfig, nil
}

// tlsVerifyPeerCertificateWithSPIFFE creates a tls.VerifyConnection function
// that verifies the certificate chain, SPIFFE ID extension, and server name.
func tlsVerifyPeerCertificateWithSPIFFE(roots *x509.CertPool, spiffeID spiffeid.ID, serverName string) func(cs tls.ConnectionState) error {
	return func(cs tls.ConnectionState) error {
		opts := x509.VerifyOptions{
			Roots: roots,
			// When `serverName` is empty, it means no DNSName verification will
			// be done.
			DNSName: serverName,

			Intermediates: nil,
		}
		if len(cs.PeerCertificates) == 0 {
			return trace.BadParameter("no peer certificates presented")
		}
		if len(cs.PeerCertificates) > 1 {
			opts.Intermediates = x509.NewCertPool()
			for _, cert := range cs.PeerCertificates[1:] {
				opts.Intermediates.AddCert(cert)
			}
		}
		if _, err := cs.PeerCertificates[0].Verify(opts); err != nil {
			return trace.Wrap(err)
		}
		// Returns error if the certificate doesn't contain URI SAN (SPIFFE ID).
		upstreamID, err := spiffetls.PeerIDFromConnectionState(cs)
		if err != nil {
			return trace.Wrap(err)
		}
		if upstreamID != spiffeID {
			return trace.BadParameter("spiffe id mismatch. expected %q but got %q", spiffeID, upstreamID)
		}
		return nil
	}
}

// newTLSCertPool creates a new x509 cert pool using the list of allowed CAs.
func newTLSCertPool(ctx context.Context, logger *slog.Logger, getter CertificateAuthorityGetter, clusterName string, cas []string) (*x509.CertPool, error) {
	// If no options are provided, use the host's root CA (default behavior).
	// This is mainly to keep backwards compatibility for apps using TLS
	// connections, and that doesn't configure the CA list.
	if len(cas) == 0 {
		logger.DebugContext(ctx, "using system trust store")
		return nil, nil
	}

	caPool := x509.NewCertPool()
	for _, ca := range cas {
		switch {
		case slices.Contains(types.AppSupportedInternalCAs(), ca):
			certs, err := loadCACertificates(ctx, getter, clusterName, ca)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			var hasAny bool
			for cert, err := range certs {
				if err != nil {
					return nil, trace.Wrap(err)
				}

				hasAny = true
				caPool.AddCert(cert)
			}

			if !hasAny {
				logger.WarnContext(ctx, "CA alias contains non active keys, it won't be effective", "alias", ca)
			}
		default:
			caCert, err := tlsutils.ParseCertificatePEMStrict([]byte(ca))
			if err != nil {
				return nil, trace.Wrap(err)
			}

			caPool.AddCert(caCert)
		}
	}

	return caPool, nil
}

// loadCACertificates takes a "CA alias" and resolve to Teleport CA certificates.
func loadCACertificates(ctx context.Context, getter CertificateAuthorityGetter, clusterName string, alias types.AppTLSInternalCA) (iter.Seq2[*x509.Certificate, error], error) {
	var caType types.CertAuthType
	switch alias {
	case types.AppTLSInternalCAWorkloadIdentity:
		caType = types.SPIFFECA
	default:
		// This should be unreachable. If it happens there is probably a
		// mismatch between this switch and the
		// `types.AppSupportedInternalCAs` function.
		return nil, trace.BadParameter("unsupported CA %q", alias)
	}

	ca, err := getter.GetCertAuthority(ctx, types.CertAuthID{
		Type:       caType,
		DomainName: clusterName,
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return services.GetX509Certs(ca), nil
}
