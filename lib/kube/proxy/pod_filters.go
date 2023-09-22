// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxy

import (
	"bytes"
	"io"
	"mime"
	"net/http"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/kube/proxy/responsewriters"
)

// newPodFilterer creates a wrapper function that once executed creates
// a runtime filter for Pods.
// The filter exclusion criteria is:
// - deniedPods: excluded if (namespace,name) matches an entry even if it matches
// the allowedPod's list.
// - allowedPods: excluded if (namespace,name) not match a single entry.
func newPodFilterer(allowedPods, deniedPods []types.KubernetesResource, log logrus.FieldLogger) responsewriters.FilterWrapper {
	// If the list of allowed pods contains a wildcard and no deniedPods, then we
	// don't need to filter anything.
	if containsWildcard(allowedPods) && len(deniedPods) == 0 {
		return nil
	}
	return func(contentType string, responseCode int) (responsewriters.Filter, error) {
		negotiator := newClientNegotiator()
		encoder, decoder, err := newEncoderAndDecoderForContentType(contentType, negotiator)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &podFilterer{
			encoder:          encoder,
			decoder:          decoder,
			contentType:      contentType,
			responseCode:     responseCode,
			negotiator:       negotiator,
			allowedResources: allowedPods,
			deniedResources:  deniedPods,
			log:              log,
		}, nil
	}
}

// wildcardFilter is a filter that matches all pods.
var wildcardFilter = types.KubernetesResource{
	Kind:      types.KindKubePod,
	Namespace: types.Wildcard,
	Name:      types.Wildcard,
}

// containsWildcard returns true if the list of resources contains a wildcard filter.
func containsWildcard(resources []types.KubernetesResource) bool {
	for _, r := range resources {
		if r.Kind == wildcardFilter.Kind &&
			r.Name == wildcardFilter.Name &&
			r.Namespace == wildcardFilter.Namespace {
			return true
		}
	}
	return false
}

// podFilterer is a pod filterer instance.
type podFilterer struct {
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
	log logrus.FieldLogger
}

// FilterBuffer receives a byte array, decodes the response into the appropriate
// type and filters the resources based on allowed and denied rules configured.
// After filtering them, it serializes the response and dumps it into output buffer.
// If any error occurs, the call returns an error.
func (d *podFilterer) FilterBuffer(buf []byte, output io.Writer) error {
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

	var filtered runtime.Object
	switch o := obj.(type) {
	case *metav1.Status:
		// Status object is returned when the Kubernetes API returns an error and
		// should be forwarded to the user.
		filtered = obj
	case *corev1.Pod:
		// filterPod filters a single corev1.Pod and returns an error if access to
		// pod was denied.
		if err := filterPod(o, d.allowedResources, d.deniedResources); err != nil {
			if !trace.IsAccessDenied(err) {
				d.log.WithError(err).Warn("Unable to compile role kubernetes_resources.")
			}
			return trace.Wrap(err)
		}
		filtered = obj
	case *corev1.PodList:
		filtered = filterCoreV1PodList(o, d.allowedResources, d.deniedResources, d.log)
	case *metav1.Table:
		filtered, err = d.filterMetaV1Table(o, d.allowedResources, d.deniedResources)
		if err != nil {
			return trace.Wrap(err)
		}
	default:
		// It's important default types are never blindly forwarded or protocol
		// extensions could result in information disclosures.
		return trace.BadParameter("unexpected type received; got %T", obj)
	}
	// encode the filterer response back to the user.
	return d.encode(filtered, output)
}

// FilterObj receives a runtime.Object type and filters the resources on it
// based on allowed and denied rules.
// After filtering them, the obj is manipulated to hold the filtered information.
// The boolean returned indicates if the client is allowed to receive the event
// that originated this check.
func (d *podFilterer) FilterObj(obj runtime.Object) (bool, error) {
	switch o := obj.(type) {
	case *corev1.Pod:
		err := filterPod(o, d.allowedResources, d.deniedResources)
		if err != nil && !trace.IsAccessDenied(err) {
			d.log.WithError(err).Warn("Unable to compile role kubernetes_resources.")
		}
		// if err is not nil we should not include it.
		return err == nil, nil
	case *corev1.PodList:
		_ = filterCoreV1PodList(o, d.allowedResources, d.deniedResources, d.log)
		return len(o.Items) > 0, nil
	case *metav1.Table:
		_, err := d.filterMetaV1Table(o, d.allowedResources, d.deniedResources)
		if err != nil {
			return false, trace.Wrap(err)
		}
		return len(o.Rows) > 0, nil
	default:
		// It's important default types are never blindly forwarded or protocol
		// extensions could result in information disclosures.
		return false, trace.BadParameter("unexpected type received; got %T", obj)
	}
}

