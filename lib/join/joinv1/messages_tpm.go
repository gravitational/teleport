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

package joinv1

import (
	"github.com/gravitational/trace"

	joinv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/join/v1"
	"github.com/gravitational/teleport/lib/join/internal/messages"
)

func tpmInitToMessage(req *joinv1.TPMInit) (*messages.TPMInit, error) {
	clientParams, err := clientParamsToMessage(req.ClientParams)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &messages.TPMInit{
		ClientParams:      clientParams,
		Public:            req.Public,
		CreateData:        req.CreateData,
		CreateAttestation: req.CreateAttestation,
		CreateSignature:   req.CreateSignature,
		EKCert:            req.GetEkCert(),
		EKKey:             req.GetEkKey(),
	}, nil
}

func tpmInitFromMessage(msg *messages.TPMInit) (*joinv1.TPMInit, error) {
	clientParams, err := clientParamsFromMessage(msg.ClientParams)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req := &joinv1.TPMInit{
		ClientParams:      clientParams,
		Public:            msg.Public,
		CreateData:        msg.CreateData,
		CreateAttestation: msg.CreateAttestation,
		CreateSignature:   msg.CreateSignature,
	}
	var (
		hasEKCert = len(msg.EKCert) > 0
		hasEKKey  = len(msg.EKKey) > 0
	)
	switch {
	case hasEKCert == hasEKKey:
		return nil, trace.BadParameter("exactly one of EKCert and EKKey must be set")
	case hasEKCert:
		req.Ek = &joinv1.TPMInit_EkCert{
			EkCert: msg.EKCert,
		}
	case hasEKKey:
		req.Ek = &joinv1.TPMInit_EkKey{
			EkKey: msg.EKKey,
		}
	}
	return req, nil
}

func tpmEncryptedCredentialToMessage(req *joinv1.TPMEncryptedCredential) *messages.TPMEncryptedCredential {
	return &messages.TPMEncryptedCredential{
		CredentialBlob: req.CredentialBlob,
		Secret:         req.Secret,
	}
}

func tpmEncryptedCredentialFromMessage(msg *messages.TPMEncryptedCredential) *joinv1.TPMEncryptedCredential {
	return &joinv1.TPMEncryptedCredential{
		CredentialBlob: msg.CredentialBlob,
		Secret:         msg.Secret,
	}
}

func tpmSolutionToMessage(req *joinv1.TPMSolution) *messages.TPMSolution {
	return &messages.TPMSolution{
		Solution: req.Solution,
	}
}

func tpmSolutionFromMessage(msg *messages.TPMSolution) *joinv1.TPMSolution {
	return &joinv1.TPMSolution{
		Solution: msg.Solution,
	}
}
