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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/tbot/destination"
	"github.com/gravitational/teleport/tool/tbot/identity"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// TemplateSSHClient contains parameters for the ssh_config config
// template
type TemplateSSHClient struct {
	ProxyPort uint16 `yaml:"proxy_port"`
}

func (c *TemplateSSHClient) CheckAndSetDefaults() error {
	if c.ProxyPort == 0 {
		c.ProxyPort = defaults.SSHProxyListenPort
	}
	return nil
}

func (c *TemplateSSHClient) Describe() []FileDescription {
	return []FileDescription{
		{
			Name:     "ssh_config",
			ModeHint: destination.ModeHintSecret,
		},
		{
			Name:     "known_hosts",
			ModeHint: destination.ModeHintSecret,
		},
	}
}

func (c *TemplateSSHClient) Render(ctx context.Context, authClient auth.ClientI, currentIdentity *identity.Identity, destination *DestinationConfig) error {
	if !destination.ContainsKind(KindSSH) {
		return trace.BadParameter("%s config template requires kind `ssh` to be enabled", TemplateSSHClientName)
	}

	dest, err := destination.GetDestination()
	if err != nil {
		return trace.Wrap(err)
	}

	clusterName, err := authClient.GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}

	ping, err := authClient.Ping(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	proxyHost, _, err := utils.SplitHostPort(ping.ProxyPublicAddr)
	if err != nil {
		return trace.BadParameter("proxy %+v has no usable public address: %v", ping.ProxyPublicAddr, err)
	}

	// TODO: ideally it'd be nice to fetch this dynamically
	// TODO: eventually we could consider including `tsh proxy`
	// functionality and sidestep this entirely.
	proxyPort := strconv.Itoa(int(c.ProxyPort))

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

	knownHosts, err := fetchKnownHosts(authClient, clusterName.GetClusterName(), proxyHost)
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
		ProxyHost:           proxyHost,
		ProxyPort:           proxyPort,
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

func fetchKnownHosts(client auth.ClientI, clusterName, proxyHosts string) (string, error) {
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
