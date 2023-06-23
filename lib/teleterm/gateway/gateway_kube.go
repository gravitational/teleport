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

package gateway

import (
	"crypto/tls"
	"encoding/pem"
	"net"

	"github.com/gravitational/trace"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/utils"
)

// KubeconfigPath returns the kubeconfig path that can be used for clients to
// connect to the local proxy.
func (g *Gateway) KubeconfigPath() string {
	// Assumes CertPath is unique per kube cluster.
	return g.cfg.CertPath + ".kubeconfig"
}

func (g *Gateway) makeLocalProxiesForKube(listener net.Listener) error {
	if g.cfg.RootClusterCACertPoolFunc == nil {
		return trace.BadParameter("missing RootClusterCACertPoolFunc")
	}

	// A key is required here for generating local CAs. It can be any key.
	// Reading the provided key path to avoid generating a new one.
	key, err := keys.LoadPrivateKey(g.cfg.KeyPath)
	if err != nil {
		return trace.Wrap(err)
	}

	cas, err := alpnproxy.CreateKubeLocalCAs(key, []string{g.cfg.ClusterName})
	if err != nil {
		return trace.Wrap(err)
	}

	if err := g.makeALPNLocalProxyForKube(cas); err != nil {
		return trace.Wrap(err)
	}

	if err := g.makeForwardProxyForKube(listener); err != nil {
		return trace.NewAggregate(err, g.Close())
	}

	if err := g.writeKubeconfig(key, cas); err != nil {
		return trace.NewAggregate(err, g.Close())
	}
	return nil
}

func (g *Gateway) makeALPNLocalProxyForKube(cas map[string]tls.Certificate) error {
	// ALPN local proxy can use a random port.
	listener, err := alpnproxy.NewKubeListener(cas)
	if err != nil {
		return trace.Wrap(err)
	}

	middleware, err := g.makeKubeMiddleware()
	if err != nil {
		return trace.NewAggregate(err, listener.Close())
	}

	g.localProxy, err = alpnproxy.NewLocalProxy(alpnproxy.LocalProxyConfig{
		InsecureSkipVerify:      g.cfg.Insecure,
		RemoteProxyAddr:         g.cfg.WebProxyAddr,
		Listener:                listener,
		ParentContext:           g.closeContext,
		Clock:                   g.cfg.Clock,
		ALPNConnUpgradeRequired: g.cfg.TLSRoutingConnUpgradeRequired,
	},
		alpnproxy.WithHTTPMiddleware(middleware),
		alpnproxy.WithSNI(client.GetKubeTLSServerName(g.cfg.WebProxyAddr)),
		alpnproxy.WithClusterCAs(g.closeContext, g.cfg.RootClusterCACertPoolFunc),
	)
	if err != nil {
		return trace.NewAggregate(err, listener.Close())
	}
	return nil
}

func (g *Gateway) makeKubeMiddleware() (alpnproxy.LocalProxyHTTPMiddleware, error) {
	cert, err := keys.LoadX509KeyPair(g.cfg.CertPath, g.cfg.KeyPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certReissuer := newKubeCertReissuer(cert, g.onExpiredCert)
	g.onNewCert = certReissuer.updateCert

	certs := make(alpnproxy.KubeClientCerts)
	certs.Add(g.cfg.ClusterName, g.cfg.TargetName, cert)
	return alpnproxy.NewKubeMiddleware(certs, certReissuer.reissueCert, g.cfg.Clock, g.cfg.Log), nil
}

func (g *Gateway) makeForwardProxyForKube(listener net.Listener) (err error) {
	// Use provided listener with user configured port for the forward proxy.
	g.forwardProxy, err = alpnproxy.NewKubeForwardProxyWithListener(g.closeContext, listener, g.localProxy.GetAddr())
	return trace.Wrap(err)
}

func (g *Gateway) writeKubeconfig(key *keys.PrivateKey, cas map[string]tls.Certificate) error {
	ca, ok := cas[g.cfg.ClusterName]
	if !ok {
		return trace.BadParameter("CA for teleport cluster %q is missing", g.cfg.ClusterName)
	}

	x509Cert, err := utils.TLSCertLeaf(ca)
	if err != nil {
		return trace.BadParameter("could not parse CA certificate for cluster %q", g.cfg.ClusterName)
	}

	values := &kubeconfig.LocalProxyValues{
		// Ideally tc.KubeClusterAddr() should be used for
		// TeleportKubeClusterAddr as it matches what tsh kube login sets in
		// the kubeconfig. In this case it is not a big deal since this
		// ephemeral config has only a single kube cluster. Also
		// tc.KubeClusterAddr() is likely the same as WebProxyAddr anyway.
		TeleportKubeClusterAddr: "https://" + g.cfg.WebProxyAddr,
		LocalProxyURL:           "http://" + g.forwardProxy.GetAddr(),
		ClientKeyData:           key.PrivateKeyPEM(),
		Clusters: []kubeconfig.LocalProxyCluster{{
			TeleportCluster:   g.cfg.ClusterName,
			KubeCluster:       g.cfg.TargetName,
			Impersonate:       g.cfg.TargetUser,
			ImpersonateGroups: g.cfg.TargetGroups,
			Namespace:         g.cfg.TargetSubresourceName,
		}},
		LocalProxyCAs: map[string][]byte{
			g.cfg.ClusterName: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: x509Cert.Raw}),
		},
	}

	config := kubeconfig.CreateLocalProxyConfig(clientcmdapi.NewConfig(), values)
	return trace.Wrap(kubeconfig.Save(g.KubeconfigPath(), *config))
}
