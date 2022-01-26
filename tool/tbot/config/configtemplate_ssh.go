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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/tbot/identity"
	botutils "github.com/gravitational/teleport/tool/tbot/utils"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

const DEFAULT_PROXY_PORT = 3023

// ConfigTemplateSSHClient contains parameters for the ssh_config config
// template
type ConfigTemplateSSHClient struct {
	ProxyPort uint16 `yaml:"proxy_port"`
}

func (c *ConfigTemplateSSHClient) CheckAndSetDefaults() error {
	if c.ProxyPort == 0 {
		c.ProxyPort = DEFAULT_PROXY_PORT
	}
	return nil
}

func (c *ConfigTemplateSSHClient) Describe() []FileDescription {
	return []FileDescription{
		{
			Name:     "ssh_config",
			ModeHint: botutils.ModeHintSecret,
		},
		{
			Name:     "known_hosts",
			ModeHint: botutils.ModeHintSecret,
		},
	}
}

func (c *ConfigTemplateSSHClient) Render(authClient *auth.Client, currentIdentity *identity.Identity, destination *DestinationConfig) error {
	if !destination.ContainsKind(CONFIG_KIND_SSH) {
		return trace.BadParameter("%s config template requires kind `ssh` to be enabled", CONFIG_TEMPLATE_SSH_CLIENT)
	}

	dest, err := destination.GetDestination()
	if err != nil {
		return err
	}

	var (
		proxyHosts     []string
		firstProxyHost string
		firstProxyPort string
	)

	clusterName, err := authClient.GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}

	proxies, err := authClient.GetProxies()
	if err != nil {
		return trace.Wrap(err)
	}

	for i, proxy := range proxies {
		host, _, err := utils.SplitHostPort(proxy.GetPublicAddr())
		if err != nil {
			log.Debugf("proxy %+v has no usable public address", proxy)
			continue
		}

		if i == 0 {
			firstProxyHost = host

			// TODO: ideally it'd be nice to fetch this dynamically
			// TODO: eventually we could consider including `tsh proxy`
			// functionality and sidestep this entirely.
			firstProxyPort = fmt.Sprint(c.ProxyPort)
		}

		proxyHosts = append(proxyHosts, host)
	}

	if len(proxyHosts) == 0 {
		return trace.BadParameter("auth server has no proxies with a valid public address")
	}

	proxyHostStr := strings.Join(proxyHosts, ",")

	// Backend note: Prefer to use absolute paths for filesystem backends.
	// If the backend is something else, use "". ssh_config will generate with
	// paths relative to the destination.

	var dataDir string
	if dir, ok := dest.(*DestinationDirectory); ok {
		dataDir, err = filepath.Abs(dir.Path)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		dataDir = ""
	}

	knownHosts, err := fetchKnownHosts(authClient, clusterName.GetClusterName(), proxyHostStr)
	if err != nil {
		return trace.Wrap(err)
	}

	knownHostsPath := filepath.Join(dataDir, "known_hosts")
	if err := os.WriteFile(knownHostsPath, []byte(knownHosts), 0600); err != nil {
		return trace.Wrap(err)
	}

	var sshConfigBuilder strings.Builder
	identityFilePath := filepath.Join(dataDir, identity.PrivateKeyKey)
	certificateFilePath := filepath.Join(dataDir, identity.SSHCertKey)
	sshConfigPath := filepath.Join(dataDir, "ssh_config")
	if err := sshConfigTemplate.Execute(&sshConfigBuilder, sshConfigParameters{
		ClusterName:         clusterName.GetClusterName(),
		ProxyHost:           firstProxyHost,
		ProxyPort:           firstProxyPort,
		KnownHostsPath:      knownHostsPath,
		IdentityFilePath:    identityFilePath,
		CertificateFilePath: certificateFilePath,
		SSHConfigPath:       sshConfigPath,
	}); err != nil {
		return trace.Wrap(err)
	}

	if err := os.WriteFile(sshConfigPath, []byte(sshConfigBuilder.String()), 0600); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

type sshConfigParameters struct {
	ClusterName         string
	KnownHostsPath      string
	IdentityFilePath    string
	CertificateFilePath string
	ProxyHost           string
	ProxyPort           string
	SSHConfigPath       string
}

var sshConfigTemplate = template.Must(template.New("ssh-config").Parse(`
# Begin generated Teleport configuration for {{ .ProxyHost }} from tbot config

# Common flags for all {{ .ClusterName }} hosts
Host *.{{ .ClusterName }} {{ .ProxyHost }}
    UserKnownHostsFile "{{ .KnownHostsPath }}"
    IdentityFile "{{ .IdentityFilePath }}"
    CertificateFile "{{ .CertificateFilePath }}"
    HostKeyAlgorithms ssh-rsa-cert-v01@openssh.com
    PubkeyAcceptedAlgorithms +ssh-rsa-cert-v01@openssh.com

# Flags for all {{ .ClusterName }} hosts except the proxy
Host *.{{ .ClusterName }} !{{ .ProxyHost }}
    Port 3022
    ProxyCommand ssh -F {{ .SSHConfigPath }} -l %r -p {{ .ProxyPort }} {{ .ProxyHost }} -s proxy:%h:%p@{{ .ClusterName }}

# End generated Teleport configuration
`))

func fetchKnownHosts(client *auth.Client, clusterName, proxyHosts string) (string, error) {
	ca, err := client.GetCertAuthority(types.CertAuthID{
		Type:       types.HostCA,
		DomainName: clusterName,
	}, false)
	if err != nil {
		return "", trace.Wrap(err)
	}

	var sb strings.Builder
	for _, auth := range auth.AuthoritiesToTrustedCerts([]types.CertAuthority{ca}) {
		pubKeys, err := auth.SSHCertPublicKeys()
		if err != nil {
			return "", trace.Wrap(err)
		}

		for _, pubKey := range pubKeys {
			bytes := ssh.MarshalAuthorizedKey(pubKey)
			sb.WriteString(fmt.Sprintf(
				"@cert-authority %s,%s,*.%s %s type=host",
				proxyHosts, auth.ClusterName, auth.ClusterName, strings.TrimSpace(string(bytes)),
			))
		}
	}

	return sb.String(), nil
}
