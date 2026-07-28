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
	"context"
	"fmt"
	"maps"
	"reflect"

	"github.com/gravitational/trace"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types/common"
	"github.com/gravitational/teleport/integrations/operator/controllers"
	"github.com/gravitational/teleport/lib/scopes"
)

const (
	// DeletionFinalizer is a name of finalizer added to Resource's 'finalizers' field
	// for tracking deletion events.
	DeletionFinalizer = "resources.teleport.dev/deletion"
	// AnnotationFlagIgnore is the Kubernetes annotation containing the "ignore" flag.
	// When set to true, the operator will not reconcile the CR.
	AnnotationFlagIgnore = "teleport.dev/ignore"
	// AnnotationFlagKeep is the Kubernetes annotation containing the "keep" flag.
	// When set to true, the operator will not delete the Teleport Resource if the
	// CR is deleted.
	AnnotationFlagKeep = "teleport.dev/keep"
)

const (
	OperatorIDLabel              = "resources.teleport.dev/operator-id"
	operatorOwnerLabel           = "resources.teleport.dev/owner-email"
	operatorTokenNameLabel       = "resources.teleport.dev/token-name"
	operatorNamespaceLabel       = "resources.teleport.dev/namespace"
	customResourceNameLabel      = "resources.teleport.dev/custom-resource-name"
	customResourceNamespaceLabel = "resources.teleport.dev/custom-resource-namespace"
	customResourceGVKLabel       = "resources.teleport.dev/custom-resource-gvk"

	ownershipIssueMissingOriginLabel  = "teleport resource doesn't have the " + common.OriginLabel + "label"
	ownershipIssueMismatchOriginLabel = "teleport resource has the " + common.OriginLabel + "label set to %q instead of \"" + common.OriginKubernetes + "\""
	ownershipIssueMissingOperatorID   = "teleport resource doesn't have the " + OperatorIDLabel + "label"
	ownershipIssueMismatchOperatorID  = "teleport resource has the " + OperatorIDLabel + "label set to %q instead of %q. Resource is owned by another operator. The resource label contain more info on the original operator."
)

// Resource is any Teleport Resource the controller manages.
type Resource any

// Adapter is an empty struct implementing helper functions for the reconciler
// to extract information from the Resource. This avoids having to implement the
// same interface on all resources. This became an issue as new resources are
// not implementing the types.Resource interface anymore.
type Adapter[T Resource] interface {
	GetResourceName(T) string
	GetResourceRevision(T) string
	SetResourceRevision(T, string)
	SetResourceLabels(T, map[string]string, OperatorMetadata, customResourceMetadata)
	CheckOwnership(T, OperatorMetadata) (bool, string)
}

// KubernetesCR is a Kubernetes CustomResource representing a Teleport Resource.
type KubernetesCR[T Resource] interface {
	kclient.Object
	ToTeleport() T
	StatusConditions() *[]metav1.Condition
}

// ResourceKey identifies a Teleport resource. Unscoped resources use an empty
// scope. Scoped resources are identified by the pair of name and scope.
type ResourceKey struct {
	Name  string
	Scope string
}

// String produces either the Scope Qualified Name if the resource is
// scoped, or the resource name if unscoped.
func (r ResourceKey) String() string {
	if r.Scope != "" {
		return scopes.QualifiedName{Scope: r.Scope, Name: r.Name}.String()
	}
	return r.Name
}

// resourceClient is a CRUD client for a specific Teleport Resource.
// Implementing this interface allows to be reconciled by the resourceReconciler
// instead of writing a new specific reconciliation loop.
// resourceClient implementations can optionally implement the resourceMutator
// and resourceMutator interfaces.
type resourceClient[T Resource] interface {
	Get(context.Context, ResourceKey) (T, error)
	Create(context.Context, T) error
	Update(context.Context, T) error
	Delete(context.Context, ResourceKey) error
}

// resourceMutator can be implemented by TeleportResourceClients
// to edit a Resource before its creation, or before its update based on the existing one.
type resourceMutator[T Resource] interface {
	Mutate(ctx context.Context, new, existing T, crKey kclient.ObjectKey) error
}

type Config struct {
	// Scoped represents if the controller reconciles scoped resources.
	// Scoped controllers always run.
	// Unscoped controllers don't run when the controller runs in scoped mode.
	// TODO(hugoShaka): Separate scoped config to distinguish is a resource scoped from
	// is the operator itself scoped.
	// Deprecated: Don't rely on this it is overloaded and will be refactored later.
	Scoped bool
	// CheckFeatures checks if the reconciler should run against the cluster given its features.
	// This is used to disable controllers if the cluster doesn't support their resource (e.g.
	// OSS clusters might not support enterprise resources).
	CheckFeatures controllers.CheckFeaturesFunc
}

