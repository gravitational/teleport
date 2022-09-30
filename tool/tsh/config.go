/*
Copyright 2021 Gravitational, Inc.

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

package main

import (
	"fmt"
	"net"
	"strings"

	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/utils/keypaths"
)

// writeSSHConfig generates an OpenSSH config block from the `sshConfigTemplate`
// template string.
func writeSSHConfig(sb *strings.Builder, params *config.SSHConfigParameters) error {
	sshConf := config.NewSSHConfig(config.GetSystemSSHVersion, log)
	if err := sshConf.GetSSHConfig(sb, params); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// onConfig handles the `tsh config` command
func onConfig(cf *CLIConf) error {
	tc, err := makeClient(cf, true)
	if err != nil {
		return trace.Wrap(err)
	}

	// Note: TeleportClient.connectToProxy() overrides the proxy address when
	// JumpHosts are in use, which this does not currently implement.
	proxyHost, _, err := net.SplitHostPort(tc.Config.SSHProxyAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	// Note: We explicitly opt not to use RetryWithRelogin here as it will write
	// its prompt to stdout. If the user pipes this command's output, the
	// destination (possibly their ssh config file) may get polluted with
	// invalid output. Instead, rely on the normal error messages (which are
	// sent to stderr) and expect the user to log in manually.
	proxyClient, err := tc.ConnectToProxy(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}
	defer proxyClient.Close()

	rootClusterName, rootErr := proxyClient.RootClusterName(cf.Context)
	leafClusters, leafErr := proxyClient.GetLeafClusters(cf.Context)
	if err := trace.NewAggregate(rootErr, leafErr); err != nil {
		return trace.Wrap(err)
	}

	keysDir := profile.FullProfilePath(tc.Config.KeysDir)
	knownHostsPath := keypaths.KnownHostsPath(keysDir)
	identityFilePath := keypaths.UserKeyPath(keysDir, proxyHost, tc.Config.Username)

	var sb strings.Builder

	// Start with a newline in case an existing config file does not end with
	// one.
	fmt.Fprintln(&sb)
	fmt.Fprintf(&sb, "#\n# Begin generated Teleport configuration for %s from `tsh config`\n#\n", tc.Config.WebProxyAddr)

	if err := writeSSHConfig(&sb, &config.SSHConfigParameters{
		AppName:          "tsh",
		ClusterName:      rootClusterName,
		KnownHostsPath:   knownHostsPath,
		IdentityFilePath: identityFilePath,
		CertificateFilePath: keypaths.SSHCertPath(keysDir, proxyHost,
			tc.Config.Username, rootClusterName),
		ProxyHost:      proxyHost,
		ExecutablePath: cf.executablePath,
	}); err != nil {
		return trace.Wrap(err)
	}

	for _, leafCluster := range leafClusters {
		err = writeSSHConfig(&sb, &config.SSHConfigParameters{
			AppName:          "tsh",
			ClusterName:      leafCluster.GetName(),
			KnownHostsPath:   knownHostsPath,
			IdentityFilePath: identityFilePath,
			CertificateFilePath: keypaths.SSHCertPath(keysDir, proxyHost,
				tc.Config.Username, rootClusterName),
			ProxyHost:      proxyHost,
			ExecutablePath: cf.executablePath,
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}

	fmt.Fprintf(&sb, "\n# End generated Teleport configuration\n")

	stdout := cf.Stdout()
	fmt.Fprint(stdout, sb.String())
	return nil
}
