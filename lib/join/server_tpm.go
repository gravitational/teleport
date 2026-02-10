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

package join

import (
	"github.com/google/go-attestation/attest"
	"github.com/gravitational/trace"

	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/join/internal/authz"
	"github.com/gravitational/teleport/lib/join/internal/diagnostic"
	"github.com/gravitational/teleport/lib/join/internal/messages"
	"github.com/gravitational/teleport/lib/join/provision"
	"github.com/gravitational/teleport/lib/join/tpmjoin"
)

// handleTPMJoin handles join attempts for the TPM join method.
//
// The TPM join method involves the following messages:
//
// client->server ClientInit
// client<-server ServerInit
// client->server TPMInit
// client<-server TPMEncryptedCredential
// client->server TPMSolution
// client<-server Result
//
// At this point the ServerInit message has already been sent, what's left is
// to receive the TPMInit message, handle the TPMEncryptedCredential->TPMSolution
// flow, and return the final result if everything checks out.
func (s *Server) handleTPMJoin(
	stream messages.ServerStream,
	authCtx *authz.Context,
	clientInit *messages.ClientInit,
	provisionToken provision.Token,
) (messages.Response, error) {
	ptv2, ok := provisionToken.(*types.ProvisionTokenV2)
	if !ok {
		return nil, trace.BadParameter("TPM joining only supports types.ProvisionTokenV2, got %T", provisionToken)
	}

	// Receive the TPMInit message from the client.
	tpmInit, err := messages.RecvRequest[*messages.TPMInit](stream)
	if err != nil {
		return nil, trace.Wrap(err, "receiving TPMInit message")
	}
	// Set any diagnostic info from the ClientParams.
	setDiagnosticClientParams(stream.Diagnostic(), &tpmInit.ClientParams)

	solve := func(ec *attest.EncryptedCredential) ([]byte, error) {
		ecMsg := &messages.TPMEncryptedCredential{
			CredentialBlob: ec.Credential,
			Secret:         ec.Secret,
		}
		if err := stream.Send(ecMsg); err != nil {
			return nil, trace.Wrap(err, "sending TPMEncryptedCredential")
		}
		solutionMsg, err := messages.RecvRequest[*messages.TPMSolution](stream)
		if err != nil {
			return nil, trace.Wrap(err, "receiving TPMSolution")
		}
		return solutionMsg.Solution, nil
	}

	validatedEK, err := tpmjoin.CheckTPMRequest(stream.Context(), tpmjoin.CheckTPMRequestParams{
		Token:        ptv2,
		TPMValidator: s.cfg.AuthService.GetTPMValidator(),
		EKCert:       tpmInit.EKCert,
		EKKey:        tpmInit.EKKey,
		AttestParams: attest.AttestationParameters{
			Public:            tpmInit.Public,
			CreateData:        tpmInit.CreateData,
			CreateAttestation: tpmInit.CreateAttestation,
			CreateSignature:   tpmInit.CreateSignature,
		},
		Solve: solve,
	})
	// validatedEK will be returned even on error if the TPM was validated but
	// no allow rules were matched, include it in the diagnostic for debugging.
	stream.Diagnostic().Set(func(info *diagnostic.Info) {
		info.RawJoinAttrs = validatedEK
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Make and return the final result message.
	result, err := s.makeResult(
		stream.Context(),
		stream.Diagnostic(),
		authCtx,
		clientInit,
		&tpmInit.ClientParams,
		provisionToken,
		validatedEK,
		&workloadidentityv1.JoinAttrs{
			Tpm: validatedEK.JoinAttrs(),
		},
	)
	return result, trace.Wrap(err)
}

type TPMValidator = tpmjoin.TPMValidator
