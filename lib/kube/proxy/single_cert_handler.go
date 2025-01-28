/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"encoding/base64"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/tlsca"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

const (
	// paramTeleportCluster is the path parameter key containing a base64
	// encoded Teleport cluster name for path-routed forwarding.
	paramTeleportCluster = "base64Cluster"

	// paramKubernetesCluster is the path parameter key containing a base64
	// encoded Teleport cluster name for path-routed forwarding.
	paramKubernetesCluster = "base64KubeCluster"
)

// parseRouteFromPath extracts route information from the given path parameters
// using constant-defined parameter keys.
func parseRouteFromPath(p httprouter.Params) (string, string, error) {
	encodedTeleportCluster := p.ByName(paramTeleportCluster)
	if encodedTeleportCluster == "" {
		return "", "", trace.BadParameter("no Teleport cluster name found in path")
	}

	decodedTeleportCluster, err := base64.RawURLEncoding.DecodeString(encodedTeleportCluster)
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	encodedKubernetesCluster := p.ByName(paramKubernetesCluster)
	if encodedKubernetesCluster == "" {
		return "", "", trace.BadParameter("no Kubernetes cluster name found in path")
	}

	decodedKubernetesCluster, err := base64.RawURLEncoding.DecodeString(encodedKubernetesCluster)
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	if !utf8.Valid(decodedTeleportCluster) {
		return "", "", trace.BadParameter("invalid Teleport cluster name")
	}

	if !utf8.Valid(decodedKubernetesCluster) {
		return "", "", trace.BadParameter("invalid Kubernetes cluster name")
	}

	return string(decodedTeleportCluster), string(decodedKubernetesCluster), nil
}

// ensureRouteNotOverwritten checks that the path routing parameters do not
// overwrite any existing RouteToCluster or KubernetesCluster fields in the
// identity, if those fields are set. Additionally, it is valid for the path
// route to be equal to the existing value.
//
// This requirement ensures temporary certs issued for session MFA remain bound
// to the cluster for which they were initially issued and path routing cannot
// be used to access a different target cluster. If MFA is required, path routed
// requests will receive an `ErrSessionMFARequired` as usual and will need to
// request certificates with identity-based routing information. Once the
// temporary identity is issued, the request can proceed as usual through this
// path-based route so long as the path and identity route fields are equal.
func ensureRouteNotOverwritten(ident *tlsca.Identity, routeToCluster, kubernetesCluster string) error {
	teleportClusterChanged := ident.RouteToCluster != routeToCluster
	kubeClusterChanged := ident.KubernetesCluster != kubernetesCluster

	// If session MFA is enabled, either cluster-wide or for the target cluster
	// via role options, access attempts without an MFA assertion will pass
	// through here and fail during `CheckAccess()` in `authorize()`. If retried
	// with an assertion, we should not allow routing parameters to be
	// overwritten even if somehow empty ("") as that would allow MFA certs to
	// access any cluster.
	if ident.MFAVerified != "" && (teleportClusterChanged || kubeClusterChanged) {
		return trace.AccessDenied("identity routing parameters are required when MFA assertions are present")
	}

	const overwriteDeniedMsg = "existing route in identity may not be overwritten"
	if ident.RouteToCluster != "" && teleportClusterChanged {
		return trace.AccessDenied(overwriteDeniedMsg)
	}
	if ident.KubernetesCluster != "" && kubeClusterChanged {
		return trace.AccessDenied(overwriteDeniedMsg)
	}

	return nil
}

// singleCertHandler extracts routing information from base64-encoded URL
// parameters into the current auth user context and forwards the request back
// to the main router with the path prefix (and its embedded routing parameters)
// stripped.
func (f *Forwarder) singleCertHandler() httprouter.Handle {
	return httplib.MakeHandlerWithErrorWriter(func(w http.ResponseWriter, req *http.Request, p httprouter.Params) (any, error) {
		teleportCluster, kubeCluster, err := parseRouteFromPath(p)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		userTypeI, err := authz.UserFromContext(req.Context())
		if err != nil {
			f.log.WarnContext(req.Context(), "error getting user from context", "error", err)
			return nil, trace.AccessDenied(accessDeniedMsg)
		}

		// Insert the extracted routing information from the path into the
		// identity. Some implementation notes:
		// - This still relies on RouteToCluster and KubernetesCluster identity
		//   fields, even though these fields are not part of the TLS identity
		//   when using path-based routing.
		// - If the Teleport+Kube cluster names resolve to the local node, these
		//   values will be used directly in their proper handlers once the
		//   request is rewritten.
		// - If the route resolves to a remote node, the identity is encoded (in
		//   JSON form) into forwarding headers using
		//   `auth.IdentityForwardingHeaders`. The destination node's auth
		//   middleware is configured to extract this identity (due to
		//   EnableCredentialsForwarding) and implicitly trusts this routing
		//   data, assuming the request originated from a proxy.
		// - In either case, the destination node is ultimately responsible for
		//   authorizing the request, and routing information set in the
		//   identity should not be implicitly trusted. (This was ideally never
		//   the case, given access to resources could be revoked via roles
		//   before certs expired.)

		var userType authz.IdentityGetter
		switch o := userTypeI.(type) {
		case authz.LocalUser:
			if err := ensureRouteNotOverwritten(&o.Identity, teleportCluster, kubeCluster); err != nil {
				return nil, trace.Wrap(err)
			}

			o.Identity.RouteToCluster = teleportCluster
			o.Identity.KubernetesCluster = kubeCluster
			userType = o
		case authz.RemoteUser:
			if err := ensureRouteNotOverwritten(&o.Identity, teleportCluster, kubeCluster); err != nil {
				return nil, trace.Wrap(err)
			}

			o.Identity.RouteToCluster = teleportCluster
			o.Identity.KubernetesCluster = kubeCluster
			userType = o
		default:
			f.log.WarnContext(req.Context(), "Denying proxy access to unsupported user type", "user_type", logutils.TypeAttr(userTypeI))
			return nil, trace.AccessDenied(accessDeniedMsg)
		}

		ctx := authz.ContextWithUser(req.Context(), userType)
		req = req.Clone(ctx)

		path := p.ByName("path")
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}

		req.URL.Path = path
		req.URL.RawPath = ""
		req.RequestURI = req.URL.RequestURI()

		f.router.ServeHTTP(w, req)
		return nil, nil
	}, f.formatStatusResponseError)
}
