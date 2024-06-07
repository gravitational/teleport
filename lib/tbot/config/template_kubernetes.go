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

package config

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"path/filepath"

	"github.com/gravitational/trace"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/identity"
)

const defaultKubeconfigPath = "kubeconfig.yaml"

type templateKubernetes struct {
	clusterName          string
	executablePathGetter executablePathGetter
	disableExecPlugin    bool
}

func (t *templateKubernetes) name() string {
	return TemplateKubernetesName
}

func (t *templateKubernetes) describe() []FileDescription {
	return []FileDescription{
		{
			Name: defaultKubeconfigPath,
		},
	}
}

// kubernetesStatus holds teleport client information necessary to populate a
// kubeconfig.
type kubernetesStatus struct {
	clusterAddr           string
	teleportClusterName   string
	kubernetesClusterName string
	tlsServerName         string
	credentials           *client.Key
}

func generateKubeConfigWithoutPlugin(ks *kubernetesStatus) (*clientcmdapi.Config, error) {
	config := clientcmdapi.NewConfig()

	contextName := kubeconfig.ContextName(ks.teleportClusterName, ks.kubernetesClusterName)
	// Configure the cluster.
	clusterCAs, err := ks.credentials.RootClusterCAs()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cas := bytes.Join(clusterCAs, []byte("\n"))
	if len(cas) == 0 {
		return nil, trace.BadParameter("TLS trusted CAs missing in provided credentials")
	}
	config.Clusters[contextName] = &clientcmdapi.Cluster{
		Server:                   ks.clusterAddr,
		CertificateAuthorityData: cas,
		TLSServerName:            ks.tlsServerName,
	}

	config.AuthInfos[contextName] = &clientcmdapi.AuthInfo{
		ClientCertificateData: ks.credentials.TLSCert,
		ClientKeyData:         ks.credentials.PrivateKeyPEM(),
	}

	// Last, create a context linking the cluster to the auth info.
	config.Contexts[contextName] = &clientcmdapi.Context{
		Cluster:  contextName,
		AuthInfo: contextName,
	}
	config.CurrentContext = contextName

	return config, nil
}

// generateKubeConfigWithPlugin creates a Kubernetes config object with the given cluster
// config.
func generateKubeConfigWithPlugin(ks *kubernetesStatus, destPath string, executablePath string) (*clientcmdapi.Config, error) {
	config := clientcmdapi.NewConfig()

	// Implementation note: tsh/kube.go generates a kubeconfig with all
	// available clusters. This isn't especially useful in Machine ID when
	// there's _at most_ one cluster we have permission to access for this
	// Destination's set of certs, so instead of fetching all the k8s clusters
	// and adding everything, we'll just stick with the single cluster name in
	// our config file.
	// Otherwise, we adapt this from lib/kube/kubeconfig/kubeconfig.go's
	// Update(), but don't support env vars, insecure mode, or identity files.
	// We do still implement `tbot kube credentials` to help client apps
	// take better advantage of certificate renewals.

	contextName := kubeconfig.ContextName(ks.teleportClusterName, ks.kubernetesClusterName)

	// Configure the cluster.
	clusterCAs, err := ks.credentials.RootClusterCAs()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cas := bytes.Join(clusterCAs, []byte("\n"))
	if len(cas) == 0 {
		return nil, trace.BadParameter("TLS trusted CAs missing in provided credentials")
	}
	config.Clusters[contextName] = &clientcmdapi.Cluster{
		Server:                   ks.clusterAddr,
		CertificateAuthorityData: cas,
		TLSServerName:            ks.tlsServerName,
	}

	absDestPath, err := filepath.Abs(destPath)
	if err != nil {
		return nil, trace.Wrap(err, "determining absolute path for destination")
	}

	// Configure the auth info.
	execArgs := []string{"kube", "credentials",
		fmt.Sprintf("--destination-dir=%s", absDestPath),
	}
	config.AuthInfos[contextName] = &clientcmdapi.AuthInfo{
		Exec: &clientcmdapi.ExecConfig{
			APIVersion: "client.authentication.k8s.io/v1beta1",
			Command:    executablePath,
			Args:       execArgs,
		},
	}

	// Last, create a context linking the cluster to the auth info.
	config.Contexts[contextName] = &clientcmdapi.Context{
		Cluster:  contextName,
		AuthInfo: contextName,
	}
	config.CurrentContext = contextName

	return config, nil
}

