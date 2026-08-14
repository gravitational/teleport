package compute

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"log/slog"
	"math/big"
	"net/url"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffegrpc/grpccredentials"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

const (
	// CertificateTTL is the Time-to-Live of the generated X509-SVIDs.
	//
	// This is intentionally very short because the certificate only needs to be
	// valid at handshake-time.
	CertificateTTL = 5 * time.Minute

	// ClockSkewAllowance is the amount of leeway added to the certificate's
	// NotBefore to allow for clock drift.
	ClockSkewAllowance = 1 * time.Minute
)

// TransportCredentials returns a set of gRPC transport credentials that can
// be used to authenticate the connection to the beam compute orchestration
// service via mTLS and SPIFFE.
//
// The auth server will issue itself a X509-SVID client certificate with the
// SPIFFE ID: `spiffe://<cluster>/_teleport-cloud/beams/auth-server` which will
// be trusted by the orchestration service.
//
// Cloud will deploy tbot next to the orchestration service, serving the SPIFFE
// workload-identity-api service. It will be issued an X509-SVID certificate with
// the SPIFFE ID: `spiffe://<cluster>/_teleport-cloud/beams/orchestrator`, which
// the TransportCredentials returned by this method will trust.
//
// Once RFD 0251 has been implemented, the client certificate will be issued by
// the "internal" trust domain instead. This will prevent users from creating
// their own WorkloadIdentity resources with our client's SPIFFE ID, which isn't
// currently a huge issue as the orchestrator is per-tenant, but could allow them
// to bypass compute quota enforcement, etc. in the future.
//
// Note: `workload_identity_x509_issuer_override`s are not supported. From an
// issuing point-of-view, this is fine because we're the only consumer of these
// client certificates (so self-signing is sufficient). From a validation point-
// of-view it presents a challenge because the server certificate may not chain
// up to a root we trust. We should consider allowing users to disable overrides
// on the tbot-side, and doing so for the orchestrator.
func TransportCredentials(ctx context.Context, cfg TransportCredentialsConfig) (credentials.TransportCredentials, error) {
	if cfg.Insecure {
		return insecure.NewCredentials(), nil
	}

	switch {
	case cfg.ClusterName == "":
		return nil, trace.BadParameter("ClusterName is required")
	case cfg.AuthPreferenceGetter == nil:
		return nil, trace.BadParameter("AuthPreferenceGetter is required")
	case cfg.CertAuthorityGetter == nil:
		return nil, trace.BadParameter("CertAuthorityGetter is required")
	case cfg.Keystore == nil:
		return nil, trace.BadParameter("Keystore is required")
	case cfg.Emitter == nil:
		return nil, trace.BadParameter("Emitter is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	trustDomain, err := spiffeid.TrustDomainFromString(cfg.ClusterName)
	if err != nil {
		return nil, trace.Wrap(err, "parsing cluster name as trust domain")
	}

	serverID, err := spiffeid.FromPath(trustDomain, "/_teleport-cloud/beams/orchestrator")
	if err != nil {
		return nil, trace.Wrap(err, "constructing server SPIFFE ID")
	}

	clientID, err := spiffeid.FromPath(trustDomain, "/_teleport-cloud/beams/auth-server")
	if err != nil {
		return nil, trace.Wrap(err, "constructing client SPIFFE ID")
	}

	src := &source{
		ctx:                  ctx,
		trustDomain:          trustDomain,
		clientID:             clientID,
		authPreferenceGetter: cfg.AuthPreferenceGetter,
		certAuthorityGetter:  cfg.CertAuthorityGetter,
		keystore:             cfg.Keystore,
		logger:               cfg.Logger,
		emitter:              cfg.Emitter,
	}

	return grpccredentials.MTLSClientCredentials(src, src, tlsconfig.AuthorizeID(serverID)), nil
}

// TransportCredentialsConfig configures the gRPC transport credentials.
type TransportCredentialsConfig struct {
	// Insecure means the connection will be entirely cleartext without any
	// form of authentication.
	Insecure bool

	// ClusterName is the Teleport cluster name, used to determine the SPIFFE
	// trust domain.
	ClusterName string

	// AuthPreferenceGetter is used to read the cluster's current auth preference.
	AuthPreferenceGetter cryptosuites.AuthPreferenceGetter

	// CertAuthorityGetter is used to read the SPIFFE CA details.
	CertAuthorityGetter CertAuthorityGetter

	// Keystore is used to read the SPIFFE CA's private key, for signing client
	// certificates.
	Keystore Keystore

	// Logger to which errors and other messages are written.
	Logger *slog.Logger

	// Emitter used to log audit events.
	Emitter apievents.Emitter
}

// CertAuthorityGetter is used to read the certificate authority.
type CertAuthorityGetter interface {
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)
}

// Keystore is used to obtain the certificate authority's private key.
type Keystore interface {
	GetTLSCertAndSigner(ctx context.Context, ca types.CertAuthority) ([]byte, crypto.Signer, error)
}

// source implements the x509svid.Source and x509bundle.Source interfaces.
type source struct {
	// Unfortunately, we have to store the context on this struct because the
	// x509svid.Source and x509bundle.Source methods aren't context-aware.
	ctx context.Context

	trustDomain          spiffeid.TrustDomain
	clientID             spiffeid.ID
	authPreferenceGetter cryptosuites.AuthPreferenceGetter
	certAuthorityGetter  CertAuthorityGetter
	keystore             Keystore
	logger               *slog.Logger
	emitter              apievents.Emitter
}

func (src *source) GetX509BundleForTrustDomain(trustDomain spiffeid.TrustDomain) (*x509bundle.Bundle, error) {
	if src.trustDomain.String() != trustDomain.String() {
		return nil, trace.NotFound("unknown trust domain: %s", trustDomain)
	}
	return src.getX509BundleForTrustDomain(src.ctx)
}

func (src *source) getX509BundleForTrustDomain(ctx context.Context) (*x509bundle.Bundle, error) {
	ca, err := src.certAuthorityGetter.GetCertAuthority(
		ctx,
		types.CertAuthID{
			Type:       types.SPIFFECA,
			DomainName: src.trustDomain.Name(),
		},
		false, /* loadKeys */
	)
	if err != nil {
		return nil, trace.Wrap(err, "getting SPIFFE CA")
	}

	bundle := x509bundle.New(src.trustDomain)
	for _, certPEM := range services.GetTLSCerts(ca) {
		block, _ := pem.Decode(certPEM)
		if block == nil || block.Type != "CERTIFICATE" {
			return nil, trace.BadParameter("parsing SPIFFE CA certificate PEM")
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, trace.Wrap(err, "parsing SPIFFE CA certificate")
		}
		bundle.AddX509Authority(cert)
	}
	return bundle, nil
}

func (src *source) GetX509SVID() (*x509svid.SVID, error) {
	svid, err := src.getX509SVID(src.ctx)
	if err != nil {
		src.logger.ErrorContext(src.ctx, "Failed to self-sign X509-SVID for connecting to the beams compute service", "error", err)
		return nil, trace.Wrap(err)
	}
	src.logger.DebugContext(src.ctx, "Self-signed X509-SVID for connecting to the beams compute service", "spiffe_id", svid.ID)
	return svid, nil
}

func (src *source) getX509SVID(ctx context.Context) (*x509svid.SVID, error) {
	key, err := cryptosuites.GenerateKey(
		ctx,
		cryptosuites.GetCurrentSuiteFromAuthPreference(src.authPreferenceGetter),
		cryptosuites.BotSVID,
	)
	if err != nil {
		return nil, trace.Wrap(err, "generating SVID keypair")
	}

	serialNumber, err := rand.Int(
		rand.Reader,
		new(big.Int).Lsh(big.NewInt(1), 128),
	)
	if err != nil {
		return nil, trace.Wrap(err, "generating certificate serial number")
	}

	ca, err := src.getSigningCA(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "getting signing CA")
	}

	certBytes, err := x509.CreateCertificate(
		rand.Reader,
		&x509.Certificate{
			SerialNumber: serialNumber,
			NotBefore:    time.Now().Add(-ClockSkewAllowance),
			NotAfter:     time.Now().Add(CertificateTTL),
			// SPEC(X509-SVID) 4.3. Key Usage:
			// - Leaf SVIDs MUST NOT set keyCertSign or cRLSign.
			// - Leaf SVIDs MUST set digitalSignature
			// - They MAY set keyEncipherment and/or keyAgreement;
			KeyUsage: x509.KeyUsageDigitalSignature |
				x509.KeyUsageKeyEncipherment |
				x509.KeyUsageKeyAgreement,
			// SPEC(X509-SVID) 4.4. Extended Key Usage:
			// - Leaf SVIDs SHOULD include this extension, and it MAY be marked as critical.
			// - When included, fields id-kp-serverAuth and id-kp-clientAuth MUST be set.
			ExtKeyUsage: []x509.ExtKeyUsage{
				x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth,
			},
			// SPEC(X509-SVID) 4.1. Basic Constraints:
			// - leaf certificates MUST set the cA field to false
			BasicConstraintsValid: true,
			IsCA:                  false,
			// SPEC(X509-SVID) 2. SPIFFE ID:
			// - The corresponding SPIFFE ID is set as a URI type in the Subject Alternative Name extension
			// - An X.509 SVID MUST contain exactly one URI SAN, and by extension, exactly one SPIFFE ID.
			URIs: []*url.URL{src.clientID.URL()},
		},
		ca.Cert,
		key.Public(),
		ca.Signer,
	)
	if err != nil {
		return nil, trace.Wrap(err, "generating certificate")
	}

	keyPEM, err := keys.MarshalPrivateKey(key)
	if err != nil {
		return nil, trace.Wrap(err, "marshaling private key")
	}
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})
	svid, err := x509svid.Parse(certPEM, keyPEM)
	if err != nil {
		return nil, trace.Wrap(err, "parsing certificate and key to SVID")
	}

	if err := src.emitter.EmitAuditEvent(ctx, &apievents.SPIFFESVIDIssued{
		Metadata: apievents.Metadata{
			Type: events.SPIFFESVIDIssuedEvent,
			Code: events.SPIFFESVIDIssuedSuccessCode,
		},
		UserMetadata: apievents.UserMetadata{
			User: teleport.UserSystem,
		},
		SVIDType:     "x509",
		SerialNumber: serialString(serialNumber),
		SPIFFEID:     svid.ID.String(),
	}); err != nil {
		src.logger.ErrorContext(ctx, "Failed to emit audit event", "error", err)
	}

	return svid, nil
}

