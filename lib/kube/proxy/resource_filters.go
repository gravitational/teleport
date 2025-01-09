/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package proxy

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"mime"
	"net/http"

	"github.com/gravitational/trace"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	networkingv1 "k8s.io/api/networking/v1"
	authv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/kube/proxy/responsewriters"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/slices"
)

// newResourceFilterer creates a wrapper function that once executed creates
// a runtime filter for kubernetes resources.
// The filter exclusion criteria is:
// - deniedResources: excluded if (namespace,name) matches an entry even if it matches
// the allowedResources's list.
// - allowedResources: excluded if (namespace,name) not match a single entry.
func newResourceFilterer(kind, verb string, codecs *serializer.CodecFactory, allowedResources, deniedResources []types.KubernetesResource, log *slog.Logger) responsewriters.FilterWrapper {
	// If the list of allowed resources contains a wildcard and no deniedResources, then we
	// don't need to filter anything.
	if containsWildcard(allowedResources) && len(deniedResources) == 0 {
		return nil
	}
	return func(contentType string, responseCode int) (responsewriters.Filter, error) {
		negotiator := newClientNegotiator(codecs)
		encoder, decoder, err := newEncoderAndDecoderForContentType(contentType, negotiator)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &resourceFilterer{
			encoder:          encoder,
			decoder:          decoder,
			contentType:      contentType,
			responseCode:     responseCode,
			negotiator:       negotiator,
			allowedResources: allowedResources,
			deniedResources:  deniedResources,
			log:              log,
			kind:             kind,
			verb:             verb,
		}, nil
	}
}

// wildcardFilter is a filter that matches all pods.
var wildcardFilter = types.KubernetesResource{
	Kind:      types.Wildcard,
	Namespace: types.Wildcard,
	Name:      types.Wildcard,
	Verbs:     []string{types.Wildcard},
}

// containsWildcard returns true if the list of resources contains a wildcard filter.
func containsWildcard(resources []types.KubernetesResource) bool {
	for _, r := range resources {
		if r.Kind == wildcardFilter.Kind &&
			r.Name == wildcardFilter.Name &&
			r.Namespace == wildcardFilter.Namespace &&
			len(r.Verbs) == 1 && r.Verbs[0] == wildcardFilter.Verbs[0] {
			return true
		}
	}
	return false
}

// resourceFilterer is a resource filterer instance.
type resourceFilterer struct {
	encoder runtime.Encoder
	decoder runtime.Decoder
	// contentType is the response "Content-Type" header.
	contentType string
	// responseCode is the response status code.
	responseCode int
	// negotiator is an instance of a client negotiator.
	negotiator runtime.ClientNegotiator
	// allowedResources is the list of kubernetes resources the user has access to.
	allowedResources []types.KubernetesResource
	// deniedResources is the list of kubernetes resources the user must not access.
	deniedResources []types.KubernetesResource
	// log is the logger.
	log *slog.Logger
	// kind is the type of the resource.
	kind string
	// verb is the kube API verb based on HTTP verb.
	verb string
}

// FilterBuffer receives a byte array, decodes the response into the appropriate
// type and filters the resources based on allowed and denied rules configured.
// After filtering them, it serializes the response and dumps it into output buffer.
// If any error occurs, the call returns an error.
func (d *resourceFilterer) FilterBuffer(buf []byte, output io.Writer) error {
	// decode the response into the appropriate Kubernetes API type.
	obj, bf, err := d.decode(buf)
	if err != nil {
		return trace.Wrap(err)
	}
	// if bf is not empty, it means that response does not contain any valid response
	// and it should be safe to write it back into the buffer.
	if len(bf) > 0 {
		_, err = output.Write(buf)
		return trace.Wrap(err)
	}

	switch allowed, isList, err := d.FilterObj(obj); {
	case err != nil:
		return trace.Wrap(err)
	case isList:
	case !allowed:
		// if the object is not a list and it's not allowed, then we should
		// return an error.
		return trace.AccessDenied("access denied")
	}
	// encode the filterer response back to the user.
	return d.encode(obj, output)
}

