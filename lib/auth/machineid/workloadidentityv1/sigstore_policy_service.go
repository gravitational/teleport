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

package workloadidentityv1

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
)

// NewSigstorePolicyResourceService returns the Community Edition version of the
// gRPC service for managing SigstorePolicy resources.
//
// It returns a licensing error from every RPC.
func NewSigstorePolicyResourceService() workloadidentityv1.SigstorePolicyResourceServiceServer {
	return sigstorePolicyResourceService{}
}

type sigstorePolicyResourceService struct {
	workloadidentityv1.UnimplementedSigstorePolicyResourceServiceServer
}

func (s sigstorePolicyResourceService) CreateSigstorePolicy(context.Context, *workloadidentityv1.CreateSigstorePolicyRequest) (*workloadidentityv1.SigstorePolicy, error) {
	return nil, s.requireEnterprise()
}

func (s sigstorePolicyResourceService) UpdateSigstorePolicy(context.Context, *workloadidentityv1.UpdateSigstorePolicyRequest) (*workloadidentityv1.SigstorePolicy, error) {
	return nil, s.requireEnterprise()
}

func (s sigstorePolicyResourceService) DeleteSigstorePolicy(context.Context, *workloadidentityv1.DeleteSigstorePolicyRequest) (*emptypb.Empty, error) {
	return nil, s.requireEnterprise()
}

func (s sigstorePolicyResourceService) GetSigstorePolicy(context.Context, *workloadidentityv1.GetSigstorePolicyRequest) (*workloadidentityv1.SigstorePolicy, error) {
	return nil, s.requireEnterprise()
}

func (s sigstorePolicyResourceService) ListSigstorePolicies(context.Context, *workloadidentityv1.ListSigstorePoliciesRequest) (*workloadidentityv1.ListSigstorePoliciesResponse, error) {
	return nil, s.requireEnterprise()
}

func (sigstorePolicyResourceService) requireEnterprise() error {
	return trace.AccessDenied("Sigstore workload attestation is only available with an enterprise license")
}
