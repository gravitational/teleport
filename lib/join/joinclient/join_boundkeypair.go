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
			signer, err := bkParams.GetSigner(string(kind.PublicKey))
			if err != nil {
				return nil, trace.Wrap(err, "could not lookup signer for public key %+v", string(kind.PublicKey))
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

			jws, err := joseSigner.Sign([]byte(kind.Challenge))
			if err != nil {
				return nil, trace.Wrap(err, "signing challenge")
			}

			serialized, err := jws.CompactSerialize()
			if err != nil {
				return nil, trace.Wrap(err, "serializing signed challenge")
			}
			if err := stream.Send(&messages.BoundKeypairChallengeSolution{
				Solution: []byte(serialized),
			}); err != nil {
				return nil, trace.Wrap(err)
			}
		case *messages.BoundKeypairRotationRequest:
			if bkParams.RequestNewKeypair == nil {
				return nil, trace.BadParameter("RequestNewKeypair is required")
			}

			slog.InfoContext(ctx, "Server has requested keypair rotation", "suite", kind.SignatureAlgorithmSuite)

			suite, err := types.SignatureAlgorithmSuiteFromString(kind.SignatureAlgorithmSuite)
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
			if err := stream.Send(&messages.BoundKeypairRotationResponse{
				PublicKey: []byte(newPubkey),
			}); err != nil {
				return nil, trace.Wrap(err)
			}
		}
	}
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