func (t *templateKubernetes) render(
	ctx context.Context,
	bot provider,
	identity *identity.Identity,
	destination bot.Destination,
) error {
	ctx, span := tracer.Start(
		ctx,
		"templateKubernetes/render",
	)
	defer span.End()

	// Ping the proxy to resolve connection addresses.
	proxyPong, err := bot.ProxyPing(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	clusterAddr, tlsServerName, err := selectKubeConnectionMethod(proxyPong)
	if err != nil {
		return trace.Wrap(err)
	}

	hostCAs, err := bot.GetCertAuthorities(ctx, types.HostCA)
	if err != nil {
		return trace.Wrap(err)
	}

	key, err := newClientKey(identity, hostCAs)
	if err != nil {
		return trace.Wrap(err)
	}

	status := &kubernetesStatus{
		clusterAddr:           clusterAddr,
		tlsServerName:         tlsServerName,
		credentials:           key,
		teleportClusterName:   proxyPong.ClusterName,
		kubernetesClusterName: t.clusterName,
	}

	var cfg *clientcmdapi.Config
	if t.disableExecPlugin {
		// If they've disabled the exec plugin, we just write the credentials
		// directly into the kubeconfig.
		cfg, err = generateKubeConfigWithoutPlugin(status)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		// In exec plugin mode, we write the credentials to disk and write a
		// kubeconfig that execs `tbot` to load those credentials.

		// We only support directory mode for this since the exec plugin needs
		// to know the path to read the credentials from, and this is
		// unpredictable with other types of destination.
		destinationDir, ok := destination.(*DestinationDirectory)
		if !ok {
			return trace.BadParameter(
				"Destination %s must be a directory in exec plugin mode",
				destination,
			)
		}

		executablePath, err := t.executablePathGetter()
		if err != nil {
			return trace.Wrap(err)
		}

		cfg, err = generateKubeConfigWithPlugin(status, destinationDir.Path, executablePath)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	yamlCfg, err := clientcmd.Write(*cfg)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(destination.Write(ctx, defaultKubeconfigPath, yamlCfg))
}

// selectKubeConnectionMethod determines the address and SNI that should be
// put into the kubeconfig file.
func selectKubeConnectionMethod(proxyPong *webclient.PingResponse) (clusterAddr string, sni string, err error) {
	// First we check for TLS routing. If this is enabled, we use the Proxy's
	// PublicAddr, and we must also specify a special SNI.
	//
	// Even if KubePublicAddr is specified, we still use the general
	// PublicAddr when using TLS routing.
	if proxyPong.Proxy.TLSRoutingEnabled {
		addr := proxyPong.Proxy.SSH.PublicAddr
		host, _, err := net.SplitHostPort(proxyPong.Proxy.SSH.PublicAddr)
		if err != nil {
			return "", "", trace.Wrap(err, "parsing proxy public_addr")
		}

		return fmt.Sprintf("https://%s", addr), client.GetKubeTLSServerName(host), nil
	}

	// Next, we try to use the KubePublicAddr.
	if proxyPong.Proxy.Kube.PublicAddr != "" {
		return fmt.Sprintf("https://%s", proxyPong.Proxy.Kube.PublicAddr), "", nil
	}

	// Finally, we fall back to the main proxy PublicAddr with the port from
	// KubeListenAddr.
	if proxyPong.Proxy.Kube.ListenAddr != "" {
		host, _, err := net.SplitHostPort(proxyPong.Proxy.SSH.PublicAddr)
		if err != nil {
			return "", "", trace.Wrap(err, "parsing proxy public_addr")
		}

		_, port, err := net.SplitHostPort(proxyPong.Proxy.Kube.ListenAddr)
		if err != nil {
			return "", "", trace.Wrap(err, "parsing proxy kube_listen_addr")
		}

		return fmt.Sprintf("https://%s:%s", host, port), "", nil
	}

	return "", "", trace.BadParameter("unable to determine kubernetes address")
}
