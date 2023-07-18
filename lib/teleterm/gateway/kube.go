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
	"context"
	"crypto/tls"
	"encoding/pem"

	"github.com/gravitational/trace"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/teleport/api/utils/keys"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/utils"
)

type kube struct {
	*base
}

// KubeconfigPath returns the kubeconfig path that can be used for clients to
// connect to the local proxy.
func (k *kube) KubeconfigPath() string {
	return keypaths.KubeConfigPath(
		k.cfg.ProfileDir,
		k.cfg.TargetURI.GetProfileName(),
		k.cfg.Username,
		k.cfg.ClusterName,
		k.cfg.TargetName,
	)
}

func makeKubeGateway(cfg Config) (Kube, error) {
	base, err := newBase(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	k := &kube{base}

	// A key is required here for generating local CAs. It can be any key.
	// Reading the provided key path to avoid generating a new one.
	key, err := keys.LoadPrivateKey(k.cfg.KeyPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cas, err := alpnproxy.CreateKubeLocalCAs(key, []string{k.cfg.ClusterName})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := k.makeALPNLocalProxyForKube(cas); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := k.makeForwardProxyForKube(); err != nil {
		return nil, trace.NewAggregate(err, k.Close())
	}

	if err := k.writeKubeconfig(key, cas); err != nil {
		return nil, trace.NewAggregate(err, k.Close())
	}
	// make sure kubeconfig is written again on new cert as a relogin may
	// cleanup profile dir.
	k.onNewCertFuncs = append(k.onNewCertFuncs, func(_ tls.Certificate) error {
		return trace.Wrap(k.writeKubeconfig(key, cas))
	})
	return k, nil
}

func (k *kube) makeALPNLocalProxyForKube(cas map[string]tls.Certificate) error {
	// ALPN local proxy can use a random port as it receives requests from the
	// forward proxy so there should be no requests coming from users' clients
	// directly.
	listener, err := alpnproxy.NewKubeListener(cas)
	if err != nil {
		return trace.Wrap(err)
	}

	middleware, err := k.makeKubeMiddleware()
	if err != nil {
		return trace.NewAggregate(err, listener.Close())
	}

	k.localProxy, err = alpnproxy.NewLocalProxy(alpnproxy.LocalProxyConfig{
		InsecureSkipVerify:      k.cfg.Insecure,
		RemoteProxyAddr:         k.cfg.WebProxyAddr,
		Listener:                listener,
		ParentContext:           k.closeContext,
		Clock:                   k.cfg.Clock,
		ALPNConnUpgradeRequired: k.cfg.TLSRoutingConnUpgradeRequired,
	},
		alpnproxy.WithHTTPMiddleware(middleware),
		alpnproxy.WithSNI(client.GetKubeTLSServerName(k.cfg.WebProxyAddr)),
		alpnproxy.WithClusterCAs(k.closeContext, k.cfg.RootClusterCACertPoolFunc),
	)
	if err != nil {
		return trace.NewAggregate(err, listener.Close())
	}
	return nil
}

func (k *kube) makeKubeMiddleware() (alpnproxy.LocalProxyHTTPMiddleware, error) {
	cert, err := keys.LoadX509KeyPair(k.cfg.CertPath, k.cfg.KeyPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certReissuer := newKubeCertReissuer(cert, func(ctx context.Context) error {
		return trace.Wrap(k.cfg.OnExpiredCert(ctx, k))
	})
	k.onNewCertFuncs = append(k.onNewCertFuncs, certReissuer.updateCert)

	certs := make(alpnproxy.KubeClientCerts)
	certs.Add(k.cfg.ClusterName, k.cfg.TargetName, cert)
	return alpnproxy.NewKubeMiddleware(certs, certReissuer.reissueCert, k.cfg.Clock, k.cfg.Log), nil
}

func (k *kube) makeForwardProxyForKube() error {
	listener, err := k.cfg.makeListener()
	if err != nil {
		return trace.Wrap(err)
	}

	// Use provided listener with user configured port for the forward proxy.
	k.forwardProxy, err = alpnproxy.NewKubeForwardProxy(alpnproxy.KubeForwardProxyConfig{
		CloseContext: k.closeContext,
		Listener:     listener,
		ForwardAddr:  k.localProxy.GetAddr(),
	})
	if err != nil {
		return trace.NewAggregate(err, listener.Close())
	}
	return nil
}

func (k *kube) writeKubeconfig(key *keys.PrivateKey, cas map[string]tls.Certificate) error {
	ca, ok := cas[k.cfg.ClusterName]
	if !ok {
		return trace.BadParameter("CA for teleport cluster %q is missing", k.cfg.ClusterName)
	}

	x509Cert, err := utils.TLSCertLeaf(ca)
	if err != nil {
		return trace.BadParameter("could not parse CA certificate for cluster %q", k.cfg.ClusterName)
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
		// k.cfg.WebProxyAddr anyway.
		TeleportKubeClusterAddr: "https://" + k.cfg.WebProxyAddr,
		LocalProxyURL:           "http://" + k.forwardProxy.GetAddr(),
		ClientKeyData:           key.PrivateKeyPEM(),
		Clusters: []kubeconfig.LocalProxyCluster{{
			TeleportCluster:   k.cfg.ClusterName,
			KubeCluster:       k.cfg.TargetName,
			Impersonate:       k.cfg.TargetUser,
			ImpersonateGroups: k.cfg.TargetGroups,
			Namespace:         k.cfg.TargetSubresourceName,
		}},
		LocalProxyCAs: map[string][]byte{
			k.cfg.ClusterName: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: x509Cert.Raw}),
		},
	}

	config := kubeconfig.CreateLocalProxyConfig(clientcmdapi.NewConfig(), values)
	if err := kubeconfig.Save(k.KubeconfigPath(), *config); err != nil {
		return trace.Wrap(err)
	}

	k.onCloseFuncs = append(k.onCloseFuncs, func() error {
		return trace.Wrap(utils.RemoveFileIfExist(k.KubeconfigPath()))
	})
	return nil
}

func (k *kube) CLICommand() (*api.GatewayCLICommand, error) {
	// TODO(greedy52) currently kube must implement CLICommand in order to pass
	// Kube to CLICommandProvider. We should revisit gateway design/flows like
	// this. For example, one alternative is to move gateway.CLICommand to
	// daemon.GatewayCLICommand as daemon owns all CLICommandProvider.
	cmd, err := k.cfg.CLICommandProvider.GetCommand(k)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return makeCLICommand(cmd), nil
}
