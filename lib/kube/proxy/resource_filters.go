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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/kube/proxy/responsewriters"
)

// newResourceFilterer creates a wrapper function that once executed creates
// a runtime filter for kubernetes resources.
// The filter exclusion criteria is:
// - deniedResources: excluded if (namespace,name) matches an entry even if it matches
// the allowedResources's list.
// - allowedResources: excluded if (namespace,name) not match a single entry.
func newResourceFilterer(kind, group, verb string, isClusterWideResource bool, codecs *serializer.CodecFactory, allowedResources, deniedResources []types.KubernetesResource, log *slog.Logger) responsewriters.FilterWrapper {
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
			encoder:               encoder,
			decoder:               decoder,
			contentType:           contentType,
			responseCode:          responseCode,
			negotiator:            negotiator,
			allowedResources:      allowedResources,
			deniedResources:       deniedResources,
			log:                   log,
			kind:                  kind,
			group:                 group,
			verb:                  verb,
			isClusterWideResource: isClusterWideResource,
		}, nil
	}
}

// wildcardFilter is a filter that matches all pods.
var wildcardFilter = types.KubernetesResource{
	Kind:      types.Wildcard,
	APIGroup:  types.Wildcard,
	Namespace: types.Wildcard,
	Name:      types.Wildcard,
	Verbs:     []string{types.Wildcard},
}

// containsWildcard returns true if the list of resources contains a wildcard filter.
func containsWildcard(resources []types.KubernetesResource) bool {
	for _, r := range resources {
		if r.Kind == wildcardFilter.Kind &&
			r.APIGroup == wildcardFilter.APIGroup &&
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
	// group is the api group of the resource.
	group string
	// verb is the kube API verb based on HTTP verb.
	verb string
	// isClusterWideResource is true if the resource is cluster wide.
	isClusterWideResource bool
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

	case *unstructured.Unstructured:
		if o.IsList() {
			hasElemts := d.filterUnstructuredList(o)
			return hasElemts, true, nil
		}

		r := getKubeResource(d.kind, d.group, d.verb, o)
		result, err := matchKubernetesResource(
			r, d.isClusterWideResource,
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
		if isListObj(o) {
			output, err := getItemsUsingReflection(obj)
			if err != nil {
				return false, false, trace.Wrap(err, "failed to get items from list object")
			}
			if len(output.items) > 0 {
				output.items = filterResourceList(d, output.items)
				setItemsUsingReflection(output.underlyingValue, output.underlyingType, output.items)
			}
			return len(output.items) > 0, true, nil
		} else if kubeObj, ok := o.(kubeObjectInterface); ok {
			result, err := d.filterResource(kubeObj)
			if err != nil {
				d.log.WarnContext(ctx, "Unable to compile regex expressions within kubernetes_resources", "error", err)
			}
			// if err is not nil or result is false, we should not include it.
			return result, false, nil
		}

		// It's important default types are never blindly forwarded or protocol
		// extensions could result in information disclosures.
		return false, false, trace.BadParameter("unexpected type received; got %T", obj)
	}
}

func isListObj(obj runtime.Object) bool {
	_, ok := obj.(metav1.ListInterface)
	return ok
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
func filterResourceList[T kubeObjectInterface](d *resourceFilterer, originalList []T) []T {
	filteredList := make([]T, 0, len(originalList))
	for _, resource := range originalList {
		if result, err := d.filterResource(resource); err == nil && result {
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
	GroupVersionKind() schema.GroupVersionKind
	GetNamespace() string
	GetName() string
}

// filterResource validates if the user should access the current resource.
func (d *resourceFilterer) filterResource(resource kubeObjectInterface) (bool, error) {
	result, err := matchKubernetesResource(
		getKubeResource(d.kind, d.group, d.verb, resource),
		d.isClusterWideResource,
		d.allowedResources, d.deniedResources,
	)
	return result, trace.Wrap(err)
}

func getKubeResource(kind, group, verb string, obj kubeObjectInterface) types.KubernetesResource {
	return types.KubernetesResource{
		Kind:      kind,
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
		Verbs:     []string{verb},
		APIGroup:  group,
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
		resource, err := getKubeResourcePartialMetadataObject(d.kind, d.group, d.verb, row.Object.Object)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if result, err := matchKubernetesResource(resource, d.isClusterWideResource, allowedResources, deniedResources); err == nil && result {
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
func getKubeResourcePartialMetadataObject(kind, group, verb string, obj runtime.Object) (types.KubernetesResource, error) {
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
			APIGroup:  group,
		}, nil
	case namer:
		return types.KubernetesResource{
			Name:     o.GetName(),
			Kind:     kind,
			Verbs:    []string{verb},
			APIGroup: group,
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
func (d *resourceFilterer) filterUnstructuredList(obj *unstructured.Unstructured) (hasElems bool) {
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
		gvk := resource.GroupVersionKind()
		r := getKubeResource(d.kind, gvk.Group, d.verb, &resource)
		if result, err := matchKubernetesResource(
			r, d.isClusterWideResource,
			d.allowedResources, d.deniedResources,
		); result {
			filteredList = append(filteredList, resource.Object)
		} else if err != nil {
			slog.WarnContext(context.Background(), "Unable to compile regex expressions within kubernetes_resources", "error", err)
		}
	}
	obj.Object[itemsKey] = filteredList
	return len(filteredList) > 0
}
