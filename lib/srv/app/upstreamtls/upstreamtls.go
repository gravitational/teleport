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
	"log/slog"
	"net/url"
	"slices"

	"github.com/gravitational/trace"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/utils"
)

// AccessPoint is a caching client connected to the Auth Server.
type AccessPoint interface {
	// GetCertAuthority returns cert authority by id.
	GetCertAuthority(context.Context, types.CertAuthID, bool) (types.CertAuthority, error)
	// GetAuthPreference returns the current cluster auth preference.
	GetAuthPreference(context.Context) (types.AuthPreference, error)
}

// WorkloadIdentityClientGetter used to retrieve Workload identity clients.
type WorkloadIdentityClientGetter interface {
	// WorkloadIdentityIssuanceClient returns an unadorned client for the
	// workload identity service.
	WorkloadIdentityIssuanceClient() workloadidentityv1pb.WorkloadIdentityIssuanceServiceClient
}

// Options are the options used to configure the TLS.
type Options struct {
	// Logger is the slog.Logger used by the configurator.
	Logger *slog.Logger
	// AccessPoint is a caching client connected to the Auth Server.
	AccessPoint AccessPoint
	// WorkloadIdentityClientGetter is the interface used to retrieve Workload
	// identity clients.
	WorkloadIdentityClientGetter WorkloadIdentityClientGetter
	// UserCertificate is the requesting user certificate.
	UserCertificate []byte
	// ClusterName is the current cluster name.
	ClusterName string
	// App is the app being configured.
	App types.Application
	// CipherSuites are the TLS cipher suites.
	CipherSuites []uint16
	// InsecureMode indicates the service is running in insecure mode.
	InsecureMode bool
}

// Configure creates and configures a *tls.Config that will be used for
// mutual authentication. This function assumes a validated AppTLS, meaning it
// won't perform validation checks for fields and their contents.
//
// Note that [types.AppTLS] can be nil since it is not required for supported
// protocols. This function must take that into account.
func Configure(ctx context.Context, opts Options) (*tls.Config, error) {
	// Service-level insecure mode takes precedence over app configuration.
	if opts.InsecureMode {
		return configureTLSInsecure(opts.CipherSuites, nil)
	}

	appTLS := cmp.Or(opts.App.GetTLS(), &types.AppTLS{})

	caPool, err := newTLSCertPool(ctx, opts.Logger, opts.AccessPoint, opts.ClusterName, appTLS.AllowedCas)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	serverName, err := upstreamVerifyName(opts.App, appTLS)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var clientCerts []tls.Certificate
	switch appTLS.ClientCertMode {
	case types.AppClientCertModeManaged:
		var err error
		clientCerts, err = issueClientCertificates(ctx, opts)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	default:
		// Nothing to do, no client cert being used.
	}

	switch opts.App.GetTLSMode() {
	case types.AppTLSModeVerifyFull:
		spiffeID, _ := spiffeid.FromString(appTLS.ServerSpiffeId)
		return configureTLSVerifyFull(opts.CipherSuites, caPool, spiffeID, serverName, clientCerts)
	case types.AppTLSModeVerifySpiffeID:
		spiffeID, _ := spiffeid.FromString(appTLS.ServerSpiffeId)
		return configureTLSSpiffeIDVerify(opts.CipherSuites, caPool, spiffeID, clientCerts)
	case types.AppTLSModeVerifyServerName:
		return configureTLSVerifyServerName(opts.CipherSuites, caPool, serverName, clientCerts)
	case types.AppTLSModeInsecure:
		return configureTLSInsecure(opts.CipherSuites, clientCerts)
	default:
		// Unsupported protocols will return a non-valid TLS mode. For those
		// cases, the TLS configuration won't be effective, so we can return
		// an empty option here.
		return utils.TLSConfig(opts.CipherSuites), nil
	}
}

func configureTLSSpiffeIDVerify(cipherSuites []uint16, caPool *x509.CertPool, spiffeID spiffeid.ID, certs []tls.Certificate) (*tls.Config, error) {
	tlsConfig := utils.TLSConfig(cipherSuites)
	tlsConfig.RootCAs = caPool
	tlsConfig.InsecureSkipVerify = true
	tlsConfig.Certificates = certs
	tlsConfig.ServerName = ""
	// Skips server name verification.
	tlsConfig.VerifyConnection = tlsVerifyPeerCertificate(tlsConfig.RootCAs, spiffeID, "")
	return tlsConfig, nil
}

func configureTLSVerifyFull(cipherSuites []uint16, caPool *x509.CertPool, spiffeID spiffeid.ID, serverName string, certs []tls.Certificate) (*tls.Config, error) {
	tlsConfig := utils.TLSConfig(cipherSuites)
	tlsConfig.RootCAs = caPool
	tlsConfig.InsecureSkipVerify = true
	tlsConfig.Certificates = certs
	tlsConfig.ServerName = serverName
	tlsConfig.VerifyConnection = tlsVerifyPeerCertificate(tlsConfig.RootCAs, spiffeID, serverName)
	return tlsConfig, nil
}

