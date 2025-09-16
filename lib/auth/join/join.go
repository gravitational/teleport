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

package join

import (
	"context"
	"crypto/x509"
	"log/slog"
	"os"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/join/iam"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/bitbucket"
	"github.com/gravitational/teleport/lib/circleci"
	proxyinsecureclient "github.com/gravitational/teleport/lib/client/proxy/insecure"
	"github.com/gravitational/teleport/lib/cloud/imds/azure"
	"github.com/gravitational/teleport/lib/cloud/imds/gcp"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/githubactions"
	"github.com/gravitational/teleport/lib/gitlab"
	kubetoken "github.com/gravitational/teleport/lib/kube/token"
	"github.com/gravitational/teleport/lib/spacelift"
	"github.com/gravitational/teleport/lib/terraformcloud"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/tpm"
	"github.com/gravitational/teleport/lib/utils"
)

var tracer = otel.Tracer("github.com/gravitational/teleport/lib/auth/join")

// HostCredentials is an interface for a client that can be used to get host
// credentials. This interface is needed because lib/client cannot be imported
// in lib/auth due to circular imports.
type HostCredentials func(context.Context, string, bool, types.RegisterUsingTokenRequest) (*proto.Certs, error)

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
	ID state.IdentityID
	// AuthServers is a list of auth servers to dial
	// Ignored if AuthClient is provided.
	AuthServers []utils.NetAddr
	// ProxyServer is a proxy server to dial
	// Ignored if AuthClient is provided.
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
	// Ignored if AuthClient is provided.
	CipherSuites []uint16
	// CAPins are the SKPI hashes of the CAs used to verify the Auth Server.
	// Ignored if AuthClient is provided.
	CAPins []string
	// CAPath is the path to the CA file.
	// Ignored if AuthClient is provided.
	CAPath string
	// GetHostCredentials is a client that can fetch host credentials.
	// Ignored if AuthClient is provided.
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
	// Ignored if AuthClient is provided.
	CircuitBreakerConfig breaker.Config
	// FIPS means FedRAMP/FIPS 140-2 compliant configuration was requested.
	// Ignored if AuthClient is provided.
	FIPS bool
	// IDToken is a token retrieved from a workload identity provider for
	// certain join types e.g GitHub, Google.
	IDToken string
	// Expires is an optional field for bots that specifies a time that the
	// certificates that are returned by registering should expire at.
	// It should not be specified for non-bot registrations.
	Expires *time.Time
	// Insecure trusts the certificates from the Auth Server or Proxy during registration without verification.
	// Ignored if AuthClient is provided.
	Insecure bool
	// AuthClient allows an existing client with a connection to the auth
	// server to be used for the registration process. If specified, then the
	// Register method will not attempt to dial, and many other parameters
	// may be ignored.
	AuthClient AuthJoinClient
	// KubernetesReadFileFunc is a function used to read the Kubernetes token
	// from disk. Used in tests, and set to `os.ReadFile` if unset.
	KubernetesReadFileFunc func(name string) ([]byte, error)
	// TerraformCloudAudienceTag is a tag name for the environment variable
	// containing TF Cloud's Workload Identity Token when using Terraform Cloud
	// joining.
	TerraformCloudAudienceTag string
}

func (r *RegisterParams) checkAndSetDefaults() error {
	if r.Clock == nil {
		r.Clock = clockwork.NewRealClock()
	}

	if r.KubernetesReadFileFunc == nil {
		r.KubernetesReadFileFunc = os.ReadFile
	}

	if err := r.verifyAuthOrProxyAddress(); err != nil {
		return trace.BadParameter("no auth or proxy servers set")
	}

	return nil
}