// FilterObj receives a runtime.Object type and filters the resources on it
// based on allowed and denied rules.
// After filtering them, the obj is manipulated to hold the filtered information.
// The isAllowed boolean returned indicates if the client is allowed to receive the event
// with the object.
// The isListObj boolean returned indicates if the object is a list of resources.
func (d *resourceFilterer) FilterObj(obj runtime.Object) (isAllowed bool, isList bool, err error) {
	ctx := context.Background()

	switch o := obj.(type) {
	case *metav1.Status:
		// Status object is returned when the Kubernetes API returns an error and
		// should be forwarded to the user.
		return true, false, nil
	case *corev1.Pod:
		result, err := filterResource(d.kind, d.verb, o, d.allowedResources, d.deniedResources)
		if err != nil {
			d.log.WarnContext(ctx, "Unable to compile regex expressions within kubernetes_resources", "error", err)
		}
		// if err is not nil or result is false, we should not include it.
		return result, false, nil
	case *corev1.PodList:
		o.Items = slices.FromPointers(
			filterResourceList(
				d.kind, d.verb,
				slices.ToPointers(o.Items), d.allowedResources, d.deniedResources, d.log),
		)
		return len(o.Items) > 0, true, nil
	case *corev1.Secret:
		result, err := filterResource(d.kind, d.verb, o, d.allowedResources, d.deniedResources)
		if err != nil {
			d.log.WarnContext(ctx, "Unable to compile regex expressions within kubernetes_resources", "error", err)
		}
		// if err is not nil or result is false, we should not include it.
		return result, false, nil
	case *corev1.SecretList:
		o.Items = slices.FromPointers(
			filterResourceList(
				d.kind, d.verb,
				slices.ToPointers(o.Items), d.allowedResources, d.deniedResources, d.log),
		)
		return len(o.Items) > 0, true, nil
	case *corev1.ConfigMap:
		result, err := filterResource(d.kind, d.verb, o, d.allowedResources, d.deniedResources)
		if err != nil {
			d.log.WarnContext(ctx, "Unable to compile regex expressions within kubernetes_resources", "error", err)
		}
		// if err is not nil or result is false, we should not include it.
		return result, false, nil
	case *corev1.ConfigMapList:
		o.Items = slices.FromPointers(
			filterResourceList(
				d.kind, d.verb,
				slices.ToPointers(o.Items), d.allowedResources, d.deniedResources, d.log),
		)
		return len(o.Items) > 0, true, nil
	case *corev1.Namespace:
		result, err := filterResource(d.kind, d.verb, o, d.allowedResources, d.deniedResources)
		if err != nil {
			d.log.WarnContext(ctx, "Unable to compile regex expressions within kubernetes_resources", "error", err)
		}
		// if err is not nil or result is false, we should not include it.
		return result, false, nil
	case *corev1.NamespaceList:
		o.Items = slices.FromPointers(
			filterResourceList(
				d.kind, d.verb,
				slices.ToPointers(o.Items), d.allowedResources, d.deniedResources, d.log),
		)
		return len(o.Items) > 0, true, nil
	case *corev1.Service:
		result, err := filterResource(d.kind, d.verb, o, d.allowedResources, d.deniedResources)
		if err != nil {
			d.log.WarnContext(ctx, "Unable to compile regex expressions within kubernetes_resources", "error", err)
		}
		// if err is not nil or result is false, we should not include it.
		return result, false, nil
	case *corev1.ServiceList:
		o.Items = slices.FromPointers(
			filterResourceList(
				d.kind, d.verb,
				slices.ToPointers(o.Items), d.allowedResources, d.deniedResources, d.log),
		)
		return len(o.Items) > 0, true, nil
	case *corev1.Endpoints:
		result, err := filterResource(d.kind, d.verb, o, d.allowedResources, d.deniedResources)
		if err != nil {
			d.log.WarnContext(ctx, "Unable to compile regex expressions within kubernetes_resources", "error", err)
		}
		// if err is not nil or result is false, we should not include it.
		return result, false, nil
	case *corev1.EndpointsList:
		o.Items = slices.FromPointers(
			filterResourceList(
				d.kind, d.verb,
				slices.ToPointers(o.Items), d.allowedResources, d.deniedResources, d.log),
		)
		return len(o.Items) > 0, true, nil
	case *corev1.ServiceAccount:
		result, err := filterResource(d.kind, d.verb, o, d.allowedResources, d.deniedResources)
		if err != nil {
			d.log.WarnContext(ctx, "Unable to compile regex expressions within kubernetes_resources", "error", err)
		}
		// if err is not nil or result is false, we should not include it.
		return result, false, nil
	case *corev1.ServiceAccountList:
		o.Items = slices.FromPointers(
			filterResourceList(
				d.kind, d.verb,
				slices.ToPointers(o.Items), d.allowedResources, d.deniedResources, d.log),
		)
		return len(o.Items) > 0, true, nil
	case *corev1.Node:
		result, err := filterResource(d.kind, d.verb, o, d.allowedResources, d.deniedResources)
		if err != nil {
			d.log.WarnContext(ctx, "Unable to compile regex expressions within kubernetes_resources", "error", err)
		}
		// if err is not nil or result is false, we should not include it.
		return result, false, nil
	case *corev1.NodeList:
		o.Items = slices.FromPointers(
			filterResourceList(
				d.kind, d.verb,
				slices.ToPointers(o.Items), d.allowedResources, d.deniedResources, d.log),
		)
		return len(o.Items) > 0, true, nil
	case *corev1.PersistentVolume:
		result, err := filterResource(d.kind, d.verb, o, d.allowedResources, d.deniedResources)
		if err != nil {
			d.log.WarnContext(ctx, "Unable to compile regex expressions within kubernetes_resources", "error", err)
		}
		// if err is not nil or result is false, we should not include it.
		return result, false, nil
	case *corev1.PersistentVolumeList:
		o.Items = slices.FromPointers(
			filterResourceList(
				d.kind, d.verb,
				slices.ToPointers(o.Items), d.allowedResources, d.deniedResources, d.log),
		)
		return len(o.Items) > 0, true, nil
	case *corev1.PersistentVolumeClaim:
		result, err := filterResource(d.kind, d.verb, o, d.allowedResources, d.deniedResources)
		if err != nil {
			d.log.WarnContext(ctx, "Unable to compile regex expressions within kubernetes_resources", "error", err)
		}
		// if err is not nil or result is false, we should not include it.
		return result, false, nil
	case *corev1.PersistentVolumeClaimList:
		o.Items = slices.FromPointers(
			filterResourceList(
				d.kind, d.verb,
				slices.ToPointers(o.Items), d.allowedResources, d.deniedResources, d.log),
		)
		return len(o.Items) > 0, true, nil

	case *appsv1.Deployment:
		result, err := filterResource(d.kind, d.verb, o, d.allowedResources, d.deniedResources)
		if err != nil {
			d.log.WarnContext(ctx, "Unable to compile regex expressions within kubernetes_resources", "error", err)
		}
		// if err is not nil or result is false, we should not include it.
		return result, false, nil
	case *appsv1.DeploymentList:
		o.Items = slices.FromPointers(
			filterResourceList(
				d.kind, d.verb,
				slices.ToPointers(o.Items), d.allowedResources, d.deniedResources, d.log),
		)
		return len(o.Items) > 0, true, nil

	case *appsv1.ReplicaSet:
		result, err := filterResource(d.kind, d.verb, o, d.allowedResources, d.deniedResources)
		if err != nil {
			d.log.WarnContext(ctx, "Unable to compile regex expressions within kubernetes_resources", "error", err)
		}
		// if err is not nil or result is false, we should not include it.
		return result, false, nil
	case *appsv1.ReplicaSetList:
		o.Items = slices.FromPointers(
			filterResourceList(
				d.kind, d.verb,
				slices.ToPointers(o.Items), d.allowedResources, d.deniedResources, d.log),
		)
		return len(o.Items) > 0, true, nil
	case *appsv1.StatefulSet:
		result, err := filterResource(d.kind, d.verb, o, d.allowedResources, d.deniedResources)
		if err != nil {
			d.log.WarnContext(ctx, "Unable to compile regex expressions within kubernetes_resources", "error", err)
		}
		// if err is not nil or result is false, we should not include it.
		return result, false, nil
	case *appsv1.StatefulSetList:
		o.Items = slices.FromPointers(
			filterResourceList(
				d.kind, d.verb,
				slices.ToPointers(o.Items), d.allowedResources, d.deniedResources, d.log),
		)
		return len(o.Items) > 0, true, nil

	case *appsv1.DaemonSet:
		result, err := filterResource(d.kind, d.verb, o, d.allowedResources, d.deniedResources)
		if err != nil {
			d.log.WarnContext(ctx, "Unable to compile regex expressions within kubernetes_resources", "error", err)
		}
		// if err is not nil or result is false, we should not include it.
		return result, false, nil
	case *appsv1.DaemonSetList:
		o.Items = slices.FromPointers(
			filterResourceList(
				d.kind, d.verb,
				slices.ToPointers(o.Items), d.allowedResources, d.deniedResources, d.log),
		)
		return len(o.Items) > 0, true, nil
	case *authv1.ClusterRole:
		result, err := filterResource(d.kind, d.verb, o, d.allowedResources, d.deniedResources)
		if err != nil {
			d.log.WarnContext(ctx, "Unable to compile regex expressions within kubernetes_resources", "error", err)
		}
		// if err is not nil or result is false, we should not include it.
		return result, false, nil
	case *authv1.ClusterRoleList:
		o.Items = slices.FromPointers(
			filterResourceList(
				d.kind, d.verb,
				slices.ToPointers(o.Items), d.allowedResources, d.deniedResources, d.log),
		)
		return len(o.Items) > 0, true, nil
	case *authv1.Role:
		result, err := filterResource(d.kind, d.verb, o, d.allowedResources, d.deniedResources)
		if err != nil {
			d.log.WarnContext(ctx, "Unable to compile regex expressions within kubernetes_resources", "error", err)
		}
		// if err is not nil or result is false, we should not include it.
		return result, false, nil
	case *authv1.RoleList:
		o.Items = slices.FromPointers(
			filterResourceList(
				d.kind, d.verb,
				slices.ToPointers(o.Items), d.allowedResources, d.deniedResources, d.log),
		)
		return len(o.Items) > 0, true, nil

	case *authv1.ClusterRoleBinding:
		result, err := filterResource(d.kind, d.verb, o, d.allowedResources, d.deniedResources)
		if err != nil {
			d.log.WarnContext(ctx, "Unable to compile regex expressions within kubernetes_resources", "error", err)
		}
		// if err is not nil or result is false, we should not include it.
		return result, false, nil
	case *authv1.ClusterRoleBindingList:
		o.Items = slices.FromPointers(
			filterResourceList(
				d.kind, d.verb,
				slices.ToPointers(o.Items), d.allowedResources, d.deniedResources, d.log),
		)
		return len(o.Items) > 0, true, nil

	case *authv1.RoleBinding:
		result, err := filterResource(d.kind, d.verb, o, d.allowedResources, d.deniedResources)
		if err != nil {
			d.log.WarnContext(ctx, "Unable to compile regex expressions within kubernetes_resources", "error", err)
		}
		// if err is not nil or result is false, we should not include it.
		return result, false, nil
	case *authv1.RoleBindingList:
		o.Items = slices.FromPointers(
			filterResourceList(
				d.kind, d.verb,
				slices.ToPointers(o.Items), d.allowedResources, d.deniedResources, d.log),
		)
		return len(o.Items) > 0, true, nil

	case *batchv1.CronJob:
		result, err := filterResource(d.kind, d.verb, o, d.allowedResources, d.deniedResources)
		if err != nil {
			d.log.WarnContext(ctx, "Unable to compile regex expressions within kubernetes_resources", "error", err)
		}
		// if err is not nil or result is false, we should not include it.
		return result, false, nil
	case *batchv1.CronJobList:
		o.Items = slices.FromPointers(
			filterResourceList(
				d.kind, d.verb,
				slices.ToPointers(o.Items), d.allowedResources, d.deniedResources, d.log),
		)
		return len(o.Items) > 0, true, nil

	case *batchv1.Job:
		result, err := filterResource(d.kind, d.verb, o, d.allowedResources, d.deniedResources)
		if err != nil {
			d.log.WarnContext(ctx, "Unable to compile regex expressions within kubernetes_resources", "error", err)
		}
		// if err is not nil or result is false, we should not include it.
		return result, false, nil
	case *batchv1.JobList:
		o.Items = slices.FromPointers(
			filterResourceList(
				d.kind, d.verb,
				slices.ToPointers(o.Items), d.allowedResources, d.deniedResources, d.log),
		)
		return len(o.Items) > 0, true, nil

	case *certificatesv1.CertificateSigningRequest:
		result, err := filterResource(d.kind, d.verb, o, d.allowedResources, d.deniedResources)
		if err != nil {
			d.log.WarnContext(ctx, "Unable to compile regex expressions within kubernetes_resources", "error", err)
		}
		// if err is not nil or result is false, we should not include it.
		return result, false, nil
	case *certificatesv1.CertificateSigningRequestList:
		o.Items = slices.FromPointers(
			filterResourceList(
				d.kind, d.verb,
				slices.ToPointers(o.Items), d.allowedResources, d.deniedResources, d.log),
		)
		return len(o.Items) > 0, true, nil
	case *networkingv1.Ingress:
		result, err := filterResource(d.kind, d.verb, o, d.allowedResources, d.deniedResources)
		if err != nil {
			d.log.WarnContext(ctx, "Unable to compile regex expressions within kubernetes_resources", "error", err)
		}
		// if err is not nil or result is false, we should not include it.
		return result, false, nil
	case *networkingv1.IngressList:
		o.Items = slices.FromPointers(
			filterResourceList(
				d.kind, d.verb,
				slices.ToPointers(o.Items), d.allowedResources, d.deniedResources, d.log),
		)
		return len(o.Items) > 0, true, nil
	case *extensionsv1beta1.Ingress:
		result, err := filterResource(d.kind, d.verb, o, d.allowedResources, d.deniedResources)
		if err != nil {
			d.log.WarnContext(ctx, "Unable to compile regex expressions within kubernetes_resources", "error", err)
		}
		// if err is not nil or result is false, we should not include it.
		return result, false, nil
	case *extensionsv1beta1.IngressList:
		o.Items = slices.FromPointers(
			filterResourceList(
				d.kind, d.verb,
				slices.ToPointers(o.Items), d.allowedResources, d.deniedResources, d.log),
		)
		return len(o.Items) > 0, true, nil

	case *extensionsv1beta1.DaemonSet:
		result, err := filterResource(d.kind, d.verb, o, d.allowedResources, d.deniedResources)
		if err != nil {
			d.log.WarnContext(ctx, "Unable to compile regex expressions within kubernetes_resources", "error", err)
		}
		// if err is not nil or result is false, we should not include it.
		return result, false, nil
	case *extensionsv1beta1.DaemonSetList:
		o.Items = slices.FromPointers(
			filterResourceList(
				d.kind, d.verb,
				slices.ToPointers(o.Items), d.allowedResources, d.deniedResources, d.log),
		)
		return len(o.Items) > 0, true, nil

	case *extensionsv1beta1.Deployment:
		result, err := filterResource(d.kind, d.verb, o, d.allowedResources, d.deniedResources)
		if err != nil {
			d.log.WarnContext(ctx, "Unable to compile regex expressions within kubernetes_resources", "error", err)
		}
		// if err is not nil or result is false, we should not include it.
		return result, false, nil
	case *extensionsv1beta1.DeploymentList:
		o.Items = slices.FromPointers(
			filterResourceList(
				d.kind, d.verb,
				slices.ToPointers(o.Items), d.allowedResources, d.deniedResources, d.log),
		)
		return len(o.Items) > 0, true, nil

	case *extensionsv1beta1.ReplicaSet:
		result, err := filterResource(d.kind, d.verb, o, d.allowedResources, d.deniedResources)
		if err != nil {
			d.log.WarnContext(ctx, "Unable to compile regex expressions within kubernetes_resources", "error", err)
		}
		// if err is not nil or result is false, we should not include it.
		return result, false, nil
	case *extensionsv1beta1.ReplicaSetList:
		o.Items = slices.FromPointers(
			filterResourceList(
				d.kind, d.verb,
				slices.ToPointers(o.Items), d.allowedResources, d.deniedResources, d.log),
		)
		return len(o.Items) > 0, true, nil

	case *unstructured.Unstructured:
		if o.IsList() {
			hasElemts := filterUnstructuredList(d.verb, o, d.allowedResources, d.deniedResources, d.log)
			return hasElemts, true, nil
		}

		r := getKubeResource(utils.KubeCustomResource, d.verb, o)
		result, err := matchKubernetesResource(
			r,
			d.allowedResources, d.deniedResources,
		)
		if err != nil {
			d.log.WarnContext(ctx, "Unable to compile regex expressions within kubernetes_resources", "error", err)
		}
		// if err is not nil or result is false, we should not include it.
		return result, false, nil

	case *metav1.Table:
		_, err := d.filterMetaV1Table(o, d.allowedResources, d.deniedResources)
		if err != nil {
			return false, false, trace.Wrap(err)
		}
		return len(o.Rows) > 0, true, nil
	default:
		// It's important default types are never blindly forwarded or protocol
		// extensions could result in information disclosures.
		return false, false, trace.BadParameter("unexpected type received; got %T", obj)
	}
}

