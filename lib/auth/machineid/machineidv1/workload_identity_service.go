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
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/gogo/protobuf/jsonpb"
	gogotypes "github.com/gogo/protobuf/types"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"google.golang.org/protobuf/encoding/protojson"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	v1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/trait/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/oidc"
	"github.com/gravitational/teleport/lib/utils/typical"
)

const (
	spiffeScheme       = "spiffe"
	jtiLength          = 16
	maxSVIDTTL         = 24 * time.Hour
	defaultX509SVIDTTL = 1 * time.Hour
	defaultJWTSVIDTTL  = 5 * time.Minute
)

// WorkloadIdentityServiceConfig holds configuration options for
// the WorkloadIdentity gRPC service.
type WorkloadIdentityServiceConfig struct {
	Authorizer authz.Authorizer
	Cache      WorkloadIdentityCacher
	Logger     *slog.Logger
	Emitter    apievents.Emitter
	Reporter   usagereporter.UsageReporter
	Clock      clockwork.Clock
	KeyStore   KeyStorer
}

// WorkloadIdentityCacher is an interface that provides methods to retrieve
// cached information that is necessary for the workload identity service to
// function.
type WorkloadIdentityCacher interface {
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)
	GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error)
	GetProxies() ([]types.Server, error)
}

// KeyStorer is an interface that provides methods to retrieve keys and
// certificates from the backend.
type KeyStorer interface {
	GetTLSCertAndSigner(ctx context.Context, ca types.CertAuthority) ([]byte, crypto.Signer, error)
	GetJWTSigner(ctx context.Context, ca types.CertAuthority) (crypto.Signer, error)
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
	case cfg.Logger == nil:
		return nil, trace.BadParameter("logger is required")
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
	logger     *slog.Logger
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
	var spiffeID spiffeid.ID
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
		if !spiffeID.IsZero() {
			evt.SPIFFEID = spiffeID.String()
		}
		if emitErr := wis.emitter.EmitAuditEvent(ctx, evt); emitErr != nil {
			wis.logger.WarnContext(
				ctx, "Failed to emit SPIFFE SVID issued event", "error", err,
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

	spiffeID, err = spiffeid.FromURI(&url.URL{
		Scheme: spiffeScheme,
		Host:   clusterName,
		Path:   req.SpiffeIdPath,
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating SPIFFE ID")
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
	ttl := defaultX509SVIDTTL
	if reqTTL := req.Ttl.AsDuration(); reqTTL > 0 {
		ttl = reqTTL
	}
	if ttl > maxSVIDTTL {
		ttl = maxSVIDTTL
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
		notBefore, notAfter, ca, req.PublicKey, spiffeID.URL(), req.DnsSans, ipSans,
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

func (wis *WorkloadIdentityService) IssueWorkloadIdentity(
	ctx context.Context, req *pb.IssueWorkloadIdentityRequest,
) (*pb.IssueWorkloadIdentityResponse, error) {
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

	workloadIdentity := &pb.WorkloadIdentity{
		Metadata: &headerv1.Metadata{
			Name: "gitlab-pipeline",
		},
		Spec: &pb.WorkloadIdentitySpec{
			Rules: &pb.WorkloadIdentityRules{
				Allow: []*pb.WorkloadIdentityRule{
					{
						Conditions: []*pb.WorkloadIdentityRuleCondition{
							{
								Attribute: "join.gitlab.namespace_path",
								Equals:    "strideynet",
							},
							{
								Attribute: "join.gitlab.user_login",
								Equals:    "strideynet",
							},
						},
					},
				},
				Deny: []*pb.WorkloadIdentityRule{
					{
						Conditions: []*pb.WorkloadIdentityRuleCondition{
							{
								Attribute: "join.gitlab.environment",
								Equals:    "dev",
							},
						},
					},
				},
			},
			Spiffe: &pb.WorkloadIdentitySPIFFE{
				Id: "/gitlab/{{join.gitlab.project_path}}/{{join.gitlab.environment}}",
			},
		},
	}

	attrs := &pb.Attributes{
		Join:   authCtx.Identity.GetIdentity().BotJoinAttributes,
		Traits: []*v1.Trait{},
	}
	for k, values := range authCtx.Identity.GetIdentity().Traits {
		attrs.Traits = append(attrs.Traits, &v1.Trait{
			Key:    k,
			Values: values,
		})
	}

	if ruleMatch(workloadIdentity.GetSpec().GetRules().GetDeny(), attrs) {
		return nil, trace.AccessDenied("denied by workload identity deny rules")
	}
	if !ruleMatch(workloadIdentity.GetSpec().GetRules().GetAllow(), attrs) {
		return nil, trace.AccessDenied("denied by lack of workload identity allow rules")
	}

	templateIDPath, err := template(
		workloadIdentity.GetSpec().GetSpiffe().GetId(), attrs,
	)
	if err != nil {
		return nil, trace.Wrap(err, "templating spiffe id")
	}
	spiffeID, err := spiffeid.FromURI(&url.URL{
		Scheme: spiffeScheme,
		Host:   clusterName.GetClusterName(),
		Path:   templateIDPath,
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating SPIFFE ID")
	}

	ttl := defaultX509SVIDTTL
	notAfter := wis.clock.Now().Add(ttl)
	notBefore := wis.clock.Now().UTC().Add(-1 * time.Minute)

	pemBytes, serialNumber, err := signx509SVID(
		notBefore,
		notAfter,
		tlsCA,
		req.PublicKey,
		spiffeID.URL(),
		[]string{},
		[]net.IP{},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Convert Attrs proto to Structpb for audit log event...
	jsonAttrs, err := (protojson.MarshalOptions{
		UseProtoNames: true,
	}).Marshal(attrs)
	if err != nil {
		return nil, trace.Wrap(err, "marshalling attributes")
	}
	attrStruct := &gogotypes.Struct{}
	if err := jsonpb.Unmarshal(bytes.NewReader(jsonAttrs), attrStruct); err != nil {
		return nil, trace.Wrap(err, "failed to unmarshal attributes into struct...")
	}

	evt := &apievents.SPIFFESVIDIssued{
		Metadata: apievents.Metadata{
			Type: events.SPIFFESVIDIssuedEvent,
			Code: events.SPIFFESVIDIssuedSuccessCode,
		},
		UserMetadata:         authz.ClientUserMetadata(ctx),
		ConnectionMetadata:   authz.ConnectionMetadata(ctx),
		SVIDType:             "x509",
		WorkloadIdentityName: workloadIdentity.GetMetadata().GetName(),
		AttributeContext: &apievents.Struct{
			Struct: *attrStruct,
		},
	}
	if serialNumber != nil {
		evt.SerialNumber = serialString(serialNumber)
	}
	if !spiffeID.IsZero() {
		evt.SPIFFEID = spiffeID.String()
	}
	if emitErr := wis.emitter.EmitAuditEvent(ctx, evt); emitErr != nil {
		wis.logger.WarnContext(
			ctx, "Failed to emit SPIFFE SVID issued event", "error", err,
		)
	}

	return &pb.IssueWorkloadIdentityResponse{
		X509: pemBytes,
	}, nil
}

func ruleMatch(rules []*pb.WorkloadIdentityRule, attr *pb.Attributes) bool {
	attrMap := attrsToMap(attr)
ruleLoop:
	for _, rule := range rules {
		for _, cond := range rule.Conditions {
			v, ok := attrMap[cond.Attribute]
			if !ok {
				continue ruleLoop
			}
			if v != cond.Equals {
				continue ruleLoop
			}
		}
		return true
	}
	return false
}

func attrsToMap(attrs *pb.Attributes) map[string]string {
	if attrs == nil {
		return map[string]string{}
	}
	out := map[string]string{}
	join := attrs.Join
	if join != nil {
		if join.Github != nil {
			out["join.github.repository"] = join.Github.Repository
			out["join.github.environment"] = join.Github.Environment
			out["join.github.workflow"] = join.Github.Workflow
		}
		if join.Gitlab != nil {
			out["join.gitlab.user_login"] = join.Gitlab.UserLogin
			out["join.gitlab.project_path"] = join.Gitlab.ProjectPath
			out["join.gitlab.environment"] = join.Gitlab.Environment
			out["join.gitlab.namespace_path"] = join.Gitlab.NamespacePath
		}
	}
	return out
}

// This place is not a place of honor...
// no highly esteemed deed is commemorated here...
// nothing valued is here.
//
// What is here was dangerous and repulsive to us.
// This message is a warning about danger.
func template(in string, attrs *pb.Attributes) (string, error) {
	variables := map[string]typical.Variable{}
	for k, v := range attrsToMap(attrs) {
		variables[k] = v
	}

	parser, err := typical.NewParser[map[string]string, string](typical.ParserSpec[map[string]string]{
		Variables: variables,
	})
	if err != nil {
		panic(err)
	}

	re := regexp.MustCompile(`\{\{(.*?)\}\}`)
	matches := re.FindAllStringSubmatch(in, -1)

	for _, match := range matches {
		fmt.Println(strings.Trim(match[0], "{}"))
		expr, err := parser.Parse(strings.Trim(match[0], "{}"))
		if err != nil {
			panic(err)
		}
		value, err := expr.Evaluate(map[string]string{})
		if err != nil {
			panic(err)
		}
		in = strings.Replace(in, match[0], value, 1)
	}

	return in, nil
}

func (wis *WorkloadIdentityService) signJWTSVID(
	ctx context.Context,
	authCtx *authz.Context,
	clusterName string,
	issuer string,
	key *jwt.Key,
	req *pb.JWTSVIDRequest,
) (res *pb.JWTSVIDResponse, err error) {
	var jti string
	var spiffeID spiffeid.ID
	defer func() {
		evt := &apievents.SPIFFESVIDIssued{
			Metadata: apievents.Metadata{
				Type: events.SPIFFESVIDIssuedEvent,
				Code: events.SPIFFESVIDIssuedSuccessCode,
			},
			UserMetadata:       authz.ClientUserMetadata(ctx),
			ConnectionMetadata: authz.ConnectionMetadata(ctx),
			Hint:               req.Hint,
			SVIDType:           "jwt",
			Audiences:          req.Audiences,
		}
		if err != nil {
			evt.Code = events.SPIFFESVIDIssuedFailureCode
		}
		if !spiffeID.IsZero() {
			evt.SPIFFEID = spiffeID.String()
		}
		if jti != "" {
			evt.JTI = jti
		}
		if emitErr := wis.emitter.EmitAuditEvent(ctx, evt); emitErr != nil {
			wis.logger.WarnContext(
				ctx,
				"Failed to emit SPIFFE SVID issued event.",
				"error", emitErr,
			)
		}
	}()

	switch {
	case req.SpiffeIdPath == "":
		return nil, trace.BadParameter("spiffe_id_path: must be non-empty")
	case !strings.HasPrefix(req.SpiffeIdPath, "/"):
		return nil, trace.BadParameter("spiffe_id_path: must start with '/'")
	case len(req.Audiences) == 0:
		return nil, trace.BadParameter("audiences: must be non-empty")
	}

	// Perform authz checks. They must be allowed to issue the SPIFFE ID. Since
	// this is a JWT SVID, there are no SANs to check.
	if err := authCtx.Checker.CheckSPIFFESVID(
		req.SpiffeIdPath,
		[]string{},
		[]net.IP{},
	); err != nil {
		return nil, trace.Wrap(err)
	}

	// Create a JTI to uniquely identify this JWT for audit logging purposes
	jti, err = utils.CryptoRandomHex(jtiLength)
	if err != nil {
		return nil, trace.Wrap(err, "generating JTI")
	}

	spiffeID, err = spiffeid.FromURI(&url.URL{
		Scheme: spiffeScheme,
		Host:   clusterName,
		Path:   req.SpiffeIdPath,
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating SPIFFE ID")
	}

	ttl := defaultJWTSVIDTTL
	if reqTTL := req.Ttl.AsDuration(); reqTTL > 0 {
		ttl = reqTTL
	}
	if ttl > maxSVIDTTL {
		ttl = maxSVIDTTL
	}

	signed, err := key.SignJWTSVID(jwt.SignParamsJWTSVID{
		Audiences: req.Audiences,
		SPIFFEID:  spiffeID,
		TTL:       ttl,
		JTI:       jti,
		Issuer:    issuer,
	})
	if err != nil {
		return nil, trace.Wrap(err, "signing jwt")
	}

	return &pb.JWTSVIDResponse{
		Audiences: req.Audiences,
		Jwt:       signed,
		SpiffeId:  spiffeID.String(),
		Jti:       jti,
		Hint:      req.Hint,
	}, nil
}

// SignJWTSVIDs signs and returns the requested JWT SVIDs.
func (wis *WorkloadIdentityService) SignJWTSVIDs(
	ctx context.Context, req *pb.SignJWTSVIDsRequest,
) (*pb.SignJWTSVIDsResponse, error) {
	if len(req.Svids) == 0 {
		return nil, trace.BadParameter("svids: must be non-empty")
	}

	authCtx, err := wis.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Fetch info that will be needed to create the SVIDs
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
	jwtSigner, err := wis.keyStorer.GetJWTSigner(ctx, ca)
	if err != nil {
		return nil, trace.Wrap(err, "getting JWT signer")
	}
	jwtKey, err := services.GetJWTSigner(
		jwtSigner, clusterName.GetClusterName(), wis.clock,
	)
	if err != nil {
		return nil, trace.Wrap(err, "getting JWT key")
	}

	// Determine the public address of the proxy for inclusion in the JWT as
	// the issuer for purposes of OIDC compatibility.
	issuer, err := oidc.IssuerForCluster(ctx, wis.cache, "/workload-identity")
	if err != nil {
		return nil, trace.Wrap(err, "determining issuer")
	}

	res := &pb.SignJWTSVIDsResponse{}
	for i, svidReq := range req.Svids {
		svidRes, err := wis.signJWTSVID(
			ctx, authCtx, clusterName.GetClusterName(), issuer, jwtKey, svidReq,
		)
		if err != nil {
			return nil, trace.Wrap(err, "signing svid %d", i)
		}
		res.Svids = append(res.Svids, svidRes)
	}

	return res, nil
}
