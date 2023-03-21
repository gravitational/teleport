/*
Copyright 2023 Gravitational, Inc.

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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path"
	"strings"
	"text/template"
	"time"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/utils"
)

type proxyKubeCommand struct {
	*kingpin.CmdClause
	kubeClusters      []string
	siteName          string
	impersonateUser   string
	impersonateGroups []string
	namespace         string
	port              string
	format            string
	configPath        string
	exec              string
}

func newProxyKubeCommand(parent *kingpin.CmdClause) *proxyKubeCommand {
	c := &proxyKubeCommand{
		CmdClause: parent.Command("kube", "Start local proxy for Kubernetes access."),
	}

	c.Flag("cluster", clusterHelp).Short('c').StringVar(&c.siteName)
	c.Arg("kube-cluster", "Name of the Kubernetes cluster to proxy. Check 'tsh kube ls' for a list of available clusters. If not specified, all clusters previously logged in through `tsh kube login` will be used.").StringsVar(&c.kubeClusters)
	c.Flag("as", "Configure custom Kubernetes user impersonation.").StringVar(&c.impersonateUser)
	c.Flag("as-groups", "Configure custom Kubernetes group impersonation.").StringsVar(&c.impersonateGroups)
	// TODO (tigrato): move this back to namespace once teleport drops the namespace flag.
	c.Flag("kube-namespace", "Configure the default Kubernetes namespace.").Short('n').StringVar(&c.namespace)
	c.Flag("port", "Specifies the source port used by the proxy listener").Short('p').StringVar(&c.port)
	c.Flag("format", envVarFormatFlagDescription()).Short('f').Default(envVarDefaultFormat()).EnumVar(&c.format, envVarFormats...)
	c.Flag("config-path", "Overwrites the default path for generating the ephemeral config.").StringVar(&c.configPath)
	c.Flag("exec", "Execute a command against the local proxy.").StringVar(&c.exec)
	return c
}

func (c *proxyKubeCommand) run(cf *CLIConf) error {
	tc, err := makeClient(cf, true)
	if err != nil {
		return trace.Wrap(err)
	}

	defaultConfig, clusters, err := c.prepare(cf, tc)
	if err != nil {
		return trace.Wrap(err)
	}

	profile, certs, err := loadAllKubeCerts(cf.Context, tc, clusters)
	if err != nil {
		return trace.Wrap(err)
	}

	listener, err := kubeLocalProxyListener(profile, c.port)
	if err != nil {
		return trace.Wrap(err)
	}

	lp, err := alpnproxy.NewLocalProxy(
		makeBasicLocalProxyConfig(cf, tc, listener),
		alpnproxy.WithHTTPMiddleware(alpnproxy.NewKubeMiddleware(certs)),
		alpnproxy.WithSNI(client.GetKubeTLSServerName(tc.WebProxyHost())),
		alpnproxy.WithClusterCAs(cf.Context, tc.RootClusterCACertPool),
	)
	if err != nil {
		if cerr := listener.Close(); cerr != nil {
			return trace.NewAggregate(err, cerr)
		}
		return trace.Wrap(err)
	}
	defer lp.Close()

	// Save the config for local proxy.
	if err := c.writeConfig(profile, tc, defaultConfig, clusters, lp); err != nil {
		return trace.Wrap(err)
	}
	defer removeFileIfExist(c.configPath)

	waitCtx, cancel := context.WithCancel(cf.Context)
	go func() {
		if err := lp.Start(cf.Context); err != nil {
			log.WithError(err).Errorf("Failed to start local ALPN proxy.")
		}
		cancel()
	}()

	if c.exec != "" {
		return trace.Wrap(c.runCommand(cf))
	}

	if err := c.printTemplate(cf, lp.GetAddr()); err != nil {
		return trace.Wrap(err)
	}
	<-waitCtx.Done()
	return nil
}

func (c *proxyKubeCommand) runCommand(cf *CLIConf) error {
	args := strings.Fields(c.exec)
	cmd := exec.CommandContext(cf.Context, args[0], args[1:]...)
	cmd.Stdin = cf.Stdin()
	cmd.Stdout = cf.Stdout()
	cmd.Stderr = cf.Stderr()
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%v", c.configPath))
	return trace.Wrap(cf.RunCommand(cmd))
}

func (c *proxyKubeCommand) prepare(cf *CLIConf, tc *client.TeleportClient) (*clientcmdapi.Config, []kubeconfig.LocalProxyClusterValues, error) {
	defaultConfig, err := kubeconfig.Load("")
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Use clusters from `tsh proxy kube` parameters
	if len(c.kubeClusters) > 0 {
		if c.siteName == "" {
			c.siteName = tc.SiteName
		}

		var clusters []kubeconfig.LocalProxyClusterValues
		for _, kubeCluster := range c.kubeClusters {
			clusters = append(clusters, kubeconfig.LocalProxyClusterValues{
				TeleportCluster:   c.siteName,
				KubeCluster:       kubeCluster,
				Impersonate:       c.impersonateUser,
				ImpersonateGroups: c.impersonateGroups,
				Namespace:         c.namespace,
				KubeClusters:      c.kubeClusters,
			})
		}
		c.printPrepare(cf, "Preparing the following Teleport Kubernetes clusters:", clusters)
		return defaultConfig, clusters, nil
	}

	// Use logged-in clusters.
	clusters := kubeconfig.LocalProxyClustersFromDefaultConfig(defaultConfig, tc.KubeClusterAddr())
	if len(clusters) == 0 {
		return nil, nil, trace.BadParameter(`No Kubernetes clusters found from the default kubeconfig.

Please provide Kubernetes cluster names to this command:
    tsh proxy kube <kube-cluster-1> <kube-cluster-2>

Or login the Kubernetes cluster first:
	tsh kube login <kube-cluster-1>
	tsh proxy kube`)
	}

	c.printPrepare(cf, "Preparing the following Teleport Kubernetes clusters from the default kubeconfig:", clusters)
	return defaultConfig, clusters, nil
}

func (c *proxyKubeCommand) writeConfig(profile *client.ProfileStatus, tc *client.TeleportClient, defaultConfig *clientcmdapi.Config, clusters []kubeconfig.LocalProxyClusterValues, lp *alpnproxy.LocalProxy) error {
	if c.configPath == "" {
		c.configPath = path.Join(profile.KubeConfigPath(fmt.Sprintf("localproxy-%v", lp.GetPort())))
	}

	// Let clients use the same cert as the local proxy server for simplicity.
	values := &kubeconfig.LocalProxyValues{
		LocalProxyAddr:   fmt.Sprintf("https://%s", lp.GetAddr()),
		LocalProxyCAPath: profile.LocalCAPath(),
		ClientKeyPath:    profile.KeyPath(),
		CliertCertPath:   profile.LocalCAPath(),
		Clusters:         clusters,
	}
	return trace.Wrap(kubeconfig.SaveLocalProxyValues(c.configPath, tc.KubeClusterAddr(), defaultConfig, values))
}

func (c *proxyKubeCommand) printPrepare(cf *CLIConf, title string, clusters []kubeconfig.LocalProxyClusterValues) {
	// Do not print anything if executing a command so that only the output of
	// the executed command will be shown.
	if c.exec != "" {
		return
	}

	fmt.Fprintln(cf.Stdout(), title)
	table := asciitable.MakeTable([]string{"Teleport Cluster Name", "Kube Cluster Name"})
	for _, cluster := range clusters {
		table.AddRow([]string{cluster.TeleportCluster, cluster.KubeCluster})
	}
	fmt.Fprintln(cf.Stdout(), table.AsBuffer().String())
}

func (c *proxyKubeCommand) printTemplate(cf *CLIConf, addr string) error {
	return trace.Wrap(proxyKubeTemplate.Execute(cf.Stdout(), map[string]interface{}{
		"addr":             addr,
		"format":           c.format,
		"randomPort":       c.port == "",
		"kubeconfigPath":   c.configPath,
		"kubeconfigEnvKey": "KUBECONFIG",
	}))
}

func kubeLocalProxyListener(profile *client.ProfileStatus, port string) (net.Listener, error) {
	localCA, err := loadSelfSignedCA(profile)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	x509ca, err := utils.TLSCertLeaf(localCA)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Let clients use the same cert as the local proxy server for simplicity.
	clientCertPool := x509.NewCertPool()
	clientCertPool.AddCert(x509ca)

	listener, err := alpnproxy.NewCertGenListener(alpnproxy.CertGenListenerConfig{
		ListenAddr: localListenAddr(port),
		CA:         localCA,
		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  clientCertPool,
	})
	return listener, trace.Wrap(err)
}

func loadAllKubeCerts(ctx context.Context, tc *client.TeleportClient, clusters []kubeconfig.LocalProxyClusterValues) (*client.ProfileStatus, alpnproxy.KubeClientCerts, error) {
	// Reuse the proxy client.
	var proxy *client.ProxyClient
	err := client.RetryWithRelogin(ctx, tc, func() error {
		var err error
		proxy, err = tc.ConnectToProxy(ctx)
		return trace.Wrap(err)
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	defer proxy.Close()

	// Local profile and kube certs at the same time.
	profile, key, err := tc.ProfileStatusAndKey(client.WithSSHCerts{}, client.WithKubeCerts{})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	keys := newKeyCache(tc.SiteName, key)
	all := make(alpnproxy.KubeClientCerts)
	for _, clusterValues := range clusters {
		clusterKey := alpnproxy.KubeClientCertKey{
			TeleportCluster: clusterValues.TeleportCluster,
			KubeCluster:     clusterValues.KubeCluster,
		}

		// Try load from store.
		cert, err := kubeCertFromClientStore(tc, keys, clusterKey)
		if err == nil {
			log.Debugf("Client cert loaded from keystore for %v.", clusterKey)
			all[clusterKey] = cert
			continue
		}
		if !trace.IsNotFound(err) {
			return nil, nil, trace.Wrap(err)
		}

		// Try issue.
		cert, err = issueKubeCert(ctx, tc, proxy, clusterKey)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		log.Debugf("Client cert issued for %v.", clusterKey)
		all[clusterKey] = cert
	}
	return profile, all, nil
}

func issueKubeCert(ctx context.Context, tc *client.TeleportClient, proxy *client.ProxyClient, cluster alpnproxy.KubeClientCertKey) (tls.Certificate, error) {
	key, mfaRequired, err := proxy.IssueUserCertsWithMFA(
		ctx,
		client.ReissueParams{
			RouteToCluster:    cluster.TeleportCluster,
			KubernetesCluster: cluster.KubeCluster,
		},
		nil, /*applyOpts*/ // TODO provide a hint for each cluster?
	)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	// Make sure the cert is allowed to access the cluster.
	// At this point we already know that the user has access to the cluster
	// via the RBAC rules, but we also need to make sure that the user has
	// access to the cluster with at least one kubernetes_user or kubernetes_group
	// defined.
	if err := checkIfCertsAreAllowedToAccessCluster(key, cluster.KubeCluster); err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	// Save it if not using MFA.
	if !mfaRequired {
		if err := tc.LocalAgent().AddKubeKey(key); err != nil {
			return tls.Certificate{}, trace.Wrap(err)
		}
	}

	cert, err := kubeCertFromAgentKey(key, cluster.KubeCluster)
	return cert, trace.Wrap(err)
}

