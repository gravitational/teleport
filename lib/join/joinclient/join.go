// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package joinclient

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"log/slog"
	"os"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client/proto"
	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/types"
	authjoin "github.com/gravitational/teleport/lib/auth/join"
	proxyinsecureclient "github.com/gravitational/teleport/lib/client/proxy/insecure"
	"github.com/gravitational/teleport/lib/cloud/imds/gcp"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/join/azuredevops"
	"github.com/gravitational/teleport/lib/join/bitbucket"
	"github.com/gravitational/teleport/lib/join/circleci"
	"github.com/gravitational/teleport/lib/join/env0"
	"github.com/gravitational/teleport/lib/join/githubactions"
	"github.com/gravitational/teleport/lib/join/gitlab"
	"github.com/gravitational/teleport/lib/join/internal/messages"
	"github.com/gravitational/teleport/lib/join/joinv1"
	"github.com/gravitational/teleport/lib/join/spacelift"
	"github.com/gravitational/teleport/lib/join/terraformcloud"
	kubetoken "github.com/gravitational/teleport/lib/kube/token"
	"github.com/gravitational/teleport/lib/utils/hostid"
)

type (
	JoinParams   = authjoin.RegisterParams
	JoinResult   = authjoin.RegisterResult
	AzureParams  = authjoin.AzureParams
	GitlabParams = authjoin.GitlabParams
)

// Join is used to join a cluster. A host or bot calls this with the name of a
// provision token to get its initial certificates.
func Join(ctx context.Context, params JoinParams) (*JoinResult, error) {
	if err := params.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if params.AuthClient == nil && params.ID.HostUUID != "" {
		// This check is skipped if AuthClient is provided because this is a
		// re-join with an existing identity and the HostUUID will be
		// maintained.
		return nil, trace.BadParameter("HostUUID must not be provided to Join, it will be assigned by the Auth server")
	}
	if params.ID.Role != types.RoleInstance && params.ID.Role != types.RoleBot {
		return nil, trace.BadParameter("Only Instance and Bot roles may be used for direct join attempts")
	}
	slog.InfoContext(ctx, "Trying to join with the new join service")
	result, err := joinNew(ctx, params)
	if trace.IsNotImplemented(err) || isConnectionError(err) {
		// Fall back to joining via legacy service.
		slog.InfoContext(ctx, "Joining via new join service failed, falling back to joining via the legacy join service", "error", err)
		// Non-bots must provide their own host UUID when joining via legacy service.
		if params.ID.HostUUID == "" && params.ID.Role != types.RoleBot {
			hostID, err := hostid.Generate(ctx, params.JoinMethod)
			if err != nil {
				return nil, trace.Wrap(err, "generating host ID")
			}
			slog.InfoContext(ctx, "Generated host UUID for legacy join attempt", "host_uuid", hostID)
			params.ID.HostUUID = hostID
		}
		result, err := LegacyJoin(ctx, params)
		if err != nil {
			return nil, trace.Wrap(&LegacyJoinError{err})
		}
		return result, nil
	}
	return result, trace.Wrap(err)
}

// LegacyJoin is used to join the cluster via the legacy service with client-chosen host UUIDs.
func LegacyJoin(ctx context.Context, params JoinParams) (*JoinResult, error) {
	if params.ID.Role != types.RoleBot && params.ID.HostUUID == "" {
		return nil, trace.BadParameter("HostUUID is required for LegacyJoin")
	}
	//nolint:staticcheck // SA1019 falling back to deprecated method for compatibility.
	result, err := authjoin.Register(ctx, params)
	return result, trace.Wrap(err)
}

