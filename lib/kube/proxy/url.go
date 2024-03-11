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
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/gravitational/trace"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/kube/proxy/responsewriters"
)

type apiResource struct {
	apiGroup        string
	apiGroupVersion string
	namespace       string
	resourceKind    string
	resourceName    string
	skipEvent       bool
	isWatch         bool
}

// parseResourcePath does best-effort parsing of a Kubernetes API request path.
// All fields of the returned apiResource may be empty.
func parseResourcePath(p string) apiResource {
	// Kubernetes API reference: https://kubernetes.io/docs/reference/kubernetes-api/
	// Let's try to parse this. Here be dragons!
	//
	// URLs have a prefix that defines an "API group":
	// - /api/v1/ - the special "core" API group (e.g. pods, secrets, etc. belong here)
	// - /apis/{group}/{version} - the other properly named groups (e.g. apps/v1 or rbac.authorization.k8s.io/v1beta1)
	//
	// After the prefix, we have the resource info:
	// - /namespaces/{namespace}/{resource kind}/{resource name} for namespaced resources
	//   - turns out, namespace is optional when you query across all
	//     namespaces (e.g. /api/v1/pods to get pods in all namespaces)
	// - /{resource kind}/{resource name} for cluster-scoped resources (e.g. namespaces or nodes)
	//
	// If {resource name} is missing, the request refers to all resources of
	// that kind (e.g. list all pods).
	//
	// There can be more items after {resource name} (a "subresource"), like
	// pods/foo/exec, but the depth is arbitrary (e.g.
	// /api/v1/namespaces/{namespace}/pods/{name}/proxy/{path})
	//
	// And the cherry on top - watch endpoints, e.g.
	// /api/v1/watch/namespaces/{namespace}/pods/{name}
	// for live updates on resources (specific resources or all of one kind)
	var r apiResource

	// Clean up the path and make it absolute.
	p = path.Clean(p)
	if !path.IsAbs(p) {
		p = "/" + p
	}

	parts := strings.Split(p, "/")
	switch {
	// Core API group has a "special" URL prefix /api/v1/.
	case len(parts) >= 3 && parts[1] == "api" && parts[2] == "v1":
		r.apiGroup = "core"
		r.apiGroupVersion = parts[2]
		parts = parts[3:]
	// Other API groups have URL prefix /apis/{group}/{version}.
	case len(parts) >= 4 && parts[1] == "apis":
		r.apiGroup, r.apiGroupVersion = parts[2], parts[3]
		parts = parts[4:]
	case len(parts) >= 2 && (parts[1] == "api" || parts[1] == "apis"):
		// /api or /apis.
		// This is part of API discovery. Don't emit to audit log to reduce
		// noise.
		r.skipEvent = true
		return r
	default:
		// Doesn't look like a k8s API path, return empty result.
		return r
	}

	// Watch API endpoints have an extra /watch/ prefix. For now, silently
	// strip it from our result.
	if len(parts) > 0 && parts[0] == "watch" {
		r.isWatch = true
		parts = parts[1:]
	}

	switch len(parts) {
	case 0:
		// e.g. /apis/apps/v1
		// This is part of API discovery. Don't emit to audit log to reduce
		// noise.
		r.skipEvent = true
		return r
	case 1:
		// e.g. /api/v1/pods - list pods in all namespaces
		r.resourceKind = parts[0]
	case 2:
		// e.g. /api/v1/clusterroles/{name} - read a cluster-level resource
		r.resourceKind = parts[0]
		r.resourceName = parts[1]
	case 3:
		if parts[0] == "namespaces" {
			// e.g. /api/v1/namespaces/{namespace}/pods - list pods in a
			// specific namespace
			r.namespace = parts[1]
			r.resourceKind = parts[2]
		} else {
			// e.g. /apis/apiregistration.k8s.io/v1/apiservices/{name}/status
			kind := append([]string{parts[0]}, parts[2:]...)
			r.resourceKind = strings.Join(kind, "/")
			r.resourceName = parts[1]
		}
	default:
		// e.g. /api/v1/namespaces/{namespace}/pods/{name} - get a specific pod
		// or /api/v1/namespaces/{namespace}/pods/{name}/exec - exec command in a pod
		if parts[0] == "namespaces" {
			r.namespace = parts[1]
			kind := append([]string{parts[2]}, parts[4:]...)
			r.resourceKind = strings.Join(kind, "/")
			r.resourceName = parts[3]
		} else {
			// e.g. /api/v1/nodes/{name}/proxy/{path}
			kind := append([]string{parts[0]}, parts[2:]...)
			r.resourceKind = strings.Join(kind, "/")
			r.resourceName = parts[1]
		}
	}
	return r
}

