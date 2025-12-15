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

// ResourceWithLabelsAdapter implements the Adapter interface for any resource
// implementing types.ResourceWithLabels.
type ResourceWithLabelsAdapter[T types.ResourceWithLabels] struct {
}

// GetResourceName implements the Adapter interface.
func (a ResourceWithLabelsAdapter[T]) GetResourceName(res T) string {
	return res.GetName()
}

// GetResourceRevision implements the Adapter interface.
func (a ResourceWithLabelsAdapter[T]) GetResourceRevision(res T) string {
	return res.GetRevision()
}

// GetResourceOrigin implements the Adapter interface.
func (a ResourceWithLabelsAdapter[T]) GetResourceOrigin(res T) string {
	origin, _ := res.GetLabel(types.OriginLabel)
	return origin
}

// SetResourceRevision implements the Adapter interface.
func (a ResourceWithLabelsAdapter[T]) SetResourceRevision(res T, revision string) {
	res.SetRevision(revision)
}

// SetResourceLabels implements the Adapter interface.
func (a ResourceWithLabelsAdapter[T]) SetResourceLabels(res T, labels map[string]string) {
	res.SetStaticLabels(labels)
}

// NewTeleportResourceWithLabelsReconciler instantiates a resourceReconciler for a
// types.ResourceWithLabels resource.
func NewTeleportResourceWithLabelsReconciler[T types.ResourceWithLabels, K KubernetesCR[T]](
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
		adapter:        ResourceWithLabelsAdapter[T]{},
	}
	return reconciler, nil
}
