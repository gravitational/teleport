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

	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/utils"
)

// KubeconfigPath returns the kubeconfig path that can be used for clients to
// connect to the local proxy.
func (g *gatewayImpl) KubeconfigPath() string {
	return keypaths.KubeConfigPath(
		g.cfg.ProfileDir,
		g.cfg.TargetURI.GetProfileName(),
		g.cfg.Username,
		g.cfg.ClusterName,
		g.cfg.TargetName,
	)
}

func (g *gatewayImpl) makeLocalProxiesForKube(listener net.Listener) error {
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
	// make sure kubeconfig is written again on new cert as a relogin may
	// cleanup profile dir.
	g.onNewCertFuncs = append(g.onNewCertFuncs, func(_ tls.Certificate) error {
		return trace.Wrap(g.writeKubeconfig(key, cas))
	})
	return nil
}

func (g *gatewayImpl) makeALPNLocalProxyForKube(cas map[string]tls.Certificate) error {
	// ALPN local proxy can use a random port as it receives requests from the
	// forward proxy so there should be no requests coming from users' clients
	// directly.
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

func (g *gatewayImpl) makeKubeMiddleware() (alpnproxy.LocalProxyHTTPMiddleware, error) {
	cert, err := keys.LoadX509KeyPair(g.cfg.CertPath, g.cfg.KeyPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certReissuer := newKubeCertReissuer(cert, g.onExpiredCert)
	g.onNewCertFuncs = append(g.onNewCertFuncs, certReissuer.updateCert)

	certs := make(alpnproxy.KubeClientCerts)
	certs.Add(g.cfg.ClusterName, g.cfg.TargetName, cert)
	return alpnproxy.NewKubeMiddleware(certs, certReissuer.reissueCert, g.cfg.Clock, g.cfg.Log), nil
}

func (g *gatewayImpl) makeForwardProxyForKube(listener net.Listener) (err error) {
	// Use provided listener with user configured port for the forward proxy.
	g.forwardProxy, err = alpnproxy.NewKubeForwardProxy(alpnproxy.KubeForwardProxyConfig{
		CloseContext: g.closeContext,
		Listener:     listener,
		ForwardAddr:  g.localProxy.GetAddr(),
	})
	return trace.Wrap(err)
}

func (g *gatewayImpl) writeKubeconfig(key *keys.PrivateKey, cas map[string]tls.Certificate) error {
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
		// TeleportKubeClusterAddr here.
		//
		// Kube cluster address is used as server address when `tsh kube login`
		// adds cluster entries in the default kubeconfig. When creating
		// kubeconfig for a local proxy, TeleportKubeClusterAddr is mainly used
		// to identify which clusters in the kubeconfig belong to the current
		// tsh profile, in case the default kubeconfig has other clusters. It
		// also serves as a reference so that the server address of a cluster
		// in the kubeconfig of `tsh proxy kube` and `tsh kube login` are the
		// same.
		//
		// In this case here, since the kubeconfig for the local proxy is only
		// for a single kube cluster and it is not created from the default
		// kubeconfig, specifying the correct kube cluster address is not
		// necessary.
		//
		// In most cases, tc.KubeClusterAddr() is the same as
		// g.cfg.WebProxyAddr anyway.
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
	if err := kubeconfig.Save(g.KubeconfigPath(), *config); err != nil {
		return trace.Wrap(err)
	}

	g.onCloseFuncs = append(g.onCloseFuncs, func() error {
		return trace.Wrap(utils.RemoveFileIfExist(g.KubeconfigPath()))
	})
	return nil
}
