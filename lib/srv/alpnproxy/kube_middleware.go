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
	"net/http"

	"github.com/gravitational/trace"
)

// KubeClientCerts is a map of Kubernetes client certs.
type KubeClientCerts map[string]tls.Certificate

// KubeMiddleware is a LocalProxyHTTPMiddleware for handling Kubernetes
// requests.
type KubeMiddleware struct {
	DefaultLocalProxyHTTPMiddleware

	certs KubeClientCerts
}

// NewKubeMiddleware creates a new KubeMiddleware.
func NewKubeMiddleware(certs KubeClientCerts) LocalProxyHTTPMiddleware {
	return &KubeMiddleware{
		certs: certs,
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

	cert, ok := m.certs[req.TLS.ServerName]
	if !ok {
		return nil, trace.NotFound("no client cert found for %v", req.TLS.ServerName)
	}
	return []tls.Certificate{cert}, nil
}
