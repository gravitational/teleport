/*
Copyright 2022 Gravitational, Inc.

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

package config

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"

	"github.com/gravitational/trace"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/utils"
)

const defaultKubeconfigPath = "kubeconfig.yaml"

type TemplateKubernetes struct {
	Path string `yaml:"path,omitempty"`

	// ClusterAddress may be used to override the k8s cluster address. It's
	// resolved from Teleport's ping responses if unset.
	ClusterAddress string `yaml:"cluster_addr,omitempty"`

	getExecutablePath func() (string, error)
}

func (t *TemplateKubernetes) CheckAndSetDefaults() error {
	if t.Path == "" {
		t.Path = defaultKubeconfigPath
	}

	if t.getExecutablePath == nil {
		t.getExecutablePath = os.Executable
	}

	return nil
}

func (t *TemplateKubernetes) Name() string {
	return TemplateKubernetesName
}

func (t *TemplateKubernetes) Describe(destination bot.Destination) []FileDescription {
	return []FileDescription{
		{
			Name:  t.Path,
			IsDir: false,
		},
	}
}

// kubernetesStatus holds teleport client information necessary to populate a
// kubeconfig.
type kubernetesStatus struct {
	clusterAddr           string
	proxyAddr             string
	teleportClusterName   string
	kubernetesClusterName string
	tlsServerName         string
	credentials           *client.Key
}

func getKubeProxyHostPort(authPong *proto.PingResponse, proxyPong *webclient.PingResponse) (string, int, error) {
	addr := proxyPong.Proxy.Kube.PublicAddr
	if addr == "" {
		addr = authPong.ProxyPublicAddr
	}

	if addr == "" {
		return "", 0, trace.BadParameter(
			"Teleport server reported no usable public proxy address")
	}

	parsed, err := utils.ParseAddr(addr)
	if err != nil {
		return "", 0, trace.Wrap(err, "invalid proxy address")
	}

	return parsed.Host(), parsed.Port(defaults.KubeListenPort), nil
}

// generateKubeConfig creates a Kubernetes config object with the given cluster
// config.
func generateKubeConfig(t *TemplateKubernetes, ks *kubernetesStatus, destPath string) (*clientcmdapi.Config, error) {
	config := clientcmdapi.NewConfig()

	// Implementation note: tsh/kube.go generates a kubeconfig with all
	// available clusters. This isn't especially useful in Machine ID when
	// there's _at most_ one cluster we have permission to access for this
	// destination's set of certs, so instead of fetching all the k8s clusters
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

	// Configure the auth info.
	execArgs := []string{"kube", "credentials",
		fmt.Sprintf("--destination-dir=%s", destPath),
	}
	binaryPath, err := t.getExecutablePath()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	config.AuthInfos[contextName] = &clientcmdapi.AuthInfo{
		Exec: &clientcmdapi.ExecConfig{
			APIVersion: "client.authentication.k8s.io/v1beta1",
			Command:    binaryPath,
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

func (t *TemplateKubernetes) Render(
	ctx context.Context,
	bot provider,
	identity *identity.Identity,
	destination *DestinationConfig,
) error {
	if destination.KubernetesCluster == nil {
		dest, err := destination.GetDestination()
		if err != nil {
			return trace.Wrap(err)
		}

		return trace.BadParameter("No Kubernetes cluster was configured for destination %s, cannot generate config", dest)
	}

	dest, err := destination.GetDestination()
	if err != nil {
		return trace.Wrap(err)
	}

	// Only destination dirs are supported right now, but we could be flexible
	// on this in the future if needed.
	destinationDir, ok := dest.(*DestinationDirectory)
	if !ok {
		return trace.BadParameter("destination %s must be a directory", dest)
	}

	// Ping the auth server and proxy to resolve connection addresses.
	authPong, err := bot.AuthPing(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	proxyPong, err := bot.ProxyPing(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	host, port, err := getKubeProxyHostPort(authPong, proxyPong)
	if err != nil {
		return trace.Wrap(err)
	}
	kubeAddr := fmt.Sprintf("https://%s:%d", host, port)

	// Next, determine the TLS routing config (if any)
	// Note: derived from tool/tsh/kube.go; this impl should defer to it for
	// future changes.
	serverName := fmt.Sprintf("%s%s", constants.KubeTeleportProxyALPNPrefix, host)
	isIPFormat := net.ParseIP(host) != nil
	if host == "" || isIPFormat {
		serverName = fmt.Sprintf("%s%s", constants.KubeTeleportProxyALPNPrefix, constants.APIDomain)
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
		clusterAddr:           kubeAddr,
		proxyAddr:             authPong.ProxyPublicAddr,
		credentials:           key,
		teleportClusterName:   authPong.ClusterName,
		kubernetesClusterName: destination.KubernetesCluster.ClusterName,
	}

	if proxyPong.Proxy.TLSRoutingEnabled {
		status.tlsServerName = serverName
	}

	cfg, err := generateKubeConfig(t, status, destinationDir.Path)
	if err != nil {
		return trace.Wrap(err)
	}

	yamlCfg, err := clientcmd.Write(*cfg)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(dest.Write(t.Path, yamlCfg))
}