// decode decodes the buffer into the appropriate type if the responseCode
// belongs to the range 200(OK)-206(PartialContent).
// If it does not belong, it returns the buffer unchanged since it contains
// an error message from the Kubernetes API server and it's safe to return
// it back to the user.
func (d *resourceFilterer) decode(buffer []byte) (runtime.Object, []byte, error) {
	switch {
	case d.responseCode == http.StatusSwitchingProtocols:
		// no-op, we've been upgraded
		return nil, buffer, nil
	case d.responseCode < http.StatusOK /* 200 */ || d.responseCode > http.StatusPartialContent /* 206 */ :
		// calculate an unstructured error from the response which the Result object may use if the caller
		// did not return a structured error.
		// Logic from: https://github.com/kubernetes/client-go/blob/58ff029093df37cad9fa28778a37f11fa495d9cf/rest/request.go#L1040
		return nil, buffer, nil
	default:
		// We are reading an API request and API honors the GVK in the request so we don't
		// need to set it.
		out, err := decodeAndSetGVK(d.decoder, buffer, nil /* defaults GVK */)
		return out, nil, trace.Wrap(err)
	}
}

// decodePartialObjectMetadata decodes the metav1.PartialObjectMetadata present
// in the metav1.TableRow entry. This information comes from server side and
// includes the resource name and namespace as a structured object.
func (d *resourceFilterer) decodePartialObjectMetadata(row *metav1.TableRow) error {
	if row.Object.Object != nil {
		return nil
	}
	var err error
	// decode only if row.Object.Object was not decoded before.
	// We are reading an API request and API honors the GVK in the request so we don't
	// need to set it.
	row.Object.Object, err = decodeAndSetGVK(d.decoder, row.Object.Raw, nil /* defaults GVK */)
	return trace.Wrap(err)
}