func (r *RegisterParams) verifyAuthOrProxyAddress() error {
	// If AuthClient is provided we do not need addresses to dial with.
	if r.AuthClient != nil {
		return nil
	}

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
		params.IDToken, err = kubetoken.GetIDToken(os.Getenv, params.KubernetesReadFileFunc)
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
	case types.JoinMethodTerraformCloud:
		params.IDToken, err = terraformcloud.NewIDTokenSource(params.TerraformCloudAudienceTag, os.Getenv).GetIDToken()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case types.JoinMethodBitbucket:
		params.IDToken, err = bitbucket.NewIDTokenSource(os.Getenv).GetIDToken()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// If an explicit AuthClient has been provided, we want to go straight to
	// using that rather than trying both proxy and auth dialing.
	if params.AuthClient != nil {
		log.Info("Attempting registration with existing auth client.")
		certs, err := registerThroughAuthClient(ctx, token, params, params.AuthClient)
		if err != nil {
			log.WithError(err).Error("Registration with existing auth client failed.")
			return nil, trace.Wrap(err)
		}
		log.Info("Successfully registered with existing auth client.")
		return certs, nil
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
		conn, err := proxyinsecureclient.NewConnection(
			ctx,
			proxyinsecureclient.ConnectionConfig{
				ProxyServer:  getHostAddresses(params)[0],
				CipherSuites: params.CipherSuites,
				Clock:        params.Clock,
				Insecure:     params.Insecure,
				Log:          slog.Default(),
			},
		)
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

// registerThroughAuth is used to register through the auth server.
func registerThroughAuth(
	ctx context.Context, token string, params RegisterParams,
) (certs *proto.Certs, err error) {
	ctx, span := tracer.Start(ctx, "registerThroughAuth")
	defer func() { tracing.EndSpan(span, err) }()

	var client *authclient.Client
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

	certs, err = registerThroughAuthClient(ctx, token, params, client)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return certs, nil
}

// AuthJoinClient is a client that allows access to the Auth Servers join
// service and RegisterUsingToken method for the purposes of joining.
type AuthJoinClient interface {
	joinServiceClient
	RegisterUsingToken(ctx context.Context, req *types.RegisterUsingTokenRequest) (*proto.Certs, error)
}

func registerThroughAuthClient(
	ctx context.Context,
	token string,
	params RegisterParams,
	client AuthJoinClient,
) (certs *proto.Certs, err error) {
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

func getHostAddresses(params RegisterParams) []string {
	if !params.ProxyServer.IsEmpty() {
		return []string{params.ProxyServer.String()}
	}

	return utils.NetAddrsToStrings(params.AuthServers)
}

// insecureRegisterClient attempts to connects to the Auth Server using the
// CA on disk. If no CA is found on disk, Teleport will not verify the Auth
// Server it is connecting to.
func insecureRegisterClient(params RegisterParams) (*authclient.Client, error) {
	log.Warnf("Joining cluster without validating the identity of the Auth " +
		"Server. This may open you up to a Man-In-The-Middle (MITM) attack if an " +
		"attacker can gain privileged network access. To remedy this, use the CA pin " +
		"value provided when join token was generated to validate the identity of " +
		"the Auth Server or point to a valid Certificate via the CA Path option.")

	tlsConfig := utils.TLSConfig(params.CipherSuites)
	tlsConfig.Time = params.Clock.Now
	tlsConfig.InsecureSkipVerify = true

	client, err := authclient.NewClient(client.Config{
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

// pinRegisterClient first connects to the Auth Server using a insecure
// connection to fetch the root CA. If the root CA matches the provided CA
// pin, a connection will be re-established and the root CA will be used to
// validate the certificate presented. If both conditions hold true, then we
// know we are connecting to the expected Auth Server.
func pinRegisterClient(
	ctx context.Context, params RegisterParams,
) (*authclient.Client, error) {
	// Build a insecure client to the Auth Server. This is safe because even if
	// an attacker were to MITM this connection the CA pin will not match below.
	tlsConfig := utils.TLSConfig(params.CipherSuites)
	tlsConfig.InsecureSkipVerify = true
	tlsConfig.Time = params.Clock.Now
	authClient, err := authclient.NewClient(client.Config{
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

	authClient, err = authclient.NewClient(client.Config{
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

func caPathRegisterClient(params RegisterParams) (*authclient.Client, error) {
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

	client, err := authclient.NewClient(client.Config{
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
		signedRequest, err := iam.CreateSignedSTSIdentityRequest(ctx, challenge,
			iam.WithFIPSEndpoint(params.FIPS),
			iam.WithRegionalEndpoint(true),
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
