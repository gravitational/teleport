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

	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/trace"
)

func writeSSHConfig(sb *strings.Builder, clusterName string, knownHostsPath string, proxyHost string, proxyPort string, leaf bool) {
	// Generate configuration for all Teleport targets.
	fmt.Fprintf(sb, "# Common flags for all %s hosts\n", clusterName)
	fmt.Fprintf(sb, "Host *.%s %s\n", clusterName, proxyHost)
	fmt.Fprintf(sb, "    UserKnownHostsFile \"%s\"\n\n", knownHostsPath)

	fmt.Fprintf(sb, "# Flags for all %s hosts except the proxy\n", clusterName)
	fmt.Fprintf(sb, "Host *.%s !%s\n", clusterName, proxyHost)
	fmt.Fprintf(sb, "    Port 3022\n")

	// Note: This hard-codes port 3022. No single port can be guaranteed to
	// work for all hosts in a cluster and we'd need a custom proxy command to
	// look up the true value on demand.
	// Note: This relies on bash subshells and availability of the `cut`
	// command, and as such is not expected to work on Windows.
	var subsystem string
	if leaf {
		// For leaf clusters, append the cluster name per parseProxySubsysRequest().
		subsystem = fmt.Sprintf("proxy:$(echo %%h | cut -d '.' -f 1):%%p@%s", clusterName)
	} else {
		subsystem = fmt.Sprintf("proxy:$(echo %%h | cut -d '.' -f 1):%%p")
	}

	fmt.Fprintf(sb, "    ProxyCommand ssh -p %s %s -s %s\n\n", proxyPort, proxyHost, subsystem)
}

// onConfigSSH handles the `tsh config ssh` command
func onConfigSSH(cf *CLIConf) error {
	tc, err := makeClient(cf, true)
	if err != nil {
		return trace.Wrap(err)
	}

	// Note: TeleportClient.connectToProxy() overrides the proxy address when
	// JumpHosts are in use, which this does not currently implement.
	proxyHost, proxyPort, err := net.SplitHostPort(tc.Config.SSHProxyAddr)
	if err != nil {
		return err
	}

	// Note: We explicitly opt not to use RetryWithRelogin here as it will write
	// its prompt to stdout. If the user pipes this command's output, the
	// destination (possibly their ssh config file) may get polluted with
	// invalid output. Instead, rely on the normal error messages (which are
	// sent to stderr) and expect the user to log in manually.
	proxyClient, err := tc.ConnectToProxy(cf.Context)
	if err != nil {
		return err
	}
	defer proxyClient.Close()

	rootClusterName, rootErr := proxyClient.RootClusterName()
	leafClusters, leafErr := proxyClient.GetLeafClusters(cf.Context)
	if err := trace.NewAggregate(rootErr, leafErr); err != nil {
		return trace.Wrap(err)
	}

	keysDir := profile.FullProfilePath(tc.Config.KeysDir)
	knownHostsPath := keypaths.KnownHostsPath(keysDir)

	var sb strings.Builder

	// Start with a newline in case an existing config file does not end with
	// one.
	fmt.Fprintln(&sb)
	fmt.Fprintf(&sb, "#\n# Begin generated Teleport configuration for %s from `tsh config`\n#\n\n", tc.Config.WebProxyAddr)

	writeSSHConfig(&sb, rootClusterName, knownHostsPath, proxyHost, proxyPort, false)

	for _, leafCluster := range leafClusters {
		writeSSHConfig(&sb, leafCluster.GetName(), knownHostsPath, proxyHost, proxyPort, true)
	}

	fmt.Fprintf(&sb, "# End generated Teleport configuration\n")
	fmt.Print(sb.String())
	return nil
}