func configureTLSVerifyServerName(cipherSuites []uint16, caPool *x509.CertPool, serverName string, certs []tls.Certificate) (*tls.Config, error) {
	tlsConfig := utils.TLSConfig(cipherSuites)
	tlsConfig.RootCAs = caPool
	tlsConfig.InsecureSkipVerify = true
	tlsConfig.Certificates = certs
	tlsConfig.ServerName = serverName
	// We cannot use regular verify function since it would skip DNSName
	// validation when the value set or the dialed address are an IP.
	//
	// Skips SPIFFE ID verification.
	tlsConfig.VerifyConnection = tlsVerifyPeerCertificate(tlsConfig.RootCAs, spiffeid.ID{}, serverName)
	return tlsConfig, nil
}

func configureTLSInsecure(cipherSuites []uint16, certs []tls.Certificate) (*tls.Config, error) {
	tlsConfig := utils.TLSConfig(cipherSuites)
	tlsConfig.Certificates = certs
	tlsConfig.InsecureSkipVerify = true
	return tlsConfig, nil
}

// tlsVerifyPeerCertificate creates a tls.VerifyConnection function that
// verifies the certificate, SPIFFE ID extension (if not zero), and server name
// when set to non-empty value.
func tlsVerifyPeerCertificate(roots *x509.CertPool, spiffeID spiffeid.ID, serverName string) func(cs tls.ConnectionState) error {
	return func(cs tls.ConnectionState) error {
		opts := x509.VerifyOptions{
			Roots:   roots,
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
		if !spiffeID.IsZero() {
			// Returns error if the certificate doesn't contain URI SAN (SPIFFE ID).
			upstreamID, err := spiffetls.PeerIDFromConnectionState(cs)
			if err != nil {
				return trace.Wrap(err)
			}
			if upstreamID != spiffeID {
				return trace.BadParameter("spiffe id mismatch. expected %q but got %q", spiffeID, upstreamID)
			}
		}
		return nil
	}
}

// upstreamVerifyName returns the DNSName value that will be used on verify.
func upstreamVerifyName(app types.Application, appTLS *types.AppTLS) (string, error) {
	if appTLS != nil && appTLS.ServerName != "" {
		return appTLS.ServerName, nil
	}

	u, err := url.Parse(app.GetURI())
	if err != nil {
		return "", trace.Wrap(err)
	}
	return u.Hostname(), nil // strips port and IPv6 brackets
}

// newTLSCertPool creates a new x509 cert pool using the list of allowed CAs.
func newTLSCertPool(ctx context.Context, logger *slog.Logger, getter AccessPoint, clusterName string, cas []string) (*x509.CertPool, error) {
	// If no options are provided, use the host's root CA (default behavior).
	// This is mainly to keep backwards compatibility for apps using TLS
	// connections, and that doesn't configure the CA list.
	if len(cas) == 0 {
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

			if certs == nil {
				logger.WarnContext(ctx, "CA alias contains non active keys, it won't be effective", "alias", ca)
				continue
			}

			for _, cert := range certs {
				caPool.AddCert(cert)
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
func loadCACertificates(ctx context.Context, getter AccessPoint, clusterName string, alias types.AppTLSInternalCA) ([]*x509.Certificate, error) {
	var caType types.CertAuthType
	switch alias {
	case types.AppTLSInternalCAWorkloadIdentity:
		caType = types.SPIFFECA
	default:
		// This should be unreachable. If it happens there is probably a
		// mismatch between this switch and the
		// `types.AppSupportedAllowedInternalCAs` slice.
		return nil, trace.BadParameter("unsupported CA %q", alias)
	}

	ca, err := getter.GetCertAuthority(ctx, types.CertAuthID{
		Type:       caType,
		DomainName: clusterName,
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	keyPairs := ca.GetTrustedTLSKeyPairs()
	if len(keyPairs) == 0 {
		return nil, nil
	}

	certs := make([]*x509.Certificate, 0, len(keyPairs))
	for _, keyPair := range keyPairs {
		cert, err := tlsutils.ParseCertificatePEM(keyPair.Cert)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		certs = append(certs, cert)
	}

	return certs, nil
}

func issueClientCertificates(ctx context.Context, opts Options) ([]tls.Certificate, error) {
	privateKey, err := cryptosuites.GenerateKey(ctx,
		cryptosuites.GetCurrentSuiteFromAuthPreference(opts.AccessPoint),
		cryptosuites.AppClientCATLS)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pubBytes, err := x509.MarshalPKIXPublicKey(privateKey.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt := opts.WorkloadIdentityClientGetter.WorkloadIdentityIssuanceClient()
	resp, err := clt.IssueTeleportWorkloadIdentity(ctx, &workloadidentityv1pb.IssueTeleportWorkloadIdentityRequest{
		Credential: &workloadidentityv1pb.IssueTeleportWorkloadIdentityRequest_X509SvidParams{
			X509SvidParams: &workloadidentityv1pb.X509SVIDParams{
				PublicKey: pubBytes,
			},
		},
		Usage: &workloadidentityv1pb.IssueTeleportWorkloadIdentityRequest_AppAccess{
			AppAccess: &workloadidentityv1pb.AppAccessUsage{
				UserCertificate: opts.UserCertificate,
			},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	svid := resp.GetCredential().GetX509Svid()
	return []tls.Certificate{
		{
			Certificate: append([][]byte{svid.GetCert()}, svid.Chain...),
			PrivateKey:  privateKey,
		},
	}, nil
}
