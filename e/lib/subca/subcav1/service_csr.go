// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package subcav1

import (
	"context"
	"crypto/x509/pkix"
	"fmt"

	"github.com/gravitational/trace"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"

	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/lib/subca"
)

func (s *Service) fulfillPendingCSRRequest(
	ctx context.Context,
	getParsedCA getParsedCAFunc,
	pendingReq *subcav1.PendingCSRRequest,
) (changed bool, _ error) {
	reqID := pendingReq.GetMetadata().GetName()
	logger := s.logger.With(
		"request_id", reqID,
		"ca_type", pendingReq.GetSpec().GetCaType(),
	)

	spec := pendingReq.GetSpec()

	// Initialize status and the csrMap.
	status := pendingReq.GetStatus()
	if status == nil {
		status = &subcav1.PendingCSRRequestStatus{}
		pendingReq.SetStatus(status)
	}
	csrMap := status.GetPublicKeyHashToPendingCsr()
	if csrMap == nil {
		csrMap = map[string]*subcav1.PendingCSR{}
		status.SetPublicKeyHashToPendingCsr(csrMap)
	}

	var customSubject *pkix.Name
	if subj := spec.GetCustomSubject(); subj != nil {
		rdns, err := subca.DistinguishedNameProtoToRDNSequence(subj)
		if err != nil {
			// Unexpected, storage validates the subject.
			return false, trace.Wrap(err, "parse custom_subject")
		}
		customSubject = &pkix.Name{}
		customSubject.FillFromRDNSequence(&rdns)
	}

	var errs []error
	for i, pkh := range spec.GetPublicKeyHashes() {
		if _, ok := csrMap[pkh.GetValue()]; ok {
			continue
		}

		pendingCSR, err := s.processPendingCSR(ctx, getParsedCA, customSubject, pkh.GetValue())
		if err != nil {
			errs = append(errs, fmt.Errorf("request %v, pending CSR %d: %w", reqID, i, err))
			continue
		}
		if pendingCSR != nil {
			logger.DebugContext(ctx, "Processed pending CSR request",
				"pkh", pkh.GetValue(),
			)
			csrMap[pkh.GetValue()] = pendingCSR
			changed = true
		}
	}
	return changed, trace.NewAggregate(errs...)
}

func (s *Service) processPendingCSR(
	ctx context.Context,
	getParsedCA getParsedCAFunc,
	customSubject *pkix.Name,
	pkh string,
) (*subcav1.PendingCSR, error) {
	ca, err := getParsedCA()
	if err != nil {
		return nil, trace.Wrap(err, "read parsed CA")
	}

	candidateResp, err := s.getCandidateCSRSigners(ctx, ca.CA, pkh)
	if err != nil {
		return nil, trace.Wrap(err, "prepare signers")
	}
	if len(candidateResp.Signers) == 0 {
		return nil, nil
	}
	// There is at most one signer, because we target by public key hash.
	signer := candidateResp.Signers[0]

	csr, err := createCSR(signer, customSubject)
	if err != nil {
		return subcav1.PendingCSR_builder{
			Status: &status.Status{
				Code:    int32(codes.Internal),
				Message: "signer: " + err.Error(),
			},
		}.Build(), nil
	}

	return subcav1.PendingCSR_builder{
		Status: &status.Status{}, // zero == OK
		Csr:    csr,
	}.Build(), nil
}
