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

package reconcilers

import (
	"github.com/gravitational/trace"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/operator/controllers"
)

// Resource153Adapter implements the Adapter interface for any resource
// implementing types.Resource153.
type Resource153Adapter[T types.Resource153] struct{}

// GetResourceName implements the Adapter interface.
func (a Resource153Adapter[T]) GetResourceName(res T) string {
	return res.GetMetadata().GetName()
}

// GetResourceRevision implements the Adapter interface.
func (a Resource153Adapter[T]) GetResourceRevision(res T) string {
	return res.GetMetadata().GetRevision()
}

// GetResourceOrigin implements the Adapter interface.
func (a Resource153Adapter[T]) GetResourceOrigin(res T) string {
	labels := res.GetMetadata().GetLabels()
	// catches nil and empty maps
	if len(labels) == 0 {
		return ""
	}

	if origin, ok := labels[types.OriginLabel]; ok {
		return origin
	}
	// Origin label is not set
	return ""
}

// SetResourceRevision implements the Adapter interface.
func (a Resource153Adapter[T]) SetResourceRevision(res T, revision string) {
	res.GetMetadata().Revision = revision
}

// SetResourceLabels implements the Adapter interface.
func (a Resource153Adapter[T]) SetResourceLabels(res T, labels map[string]string) {
	res.GetMetadata().Labels = labels
}

// NewTeleportResource153Reconciler instantiates a resourceReconciler for a
// types.Resource153 resource.
func NewTeleportResource153Reconciler[T types.Resource153, K KubernetesCR[T]](
	client kclient.Client,
	resourceClient resourceClient[T],
) (controllers.Reconciler, error) {
	gvk, err := gvkFromScheme[K](controllers.Scheme)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	reconciler := &resourceReconciler[T, K]{
		kubeClient:     client,
		resourceClient: resourceClient,
		gvk:            gvk,
		adapter:        Resource153Adapter[T]{},
	}
	return reconciler, nil
}