// OperatorMetadata contains the metadata about the operator runtime and configuration.
// This is used to label resources and validate them (check their scope and provenance)
type OperatorMetadata struct {
	// Namespace is the operator namespace.
	Namespace string
	// ID is the operator ID.
	ID string
	// TokenName is the name of the token used by the operator to join.
	TokenName string
	// Scope is the operator scope. Set to `/` if the operator is unscoped.
	Scope string
	// Owner is the email of the operator owner. Specified by the user when deploying.
	Owner string
}

type customResourceMetadata struct {
	name      string
	namespace string
	gvk       string
}

// resourceReconciler is a Teleport generic reconciler.
type resourceReconciler[T any, K KubernetesCR[T]] struct {
	kubeClient       kclient.Client
	resourceClient   resourceClient[T]
	gvk              schema.GroupVersionKind
	adapter          Adapter[T]
	scoped           bool
	teleportKind     string
	operatorMetadata OperatorMetadata
	checkFeatures    controllers.CheckFeaturesFunc
}

func (r resourceReconciler[T, K]) GVK() schema.GroupVersionKind {
	return r.gvk
}

func (r resourceReconciler[T, K]) Scoped() bool {
	return r.scoped
}

func (r resourceReconciler[T, K]) CheckFeatures(features *proto.Features) bool {
	return r.checkFeatures(features)
}

func (r resourceReconciler[T, K]) TeleportKind() string {
	return r.teleportKind
}

