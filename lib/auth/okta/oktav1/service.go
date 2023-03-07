// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package oktav1

import (
	"context"

	oktapb "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Service struct {
	oktapb.OktaServiceServer
}

// ListOktaImportRules returns a paginated list of all Okta import rule resources.
func (s *Service) ListOktaImportRules(ctx context.Context, req *oktapb.ListOktaImportRulesRequest) (*oktapb.ListOktaImportRulesResponse, error) {
	auth, err := s.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	results, nextToken, err := auth.ListOktaImportRules(ctx, int(req.GetPageSize()), req.GetPageToken())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	importRulesV1 := make([]*types.OktaImportRuleV1, len(results))
	for i, r := range results {
		v1, ok := r.(*types.OktaImportRuleV1)
		if !ok {
			return nil, trace.BadParameter("unexpected Okta import rule type %T", g)
		}
		importRulesV1[i] = v1
	}

	return &oktapb.ListOktaImportRulesResponse{
		ImportRules: importRulesV1,
		NextToken:   nextToken,
	}, nil
}

// GetOktaImportRule returns the specified Okta import rule resources.
func (s *Service) GetOktaImportRule(ctx context.Context, req *oktapb.GetOktaImportRuleRequest) (*types.OktaImportRuleV1, error) {
	auth, err := s.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	importRule, err := auth.GetOktaImportRule(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	importRuleV1, ok := importRule.(*types.OktaImportRuleV1)
	if !ok {
		return nil, trace.BadParameter("unexpected Okta import rule type %T", importRule)
	}

	return importRuleV1, nil
}

// CreateOktaImportRule creates a new Okta import rule resource.
func (s *Service) CreateOktaImportRule(ctx context.Context, req *oktapb.CreateOktaImportRuleRequest) (*emptypb.Empty, error) {
	auth, err := s.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, trace.Wrap(auth.CreateOktaImportRule(ctx, req.GetImportRule()))
}

// UpdateOktaImportRule updates an existing Okta import rule resource.
func (s *Service) UpdateOktaImportRule(ctx context.Context, req *oktapb.UpdateOktaImportRuleRequest) (*emptypb.Empty, error) {
	auth, err := s.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, trace.Wrap(auth.UpdateOktaImportRule(ctx, req.GetImportRule()))
}

// DeleteOktaImportRule removes the specified Okta import rule resource.
func (s *Service) DeleteOktaImportRule(ctx context.Context, req *oktapb.DeleteOktaImportRuleRequest) (*emptypb.Empty, error) {
	auth, err := s.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, trace.Wrap(auth.DeleteOktaImportRule(ctx, req.GetName()))
}

// DeleteAllOktaImportRules removes all Okta import rules.
func (s *Service) DeleteAllOktaImportRules(ctx context.Context, _ *oktapb.DeleteAllOktaImportRulesRequest) (*emptypb.Empty, error) {
	auth, err := s.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, trace.Wrap(auth.DeleteAllOktaImportRules(ctx))
}

// ListOktaAssignments returns a paginated list of all Okta assignment resources.
func (s *Service) ListOktaAssignments(ctx context.Context, req *oktapb.ListOktaAssignmentsRequest) (*oktapb.ListOktaAssignmentsResponse, error) {
	auth, err := s.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	results, nextToken, err := auth.ListOktaAssignments(ctx, int(req.GetPageSize()), req.GetPageToken())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	assignmentsV1 := make([]*types.OktaAssignmentV1, len(results))
	for i, a := range results {
		v1, ok := a.(*types.OktaAssignmentV1)
		if !ok {
			return nil, trace.BadParameter("unexpected Okta assignment type %T", g)
		}
		assignmentsV1[i] = v1
	}

	return &oktapb.ListOktaAssignmentsResponse{
		Assignments: assignmentsV1,
		NextToken:   nextToken,
	}, nil
}

// GetOktaAssignmentreturns the specified Okta assignment resources.
func (s *Service) GetOktaAssignment(ctx context.Context, req *oktapb.GetOktaAssignmentRequest) (*types.OktaAssignmentV1, error) {
	auth, err := s.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	assignment, err := auth.GetOktaAssignment(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	assignmentV1, ok := assignment.(*types.OktaAssignmentV1)
	if !ok {
		return nil, trace.BadParameter("unexpected Okta assignment type %T", assignment)
	}

	return assignmentV1, nil
}

// CreateOktaAssignmentcreates a new Okta assignment resource.
func (s *Service) CreateOktaAssignment(ctx context.Context, req *oktapb.CreateOktaAssignmentRequest) (*emptypb.Empty, error) {
	auth, err := s.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, trace.Wrap(auth.CreateOktaAssignment(ctx, req.GetAssignment()))
}

// UpdateOktaAssignmentupdates an existing Okta assignment resource.
func (s *Service) UpdateOktaAssignment(ctx context.Context, req *oktapb.UpdateOktaAssignmentRequest) (*emptypb.Empty, error) {
	auth, err := s.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, trace.Wrap(auth.UpdateOktaAssignment(ctx, req.GetAssignment()))
}

// DeleteOktaAssignmentremoves the specified Okta assignment resource.
func (s *Service) DeleteOktaAssignment(ctx context.Context, req *oktapb.DeleteOktaAssignmentRequest) (*emptypb.Empty, error) {
	auth, err := s.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, trace.Wrap(auth.DeleteOktaAssignment(ctx, req.GetName()))
}

// DeleteAllOktaAssignments removes all Okta assignments.
func (s *Service) DeleteAllOktaAssignments(ctx context.Context, _ *oktapb.DeleteAllOktaAssignmentsRequest) (*emptypb.Empty, error) {
	auth, err := s.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, trace.Wrap(auth.DeleteAllOktaAssignments(ctx))
}