type keyCache map[string]*client.Key

func newKeyCache(siteName string, siteKey *client.Key) keyCache {
	return map[string]*client.Key{
		siteName: siteKey,
	}
}

func (c keyCache) get(tc *client.TeleportClient, siteName string) (*client.Key, error) {
	if key, ok := c[siteName]; ok {
		return key, nil
	}
	if err := c.load(tc, siteName); err != nil {
		return nil, trace.Wrap(err)
	}
	return c[siteName], nil
}

func (c keyCache) load(tc *client.TeleportClient, siteName string) error {
	index := client.KeyIndex{
		ProxyHost:   tc.WebProxyAddr,
		Username:    tc.Username,
		ClusterName: siteName,
	}
	key, err := tc.ClientStore.KeyStore.GetKey(index, client.WithKubeCerts{})
	if err != nil {
		return trace.Wrap(err)
	}
	c[siteName] = key
	return nil
}

func kubeCertFromClientStore(tc *client.TeleportClient, keys keyCache, cluster alpnproxy.KubeClientCertKey) (tls.Certificate, error) {
	key, err := keys.get(tc, cluster.TeleportCluster)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	x509cert, err := key.KubeTLSCertificate(cluster.KubeCluster)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	if time.Until(x509cert.NotAfter) <= time.Minute {
		return tls.Certificate{}, trace.NotFound("TLS cert is expiring in a minute")
	}

	cert, err := kubeCertFromAgentKey(key, cluster.KubeCluster)
	return cert, trace.Wrap(err)
}

func kubeCertFromAgentKey(key *client.Key, kubeCluster string) (tls.Certificate, error) {
	certPem := key.KubeTLSCerts[kubeCluster]
	keyPem, err := key.PrivateKey.RSAPrivateKeyPEM()
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	cert, err := keys.X509KeyPair(certPem, keyPem)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	return cert, nil
}

// proxyKubeTemplate is the message that gets printed to a user when a kube proxy is started.
var proxyKubeTemplate = template.Must(template.New("").
	Funcs(template.FuncMap{
		"envVarCommand": envVarCommand,
	}).
	Parse(`Started local proxy for Kubernetes on {{.addr}}
{{if .randomPort}}To avoid port randomization, you can choose the listening port using the --port flag.
{{end}}
Use the following config for your Kubernetes applications. For example:
{{envVarCommand .format .kubeconfigEnvKey .kubeconfigPath}}
kubectl version

`))
