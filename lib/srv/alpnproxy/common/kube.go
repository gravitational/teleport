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

package common

import (
	"encoding/base64"
	"strings"

	"github.com/gravitational/trace"
)

// kubeLocalProxySNIPrefix is the fixed first DNS label of the SNI that kube
// clients present when connecting to the local proxy. It exists only to match
// the wildcard cert (*.<teleport-cluster>); the actual cluster identity for
// request routing is carried in the URL path (see KubeLocalProxyPathPrefix).
const kubeLocalProxySNIPrefix = "kube."

// KubeLocalProxySNI returns the SNI that kube clients should present when
// connecting to the local proxy for the given Teleport cluster. The first
// label is a fixed literal so that it cannot exceed RFC 1035's 63-byte DNS
// label limit; the suffix identifies the Teleport cluster for listener
// routing.
func KubeLocalProxySNI(teleportCluster string) string {
	return kubeLocalProxySNIPrefix + teleportCluster
}

// TeleportClusterFromKubeLocalProxySNI returns the Teleport cluster name
// encoded in the suffix of a local-proxy SNI produced by [KubeLocalProxySNI].
func TeleportClusterFromKubeLocalProxySNI(serverName string) string {
	_, teleportCluster, _ := strings.Cut(serverName, ".")
	return teleportCluster
}

// KubeLocalProxyWildcardDomain returns the wildcard domain used to generate
// the local self-signed CA for the provided Teleport cluster.
func KubeLocalProxyWildcardDomain(teleportCluster string) string {
	return "*." + teleportCluster
}

// KubeLocalProxyPathPrefix returns the kubeconfig `server:` URL suffix that
// encodes the (teleport cluster, kube cluster) pair for URL-based routing.
// Format: /v1/teleport/<base64url(teleport)>/<base64url(kube)>. Mirrors the
// path consumed by the Teleport proxy's single-cert kube handler and the
// format emitted by tbot v2 (lib/tbot/services/k8s/output_v2.go).
func KubeLocalProxyPathPrefix(teleportCluster, kubeCluster string) string {
	return "/v1/teleport/" +
		base64.RawURLEncoding.EncodeToString([]byte(teleportCluster)) + "/" +
		base64.RawURLEncoding.EncodeToString([]byte(kubeCluster))
}

// ClustersFromKubeLocalProxyPath parses the leading
// /v1/teleport/<b64>/<b64> prefix of a request URL path and returns the
// decoded (teleport cluster, kube cluster) pair along with the rest of the
// path after the prefix (including the leading slash, e.g. "/api/v1/pods").
func ClustersFromKubeLocalProxyPath(path string) (teleportCluster, kubeCluster, rest string, err error) {
	trimmed := strings.TrimPrefix(path, "/")
	parts := strings.SplitN(trimmed, "/", 5)
	if len(parts) < 4 || parts[0] != "v1" || parts[1] != "teleport" {
		return "", "", "", trace.BadParameter("invalid kube local proxy path %q", path)
	}
	tcBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", "", "", trace.Wrap(err, "decoding teleport cluster name from path")
	}
	kcBytes, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil {
		return "", "", "", trace.Wrap(err, "decoding kube cluster name from path")
	}
	rest = "/"
	if len(parts) == 5 {
		rest = "/" + parts[4]
	}
	return string(tcBytes), string(kcBytes), rest, nil
}
