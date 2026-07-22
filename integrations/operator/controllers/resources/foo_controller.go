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
	foov1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/foo/v1"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
)

type fooClient struct {
	teleportClient *client.Client
}

func (s *fooClient) Create(ctx context.Context, foo *foov1.Foo) error {
	_, err := s.teleportClient.FooClient().CreateFoo(ctx, foov1.CreateFooRequest_builder{
		Foo: foo,
	}.Build())
	return trace.Wrap(err)
}

func (s *fooClient) Delete(ctx context.Context, key reconcilers.ResourceKey) error {
	_, err := s.teleportClient.FooClient().DeleteFoo(ctx, foov1.DeleteFooRequest_builder{
		Scope: key.Scope,
		Name:  key.Name,
	}.Build())
	return trace.Wrap(err)
}

func (s *fooClient) Get(ctx context.Context, key reconcilers.ResourceKey) (*foov1.Foo, error) {
	resp, err := s.teleportClient.FooClient().GetFoo(ctx, foov1.GetFooRequest_builder{
		Scope: key.Scope,
		Name:  key.Name,
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp.GetFoo(), nil
}

func (s *fooClient) Update(ctx context.Context, foo *foov1.Foo) error {
	_, err := s.teleportClient.FooClient().UpdateFoo(ctx, foov1.UpdateFooRequest_builder{
		Foo: foo,
	}.Build())
	return trace.Wrap(err)
}

func NewFooV1Reconciler(client kclient.Client, tClient *client.Client) (controllers.Reconciler, error) {
	return reconcilers.NewTeleportResource153Reconciler[*foov1.Foo, *resourcesv1.TeleportFooV1](
		client,
		&fooClient{
			teleportClient: tClient,
		},
		reconcilers.Config{
			Scoped: true,
		},
	)
}
