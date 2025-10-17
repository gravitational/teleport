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
	"log/slog"
	"strings"

	"github.com/go-jose/go-jose/v3"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/types"
	authjoin "github.com/gravitational/teleport/lib/auth/join"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/join/internal/messages"
	"github.com/gravitational/teleport/lib/jwt"
)

type (
	BoundKeypairParams = authjoin.BoundKeypairParams
	GetSignerFunc      = authjoin.GetSignerFunc
	KeygenFunc         = authjoin.KeygenFunc
	BoundKeypairResult = authjoin.BoundKeypairRegisterResult
)

func boundKeypairJoin(
	ctx context.Context,
	stream messages.ClientStream,
	joinParams JoinParams,
	clientParams messages.ClientParams,
) (messages.Response, error) {
	// The bound keypair join method is relatively complex compared to other
	// join methods, the flow is:
	//
	// client->server ClientInit
	// client<-server ServerInit
	// client->server BoundKeypairInit
	// client<-server BoundKeypairChallenge
	// client->server BoundKeypairChallengeSolution
	//   (optional additional steps if keypair rotation is required)
	//   client<-server: BoundKeypairRotationRequest
	//   client->server: BoundKeypairRotationResponse
	//   client<-server: BoundKeypairChallenge
	//   client->server: BoundKeypairChallengeSolution
	// client<-server: Result containing BoundKeypairResult
	//
	// At this point the ServerInit message has already been received, this
	// function needs to send the BoundKeyPairInit and then handle any
	// challenges and rotation requests.
	boundKeypairInit := &messages.BoundKeypairInit{
		ClientParams:      clientParams,
		InitialJoinSecret: joinParams.BoundKeypairParams.RegistrationSecret,
		PreviousJoinState: joinParams.BoundKeypairParams.PreviousJoinState,
	}
	if err := stream.Send(boundKeypairInit); err != nil {
		return nil, trace.Wrap(err)
	}

	bkParams := joinParams.BoundKeypairParams
	for {
		msg, err := stream.Recv()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		switch kind := msg.(type) {
		case *messages.BotResult:
			// Return the final result.
			return kind, nil
		case *messages.BoundKeypairChallenge:
			solution, err := solveBoundKeypairChallenge(bkParams, kind)
			if err != nil {
				sendGivingUpErr := stream.Send(&messages.GivingUp{
					Reason: messages.GivingUpReasonChallengeSolutionFailed,
					Msg:    err.Error(),
				})
				return nil, trace.NewAggregate(
					err,
					trace.Wrap(sendGivingUpErr, "sending GivingUp message to server"),
				)
			}
			if err := stream.Send(&messages.BoundKeypairChallengeSolution{
				Solution: solution,
			}); err != nil {
				return nil, trace.Wrap(err)
			}
		case *messages.BoundKeypairRotationRequest:
			newPubkey, err := handleBoundKeypairRotationRequest(ctx, bkParams, kind)
			if err != nil {
				sendGivingUpErr := stream.Send(&messages.GivingUp{
					Reason: messages.GivingUpReasonChallengeSolutionFailed,
					Msg:    err.Error(),
				})
				return nil, trace.NewAggregate(
					err,
					trace.Wrap(sendGivingUpErr, "sending GivingUp message to server"),
				)
			}
			if err := stream.Send(&messages.BoundKeypairRotationResponse{
				PublicKey: newPubkey,
			}); err != nil {
				return nil, trace.Wrap(err)
			}
		default:
			return nil, trace.Errorf("server sent unexpected message type %T", msg)
		}
	}
}

func solveBoundKeypairChallenge(bkParams *BoundKeypairParams, challengeMsg *messages.BoundKeypairChallenge) ([]byte, error) {
	signer, err := bkParams.GetSigner(string(challengeMsg.PublicKey))
	if err != nil {
		return nil, trace.Wrap(err, "could not lookup signer for public key %s", string(challengeMsg.PublicKey))
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

	jws, err := joseSigner.Sign([]byte(challengeMsg.Challenge))
	if err != nil {
		return nil, trace.Wrap(err, "signing challenge")
	}

	serialized, err := jws.CompactSerialize()
	if err != nil {
		return nil, trace.Wrap(err, "serializing signed challenge")
	}
	return []byte(serialized), nil
}

func handleBoundKeypairRotationRequest(ctx context.Context, bkParams *BoundKeypairParams, rotationRequest *messages.BoundKeypairRotationRequest) ([]byte, error) {
	if bkParams.RequestNewKeypair == nil {
		return nil, trace.BadParameter("RequestNewKeypair is required")
	}

	slog.InfoContext(ctx, "Server has requested keypair rotation", "suite", rotationRequest.SignatureAlgorithmSuite)

	suite, err := types.SignatureAlgorithmSuiteFromString(rotationRequest.SignatureAlgorithmSuite)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	newSigner, err := bkParams.RequestNewKeypair(ctx, cryptosuites.StaticAlgorithmSuite(suite))
	if err != nil {
		return nil, trace.Wrap(err, "requesting new keypair")
	}

	newPubkey, err := sshPubKeyFromSigner(newSigner)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []byte(newPubkey), nil
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
