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
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"slices"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	authv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/kube/proxy/responsewriters"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// selfSubjectAccessReviews intercepts self subject access reviews requests and pre-validates
// them by applying the kubernetes resources RBAC rules to the request.
// If the self subject access review is allowed, the request is forwarded to the
// kubernetes API server for final validation.
func (f *Forwarder) selfSubjectAccessReviews(authCtx *authContext, w http.ResponseWriter, req *http.Request, p httprouter.Params) (resp any, err error) {
	ctx, span := f.cfg.tracer.Start(
		req.Context(),
		"kube.Forwarder/selfSubjectAccessReviews",
		oteltrace.WithSpanKind(oteltrace.SpanKindServer),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String(f.cfg.KubeServiceType),
			semconv.RPCMethodKey.String("selfSubjectAccessReviews"),
			semconv.RPCSystemKey.String("kube"),
		),
	)
	req = req.WithContext(ctx)
	defer span.End()

	sess, err := f.newClusterSession(req.Context(), *authCtx)
	if err != nil {
		// This error goes to kubernetes client and is not visible in the logs
		// of the teleport server if not logged here.
		f.log.ErrorContext(req.Context(), "Failed to create cluster session", "error", err)
		return nil, trace.Wrap(err)
	}
	// sess.Close cancels the connection monitor context to release it sooner.
	// When the server is under heavy load it can take a while to identify that
	// the underlying connection is gone. This change prevents that and releases
	// the resources as soon as we know the session is no longer active.
	defer sess.close()

	sess.upgradeToHTTP2 = true
	sess.forwarder, err = f.makeSessionForwarder(sess)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// only allow self subject access reviews for the service that proxies the
	// request to the kubernetes API server.
	if sess.isLocalKubernetesCluster {
		if err := f.validateSelfSubjectAccessReview(sess, w, req); trace.IsAccessDenied(err) {
			return nil, nil
		} else if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if err := f.setupForwardingHeaders(sess, req, true /* withImpersonationHeaders */); err != nil {
		// This error goes to kubernetes client and is not visible in the logs
		// of the teleport server if not logged here.
		f.log.ErrorContext(req.Context(), "Failed to set up forwarding headers", "error", err)
		return nil, trace.Wrap(err)
	}
	rw := httplib.NewResponseStatusRecorder(w)
	sess.forwarder.ServeHTTP(rw, req)

	f.emitAuditEvent(req, sess, rw.Status())

	return nil, nil
}