func joinNew(ctx context.Context, params JoinParams) (*JoinResult, error) {
	if params.AuthClient != nil {
		slog.InfoContext(ctx, "Attempting to join cluster with existing Auth client")
		return joinViaAuthClient(ctx, params, params.AuthClient)
	}
	if !params.ProxyServer.IsEmpty() {
		slog.InfoContext(ctx, "Attempting to join cluster via Proxy")
		return joinViaProxy(ctx, params, params.ProxyServer.String())
	}

	// params.AuthServers could contain auth or proxy addresses, try both.
	// params.CheckAndSetDefaults() asserts that this list is not empty when
	// AuthClient and ProxyServer are both unset.
	addr := params.AuthServers[0].String()
	slog := slog.With("addr", addr)

	type strategy struct {
		name string
		fn   func() (*JoinResult, error)
	}
	proxyStrategy := strategy{
		name: "proxy",
		fn: func() (*JoinResult, error) {
			return joinViaProxy(ctx, params, addr)
		},
	}
	authStrategy := strategy{
		name: "auth",
		fn: func() (*JoinResult, error) {
			return joinViaAuth(ctx, params)
		},
	}
	var strategies []strategy
	if authjoin.LooksLikeProxy(params.AuthServers) {
		slog.InfoContext(ctx, "Attempting to join cluster, address looks like a Proxy")
		strategies = []strategy{proxyStrategy, authStrategy}
	} else {
		slog.InfoContext(ctx, "Attempting to join cluster, address looks like an Auth server")
		strategies = []strategy{authStrategy, proxyStrategy}
	}

	var errs []error
	for i, strat := range strategies { //nolint:misspell // strat is an intentional abbreviation of strategy
		result, err := strat.fn()
		switch {
		case err == nil:
			return result, nil
		case !isConnectionError(err):
			// Non-connection errors are hard failures: return immediately.
			return nil, trace.Wrap(err, "joining via %s", strat.name)
		}
		// Connection error: keep for aggregate and try next strategy (if any).
		errs = append(errs, trace.Wrap(err, "joining via %s", strat.name))
		if i+1 < len(strategies) {
			slog.InfoContext(ctx, "Failed to join cluster with a connection error, will try next method",
				"method", strat.name, "next_method", strategies[i+1].name)
		}
	}
	return nil, trace.NewAggregate(errs...)
}

func joinViaProxy(ctx context.Context, params JoinParams, proxyAddr string) (*JoinResult, error) {
	// Connect to the proxy's insecure gRPC listener (this is regular TLS, the
	// client is not authenticated because it doesn't have certs yet).
	conn, err := proxyinsecureclient.NewConnection(ctx,
		proxyinsecureclient.ConnectionConfig{
			ProxyServer:  proxyAddr,
			CipherSuites: params.CipherSuites,
			Clock:        params.Clock,
			Insecure:     params.Insecure,
			Log:          slog.Default(),
		},
	)
	if err != nil {
		return nil, &connectionError{trace.Wrap(err, "building proxy client")}
	}
	defer conn.Close()
	return joinWithClient(ctx, params, joinv1.NewClientFromConn(conn))
}

func joinViaAuth(ctx context.Context, params JoinParams) (*JoinResult, error) {
	authClient, err := authjoin.NewAuthClient(ctx, params)
	if err != nil {
		return nil, &connectionError{trace.Wrap(err, "building auth client")}
	}
	defer authClient.Close()
	return joinViaAuthClient(ctx, params, authClient)
}

func joinViaAuthClient(ctx context.Context, params JoinParams, authClient authjoin.AuthJoinClient) (*JoinResult, error) {
	return joinWithClient(ctx, params, joinv1.NewClient(authClient.JoinV1Client()))
}

func joinWithClient(ctx context.Context, params JoinParams, client *joinv1.Client) (*JoinResult, error) {
	// Clients may specify the join method or not, to let the server choose the
	// method based on the provision token.
	var joinMethodPtr *string
	switch params.JoinMethod {
	case types.JoinMethodUnspecified:
		// leave joinMethodPtr nil to let the server pick based on the token
	default:
		joinMethod := string(params.JoinMethod)
		joinMethodPtr = &joinMethod
	}

	// Initiate the join request, using a cancelable context to make sure the
	// stream is closed when this function returns.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stream, err := client.Join(ctx)
	if err != nil {
		// Connection errors may manifest when initiating the RPC.
		return nil, &connectionError{trace.Wrap(err, "initiating join stream")}
	}
	defer stream.CloseSend()

	// Send the ClientInit message with the intended join method, token name,
	// and system role.
	if err := stream.Send(&messages.ClientInit{
		JoinMethod: joinMethodPtr,
		TokenName:  params.Token,
		SystemRole: params.ID.Role.String(),
	}); err != nil {
		// Failing to send the first message on the stream is always a connection error.
		return nil, &connectionError{trace.Wrap(err, "sending ClientInit message")}
	}

	// Receive the ServerInit message.
	serverInit, err := messages.RecvResponse[*messages.ServerInit](stream)
	if err != nil {
		err = trace.Wrap(err, "receiving ServerInit message")
		if !trace.IsAccessDenied(err) && !trace.IsBadParameter(err) {
			// Any unrecognized error reading the first response on the stream
			// is likely to be a connection error.
			err = &connectionError{err}
		}
		return nil, err
	}

	// Generate keys based on the signature algorithm suite from the ServerInit message.
	signer, publicKeys, err := GenerateKeys(ctx, serverInit.SignatureAlgorithmSuite)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Build the ClientParams message that will be sent for all join methods.
	clientParams := makeClientParams(params, publicKeys)

	// Delegate out to the handler for the specific join method.
	resultMsg, err := joinWithMethod(ctx, stream, params, clientParams, serverInit.JoinMethod)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Convert the result message into a JoinResult.
	switch typedResult := resultMsg.(type) {
	case *messages.HostResult:
		return makeJoinResult(signer, typedResult.Certificates, typedResult.ImmutableLabels)
	case *messages.BotResult:
		joinResult, err := makeJoinResult(signer, typedResult.Certificates, nil)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if typedResult.BoundKeypairResult != nil {
			joinResult.BoundKeypair = &authjoin.BoundKeypairRegisterResult{
				BoundPublicKey: string(typedResult.BoundKeypairResult.PublicKey),
				JoinState:      typedResult.BoundKeypairResult.JoinState,
			}
		}
		return joinResult, nil
	default:
		return nil, trace.BadParameter("unhandled result message type %T", resultMsg)
	}
}

