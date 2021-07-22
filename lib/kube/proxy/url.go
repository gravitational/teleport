/*
Copyright 2020 Gravitational, Inc.

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

package proxy

import (
	"fmt"
	"path"
	"strings"

	apievents "github.com/gravitational/teleport/api/types/events"
)

type apiResource struct {
	apiGroup     string
	namespace    string
	resourceKind string
	resourceName string
	skipEvent    bool
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
		r.apiGroup = "core/v1"
		parts = parts[3:]
	// Other API groups have URL prefix /apis/{group}/{version}.
	case len(parts) >= 4 && parts[1] == "apis":
		r.apiGroup = fmt.Sprintf("%s/%s", parts[2], parts[3])
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
	e.ResourceAPIGroup = r.apiGroup
	e.ResourceNamespace = r.namespace
	e.ResourceKind = r.resourceKind
	e.ResourceName = r.resourceName
}
