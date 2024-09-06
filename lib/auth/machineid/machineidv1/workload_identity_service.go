/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package machineidv1

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"math/big"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

const spiffeScheme = "spiffe"

// WorkloadIdentityServiceConfig holds configuration options for
// the WorkloadIdentity gRPC service.
type WorkloadIdentityServiceConfig struct {
	Authorizer authz.Authorizer
	Cache      WorkloadIdentityCacher
	Logger     logrus.FieldLogger
	Emitter    apievents.Emitter
	Reporter   usagereporter.UsageReporter
	Clock      clockwork.Clock
	KeyStore   KeyStorer
}

type WorkloadIdentityCacher interface {
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)
	GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error)
}

type KeyStorer interface {
	GetTLSCertAndSigner(ctx context.Context, ca types.CertAuthority) ([]byte, crypto.Signer, error)
}

// NewWorkloadIdentityService returns a new instance of the
// WorkloadIdentityService.
func NewWorkloadIdentityService(
	cfg WorkloadIdentityServiceConfig,
) (*WorkloadIdentityService, error) {
	switch {
	case cfg.Cache == nil:
		return nil, trace.BadParameter("cache service is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	case cfg.Emitter == nil:
		return nil, trace.BadParameter("emitter is required")
	case cfg.Reporter == nil:
		return nil, trace.BadParameter("reporter is required")
	case cfg.KeyStore == nil:
		return nil, trace.BadParameter("keyStore is required")
	}

	if cfg.Logger == nil {
		cfg.Logger = logrus.WithField(teleport.ComponentKey, "workload-identity.service")
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}

	return &WorkloadIdentityService{
		logger:     cfg.Logger,
		authorizer: cfg.Authorizer,
		cache:      cfg.Cache,
		emitter:    cfg.Emitter,
		reporter:   cfg.Reporter,
		clock:      cfg.Clock,
		keyStorer:  cfg.KeyStore,
	}, nil
}

// WorkloadIdentityService implements the teleport.machineid.v1.WorkloadIdentity
// RPC service.
type WorkloadIdentityService struct {
	pb.UnimplementedWorkloadIdentityServiceServer

	cache      WorkloadIdentityCacher
	authorizer authz.Authorizer
	keyStorer  KeyStorer
	logger     logrus.FieldLogger
	emitter    apievents.Emitter
	reporter   usagereporter.UsageReporter
	clock      clockwork.Clock
}

func signx509SVID(
	notBefore time.Time,
	notAfter time.Time,
	ca *tlsca.CertAuthority,
	publicKeyBytes []byte,
	spiffeID *url.URL,
	dnsSANs []string,
	ipSANS []net.IP,
) (pemBytes []byte, serialNumber *big.Int, err error) {
	pubKey, err := x509.ParsePKIXPublicKey(publicKeyBytes)
	if err != nil {
		return nil, nil, trace.Wrap(
			err, "parsing public key pkix",
		)
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err = rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	template := &x509.Certificate{
		SerialNumber: serialNumber,
		NotBefore:    notBefore,
		NotAfter:     notAfter,
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
		// - An X.509 SVID MAY contain any number of other SAN field types, including DNS SANs.
		URIs:        []*url.URL{spiffeID},
		DNSNames:    dnsSANs,
		IPAddresses: ipSANS,
	}

	// For legacy compatibility, we set the subject common name to the first
	// DNS SAN. This allows interoperability with non-SPIFFE aware clients that
	// trust the CA, or interoperability with servers like Postgres which can
	// only inspect the common name when making authz/authn decisions.
	// Eventually, we may wish to make this behavior more configurable.
	if len(dnsSANs) > 0 {
		template.Subject.CommonName = dnsSANs[0]
	}

	certBytes, err := x509.CreateCertificate(
		rand.Reader, template, ca.Cert, pubKey, ca.Signer,
	)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return certBytes, serialNumber, nil
}

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

func (wis *WorkloadIdentityService) signX509SVID(
	ctx context.Context,
	authCtx *authz.Context,
	req *pb.SVIDRequest,
	clusterName string,
	ca *tlsca.CertAuthority,
) (res *pb.SVIDResponse, err error) {
	// Setup audit log event, we will emit these even on failure to catch any
	// authz denials
	var serialNumber *big.Int
	var spiffeID *url.URL
	defer func() {
		evt := &apievents.SPIFFESVIDIssued{
			Metadata: apievents.Metadata{
				Type: events.SPIFFESVIDIssuedEvent,
				Code: events.SPIFFESVIDIssuedSuccessCode,
			},
			UserMetadata:       authz.ClientUserMetadata(ctx),
			ConnectionMetadata: authz.ConnectionMetadata(ctx),
			Hint:               req.Hint,
			SVIDType:           "x509",
			DNSSANs:            req.DnsSans,
			IPSANs:             req.IpSans,
		}
		if err != nil {
			evt.Code = events.SPIFFESVIDIssuedFailureCode
		}
		if serialNumber != nil {
			evt.SerialNumber = serialString(serialNumber)
		}
		if spiffeID != nil {
			evt.SPIFFEID = spiffeID.String()
		}
		if emitErr := wis.emitter.EmitAuditEvent(ctx, evt); emitErr != nil {
			wis.logger.WithError(emitErr).Warn(
				"Failed to emit SPIFFE SVID issued event.",
			)
		}
	}()

	// Parse and validate parameters
	switch {
	case req.SpiffeIdPath == "":
		return nil, trace.BadParameter("spiffeIdPath: must be non-empty")
	case !strings.HasPrefix(req.SpiffeIdPath, "/"):
		return nil, trace.BadParameter("spiffeIdPath: must start with '/'")
	case len(req.PublicKey) == 0:
		return nil, trace.BadParameter("publicKey: must be non-empty")
	}
	spiffeID = &url.URL{
		Scheme: spiffeScheme,
		Host:   clusterName,
		Path:   req.SpiffeIdPath,
	}
	ipSans := []net.IP{}
	for i, stringIP := range req.IpSans {
		ip := net.ParseIP(stringIP)
		if ip == nil {
			return nil, trace.BadParameter(
				"ipSans[%d]: invalid IP address %q", i, stringIP,
			)
		}
		ipSans = append(ipSans, ip)
	}

	// Default TTL is 1 hour - maximum is 24 hours. If TTL is greater than max,
	// we will use the max.
	ttl := defaults.DefaultRenewableCertTTL
	if reqTTL := req.Ttl.AsDuration(); reqTTL > 0 {
		ttl = reqTTL
	}
	if ttl > defaults.MaxRenewableCertTTL {
		ttl = defaults.MaxRenewableCertTTL
	}
	notAfter := wis.clock.Now().Add(ttl)
	// NotBefore is one minute in the past to prevent "Not yet valid" errors on
	// time skewed clusters.
	notBefore := wis.clock.Now().UTC().Add(-1 * time.Minute)

	// Perform authz checks. They must be allowed to issue the SPIFFE ID and
	// any listed SANs.
	if err := authCtx.Checker.CheckSPIFFESVID(
		req.SpiffeIdPath,
		req.DnsSans,
		ipSans,
	); err != nil {
		return nil, trace.Wrap(err)
	}

	var pemBytes []byte
	pemBytes, serialNumber, err = signx509SVID(
		notBefore, notAfter, ca, req.PublicKey, spiffeID, req.DnsSans, ipSans,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &pb.SVIDResponse{
		SpiffeId:    spiffeID.String(),
		Hint:        req.Hint,
		Certificate: pemBytes,
	}, nil
}

func (wis *WorkloadIdentityService) SignX509SVIDs(ctx context.Context, req *pb.SignX509SVIDsRequest) (*pb.SignX509SVIDsResponse, error) {
	if len(req.Svids) == 0 {
		return nil, trace.BadParameter("svids: must be non-empty")
	}

	authCtx, err := wis.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Fetch info that will be needed for all SPIFFE SVIDs requested
	clusterName, err := wis.cache.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err, "getting cluster name")
	}
	ca, err := wis.cache.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.SPIFFECA,
		DomainName: clusterName.GetClusterName(),
	}, true)
	if err != nil {
		return nil, trace.Wrap(err, "getting SPIFFE CA")
	}
	tlsCert, tlsSigner, err := wis.keyStorer.GetTLSCertAndSigner(ctx, ca)
	if err != nil {
		return nil, trace.Wrap(err, "getting CA cert and key")
	}
	tlsCA, err := tlsca.FromCertAndSigner(tlsCert, tlsSigner)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	res := &pb.SignX509SVIDsResponse{}
	for i, svidReq := range req.Svids {
		svidRes, err := wis.signX509SVID(
			ctx, authCtx, svidReq, clusterName.GetClusterName(), tlsCA,
		)
		if err != nil {
			return nil, trace.Wrap(err, "signing svid %d", i)
		}
		res.Svids = append(res.Svids, svidRes)
	}

	return res, nil
}
