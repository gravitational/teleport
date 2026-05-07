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
	tokenv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
)

type scopedTokenClient struct {
	teleportClient *client.Client
}

func (s *scopedTokenClient) Create(ctx context.Context, token *tokenv1.ScopedToken) error {
	_, err := s.teleportClient.CreateScopedToken(ctx, token)
	return trace.Wrap(err)
}

func (s *scopedTokenClient) Delete(ctx context.Context, name string) error {
	return s.teleportClient.DeleteScopedToken(ctx, name)
}

func (s *scopedTokenClient) Get(ctx context.Context, name string) (*tokenv1.ScopedToken, error) {
	return s.teleportClient.GetScopedToken(ctx, name, false)
}

func (s *scopedTokenClient) Update(ctx context.Context, token *tokenv1.ScopedToken) error {
	_, err := s.teleportClient.UpdateScopedToken(ctx, token)
	return trace.Wrap(err)
}

func NewScopedTokenV1Reconciler(client kclient.Client, tClient *client.Client) (controllers.Reconciler, error) {
	return reconcilers.NewTeleportResource153Reconciler[*tokenv1.ScopedToken, *resourcesv1.TeleportScopedTokenV1](
		client,
		&scopedTokenClient{
			teleportClient: tClient,
		},
	)
}
