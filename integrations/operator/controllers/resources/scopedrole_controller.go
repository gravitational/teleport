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
)

type scopedRoleClient struct {
	teleportClient *client.Client
}

func (s *scopedRoleClient) Create(ctx context.Context, role *accessv1.ScopedRole) error {
	_, err := s.teleportClient.ScopedAccessServiceClient().CreateScopedRole(ctx, &accessv1.CreateScopedRoleRequest{
		Role: role,
	})
	return trace.Wrap(err)
}

func (s *scopedRoleClient) Delete(ctx context.Context, name string) error {
	_, err := s.teleportClient.ScopedAccessServiceClient().DeleteScopedRole(ctx, &accessv1.DeleteScopedRoleRequest{
		Name: name,
	})
	// The deletetion API for scoped roles returns CompareFailed if not found, which was an intentional decision
	// We handle it here and return a not found error for consistency with the deletion reconcile.
	if err != nil && trace.IsCompareFailed(err) {
		return trace.NotFound("scoped role %q not found: %v", name, err)
	}
	return trace.Wrap(err)
}

func (s *scopedRoleClient) Get(ctx context.Context, name string) (*accessv1.ScopedRole, error) {
	resp, err := s.teleportClient.ScopedAccessServiceClient().GetScopedRole(ctx, &accessv1.GetScopedRoleRequest{
		Name: name,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp.GetRole(), nil
}

func (s *scopedRoleClient) Update(ctx context.Context, role *accessv1.ScopedRole) error {
	_, err := s.teleportClient.ScopedAccessServiceClient().UpdateScopedRole(ctx, &accessv1.UpdateScopedRoleRequest{
		Role: role,
	})
	return trace.Wrap(err)
}

func NewScopedRoleV1Reconciler(client kclient.Client, tClient *client.Client) (controllers.Reconciler, error) {
	return reconcilers.NewTeleportResource153Reconciler[*accessv1.ScopedRole, *resourcesv1.TeleportScopedRoleV1](
		client,
		&scopedRoleClient{
			teleportClient: tClient,
		},
	)
}
