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

// CheckOwnership implements the Adapter interface.
func (a Resource153Adapter[T]) CheckOwnership(res T, _ OperatorMetadata) (bool, string) {
	labels := res.GetMetadata().GetLabels()
	// catches nil and empty maps
	if len(labels) == 0 {
		return false, ownershipIssueMissingOriginLabel
	}

	if origin, ok := labels[types.OriginLabel]; ok {
		if origin == types.OriginKubernetes {
			return true, ""
		}
		return false, fmt.Sprintf(ownershipIssueMismatchOriginLabel, origin)
	}
	// Origin label is not set
	return false, ownershipIssueMissingOriginLabel
}

// SetResourceRevision implements the Adapter interface.
func (a Resource153Adapter[T]) SetResourceRevision(res T, revision string) {
	res.GetMetadata().Revision = revision
}

// SetResourceLabels implements the Adapter interface.
func (a Resource153Adapter[T]) SetResourceLabels(res T, labels map[string]string, _ OperatorMetadata, _ customResourceMetadata) {
	labels[types.OriginLabel] = types.OriginKubernetes
	res.GetMetadata().Labels = labels
}

// NewTeleportResource153Reconciler instantiates a resourceReconciler for a
// types.Resource153 resource.
func NewTeleportResource153Reconciler[T types.Resource153, K KubernetesCR[T]](
	client kclient.Client,
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
		kubeClient:     client,
		resourceClient: resourceClient,
		gvk:            gvk,
		adapter:        Resource153Adapter[T]{},
		scoped:         config.Scoped,
		teleportKind:   teleportKind,
		checkFeatures:  checkFeatures,
	}
	return reconciler, nil
}

// ScopedResource153 extends [types.Resource153] for Teleport
// resources that are scoped.
type ScopedResource153 interface {
	types.Resource153
	GetScope() string
}

type ScopedResource153Adapter[T ScopedResource153] struct {
	Resource153Adapter[T]
}

func (a ScopedResource153Adapter[T]) GetResourceScope(res T) string {
	return res.GetScope()
}

func (a ScopedResource153Adapter[T]) CheckOwnership(res T, metadata OperatorMetadata) (bool, string) {
	// Do the base tests.
	if ok, reason := a.Resource153Adapter.CheckOwnership(res, metadata); !ok {
		return ok, reason
	}

	// For scoped resources, also check the operator ID.
	if res.GetScope() == "" {
		return true, ""
	}
	if id, ok := res.GetMetadata().GetLabels()[OperatorIDLabel]; ok {
		if id != metadata.ID {
			return false, fmt.Sprintf(ownershipIssueMismatchOperatorID, id, metadata.ID)
		}
		return true, ""
	}

	return false, ownershipIssueMissingOperatorID
}

func (a ScopedResource153Adapter[T]) SetResourceLabels(res T, labels map[string]string, metadata OperatorMetadata, customResourceMetadata customResourceMetadata) {
	// No need to copy the label map here, the caller already does it.
	if res.GetScope() == "" {
		a.Resource153Adapter.SetResourceLabels(res, labels, metadata, customResourceMetadata)
		return
	}
	updateScopedLabels(labels, metadata, customResourceMetadata)
	res.GetMetadata().SetLabels(labels)
}

// NewTeleportScopedResource153Reconciler instantiates a resourceReconciler for a
// ScopedResource153 resource.
func NewTeleportScopedResource153Reconciler[T ScopedResource153, K KubernetesCR[T]](
	client kclient.Client,
	resourceClient resourceClient[T],
	config Config,
	operatorMetadata OperatorMetadata,
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
		kubeClient:       client,
		resourceClient:   resourceClient,
		gvk:              gvk,
		adapter:          ScopedResource153Adapter[T]{},
		scoped:           true,
		teleportKind:     teleportKind,
		checkFeatures:    checkFeatures,
		operatorMetadata: operatorMetadata,
	}
	return reconciler, nil
}
