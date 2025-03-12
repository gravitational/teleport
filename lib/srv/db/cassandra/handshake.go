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

package cassandra

import (
	"context"

	"github.com/aws/aws-sigv4-auth-cassandra-gocql-driver-plugin/sigv4"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/gocql/gocql"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/srv/db/cassandra/protocol"
	"github.com/gravitational/teleport/lib/srv/db/common"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

const (
	passwordAuthenticator = "org.apache.cassandra.auth.PasswordAuthenticator"
)

// HandleAuthResponse implements the Cassandra handshake.
// The example Cassandra handshake looks like follows:
//
// Client -> Server: Options
// Server <- Client: Supported
// Client -> Server: Startup
// Server <- Client: Authenticate
// Client -> Server: AuthResponse
// Server <- Client: Ready/ErrorResponse/AuthSuccess
type handshakeHandler interface {
	handleHandshake(ctx context.Context, clientConn, serverConn *protocol.Conn) error
}

// basicHandshake is a basic handshake handler that does not perform any
// additional flow but validates the username received in the AuthResponse.
type basicHandshake struct {
	ses *common.Session
}

func (pp *basicHandshake) handleHandshake(_ context.Context, clientConn, serverConn *protocol.Conn) error {
	for {
		// Read a packet from the cassandra client.
		req, err := clientConn.ReadPacket()
		if err != nil {
			return trace.Wrap(err)
		}

		// Pass all packets to the server except AuthResponse.
		// In case of AuthResponse, validate the username and send
		// an error message to the client if the username is invalid
		// Otherwise, pass the AuthResponse to the server.
		if req.Header().OpCode == primitive.OpCodeAuthResponse {
			if err := handleAuthResponse(clientConn, pp.ses, req); err != nil {
				return trace.Wrap(err)
			}
		}

		// Forward the packet to the server.
		if err := serverConn.WriteFrame(req.Frame()); err != nil {
			return trace.Wrap(err)
		}

		// Read a response from the server.
		rcv, err := serverConn.ReadPacket()
		if err != nil {
			return trace.Wrap(err)
		}
		// Forward the response from the server to the client.
		if _, err := clientConn.Write(rcv.Raw()); err != nil {
			return trace.Wrap(err)
		}
		switch rcv.Header().OpCode {
		case primitive.OpCodeReady, primitive.OpCodeAuthSuccess:
			// If the server responded with Ready or AuthSuccess, the handshake
			// was complete. Return to the caller to allow audit events by another handler.
			return nil
		}
	}
}

// failedHandshake triggers a cassandra handshake without sending any packets the cassandra server.
// This is used to trigger an authentication error message to the client.
type failedHandshake struct {
	// err is the error to send to the client.
	// If err is AccessDenied, the client will receive an AuthenticationError
	// otherwise, the client will receive an ErrorResponse.
	error error
}

// handleHandshake triggers a cassandra handshake without sending any packets the cassandra server.
//
// Client -> Engine: Options
// Client <- Engine: Supported
// Client -> Engine: Startup
// Client <- Engine: Authenticate
// Client -> Engine: AuthResponse
// Client <- Engine: ErrorResponse/AuthenticationError
func (h failedHandshake) handshake(clientConn, _ *protocol.Conn) error {
	for {
		packet, err := clientConn.ReadPacket()
		if err != nil {
			return trace.Wrap(err)
		}

		switch packet.Header().OpCode {
		case primitive.OpCodeStartup:
			fr := frame.NewFrame(
				packet.Header().Version,
				packet.Header().StreamId,
				&message.Authenticate{Authenticator: passwordAuthenticator},
			)
			if err := clientConn.WriteFrame(fr); err != nil {
				return trace.Wrap(err)
			}
		case primitive.OpCodeAuthResponse:
			var msg message.Message
			if trace.IsAccessDenied(err) {
				msg = &message.AuthenticationError{ErrorMessage: h.error.Error()}
			} else {
				msg = &message.ServerError{ErrorMessage: h.error.Error()}
			}
			fr := frame.NewFrame(packet.Header().Version, packet.Header().StreamId, msg)
			if err := clientConn.WriteFrame(fr); err != nil {
				return trace.Wrap(err)
			}
			return nil

		case primitive.OpCodeOptions:
			fr := frame.NewFrame(
				packet.Header().Version,
				packet.Header().StreamId,
				&message.Supported{
					Options: map[string][]string{
						"CQL_VERSION": {"3.4.5"},
						"COMPRESSION": {},
					},
				},
			)
			if err := clientConn.WriteFrame(fr); err != nil {
				return trace.Wrap(err)
			}
		}
	}
}

func handleAuthResponse(clientConn *protocol.Conn, ses *common.Session, pkt *protocol.Packet) error {
	msg, ok := pkt.FrameBody().Message.(*message.AuthResponse)
	if !ok {
		return trace.BadParameter("got unexpected packet %T", pkt.FrameBody().Message)
	}
	vErr := validateUsername(ses, msg)
	if vErr == nil {
		return nil
	}
	if err := sendAuthenticationErrorMessage(vErr, clientConn, pkt.Frame()); err != nil {
		return trace.NewAggregate(vErr, err)
	}
	return vErr
}

func sendAuthenticationErrorMessage(authErr error, clientConn *protocol.Conn, incoming *frame.Frame) error {
	authErrMsg := frame.NewFrame(incoming.Header.Version, incoming.Header.StreamId, &message.AuthenticationError{
		ErrorMessage: authErr.Error(),
	})
	if err := clientConn.WriteFrame(authErrMsg); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// authHandler is a handler that performs the Cassandra authentication flow.
type authAWSSigV4Auth struct {
	ses       *common.Session
	awsConfig awsconfig.Provider
}

func (a *authAWSSigV4Auth) getSigV4Authenticator(ctx context.Context) (gocql.Authenticator, error) {
	meta := a.ses.Database.GetAWS()
	roleARN, err := awsutils.BuildRoleARN(a.ses.DatabaseUser, meta.Region, meta.AccountID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// ExternalID should only be used in one of the assumed roles. If the
	// configuration doesn't specify the AssumeRoleARN, it should be used for
	// the database role.
	var dbRoleExternalID string
	if meta.AssumeRoleARN == "" {
		dbRoleExternalID = meta.ExternalID
	}
	awsCfg, err := a.awsConfig.GetConfig(ctx, meta.Region,
		awsconfig.WithAssumeRole(meta.AssumeRoleARN, meta.ExternalID),
		awsconfig.WithAssumeRole(roleARN, dbRoleExternalID),
		awsconfig.WithAmbientCredentials(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cred, err := awsCfg.Credentials.Retrieve(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	auth := sigv4.NewAwsAuthenticator()
	auth.Region = meta.Region
	auth.AccessKeyId = cred.AccessKeyID
	auth.SessionToken = cred.SessionToken
	auth.SecretAccessKey = cred.SecretAccessKey
	return auth, nil
}

func (a *authAWSSigV4Auth) initPasswordAuth(clientConn *protocol.Conn, req *protocol.Packet) (*protocol.Packet, error) {
	authMsg := frame.NewFrame(
		req.Header().Version,
		req.Header().StreamId,
		&message.Authenticate{Authenticator: passwordAuthenticator},
	)
	if err := clientConn.WriteFrame(authMsg); err != nil {
		return nil, trace.Wrap(err)
	}
	pkt, err := clientConn.ReadPacket()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return pkt, nil
}

func (a *authAWSSigV4Auth) handleStartupMessage(ctx context.Context, clientConn, serverConn *protocol.Conn, req *protocol.Packet) error {
	authResp, err := a.initPasswordAuth(clientConn, req)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := handleAuthResponse(clientConn, a.ses, authResp); err != nil {
		return trace.Wrap(err)
	}

	if err := serverConn.WriteFrame(req.Frame()); err != nil {
		return trace.Wrap(err)
	}
	authFrame, err := serverConn.ReadPacket()
	if err != nil {
		return trace.Wrap(err)
	}
	if err := a.handleAuth(ctx, clientConn, serverConn, authFrame.Frame()); err != nil {
		// Likely the agent is not authorized to access AWS resources or
		// the AWS configuration doesn't allow the agent to assume the role.
		userErr := trace.AccessDenied(
			"failed to authenticate to AWS Keyspaces with %s user, contact your administrator for help",
			a.ses.DatabaseUser,
		)
		return trace.NewAggregate(err, sendAuthenticationErrorMessage(userErr, clientConn, authResp.Frame()))
	}

	readyFrame := frame.NewFrame(authResp.Header().Version, authResp.Header().StreamId, &message.AuthSuccess{})
	if err := clientConn.WriteFrame(readyFrame); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// handleHandshake is a handler that performs the Cassandra authentication flow with to AWS Keyspaces using
// AWS Signature V4. The flow is as follows:
// Client -> Engine    AWS: Options
// Client    Engine -> AWS: Options
// Client    Engine <- AWS: Supported
// Client <- Engine    AWS: Supported
// Client -> Engine    AWS: Startup
// Client <- Engine    AWS: Authenticate
// Client -> Engine    AWS: AuthResponse
// Client    Engine -> AWS: Startup
// Client    Engine <- AWS: AuthChallenge
// Client    Engine -> AWS: AuthResponse
// Client    Engine <- AWS: AuthSuccess
// Client <- Engine    AWS: AuthSuccess
func (a *authAWSSigV4Auth) handleHandshake(ctx context.Context, clientConn, serverConn *protocol.Conn) error {
	for {
		req, err := clientConn.ReadPacket()
		if err != nil {
			return trace.Wrap(err)
		}
		switch req.Header().OpCode {
		case primitive.OpCodeStartup:
			if err := a.handleStartupMessage(ctx, clientConn, serverConn, req); err != nil {
				return trace.Wrap(err)
			}
			return nil
		default:
			if err := serverConn.WriteFrame(req.Frame()); err != nil {
				return trace.Wrap(err)
			}
			resp, err := serverConn.ReadPacket()
			if err != nil {
				return trace.Wrap(err)
			}
			if _, err := clientConn.Write(resp.Raw()); err != nil {
				return trace.Wrap(err)
			}
		}
	}
}

// handleAuth is a function that handles the authentication flow with AWS Keyspaces.
// Signature V4 is used to authenticate with AWS Keyspaces where the username is the role ARN.
// STS AWS is used to get temporary credentials for the role.
func (a *authAWSSigV4Auth) handleAuth(ctx context.Context, _, serverConn *protocol.Conn, fr *frame.Frame) error {
	authMsg, ok := fr.Body.Message.(*message.Authenticate)
	if !ok {
		return trace.BadParameter("unexpected message type %T", fr.Body.Message)
	}
	awsAuth, err := a.getSigV4Authenticator(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	data, challenger, err := awsAuth.Challenge([]byte(authMsg.Authenticator))
	if err != nil {
		return trace.Wrap(err)
	}

	authRespFrame := frame.NewFrame(
		fr.Header.Version,
		fr.Header.StreamId,
		&message.AuthResponse{Token: data},
	)
	for {
		if err := serverConn.WriteFrame(authRespFrame); err != nil {
			return trace.Wrap(err)
		}
		rcv, err := serverConn.ReadPacket()
		if err != nil {
			return trace.Wrap(err)
		}

		switch v := rcv.FrameBody().Message.(type) {
		case *message.AuthSuccess:
			if challenger != nil {
				if err = challenger.Success(v.Token); err != nil {
					return trace.Wrap(err)
				}
			}
			return nil
		case *message.AuthChallenge:
			data, challenger, err = challenger.Challenge(v.Token)
			if err != nil {
				return err
			}
			authRespFrame = frame.NewFrame(
				rcv.Header().Version,
				rcv.Header().StreamId,
				&message.AuthResponse{
					Token: data,
				})
		default:
			return trace.BadParameter("unknown frame response during authentication: %v", v)
		}
	}
}
