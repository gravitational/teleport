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

// resourceWithoutLabels is for resources that don't implement types.ResourceWithLabels
// but implement types.ResourceWithOrigin. This is a subset of types.ResourceWithOrigin.
type resourceWithoutLabels interface {
	GetKind() string
	GetName() string
	Origin() string
	SetOrigin(string)
	GetRevision() string
	SetRevision(string)
}

// ResourceWithoutLabelsAdapter implements the Adapter interface for resources
// implementing resourceWithoutLabels.
type ResourceWithoutLabelsAdapter[T resourceWithoutLabels] struct {
}

// GetResourceName implements the Adapter interface.
func (a ResourceWithoutLabelsAdapter[T]) GetResourceName(res T) string {
	return res.GetName()
}

// GetResourceRevision implements the Adapter interface.
func (a ResourceWithoutLabelsAdapter[T]) GetResourceRevision(res T) string {
	return res.GetRevision()
}

// GetResourceOrigin implements the Adapter interface.
func (a ResourceWithoutLabelsAdapter[T]) GetResourceOrigin(res T) string {
	return res.Origin()
}

// SetResourceRevision implements the Adapter interface.
func (a ResourceWithoutLabelsAdapter[T]) SetResourceRevision(res T, revision string) {
	res.SetRevision(revision)
}

// SetResourceLabels implements the Adapter interface. As the resource does not
// support labels, it only sets the origin label.
func (a ResourceWithoutLabelsAdapter[T]) SetResourceLabels(res T, labels map[string]string) {
	// We don't set all labels as the Resource doesn't support them
	// Only the origin
	origin := labels[types.OriginLabel]
	res.SetOrigin(origin)
}

// NewTeleportResourceWithoutLabelsReconciler instantiates a resourceReconciler for a
// resource not implementing types.ResourcesWithLabels but implementing
// resourceWithoutLabels.
func NewTeleportResourceWithoutLabelsReconciler[T resourceWithoutLabels, K KubernetesCR[T]](
	kubeClient kclient.Client,
	resourceClient resourceClient[T],
	config Config,
) (controllers.Reconciler, error) {
	checkFeatures := controllers.AlwaysEnabled
	if config.CheckFeatures != nil {
		checkFeatures = config.CheckFeatures
	}

	gvk, err := gvkFromScheme[K](controllers.Scheme)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	teleportKind := newKubeResource[K]().ToTeleport().GetKind()
	if teleportKind == "" {
		return nil, trace.BadParameter("teleport kind is required, this is a bug")
	}

	reconciler := &resourceReconciler[T, K]{
		kubeClient:     kubeClient,
		resourceClient: resourceClient,
		gvk:            gvk,
		adapter:        ResourceWithoutLabelsAdapter[T]{},
		scoped:         config.Scoped,
		teleportKind:   teleportKind,
		checkFeatures:  checkFeatures,
	}
	return reconciler, nil
}
