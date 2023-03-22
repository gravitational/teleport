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
	"path"
	"strings"
	"text/template"
	"time"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	apiutils "github.com/gravitational/teleport/api/utils"
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
	// TODO support --exec to execute a command against the local proxy.
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

	localProxy, err := newKubeLocalProxy(cf, tc, clusters, c.port)
	if err != nil {
		return trace.Wrap(err)
	}
	defer localProxy.Close()

	// Save the config for local proxy.
	if err := c.writeConfig(tc, defaultConfig, clusters, localProxy); err != nil {
		return trace.Wrap(err)
	}

	if err := c.printTemplate(cf, localProxy.GetAddr()); err != nil {
		return trace.Wrap(err)
	}

	errChan := localProxy.Start(cf.Context)
	select {
	case err := <-errChan:
		return trace.Wrap(err)
	case <-cf.Context.Done():
		return nil
	}
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

func (c *proxyKubeCommand) writeConfig(tc *client.TeleportClient, defaultConfig *clientcmdapi.Config, clusters []kubeconfig.LocalProxyClusterValues, localProxy *kubeLocalProxy) error {
	if c.configPath == "" {
		_, port, _ := net.SplitHostPort(localProxy.GetAddr())
		c.configPath = path.Join(localProxy.KubeConfigPath(fmt.Sprintf("localproxy-%v", port)))
	}

	// Let clients use the same cert as the local proxy server for simplicity.
	values := &kubeconfig.LocalProxyValues{
		TeleportKubeClusterAddr: tc.KubeClusterAddr(),
		LocalProxyURL:           "http://" + localProxy.GetAddr(),
		LocalProxyCAPaths:       make(map[string]string),
		ClientKeyPath:           localProxy.KeyPath(),
		Clusters:                clusters,
	}
	for _, cluster := range clusters {
		values.LocalProxyCAPaths[cluster.TeleportCluster] = localProxy.KubeLocalCAPathForCluster(cluster.TeleportCluster)
	}
	return trace.Wrap(kubeconfig.SaveLocalProxyValues(c.configPath, defaultConfig, values))
}

