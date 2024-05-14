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
	"context"
	"fmt"
	"path/filepath"
	"strings"

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

// templateSSHClient contains parameters for the ssh_config config
// template
type templateSSHClient struct {
	getSSHVersion        func() (*semver.Version, error)
	executablePathGetter executablePathGetter
	// destPath controls whether or not to write the SSH config file.
	// This is lets this be skipped on non-directory destinations where this
	// doesn't make sense.
	destPath string
}

const (
	// sshConfigName is the name of the ssh_config file on disk
	sshConfigName = "ssh_config"

	// knownHostsName is the name of the known_hosts file on disk
	knownHostsName = "known_hosts"
)

func (c *templateSSHClient) name() string {
	return TemplateSSHClientName
}

func (c *templateSSHClient) describe() []FileDescription {
	fds := []FileDescription{
		{
			Name: knownHostsName,
		},
	}

	if c.destPath != "" {
		fds = append(fds, FileDescription{
			Name: sshConfigName,
		})
	}

	return fds
}

func getClusterNames(
	ctx context.Context, bot provider, connectedClusterName string,
) ([]string, error) {
	allClusterNames := []string{connectedClusterName}

	leafClusters, err := bot.GetRemoteClusters(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, lc := range leafClusters {
		allClusterNames = append(allClusterNames, lc.GetName())
	}

	return allClusterNames, nil
}

func (c *templateSSHClient) render(
	ctx context.Context,
	bot provider,
	_ *identity.Identity,
	destination bot.Destination,
) error {
	ctx, span := tracer.Start(
		ctx,
		"templateSSHClient/render",
	)
	defer span.End()

	ping, err := bot.ProxyPing(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	proxyHost, proxyPort, err := utils.SplitHostPort(ping.Proxy.SSH.PublicAddr)
	if err != nil {
		return trace.BadParameter("proxy %+v has no usable public address: %v", ping.Proxy.SSH.PublicAddr, err)
	}

	clusterNames, err := getClusterNames(ctx, bot, ping.ClusterName)
	if err != nil {
		return trace.Wrap(err)
	}

	// We'll write known_hosts regardless of Destination type, it's still
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

	if err := destination.Write(ctx, knownHostsName, []byte(knownHosts)); err != nil {
		return trace.Wrap(err)
	}

	if c.destPath == "" {
		return nil
	}

	// Backend note: Prefer to use absolute paths for filesystem backends.
	// If the backend is something else, use "". ssh_config will generate with
	// paths relative to the Destination. This doesn't work with ssh in
	// practice so adjusting the config for impossible-to-determine-in-advance
	// Destination backends is left as an exercise to the user.
	absDestPath, err := filepath.Abs(c.destPath)
	if err != nil {
		return trace.Wrap(err)
	}

	executablePath, err := c.executablePathGetter()
	if err != nil {
		return trace.Wrap(err)
	}

	var sshConfigBuilder strings.Builder
	knownHostsPath := filepath.Join(absDestPath, knownHostsName)
	identityFilePath := filepath.Join(absDestPath, identity.PrivateKeyKey)
	certificateFilePath := filepath.Join(absDestPath, identity.SSHCertKey)

	sshConf := openssh.NewSSHConfig(c.getSSHVersion, nil)
	if err := sshConf.GetSSHConfig(&sshConfigBuilder, &openssh.SSHConfigParameters{
		AppName:             openssh.TbotApp,
		ClusterNames:        clusterNames,
		KnownHostsPath:      knownHostsPath,
		IdentityFilePath:    identityFilePath,
		CertificateFilePath: certificateFilePath,
		ProxyHost:           proxyHost,
		ProxyPort:           proxyPort,
		ExecutablePath:      executablePath,
		DestinationDir:      absDestPath,
	}); err != nil {
		return trace.Wrap(err)
	}

	if err := destination.Write(ctx, sshConfigName, []byte(sshConfigBuilder.String())); err != nil {
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
