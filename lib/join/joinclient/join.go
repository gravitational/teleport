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
	"log/slog"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	authjoin "github.com/gravitational/teleport/lib/auth/join"
	proxyinsecureclient "github.com/gravitational/teleport/lib/client/proxy/insecure"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/join/internal/messages"
	"github.com/gravitational/teleport/lib/join/joinv1"
)

type JoinParams = authjoin.RegisterParams
type JoinResult = authjoin.RegisterResult

// Join is used to join a cluster. A host or bot calls this with the name of a
// provision token to get its initial certificates.
func Join(ctx context.Context, params JoinParams) (*JoinResult, error) {
	if err := params.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	slog.InfoContext(ctx, "Trying to join with the new join service")
	result, err := joinNew(ctx, params)
	if trace.IsNotImplemented(err) {
		// Fall back to joining via legacy service.
		slog.InfoContext(ctx, "Falling back to joining via the legacy join service", "error", err)
		result, err := authjoin.Register(ctx, params)
		return result, trace.Wrap(err)
	}
	return result, trace.Wrap(err)
}

func joinNew(ctx context.Context, params JoinParams) (*JoinResult, error) {
	if params.AuthClient != nil {
		return joinViaAuthClient(ctx, params, params.AuthClient)
	}
	if !params.ProxyServer.IsEmpty() {
		return joinViaProxy(ctx, params, params.ProxyServer.String())
	}
	// params.AuthServers could contain auth or proxy addresses, try both.
	// params.CheckAndSetDefaults() asserts that this list is not empty when
	// AuthClient and ProxyServer are both unset.
	if authjoin.LooksLikeProxy(params.AuthServers) {
		proxyAddr := params.AuthServers[0].String()
		slog.InfoContext(ctx, "Attempting to join cluster, address looks like a Proxy", "addr", proxyAddr)
		result, proxyJoinErr := joinViaProxy(ctx, params, proxyAddr)
		if proxyJoinErr == nil {
			return result, nil
		}
		slog.InfoContext(ctx, "Joining via proxy failed, will try to join via Auth", "error", proxyJoinErr)
		result, authJoinErr := joinViaAuth(ctx, params)
		return result, trace.Wrap(authJoinErr)
	}
	addr := params.AuthServers[0].String()
	slog.InfoContext(ctx, "Attempting to join cluster, address looks like an Auth server", "addr", addr)
	result, authJoinErr := joinViaAuth(ctx, params)
	if authJoinErr == nil {
		return result, nil
	}
	slog.InfoContext(ctx, "Joining via auth failed, will try to join via Proxy", "error", authJoinErr)
	result, proxyJoinErr := joinViaProxy(ctx, params, addr)
	return result, trace.Wrap(proxyJoinErr)
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
		return nil, trace.Wrap(err)
	}
	defer conn.Close()
	return joinWithClient(ctx, params, joinv1.NewClientFromConn(conn))
}

func joinViaAuth(ctx context.Context, params JoinParams) (*JoinResult, error) {
	authClient, err := authjoin.NewAuthClient(ctx, params)
	if err != nil {
		return nil, trace.Wrap(err, "building auth client")
	}
	defer authClient.Close()
	return joinViaAuthClient(ctx, params, authClient)
}

func joinViaAuthClient(ctx context.Context, params JoinParams, authClient authjoin.AuthJoinClient) (*JoinResult, error) {
	return joinWithClient(ctx, params, joinv1.NewClient(authClient.JoinV1Client()))
}

func joinWithClient(ctx context.Context, params JoinParams, client *joinv1.Client) (*JoinResult, error) {
	// Clients may specify the join method or not, to let the server choose the
	// method based on the provsion token.
	var joinMethodPtr *string
	switch params.JoinMethod {
	case types.JoinMethodUnspecified:
		// leave joinMethodPtr nil to let the server pick based on the token
	case types.JoinMethodToken:
		joinMethod := string(params.JoinMethod)
		joinMethodPtr = &joinMethod
	default:
		return nil, trace.NotImplemented("new join service is not implemented for method %v", params.JoinMethod)
	}

	// Initiate the join request, using a cancelable context to make sure the
	// stream is closed when this function returns.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stream, err := client.Join(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer stream.CloseSend()

	// Send the ClientInit message with the intended join method, token name,
	// and system role.
	if err := stream.Send(&messages.ClientInit{
		JoinMethod: joinMethodPtr,
		TokenName:  params.Token,
		SystemRole: params.ID.Role.String(),
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	// Receive the ServerInit message.
	serverInit, err := messages.RecvResponse[*messages.ServerInit](stream)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Generate keys based on the signature algorithm suite from the ServerInit message.
	signer, publicKeys, err := generateKeys(ctx, serverInit.SignatureAlgorithmSuite)
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
		return makeJoinResult(signer, typedResult.Certificates)
	case *messages.BotResult:
		joinResult, err := makeJoinResult(signer, typedResult.Certificates)
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
	switch types.JoinMethod(method) {
	case types.JoinMethodToken:
		return tokenJoin(stream, clientParams)
	case types.JoinMethodBoundKeypair:
		return boundKeypairJoin(ctx, stream, joinParams, clientParams)
	default:
		// TODO(nklaassen): implement remaining join methods.
		return nil, trace.NotImplemented("server selected join method %v which is not supported by this client", method)
	}
}

func tokenJoin(
	stream messages.ClientStream,
	clientParams messages.ClientParams,
) (messages.Response, error) {
	// The token join method is relatively simple, the flow is
	//
	// client->server ClientInit
	// client<-server ServerInit
	// client->server Tokeninit
	// client<-server Result
	//
	// At this point the ServerInit messages has already been received, all
	// that's left is to send the TokenInit message and receive the final result.
	tokenInitMsg := &messages.TokenInit{
		ClientParams: clientParams,
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

func makeJoinResult(signer crypto.Signer, certs messages.Certificates) (*JoinResult, error) {
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
		PrivateKey: signer,
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

func generateKeys(ctx context.Context, suite types.SignatureAlgorithmSuite) (crypto.Signer, *messages.PublicKeys, error) {
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
