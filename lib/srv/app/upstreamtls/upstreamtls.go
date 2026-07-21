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
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls"
	"google.golang.org/protobuf/types/known/durationpb"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/utils"
)

// AccessPoint is used to retrieve CA public information and the cluster auth
// preference.
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
	// ClusterName is the current cluster name.
	ClusterName string
	// App is the app being configured.
	App types.Application
	// CipherSuites are the TLS cipher suites.
	CipherSuites []uint16
	// InsecureMode indicates the service is running in insecure mode.
	InsecureMode bool
	// Clock is used to control time.
	Clock clockwork.Clock
	// WorkloadIdentityClientGetter is the interface used to retrieve Workload
	// identity clients.
	WorkloadIdentityClientGetter WorkloadIdentityClientGetter
	// GetUserCertFunc is the function used to retrieve user certificate.
	GetUserCertFunc func() ([]byte, error)
}

// CheckAndSetDefaults validates required fields and sets defaults for optional
// ones.
func (o *Options) CheckAndSetDefaults() error {
	if o.AccessPoint == nil {
		return trace.BadParameter("AccessPoint is required")
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
	if o.Clock == nil {
		o.Clock = clockwork.NewRealClock()
	}
	if o.WorkloadIdentityClientGetter == nil {
		return trace.BadParameter("WorkloadIdentityClientGetter is required")
	}
	if o.GetUserCertFunc == nil {
		return trace.BadParameter("GetUserCertFunc is required")
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

	appTLS := cmp.Or(opts.App.GetTLS(), &types.AppTLS{})
	log := opts.Logger.With("app", opts.App.GetName())
	getCertsFunc := newGetClientCertFunc(log, opts, appTLS)

	// Service-level insecure mode takes precedence over app configuration.
	if opts.InsecureMode {
		return configureTLSInsecure(opts.CipherSuites, getCertsFunc)
	}

	caPool, err := newTLSCertPool(ctx, log, opts.AccessPoint, opts.ClusterName, appTLS.AllowedCas)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch opts.App.GetTLSMode() {
	case types.AppTLSModeVerifyFull:
		spiffeID, err := spiffeid.FromString(appTLS.ServerSpiffeId)
		if err != nil {
			return nil, trace.BadParameter("app contains an invalid tls.server_spiffe_id field: %v", err)
		}
		return configureTLSVerifyFull(opts.CipherSuites, caPool, opts.App.GetURI(), spiffeID, appTLS.ServerName, getCertsFunc)
	case types.AppTLSModeVerifySpiffeID:
		spiffeID, err := spiffeid.FromString(appTLS.ServerSpiffeId)
		if err != nil {
			return nil, trace.BadParameter("app contains an invalid tls.server_spiffe_id field: %v", err)
		}
		return configureTLSSpiffeIDVerify(opts.CipherSuites, caPool, spiffeID, getCertsFunc)
	case types.AppTLSModeVerifyServerName:
		return configureTLSVerifyServerName(opts.CipherSuites, caPool, appTLS.ServerName, getCertsFunc)
	case types.AppTLSModeInsecure:
		return configureTLSInsecure(opts.CipherSuites, getCertsFunc)
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

func configureTLSSpiffeIDVerify(cipherSuites []uint16, caPool *x509.CertPool, spiffeID spiffeid.ID, getCertsFunc getClientCertFunc) (*tls.Config, error) {
	tlsConfig := utils.TLSConfig(cipherSuites)
	tlsConfig.RootCAs = caPool
	tlsConfig.InsecureSkipVerify = true
	tlsConfig.ServerName = ""
	tlsConfig.GetClientCertificate = getCertsFunc
	// Skips server name verification.
	tlsConfig.VerifyConnection = tlsVerifyPeerCertificateWithSPIFFE(tlsConfig.RootCAs, spiffeID, "")
	return tlsConfig, nil
}

func configureTLSVerifyFull(cipherSuites []uint16, caPool *x509.CertPool, appURI string, spiffeID spiffeid.ID, serverName string, getCertsFunc getClientCertFunc) (*tls.Config, error) {
	tlsConfig := utils.TLSConfig(cipherSuites)
	tlsConfig.RootCAs = caPool
	tlsConfig.InsecureSkipVerify = true
	tlsConfig.GetClientCertificate = getCertsFunc

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
func configureTLSVerifyServerName(cipherSuites []uint16, caPool *x509.CertPool, serverName string, getCertsFunc getClientCertFunc) (*tls.Config, error) {
	tlsConfig := utils.TLSConfig(cipherSuites)
	tlsConfig.RootCAs = caPool
	tlsConfig.GetClientCertificate = getCertsFunc
	// In case the property is empty (default value), the standard library
	// will pickup the appropriate hostname value.
	tlsConfig.ServerName = serverName
	return tlsConfig, nil
}

func configureTLSInsecure(cipherSuites []uint16, getCertsFunc getClientCertFunc) (*tls.Config, error) {
	tlsConfig := utils.TLSConfig(cipherSuites)
	tlsConfig.InsecureSkipVerify = true
	tlsConfig.GetClientCertificate = getCertsFunc
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
func newTLSCertPool(ctx context.Context, logger *slog.Logger, getter AccessPoint, clusterName string, cas []string) (*x509.CertPool, error) {
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
func loadCACertificates(ctx context.Context, getter AccessPoint, clusterName string, alias types.AppTLSInternalCA) (iter.Seq2[*x509.Certificate, error], error) {
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

// getClientCertFunc function compatible with [tls.Config.GetClientCertificate].
type getClientCertFunc func(*tls.CertificateRequestInfo) (*tls.Certificate, error)

// clientCertExpiryMargin is a small safety margin applied when deciding whether
// a cached managed client certificate is still usable. It only needs to cover
// clock skew between us and the upstream (the certificate is presented
// immediately during the handshake), not act as a proactive renewal window.
const clientCertExpiryMargin = time.Minute

func newGetClientCertFunc(
	log *slog.Logger,
	opts Options,
	appTLS *types.AppTLS,
) getClientCertFunc {
	if appTLS.ClientCertMode != types.AppClientCertModeManaged {
		// We're ok returning nil function here since [tls.Config] checks before
		// calling it, and a nil function would mean no client certificates will
		// be used.
		return nil
	}

	// Managed upstream client certificates are issued lazily during TLS
	// handshakes and cached until they (effectively) expire.
	//
	// Serialize refreshes so concurrent handshakes on the same transport share
	// one issuance request instead of stampeding the auth service.
	//
	// This is helpful for apps that use a TCP connection pool
	// (basically apps served using HTTP client). For such apps, we reuse the
	// certificate for new connections (when they are still valid).
	//
	// This also prevents services that close connections shortly to trigger a
	// large amount of certificates generation, acting as a "certificate
	// generation rate limiter".
	var (
		mu     sync.Mutex
		cert   *tls.Certificate
		expiry time.Time
	)

	return func(cri *tls.CertificateRequestInfo) (*tls.Certificate, error) {
		ctx := cri.Context()
		mu.Lock()
		defer mu.Unlock()
		if cert == nil || opts.Clock.Now().Add(clientCertExpiryMargin).After(expiry) {
			log.DebugContext(ctx, "issuing client certificate")
			// Use short-lived context from the handshake. So if the handshake
			// is canceled the RPC is also canceled.
			//
			// Since we're holding the lock during the RPC call, this also helps
			// unblocking other handshake calls.
			newCert, notAfter, err := issueClientCertificate(ctx, opts)
			if err != nil {
				log.ErrorContext(ctx, "failed to issue certificates", "error", err)
				// Do not fall back to the cached cert inside the expiry margin.
				// Issuance failures may mean the app session is no longer valid,
				// so failing closed is preferred over using unusable (expired
				// with threshold) cached cert.
				return nil, trace.Wrap(err)
			}
			cert, expiry = newCert, notAfter
		}

		// Certificate selection follows the same logic as standard library.
		if err := cri.SupportsCertificate(cert); err != nil {
			log.WarnContext(ctx, "server doesn't support generated certificate", "error", err)
			// No acceptable certificate found. Don't send a certificate.
			return new(tls.Certificate), nil
		}

		return cert, nil
	}
}

func issueClientCertificate(ctx context.Context, opts Options) (*tls.Certificate, time.Time, error) {
	userCert, err := opts.GetUserCertFunc()
	if err != nil {
		return nil, time.Time{}, trace.Wrap(err, "unable to retrieve user certificate")
	}

	privateKey, err := cryptosuites.GenerateKey(ctx,
		cryptosuites.GetCurrentSuiteFromAuthPreference(opts.AccessPoint),
		cryptosuites.AppClientCATLS)
	if err != nil {
		return nil, time.Time{}, trace.Wrap(err)
	}
	pubBytes, err := x509.MarshalPKIXPublicKey(privateKey.Public())
	if err != nil {
		return nil, time.Time{}, trace.Wrap(err)
	}

	clt := opts.WorkloadIdentityClientGetter.WorkloadIdentityIssuanceClient()
	resp, err := clt.IssueTeleportWorkloadIdentity(ctx, workloadidentityv1pb.IssueTeleportWorkloadIdentityRequest_builder{
		X509SvidParams: workloadidentityv1pb.X509SVIDParams_builder{
			PublicKey: pubBytes,
		}.Build(),
		RequestedTtl: durationpb.New(common.MaxSessionChunkDuration),
		AppAccess: workloadidentityv1pb.AppAccessUsage_builder{
			UserCertificate: userCert,
		}.Build(),
	}.Build())
	if err != nil {
		return nil, time.Time{}, trace.Wrap(err)
	}

	cred := resp.GetCredential()
	svid := cred.GetX509Svid()
	if len(svid.GetCert()) == 0 {
		return nil, time.Time{}, trace.BadParameter("workload identity issuance returned no X.509 SVID certificate")
	}
	// This is an unexpected behavior, and we're just being defensive and avoiding
	// having a branch where the certificate cache never expires because the
	// issuer didn't return an expiration time.
	if cred.GetExpiresAt() == nil {
		return nil, time.Time{}, trace.BadParameter("workload identity issuance returned no credential expiration time")
	}
	return &tls.Certificate{
		Certificate: append([][]byte{svid.GetCert()}, svid.GetChain()...),
		PrivateKey:  privateKey,
	}, cred.GetExpiresAt().AsTime(), nil
}