// validateSelfSubjectAccessReview validates the self subject access review
// request by applying the kubernetes resources RBAC rules to the request.
func (f *Forwarder) validateSelfSubjectAccessReview(sess *clusterSession, w http.ResponseWriter, req *http.Request) error {
	negotiator := newClientNegotiator(sess.codecFactory)
	encoder, decoder, err := newEncoderAndDecoderForContentType(responsewriters.GetContentTypeHeader(req.Header), negotiator)
	if err != nil {
		return trace.Wrap(err)
	}
	accessReview, err := parseSelfSubjectAccessReviewRequest(decoder, req)
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO(@creack): Remove this as part of the RBAC RFC. It grants excessive permissions.
	if accessReview.Spec.ResourceAttributes == nil {
		return nil
	}

	namespace := accessReview.Spec.ResourceAttributes.Namespace
	resource, ok := sess.rbacSupportedResources.getResource(accessReview.Spec.ResourceAttributes.Group, accessReview.Spec.ResourceAttributes.Resource)
	// If the request is for a resource that Teleport does not support, return
	// nil and let the Kubernetes API server handle the request.
	// TODO(@creack): Remove this as part of the RBAC RFC. It grants excessive permissions.
	if !ok {
		return nil
	}
	name := accessReview.Spec.ResourceAttributes.Name

	authPref, err := f.cfg.CachingAuthClient.GetAuthPreference(req.Context())
	if err != nil {
		return trace.Wrap(err)
	}

	actx := sess.authContext
	state := actx.GetAccessState(authPref)
	switch err := actx.Checker.CheckAccess(
		actx.kubeCluster,
		state,
		services.RoleMatchers{
			// Append a matcher that validates if the Kubernetes resource is allowed
			// by the roles that satisfy the Kubernetes Cluster.
			&kubernetesResourceMatcher{
				resource: types.KubernetesResource{
					Kind:      resource.Name,
					Name:      name,
					Namespace: namespace,
					Verbs:     []string{accessReview.Spec.ResourceAttributes.Verb},
					APIGroup:  accessReview.Spec.ResourceAttributes.Group,
				},
				isClusterWideResource: !resource.Namespaced,
			},
		}...); {
	case errors.Is(err, services.ErrTrustedDeviceRequired):
		return trace.Wrap(err)
	case err != nil && resource.Namespaced:
		namespaceNameToString := func(namespace, name string) string {
			switch {
			case namespace == "" && name == "":
				return ""
			case namespace != "" && name != "":
				return path.Join(namespace, name)
			case namespace != "":
				return path.Join(namespace, "*")
			default:
				return path.Join("*", name)
			}
		}
		accessReview.Status = authv1.SubjectAccessReviewStatus{
			Allowed: false,
			Denied:  true,
			Reason: fmt.Sprintf(
				"access to %s %s denied by Teleport: please ensure that %q field in your Teleport role defines access to the desired resource.\n\n"+
					"Valid example:\n"+
					"kubernetes_resources:\n"+
					"- kind: %s\n"+
					"  name: %s\n"+
					"  namespace: %s\n"+
					"  verbs: [%s]\n"+
					"  api_group: %s\n",
				accessReview.Spec.ResourceAttributes.Resource,
				namespaceNameToString(namespace, name),
				kubernetesResourcesKey,
				resource.Name,
				emptyOrWildcard(name),
				emptyOrWildcard(namespace),
				emptyOrWildcard(""),
				emptyOrWildcard(accessReview.Spec.ResourceAttributes.Group),
			),
		}

		responsewriters.SetContentTypeHeader(w, req.Header)
		if encodeErr := encoder.Encode(accessReview, w); encodeErr != nil {
			return trace.Wrap(encodeErr)
		}
		return trace.Wrap(err)
	case err != nil:
		// If the request is for a cluster-wide resource, we need to grant access
		// to it.
		accessReview.Status = authv1.SubjectAccessReviewStatus{
			Allowed: false,
			Denied:  true,
			Reason: fmt.Sprintf(
				"access to %s %s denied by Teleport: please ensure that %q field in your Teleport role defines access to the desired resource.\n\n"+
					"Valid example:\n"+
					"kubernetes_resources:\n"+
					"- kind: %s\n"+
					"  name: %s\n"+
					"  verbs: [%s]\n"+
					"  api_group: %s",
				accessReview.Spec.ResourceAttributes.Resource,
				name,
				kubernetesResourcesKey,
				resource.Name,
				emptyOrWildcard(name),
				emptyOrWildcard(""),
				emptyOrWildcard(accessReview.Spec.ResourceAttributes.Group),
			),
		}
		responsewriters.SetContentTypeHeader(w, req.Header)
		if encodeErr := encoder.Encode(accessReview, w); encodeErr != nil {
			return trace.Wrap(encodeErr)
		}
		return trace.Wrap(err)
	}
	return nil
}

// emptyOrWildcard returns the string s if it is not empty, otherwise it returns
// '*'.
func emptyOrWildcard(s string) string {
	if s == "" {
		return fmt.Sprintf("'%s'", types.Wildcard)
	}
	return s
}

// parseSelfSubjectAccessReviewRequest parses the request body into a SelfSubjectAccessReview object
// and replaces the body so it can be read again.
func parseSelfSubjectAccessReviewRequest(decoder runtime.Decoder, req *http.Request) (*authv1.SelfSubjectAccessReview, error) {
	payload, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.Body.Close()

	req.Body = io.NopCloser(bytes.NewReader(payload))
	gvk := authv1.SchemeGroupVersion.WithKind("SelfSubjectAccessReview")
	obj, err := decodeAndSetGVK(decoder, payload, &gvk)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch o := obj.(type) {
	case *authv1.SelfSubjectAccessReview:
		return o, nil
	default:
		return nil, trace.BadParameter("unexpected object type: %T", obj)
	}
}

