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
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"

	"github.com/gravitational/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/httplib/reverseproxy"
	"github.com/gravitational/teleport/lib/kube/proxy/responsewriters"
	"github.com/gravitational/teleport/lib/services"
)

// rewriteResponseForbidden rewrites the response body when the response includes
// a GKE Autopilot forbidden error caused by impersonating system:masters group.
// The response body is rewritten to include a more user friendly error message.
// All other responses are returned as is.
// Example of response body that is rewritten:
//
// Error from server (Forbidden): groups "system:masters" is forbidden:
// User "<user>" cannot impersonate resource "groups" in API group "" at the cluster
// scope: GKE Warden authz [denied by user-impersonation-limitation]: impersonating
// system identities are not allowed
//
// The rewritten response body will look like:
//
// Error from server (Forbidden): "GKE Autopilot denied the request because it impersonates the "system:masters" group.
// Your Teleport Roles [role1,role2] have given access to the "system:masters" group for the cluster "<cluster>".
// For additional information and resolution, please visit
// https://goteleport.com/docs/enroll-resources/kubernetes-access/troubleshooting/#unable-to-connect-to-gke-autopilot-clusters
func (f *Forwarder) rewriteResponseForbidden(s *clusterSession) func(r *http.Response) error {
	return func(r *http.Response) error {
		const (
			// The string that is returned by the GKE Autopilot cluster when
			// users try to impersonate system:masters group.
			autopilotForbidden = "impersonating system identities are not allowed"
		)
		// If the response is not forbidden, we don't need to do anything.
		// The response will be returned as is and written to the client.
		if r.StatusCode != http.StatusForbidden || r.Body == nil {
			return nil
		}
		// create a new buffer to read the response body into.
		b := bytes.NewBuffer(make([]byte, 0, 4096))

		// Read the response body into the buffer.
		if _, err := io.Copy(b, r.Body); err != nil {
			return trace.Wrap(err)
		}
		// Close the response body.
		if err := r.Body.Close(); err != nil {
			return trace.Wrap(err)
		}

		// Replace the response body with the new buffer.
		r.Body = io.NopCloser(b)

		switch {
		case bytes.Contains(b.Bytes(), []byte(autopilotForbidden)):
			// If the response body contains the forbidden string, we rewrite the
			// response body to include a more user friendly error message.
			encoder, _, err := newEncoderAndDecoderForContentType(
				r.Header.Get(responsewriters.ContentTypeHeader),
				newClientNegotiator(&globalKubeCodecs),
			)
			if err != nil {
				f.log.ErrorContext(r.Request.Context(), "Failed to create encoder", "error", err)
				return nil
			}

			status := &metav1.Status{
				Status: metav1.StatusFailure,
				Code:   int32(http.StatusForbidden),
				Reason: metav1.StatusReasonForbidden,
				Message: "GKE Autopilot denied the request because it impersonates the \"system:masters\" group.\n" +
					fmt.Sprintf(
						"Your Teleport Roles %v have given access to the \"system:masters\" group "+
							"for the cluster %q.\n", collectSystemMastersTeleportRoles(s), s.kubeClusterName) +
					"For additional information and resolution, " +
					"please visit https://goteleport.com/docs/enroll-resources/kubernetes-access/troubleshooting/#unable-to-connect-to-gke-autopilot-clusters\n",
			}
			// Reset the buffer to write the new response.
			b.Reset()

			// Encode the new response.
			if err = encoder.Encode(status, b); err != nil {
				f.log.ErrorContext(r.Request.Context(), "Failed to encode response", "error", err)
				return trace.Wrap(err)
			}

			// This function rewrote the response body, so we need update delete the
			// Content-Length header to avoid mismatch between the actual body
			// length and the original Content-Length header value.
			r.Header.Set(reverseproxy.ContentLength, strconv.Itoa(b.Len()))
			return nil
		}

		return nil
	}
}

// collectSystemMastersTeleportRoles returns a list of teleport roles that grant
// system:masters to the target cluster.
func collectSystemMastersTeleportRoles(s *clusterSession) []string {
	const (
		systemMastersGroup = "system:masters"
	)
	accessChecker := s.authContext.Checker
	matchers := make([]services.RoleMatcher, 0, 3)
	// Creates a matcher that matches the cluster labels against `kubernetes_labels`
	// defined for each user's role.
	matchers = append(
		matchers,
		services.NewKubernetesClusterLabelMatcher(s.kubeClusterLabels, accessChecker.Traits()),
	)

	// If the kubeResource is available, append an extra matcher that validates
	// if the kubernetes resource is allowed by the user roles that satisfy the
	// target cluster labels.
	// Each role defines `kubernetes_resources` and when kubeResource is available,
	// KubernetesResourceMatcher will match roles that statisfy the resources at the
	// same time that ClusterLabelMatcher matches the role's "kubernetes_labels".
	// The call to roles.CheckKubeGroupsAndUsers when both matchers are provided
	// results in the intersection of roles that match the "kubernetes_labels" and
	// roles that allow access to the desired "kubernetes_resource".
	// If from the intersection results an empty set, the request is denied.
	if rbacRes := s.metaResource.rbacResource(); rbacRes != nil {
		matchers = append(
			matchers,
			services.NewKubernetesResourceMatcher(*rbacRes, s.metaResource.isClusterWideResource()),
		)
	}
	var rolesWithSystemMasters []string
	matchers = append(matchers,
		// Creates a matcher that checks if the role grants system:masters group.
		// The matcher will be called for each role that matches the cluster labels
		// and the kubernetes resource (if available).
		// It's important to note that this matcher must be the last one in the list
		// otherwise the returned roles may not match the cluster labels and the
		// kubernetes resource.
		services.RoleMatcherFunc(func(r types.Role, cond types.RoleConditionType) (bool, error) {
			groups := r.GetKubeGroups(cond)
			if slices.Contains(groups, systemMastersGroup) {
				rolesWithSystemMasters = append(rolesWithSystemMasters, r.GetName())
			}
			return true, nil
		}),
	)

	_, _, _ = accessChecker.CheckKubeGroupsAndUsers(s.sessionTTL, false /* overrideTTL */, matchers...)
	return rolesWithSystemMasters
}