// Upsert is the resourceReconciler of the ResourceBaseReconciler UpsertExternal
// It contains the logic to check if the Resource already exists, if it is owned by the operator and what
// to do to reconcile the Teleport Resource based on the Kubernetes one.
func (r resourceReconciler[T, K]) Upsert(ctx context.Context, obj kclient.Object) error {
	debugLog := ctrllog.FromContext(ctx).V(1)
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("failed to convert Object into Resource object: %T", obj)
	}
	k8sResource := newKubeResource[K]()
	debugLog.Info("Converting resource from unstructured", "crType", reflect.TypeOf(k8sResource))

	// If an error happen we want to put it in status.conditions before returning.
	err := runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(
		u.Object,
		k8sResource,
		true, /* returnUnknownFields */
	)

	// TODO(hugoShaka): optimize the updateStatus logic so we batch status updates
	updateErr := updateStatus(updateStatusConfig{
		ctx:         ctx,
		client:      r.kubeClient,
		k8sResource: k8sResource,
		condition:   getStructureConditionFromError(err),
	})
	if err != nil || updateErr != nil {
		return trace.NewAggregate(err, updateErr)
	}

	teleportResource := k8sResource.ToTeleport()

	debugLog.Info("Converting resource to teleport")
	key := r.resourceKey(teleportResource)

	scopeCondition, scopeOK := r.checkScope(key, r.operatorMetadata)
	updateErr = updateStatus(updateStatusConfig{
		ctx:         ctx,
		client:      r.kubeClient,
		k8sResource: k8sResource,
		condition:   scopeCondition,
	})
	if updateErr != nil {
		return trace.Wrap(updateErr, "updating status scope condition")
	}
	if !scopeOK {
		return trace.CompareFailed("%s", scopeCondition.Message)
	}

	existingResource, err := r.resourceClient.Get(ctx, key)
	updateErr = updateStatus(updateStatusConfig{
		ctx:         ctx,
		client:      r.kubeClient,
		k8sResource: k8sResource,
		condition:   getReconciliationConditionFromError(err, true /* ignoreNotFound */),
	})

	if err != nil && !trace.IsNotFound(err) || updateErr != nil {
		return trace.NewAggregate(err, updateErr)
	}
	// If err is nil, we found the Resource. If err != nil (and we did return), then the error was `NotFound`
	exists := err == nil

	if exists {
		debugLog.Info("Resource already exists")
		newOwnershipCondition, isOwned := r.checkOwnership(existingResource, r.operatorMetadata)
		debugLog.Info("Resource is owned")
		if updateErr = updateStatus(updateStatusConfig{
			ctx:         ctx,
			client:      r.kubeClient,
			k8sResource: k8sResource,
			condition:   newOwnershipCondition,
		}); updateErr != nil {
			return trace.Wrap(updateErr)
		}
		if !isOwned {
			return trace.AlreadyExists("unowned Resource %q already exists", key)
		}
	} else {
		debugLog.Info("Resource does not exist yet")
		if updateErr = updateStatus(updateStatusConfig{
			ctx:         ctx,
			client:      r.kubeClient,
			k8sResource: k8sResource,
			condition:   newResourceCondition,
		}); updateErr != nil {
			return trace.Wrap(updateErr)
		}
	}

	kubeLabels := obj.GetLabels()
	teleportLabels := make(map[string]string)
	maps.Copy(teleportLabels, kubeLabels)
	r.adapter.SetResourceLabels(
		teleportResource,
		teleportLabels,
		r.operatorMetadata,
		customResourceMetadata{name: obj.GetName(), namespace: obj.GetNamespace(), gvk: r.gvk.String()},
	)
	debugLog.Info("Propagating labels from kube resource", "kubeLabels", kubeLabels, "teleportLabels", teleportLabels)

	if mutator, ok := r.resourceClient.(resourceMutator[T]); ok {
		debugLog.Info("Mutating resource")
		objKey := kclient.ObjectKeyFromObject(k8sResource)
		if err := mutator.Mutate(ctx, teleportResource, existingResource, objKey); err != nil {
			// If an error happens we want to put it in status.conditions before returning.
			updateErr = updateStatus(updateStatusConfig{
				ctx:         ctx,
				client:      r.kubeClient,
				k8sResource: k8sResource,
				condition: metav1.Condition{
					Type:    ConditionTypeSuccessfullyReconciled,
					Status:  metav1.ConditionFalse,
					Reason:  ConditionReasonMutationError,
					Message: fmt.Sprintf("The reconciliation failed, the operator failed to mutate the resource before creating it in Teleport. Mutation failed with error: %s", err),
				},
			})

			return trace.NewAggregate(err, updateErr)
		}
	}

	if !exists {
		// This is a new Resource
		err = r.resourceClient.Create(ctx, teleportResource)
	} else {
		// This is a Resource update, we must propagate the revision
		currentRevision := r.adapter.GetResourceRevision(existingResource)
		r.adapter.SetResourceRevision(teleportResource, currentRevision)
		debugLog.Info("Propagating revision", "currentRevision", currentRevision)

		err = r.resourceClient.Update(ctx, teleportResource)
	}
	// If an error happens we want to put it in status.conditions before returning.
	updateErr = updateStatus(updateStatusConfig{
		ctx:         ctx,
		client:      r.kubeClient,
		k8sResource: k8sResource,
		condition:   getReconciliationConditionFromError(err, false /* ignoreNotFound */),
	})

	return trace.NewAggregate(err, updateErr)
}

func (r resourceReconciler[T, K]) resourceKey(resource T) ResourceKey {
	key := ResourceKey{Name: r.adapter.GetResourceName(resource)}

	if scopedAdapter, ok := r.adapter.(interface{ GetResourceScope(T) string }); ok {
		key.Scope = scopedAdapter.GetResourceScope(resource)
	}
	return key
}

// Delete is the resourceReconciler of the ResourceBaseReconciler DeleteExertal
func (r resourceReconciler[T, K]) Delete(ctx context.Context, obj kclient.Object) error {
	key := ResourceKey{Name: obj.GetName()}
	if r.scoped {
		// Unmarshaling is avoided by pulling the scope directly out of the Object.
		u, ok := obj.(*unstructured.Unstructured)
		if !ok {
			return trace.BadParameter("failed to convert Object into Resource object: %T", obj)
		}

		scope, _, err := unstructured.NestedString(u.Object, "scope")
		if err != nil {
			return trace.Wrap(err)
		}

		key.Scope = scope
	}

	if condition, ok := r.checkScope(key, r.operatorMetadata); !ok {
		// If the scope doesn't match, we must not delete the resource on the Teleport side.
		// Then we have 2 choices:
		// - error continually until the user manually adds the "keep" label.
		// - silently skp the deletion to let the CR be removed.
		// As it's unlikely that the operator was managing the resource to begin with, the second option seems saner.
		// There's a small risk of an operator changing scope, the leaving leftovers. Today we cannot detect this edge
		// case, a potential workaround would be to introduce something in the CR status to track if it was reconciled
		// once, and keep the last known SQN.
		log := ctrllog.FromContext(ctx).V(0)
		log.Info("Scope mismatch, skipping deletion", "reason", condition.Reason, "operatorScope", r.operatorMetadata.Scope, "resourceScope", key.Scope, "resourceName", key.Name, "resourceNamespace", obj.GetNamespace())
		return nil
	}

	// This call catches non-existing resources or subkind mismatch (e.g. openssh nodes)
	// We can then check that we own the Resource before deleting it.
	resource, err := r.resourceClient.Get(ctx, key)
	if err != nil {
		return trace.Wrap(err)
	}

	_, isOwned := r.checkOwnership(resource, r.operatorMetadata)
	if !isOwned {
		// The Resource doesn't belong to us, we bail out but unblock the CR deletion
		return nil
	}
	// This GET->check->DELETE dance is race-prone, but it's good enough for what
	// we want to do. No one should reconcile the same Resource as the operator.
	// If they do, it's their fault as the Resource was clearly flagged as belonging to us.
	return r.resourceClient.Delete(ctx, key)
}