// kubernetesResourceMatcher matches a role against a Kubernetes Resource.
// This matcher is different form services.KubernetesResourceMatcher because
// if skips some validations if the user doesn't ask for a specific resource.
// If name and namespace are empty, it means that the user wants to match all
// resources of the specified kind for which the user might have access to.
// If the user asks for name="", namespace="" and the role has a matcher
// with name="foo", namespace="bar", the matcher will match but the user
// might not be able to see any resource if the resource does not exist
// in the cluster.
// This matcher assumes the role's kubernetes_resources configured eventually
// match with resources that exist in the cluster.
type kubernetesResourceMatcher struct {
	resource              types.KubernetesResource
	isClusterWideResource bool
}

// Match matches a Kubernetes Resource against provided role and condition.
func (m *kubernetesResourceMatcher) Match(role types.Role, condition types.RoleConditionType) (bool, error) {
	resources := role.GetKubeResources(condition)
	if len(resources) == 0 {
		return false, nil
	}
	kind := m.resource.Kind
	name := m.resource.Name
	namespace := m.resource.Namespace

	// If the resource is global, clear the namespace.
	// NOTE: kubectl will yield a warning for this case, but we still need to process the request.
	if m.isClusterWideResource {
		namespace = ""
	}

	// If we are dealing with a namespace resource, consider it cluster wide even though it is not.
	if m.resource.Kind == "namespaces" {
		m.isClusterWideResource = true
	}

	for _, resource := range resources {
		isResourceTheSameKind := kind == resource.Kind || resource.Kind == types.Wildcard

		namespaceScopeMatch := resource.Kind == "namespaces" && !m.isClusterWideResource
		if !isResourceTheSameKind && !namespaceScopeMatch {
			continue
		}
		if len(m.resource.Verbs) == 1 && !isVerbAllowed(resource.Verbs, m.resource.Verbs[0]) {
			continue
		}

		switch ok, err := utils.SliceMatchesRegex(m.resource.APIGroup, []string{resource.APIGroup}); {
		case err != nil:
			return false, trace.Wrap(err)
		case !ok:
			continue
		}

		// If the resource name and namespace are empty, it means that the
		// user wants to match all resources of the specified kind.
		// We can return true immediately because the user is allowed to get resources
		// of the specified kind but might not be able to see any if the matchers do not
		// match with any resource.
		if (resource.Namespace == "" || resource.Namespace == types.Wildcard) && name == "" && namespace == "" {
			return true, nil
		}
		// If the resource name isn't empty but the resource kind is a namespace scope
		// match - i.e. the resource.Kind==types.KindKubeNamespace and the desired
		// resource kind is not a cluster-wide resource - we should skip the resource
		// name validation.
		if name != "" && !namespaceScopeMatch {
			switch ok, err := utils.SliceMatchesRegex(name, []string{resource.Name}); {
			case err != nil:
				return false, trace.Wrap(err)
			case !ok:
				continue
			}
		}
		if resource.Kind == "namespaces" && namespace != "" {
			if ok, err := utils.SliceMatchesRegex(namespace, []string{resource.Name}); err != nil || ok {
				return ok, trace.Wrap(err)
			}
		} else {
			if ok, err := utils.SliceMatchesRegex(namespace, []string{resource.Namespace}); err != nil || ok {
				return ok, trace.Wrap(err)
			}
		}
	}

	return false, nil
}

// isVerbAllowed returns true if the verb is allowed in the resource.
// If the resource has a wildcard verb, it matches all verbs, otherwise
// the resource must have the verb we're looking for.
func isVerbAllowed(allowedVerbs []string, verb string) bool {
	return len(allowedVerbs) != 0 && (allowedVerbs[0] == types.Wildcard || slices.Contains(allowedVerbs, verb))
}
