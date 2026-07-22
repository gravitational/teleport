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

	"github.com/google/go-attestation/attest"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/join/internal/messages"
)

func tpmJoin(
	ctx context.Context,
	stream messages.ClientStream,
	joinParams JoinParams,
	clientParams messages.ClientParams,
) (messages.Response, error) {
	// The TPM join method involves the following messages:
	//
	// client->server ClientInit
	// client<-server ServerInit
	// client->server TPMInit
	// client<-server TPMEncryptedCredential
	// client->server TPMSolution
	// client<-server Result
	//
	// At this point the ServerInit message has already been received,
	// what's left is to send the TPMInit message, handle the
	// TPMEncryptedCredential->TPMSolution flow, and receive and return the
	// final result.

	log := joinParams.Log

	attestation, close, err := joinParams.AttestTPM(ctx, log)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer func() {
		if err := close(); err != nil {
			log.WarnContext(ctx, "Failed to close TPM", "error", err)
		}
	}()

	tpmInit := &messages.TPMInit{
		ClientParams:      clientParams,
		Public:            attestation.AttestParams.Public,
		CreateData:        attestation.AttestParams.CreateData,
		CreateAttestation: attestation.AttestParams.CreateAttestation,
		CreateSignature:   attestation.AttestParams.CreateSignature,
	}

	// Get the EKKey or EKCert. We want to prefer the EKCert if it is available
	// as this is signed by the manufacturer.
	switch {
	case attestation.Data.EKCert != nil:
		log.DebugContext(ctx, "Using EKCert for TPM registration",
			"ekcert_serial", attestation.Data.EKCert.SerialNumber)
		tpmInit.EKCert = attestation.Data.EKCert.Raw
	case attestation.Data.EKPub != nil:
		log.DebugContext(ctx, "Using EKKey for TPM registration",
			"ekpub_hash", attestation.Data.EKPubHash)
		tpmInit.EKKey = attestation.Data.EKPub
	default:
		return nil, trace.BadParameter("tpm has neither ekkey or ekcert")
	}

	if err := stream.Send(tpmInit); err != nil {
		return nil, trace.Wrap(err, "sending TPMInit")
	}

	encryptedCredential, err := messages.RecvResponse[*messages.TPMEncryptedCredential](stream)
	if err != nil {
		return nil, trace.Wrap(err, "receiving TPMEncryptedCredential")
	}

	solution, err := attestation.Solve(&attest.EncryptedCredential{
		Credential: encryptedCredential.CredentialBlob,
		Secret:     encryptedCredential.Secret,
	})
	if err != nil {
		err = trace.Wrap(err, "activating credential")
		sendGivingUpErr := stream.Send(&messages.GivingUp{
			Reason: messages.GivingUpReasonChallengeSolutionFailed,
			Msg:    err.Error(),
		})
		return nil, trace.NewAggregate(
			err,
			trace.Wrap(sendGivingUpErr, "sending GivingUp message to server"),
		)
	}

	if err := stream.Send(&messages.TPMSolution{
		Solution: solution,
	}); err != nil {
		return nil, trace.Wrap(err, "sending TPMSolution")
	}

	result, err := stream.Recv()
	return result, trace.Wrap(err, "receiving join result")
}
