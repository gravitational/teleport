// Copyright 2023 Gravitational, Inc
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
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/exp/slices"
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

	// only allow self subject access reviews for the local teleport cluster
	// and not for remote clusters
	if !authCtx.teleportCluster.isRemote {
		if err := f.validateSelfSubjectAccessReview(authCtx, w, req); trace.IsAccessDenied(err) {
			return nil, nil
		} else if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	sess, err := f.newClusterSession(req.Context(), *authCtx)
	if err != nil {
		// This error goes to kubernetes client and is not visible in the logs
		// of the teleport server if not logged here.
		f.log.Errorf("Failed to create cluster session: %v.", err)
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

	if err := f.setupForwardingHeaders(sess, req, true /* withImpersonationHeaders */); err != nil {
		// This error goes to kubernetes client and is not visible in the logs
		// of the teleport server if not logged here.
		f.log.Errorf("Failed to set up forwarding headers: %v.", err)
		return nil, trace.Wrap(err)
	}
	rw := httplib.NewResponseStatusRecorder(w)
	sess.forwarder.ServeHTTP(rw, req)

	f.emitAuditEvent(authCtx, req, sess, rw.Status())

	return nil, nil
}

// validateSelfSubjectAccessReview validates the self subject access review
// request by applying the kubernetes resources RBAC rules to the request.
func (f *Forwarder) validateSelfSubjectAccessReview(actx *authContext, w http.ResponseWriter, req *http.Request) error {
	negotiator := newClientNegotiator()
	encoder, decoder, err := newEncoderAndDecoderForContentType(responsewriters.GetContentTypeHeader(req.Header), negotiator)
	if err != nil {
		return trace.Wrap(err)
	}
	accessReview, err := parseSelfSubjectAccessReviewRequest(decoder, req)
	if err != nil {
		return trace.Wrap(err)
	}

	if accessReview.Spec.ResourceAttributes == nil {
		return nil
	}

	namespace := accessReview.Spec.ResourceAttributes.Namespace
	resource := depluralizeResource(accessReview.Spec.ResourceAttributes.Resource)
	name := accessReview.Spec.ResourceAttributes.Name
	// If the request is for a resource that Teleport does not support, return
	// nil and let the Kubernetes API server handle the request.
	if !slices.Contains(types.KubernetesResourcesKinds, resource) {
		return nil
	}

	authPref, err := f.cfg.CachingAuthClient.GetAuthPreference(req.Context())
	if err != nil {
		return trace.Wrap(err)
	}

	state := actx.GetAccessState(authPref)
	switch err := actx.Checker.CheckAccess(
		actx.kubeCluster,
		state,
		services.RoleMatchers{
			// Append a matcher that validates if the Kubernetes resource is allowed
			// by the roles that satisfy the Kubernetes Cluster.
			&kubernetesResourceMatcher{
				types.KubernetesResource{
					Kind:      resource,
					Name:      name,
					Namespace: namespace,
				},
			},
		}...); {
	case errors.Is(err, services.ErrTrustedDeviceRequired):
		return trace.Wrap(err)
	case err != nil:
		accessReview.Status = authv1.SubjectAccessReviewStatus{
			Allowed: false,
			Denied:  true,
			Reason: fmt.Sprintf(
				"access to %s %s/%s denied by Teleport: please ensure that %q field in your Teleport role defines access to the desired resource.\n\n"+
					"Valid example:\n"+
					"kubernetes_resources:\n"+
					"- kind: %s\n"+
					"  name: %s\n"+
					"  namespace: %s\n", resource, namespace, name, kubernetesResourcesKey, resource, emptyOrWildcard(name), emptyOrWildcard(namespace)),
		}

		responsewriters.SetContentTypeHeader(w, req.Header)
		if err := encoder.Encode(accessReview, w); err != nil {
			return trace.Wrap(err)
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
	obj, err := decodeAndSetGVK(decoder, payload)
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

// depluralizeResource returns the singular form of the resource if it is plural.
func depluralizeResource(resource string) string {
	if strings.HasSuffix(resource, "s") {
		return resource[:len(resource)-1]
	}
	return resource
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
	resource types.KubernetesResource
}

// Match matches a Kubernetes Resource against provided role and condition.
func (m *kubernetesResourceMatcher) Match(role types.Role, condition types.RoleConditionType) (bool, error) {
	resources := role.GetKubeResources(condition)
	if len(resources) == 0 {
		return false, nil
	}
	for _, resource := range resources {
		// TODO(tigrato): evaluate if we should support wildcards as well
		// for future compatibility.
		if m.resource.Kind != resource.Kind {
			continue
		}
		// If the resource name and namespace are empty, it means that the
		// user wants to match all resources of the specified kind.
		// We can return true immediately because the user is allowed to get resources
		// of the specified kind but might not be able to see any if the matchers do not
		// match with any resource.
		if m.resource.Name == "" && m.resource.Namespace == "" {
			return true, nil
		}
		if m.resource.Name != "" {
			switch ok, err := utils.SliceMatchesRegex(m.resource.Name, []string{resource.Name}); {
			case err != nil:
				return false, trace.Wrap(err)
			case !ok:
				continue
			}
		}
		if ok, err := utils.SliceMatchesRegex(m.resource.Namespace, []string{resource.Namespace}); err != nil || ok || m.resource.Namespace == "" {
			return ok || m.resource.Namespace == "", trace.Wrap(err)
		}
	}

	return false, nil
}

// String returns the matcher's string representation.
func (m *kubernetesResourceMatcher) String() string {
	return fmt.Sprintf("kubernetesResourceMatcher(Resource=%v)", m.resource)
}
