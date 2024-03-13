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

	kubewaitingcontainerclient "github.com/gravitational/teleport/api/client/kubewaitingcontainer"
	"github.com/gravitational/teleport/api/types/kubewaitingcontainer"
	"github.com/gravitational/teleport/lib/utils"
)

// KubeWaitingContainerGetter is responsible for getting Kubernetes
// ephemeral containers that are waiting to be created until moderated
// session conditions are met.
type KubeWaitingContainerGetter interface {
	ListKubernetesWaitingContainers(ctx context.Context, pageSize int, pageToken string) ([]*kubewaitingcontainer.KubeWaitingContainer, string, error)
	GetKubernetesWaitingContainer(ctx context.Context, req kubewaitingcontainerclient.KubeWaitingContainerRequest) (*kubewaitingcontainer.KubeWaitingContainer, error)
}

// KubeWaitingContainer is responsible for managing Kubernetes
// ephemeral containers that are waiting to be created until moderated
// session conditions are met.
type KubeWaitingContainer interface {
	KubeWaitingContainerGetter

	CreateKubernetesWaitingContainer(ctx context.Context, in *kubewaitingcontainer.KubeWaitingContainer) (*kubewaitingcontainer.KubeWaitingContainer, error)
	DeleteKubernetesWaitingContainer(ctx context.Context, req kubewaitingcontainerclient.KubeWaitingContainerRequest) error
}

// MarshalKubeWaitingContainer marshals a KubernetesWaitingContainer resource to JSON.
func MarshalKubeWaitingContainer(in *kubewaitingcontainer.KubeWaitingContainer, opts ...MarshalOption) ([]byte, error) {
	if in == nil {
		return nil, trace.BadParameter("message is nil")
	}
	if err := in.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !cfg.PreserveResourceID {
		copy := *in
		copy.SetResourceID(0)
		copy.SetRevision("")
		in = &copy
	}

	return utils.FastMarshal(in)
}

// UnmarshalKubeWaitingContainer unmarshals a KubernetesWaitingContainer resource from JSON.
func UnmarshalKubeWaitingContainer(data []byte, opts ...MarshalOption) (*kubewaitingcontainer.KubeWaitingContainer, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("data is empty")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out *kubewaitingcontainer.KubeWaitingContainer
	if err := utils.FastUnmarshal(data, &out); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if err := out.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.ID != 0 {
		out.SetResourceID(cfg.ID)
	}
	if cfg.Revision != "" {
		out.SetRevision(cfg.Revision)
	}
	if !cfg.Expires.IsZero() {
		out.SetExpiry(cfg.Expires)
	}

	return out, nil
}
