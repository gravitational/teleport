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
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/utils"
)

// KubeClientCerts is a map of Kubernetes client certs.
type KubeClientCerts map[string]tls.Certificate

// Add adds a tls.Certificate for a kube cluster.
func (c KubeClientCerts) Add(teleportCluster, kubeCluster string, cert tls.Certificate) {
	c[common.KubeLocalProxySNI(teleportCluster, kubeCluster)] = cert
}

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

// NewKubeListener creates a listener for kube local proxy.
func NewKubeListener(casByTeleportCluster map[string]tls.Certificate) (net.Listener, error) {
	configs := make(map[string]*tls.Config)
	for teleportCluster, ca := range casByTeleportCluster {
		caLeaf, err := utils.TLSCertLeaf(ca)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		ca.Leaf = caLeaf

		// Server and client are using the same certs.
		clientCAs := x509.NewCertPool()
		clientCAs.AddCert(caLeaf)

		configs[teleportCluster] = &tls.Config{
			Certificates: []tls.Certificate{ca},
			ClientAuth:   tls.RequireAndVerifyClientCert,
			ClientCAs:    clientCAs,
		}
	}
	listener, err := tls.Listen("tcp", "localhost:0", &tls.Config{
		GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
			config, ok := configs[common.TeleportClusterFromKubeLocalProxySNI(hello.ServerName)]
			if !ok {
				return nil, trace.BadParameter("invalid server name %v", hello.ServerName)
			}
			return config, nil
		},
	})
	return listener, trace.Wrap(err)
}

// NewKubeForwardProxy creates a forward proxy for kube access.
func NewKubeForwardProxy(ctx context.Context, listenPort, forwardAddr string) (*ForwardProxy, error) {
	listenAddr := "localhost:0"
	if listenPort != "" {
		listenAddr = "localhost:" + listenPort
	}

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	fp, err := NewForwardProxy(ForwardProxyConfig{
		Listener:     listener,
		CloseContext: ctx,
		Handlers: []ConnectRequestHandler{
			NewForwardToHostHandler(ForwardToHostHandlerConfig{
				MatchFunc: MatchAllRequests,
				Host:      forwardAddr,
			}),
		},
	})
	if err != nil {
		return nil, trace.NewAggregate(listener.Close(), err)
	}
	return fp, nil
}
