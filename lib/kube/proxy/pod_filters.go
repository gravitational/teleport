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
	"io"
	"mime"
	"net/http"

	"github.com/gravitational/trace"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/kube/proxy/responsewriters"
)

// newPodFiltererBuilder creates a wrapper function that once executed creates
// a runtime filter for Pods.
// The filter exclusion criteria is:
// - deniedPods: excluded if (namespace,name) matches an entry even if it matches
// the allowedPod's list.
// - allowedPods: excluded if (namespace,name) not match a single entry.
func newPodFiltererBuilder(allowedPods, deniedPods []types.KubernetesResource) responsewriters.FilterWrapper {
	negotiator := newClientNegotiator()
	return func(contentType string, responseCode int) (responsewriters.Filter, error) {
		return newPodFilterer(
			podFiltererConfig{
				contentType:      contentType,
				responseCode:     responseCode,
				negotiator:       negotiator,
				allowedResources: allowedPods,
				deniedResources:  deniedPods,
			},
		)
	}
}

// podFiltererConfig holds the configuration for filtering pods from responses.
type podFiltererConfig struct {
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
}

// newPodFilterer creates a new instance of podFilterer with the encoders and
// decoders for the given contentType value.
// podFilterer is able to exclude Pods from list requests executed against the
// target cluster.
// The filter exclusion criteria is:
// - deniedResources: excluded if (namespace,name) matches an entry even if it matches
// the allowedResources's list.
// - allowedResources: excluded if (namespace,name) not match a single entry.
func newPodFilterer(cfg podFiltererConfig) (*podFilterer, error) {
	encoder, decoder, err := newEncoderAndDecoderForContentType(cfg.contentType, cfg.negotiator)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &podFilterer{
		encoder:           encoder,
		decoder:           decoder,
		podFiltererConfig: cfg,
	}, nil
}

// podFilterer is a pod filterer instance.
type podFilterer struct {
	podFiltererConfig
	encoder runtime.Encoder
	decoder runtime.Decoder
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
		filtered = obj
	case *corev1.Pod:
		// filterPod filters a single corev1.Pod and returns an error if access to
		// pod was denied.
		if err := filterPod(o, d.allowedResources, d.deniedResources); err != nil {
			return trace.Wrap(err)
		}
	case *corev1.PodList:
		filtered = filterCoreV1PodList(o, d.allowedResources, d.deniedResources)
	case *metav1.Table:
		filtered, err = d.filterMetav1Table(o, d.allowedResources, d.deniedResources)
		if err != nil {
			return trace.Wrap(err)
		}
	default:
		return trace.BadParameter("unexpected type received; got %T", obj)
	}
	// encode the filterer response back to the user.
	return d.encode(filtered, output)
}

// FilterObj receives a runtime.Object type and filters the resources on it
// based on allowed and denied rules resources.
// After filtering them, the obj is manipulated to hold the filtered information.
func (d *podFilterer) FilterObj(obj runtime.Object) (bool, error) {
	switch o := obj.(type) {
	case *corev1.Pod:
		err := filterPod(o, d.allowedResources, d.deniedResources)
		// if err is not nil we should not include it.
		return err == nil, nil
	case *corev1.PodList:
		_ = filterCoreV1PodList(o, d.allowedResources, d.deniedResources)
		return len(o.Items) > 0, nil
	case *metav1.Table:
		_, err := d.filterMetav1Table(o, d.allowedResources, d.deniedResources)
		if err != nil {
			return false, trace.Wrap(err)
		}
		return len(o.Rows) > 0, nil
	default:
		return false, trace.BadParameter("unexpected type received; got %T", obj)
	}
}

// decode decodes the buffer into the appropriate type if the responseCode is valid.
// If not valid, returns the buffer unchanged.
func (d *podFilterer) decode(buffer []byte) (runtime.Object, []byte, error) {
	switch {
	case d.responseCode == http.StatusSwitchingProtocols:
		// no-op, we've been upgraded
		return nil, buffer, nil
	case d.responseCode < http.StatusOK || d.responseCode > http.StatusPartialContent:
		// calculate an unstructured error from the response which the Result object may use if the caller
		// did not return a structured error.
		return nil, buffer, nil
	default:
		out, gvk, err := d.decoder.Decode(buffer, nil, nil)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		if gvk != nil {
			// objects from decode do not contain GroupVersionKind.
			// We force it to be present for later encoding.
			out.GetObjectKind().SetGroupVersionKind(*gvk)
		}
		return out, nil, nil
	}
}

// decodePartialObjectMetadata decodes the metav1.PartialObjectMetadata present
// in the metav1.TableRow entry. This information comes from server side and
// includes the resource name and namespace as a structured object.
func (d *podFilterer) decodePartialObjectMetadata(row *metav1.TableRow) error {
	if row.Object.Object != nil {
		return nil
	}

	// decode only if row.Object.Object was not decoded before.
	var (
		gvk *schema.GroupVersionKind
		err error
	)
	row.Object.Object, gvk, err = d.decoder.Decode(row.Object.Raw, nil, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	if gvk != nil {
		row.Object.Object.GetObjectKind().SetGroupVersionKind(*gvk)
	}

	return nil
}

// encode encodes the filtered object into the io.Writer using the same
// content-type.
func (d *podFilterer) encode(obj runtime.Object, w io.Writer) error {
	return trace.Wrap(d.encoder.Encode(obj, w))
}

// filterCoreV1PodList exludes pods the user should not have access to.
func filterCoreV1PodList(list *corev1.PodList, allowed, denied []types.KubernetesResource) *corev1.PodList {
	pods := make([]corev1.Pod, 0, len(list.Items))
	for _, pod := range list.Items {
		if err := filterPod(&pod, allowed, denied); err == nil {
			pods = append(pods, pod)
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

func (d *podFilterer) filterMetav1Table(table *metav1.Table, allowedPods, deniedPods []types.KubernetesResource) (*metav1.Table, error) {
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
