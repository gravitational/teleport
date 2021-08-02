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
	"text/template"

	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/trace"
)

const sshConfigTemplate = `
# Common flags for all {{ .clusterName }} hosts
Host *.{{ .clusterName }} {{ .proxyHost }}
    UserKnownHostsFile "{{ .knownHostsPath }}"

# Flags for all {{ .clusterName }} hosts except the proxy
Host *.{{ .clusterName }} !{{ .proxyHost }}
    Port 3022
    {{- if .leaf }}
    ProxyCommand ssh -p {{ .proxyPort }} {{ .proxyHost }} -s proxy:$(echo %h | cut -d '.' -f 1):%p@{{ .clusterName }}	
    {{- else }}
    ProxyCommand ssh -p {{ .proxyPort }} {{ .proxyHost }} -s proxy:$(echo %h | cut -d '.' -f 1):%p
    {{- end }}
`

// writeSSHConfig generates an OpenSSH config block from the `sshConfigTemplate`
// template string.
func writeSSHConfig(sb *strings.Builder, clusterName string, knownHostsPath string, proxyHost string, proxyPort string, leaf bool) error {
	t, err := template.New("ssh-config").Parse(sshConfigTemplate)
	if err != nil {
		return trace.Wrap(err)
	}

	err = t.Execute(sb, map[string]interface{}{
		"clusterName":    clusterName,
		"proxyPort":      proxyPort,
		"proxyHost":      proxyHost,
		"knownHostsPath": knownHostsPath,
		"leaf":           leaf,
	})
	if err != nil {
		return trace.WrapWithMessage(err, "error generating SSH configuration from template")
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
	proxyHost, proxyPort, err := net.SplitHostPort(tc.Config.SSHProxyAddr)
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
	fmt.Fprintf(&sb, "#\n# Begin generated Teleport configuration for %s from `tsh config`\n#\n", tc.Config.WebProxyAddr)

	err = writeSSHConfig(&sb, rootClusterName, knownHostsPath, proxyHost, proxyPort, false)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, leafCluster := range leafClusters {
		err = writeSSHConfig(&sb, leafCluster.GetName(), knownHostsPath, proxyHost, proxyPort, true)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	fmt.Fprintf(&sb, "\n# End generated Teleport configuration\n")
	fmt.Print(sb.String())
	return nil
}
