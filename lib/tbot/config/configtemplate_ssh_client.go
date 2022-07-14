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
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"text/template"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/tbot/destination"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// TemplateSSHClient contains parameters for the ssh_config config
// template
type TemplateSSHClient struct {
	ProxyPort         uint16 `yaml:"proxy_port"`
	getSSHVersion     func() (*semver.Version, error)
	getExecutablePath func() (string, error)
}

// openSSHVersionRegex is a regex used to parse OpenSSH version strings.
var openSSHVersionRegex = regexp.MustCompile(`^OpenSSH_(?P<major>\d+)\.(?P<minor>\d+)(?:p(?P<patch>\d+))?`)

// openSSHMinVersionForRSAWorkaround is the OpenSSH version after which the
// RSA deprecation workaround should be added to generated ssh_config.
var openSSHMinVersionForRSAWorkaround = semver.New("8.5.0")

const (
	// sshConfigName is the name of the ssh_config file on disk
	sshConfigName = "ssh_config"

	// knownHostsName is the name of the known_hosts file on disk
	knownHostsName = "known_hosts"
)

// parseSSHVersion attempts to parse the local SSH version, used to determine
// certain config template parameters for client version compatibility.
func parseSSHVersion(versionString string) (*semver.Version, error) {
	versionTokens := strings.Split(versionString, " ")
	if len(versionTokens) == 0 {
		return nil, trace.BadParameter("invalid version string: %s", versionString)
	}

	versionID := versionTokens[0]
	matches := openSSHVersionRegex.FindStringSubmatch(versionID)
	if matches == nil {
		return nil, trace.BadParameter("cannot parse version string: %q", versionID)
	}

	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, trace.Wrap(err, "invalid major version number: %s", matches[1])
	}

	minor, err := strconv.Atoi(matches[2])
	if err != nil {
		return nil, trace.Wrap(err, "invalid minor version number: %s", matches[2])
	}

	patch := 0
	if matches[3] != "" {
		patch, err = strconv.Atoi(matches[3])
		if err != nil {
			return nil, trace.Wrap(err, "invalid patch version number: %s", matches[3])
		}
	}

	return &semver.Version{
		Major: int64(major),
		Minor: int64(minor),
		Patch: int64(patch),
	}, nil
}

// getSystemSSHVersion attempts to query the system SSH for its current version.
func getSystemSSHVersion() (*semver.Version, error) {
	var out bytes.Buffer

	cmd := exec.Command("ssh", "-V")
	cmd.Stderr = &out

	err := cmd.Run()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return parseSSHVersion(out.String())
}

func (c *TemplateSSHClient) CheckAndSetDefaults() error {
	if c.ProxyPort != 0 {
		log.Warn("ssh_client's proxy_port parameter is deprecated and will be removed in a future release.")
	}
	if c.getSSHVersion == nil {
		c.getSSHVersion = getSystemSSHVersion
	}
	if c.getExecutablePath == nil {
		c.getExecutablePath = os.Executable
	}
	return nil
}

func (c *TemplateSSHClient) Name() string {
	return TemplateSSHClientName
}

func (c *TemplateSSHClient) Describe(destination destination.Destination) []FileDescription {
	ret := []FileDescription{
		{
			Name: "known_hosts",
		},
	}

	// Only include ssh_config if we're using a filesystem destination as
	// otherwise ssh_config will not be sensible.
	if _, ok := destination.(*DestinationDirectory); ok {
		ret = append(ret, FileDescription{
			Name: "ssh_config",
		})
	}

	return ret
}

// sshConfigUnsupportedWarning is used to ensure we don't spam log messages if
// using non-filesystem backends.
var sshConfigUnsupportedWarning sync.Once

