package resources

import (
	"context"
	"fmt"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type TeleportKubernetesResource[T types.Resource] interface {
	kclient.Object
	ToTeleport() T
	StatusConditions() *[]v1.Condition
}

type TeleportResourceReconciler[T types.ResourceWithOrigin, K TeleportKubernetesResource[T]] struct {
	ResourceBaseReconciler
	resourceClient TeleportResourceClient[T]
	kubeResource   K
}

type TeleportResourceClient[T types.Resource] interface {
	Get(context.Context, string) (T, error)
	Create(context.Context, T) error
	Update(context.Context, T) error
	Delete(context.Context, string) error
}

func NewTeleportResourceReconciler[T types.ResourceWithOrigin, K TeleportKubernetesResource[T]](
	client kclient.Client,
	resourceClient TeleportResourceClient[T],
	kubeResource K) *TeleportResourceReconciler[T, K] {

	reconciler := &TeleportResourceReconciler[T, K]{
		ResourceBaseReconciler: ResourceBaseReconciler{Client: client},
		resourceClient:         resourceClient,
		kubeResource:           kubeResource,
	}
	reconciler.ResourceBaseReconciler.UpsertExternal = reconciler.Upsert
	reconciler.ResourceBaseReconciler.DeleteExternal = reconciler.Delete
	return reconciler
}

func (r TeleportResourceReconciler[T, K]) Upsert(ctx context.Context, obj kclient.Object) error {
	k8sResource, ok := obj.(K)
	if !ok {
		return fmt.Errorf("failed to convert Object into resource object: %T", obj)
	}
	teleportResource := k8sResource.ToTeleport()

	existingResource, err := r.resourceClient.Get(ctx, teleportResource.GetName())
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	exists := !trace.IsNotFound(err)

	newOwnershipCondition, isOwned := checkOwnership(existingResource)
	meta.SetStatusCondition(k8sResource.StatusConditions(), newOwnershipCondition)
	if !isOwned {
		silentUpdateStatus(ctx, r.Client, k8sResource)
		return trace.AlreadyExists("unowned resource '%s' already exists", existingResource.GetName())
	}

	teleportResource.SetOrigin(types.OriginKubernetes)

	if !exists {
		err = r.resourceClient.Create(ctx, teleportResource)
	} else {
		/* TODO: handle modifier logic like CreatedBy for users,
		we can add mutate logic, diffing could also happen here */
		err = r.resourceClient.Update(ctx, teleportResource)
	}
	// If an error happens we want to put it in status.conditions before returning.
	newReconciliationCondition := getReconciliationConditionFromError(err)
	meta.SetStatusCondition(k8sResource.StatusConditions(), newReconciliationCondition)
	if err != nil {
		silentUpdateStatus(ctx, r.Client, k8sResource)
		return trace.Wrap(err)
	}

	// We update the status conditions on exit
	return trace.Wrap(r.Status().Update(ctx, k8sResource))
}
func (r TeleportResourceReconciler[T, K]) Delete(ctx context.Context, obj kclient.Object) error {
	return r.resourceClient.Delete(ctx, obj.GetName())
}

func (r TeleportResourceReconciler[T, K]) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return r.Do(ctx, req, r.kubeResource)
}

func (r TeleportResourceReconciler[T, K]) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(r.kubeResource).Complete(r)
}
