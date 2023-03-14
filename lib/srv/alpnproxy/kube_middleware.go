/*
Copyright 2023 Gravitational, Inc.

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

package alpnproxy

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/constants"
)

// KubeClientCertKey is the key used for caching client keys.
type KubeClientCertKey struct {
	// TeleportCluster is the name of the Teleport cluster.
	TeleportCluster string
	// KubeCluster is the name of the Kubernetes cluster.
	KubeCluster string
}

// String implements Stringer interface.
func (k KubeClientCertKey) String() string {
	return fmt.Sprintf("Teleport cluster %q Kubernetes cluster %q", k.TeleportCluster, k.KubeCluster)
}

func newKubeClientCertKeyFromSNI(sni string) (KubeClientCertKey, error) {
	kubeCluster, teleportCluster, found := strings.Cut(sni, constants.KubeTeleportLocalProxyDelimiter)
	if !found || kubeCluster == "" || teleportCluster == "" {
		return KubeClientCertKey{}, trace.BadParameter("expect tls-server-name in format of <kube-cluster>%s<teleport-cluster>", constants.KubeTeleportLocalProxyDelimiter)
	}

	return KubeClientCertKey{
		TeleportCluster: teleportCluster,
		KubeCluster:     kubeCluster,
	}, nil
}

// KubeClientCerts is a map of Kubernetes client certs.
type KubeClientCerts map[KubeClientCertKey]tls.Certificate

// KubeMiddleware is a LocalProxyHTTPMiddleware for handling Kubernetes
// requests.
type KubeMiddleware struct {
	DefaultLocalProxyHTTPMiddleware

	certs KubeClientCerts
	log   logrus.FieldLogger
}

// NewKubeMiddleware creates a new KubeMiddleware.
func NewKubeMiddleware(certs KubeClientCerts) LocalProxyHTTPMiddleware {
	return &KubeMiddleware{
		certs: certs,
		log:   logrus.WithField(trace.Component, "kube"),
	}
}

// CheckAndSetDefaults checks configuration validity and sets defaults
func (m *KubeMiddleware) CheckAndSetDefaults() error {
	if m.certs == nil {
		return trace.BadParameter("missing certs")
	}
	return nil
}

// OverwriteClientCerts overwrites the client certs used for upstream connection.
func (m *KubeMiddleware) OverwriteClientCerts(req *http.Request) ([]tls.Certificate, error) {
	if req.TLS == nil {
		return nil, trace.BadParameter("expect a TLS request")
	}

	key, err := newKubeClientCertKeyFromSNI(req.TLS.ServerName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	m.log.Debugf("Received Kubernetes request for %v", key)
	cert, ok := m.certs[key]
	if !ok {
		return nil, trace.NotFound("no client cert found for %v", key)
	}
	return []tls.Certificate{cert}, nil
}
