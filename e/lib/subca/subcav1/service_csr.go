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
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/subca"
)

func (s *Service) requestAsyncCSRs(
	ctx context.Context,
	id types.CertAuthorityOverrideID,
	customSubject []pkix.AttributeTypeAndValue,
	publicKeys []string,
) ([]*subcav1.CertificateSigningRequest, []*subcav1.CreateCSRWarning, error) {
	// Convert the normalized ATVs back to proto.
	// Don't take the CreateCSRRequest subject, it's not normalized.
	var customSubjectPB *subcav1.DistinguishedName
	if customSubject != nil {
		// An RDNSequence created by pkix.Name.ToRDNSequence() has one RDNSET per
		// attribute. The subca.RDNSequenceToDistinguishedNameProto() helper
		// flattens the sets into the proto, so we don't need to mimic that exactly
		// here.
		rdns := pkix.RDNSequence{customSubject}

		var err error
		customSubjectPB, err = subca.RDNSequenceToDistinguishedNameProto(rdns)
		if err != nil {
			return nil, nil, trace.Wrap(err, "convert custom subject to proto")
		}
	}

	// Register CSR requests for watchers.
	requestID := uuid.NewString()
	pendingReq, err := s.pendingCSR.CreatePendingCSRRequest(
		ctx, newPendingCSRRequest(id, requestID, customSubjectPB, publicKeys))
	if err != nil {
		return nil, nil, trace.Wrap(err, "create pending CSR request")
	}

	// Cleanup CSR request before returning.
	defer func() {
		// Do not use the request context to cleanup, otherwise a client cancel will
		// accumulate storage entries (until they expire).
		ctx := context.Background()
		if err := s.pendingCSR.DeletePendingCSRRequest(ctx, requestID); err != nil {
			s.logger.WarnContext(ctx, "Failed to cleanup PendingCSRRequest",
				"error", err,
				"ca_type", id.CAType,
				"cluster_name", id.ClusterName,
				"request_id", requestID,
			)
		}
	}()

	// Prepare a timed context.
	const maxWatcherWait = 5 * time.Minute // Arbitrary.
	wCtx, wCancel := context.WithTimeout(ctx, maxWatcherWait)
	defer wCancel()
	var wErr error // Captures unexpected watcher errors.

	// Watch the PendingCSRRequest.
	if err := s.runPendingCSRWatcher(
		wCtx,
		requestID,
		// onInit.
		func(*types.Event) {
			current, err := s.pendingCSR.GetPendingCSRRequest(ctx, requestID)
			if err != nil {
				wErr = trace.Wrap(err, "read pending CSR request")
				wCancel()
				return
			}
			pendingReq = current
			if _, ok := csrsDone(pendingReq, publicKeys); ok {
				wCancel()
			}
		},
		// onEvent.
		func(op types.OpType, current *subcav1.PendingCSRRequest) {
			if op == types.OpDelete {
				wErr = trace.Wrap(errors.New("pending CSR request deleted while in-flight"))
				wCancel()
				return
			}
			if op != types.OpPut {
				s.logger.WarnContext(ctx, "Got unexpected event OpType while watching a PendingCSRRequest",
					"op", op,
				)
				return // Unexpected
			}
			pendingReq = current
			if _, ok := csrsDone(pendingReq, publicKeys); ok {
				wCancel()
			}
		},
	); err != nil {
		return nil, nil, trace.Wrap(err, "create CSR watcher")
	}
	if wErr != nil {
		return nil, nil, trace.Wrap(wErr, "watcher")
	}

	var csrs []*subcav1.CertificateSigningRequest
	var warns []*subcav1.CreateCSRWarning

	// Collect CSRs and warnings.
	for pkh, pendingCSR := range pendingReq.GetStatus().GetPublicKeyHashToPendingCsr() {
		if codes.Code(pendingCSR.GetStatus().GetCode()) == codes.OK {
			csrs = append(csrs, pendingCSR.GetCsr())
			continue
		}
		warns = append(warns, subcav1.CreateCSRWarning_builder{
			UserMessage:   pendingCSR.GetStatus().GetMessage(),
			PublicKeyHash: pkh,
		}.Build())
	}

	// Did we fulfill all requests? Warn if not.
	if missing, ok := csrsDone(pendingReq, publicKeys); !ok {
		warns = appendMissingKeyWarnings(
			warns,
			missing,
			"remote signing request not fulfilled before timeout",
		)
	}

	return csrs, warns, nil
}

func newPendingCSRRequest(
	overrideID types.CertAuthorityOverrideID,
	requestID string,
	customSubject *subcav1.DistinguishedName,
	publicKeys []string) *subcav1.PendingCSRRequest {
	pkhs := make([]*subcav1.PublicKeyHash, len(publicKeys))
	for i, pubKey := range publicKeys {
		pkhs[i] = subcav1.PublicKeyHash_builder{Value: pubKey}.Build()
	}
	return subcav1.PendingCSRRequest_builder{
		Kind:    types.KindPendingCSRRequest,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: requestID,
		}.Build(),
		Spec: subcav1.PendingCSRRequestSpec_builder{
			ClusterName:     overrideID.ClusterName,
			CaType:          overrideID.CAType,
			CustomSubject:   customSubject,
			PublicKeyHashes: pkhs,
		}.Build(),
	}.Build()
}

func csrsDone(pendingReq *subcav1.PendingCSRRequest, publicKeys []string) (missing []string, done bool) {
	csrMap := pendingReq.GetStatus().GetPublicKeyHashToPendingCsr()
	for _, pkh := range publicKeys {
		if _, ok := csrMap[pkh]; !ok {
			missing = append(missing, pkh)
		}
	}
	return missing, len(missing) == 0
}

func (s *Service) fulfillPendingCSRRequest(
	ctx context.Context,
	getParsedCA getParsedCAFunc,
	pendingReq *subcav1.PendingCSRRequest,
	generatedCSRCache map[string]*subcav1.PendingCSR,
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

	var customSubject []pkix.AttributeTypeAndValue
	if subj := spec.GetCustomSubject(); subj != nil {
		rdns, err := subca.DistinguishedNameProtoToRDNSequence(subj)
		if err != nil {
			// Unexpected, storage validates the subject.
			return false, trace.Wrap(err, "parse custom_subject")
		}
		customSubject = flattenRDNSequence(rdns)
	}

	var errs []error
	for i, pkh := range spec.GetPublicKeyHashes() {
		if _, ok := csrMap[pkh.GetValue()]; ok {
			continue
		}

		if pendingCSR, ok := generatedCSRCache[pkh.GetValue()]; ok {
			logger.DebugContext(ctx, "Using pre-calculated CSR",
				"pkh", pkh.GetValue(),
			)
			csrMap[pkh.GetValue()] = pendingCSR
			changed = true
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
			// Save successful CSRs to the cache in case the update gets retried.
			if pendingCSR.HasCsr() {
				generatedCSRCache[pkh.GetValue()] = pendingCSR
			}
			csrMap[pkh.GetValue()] = pendingCSR
			changed = true
		}
	}
	return changed, trace.NewAggregate(errs...)
}

func (s *Service) processPendingCSR(
	ctx context.Context,
	getParsedCA getParsedCAFunc,
	customSubject []pkix.AttributeTypeAndValue,
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
