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
	"strings"
	"sync"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/config/openssh"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/utils"
)

// TemplateSSHClient contains parameters for the ssh_config config
// template
type TemplateSSHClient struct {
	ProxyPort         uint16 `yaml:"proxy_port"`
	getSSHVersion     func() (*semver.Version, error)
	getExecutablePath func() (string, error)
}

const (
	// sshConfigName is the name of the ssh_config file on disk
	sshConfigName = "ssh_config"

	// knownHostsName is the name of the known_hosts file on disk
	knownHostsName = "known_hosts"
)

func (c *TemplateSSHClient) CheckAndSetDefaults() error {
	if c.ProxyPort != 0 {
		log.Warn("ssh_client's proxy_port parameter is deprecated and will be removed in a future release.")
	}
	if c.getSSHVersion == nil {
		c.getSSHVersion = openssh.GetSystemSSHVersion
	}
	if c.getExecutablePath == nil {
		c.getExecutablePath = os.Executable
	}
	return nil
}

func (c *TemplateSSHClient) Name() string {
	return TemplateSSHClientName
}

func (c *TemplateSSHClient) Describe(destination bot.Destination) []FileDescription {
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

func getClusterNames(
	bot provider, connectedClusterName string,
) ([]string, error) {
	allClusterNames := []string{connectedClusterName}

	leafClusters, err := bot.GetRemoteClusters()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, lc := range leafClusters {
		allClusterNames = append(allClusterNames, lc.GetName())
	}

	return allClusterNames, nil
}

func (c *TemplateSSHClient) Render(
	ctx context.Context,
	bot provider,
	_ *identity.Identity,
	destination *DestinationConfig,
) error {
	dest, err := destination.GetDestination()
	if err != nil {
		return trace.Wrap(err)
	}

	ping, err := bot.AuthPing(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	proxyHost, proxyPort, err := utils.SplitHostPort(ping.ProxyPublicAddr)
	if err != nil {
		return trace.BadParameter("proxy %+v has no usable public address: %v", ping.ProxyPublicAddr, err)
	}

	clusterNames, err := getClusterNames(bot, ping.ClusterName)
	if err != nil {
		return trace.Wrap(err)
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
	knownHosts, err := fetchKnownHosts(
		ctx,
		bot,
		clusterNames,
		proxyHost,
	)
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

	executablePath, err := c.getExecutablePath()
	if err != nil {
		return trace.Wrap(err)
	}

	var sshConfigBuilder strings.Builder
	knownHostsPath := filepath.Join(destDir, knownHostsName)
	identityFilePath := filepath.Join(destDir, identity.PrivateKeyKey)
	certificateFilePath := filepath.Join(destDir, identity.SSHCertKey)

	sshConf := openssh.NewSSHConfig(c.getSSHVersion, log)
	if err := sshConf.GetSSHConfig(&sshConfigBuilder, &openssh.SSHConfigParameters{
		AppName:             openssh.TbotApp,
		ClusterNames:        clusterNames,
		KnownHostsPath:      knownHostsPath,
		IdentityFilePath:    identityFilePath,
		CertificateFilePath: certificateFilePath,
		ProxyHost:           proxyHost,
		ProxyPort:           proxyPort,
		ExecutablePath:      executablePath,
		DestinationDir:      destDir,
	}); err != nil {
		return trace.Wrap(err)
	}

	if err := dest.Write(sshConfigName, []byte(sshConfigBuilder.String())); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func fetchKnownHosts(ctx context.Context, bot provider, clusterNames []string, proxyHosts string) (string, error) {
	certAuthorities := make([]types.CertAuthority, 0, len(clusterNames))
	for _, cn := range clusterNames {
		ca, err := bot.GetCertAuthority(ctx, types.CertAuthID{
			Type:       types.HostCA,
			DomainName: cn,
		}, false)
		if err != nil {
			return "", trace.Wrap(err)
		}
		certAuthorities = append(certAuthorities, ca)
	}

	var sb strings.Builder
	for _, auth := range auth.AuthoritiesToTrustedCerts(certAuthorities) {
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
