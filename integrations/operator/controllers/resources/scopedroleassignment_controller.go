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

package resources

import (
	"context"

	"github.com/gravitational/trace"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/client"
	accessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
)

type scopedRoleAssignmentClient struct {
	teleportClient *client.Client
}

func (s *scopedRoleAssignmentClient) Create(ctx context.Context, assignment *accessv1.ScopedRoleAssignment) error {
	_, err := s.teleportClient.ScopedAccessServiceClient().CreateScopedRoleAssignment(ctx, &accessv1.CreateScopedRoleAssignmentRequest{
		Assignment: assignment,
	})
	return trace.Wrap(err)
}

func (s *scopedRoleAssignmentClient) Delete(ctx context.Context, name string) error {
	_, err := s.teleportClient.ScopedAccessServiceClient().DeleteScopedRoleAssignment(ctx, &accessv1.DeleteScopedRoleAssignmentRequest{
		Name:    name,
		SubKind: scopedaccess.SubKindDynamic,
	})
	return trace.Wrap(err)
}

func (s *scopedRoleAssignmentClient) Get(ctx context.Context, name string) (*accessv1.ScopedRoleAssignment, error) {
	resp, err := s.teleportClient.ScopedAccessServiceClient().GetScopedRoleAssignment(ctx, &accessv1.GetScopedRoleAssignmentRequest{
		Name:    name,
		SubKind: scopedaccess.SubKindDynamic,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp.GetAssignment(), nil
}

func (s *scopedRoleAssignmentClient) Update(ctx context.Context, assignment *accessv1.ScopedRoleAssignment) error {
	_, err := s.teleportClient.ScopedAccessServiceClient().UpdateScopedRoleAssignment(ctx, &accessv1.UpdateScopedRoleAssignmentRequest{
		Assignment: assignment,
	})
	return trace.Wrap(err)
}

func NewScopedRoleAssignmentV1Reconciler(client kclient.Client, tClient *client.Client) (controllers.Reconciler, error) {
	return reconcilers.NewTeleportResource153Reconciler[*accessv1.ScopedRoleAssignment, *resourcesv1.TeleportScopedRoleAssignmentV1](
		client,
		&scopedRoleAssignmentClient{
			teleportClient: tClient,
		},
	)
}