func (c *TemplateSSHClient) Render(ctx context.Context, authClient auth.ClientI, currentIdentity *identity.Identity, destination *DestinationConfig) error {
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

	// Backend note: Prefer to use absolute paths for filesystem backends.
	// If the backend is something else, use "". ssh_config will generate with
	// paths relative to the destination. This doesn't work with ssh in
	// practice so adjusting the config for impossible-to-determine-in-advance
	// destination backends is left as an exercise to the user.
	var destDir string
	if dir, ok := dest.(*DestinationDirectory); ok {
		destDir, err = filepath.Abs(dir.Path)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		destDir = ""
	}

	// We'll write known_hosts regardless of destination type, it's still
	// useful alongside a manually-written ssh_config.
	knownHosts, err := fetchKnownHosts(ctx, authClient, clusterName.GetClusterName(), proxyHost)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := dest.Write(knownHostsName, []byte(knownHosts)); err != nil {
		return trace.Wrap(err)
	}

	// If destDir is unset, we're not using a filesystem destination and
	// ssh_config will not be sensible. Log a note and bail early without
	// writing ssh_config. (Future users of k8s secrets will need to bring
	// their own config, we can't predict where paths will be in practice.)
	if destDir == "" {
		sshConfigUnsupportedWarning.Do(func() {
			log.Infof("Note: no ssh_config will be written for non-filesystem "+
				"destination %s.", dest)
		})
		return nil
	}

	// Default to including the RSA deprecation workaround.
	rsaWorkaround := true
	version, err := c.getSSHVersion()
	if err != nil {
		log.WithError(err).Debugf("Could not determine SSH version, will include RSA workaround.")
	} else if version.LessThan(*openSSHMinVersionForRSAWorkaround) {
		log.Debugf("OpenSSH version %s does not require workaround for RSA deprecation", version)
		rsaWorkaround = false
	} else {
		log.Debugf("OpenSSH version %s will use workaround for RSA deprecation", version)
	}

	executablePath, err := c.getExecutablePath()
	if err != nil {
		return trace.Wrap(err)
	}

	var sshConfigBuilder strings.Builder
	knownHostsPath := filepath.Join(destDir, knownHostsName)
	identityFilePath := filepath.Join(destDir, identity.PrivateKeyKey)
	certificateFilePath := filepath.Join(destDir, identity.SSHCertKey)
	if err := sshConfigTemplate.Execute(&sshConfigBuilder, sshConfigParameters{
		ClusterName:          clusterName.GetClusterName(),
		ProxyHost:            proxyHost,
		KnownHostsPath:       knownHostsPath,
		IdentityFilePath:     identityFilePath,
		CertificateFilePath:  certificateFilePath,
		IncludeRSAWorkaround: rsaWorkaround,
		TBotPath:             executablePath,
		DestinationDir:       destDir,
	}); err != nil {
		return trace.Wrap(err)
	}

	if err := dest.Write(sshConfigName, []byte(sshConfigBuilder.String())); err != nil {
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
	TBotPath            string
	DestinationDir      string

	// IncludeRSAWorkaround controls whether the RSA deprecation workaround is
	// included in the generated configuration. Newer versions of OpenSSH
	// deprecate RSA certificates and, due to a bug in golang's ssh package,
	// Teleport wrongly advertises its unaffected certificates as a
	// now-deprecated certificate type. The workaround includes a config
	// override to re-enable RSA certs for just Teleport hosts, however it is
	// only supported on OpenSSH 8.5 and later.
	IncludeRSAWorkaround bool
}

var sshConfigTemplate = template.Must(template.New("ssh-config").Parse(`
# Begin generated Teleport configuration for {{ .ProxyHost }} by tbot

# Common flags for all {{ .ClusterName }} hosts
Host *.{{ .ClusterName }} {{ .ProxyHost }}
    UserKnownHostsFile "{{ .KnownHostsPath }}"
    IdentityFile "{{ .IdentityFilePath }}"
    CertificateFile "{{ .CertificateFilePath }}"
    HostKeyAlgorithms ssh-rsa-cert-v01@openssh.com{{- if .IncludeRSAWorkaround }}
    PubkeyAcceptedAlgorithms +ssh-rsa-cert-v01@openssh.com{{- end }}

# Flags for all {{ .ClusterName }} hosts except the proxy
Host *.{{ .ClusterName }} !{{ .ProxyHost }}
    Port 3022
    ProxyCommand "{{ .TBotPath }}" proxy --destination-dir={{ .DestinationDir }} --proxy={{ .ProxyHost }} ssh --cluster={{ .ClusterName }}  %r@%h:%p

# End generated Teleport configuration
`))

func fetchKnownHosts(ctx context.Context, client auth.ClientI, clusterName, proxyHosts string) (string, error) {
	ca, err := client.GetCertAuthority(ctx, types.CertAuthID{
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
				"@cert-authority %s,%s,*.%s %s type=host\n",
				proxyHosts, auth.ClusterName, auth.ClusterName, strings.TrimSpace(string(bytes)),
			))
		}
	}

	return sb.String(), nil
}
