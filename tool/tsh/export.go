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
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/trace"
)

func findServerByHostname(ctx context.Context, tc *client.TeleportClient, hostname string) (types.Server, error) {
	proxyClient, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer proxyClient.Close()

	site, err := proxyClient.CurrentClusterAccessPoint(ctx, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	siteNodes, err := site.GetNodes(ctx, tc.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, node := range siteNodes {
		if node.GetHostname() == hostname {
			return node, nil
		}
	}

	return nil, nil
}

// onExportSSH handles the `tsh export ssh` command.
func onExportSSH(cf *CLIConf) error {
	// TODO: Consider writing our per-cluster ssh configuration to a file in
	// .tsh and providing the user a Include directive to put in their
	// ~/.ssh/config. This will allow us to easily update only our configuration
	// (e.g. add/remove clusters, tweak proxy settings, etc).
	// Additionally, it becomes much easier to validate a correct config this
	// way, since we can check for the existence of a single (semi-)static line
	// in the user's ssh config.
	tc, err := makeClient(cf, true)
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO: TeleportClient.connectToProxy has fairly complicated logic to
	// resolve the proxyAddress when JumpHosts are in use, which this does not
	// currently implement. It should work as-written otherwise, however.
	proxyHost, _, err := net.SplitHostPort(tc.Config.SSHProxyAddr)
	if err != nil {
		return err
	}

	var rootClusterName string
	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		proxyClient, err := tc.ConnectToProxy(cf.Context)
		if err != nil {
			return err
		}
		defer proxyClient.Close()

		var rootErr, leafErr error
		rootClusterName, rootErr = proxyClient.RootClusterName()
		return trace.NewAggregate(rootErr, leafErr)
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Note: We technically could exclude proxyHost and rootClusterName, however
	// "caching" them here saves an additional network call and can be used to
	// make profile resolution unambiguous.
	proxyCommand := fmt.Sprintf("\"%s\" export proxy \"%s\" \"%s\" \"%%h\"", cf.executablePath, proxyHost, rootClusterName)

	var sb strings.Builder

	// Generate the root cluster config.
	fmt.Fprintf(&sb, "Host *.%s !%s\n", rootClusterName, proxyHost)
	fmt.Fprintf(&sb, "    ProxyCommand %s\n", proxyCommand)

	// TODO: leaf clusters

	fmt.Print(sb.String())

	return nil
}

func onTestNode(cf *CLIConf) error {
	tc, err := makeClient(cf, true)
	if err != nil {
		return trace.Wrap(err)
	}

	proxyHost, _, err := net.SplitHostPort(tc.Config.SSHProxyAddr)
	if err != nil {
		return err
	}

	// Save some time by not testing the proxy host itself.
	if cf.proxyNode == proxyHost {
		os.Exit(1)
	}

	// If the proxyNode flag is suffixed by the root cluster, remove it.
	target := strings.TrimSuffix(cf.proxyNode, "."+cf.proxyRootCluster)

	node, err := findServerByHostname(cf.Context, tc, target)
	if err != nil {
		return err
	} else if node == nil {
		return trace.NotFound("no node found with name %s", target)
	}

	return nil
}

// onExportProxy handles the `tsh export proxy` command.
func onExportProxy(cf *CLIConf) error {
	// TODO: This may replaced/enhanced by upcoming `tsh proxy` support.
	// TODO: Can we automatically select the proper proxy (if multiple profiles
	// are present) by accepting a proxy flag in this command?
	// TODO: How could we support non-default login names?
	// TODO: Can we use ProxyClient.dialAuthServer directly rather than using an
	// ssh subprocess?
	// TODO: Can we pre-bake the identity file?
	// TODO: Can we pre-bake known hosts?
	// TODO: Can we automatically refresh certificates as done in
	// https://gist.github.com/webvictim/6a306267cad85c024d93641985acfa0b ?

	tc, err := makeClient(cf, true)
	if err != nil {
		return trace.Wrap(err)
	}

	proxyHost, proxyPort, err := net.SplitHostPort(tc.Config.SSHProxyAddr)
	if err != nil {
		return err
	}

	// If the proxyNode flag is suffixed by the root cluster, remove it.
	target := strings.TrimSuffix(cf.proxyNode, "."+cf.proxyRootCluster)

	// Resolve the Server instance for the target and fetch its internal
	// address.
	var nodeAddr string
	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		node, err := findServerByHostname(cf.Context, tc, target)
		if err != nil {
			return err
		} else if node == nil {
			return trace.NotFound("no node found with name %s", target)
		}

		nodeAddr = node.GetAddr()
		return nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Note: This prefers to use the hostname as entered by the user rather than
	// the one returned by Teleport. Perhaps the latter should be preferred?
	_, nodePort, err := net.SplitHostPort(nodeAddr)
	if err != nil {
		return err
	}

	// TODO: If we can't proxy directly through teleport, we'll need something
	// more sophisticated to resolve the ssh client path.
	proxyString := fmt.Sprintf("proxy:%s:%s", target, nodePort)
	child := exec.Command("ssh", "-p", proxyPort, proxyHost, "-s", proxyString)
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	return child.Run()
}
