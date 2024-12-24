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
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
)

const (
	// paramTeleportCluster is the path parameter key containing a base64
	// encoded Teleport cluster name for path-routed forwarding.
	paramTeleportCluster = "encodedTeleportCluster"

	// paramKubernetesCluster is the path parameter key containing a base64
	// encoded Teleport cluster name for path-routed forwarding.
	paramKubernetesCluster = "encodedKubernetesCluster"
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

	decodedKubernetesCluster, err := base64.RawStdEncoding.DecodeString(encodedKubernetesCluster)
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	// TODO: do we care to otherwise validate these results before casting
	// to a string?
	return string(decodedTeleportCluster), string(decodedKubernetesCluster), nil
}

func (f *Forwarder) singleCertHandler(_ *authContext, w http.ResponseWriter, req *http.Request, p httprouter.Params) (resp any, err error) {
	teleportCluster, kubeCluster, err := parseRouteFromPath(p)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	userTypeI, err := authz.UserFromContext(req.Context())
	if err != nil {
		f.log.WithError(err).Warn("error getting user from context")
		return nil, trace.AccessDenied(accessDeniedMsg)
	}

	var userType authz.IdentityGetter
	switch o := userTypeI.(type) {
	case authz.LocalUser:
		o.Identity.RouteToCluster = teleportCluster
		o.Identity.KubernetesCluster = kubeCluster
		userType = o
	case authz.RemoteUser:
		o.Identity.RouteToCluster = teleportCluster
		o.Identity.KubernetesCluster = kubeCluster
		userType = o
	default:
		f.log.Warningf("Denying proxy access to unsupported user type: %T.", userTypeI)
		return nil, trace.AccessDenied(accessDeniedMsg)
	}

	ctx := authz.ContextWithUser(req.Context(), userType)
	req = req.WithContext(ctx)

	path := p.ByName("path")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	req.URL.Path = path
	req.RequestURI = req.URL.RequestURI()

	f.router.ServeHTTP(w, req)
	return nil, nil
}
