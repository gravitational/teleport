package configtemplate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/tbot/identity"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentTBot,
})

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

func WriteSSHConfig(client *auth.Client, dataDir string, validPrincipals []string) error {
	var (
		proxyHosts     []string
		firstProxyHost string
		firstProxyPort string
	)

	clusterName, err := client.GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}

	proxies, err := client.GetProxies()
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
			firstProxyPort = "3023" // TODO: need to resolve correct port somehow
		}

		proxyHosts = append(proxyHosts, host)
	}

	if len(proxyHosts) == 0 {
		return trace.BadParameter("auth server has no proxies with a valid public address")
	}

	proxyHostStr := strings.Join(proxyHosts, ",")

	knownHosts, err := fetchKnownHosts(client, clusterName.GetClusterName(), proxyHostStr)
	if err != nil {
		return trace.Wrap(err)
	}

	dataDir, err = filepath.Abs(dataDir)
	if err != nil {
		return trace.Wrap(err)
	}

	knownHostsPath := filepath.Join(dataDir, "known_hosts")
	if err := os.WriteFile(knownHostsPath, []byte(knownHosts), 0600); err != nil {
		return trace.Wrap(err)
	}

	log.Infof("Wrote known hosts configuration to %s", knownHostsPath)

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

	var principals string
	switch len(validPrincipals) {
	case 0:
		principals = "[user]"
	case 1:
		principals = validPrincipals[0]
	default:
		principals = fmt.Sprintf("[%s]", strings.Join(validPrincipals, "|"))
	}

	log.Infof("Wrote SSH configuration to %s", sshConfigPath)
	fmt.Printf("\nSSH usage example: ssh -F %s %s@[node].%s\n\n", sshConfigPath, principals, clusterName.GetClusterName())

	return nil
}

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
