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
	"crypto"
	"crypto/x509"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v3"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"go.opentelemetry.io/otel"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/join/iam"
	"github.com/gravitational/teleport/lib/auth/join/oracle"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/azuredevops"
	"github.com/gravitational/teleport/lib/bitbucket"
	"github.com/gravitational/teleport/lib/circleci"
	proxyinsecureclient "github.com/gravitational/teleport/lib/client/proxy/insecure"
	"github.com/gravitational/teleport/lib/cloud/imds/azure"
	"github.com/gravitational/teleport/lib/cloud/imds/gcp"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/githubactions"
	"github.com/gravitational/teleport/lib/gitlab"
	"github.com/gravitational/teleport/lib/jwt"
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

// GitlabParams is the parameters specific to the gitlab join method.
type GitlabParams struct {
	// EnvVarName is the name of the environment variable that contains the
	// IDToken. If unset, this will default to "TBOT_GITLAB_JWT".
	EnvVarName string
}

// GetSignerFunc is a function that fetches a keypair from bound keypair client
// state.
type GetSignerFunc func(pubKey string) (crypto.Signer, error)

// KeygenFunc is a function to generate a new keypair for bound keypair joining.
// Clients will generally need to store this for future use, so this function
// should include some mechanism for storage and retrieval.
type KeygenFunc func(ctx context.Context, getSuite cryptosuites.GetSuiteFunc) (crypto.Signer, error)