// Reconcile receives an update request and reconcile the Resource,
// it implements the controllers.Reconciler interface.
//
// When an event arrives we must propagate that change into the Teleport cluster.
// We have two types of events: update/create and delete.
//
// For creating/updating we check if the Resource exists in Teleport
// - if it does, we update it
// - otherwise we create it
// Always using the state of the Resource in the cluster as the source of truth.
//
// For deleting, the recommendation is to use finalizers.
// Finalizers allow us to map an external Resource to a kubernetes Resource.
// So, when we create or update a Resource, we add our own finalizer to the kubernetes Resource list of finalizers.
//
// For a delete event which has our finalizer: the Resource is deleted in Teleport.
// If it doesn't have the finalizer, we do nothing.
//
// ----
//
// Every time we update a Resource in Kubernetes (adding finalizers or the OriginLabel), we end the reconciliation process.
// Afterwards, we receive the request again and we progress to the next step.
// This allow us to progress with smaller changes and avoid a long-running reconciliation.
// */
func (r resourceReconciler[T, K]) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	obj, err := GetUnstructuredObjectFromGVK(r.gvk)
	if err != nil {
		return ctrl.Result{}, trace.Wrap(err, "creating object in which the CR will be unmarshalled")
	}
	// https://sdk.operatorframework.io/docs/building-operators/golang/advanced-topics/#external-resources
	log := ctrllog.FromContext(ctx).WithValues("namespacedname", req.NamespacedName)

	if err := r.kubeClient.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("not found")
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to get Resource")
		return ctrl.Result{}, trace.Wrap(err)
	}

	if isIgnored(obj) {
		log.Info(fmt.Sprintf("Resource is flagged with annotation %q, it will not be reconciled.", AnnotationFlagIgnore))
		return ctrl.Result{}, nil
	}

	hasDeletionFinalizer := controllerutil.ContainsFinalizer(obj, DeletionFinalizer)
	isMarkedToBeDeleted := !obj.GetDeletionTimestamp().IsZero()

	// Delete
	if isMarkedToBeDeleted {
		if hasDeletionFinalizer {
			if isKept(obj) {
				log.Info(fmt.Sprintf("Resource is flagged with annotation %q, it will not be deleted in Teleport.", AnnotationFlagKeep))
			} else {
				log.Info("deleting object in Teleport")
				if err := r.Delete(ctx, obj); err != nil && !trace.IsNotFound(err) {
					return ctrl.Result{}, trace.Wrap(err)
				}
			}

			log.Info("removing finalizer")
			controllerutil.RemoveFinalizer(obj, DeletionFinalizer)
			if err := r.kubeClient.Update(ctx, obj); err != nil {
				return ctrl.Result{}, trace.Wrap(err, "failed to remove finalizer after deleting in teleport")
			}
		}

		// marked to be deleted without finalizer
		return ctrl.Result{}, nil
	}

	if !hasDeletionFinalizer {
		log.Info("adding finalizer")
		controllerutil.AddFinalizer(obj, DeletionFinalizer)

		err := r.kubeClient.Update(ctx, obj)

		return ctrl.Result{}, trace.Wrap(err, "failed to add finalizer")
	}

	// Create or update
	log.Info("upsert object in Teleport")
	err = r.Upsert(ctx, obj)
	return ctrl.Result{}, trace.Wrap(err)
}

