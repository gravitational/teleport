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

package services

import (
	"context"

	"github.com/gravitational/trace"

	kubewaitingcontainerpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/kubewaitingcontainer/v1"
	"github.com/gravitational/teleport/api/types/kubewaitingcontainer"
)

// KubeWaitingContainer is responsible for managing Kubernetes
// ephemeral containers that are waiting to be created until moderated
// session conditions are met.
type KubeWaitingContainer interface {
	ListKubernetesWaitingContainers(ctx context.Context, pageSize int, pageToken string) ([]*kubewaitingcontainerpb.KubernetesWaitingContainer, string, error)
	GetKubernetesWaitingContainer(ctx context.Context, req *kubewaitingcontainerpb.GetKubernetesWaitingContainerRequest) (*kubewaitingcontainerpb.KubernetesWaitingContainer, error)
	CreateKubernetesWaitingContainer(ctx context.Context, in *kubewaitingcontainerpb.KubernetesWaitingContainer) (*kubewaitingcontainerpb.KubernetesWaitingContainer, error)
	DeleteKubernetesWaitingContainer(ctx context.Context, req *kubewaitingcontainerpb.DeleteKubernetesWaitingContainerRequest) error
}

// MarshalKubeWaitingContainer marshals a KubernetesWaitingContainer resource to JSON.
func MarshalKubeWaitingContainer(in *kubewaitingcontainerpb.KubernetesWaitingContainer, opts ...MarshalOption) ([]byte, error) {
	if err := kubewaitingcontainer.ValidateKubeWaitingContainer(in); err != nil {
		return nil, trace.Wrap(err)
	}

	return FastMarshalProtoResourceDeprecated(in, opts...)
}

// UnmarshalKubeWaitingContainer unmarshals a KubernetesWaitingContainer resource from JSON.
func UnmarshalKubeWaitingContainer(data []byte, opts ...MarshalOption) (*kubewaitingcontainerpb.KubernetesWaitingContainer, error) {
	out, err := FastUnmarshalProtoResourceDeprecated[*kubewaitingcontainerpb.KubernetesWaitingContainer](data, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := kubewaitingcontainer.ValidateKubeWaitingContainer(out); err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}