func (r apiResource) populateEvent(e *apievents.KubeRequest) {
	e.ResourceAPIGroup = path.Join(r.apiGroup, r.apiGroupVersion)
	e.ResourceNamespace = r.namespace
	e.ResourceKind = r.resourceKind
	e.ResourceName = r.resourceName
}

// allowedResourcesKey is a key used to identify a resource in the allowedResources map.
type allowedResourcesKey struct {
	apiGroup     string
	resourceKind string
}

type rbacSupportedResources map[allowedResourcesKey]string

// getResourceWithKey returns the teleport resource kind for a given resource key if
// it exists, otherwise returns an empty string.
func (r rbacSupportedResources) getResourceWithKey(k allowedResourcesKey) string {
	if k.apiGroup == "" {
		k.apiGroup = "core"
	}
	return r[k]
}

func (r rbacSupportedResources) getTeleportResourceKindFromAPIResource(api apiResource) (string, bool) {
	resource := getResourceFromAPIResource(api.resourceKind)
	resourceType, ok := r[allowedResourcesKey{apiGroup: api.apiGroup, resourceKind: resource}]
	return resourceType, ok
}

// defaultRBACResources is a map of supported resources and their corresponding
// teleport resource kind for the purpose of resource rbac.
var defaultRBACResources = rbacSupportedResources{
	{apiGroup: "core", resourceKind: "pods"}:                                      types.KindKubePod,
	{apiGroup: "core", resourceKind: "secrets"}:                                   types.KindKubeSecret,
	{apiGroup: "core", resourceKind: "configmaps"}:                                types.KindKubeConfigmap,
	{apiGroup: "core", resourceKind: "namespaces"}:                                types.KindKubeNamespace,
	{apiGroup: "core", resourceKind: "services"}:                                  types.KindKubeService,
	{apiGroup: "core", resourceKind: "endpoints"}:                                 types.KindKubeService,
	{apiGroup: "core", resourceKind: "serviceaccounts"}:                           types.KindKubeServiceAccount,
	{apiGroup: "core", resourceKind: "nodes"}:                                     types.KindKubeNode,
	{apiGroup: "core", resourceKind: "persistentvolumes"}:                         types.KindKubePersistentVolume,
	{apiGroup: "core", resourceKind: "persistentvolumeclaims"}:                    types.KindKubePersistentVolumeClaim,
	{apiGroup: "apps", resourceKind: "deployments"}:                               types.KindKubeDeployment,
	{apiGroup: "apps", resourceKind: "replicasets"}:                               types.KindKubeReplicaSet,
	{apiGroup: "apps", resourceKind: "statefulsets"}:                              types.KindKubeStatefulset,
	{apiGroup: "apps", resourceKind: "daemonsets"}:                                types.KindKubeDaemonSet,
	{apiGroup: "rbac.authorization.k8s.io", resourceKind: "clusterroles"}:         types.KindKubeClusterRole,
	{apiGroup: "rbac.authorization.k8s.io", resourceKind: "roles"}:                types.KindKubeRole,
	{apiGroup: "rbac.authorization.k8s.io", resourceKind: "clusterrolebindings"}:  types.KindKubeClusterRoleBinding,
	{apiGroup: "rbac.authorization.k8s.io", resourceKind: "rolebindings"}:         types.KindKubeRoleBinding,
	{apiGroup: "batch", resourceKind: "cronjobs"}:                                 types.KindKubeCronjob,
	{apiGroup: "batch", resourceKind: "jobs"}:                                     types.KindKubeJob,
	{apiGroup: "certificates.k8s.io", resourceKind: "certificatesigningrequests"}: types.KindKubeCertificateSigningRequest,
	{apiGroup: "networking.k8s.io", resourceKind: "ingresses"}:                    types.KindKubeIngress,
	{apiGroup: "extensions", resourceKind: "deployments"}:                         types.KindKubeDeployment,
	{apiGroup: "extensions", resourceKind: "replicasets"}:                         types.KindKubeReplicaSet,
	{apiGroup: "extensions", resourceKind: "daemonsets"}:                          types.KindKubeDaemonSet,
	{apiGroup: "extensions", resourceKind: "ingresses"}:                           types.KindKubeIngress,
}

