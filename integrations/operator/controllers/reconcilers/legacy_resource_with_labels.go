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
	"fmt"

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

// CheckOwnership implements the Adapter interface.
func (a ResourceWithLabelsAdapter[T]) CheckOwnership(res T, _ OperatorMetadata) (bool, string) {
	origin, _ := res.GetLabel(types.OriginLabel)
	switch origin {
	case types.OriginKubernetes:
		return true, ""
	case "":
		return false, ownershipIssueMissingOriginLabel
	default:
		return false, fmt.Sprintf(ownershipIssueMismatchOriginLabel, origin)
	}
}

// SetResourceRevision implements the Adapter interface.
func (a ResourceWithLabelsAdapter[T]) SetResourceRevision(res T, revision string) {
	res.SetRevision(revision)
}

// SetResourceLabels implements the Adapter interface.
func (a ResourceWithLabelsAdapter[T]) SetResourceLabels(res T, labels map[string]string, _ OperatorMetadata, _ customResourceMetadata) {
	labels[types.OriginLabel] = types.OriginKubernetes
	res.SetStaticLabels(labels)
}

// NewTeleportResourceWithLabelsReconciler instantiates a resourceReconciler for a
// types.ResourceWithLabels resource.
func NewTeleportResourceWithLabelsReconciler[T types.ResourceWithLabels, K KubernetesCR[T]](
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
		adapter:        ResourceWithLabelsAdapter[T]{},
		scoped:         config.Scoped,
		teleportKind:   teleportKind,
		checkFeatures:  checkFeatures,
	}
	return reconciler, nil
}

// ScopedResourceWithLabels extends [types.ResourceWithLabels] for Teleport
// resources that are scoped.
type ScopedResourceWithLabels interface {
	types.ResourceWithLabels
	GetScope() string
}

type ScopedResourceWithLabelsAdapter[T ScopedResourceWithLabels] struct {
	ResourceWithLabelsAdapter[T]
}

func (a ScopedResourceWithLabelsAdapter[T]) GetResourceScope(res T) string {
	return res.GetScope()
}

func (a ScopedResourceWithLabelsAdapter[T]) CheckOwnership(res T, metadata OperatorMetadata) (bool, string) {
	// Do the base tests.
	if ok, reason := a.ResourceWithLabelsAdapter.CheckOwnership(res, metadata); !ok {
		return ok, reason
	}

	// For scoped resources, also check the operator ID.
	if res.GetScope() == "" {
		return true, ""
	}
	if id, ok := res.GetLabel(OperatorIDLabel); ok {
		if id != metadata.ID {
			return false, fmt.Sprintf(ownershipIssueMismatchOperatorID, id, metadata.ID)
		}
		return true, ""
	}

	return false, ownershipIssueMissingOperatorID
}

func (a ScopedResourceWithLabelsAdapter[T]) SetResourceLabels(res T, labels map[string]string, metadata OperatorMetadata, customResourceMetadata customResourceMetadata) {
	if res.GetScope() == "" {
		a.ResourceWithLabelsAdapter.SetResourceLabels(res, labels, metadata, customResourceMetadata)
		return
	}
	updateScopedLabels(labels, metadata, customResourceMetadata)
	res.SetStaticLabels(labels)
}

func NewTeleportScopedResourceWithLabelsReconciler[T ScopedResourceWithLabels, K KubernetesCR[T]](
	kubeClient kclient.Client,
	resourceClient resourceClient[T],
	config Config,
	metadata OperatorMetadata,
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
		kubeClient:       kubeClient,
		resourceClient:   resourceClient,
		gvk:              gvk,
		adapter:          ScopedResourceWithLabelsAdapter[T]{},
		scoped:           true,
		teleportKind:     teleportKind,
		checkFeatures:    checkFeatures,
		operatorMetadata: metadata,
	}
	return reconciler, nil
}