// SetupWithManager implements the controllers.Reconciler interface.
func (r resourceReconciler[T, K]) SetupWithManager(mgr ctrl.Manager) error {
	// The resourceReconciler uses unstructured objects because of a silly json marshaling
	// issue. Teleport's utils.String is a list of strings, but marshals as a single string if there's a single item.
	// This is a questionable design as it breaks the openapi schema, but we're stuck with it. We had to relax openapi
	// validation in those CRD fields, and use an unstructured object for the client, else JSON unmarshalling fails.
	obj, err := GetUnstructuredObjectFromGVK(r.gvk)
	if err != nil {
		return trace.Wrap(err, "creating the model object for the manager watcher/client")
	}
	return ctrl.
		NewControllerManagedBy(mgr).
		For(obj).
		WithEventFilter(
			buildPredicate(),
		).
		Complete(r)
}

// isIgnored checks if the CR should be ignored
func isIgnored(obj kclient.Object) bool {
	return checkAnnotationFlag(obj, AnnotationFlagIgnore, false /* defaults to false */)
}

// isKept checks if the Teleport Resource should be kept if the CR is deleted
func isKept(obj kclient.Object) bool {
	return checkAnnotationFlag(obj, AnnotationFlagKeep, false /* defaults to false */)
}

// checkOwnership takes an existing Resource and validates the operator owns it.
// It returns an ownership condition and a boolean representing if the Resource is
// owned by the operator. The ownedResource must be non-nil.
func (r resourceReconciler[T, K]) checkOwnership(existingResource T, metadata OperatorMetadata) (metav1.Condition, bool) {
	if ok, reason := r.adapter.CheckOwnership(existingResource, metadata); !ok {
		// Existing Teleport Resource does not belong to us, bailing out

		condition := metav1.Condition{
			Type:    ConditionTypeTeleportResourceOwned,
			Status:  metav1.ConditionFalse,
			Reason:  ConditionReasonOriginLabelNotMatching,
			Message: fmt.Sprintf("A resource with the same name already exists in Teleport and is not owned by the operator (%s). Refusing to reconcile.", reason),
		}
		return condition, false
	}

	condition := metav1.Condition{
		Type:    ConditionTypeTeleportResourceOwned,
		Status:  metav1.ConditionTrue,
		Reason:  ConditionReasonOriginLabelMatching,
		Message: "Teleport resource is owned by the operator.",
	}
	return condition, true
}

// checkOwnership takes an existing Resource and validates the operator owns it.
// It returns an ownership condition and a boolean representing if the Resource is
// owned by the operator. The ownedResource must be non-nil.
func (r resourceReconciler[T, K]) checkScope(key ResourceKey, metadata OperatorMetadata) (metav1.Condition, bool) {
	switch {
	case key.Scope == "" && metadata.Scope == "":
		return metav1.Condition{
			Type:    ConditionTypeValidScope,
			Status:  metav1.ConditionTrue,
			Reason:  ConditionTypeUnscoped,
			Message: "Neither resource or operator are scoped",
		}, true
	case key.Scope != "" && metadata.Scope == "":
		return metav1.Condition{
			Type:    ConditionTypeValidScope,
			Status:  metav1.ConditionFalse,
			Reason:  ConditionReasonNonMatchingScope,
			Message: "Resource is scoped but operator is not. Refusing to reconcile.",
		}, false
	case key.Scope == "" && metadata.Scope != "":
		return metav1.Condition{
			Type:    ConditionTypeValidScope,
			Status:  metav1.ConditionFalse,
			Reason:  ConditionReasonNonMatchingScope,
			Message: "Operator is scoped but resource is not. Refusing to reconcile.",
		}, false
	case metadata.Scope == key.Scope:
		return metav1.Condition{
			Type:    ConditionTypeValidScope,
			Status:  metav1.ConditionTrue,
			Reason:  ConditionReasonMatchingScope,
			Message: "Resource scope matches the operator scope.",
		}, true
	default:
		return metav1.Condition{
			Type:    ConditionTypeValidScope,
			Status:  metav1.ConditionFalse,
			Reason:  ConditionReasonNonMatchingScope,
			Message: fmt.Sprintf("Resource scope %q does not match the operator scope %q.", key.Scope, metadata.Scope),
		}, false
	}
}

var newResourceCondition = metav1.Condition{
	Type:    ConditionTypeTeleportResourceOwned,
	Status:  metav1.ConditionTrue,
	Reason:  ConditionReasonNewResource,
	Message: "No existing Teleport Resource found with that name. The created Resource is owned by the operator.",
}