// getResourceFromRequest returns a KubernetesResource if the user tried to access
// a specific endpoint that Teleport support resource filtering. Otherwise, returns nil.
func getResourceFromRequest(req *http.Request, kubeDetails *kubeDetails) (*types.KubernetesResource, apiResource, error) {
	apiResource := parseResourcePath(req.URL.Path)
	verb := apiResource.getVerb(req)
	if kubeDetails == nil {
		return nil, apiResource, nil
	}

	codecFactory, rbacSupportedTypes, err := kubeDetails.getClusterSupportedResources()
	if err != nil {
		return nil, apiResource, trace.Wrap(err)
	}

	resourceType, ok := rbacSupportedTypes.getTeleportResourceKindFromAPIResource(apiResource)
	switch {
	case !ok:
		// if the resource is not supported, return nil.
		return nil, apiResource, nil
	case apiResource.resourceName == "" && verb != types.KubeVerbCreate:
		// if the resource is supported but the resource name is not present and not a create request,
		// return nil because it's a list request.
		return nil, apiResource, nil

	case apiResource.resourceName == "" && verb == types.KubeVerbCreate:
		// If the request is a create request, extract the resource name from the request body.
		var err error
		if apiResource.resourceName, err = extractResourceNameFromPostRequest(req, codecFactory, kubeDetails.getObjectGVK(apiResource)); err != nil {
			return nil, apiResource, trace.Wrap(err)
		}
	}

	return &types.KubernetesResource{
		Kind:      resourceType,
		Namespace: apiResource.namespace,
		Name:      apiResource.resourceName,
		Verbs:     []string{verb},
	}, apiResource, nil
}

// extractResourceNameFromPostRequest extracts the resource name from a POST body.
// It reads the full body - required because data can be proto encoded -
// and decodes it into a Kubernetes object. It then extracts the resource name
// from the object.
// The body is then reset to the original request body using a new buffer.
func extractResourceNameFromPostRequest(
	req *http.Request,
	codecs *serializer.CodecFactory,
	defaults *schema.GroupVersionKind,
) (string, error) {
	if req.Body == nil {
		return "", trace.BadParameter("request body is empty")
	}

	negotiator := newClientNegotiator(codecs)
	_, decoder, err := newEncoderAndDecoderForContentType(
		responsewriters.GetContentTypeHeader(req.Header),
		negotiator,
	)
	if err != nil {
		return "", trace.Wrap(err)
	}

	newBody := bytes.NewBuffer(make([]byte, 0, 2048))
	if _, err := io.Copy(newBody, req.Body); err != nil {
		return "", trace.Wrap(err)
	}
	if err := req.Body.Close(); err != nil {
		return "", trace.Wrap(err)
	}
	req.Body = io.NopCloser(newBody)
	// decode memory rw body.
	obj, err := decodeAndSetGVK(decoder, newBody.Bytes(), defaults)
	if err != nil {
		return "", trace.Wrap(err)
	}
	namer, ok := obj.(kubeObjectInterface)
	if !ok {
		return "", trace.BadParameter("object %T does not implement kubeObjectInterface", obj)
	}
	return namer.GetName(), nil
}

// getResourceFromAPIResource returns the resource kind from the api resource.
// If the resource kind contains sub resources (e.g. pods/exec), it returns the
// resource kind without the subresource.
func getResourceFromAPIResource(resourceKind string) string {
	if idx := strings.Index(resourceKind, "/"); idx != -1 {
		return resourceKind[:idx]
	}
	return resourceKind
}

// isKubeWatchRequest returns true if the request is a watch request.
func isKubeWatchRequest(req *http.Request, r apiResource) bool {
	if values := req.URL.Query()["watch"]; len(values) > 0 {
		switch strings.ToLower(values[0]) {
		case "false", "0":
		default:
			return true
		}
	}
	return r.isWatch
}

func (r apiResource) getVerb(req *http.Request) string {
	verb := ""
	isWatch := isKubeWatchRequest(req, r)
	switch r.resourceKind {
	case "pods/exec", "pods/attach":
		verb = types.KubeVerbExec
	case "pods/portforward":
		verb = types.KubeVerbPortForward
	default:
		switch req.Method {
		case http.MethodPost:
			verb = types.KubeVerbCreate
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			switch {
			case isWatch:
				return types.KubeVerbWatch
			case r.resourceName == "":
				return types.KubeVerbList
			default:
				return types.KubeVerbGet
			}
		case http.MethodPut:
			verb = types.KubeVerbUpdate
		case http.MethodPatch:
			verb = types.KubeVerbPatch
		case http.MethodDelete:
			switch {
			case r.resourceName != "":
				verb = types.KubeVerbDelete
			default:
				verb = types.KubeVerbDeleteCollection
			}
		default:
			verb = ""
		}
	}

	return verb
}