// encode encodes the filtered object into the io.Writer using the same
// content-type.
func (d *resourceFilterer) encode(obj runtime.Object, w io.Writer) error {
	return trace.Wrap(d.encoder.Encode(obj, w))
}

// filterResourceList excludes resources the user should not have access to.
func filterResourceList[T kubeObjectInterface](kind, verb string, originalList []T, allowed, denied []types.KubernetesResource, log *slog.Logger) []T {
	filteredList := make([]T, 0, len(originalList))
	for _, resource := range originalList {
		if result, err := filterResource(kind, verb, resource, allowed, denied); err == nil && result {
			filteredList = append(filteredList, resource)
		} else if err != nil {
			slog.WarnContext(context.Background(), "Unable to compile regex expressions within kubernetes_resources", "error", err)
		}
	}
	return filteredList
}

// kubeObjectInterface is an interface that all Kubernetes objects must
// implement to be able to filter them. It is used to extract the kind of the
// object from the GroupVersionKind object, the namespace and the name.
type kubeObjectInterface interface {
	GetNamespace() string
	GetName() string
}

// filterResource validates if the user should access the current resource.
func filterResource(kind, verb string, resource kubeObjectInterface, allowed, denied []types.KubernetesResource) (bool, error) {
	result, err := matchKubernetesResource(
		getKubeResource(kind, verb, resource),
		allowed, denied,
	)
	return result, trace.Wrap(err)
}

