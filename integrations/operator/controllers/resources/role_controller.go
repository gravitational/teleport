/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package resources

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/gravitational/teleport/api/types"
	v5 "github.com/gravitational/teleport/integrations/operator/apis/resources/v5"
	v6 "github.com/gravitational/teleport/integrations/operator/apis/resources/v6"
	v7 "github.com/gravitational/teleport/integrations/operator/apis/resources/v7"
	"github.com/gravitational/teleport/integrations/operator/sidecar"
)

const teleportRoleKind = "TeleportRole"

// TODO(for v12): Have the Role controller to use the generic Teleport reconciler
// This means we'll have to move back to a statically typed client.
// This will require removing the crdgen hack, fixing TeleportRole JSON serialization

var TeleportRoleGVKV7 = schema.GroupVersionKind{
	Group:   v7.GroupVersion.Group,
	Version: v7.GroupVersion.Version,
	Kind:    teleportRoleKind,
}

var TeleportRoleGVKV6 = schema.GroupVersionKind{
	Group:   v6.GroupVersion.Group,
	Version: v6.GroupVersion.Version,
	Kind:    teleportRoleKind,
}

var TeleportRoleGVKV5 = schema.GroupVersionKind{
	Group:   v5.GroupVersion.Group,
	Version: v5.GroupVersion.Version,
	Kind:    teleportRoleKind,
}

// RoleReconciler reconciles a TeleportRole object
type RoleReconciler struct {
	kclient.Client
	Scheme                 *runtime.Scheme
	TeleportClientAccessor sidecar.ClientAccessor
}

//+kubebuilder:rbac:groups=resources.teleport.dev,resources=roles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=resources.teleport.dev,resources=roles/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=resources.teleport.dev,resources=roles/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *RoleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// The TeleportRole OpenAPI spec does not validate typing of Label fields like `node_labels`.
	// This means we can receive invalid data, by default it won't be unmarshalled properly and will crash the operator.
	// To handle this more gracefully we unmarshall first in an unstructured object.
	// The unstructured object will be converted later to a typed one, in r.UpsertExternal.
	// See `/operator/crdgen/schemagen.go` and https://github.com/gravitational/teleport/issues/15204 for context.
	// TODO: (Check how to handle multiple versions)
	log := ctrllog.FromContext(ctx).WithValues("namespacedname", req.NamespacedName)

	obj := GetUnstructuredObjectFromGVK(TeleportRoleGVKV7)
	err := r.Get(ctx, req.NamespacedName, obj)
	switch {
	case apierrors.IsNotFound(err):
		log.Info("not found")
		return ctrl.Result{}, nil
	case err != nil:
		log.Error(err, "failed to get resource")
		return ctrl.Result{}, trace.Wrap(err)
	case getAPIVersionFromManagedFields(obj) == v7.GroupVersion.String():
		obj = GetUnstructuredObjectFromGVK(TeleportRoleGVKV7)
	case getAPIVersionFromManagedFields(obj) == v6.GroupVersion.String():
		obj = GetUnstructuredObjectFromGVK(TeleportRoleGVKV6)
	case getAPIVersionFromManagedFields(obj) == v5.GroupVersion.String():
		obj = GetUnstructuredObjectFromGVK(TeleportRoleGVKV5)
	default:
		return ctrl.Result{}, trace.Errorf("unknown api version %s", getAPIVersionFromManagedFields(obj))
	}

	return ResourceBaseReconciler{
		Client:         r.Client,
		DeleteExternal: r.Delete,
		UpsertExternal: r.Upsert,
	}.Do(ctx, req, obj)
}

// getAPIVersionFromManagedFields returns the API version of the object from the managed fields.
// If the object does not have any managed fields, it returns the API version of the object.
// This is required because if the object version differs from the requested version,
// the object will not be converted properly and everything will be stored in the
// managed fields.
func getAPIVersionFromManagedFields(obj *unstructured.Unstructured) string {
	for _, field := range obj.GetManagedFields() {
		if field.APIVersion != "" {
			return field.APIVersion
		}
	}
	return obj.GetAPIVersion()
}

