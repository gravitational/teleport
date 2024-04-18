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

package auth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"log/slog"
	"os"
	"slices"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/net/http2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/circleci"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/cloud/gcp"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/githubactions"
	"github.com/gravitational/teleport/lib/gitlab"
	"github.com/gravitational/teleport/lib/kubernetestoken"
	"github.com/gravitational/teleport/lib/spacelift"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/tpm"
	"github.com/gravitational/teleport/lib/utils"
)

// LocalRegister is used to generate host keys when a node or proxy is running
// within the same process as the Auth Server and as such, does not need to
// use provisioning tokens.
func LocalRegister(id IdentityID, authServer *Server, additionalPrincipals, dnsNames []string, remoteAddr string, systemRoles []types.SystemRole) (*Identity, error) {
	priv, pub, err := native.GenerateKeyPair()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsPub, err := PrivateKeyToPublicKeyTLS(priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If local registration is happening and no remote address was passed in
	// (which means no advertise IP was set), use localhost.
	if remoteAddr == "" {
		remoteAddr = defaults.Localhost
	}
	certs, err := authServer.GenerateHostCerts(context.Background(),
		&proto.HostCertsRequest{
			HostID:               id.HostUUID,
			NodeName:             id.NodeName,
			Role:                 id.Role,
			AdditionalPrincipals: additionalPrincipals,
			RemoteAddr:           remoteAddr,
			DNSNames:             dnsNames,
			NoCache:              true,
			PublicSSHKey:         pub,
			PublicTLSKey:         tlsPub,
			SystemRoles:          systemRoles,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	identity, err := ReadIdentityFromKeyPair(priv, certs)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return identity, nil
}

// AzureParams is the parameters specific to the azure join method.
type AzureParams struct {
	// ClientID is the client ID of the managed identity for Teleport to assume
	// when authenticating a node.
	ClientID string
}

// RegisterParams specifies parameters
// for first time register operation with auth server
type RegisterParams struct {
	// Token is a secure token to join the cluster
	Token string
	// ID is identity ID
	ID IdentityID
	// AuthServers is a list of auth servers to dial
	AuthServers []utils.NetAddr
	// ProxyServer is a proxy server to dial
	ProxyServer utils.NetAddr
	// AdditionalPrincipals is a list of additional principals to dial
	AdditionalPrincipals []string
	// DNSNames is a list of DNS names to add to x509 certificate
	DNSNames []string
	// PublicTLSKey is a server's public key to sign
	PublicTLSKey []byte
	// PublicSSHKey is a server's public SSH key to sign
	PublicSSHKey []byte
	// CipherSuites is a list of cipher suites to use for TLS client connection
	CipherSuites []uint16
	// CAPins are the SKPI hashes of the CAs used to verify the Auth Server.
	CAPins []string
	// CAPath is the path to the CA file.
	CAPath string
	// GetHostCredentials is a client that can fetch host credentials.
	GetHostCredentials HostCredentials
	// Clock specifies the time provider. Will be used to override the time anchor
	// for TLS certificate verification.
	// Defaults to real clock if unspecified
	Clock clockwork.Clock
	// JoinMethod is the joining method used for this register request.
	JoinMethod types.JoinMethod
	// ec2IdentityDocument is used for Simplified Node Joining to prove the
	// identity of a joining EC2 instance.
	ec2IdentityDocument []byte
	// AzureParams is the parameters specific to the azure join method.
	AzureParams AzureParams
	// CircuitBreakerConfig defines how the circuit breaker should behave.
	CircuitBreakerConfig breaker.Config
	// FIPS means FedRAMP/FIPS 140-2 compliant configuration was requested.
	FIPS bool
	// IDToken is a token retrieved from a workload identity provider for
	// certain join types e.g GitHub, Google.
	IDToken string
	// Expires is an optional field for bots that specifies a time that the
	// certificates that are returned by registering should expire at.
	// It should not be specified for non-bot registrations.
	Expires *time.Time
	// Insecure trusts the certificates from the Auth Server or Proxy during registration without verification.
	Insecure bool
}

func (r *RegisterParams) checkAndSetDefaults() error {
	if r.Clock == nil {
		r.Clock = clockwork.NewRealClock()
	}

	if err := r.verifyAuthOrProxyAddress(); err != nil {
		return trace.BadParameter("no auth or proxy servers set")
	}

	return nil
}

func (r *RegisterParams) verifyAuthOrProxyAddress() error {
	haveAuthServers := len(r.AuthServers) > 0
	haveProxyServer := !r.ProxyServer.IsEmpty()

	if !haveAuthServers && !haveProxyServer {
		return trace.BadParameter("no auth or proxy servers set")
	}

	if haveAuthServers && haveProxyServer {
		return trace.BadParameter("only one of auth or proxy server should be set")
	}

	return nil
}

// CredGetter is an interface for a client that can be used to get host
// credentials. This interface is needed because lib/client can not be imported
// in lib/auth due to circular imports.
type HostCredentials func(context.Context, string, bool, types.RegisterUsingTokenRequest) (*proto.Certs, error)

// Register is used to generate host keys when a node or proxy are running on
// different hosts than the auth server. This method requires provisioning
// tokens to prove a valid auth server was used to issue the joining request
// as well as a method for the node to validate the auth server.
func Register(ctx context.Context, params RegisterParams) (certs *proto.Certs, err error) {
	ctx, span := tracer.Start(ctx, "Register")
	defer func() { tracing.EndSpan(span, err) }()

	if err := params.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	// Read in the token. The token can either be passed in or come from a file
	// on disk.
	token, err := utils.TryReadValueAsFile(params.Token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// add EC2 Identity Document to params if required for given join method
	switch params.JoinMethod {
	case types.JoinMethodEC2:
		if !aws.IsEC2NodeID(params.ID.HostUUID) {
			return nil, trace.BadParameter(
				`Host ID %q is not valid when using the EC2 join method, `+
					`try removing the "host_uuid" file in your teleport data dir `+
					`(e.g. /var/lib/teleport/host_uuid)`,
				params.ID.HostUUID)
		}
		params.ec2IdentityDocument, err = utils.GetRawEC2IdentityDocument(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case types.JoinMethodGitHub:
		params.IDToken, err = githubactions.NewIDTokenSource().GetIDToken(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case types.JoinMethodGitLab:
		params.IDToken, err = gitlab.NewIDTokenSource(os.Getenv).GetIDToken()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case types.JoinMethodCircleCI:
		params.IDToken, err = circleci.GetIDToken(os.Getenv)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case types.JoinMethodKubernetes:
		params.IDToken, err = kubernetestoken.GetIDToken(os.Getenv, os.ReadFile)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case types.JoinMethodGCP:
		params.IDToken, err = gcp.GetIDToken(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case types.JoinMethodSpacelift:
		params.IDToken, err = spacelift.NewIDTokenSource(os.Getenv).GetIDToken()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	type registerMethod struct {
		call func(ctx context.Context, token string, params RegisterParams) (*proto.Certs, error)
		desc string
	}

	registerThroughAuth := registerMethod{registerThroughAuth, "with auth server"}
	registerThroughProxy := registerMethod{registerThroughProxy, "via proxy server"}

	registerMethods := []registerMethod{registerThroughAuth, registerThroughProxy}

	if !params.ProxyServer.IsEmpty() {
		log.WithField("proxy-server", params.ProxyServer).Debugf("Registering node to the cluster.")

		registerMethods = []registerMethod{registerThroughProxy}

		if proxyServerIsAuth(params.ProxyServer) {
			log.Debugf("The specified proxy server appears to be an auth server.")
		}
	} else {
		log.WithField("auth-servers", params.AuthServers).Debugf("Registering node to the cluster.")

		if params.GetHostCredentials == nil {
			log.Debugf("Missing client, it is not possible to register through proxy.")
			registerMethods = []registerMethod{registerThroughAuth}
		} else if authServerIsProxy(params.AuthServers) {
			log.Debugf("The first specified auth server appears to be a proxy.")
			registerMethods = []registerMethod{registerThroughProxy, registerThroughAuth}
		}
	}

	var collectedErrs []error
	for _, method := range registerMethods {
		log.Infof("Attempting registration %s.", method.desc)
		certs, err := method.call(ctx, token, params)
		if err != nil {
			collectedErrs = append(collectedErrs, err)
			log.WithError(err).Debugf("Registration %s failed.", method.desc)
			continue
		}
		log.Infof("Successfully registered %s.", method.desc)
		return certs, nil
	}
	return nil, trace.NewAggregate(collectedErrs...)
}

// authServerIsProxy returns true if the first specified auth server
// to register with appears to be a proxy.
func authServerIsProxy(servers []utils.NetAddr) bool {
	if len(servers) == 0 {
		return false
	}
	port := servers[0].Port(0)
	return port == defaults.HTTPListenPort || port == teleport.StandardHTTPSPort
}

// proxyServerIsAuth returns true if the address given to register with
// appears to be an auth server.
func proxyServerIsAuth(server utils.NetAddr) bool {
	port := server.Port(0)
	return port == defaults.AuthListenPort
}

// registerThroughProxy is used to register through the proxy server.
func registerThroughProxy(
	ctx context.Context,
	token string,
	params RegisterParams,
) (certs *proto.Certs, err error) {
	ctx, span := tracer.Start(ctx, "registerThroughProxy")
	defer func() { tracing.EndSpan(span, err) }()

	switch params.JoinMethod {
	case types.JoinMethodIAM, types.JoinMethodAzure, types.JoinMethodTPM:
		// IAM and Azure join methods require gRPC client
		conn, err := proxyJoinServiceConn(ctx, params, params.Insecure)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		defer conn.Close()

		joinServiceClient := client.NewJoinServiceClient(proto.NewJoinServiceClient(conn))
		switch params.JoinMethod {
		case types.JoinMethodIAM:
			certs, err = registerUsingIAMMethod(ctx, joinServiceClient, token, params)
		case types.JoinMethodAzure:
			certs, err = registerUsingAzureMethod(ctx, joinServiceClient, token, params)
		case types.JoinMethodTPM:
			certs, err = registerUsingTPMMethod(ctx, joinServiceClient, token, params)
		default:
			return nil, trace.BadParameter("unhandled join method %q", params.JoinMethod)
		}

		if err != nil {
			return nil, trace.Wrap(err)
		}
	default:
		// The rest of the join methods use GetHostCredentials function passed through
		// params to call proxy HTTP endpoint
		var err error
		certs, err = params.GetHostCredentials(ctx,
			getHostAddresses(params)[0],
			params.Insecure,
			types.RegisterUsingTokenRequest{
				Token:                token,
				HostID:               params.ID.HostUUID,
				NodeName:             params.ID.NodeName,
				Role:                 params.ID.Role,
				AdditionalPrincipals: params.AdditionalPrincipals,
				DNSNames:             params.DNSNames,
				PublicTLSKey:         params.PublicTLSKey,
				PublicSSHKey:         params.PublicSSHKey,
				EC2IdentityDocument:  params.ec2IdentityDocument,
				IDToken:              params.IDToken,
				Expires:              params.Expires,
			})
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return certs, nil
}

func getHostAddresses(params RegisterParams) []string {
	if !params.ProxyServer.IsEmpty() {
		return []string{params.ProxyServer.String()}
	}

	return utils.NetAddrsToStrings(params.AuthServers)
}

// registerThroughAuth is used to register through the auth server.
func registerThroughAuth(
	ctx context.Context, token string, params RegisterParams,
) (certs *proto.Certs, err error) {
	ctx, span := tracer.Start(ctx, "registerThroughAuth")
	defer func() { tracing.EndSpan(span, err) }()

	var client *Client
	// Build a client for the Auth Server with different certificate validation
	// depending on the configured values for Insecure, CAPins and CAPath.
	switch {
	case params.Insecure:
		log.Warnf("Insecure mode enabled. Auth Server cert will not be validated and CAPins and CAPath value will be ignored.")
		client, err = insecureRegisterClient(params)
	case len(params.CAPins) != 0:
		// CAPins takes precedence over CAPath
		client, err = pinRegisterClient(ctx, params)
	case params.CAPath != "":
		client, err = caPathRegisterClient(params)
	default:
		// We fall back to insecure mode here - this is a little odd but is
		// necessary to preserve the behavior of registration. At a later date,
		// we may consider making this an error asking the user to provide
		// Insecure, CAPins or CAPath.
		client, err = insecureRegisterClient(params)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer client.Close()

	switch params.JoinMethod {
	// IAM and Azure methods use unique gRPC endpoints
	case types.JoinMethodIAM:
		certs, err = registerUsingIAMMethod(ctx, client, token, params)
	case types.JoinMethodAzure:
		certs, err = registerUsingAzureMethod(ctx, client, token, params)
	case types.JoinMethodTPM:
		certs, err = registerUsingTPMMethod(ctx, client, token, params)
	default:
		// non-IAM join methods use HTTP endpoint
		// Get the SSH and X509 certificates for a node.
		certs, err = client.RegisterUsingToken(
			ctx,
			&types.RegisterUsingTokenRequest{
				Token:                token,
				HostID:               params.ID.HostUUID,
				NodeName:             params.ID.NodeName,
				Role:                 params.ID.Role,
				AdditionalPrincipals: params.AdditionalPrincipals,
				DNSNames:             params.DNSNames,
				PublicTLSKey:         params.PublicTLSKey,
				PublicSSHKey:         params.PublicSSHKey,
				EC2IdentityDocument:  params.ec2IdentityDocument,
				IDToken:              params.IDToken,
				Expires:              params.Expires,
			})
	}
	return certs, trace.Wrap(err)
}

// proxyJoinServiceConn attempts to connect to the join service running on the
// proxy. The Proxy's TLS cert will be verified using the host's root CA pool
// (PKI) unless the --insecure flag was passed.
func proxyJoinServiceConn(
	ctx context.Context, params RegisterParams, insecure bool,
) (*grpc.ClientConn, error) {
	tlsConfig := utils.TLSConfig(params.CipherSuites)
	tlsConfig.Time = params.Clock.Now
	// set NextProtos for TLS routing, the actual protocol will be h2
	tlsConfig.NextProtos = []string{string(common.ProtocolProxyGRPCInsecure), http2.NextProtoTLS}

	if insecure {
		tlsConfig.InsecureSkipVerify = true
		log.Warnf("Joining cluster without validating the identity of the Proxy Server.")
	}

	// Check if proxy is behind a load balancer. If so, the connection upgrade
	// will verify the load balancer's cert using system cert pool. This
	// provides the same level of security as the client only verifies Proxy's
	// web cert against system cert pool when connection upgrade is not
	// required.
	//
	// With the ALPN connection upgrade, the tunneled TLS Routing request will
	// skip verify as the Proxy server will present its host cert which is not
	// fully verifiable at this point since the client does not have the host
	// CAs yet before completing registration.
	alpnConnUpgrade := client.IsALPNConnUpgradeRequired(ctx, getHostAddresses(params)[0], insecure)
	if alpnConnUpgrade && !insecure {
		tlsConfig.InsecureSkipVerify = true
		tlsConfig.VerifyConnection = verifyALPNUpgradedConn(params.Clock)
	}

	dialer := client.NewDialer(
		ctx,
		apidefaults.DefaultIdleTimeout,
		apidefaults.DefaultIOTimeout,
		client.WithInsecureSkipVerify(insecure),
		client.WithALPNConnUpgrade(alpnConnUpgrade),
	)

	conn, err := grpc.Dial(
		getHostAddresses(params)[0],
		grpc.WithContextDialer(client.GRPCContextDialer(dialer)),
		grpc.WithUnaryInterceptor(metadata.UnaryClientInterceptor),
		grpc.WithStreamInterceptor(metadata.StreamClientInterceptor),
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
	)
	return conn, trace.Wrap(err)
}

// verifyALPNUpgradedConn is a tls.Config.VerifyConnection callback function
// used by the tunneled TLS Routing request to verify the host cert of a Proxy
// behind a L7 load balancer.
//
// Since the client has not obtained the cluster CAs at this point, the
// presented cert cannot be fully verified yet. For now, this function only
// checks if "teleport.cluster.local" is present as one of the DNS names and
// verifies the cert is not expired.
func verifyALPNUpgradedConn(clock clockwork.Clock) func(tls.ConnectionState) error {
	return func(server tls.ConnectionState) error {
		for _, cert := range server.PeerCertificates {
			if slices.Contains(cert.DNSNames, constants.APIDomain) && clock.Now().Before(cert.NotAfter) {
				return nil
			}
		}
		return trace.AccessDenied("server is not a Teleport proxy or server certificate is expired")
	}
}

// insecureRegisterClient attempts to connects to the Auth Server using the
// CA on disk. If no CA is found on disk, Teleport will not verify the Auth
// Server it is connecting to.
func insecureRegisterClient(params RegisterParams) (*Client, error) {
	log.Warnf("Joining cluster without validating the identity of the Auth " +
		"Server. This may open you up to a Man-In-The-Middle (MITM) attack if an " +
		"attacker can gain privileged network access. To remedy this, use the CA pin " +
		"value provided when join token was generated to validate the identity of " +
		"the Auth Server or point to a valid Certificate via the CA Path option.")

	tlsConfig := utils.TLSConfig(params.CipherSuites)
	tlsConfig.Time = params.Clock.Now
	tlsConfig.InsecureSkipVerify = true

	client, err := NewClient(client.Config{
		Addrs: getHostAddresses(params),
		Credentials: []client.Credentials{
			client.LoadTLS(tlsConfig),
		},
		CircuitBreakerConfig: params.CircuitBreakerConfig,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return client, nil
}

// readCA will read in CA that will be used to validate the certificate that
// the Auth Server presents.
func readCA(path string) (*x509.Certificate, error) {
	certBytes, err := utils.ReadPath(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cert, err := tlsca.ParseCertificatePEM(certBytes)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse certificate at %v", path)
	}
	return cert, nil
}

// pinRegisterClient first connects to the Auth Server using a insecure
// connection to fetch the root CA. If the root CA matches the provided CA
// pin, a connection will be re-established and the root CA will be used to
// validate the certificate presented. If both conditions hold true, then we
// know we are connecting to the expected Auth Server.
func pinRegisterClient(
	ctx context.Context, params RegisterParams,
) (*Client, error) {
	// Build a insecure client to the Auth Server. This is safe because even if
	// an attacker were to MITM this connection the CA pin will not match below.
	tlsConfig := utils.TLSConfig(params.CipherSuites)
	tlsConfig.InsecureSkipVerify = true
	tlsConfig.Time = params.Clock.Now
	authClient, err := NewClient(client.Config{
		Addrs: getHostAddresses(params),
		Credentials: []client.Credentials{
			client.LoadTLS(tlsConfig),
		},
		CircuitBreakerConfig: params.CircuitBreakerConfig,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer authClient.Close()

	// Fetch the root CA from the Auth Server. The NOP role has access to the
	// GetClusterCACert endpoint.
	localCA, err := authClient.GetClusterCACert(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certs, err := tlsca.ParseCertificatePEMs(localCA.TLSCA)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Check that the SPKI pin matches the CA we fetched over a insecure
	// connection. This makes sure the CA fetched over a insecure connection is
	// in-fact the expected CA.
	err = utils.CheckSPKI(params.CAPins, certs)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, cert := range certs {
		// Check that the fetched CA is valid at the current time.
		err = utils.VerifyCertificateExpiry(cert, params.Clock)
		if err != nil {
			return nil, trace.Wrap(err)
		}

	}
	log.Infof("Joining remote cluster %v with CA pin.", certs[0].Subject.CommonName)

	// Create another client, but this time with the CA provided to validate
	// that the Auth Server was issued a certificate by the same CA.
	tlsConfig = utils.TLSConfig(params.CipherSuites)
	tlsConfig.Time = params.Clock.Now
	certPool := x509.NewCertPool()
	for _, cert := range certs {
		certPool.AddCert(cert)
	}
	tlsConfig.RootCAs = certPool

	authClient, err = NewClient(client.Config{
		Addrs: getHostAddresses(params),
		Credentials: []client.Credentials{
			client.LoadTLS(tlsConfig),
		},
		CircuitBreakerConfig: params.CircuitBreakerConfig,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return authClient, nil
}

func caPathRegisterClient(params RegisterParams) (*Client, error) {
	tlsConfig := utils.TLSConfig(params.CipherSuites)
	tlsConfig.Time = params.Clock.Now

	cert, err := readCA(params.CAPath)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	// If we're unable to read the file at CAPath, we fall back to insecure
	// registration. This preserves the existing behavior. At a later date,
	// we may wish to consider changing this to return an error - but this is a
	// breaking change.
	if trace.IsNotFound(err) {
		log.Warnf("Falling back to insecurely joining because a missing or empty CA Path was provided.")
		return insecureRegisterClient(params)
	}

	certPool := x509.NewCertPool()
	certPool.AddCert(cert)
	tlsConfig.RootCAs = certPool

	log.Infof("Joining remote cluster %v, validating connection with certificate on disk.", cert.Subject.CommonName)

	client, err := NewClient(client.Config{
		Addrs: getHostAddresses(params),
		Credentials: []client.Credentials{
			client.LoadTLS(tlsConfig),
		},
		CircuitBreakerConfig: params.CircuitBreakerConfig,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return client, nil
}

type joinServiceClient interface {
	RegisterUsingIAMMethod(ctx context.Context, challengeResponse client.RegisterIAMChallengeResponseFunc) (*proto.Certs, error)
	RegisterUsingAzureMethod(ctx context.Context, challengeResponse client.RegisterAzureChallengeResponseFunc) (*proto.Certs, error)
	RegisterUsingTPMMethod(
		ctx context.Context,
		initReq *proto.RegisterUsingTPMMethodInitialRequest,
		solveChallenge client.RegisterTPMChallengeResponseFunc,
	) (*proto.Certs, error)
}

func registerUsingTokenRequestForParams(token string, params RegisterParams) *types.RegisterUsingTokenRequest {
	return &types.RegisterUsingTokenRequest{
		Token:                token,
		HostID:               params.ID.HostUUID,
		NodeName:             params.ID.NodeName,
		Role:                 params.ID.Role,
		AdditionalPrincipals: params.AdditionalPrincipals,
		DNSNames:             params.DNSNames,
		PublicTLSKey:         params.PublicTLSKey,
		PublicSSHKey:         params.PublicSSHKey,
		Expires:              params.Expires,
	}
}

// registerUsingIAMMethod is used to register using the IAM join method. It is
// able to register through a proxy or through the auth server directly.
func registerUsingIAMMethod(
	ctx context.Context, joinServiceClient joinServiceClient, token string, params RegisterParams,
) (*proto.Certs, error) {
	log.Infof("Attempting to register %s with IAM method using regional STS endpoint", params.ID.Role)
	// Call RegisterUsingIAMMethod and pass a callback to respond to the challenge with a signed join request.
	certs, err := joinServiceClient.RegisterUsingIAMMethod(ctx, func(challenge string) (*proto.RegisterUsingIAMMethodRequest, error) {
		// create the signed sts:GetCallerIdentity request and include the challenge
		signedRequest, err := createSignedSTSIdentityRequest(ctx, challenge,
			withFIPSEndpoint(params.FIPS),
			withRegionalEndpoint(true),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// send the register request including the challenge response
		return &proto.RegisterUsingIAMMethodRequest{
			RegisterUsingTokenRequest: registerUsingTokenRequestForParams(token, params),
			StsIdentityRequest:        signedRequest,
		}, nil
	})
	if err != nil {
		log.WithError(err).Infof("Failed to register %s using regional STS endpoint", params.ID.Role)
		return nil, trace.Wrap(err)
	}

	log.Infof("Successfully registered %s with IAM method using regional STS endpoint", params.ID.Role)
	return certs, nil
}

// registerUsingAzureMethod is used to register using the Azure join method. It
// is able to register through a proxy or through the auth server directly.
func registerUsingAzureMethod(
	ctx context.Context, client joinServiceClient, token string, params RegisterParams,
) (*proto.Certs, error) {
	certs, err := client.RegisterUsingAzureMethod(ctx, func(challenge string) (*proto.RegisterUsingAzureMethodRequest, error) {
		imds := azure.NewInstanceMetadataClient()
		if !imds.IsAvailable(ctx) {
			return nil, trace.AccessDenied("could not reach instance metadata. Is Teleport running on an Azure VM?")
		}
		ad, err := imds.GetAttestedData(ctx, challenge)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		accessToken, err := imds.GetAccessToken(ctx, params.AzureParams.ClientID)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &proto.RegisterUsingAzureMethodRequest{
			RegisterUsingTokenRequest: registerUsingTokenRequestForParams(token, params),
			AttestedData:              ad,
			AccessToken:               accessToken,
		}, nil
	})
	return certs, trace.Wrap(err)
}

// registerUsingTPMMethod is used to register using the TPM join method. It
// is able to register through a proxy or through the auth server directly.
func registerUsingTPMMethod(
	ctx context.Context,
	client joinServiceClient,
	token string,
	params RegisterParams,
) (*proto.Certs, error) {
	log := slog.Default()

	initReq := &proto.RegisterUsingTPMMethodInitialRequest{
		JoinRequest: registerUsingTokenRequestForParams(token, params),
	}

	attestation, close, err := tpm.Attest(ctx, log)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer func() {
		if err := close(); err != nil {
			log.WarnContext(ctx, "Failed to close TPM", "error", err)
		}
	}()

	initReq.AttestationParams = tpm.AttestationParametersToProto(
		attestation.AttestParams,
	)
	// Get the EKKey or EKCert. We want to prefer the EKCert if it is available
	// as this is signed by the manufacturer.
	switch {
	case attestation.Data.EKCert != nil:
		log.DebugContext(
			ctx,
			"Using EKCert for TPM registration",
			"ekcert_serial", attestation.Data.EKCert.SerialNumber,
		)
		initReq.Ek = &proto.RegisterUsingTPMMethodInitialRequest_EkCert{
			EkCert: attestation.Data.EKCert.Raw,
		}
	case attestation.Data.EKPub != nil:
		log.DebugContext(
			ctx,
			"Using EKKey for TPM registration",
			"ekpub_hash", attestation.Data.EKPubHash,
		)
		initReq.Ek = &proto.RegisterUsingTPMMethodInitialRequest_EkKey{
			EkKey: attestation.Data.EKPub,
		}
	default:
		return nil, trace.BadParameter("tpm has neither ekkey or ekcert")
	}

	// Submit initial request to the Auth Server.
	certs, err := client.RegisterUsingTPMMethod(
		ctx,
		initReq,
		func(
			challenge *proto.TPMEncryptedCredential,
		) (*proto.RegisterUsingTPMMethodChallengeResponse, error) {
			// Solve the encrypted credential with our AK to prove possession
			// and obtain the solution we need to complete the ceremony.
			solution, err := attestation.Solve(tpm.EncryptedCredentialFromProto(
				challenge,
			))
			if err != nil {
				return nil, trace.Wrap(err, "activating credential")
			}
			return &proto.RegisterUsingTPMMethodChallengeResponse{
				Solution: solution,
			}, nil
		},
	)
	return certs, trace.Wrap(err)
}

// ReRegisterParams specifies parameters for re-registering
// in the cluster (rotating certificates for existing members)
type ReRegisterParams struct {
	// Client is an authenticated client using old credentials
	Client ClientI
	// ID is identity ID
	ID IdentityID
	// AdditionalPrincipals is a list of additional principals to dial
	AdditionalPrincipals []string
	// DNSNames is a list of DNS Names to add to the x509 client certificate
	DNSNames []string
	// PrivateKey is a PEM encoded private key (not passed to auth servers)
	PrivateKey []byte
	// PublicTLSKey is a server's public key to sign
	PublicTLSKey []byte
	// PublicSSHKey is a server's public SSH key to sign
	PublicSSHKey []byte
	// Rotation is the rotation state of the certificate authority
	Rotation types.Rotation
	// SystemRoles is a set of additional system roles held by the instance.
	SystemRoles []types.SystemRole
}

// ReRegister renews the certificates and private keys based on the client's existing identity.
func ReRegister(ctx context.Context, params ReRegisterParams) (*Identity, error) {
	var rotation *types.Rotation
	if !params.Rotation.IsZero() {
		// older auths didn't distinguish between empty and nil rotation
		// structs, so we go out of our way to only send non-nil rotation
		// if it is truly non-empty.
		rotation = &params.Rotation
	}
	certs, err := params.Client.GenerateHostCerts(ctx,
		&proto.HostCertsRequest{
			HostID:               params.ID.HostID(),
			NodeName:             params.ID.NodeName,
			Role:                 params.ID.Role,
			AdditionalPrincipals: params.AdditionalPrincipals,
			DNSNames:             params.DNSNames,
			PublicTLSKey:         params.PublicTLSKey,
			PublicSSHKey:         params.PublicSSHKey,
			Rotation:             rotation,
			SystemRoles:          params.SystemRoles,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ReadIdentityFromKeyPair(params.PrivateKey, certs)
}