func (src *source) getSigningCA(ctx context.Context) (*tlsca.CertAuthority, error) {
	ca, err := src.certAuthorityGetter.GetCertAuthority(
		ctx,
		types.CertAuthID{
			Type:       types.SPIFFECA,
			DomainName: src.trustDomain.Name(),
		},
		true, /* loadKeys */
	)
	if err != nil {
		return nil, trace.Wrap(err, "getting SPIFFE CA")
	}

	cert, signer, err := src.keystore.GetTLSCertAndSigner(ctx, ca)
	if err != nil {
		return nil, trace.Wrap(err, "getting SPIFFE CA certificate and signer")
	}
	return tlsca.FromCertAndSigner(cert, signer)
}

// TODO(boxofrad): An equivalent of this function also exists in the machineidv1,
// workloadidentityv1, and tpm packages - we should combine them into a shared
// function.
func serialString(serial *big.Int) string {
	hex := serial.Text(16)
	if len(hex)%2 == 1 {
		hex = "0" + hex
	}

	out := strings.Builder{}
	for i := 0; i < len(hex); i += 2 {
		if i != 0 {
			out.WriteString(":")
		}
		out.WriteString(hex[i : i+2])
	}
	return out.String()
}

var (
	_ x509svid.Source   = (*source)(nil)
	_ x509bundle.Source = (*source)(nil)
)