// SetupWithManager sets up the controller with the Manager.
func (r *RoleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// The TeleportRole OpenAPI spec does not validate typing of Label fields like `node_labels`.
	// This means we can receive invalid data, by default it won't be unmarshalled properly and will crash the operator
	// To handle this more gracefully we unmarshall first in an unstructured object.
	// The unstructured object will be converted later to a typed one, in r.UpsertExternal.
	// See `/operator/crdgen/schemagen.go` and https://github.com/gravitational/teleport/issues/15204 for context
	// TODO: (Check how to handle multiple versions)
	obj := GetUnstructuredObjectFromGVK(TeleportRoleGVKV7)
	return ctrl.NewControllerManagedBy(mgr).
		For(obj).
		Complete(r)
}

func (r *RoleReconciler) Delete(ctx context.Context, obj kclient.Object) error {
	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	return teleportClient.DeleteRole(ctx, obj.GetName())
}

func (r *RoleReconciler) Upsert(ctx context.Context, obj kclient.Object) error {
	// We receive an unstructured object. We convert it to a typed TeleportRole object and gracefully handle errors.
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("failed to convert Object into resource object: %T", obj)
	}
	// We need to convert the unstructured object into a typed TeleportRole object.
	// We check the API version to determine which TeleportRole object to use.
	var k8sResource role
	switch u.GetAPIVersion() {
	case v7.GroupVersion.String():
		k8sResource = &v7.TeleportRole{}
	case v6.GroupVersion.String():
		k8sResource = &v6.TeleportRole{}
	case v5.GroupVersion.String():
		k8sResource = &v5.TeleportRole{}
	default:
		return fmt.Errorf("unknown api version: %s", u.GetAPIVersion())
	}

	// If an error happens we want to put it in status.conditions before returning.
	err := runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(
		u.Object,
		k8sResource, true, /* returnUnknownFields */
	)
	newStructureCondition := getStructureConditionFromError(err)
	meta.SetStatusCondition(k8sResource.StatusConditions(), newStructureCondition)
	if err != nil {
		silentUpdateStatus(ctx, r.Client, k8sResource)
		return trace.Wrap(err)
	}

	// Converting the Kubernetes resource into a Teleport one, checking potential ownership issues.
	teleportResource := k8sResource.ToTeleport()
	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		silentUpdateStatus(ctx, r.Client, k8sResource)
		return trace.Wrap(err)
	}

	existingResource, err := teleportClient.GetRole(ctx, teleportResource.GetName())
	if err != nil && !trace.IsNotFound(err) {
		silentUpdateStatus(ctx, r.Client, k8sResource)
		return trace.Wrap(err)
	}

	if err == nil {
		// The resource already exists
		newOwnershipCondition, isOwned := checkOwnership(existingResource)
		meta.SetStatusCondition(k8sResource.StatusConditions(), newOwnershipCondition)
		if !isOwned {
			silentUpdateStatus(ctx, r.Client, k8sResource)
			return trace.AlreadyExists("unowned resource '%s' already exists", existingResource.GetName())
		}
	} else {
		// The resource does not yet exist
		meta.SetStatusCondition(k8sResource.StatusConditions(), newResourceCondition)
	}

	r.AddTeleportResourceOrigin(teleportResource)

	// If an error happens we want to put it in status.conditions before returning.
	_, err = teleportClient.UpsertRole(ctx, teleportResource)
	newReconciliationCondition := getReconciliationConditionFromError(err)
	meta.SetStatusCondition(k8sResource.StatusConditions(), newReconciliationCondition)
	if err != nil {
		silentUpdateStatus(ctx, r.Client, k8sResource)
		return trace.Wrap(err)
	}

	// We update the status conditions on exit
	return trace.Wrap(r.Status().Update(ctx, k8sResource))
}

// role is an interface that all TeleportRole versions implement.
type role interface {
	StatusConditions() *[]metav1.Condition
	ToTeleport() types.Role
	kclient.Object
}

func (r *RoleReconciler) AddTeleportResourceOrigin(resource types.Role) {
	metadata := resource.GetMetadata()
	if metadata.Labels == nil {
		metadata.Labels = make(map[string]string)
	}
	metadata.Labels[types.OriginLabel] = types.OriginKubernetes
	resource.SetMetadata(metadata)
}

func GetUnstructuredObjectFromGVK(gvk schema.GroupVersionKind) *unstructured.Unstructured {
	obj := unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	return &obj
}