// decode decodes the buffer into the appropriate type if the responseCode
// belongs to the range 200(OK)-206(PartialContent).
// If it does not belong, it returns the buffer unchanged since it contains
// an error message from the Kubernetes API server and it's safe to return
// it back to the user.
func (d *podFilterer) decode(buffer []byte) (runtime.Object, []byte, error) {
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
		out, err := decodeAndSetGVK(d.decoder, buffer)
		return out, nil, trace.Wrap(err)
	}
}

// decodePartialObjectMetadata decodes the metav1.PartialObjectMetadata present
// in the metav1.TableRow entry. This information comes from server side and
// includes the resource name and namespace as a structured object.
func (d *podFilterer) decodePartialObjectMetadata(row *metav1.TableRow) error {
	if row.Object.Object != nil {
		return nil
	}
	var err error
	// decode only if row.Object.Object was not decoded before.
	row.Object.Object, err = decodeAndSetGVK(d.decoder, row.Object.Raw)
	return trace.Wrap(err)
}

// encode encodes the filtered object into the io.Writer using the same
// content-type.
func (d *podFilterer) encode(obj runtime.Object, w io.Writer) error {
	return trace.Wrap(d.encoder.Encode(obj, w))
}

// filterCoreV1PodList excludes pods the user should not have access to.
func filterCoreV1PodList(list *corev1.PodList, allowed, denied []types.KubernetesResource, log logrus.FieldLogger) *corev1.PodList {
	pods := make([]corev1.Pod, 0, len(list.Items))
	for _, pod := range list.Items {
		if err := filterPod(&pod, allowed, denied); err == nil {
			pods = append(pods, pod)
		} else if !trace.IsAccessDenied(err) {
			log.WithError(err).Warnf("Unable to compile role kubernetes_resources.")
		}
	}
	list.Items = pods
	return list
}

// filterPod validates if the user should access the current resource.
func filterPod(pod *corev1.Pod, allowed, denied []types.KubernetesResource) error {
	err := matchKubernetesResource(
		types.KubernetesResource{
			Kind:      types.KindKubePod,
			Namespace: pod.Namespace,
			Name:      pod.Name,
		},
		allowed, denied,
	)
	return trace.Wrap(err)
}

// filterMetaV1Table filters the serverside printed table to exclude pods
// that the user must not have access to.
func (d *podFilterer) filterMetaV1Table(table *metav1.Table, allowedPods, deniedPods []types.KubernetesResource) (*metav1.Table, error) {
	pods := make([]metav1.TableRow, 0, len(table.Rows))
	for i := range table.Rows {
		row := &(table.Rows[i])
		if err := d.decodePartialObjectMetadata(row); err != nil {
			return nil, trace.Wrap(err)
		}
		resource, err := getKubeResourcePartialMetadataObject(row.Object.Object)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := matchKubernetesResource(resource, allowedPods, deniedPods); err == nil {
			pods = append(pods, *row)
		} else if !trace.IsAccessDenied(err) {
			d.log.WithError(err).Warn("Unable to compile regex expression.")
		}
	}
	table.Rows = pods
	return table, nil
}

// getKubeResourcePartialMetadataObject checks if obj is of type *metav1.PartialObjectMetadata
// otherwise returns an error.
func getKubeResourcePartialMetadataObject(obj runtime.Object) (types.KubernetesResource, error) {
	switch o := obj.(type) {
	case *metav1.PartialObjectMetadata:
		return types.KubernetesResource{
			Namespace: o.Namespace,
			Name:      o.Name,
			Kind:      types.KindKubePod,
		}, nil
	default:
		return types.KubernetesResource{}, trace.BadParameter("expected *metav1.PartialObjectMetadata object, got %T", obj)
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
func decodeAndSetGVK(decoder runtime.Decoder, payload []byte) (runtime.Object, error) {
	obj, gvk, err := decoder.Decode(payload, nil, nil)
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