func getKubeResource(kind, verb string, obj kubeObjectInterface) types.KubernetesResource {
	return types.KubernetesResource{
		Kind:      kind,
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
		Verbs:     []string{verb},
	}
}

// filterMetaV1Table filters the serverside printed table to exclude resources
// that the user must not have access to.
func (d *resourceFilterer) filterMetaV1Table(table *metav1.Table, allowedResources, deniedResources []types.KubernetesResource) (*metav1.Table, error) {
	resources := make([]metav1.TableRow, 0, len(table.Rows))
	for i := range table.Rows {
		row := &(table.Rows[i])
		if err := d.decodePartialObjectMetadata(row); err != nil {
			return nil, trace.Wrap(err)
		}
		resource, err := getKubeResourcePartialMetadataObject(d.kind, d.verb, row.Object.Object)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if result, err := matchKubernetesResource(resource, allowedResources, deniedResources); err == nil && result {
			resources = append(resources, *row)
		} else if err != nil {
			d.log.WarnContext(context.Background(), "Unable to compile regex expression", "error", err)
		}
	}
	table.Rows = resources
	return table, nil
}

// getKubeResourcePartialMetadataObject checks if obj satisfies namespaceNamer or namer interfaces
// otherwise returns an error.
func getKubeResourcePartialMetadataObject(kind, verb string, obj runtime.Object) (types.KubernetesResource, error) {
	type namer interface {
		GetName() string
	}
	type namespaceNamer interface {
		GetNamespace() string
		namer
	}
	switch o := obj.(type) {
	case namespaceNamer:
		return types.KubernetesResource{
			Namespace: o.GetNamespace(),
			Name:      o.GetName(),
			Kind:      kind,
			Verbs:     []string{verb},
		}, nil
	case namer:
		return types.KubernetesResource{
			Name:  o.GetName(),
			Kind:  kind,
			Verbs: []string{verb},
		}, nil
	default:
		return types.KubernetesResource{}, trace.BadParameter("unexpected %T type", obj)
	}
}