func joinWithMethod(
	ctx context.Context,
	stream messages.ClientStream,
	joinParams JoinParams,
	clientParams messages.ClientParams,
	method string,
) (messages.Response, error) {
	var err error

	switch types.JoinMethod(method) {
	case types.JoinMethodAzure:
		return azureJoin(ctx, stream, joinParams, clientParams)
	case types.JoinMethodAzureDevops:
		if joinParams.IDToken == "" {
			joinParams.IDToken, err = azuredevops.NewIDTokenSource(os.Getenv).GetIDToken(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
		return oidcJoin(stream, joinParams, clientParams)
	case types.JoinMethodBitbucket:
		// Tests may specify their own IDToken, so only overwrite it when empty.
		if joinParams.IDToken == "" {
			joinParams.IDToken, err = bitbucket.NewIDTokenSource(os.Getenv).GetIDToken()
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
		return oidcJoin(stream, joinParams, clientParams)
	case types.JoinMethodBoundKeypair:
		return boundKeypairJoin(ctx, stream, joinParams, clientParams)
	case types.JoinMethodCircleCI:
		if joinParams.IDToken == "" {
			joinParams.IDToken, err = circleci.GetIDToken(os.Getenv)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
		return oidcJoin(stream, joinParams, clientParams)
	case types.JoinMethodIAM:
		return iamJoin(ctx, stream, joinParams, clientParams)
	case types.JoinMethodEC2:
		return ec2Join(ctx, stream, joinParams, clientParams)
	case types.JoinMethodEnv0:
		// Tests may specify their own IDToken, so only overwrite it when empty.
		if joinParams.IDToken == "" {
			joinParams.IDToken, err = env0.NewIDTokenSource(os.Getenv).GetIDToken()
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
		return oidcJoin(stream, joinParams, clientParams)
	case types.JoinMethodOracle:
		return oracleJoin(ctx, stream, joinParams, clientParams)
	case types.JoinMethodGCP:
		if joinParams.IDToken == "" {
			joinParams.IDToken, err = gcp.GetIDToken(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
		return oidcJoin(stream, joinParams, clientParams)
	case types.JoinMethodGitHub:
		if joinParams.IDToken == "" {
			joinParams.IDToken, err = githubactions.NewIDTokenSource().GetIDToken(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
		return oidcJoin(stream, joinParams, clientParams)
	case types.JoinMethodGitLab:
		if joinParams.IDToken == "" {
			joinParams.IDToken, err = gitlab.NewIDTokenSource(gitlab.IDTokenSourceConfig{
				EnvVarName: joinParams.GitlabParams.EnvVarName,
			}).GetIDToken()
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
		return oidcJoin(stream, joinParams, clientParams)
	case types.JoinMethodKubernetes:
		if joinParams.IDToken == "" {
			joinParams.IDToken, err = kubetoken.GetIDToken(os.Getenv, joinParams.KubernetesReadFileFunc)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
		return oidcJoin(stream, joinParams, clientParams)
	case types.JoinMethodSpacelift:
		if joinParams.IDToken == "" {
			joinParams.IDToken, err = spacelift.NewIDTokenSource(os.Getenv).GetIDToken()
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
		return oidcJoin(stream, joinParams, clientParams)
	case types.JoinMethodToken:
		return tokenJoin(stream, clientParams, joinParams.TokenSecret)
	case types.JoinMethodTPM:
		return tpmJoin(ctx, stream, joinParams, clientParams)
	case types.JoinMethodTerraformCloud:
		if joinParams.IDToken == "" {
			joinParams.IDToken, err = terraformcloud.NewIDTokenSource(joinParams.TerraformCloudAudienceTag, os.Getenv).GetIDToken()
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
		return oidcJoin(stream, joinParams, clientParams)
	default:
		sendGivingUpErr := stream.Send(&messages.GivingUp{
			Reason: messages.GivingUpReasonUnsupportedJoinMethod,
			Msg:    "join method " + method + " is not supported by this client",
		})
		return nil, trace.NewAggregate(
			trace.NotImplemented("server selected join method %v which is not supported by this client", method),
			trace.Wrap(sendGivingUpErr, "sending GivingUp message to server"),
		)
	}
}

func tokenJoin(
	stream messages.ClientStream,
	clientParams messages.ClientParams,
	secret string,
) (messages.Response, error) {
	// The token join method is relatively simple, the flow is
	//
	// client->server ClientInit
	// client<-server ServerInit
	// client->server Tokeninit
	// client<-server Result
	//
	// At this point the ServerInit message has already been received, all
	// that's left is to send the TokenInit message and receive the final result.
	tokenInitMsg := &messages.TokenInit{
		ClientParams: clientParams,
		Secret:       secret,
	}
	if err := stream.Send(tokenInitMsg); err != nil {
		return nil, trace.Wrap(err)
	}
	// Receive and return the final result.
	result, err := stream.Recv()
	return result, trace.Wrap(err)
}

func makeClientParams(params JoinParams, publicKeys *messages.PublicKeys) messages.ClientParams {
	if params.ID.Role == types.RoleBot {
		return messages.ClientParams{
			BotParams: &messages.BotParams{
				PublicKeys: *publicKeys,
				Expires:    params.Expires,
			},
		}
	}
	return messages.ClientParams{
		HostParams: &messages.HostParams{
			PublicKeys:           *publicKeys,
			HostName:             params.ID.NodeName,
			AdditionalPrincipals: params.AdditionalPrincipals,
			DNSNames:             params.DNSNames,
		},
	}
}

func makeJoinResult(signer crypto.Signer, certs messages.Certificates, immutableLabels *joiningv1.ImmutableLabels) (*JoinResult, error) {
	// Callers expect proto.Certs with PEM-formatted TLS certs and
	// authorized_keys formated SSH certs/keys.
	sshCert, err := toAuthorizedKey(certs.SSHCert)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sshCAKeys, err := toAuthorizedKeys(certs.SSHCAKeys)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &JoinResult{
		Certs: &proto.Certs{
			TLS:        pemEncodeTLSCert(certs.TLSCert),
			TLSCACerts: pemEncodeTLSCerts(certs.TLSCACerts),
			SSH:        sshCert,
			SSHCACerts: sshCAKeys, // SSHCACerts is a misnomer, SSH CAs are just public keys.
		},
		PrivateKey:      signer,
		ImmutableLabels: immutableLabels,
	}, nil
}

func toAuthorizedKeys(wireFormats [][]byte) ([][]byte, error) {
	out := make([][]byte, len(wireFormats))
	for i, wireFormat := range wireFormats {
		var err error
		out[i], err = toAuthorizedKey(wireFormat)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return out, nil
}

func toAuthorizedKey(wireFormat []byte) ([]byte, error) {
	sshPub, err := ssh.ParsePublicKey(wireFormat)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ssh.MarshalAuthorizedKey(sshPub), nil
}

func pemEncodeTLSCerts(rawCerts [][]byte) [][]byte {
	out := make([][]byte, len(rawCerts))
	for i, rawCert := range rawCerts {
		out[i] = pemEncodeTLSCert(rawCert)
	}
	return out
}

func pemEncodeTLSCert(rawCert []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: rawCert,
	})
}

// GenerateKeys generates host keys appropriate for a cluster join request
// according to the cluster's configured signature algorithm suite.
func GenerateKeys(ctx context.Context, suite types.SignatureAlgorithmSuite) (crypto.Signer, *messages.PublicKeys, error) {
	signer, err := cryptosuites.GenerateKey(
		ctx,
		cryptosuites.StaticAlgorithmSuite(suite),
		cryptosuites.HostIdentity,
	)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	tlsPub, err := x509.MarshalPKIXPublicKey(signer.Public())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	sshPub, err := ssh.NewPublicKey(signer.Public())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return signer, &messages.PublicKeys{
		PublicTLSKey: tlsPub,
		PublicSSHKey: sshPub.Marshal(),
	}, nil
}

type connectionError struct {
	wrapped error
}

func (e *connectionError) Error() string {
	return e.wrapped.Error()
}

func (e *connectionError) Unwrap() error {
	return e.wrapped
}

func isConnectionError(err error) bool {
	var ce *connectionError
	return errors.As(err, &ce)
}

// LegacyJoinError is returned when the join attempt failed while attempting to
// join via the legacy join service.
type LegacyJoinError struct {
	wrapped error
}

func (e *LegacyJoinError) Error() string {
	return e.wrapped.Error()
}

func (e *LegacyJoinError) Unwrap() error {
	return e.wrapped
}
