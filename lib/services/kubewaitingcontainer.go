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
	"google.golang.org/protobuf/types/known/timestamppb"

	kubewaitingcontainerpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/kubewaitingcontainer/v1"
	"github.com/gravitational/teleport/api/types/kubewaitingcontainer"
	"github.com/gravitational/teleport/lib/utils"
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
	if in == nil {
		return nil, trace.BadParameter("message is nil")
	}
	if err := kubewaitingcontainer.ValidateKubeWaitingContainer(in); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !cfg.PreserveResourceID {
		//nolint:staticcheck // SA1019. Deprecated, but still needed.
		in.Metadata.Id = 0
		in.Metadata.Revision = ""
	}

	out, err := utils.FastMarshal(in)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

// UnmarshalKubeWaitingContainer unmarshals a KubernetesWaitingContainer resource from JSON.
func UnmarshalKubeWaitingContainer(data []byte, opts ...MarshalOption) (*kubewaitingcontainerpb.KubernetesWaitingContainer, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("data is empty")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out *kubewaitingcontainerpb.KubernetesWaitingContainer
	if err := utils.FastUnmarshal(data, &out); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if err := kubewaitingcontainer.ValidateKubeWaitingContainer(out); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.ID != 0 {
		//nolint:staticcheck // SA1019. Deprecated, but still needed.
		out.Metadata.Id = cfg.ID
	}
	if cfg.Revision != "" {
		out.Metadata.Revision = cfg.Revision
	}
	if !cfg.Expires.IsZero() {
		out.Metadata.Expires = timestamppb.New(cfg.Expires)
	}

	return out, nil
}
