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

package kubev1

import (
	"crypto/tls"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gravitational/trace"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/kube/internal"
	"github.com/gravitational/teleport/lib/utils"
)

// getWebAddrAndKubeSNI returns the address of the web server that is running on this
// proxy and the Kube SNI. They are used to dial the Kube proxy to retrieve the pods
// available to the user.
// Since this grpc server is only enabled if the proxy is listening with
// multiplexing mode, the Kube proxy is always reachable on the same address as
// the web server using the SNI.
func getWebAddrAndKubeSNI(proxyAddr string) (string, string, error) {
	// we avoid using utils.SplitHostPort because
	// we allow the host to be empty
	addr, port, err := net.SplitHostPort(proxyAddr)
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	// validate the port
	if _, err := strconv.Atoi(port); err != nil {
		return "", "", trace.Wrap(err, "invalid port")
	}

	// if the proxy is an unspecified address (0.0.0.0, ::), use localhost.
	if ip := net.ParseIP(addr); ip != nil && ip.IsUnspecified() || addr == "" {
		addr = string(teleport.PrincipalLocalhost)
	}

	sni := client.GetKubeTLSServerName(addr)

	return sni, "https://" + net.JoinHostPort(addr, port), nil
}

// buildKubeClient creates a new Kubernetes client that is used to communicate
// with the Kubernetes API server.
func (s *Server) buildKubeClient() error {
	const idleConnsPerHost = 25

	tlsConfig := utils.TLSConfig(s.cfg.ConnTLSCipherSuites)
	tlsConfig.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
		tlsCert, err := s.cfg.GetConnTLSCertificate()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return tlsCert, nil
	}
	tlsConfig.InsecureSkipVerify = true
	tlsConfig.VerifyConnection = utils.VerifyConnectionWithRoots(s.cfg.GetConnTLSRoots)
	tlsConfig.ServerName = s.kubeProxySNI

	transport := utilnet.SetTransportDefaults(&http.Transport{
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     tlsConfig,
		MaxIdleConnsPerHost: idleConnsPerHost,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	})

	cfg := &rest.Config{
		Host:      s.proxyAddress,
		Transport: internal.NewImpersonatorRoundTripper(transport),
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return trace.Wrap(err)
	}
	s.kubeClient = kubeClient
	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return trace.Wrap(err)
	}
	s.kubeDynamicClient = dynamicClient

	return nil
}

// decideLimit returns the number of items we should request for. If respectLimit
// is true, it returns the difference between the max number of items and the
// number of items already included in the response.
// If false, returns the max number of items.
func decideLimit(limit, items int64, respectLimit bool) int64 {
	if respectLimit {
		return limit - items
	}
	return limit
}