func (c *proxyKubeCommand) printPrepare(cf *CLIConf, title string, clusters []kubeconfig.LocalProxyClusterValues) {
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

type kubeLocalProxy struct {
	*client.ProfileStatus
	lp *alpnproxy.LocalProxy
	fp *alpnproxy.ForwardProxy
}

func newKubeLocalProxy(cf *CLIConf, tc *client.TeleportClient, clusters []kubeconfig.LocalProxyClusterValues, port string) (*kubeLocalProxy, error) {
	profile, certs, err := loadAllKubeCerts(cf.Context, tc, clusters)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	lpListener, err := kubeLocalProxyListener(profile, teleportClustersFromKubeClusters(clusters))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	lp, err := alpnproxy.NewLocalProxy(
		makeBasicLocalProxyConfig(cf, tc, lpListener),
		alpnproxy.WithHTTPMiddleware(alpnproxy.NewKubeMiddleware(certs)),
		alpnproxy.WithSNI(client.GetKubeTLSServerName(tc.WebProxyHost())),
		alpnproxy.WithClusterCAs(cf.Context, tc.RootClusterCACertPool),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	addr := "localhost:0"
	if port != "" {
		addr = "localhost:" + port
	}
	fpListener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fp, err := alpnproxy.NewForwardProxy(alpnproxy.ForwardProxyConfig{
		Listener:     fpListener,
		CloseContext: cf.Context,
		Handlers: []alpnproxy.ConnectRequestHandler{
			alpnproxy.NewForwardToHostHandler(alpnproxy.ForwardToHostHandlerConfig{
				MatchFunc: alpnproxy.MatchAllRequests,
				Host:      lp.GetAddr(),
			}),
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &kubeLocalProxy{
		ProfileStatus: profile,
		lp:            lp,
		fp:            fp,
	}, nil
}

func (k *kubeLocalProxy) Start(ctx context.Context) chan error {
	errChan := make(chan error, 2)
	go func() {
		if err := k.fp.Start(); err != nil {
			errChan <- err
		}
	}()
	go func() {
		if err := k.lp.StartHTTPAccessProxy(ctx); err != nil {
			errChan <- err
		}
	}()
	return errChan
}

func (k *kubeLocalProxy) Close() error {
	return trace.NewAggregate(k.fp.Close(), k.lp.Close())
}

func (k *kubeLocalProxy) GetAddr() string {
	return k.fp.GetAddr()
}

func teleportClustersFromKubeClusters(clusters []kubeconfig.LocalProxyClusterValues) []string {
	teleportClusters := make([]string, 0, len(clusters))
	for _, cluster := range clusters {
		teleportClusters = append(teleportClusters, cluster.TeleportCluster)
	}
	return apiutils.Deduplicate(teleportClusters)
}

func kubeLocalProxyListener(profile *client.ProfileStatus, teleportClusters []string) (net.Listener, error) {
	configs := make(map[string]*tls.Config)
	for _, teleportCluster := range teleportClusters {
		localCA, err := loadSelfSignedCA(profile, profile.KubeLocalCAPathForCluster(teleportCluster), "*."+teleportCluster)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		x509ca, err := utils.TLSCertLeaf(localCA)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		localCA.Leaf = x509ca

		// Server and client are using the same certs.
		clientCerts := x509.NewCertPool()
		clientCerts.AddCert(x509ca)

		configs[teleportCluster] = &tls.Config{
			Certificates: []tls.Certificate{localCA},
			ClientAuth:   tls.RequireAndVerifyClientCert,
			ClientCAs:    clientCerts,
		}
	}

	listener, err := tls.Listen("tcp", "localhost:0", &tls.Config{
		GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
			_, teleportCluster, _ := strings.Cut(hello.ServerName, ".")
			config, ok := configs[teleportCluster]
			if !ok {
				return nil, trace.NotFound("unknown Teleport cluster %q", teleportCluster)
			}
			return config, nil
		},
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

	keys := map[string]*client.Key{
		tc.SiteName: key,
	}
	all := make(alpnproxy.KubeClientCerts)
	for _, cluster := range clusters {
		// Try load from store.
		cert, err := kubeCertFromClientStore(tc, keys, cluster.TeleportCluster, cluster.KubeCluster)
		if err == nil {
			log.Debugf("Client cert loaded from keystore for %v.", cluster)
			all[cluster.TLSServerName()] = cert
			continue
		}
		if !trace.IsNotFound(err) {
			return nil, nil, trace.Wrap(err)
		}

		// Try issue.
		cert, err = issueKubeCert(ctx, tc, proxy, cluster.TeleportCluster, cluster.KubeCluster)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		log.Debugf("Client cert issued for %v.", cluster)
		all[cluster.TLSServerName()] = cert
	}
	return profile, all, nil
}

func issueKubeCert(ctx context.Context, tc *client.TeleportClient, proxy *client.ProxyClient, teleportCluster, kubeCluster string) (tls.Certificate, error) {
	key, mfaRequired, err := proxy.IssueUserCertsWithMFA(
		ctx,
		client.ReissueParams{
			RouteToCluster:    teleportCluster,
			KubernetesCluster: kubeCluster,
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
	if err := checkIfCertsAreAllowedToAccessCluster(key, kubeCluster); err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	// Save it if not using MFA.
	if !mfaRequired {
		if err := tc.LocalAgent().AddKubeKey(key); err != nil {
			return tls.Certificate{}, trace.Wrap(err)
		}
	}

	cert, err := kubeCertFromAgentKey(key, kubeCluster)
	return cert, trace.Wrap(err)
}

func kubeCertFromClientStore(tc *client.TeleportClient, keys map[string]*client.Key, teleportCluster, kubeCluster string) (tls.Certificate, error) {
	key := keys[teleportCluster]
	if key == nil {
		index := client.KeyIndex{
			ProxyHost:   tc.WebProxyAddr,
			Username:    tc.Username,
			ClusterName: teleportCluster,
		}
		key, err := tc.ClientStore.KeyStore.GetKey(index, client.WithKubeCerts{})
		if err != nil {
			return tls.Certificate{}, trace.Wrap(err)
		}
		keys[teleportCluster] = key
	}

	x509cert, err := key.KubeTLSCertificate(kubeCluster)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	if time.Until(x509cert.NotAfter) <= time.Minute {
		return tls.Certificate{}, trace.NotFound("TLS cert is expiring in a minute")
	}

	cert, err := kubeCertFromAgentKey(key, kubeCluster)
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
