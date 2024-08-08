/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package local

import (
	"context"

	"github.com/gravitational/trace"

	kubeprovisionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/kubeprovision/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

type KubeProvisionService struct {
	service *generic.ServiceWrapper[*kubeprovisionv1.KubeProvision]
}

const kubeProvisionPrefix = "kube_provision"

// NewKubeProvisionService creates a new KubeProvisionService.
func NewKubeProvisionService(backend backend.Backend) (*KubeProvisionService, error) {
	service, err := generic.NewServiceWrapper(backend,
		types.KindKubeProvision,
		kubeProvisionPrefix,
		services.MarshalKubeProvision,
		services.UnmarshalKubeProvision)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &KubeProvisionService{service: service}, nil
}

func (s *KubeProvisionService) ListKubeProvisions(ctx context.Context, pagesize int, lastKey string) ([]*kubeprovisionv1.KubeProvision, string, error) {
	r, nextToken, err := s.service.ListResources(ctx, pagesize, lastKey)
	return r, nextToken, trace.Wrap(err)
}

func (s *KubeProvisionService) GetKubeProvision(ctx context.Context, name string) (*kubeprovisionv1.KubeProvision, error) {
	r, err := s.service.GetResource(ctx, name)
	return r, trace.Wrap(err)
}

func (s *KubeProvisionService) CreateKubeProvision(ctx context.Context, kubeProvision *kubeprovisionv1.KubeProvision) (*kubeprovisionv1.KubeProvision, error) {
	r, err := s.service.CreateResource(ctx, kubeProvision)
	return r, trace.Wrap(err)
}

func (s *KubeProvisionService) UpdateKubeProvision(ctx context.Context, kubeProvision *kubeprovisionv1.KubeProvision) (*kubeprovisionv1.KubeProvision, error) {
	r, err := s.service.ConditionalUpdateResource(ctx, kubeProvision)
	return r, trace.Wrap(err)
}

func (s *KubeProvisionService) UpsertKubeProvision(ctx context.Context, kubeProvision *kubeprovisionv1.KubeProvision) (*kubeprovisionv1.KubeProvision, error) {
	r, err := s.service.UpsertResource(ctx, kubeProvision)
	return r, trace.Wrap(err)
}

func (s *KubeProvisionService) DeleteKubeProvision(ctx context.Context, name string) error {
	err := s.service.DeleteResource(ctx, name)
	return trace.Wrap(err)
}

func (s *KubeProvisionService) DeleteAllKubeProvisions(ctx context.Context) error {
	err := s.service.DeleteAllResources(ctx)
	return trace.Wrap(err)
}