// BoundKeypairParams are parameters specific to bound-keypair joining.
type BoundKeypairParams struct {
	// InitialJoinSecret is a one-time-use joining token for use on first join.
	// May be unset if a keypair was registered with Auth out of band.
	InitialJoinSecret string

	// PreviousJoinState is the previous join state document provided by Auth
	// alongside the previous set of certs. If this is initial registration, it
	// can be empty.
	PreviousJoinState []byte

	// GetSigner is a function that fetches a signer from the client keystore.
	GetSigner GetSignerFunc

	// RequestNewKeypair is a callback function used to request a new keypair.
	// This may be called at initial onboarding when `InitialJoinSecret` is set,
	// or on any join (including the initial join) if `RotateAfter` is set on
	// the backing token and its value has elapsed.
	RequestNewKeypair KeygenFunc
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
	// CipherSuites is a list of cipher suites to use for TLS client connection
	// Ignored if AuthClient is provided.
	CipherSuites []uint16
	// CAPins are the SKPI hashes of the CAs used to verify the Auth Server.
	// Ignored if AuthClient is provided.
	CAPins []string
	// CAPath is the path to the CA file.
	// Ignored if AuthClient is provided.
	CAPath string
	// GetHostCredentials is a client that can be used to register via the
	// proxy web API.
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
	// GitlabParams is the parameters specific to the gitlab join method.
	GitlabParams GitlabParams
	// BoundKeypairParams contains parameters specific to bound keypair joining.
	BoundKeypairParams *BoundKeypairParams
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

// BoundKeypairRegistrationResult is the result from a successful bound keypair
// registration attempt. This contains additional values clients are expected to
// store for subsequent join attempts.
type BoundKeypairRegisterResult struct {
	// BoundPublicKey is the public key trusted by the server after the join
	// attempt, and can be used to confirm the current public key after
	// registration or rotation.
	BoundPublicKey string

	// JoinState is a serialized join state JWT. This should be committed to the
	// bound keypair client state and provided via `BoundKeypairParams` on the
	// next join attempt.
	JoinState []byte
}

// RegisterResult contains the certificates and the private key generated during
// the registration process.
type RegisterResult struct {
	// Certs holds the certificates issued and signed by the Auth server.
	Certs *proto.Certs
	// PrivateKey is the subject key of the certificates in [Certs]. It is
	// generated according to the current signature algorithm suite configured
	// in the cluster.
	PrivateKey crypto.Signer
	// BoundKeypair contains additional results from bound keypair registration
	// attempts. This is only set when bound keypair joining is used.
	BoundKeypair *BoundKeypairRegisterResult
}

// Register is used to get signed certificates when a node, proxy, or bot is
// running on a different host than the auth server. This method requires a
// provision token that will be used to authenticate as an identity that should
// be allowed to join the cluster.
func Register(ctx context.Context, params RegisterParams) (result *RegisterResult, err error) {
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
		params.IDToken, err = gitlab.NewIDTokenSource(gitlab.IDTokenSourceConfig{
			EnvVarName: params.GitlabParams.EnvVarName,
		}).GetIDToken()
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
	case types.JoinMethodAzureDevops:
		params.IDToken, err = azuredevops.NewIDTokenSource(os.Getenv).GetIDToken(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case types.JoinMethodBoundKeypair:
		if params.BoundKeypairParams == nil {
			return nil, trace.BadParameter("bound keypair parameters are required")
		}
	}

	// If an explicit AuthClient has been provided, we want to go straight to
	// using that rather than trying both proxy and auth dialing.
	if params.AuthClient != nil {
		slog.InfoContext(ctx, "Attempting registration with existing auth client.")
		result, err := registerThroughAuthClient(ctx, token, params, params.AuthClient)
		if err != nil {
			slog.ErrorContext(ctx, "Registration with existing auth client failed.", "error", err)
			return nil, trace.Wrap(err)
		}
		slog.InfoContext(ctx, "Successfully registered with existing auth client.")
		return result, nil
	}

	type registerMethod struct {
		call func(ctx context.Context, token string, params RegisterParams) (*RegisterResult, error)
		desc string
	}

	registerThroughAuth := registerMethod{registerThroughAuth, "with auth server"}
	registerThroughProxy := registerMethod{registerThroughProxy, "via proxy server"}

	registerMethods := []registerMethod{registerThroughAuth, registerThroughProxy}

	if !params.ProxyServer.IsEmpty() {
		slog.DebugContext(ctx, "Registering node to the cluster.", "proxy_server", params.ProxyServer)

		registerMethods = []registerMethod{registerThroughProxy}

		if proxyServerIsAuth(params.ProxyServer) {
			slog.DebugContext(ctx, "The specified proxy server appears to be an auth server.")
		}
	} else {
		slog.DebugContext(ctx, "Registering node to the cluster.", "auth_servers", params.AuthServers)

		if params.GetHostCredentials == nil {
			slog.DebugContext(ctx, "Missing client, it is not possible to register through proxy.")
			registerMethods = []registerMethod{registerThroughAuth}
		} else if authServerIsProxy(params.AuthServers) {
			slog.DebugContext(ctx, "The first specified auth server appears to be a proxy.")
			registerMethods = []registerMethod{registerThroughProxy, registerThroughAuth}
		}
	}

	var collectedErrs []error
	for _, method := range registerMethods {
		slog.InfoContext(ctx, "Attempting registration.", "method", method.desc)
		result, err := method.call(ctx, token, params)
		if err != nil {
			collectedErrs = append(collectedErrs, err)
			slog.DebugContext(ctx, "Registration failed.", "method", method.desc, "error", err)
			continue
		}
		slog.InfoContext(ctx, "Successfully registered.", "method", method.desc)
		return result, nil
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
) (result *RegisterResult, err error) {
	ctx, span := tracer.Start(ctx, "registerThroughProxy")
	defer func() { tracing.EndSpan(span, err) }()

	proxyAddr := getHostAddresses(params)[0]
	hostKeys, err := generateHostKeysForProxy(ctx, params.Insecure, proxyAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var certs *proto.Certs
	switch params.JoinMethod {
	case types.JoinMethodIAM,
		types.JoinMethodAzure,
		types.JoinMethodTPM,
		types.JoinMethodOracle,
		types.JoinMethodBoundKeypair:

		// These join methods require gRPC client
		conn, err := proxyinsecureclient.NewConnection(
			ctx,
			proxyinsecureclient.ConnectionConfig{
				ProxyServer:  proxyAddr,
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
			certs, err = registerUsingIAMMethod(ctx, joinServiceClient, token, hostKeys, params)
		case types.JoinMethodAzure:
			certs, err = registerUsingAzureMethod(ctx, joinServiceClient, token, hostKeys, params)
		case types.JoinMethodTPM:
			certs, err = registerUsingTPMMethod(ctx, joinServiceClient, token, hostKeys, params)
		case types.JoinMethodOracle:
			certs, err = registerUsingOracleMethod(ctx, joinServiceClient, token, hostKeys, params)
		case types.JoinMethodBoundKeypair:
			// Bound keypair joining needs to set additional fields on the
			// result, so it constructs the struct internally.
			result, err := registerUsingBoundKeypairMethod(ctx, joinServiceClient, token, hostKeys, params)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			return result, nil
		default:
			return nil, trace.BadParameter("unhandled join method %q", params.JoinMethod)
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}
	default:
		// The rest of the join methods use GetHostCredentials function passed
		// through params to call proxy HTTP endpoint.
		var err error
		certs, err = params.GetHostCredentials(ctx,
			proxyAddr,
			params.Insecure,
			*registerUsingTokenRequestForParams(token, hostKeys, params))
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return &RegisterResult{
		Certs:      certs,
		PrivateKey: hostKeys.privateKey,
	}, nil
}

// registerThroughAuth is used to register through the auth server.
func registerThroughAuth(
	ctx context.Context, token string, params RegisterParams,
) (result *RegisterResult, err error) {
	ctx, span := tracer.Start(ctx, "registerThroughAuth")
	defer func() { tracing.EndSpan(span, err) }()

	var client *authclient.Client
	// Build a client for the Auth Server with different certificate validation
	// depending on the configured values for Insecure, CAPins and CAPath.
	switch {
	case params.Insecure:
		slog.WarnContext(ctx, "Insecure mode enabled. Auth Server cert will not be validated and CAPins and CAPath value will be ignored.")
		client, err = insecureRegisterClient(ctx, params)
	case len(params.CAPins) != 0:
		// CAPins takes precedence over CAPath
		client, err = pinRegisterClient(ctx, params)
	case params.CAPath != "":
		client, err = caPathRegisterClient(ctx, params)
	default:
		// We fall back to insecure mode here - this is a little odd but is
		// necessary to preserve the behavior of registration. At a later date,
		// we may consider making this an error asking the user to provide
		// Insecure, CAPins or CAPath.
		client, err = insecureRegisterClient(ctx, params)
	}
	if err != nil {
		return nil, trace.Wrap(err, "building auth client")
	}
	defer client.Close()

	result, err = registerThroughAuthClient(ctx, token, params, client)
	return result, trace.Wrap(err, "registering through auth client")
}

// AuthJoinClient is a client that allows access to the Auth Servers join
// service and RegisterUsingToken method for the purposes of joining.
type AuthJoinClient interface {
	joinServiceClient
	RegisterUsingToken(ctx context.Context, req *types.RegisterUsingTokenRequest) (*proto.Certs, error)
	Ping(ctx context.Context) (proto.PingResponse, error)
}

func registerThroughAuthClient(
	ctx context.Context,
	token string,
	params RegisterParams,
	client AuthJoinClient,
) (result *RegisterResult, err error) {
	hostKeys, err := generateHostKeysForAuth(ctx, client)
	if err != nil {
		return nil, trace.Wrap(err, "generating host keys")
	}

	var certs *proto.Certs
	switch params.JoinMethod {
	// IAM and Azure methods use unique gRPC endpoints
	case types.JoinMethodIAM:
		certs, err = registerUsingIAMMethod(ctx, client, token, hostKeys, params)
	case types.JoinMethodAzure:
		certs, err = registerUsingAzureMethod(ctx, client, token, hostKeys, params)
	case types.JoinMethodTPM:
		certs, err = registerUsingTPMMethod(ctx, client, token, hostKeys, params)
	case types.JoinMethodBoundKeypair:
		// Bound keypair joining has additional return values, so it constructs
		// its RegisterResult internally.
		result, err := registerUsingBoundKeypairMethod(ctx, client, token, hostKeys, params)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return result, nil
	default:
		// non-IAM join methods use HTTP endpoint
		// Get the SSH and X509 certificates for a node.
		certs, err = client.RegisterUsingToken(ctx, registerUsingTokenRequestForParams(token, hostKeys, params))
	}
	if err != nil {
		return nil, trace.Wrap(err, "registering with %s method", params.JoinMethod)
	}
	return &RegisterResult{
		Certs:      certs,
		PrivateKey: hostKeys.privateKey,
	}, nil
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
func insecureRegisterClient(ctx context.Context, params RegisterParams) (*authclient.Client, error) {
	const msg = "Joining cluster without validating the identity of the Auth " +
		"Server. This may open you up to a Man-In-The-Middle (MITM) attack if an " +
		"attacker can gain privileged network access. To remedy this, use the CA pin " +
		"value provided when join token was generated to validate the identity of " +
		"the Auth Server or point to a valid Certificate via the CA Path option."
	slog.WarnContext(ctx, msg)

	tlsConfig := utils.TLSConfig(params.CipherSuites)
	tlsConfig.Time = params.Clock.Now
	tlsConfig.InsecureSkipVerify = true

	client, err := authclient.NewClient(client.Config{
		Addrs: getHostAddresses(params),
		Credentials: []client.Credentials{
			client.LoadTLS(tlsConfig),
		},
		CircuitBreakerConfig: params.CircuitBreakerConfig,
		Context:              ctx,
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating insecure auth client")
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
		Context:              ctx,
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
	slog.InfoContext(ctx, "Joining remote cluster with CA pin.", "cluster", certs[0].Subject.CommonName)

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
		Context:              ctx,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return authClient, nil
}

func caPathRegisterClient(ctx context.Context, params RegisterParams) (*authclient.Client, error) {
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
		slog.WarnContext(ctx, "Falling back to insecurely joining because a missing or empty CA Path was provided.")
		return insecureRegisterClient(ctx, params)
	}

	certPool := x509.NewCertPool()
	certPool.AddCert(cert)
	tlsConfig.RootCAs = certPool

	slog.InfoContext(ctx, "Joining remote cluster, validating connection with certificate on disk.", "cluster", cert.Subject.CommonName)

	client, err := authclient.NewClient(client.Config{
		Addrs: getHostAddresses(params),
		Credentials: []client.Credentials{
			client.LoadTLS(tlsConfig),
		},
		CircuitBreakerConfig: params.CircuitBreakerConfig,
		Context:              ctx,
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
	RegisterUsingOracleMethod(
		ctx context.Context,
		tokenReq *types.RegisterUsingTokenRequest,
		challengeResponse client.RegisterOracleChallengeResponseFunc,
	) (*proto.Certs, error)
	RegisterUsingBoundKeypairMethod(
		ctx context.Context,
		req *proto.RegisterUsingBoundKeypairInitialRequest,
		challengeResponse client.RegisterUsingBoundKeypairChallengeResponseFunc,
	) (*client.BoundKeypairRegistrationResponse, error)
}

func registerUsingTokenRequestForParams(token string, hostKeys *newHostKeys, params RegisterParams) *types.RegisterUsingTokenRequest {
	return &types.RegisterUsingTokenRequest{
		Token:                token,
		HostID:               params.ID.HostUUID,
		NodeName:             params.ID.NodeName,
		Role:                 params.ID.Role,
		AdditionalPrincipals: params.AdditionalPrincipals,
		DNSNames:             params.DNSNames,
		PublicTLSKey:         hostKeys.tlsPub,
		PublicSSHKey:         hostKeys.sshPub,
		EC2IdentityDocument:  params.ec2IdentityDocument,
		IDToken:              params.IDToken,
		Expires:              params.Expires,
	}
}

// registerUsingIAMMethod is used to register using the IAM join method. It is
// able to register through a proxy or through the auth server directly.
func registerUsingIAMMethod(
	ctx context.Context, joinServiceClient joinServiceClient, token string, hostKeys *newHostKeys, params RegisterParams,
) (*proto.Certs, error) {
	slog.InfoContext(ctx, "Attempting to register with IAM method using region STS endpoint.", "role", params.ID.Role)
	// Call RegisterUsingIAMMethod and pass a callback to respond to the challenge with a signed join request.
	certs, err := joinServiceClient.RegisterUsingIAMMethod(ctx, func(challenge string) (*proto.RegisterUsingIAMMethodRequest, error) {
		// create the signed sts:GetCallerIdentity request and include the challenge
		signedRequest, err := iam.CreateSignedSTSIdentityRequest(ctx, challenge,
			iam.WithFIPSEndpoint(params.FIPS),
		)
		if err != nil {
			return nil, trace.Wrap(err, "creating signed sts:GetCallerIdentity request")
		}

		// send the register request including the challenge response
		return &proto.RegisterUsingIAMMethodRequest{
			RegisterUsingTokenRequest: registerUsingTokenRequestForParams(token, hostKeys, params),
			StsIdentityRequest:        signedRequest,
		}, nil
	})
	if err != nil {
		slog.InfoContext(ctx, "Failed to register using regional STS endpoint", "role", params.ID.Role, "error", err)
		return nil, trace.Wrap(err, "registering via IAM method streaming RPC")
	}

	slog.InfoContext(ctx, "Successfully registered with IAM method using regional STS endpoint.", "role", params.ID.Role)
	return certs, nil
}

// registerUsingAzureMethod is used to register using the Azure join method. It
// is able to register through a proxy or through the auth server directly.
func registerUsingAzureMethod(
	ctx context.Context, client joinServiceClient, token string, hostKeys *newHostKeys, params RegisterParams,
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
			RegisterUsingTokenRequest: registerUsingTokenRequestForParams(token, hostKeys, params),
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
	hostKeys *newHostKeys,
	params RegisterParams,
) (*proto.Certs, error) {
	log := slog.Default()

	initReq := &proto.RegisterUsingTPMMethodInitialRequest{
		JoinRequest: registerUsingTokenRequestForParams(token, hostKeys, params),
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

func mapFromHeader(header http.Header) map[string]string {
	out := make(map[string]string, len(header))
	for k := range header {
		out[k] = header.Get(k)
	}
	return out
}

func registerUsingOracleMethod(
	ctx context.Context, client joinServiceClient, token string, hostKeys *newHostKeys, params RegisterParams,
) (*proto.Certs, error) {
	certs, err := client.RegisterUsingOracleMethod(
		ctx,
		registerUsingTokenRequestForParams(token, hostKeys, params),
		func(challenge string) (*proto.OracleSignedRequest, error) {
			innerHeaders, outerHeaders, err := oracle.CreateSignedRequest(challenge)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &proto.OracleSignedRequest{
				Headers:        mapFromHeader(outerHeaders),
				PayloadHeaders: mapFromHeader(innerHeaders),
			}, nil
		})
	return certs, trace.Wrap(err)
}

// sshPubKeyFromSigner returns the public key of the given signer in ssh
// authorized_keys format.
func sshPubKeyFromSigner(signer crypto.Signer) (string, error) {
	sshKey, err := ssh.NewPublicKey(signer.Public())
	if err != nil {
		return "", trace.Wrap(err, "creating SSH public key from signer")
	}

	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshKey))), nil
}

// registerUsingBoundKeypairMethod performs bound keypair-type registration and
// handles the joining ceremony.
func registerUsingBoundKeypairMethod(
	ctx context.Context,
	client joinServiceClient,
	token string,
	hostKeys *newHostKeys,
	params RegisterParams,
) (*RegisterResult, error) {
	bkParams := params.BoundKeypairParams

	initReq := &proto.RegisterUsingBoundKeypairInitialRequest{
		JoinRequest:       registerUsingTokenRequestForParams(token, hostKeys, params),
		InitialJoinSecret: bkParams.InitialJoinSecret,
		PreviousJoinState: bkParams.PreviousJoinState,
	}

	regResponse, err := client.RegisterUsingBoundKeypairMethod(
		ctx,
		initReq,
		func(resp *proto.RegisterUsingBoundKeypairMethodResponse) (*proto.RegisterUsingBoundKeypairMethodRequest, error) {
			switch kind := resp.GetResponse().(type) {
			case *proto.RegisterUsingBoundKeypairMethodResponse_Challenge:
				signer, err := bkParams.GetSigner(kind.Challenge.PublicKey)
				if err != nil {
					return nil, trace.Wrap(err, "could not lookup signer for public key %+v", kind.Challenge.PublicKey)
				}

				alg, err := jwt.AlgorithmForPublicKey(signer.Public())
				if err != nil {
					return nil, trace.Wrap(err, "determining signing algorithm for public key")
				}

				opts := (&jose.SignerOptions{}).WithType("JWT")
				key := jose.SigningKey{
					Algorithm: alg,
					Key:       signer,
				}

				joseSigner, err := jose.NewSigner(key, opts)
				if err != nil {
					return nil, trace.Wrap(err, "creating signer")
				}

				jws, err := joseSigner.Sign([]byte(kind.Challenge.Challenge))
				if err != nil {
					return nil, trace.Wrap(err, "signing challenge")
				}

				serialized, err := jws.CompactSerialize()
				if err != nil {
					return nil, trace.Wrap(err, "serializing signed challenge")
				}

				return &proto.RegisterUsingBoundKeypairMethodRequest{
					Payload: &proto.RegisterUsingBoundKeypairMethodRequest_ChallengeResponse{
						ChallengeResponse: &proto.RegisterUsingBoundKeypairChallengeResponse{
							Solution: []byte(serialized),
						},
					},
				}, nil
			case *proto.RegisterUsingBoundKeypairMethodResponse_Rotation:
				if bkParams.RequestNewKeypair == nil {
					return nil, trace.BadParameter("RequestNewKeypair is required")
				}

				slog.InfoContext(ctx, "Server has requested keypair rotation", "suite", kind.Rotation.SignatureAlgorithmSuite)

				newSigner, err := bkParams.RequestNewKeypair(ctx, cryptosuites.StaticAlgorithmSuite(kind.Rotation.SignatureAlgorithmSuite))
				if err != nil {
					return nil, trace.Wrap(err, "requesting new keypair")
				}

				newPubkey, err := sshPubKeyFromSigner(newSigner)
				if err != nil {
					return nil, trace.Wrap(err)
				}

				return &proto.RegisterUsingBoundKeypairMethodRequest{
					Payload: &proto.RegisterUsingBoundKeypairMethodRequest_RotationResponse{
						RotationResponse: &proto.RegisterUsingBoundKeypairRotationResponse{
							PublicKey: newPubkey,
						},
					},
				}, nil
			default:
				// Note: certs variant is handled by RegisterUsingBoundKeypairMethod()
				return nil, trace.BadParameter("received unexpected challenge response: %v", resp.GetResponse())
			}
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Implementation note, callers are expected to call
	return &RegisterResult{
		PrivateKey: hostKeys.privateKey,
		Certs:      regResponse.Certs,
		BoundKeypair: &BoundKeypairRegisterResult{
			BoundPublicKey: regResponse.BoundPublicKey,
			JoinState:      regResponse.JoinState,
		},
	}, nil
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

type newHostKeys struct {
	privateKey crypto.Signer
	sshPub     []byte
	tlsPub     []byte
}

func generateHostKeysForProxy(ctx context.Context, insecure bool, proxyAddr string) (*newHostKeys, error) {
	getSuite := func(ctx context.Context) (types.SignatureAlgorithmSuite, error) {
		pr, err := webclient.Find(&webclient.Config{
			Context:   ctx,
			ProxyAddr: proxyAddr,
			Insecure:  insecure,
		})
		if err != nil {
			return types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_UNSPECIFIED, trace.Wrap(err, "pinging proxy to determine signature algorithm suite")
		}
		return pr.Auth.SignatureAlgorithmSuite, nil
	}
	return generateHostKeys(ctx, getSuite)
}

func generateHostKeysForAuth(ctx context.Context, authClient AuthJoinClient) (*newHostKeys, error) {
	getSuite := func(ctx context.Context) (types.SignatureAlgorithmSuite, error) {
		pr, err := authClient.Ping(ctx)
		if err != nil {
			return types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_UNSPECIFIED, trace.Wrap(err, "pinging auth to determine signature algorithm suite")
		}
		return pr.SignatureAlgorithmSuite, nil
	}
	return generateHostKeys(ctx, getSuite)
}

func generateHostKeys(ctx context.Context, getSuite cryptosuites.GetSuiteFunc) (*newHostKeys, error) {
	key, err := cryptosuites.GenerateKey(ctx, getSuite, cryptosuites.HostIdentity)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sshPub, err := ssh.NewPublicKey(key.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsPub, err := keys.MarshalPublicKey(key.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &newHostKeys{
		privateKey: key,
		sshPub:     ssh.MarshalAuthorizedKey(sshPub),
		tlsPub:     tlsPub,
	}, nil
}