// newEncoderAndDecoderForContentType creates a new encoder and decoder instances
// for the given contentType.
// If the contentType is invalid or not supported this function returns an error.
// Supported content types:
// - "application/json"
// - "application/yaml"
// - "application/vnd.kubernetes.protobuf"
func newEncoderAndDecoderForContentType(contentType string, negotiator runtime.ClientNegotiator) (runtime.Encoder, runtime.Decoder, error) {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, nil, trace.WrapWithMessage(err, "unable to parse %q header %q", responsewriters.ContentTypeHeader, contentType)
	}
	dec, err := negotiator.Decoder(mediaType, params)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	enc, err := negotiator.Encoder(mediaType, params)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return enc, dec, nil
}

// decodeAndSetGVK decodes the payload into the appropriate type using the decoder
// provider and sets the GVK if available.
// defaults is the fallback GVK used by the decoder if the payload doesn't set their
// own GVK.
func decodeAndSetGVK(decoder runtime.Decoder, payload []byte, defaults *schema.GroupVersionKind) (runtime.Object, error) {
	obj, gvk, err := decoder.Decode(payload, defaults, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if gvk != nil {
		// objects from decode do not contain GroupVersionKind.
		// We force it to be present for later encoding.
		obj.GetObjectKind().SetGroupVersionKind(*gvk)
	}
	return obj, nil
}

// filterBuffer filters the response buffer before writing it into the original
// MemoryResponseWriter.
func filterBuffer(filterWrapper responsewriters.FilterWrapper, src *responsewriters.MemoryResponseWriter) error {
	if filterWrapper == nil {
		return nil
	}

	filter, err := filterWrapper(responsewriters.GetContentTypeHeader(src.Header()), src.Status())
	if err != nil {
		return trace.Wrap(err)
	}
	// copy body into another slice so we can manipulate it.
	b := bytes.NewBuffer(make([]byte, 0, src.Buffer().Len()))

	// get the compressor and decompressor for the response based on the content type.
	compressor, decompressor, err := getResponseCompressorDecompressor(src.Header())
	if err != nil {
		return trace.Wrap(err)
	}
	// decompress the response body into b.
	if err := decompressor(b, src.Buffer()); err != nil {
		return trace.Wrap(err)
	}
	// filter.FilterBuffer encodes the filtered payload into src.Buffer, so we need to
	// reset it to discard the old payload.
	src.Buffer().Reset()
	// creates a compressor that writes the filtered payload into src.Buffer.
	comp := compressor(src.Buffer())
	// Close is a no-op operation into src but it's required to put the gzip writer
	// into the sync.Pool.
	defer comp.Close()
	return trace.Wrap(filter.FilterBuffer(b.Bytes(), comp))
}

// filterUnstructuredList filters the unstructured list object to exclude resources
// that the user must not have access to.
// The filtered list is re-assigned to `obj.Object["items"]`.
func filterUnstructuredList(verb string, obj *unstructured.Unstructured, allowed, denied []types.KubernetesResource, log *slog.Logger) (hasElems bool) {
	const (
		itemsKey = "items"
	)
	if obj == nil || obj.Object == nil {
		return false
	}
	objList, err := obj.ToList()
	if err != nil {
		// This should never happen, but if it does, we should log it.
		slog.WarnContext(context.Background(), "Unable to convert unstructured object to list", "error", err)
		return false
	}

	filteredList := make([]any, 0, len(objList.Items))
	for _, resource := range objList.Items {
		r := getKubeResource(utils.KubeCustomResource, verb, &resource)
		if result, err := matchKubernetesResource(
			r,
			allowed, denied,
		); result {
			filteredList = append(filteredList, resource.Object)
		} else if err != nil {
			slog.WarnContext(context.Background(), "Unable to compile regex expressions within kubernetes_resources", "error", err)
		}
	}
	obj.Object[itemsKey] = filteredList
	return len(filteredList) > 0
}
