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

package common

import (
	"context"
	"crypto/tls"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	template "github.com/DataDog/datadog-agent/pkg/template/text"
	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/utils"
)

type proxyKubeCommand struct {
	*kingpin.CmdClause
	kubeClusters        []string
	siteName            string
	impersonateUser     string
	impersonateGroups   []string
	namespace           string
	port                string
	format              string
	overrideContextName string

	labels              string
	predicateExpression string
	exec                bool
	execCmd             string   // Command to execute when --exec is enabled
	execArgs            []string // Arguments for the command
}

func newProxyKubeCommand(parent *kingpin.CmdClause) *proxyKubeCommand {
	c := &proxyKubeCommand{
		CmdClause: parent.Command("kube", "Start local proxy for Kubernetes access."),
	}

	c.Flag("cluster", clusterHelp).Short('c').StringVar(&c.siteName)
	c.Arg("kube-cluster", "Name of the Kubernetes cluster to proxy. Check 'tsh kube ls' for a list of available clusters. If not specified, all clusters previously logged in through `tsh kube login` will be used.").StringsVar(&c.kubeClusters)
	c.Flag("as", "Configure custom Kubernetes user impersonation.").StringVar(&c.impersonateUser)
	c.Flag("as-groups", "Configure custom Kubernetes group impersonation.").StringsVar(&c.impersonateGroups)
	// kube-namespace exists for backwards compatibility.
	c.Flag("kube-namespace", "Configure the default Kubernetes namespace.").Hidden().StringVar(&c.namespace)
	c.Flag("namespace", "Configure the default Kubernetes namespace.").Short('n').StringVar(&c.namespace)
	c.Flag("port", "Specifies the source port used by the proxy listener.").Short('p').StringVar(&c.port)
	c.Flag("format", envVarFormatFlagDescription()).Short('f').Default(envVarDefaultFormat()).EnumVar(&c.format, envVarFormats...)
	c.Flag("labels", labelHelp).StringVar(&c.labels)
	c.Flag("query", queryHelp).StringVar(&c.predicateExpression)
	c.Flag("set-context-name", "Define a custom context name or template.").
		// Use the default context name template if --set-context-name is not set.
		// This works as an hint to the user that the context name can be customized.
		Default(kubeconfig.ContextName("{{.ClusterName}}", "{{.KubeName}}")).
		StringVar(&c.overrideContextName)
	c.Flag("exec", "Run the proxy in the background and reexec into a new shell with $KUBECONFIG already pointed to our config file.").BoolVar(&c.exec)
	c.Flag("exec-cmd", "Command to execute when --exec is enabled. If not specified, defaults to $SHELL or /bin/bash. Implicitly enables exec mode.").StringVar(&c.execCmd)
	c.Flag("exec-arg", "Arguments to pass to the executed command (can be specified multiple times).").StringsVar(&c.execArgs)
	return c
}

func (c *proxyKubeCommand) run(cf *CLIConf) error {
	cf.Labels = c.labels
	cf.PredicateExpression = c.predicateExpression
	cf.SiteName = c.siteName

	// Validate flag combinations
	if len(c.execArgs) > 0 && c.execCmd == "" {
		return trace.BadParameter("cannot use --exec-arg without --exec-cmd")
	}

	if len(c.kubeClusters) > 1 || cf.Labels != "" || cf.PredicateExpression != "" {
		err := kubeconfig.CheckContextOverrideTemplate(c.overrideContextName)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	if cf.Headless {
		tc.AllowHeadless = true
	}

	defaultConfig, clusters, err := c.prepare(cf, tc)
	if err != nil {
		return trace.Wrap(err)
	}

	localProxy, err := makeKubeLocalProxy(cf, tc, clusters, defaultConfig, c.port, c.overrideContextName)
	if err != nil {
		return trace.Wrap(err)
	}
	defer localProxy.Close()

	// re-exec into a new shell with $KUBECONFIG already pointed to our config file
	// if --exec flag is set, --exec-cmd is provided, or headless mode is enabled.
	reexecIntoShell := cf.Headless || c.exec || c.execCmd != ""
	if err := c.printTemplate(cf.Stdout(), reexecIntoShell, localProxy); err != nil {
		return trace.Wrap(err)
	}

	// cf.cmdRunner is used for test only.
	if cf.cmdRunner != nil {
		if err := localProxy.WriteKubeConfig(); err != nil {
			return trace.Wrap(err)
		}
		go localProxy.Start(cf.Context)

		command := getExecCommand(c.execCmd)
		cmd := &exec.Cmd{
			Path: "test",
			Args: append([]string{command}, c.execArgs...),
			Env:  []string{"KUBECONFIG=" + localProxy.KubeConfigPath()},
		}
		return trace.Wrap(cf.RunCommand(cmd))
	}

	if reexecIntoShell {
		// If headless, run proxy in the background and reexec into a new shell with $KUBECONFIG already pointed to
		// our config file
		return trace.Wrap(runHeadlessKubeProxy(cf, localProxy, c.execCmd, c.execArgs))
	} else {
		// Write kubeconfig to a file and start local proxy in regular mode
		if err := localProxy.WriteKubeConfig(); err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(localProxy.Start(cf.Context))
	}
}

// getExecCommand returns the command to execute for the kube proxy shell.
// Priority: 1) provided command, 2) SHELL env var, 3) /bin/bash
func getExecCommand(command string) string {
	if command == "" {
		command = "/bin/bash"
		if shell, ok := os.LookupEnv("SHELL"); ok {
			command = shell
		}
	}
	return command
}

func runHeadlessKubeProxy(cf *CLIConf, localProxy *kubeLocalProxy, command string, args []string) error {
	// Getting context with cancel function, so we could stop shell process if localProxy stops.
	ctx, cancel := context.WithCancel(cf.Context)

	configBytes, err := clientcmd.Write(*localProxy.kubeconfig)
	if err != nil {
		cancel()
		return trace.Wrap(err)
	}

	lpErrChan := make(chan error)
	go func() {
		defer cancel()

		lpErrChan <- localProxy.Start(ctx)
	}()

	err = reexecToShell(ctx, configBytes, command, args)
	err = trace.NewAggregate(err, localProxy.Close())
	_, _ = fmt.Fprint(cf.Stdout(), "Local proxy for Kubernetes is closed.\n")
	err = trace.NewAggregate(err, <-lpErrChan)
	return err
}

func getPrepareErrorMessage(headless bool) string {
	headlessFlag := ""
	secondPart := `

Or login the Kubernetes cluster first:
    tsh kube login <kube-cluster-1>
    tsh proxy kube`

	if headless {
		headlessFlag = "--headless "
		secondPart = ""
	}
	errorMsg := fmt.Sprintf(`No Kubernetes clusters found to proxy.

Please provide Kubernetes cluster names or labels or predicate expression to this command:
    tsh %[1]sproxy kube <kube-cluster-1> <kube-cluster-2>
    tsh %[1]sproxy kube --labels env=root
    tsh %[1]sproxy kube --query 'labels["env"]=="root"'%[2]s`, headlessFlag, secondPart)

	return errorMsg
}

func (c *proxyKubeCommand) prepare(cf *CLIConf, tc *client.TeleportClient) (*clientcmdapi.Config, kubeconfig.LocalProxyClusters, error) {
	defaultConfig, err := kubeconfig.Load(getKubeConfigPath(cf, ""))
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	errorMsg := getPrepareErrorMessage(cf.Headless)

	// Use kube clusters from arg.
	if len(c.kubeClusters) > 0 || cf.Labels != "" || cf.PredicateExpression != "" {
		_, kubeClusters, err := fetchKubeClusters(cf.Context, tc)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		switch len(c.kubeClusters) {
		case 0:
			// if no names are given, check just the labels/predicate selection.
			if err := checkClusterSelection(cf, kubeClusters, ""); err != nil {
				return nil, nil, trace.Wrap(err)
			}
		default:
			// otherwise, check that each name matches exactly one kube cluster.
			matchMap := matchClustersByNames(kubeClusters, c.kubeClusters...)
			if err := checkMultipleClusterSelections(cf, matchMap); err != nil {
				return nil, nil, trace.Wrap(err)
			}
			kubeClusters = combineMatchedClusters(matchMap)
		}
		var clusters kubeconfig.LocalProxyClusters
		for _, kc := range kubeClusters {
			clusters = append(clusters, kubeconfig.LocalProxyCluster{
				TeleportCluster:   tc.SiteName,
				KubeCluster:       kc.GetName(),
				Impersonate:       c.impersonateUser,
				ImpersonateGroups: c.impersonateGroups,
				Namespace:         c.namespace,
			})
		}
		c.printPrepare(cf, "Preparing the following Teleport Kubernetes clusters:", clusters)
		return defaultConfig, clusters, nil
	}

	// In headless mode it's assumed user works on a remote machine where they don't have
	// tsh credentials and can't login into Teleport Kubernetes clusters.
	if cf.Headless {
		return nil, nil, trace.BadParameter("%s", errorMsg)
	}

	// Use logged-in clusters.
	clusters := kubeconfig.LocalProxyClustersFromDefaultConfig(defaultConfig, tc.WebProxyHost(), tc.KubeClusterAddr())
	if len(clusters) == 0 {
		return nil, nil, trace.BadParameter("%s", errorMsg)
	}

	c.printPrepare(cf, "Preparing the following Teleport Kubernetes clusters from the default kubeconfig:", clusters)
	return defaultConfig, clusters, nil
}

func (c *proxyKubeCommand) printPrepare(cf *CLIConf, title string, clusters kubeconfig.LocalProxyClusters) {
	fmt.Fprintln(cf.Stdout(), title)
	table := asciitable.MakeTable([]string{"Teleport Cluster Name", "Kube Cluster Name", "Context Name"})
	for _, cluster := range clusters {
		contextName, err := kubeconfig.ContextNameFromTemplate(c.overrideContextName, cluster.TeleportCluster, cluster.KubeCluster)
		if err != nil {
			logger.WarnContext(cf.Context, "Failed to generate context name", "error", err)
			contextName = kubeconfig.ContextName(cluster.TeleportCluster, cluster.KubeCluster)
		}
		table.AddRow([]string{cluster.TeleportCluster, cluster.KubeCluster, contextName})
	}
	fmt.Fprintln(cf.Stdout(), table.AsBuffer().String())
}

func (c *proxyKubeCommand) printTemplate(w io.Writer, isReexec bool, localProxy *kubeLocalProxy) error {
	if isReexec {
		return trace.Wrap(proxyKubeHeadlessTemplate.Execute(w, map[string]any{
			"multipleContexts": len(localProxy.kubeconfig.Contexts) > 1,
		}))
	}
	return trace.Wrap(proxyKubeTemplate.Execute(w, map[string]any{
		"addr":           localProxy.GetAddr(),
		"format":         c.format,
		"randomPort":     c.port == "",
		"kubeConfigPath": localProxy.KubeConfigPath(),
	}))
}

type kubeLocalProxy struct {
	tc             *client.TeleportClient
	clusters       kubeconfig.LocalProxyClusters
	kubeConfigPath string

	// kubeconfig is an ephemeral kube config that is written to the file
	kubeconfig *clientcmdapi.Config
	// clientKey is private key used for credentials of local proxy
	clientKey *keys.PrivateKey
	// localCAs is local CA generated based on clientKey and used for credentials of local proxy
	localCAs map[string]tls.Certificate
	// localProxy is the ALPN local proxy for sending TLS routing requests to
	// Teleport Proxy.
	localProxy *alpnproxy.LocalProxy
	// forwardProxy is a HTTPS forward proxy used as proxy-url for the
	// Kubernetes clients.
	forwardProxy *alpnproxy.ForwardProxy
	// certIssuer issues per-cluster certificates, reusing a single MFA ceremony across issuances.
	// It is shared between the initial cert load and the middleware cert reissuer
	// so the reusable MFA response and the old-auth-server fallback state carry over.
	certIssuer *kubeCertIssuer
}

func makeKubeLocalProxy(cf *CLIConf, tc *client.TeleportClient, clusters kubeconfig.LocalProxyClusters, originalKubeConfig *clientcmdapi.Config, port, overrideContext string) (*kubeLocalProxy, error) {
	certIssuer := newKubeCertIssuer(tc)
	certs, err := certIssuer.LoadOrIssueCerts(cf.Context, clusters)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO for best performance, avoid loading the entire profile. profile is
	// currently only used for keypaths.
	profile, err := tc.ProfileStatus()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Generate a new private key for the proxy. The client's existing private key may be
	// a hardware-backed private key, which cannot be added to the local proxy kube config.
	key, err := cryptosuites.GenerateKey(cf.Context, tc.GetCurrentSignatureAlgorithmSuite, cryptosuites.UserTLS)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	localClientKey, err := keys.NewPrivateKey(key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cas, err := alpnproxy.CreateKubeLocalCAs(localClientKey, clusters.TeleportClusters())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	lpListener, err := alpnproxy.NewKubeListener(cas)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	kubeProxy := &kubeLocalProxy{
		tc:         tc,
		clusters:   clusters,
		clientKey:  localClientKey,
		localCAs:   cas,
		certIssuer: certIssuer,
	}

	kubeMiddleware := alpnproxy.NewKubeMiddleware(alpnproxy.KubeMiddlewareConfig{
		Certs:        certs,
		CertReissuer: kubeProxy.getCertReissuer(),
		Headless:     cf.Headless,
		Logger:       logger,
		CloseContext: cf.Context,
		Relay:        tc.RelayAddr != "",
	})

	if tc.RelayAddr != "" {
		localProxy, err := alpnproxy.NewLocalProxy(
			alpnproxy.LocalProxyConfig{
				RemoteProxyAddr: tc.RelayAddr,
				// connectivity to a relay is secured by internal CAs, the
				// insecure option makes no sense
				InsecureSkipVerify: false,
				Listener:           lpListener,
				ParentContext:      cf.Context,
				// the relay does not support TLS terminators
				ALPNConnUpgradeRequired: false,
			},
			alpnproxy.WithHTTPMiddleware(kubeMiddleware),
			alpnproxy.WithClusterCAs(cf.Context, tc.RootClusterCACertPool),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		kubeProxy.localProxy = localProxy
	} else {
		localProxy, err := alpnproxy.NewLocalProxy(
			makeBasicLocalProxyConfig(cf.Context, tc, lpListener, cf.InsecureSkipVerify),
			alpnproxy.WithHTTPMiddleware(kubeMiddleware),
			alpnproxy.WithSNI(client.GetKubeTLSServerName(tc.WebProxyHost())),
			alpnproxy.WithClusterCAs(cf.Context, tc.RootClusterCACertPool),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		kubeProxy.localProxy = localProxy
	}

	kubeProxy.forwardProxy, err = alpnproxy.NewKubeForwardProxy(alpnproxy.KubeForwardProxyConfig{
		CloseContext: cf.Context,
		ListenPort:   port,
		ForwardAddr:  lpListener.Addr().String(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !cf.Headless {
		kubeProxy.kubeConfigPath = os.Getenv(proxyKubeConfigEnvVar)
		if kubeProxy.kubeConfigPath == "" {
			_, port, _ := net.SplitHostPort(kubeProxy.forwardProxy.GetAddr())
			kubeProxy.kubeConfigPath = filepath.Join(profile.KubeConfigPath(fmt.Sprintf("localproxy-%v", port)))
		}
	}

	kubeProxy.kubeconfig, err = kubeProxy.createKubeConfig(originalKubeConfig, overrideContext)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return kubeProxy, nil
}

// Start starts the local proxy in background goroutines and waits until
// context is done.
func (k *kubeLocalProxy) Start(ctx context.Context) error {
	errChan := make(chan error, 2)
	go func() {
		errChan <- k.forwardProxy.Start()
	}()
	go func() {
		errChan <- k.localProxy.Start(ctx)
	}()

	select {
	case err := <-errChan:
		return trace.Wrap(err)
	case <-ctx.Done():
		return nil
	}
}

// Close removes the temporary kubeconfig and closes the listeners.
func (k *kubeLocalProxy) Close() error {
	return trace.NewAggregate(
		k.forwardProxy.Close(),
		k.localProxy.Close(),
		utils.RemoveFileIfExist(k.KubeConfigPath()),
	)
}

// GetAddr returns the address of the forward proxy for client to connect.
func (k *kubeLocalProxy) GetAddr() string {
	return k.forwardProxy.GetAddr()
}

// KubeConfigPath returns the temporary kubeconfig path.
func (k *kubeLocalProxy) KubeConfigPath() string {
	return k.kubeConfigPath
}

// createKubeConfig creates local proxy settings for the ephemeral kubeconfig.
func (k *kubeLocalProxy) createKubeConfig(defaultConfig *clientcmdapi.Config, overrideContext string) (*clientcmdapi.Config, error) {
	if defaultConfig == nil {
		return nil, trace.BadParameter("empty default config")
	}
	values := &kubeconfig.LocalProxyValues{
		TeleportProfileName:     k.tc.WebProxyHost(),
		TeleportKubeClusterAddr: k.tc.KubeClusterAddr(),
		LocalProxyURL:           "http://" + k.GetAddr(),
		LocalProxyCAs:           map[string][]byte{},
		ClientKeyData:           k.clientKey.PrivateKeyPEM(),
		Clusters:                k.clusters,
		OverrideContext:         overrideContext,
	}
	for _, kubeCluster := range k.clusters {
		ca, ok := k.localCAs[kubeCluster.TeleportCluster]
		if !ok {
			return nil, trace.BadParameter("CA for teleport cluster %q is missing", kubeCluster.TeleportCluster)
		}

		x509Cert, err := utils.TLSCertLeaf(ca)
		if err != nil {
			return nil, trace.BadParameter("could not parse CA certificate for cluster %q", kubeCluster.TeleportCluster)
		}
		values.LocalProxyCAs[kubeCluster.TeleportCluster] = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: x509Cert.Raw})
	}

	cfg, err := kubeconfig.CreateLocalProxyConfig(defaultConfig, values)
	return cfg, trace.Wrap(err)
}

// WriteKubeConfig saves local proxy settings in the temporary kubeconfig.
func (k *kubeLocalProxy) WriteKubeConfig() error {
	if k.kubeconfig == nil {
		return trace.NotFound("kubeconfig is missing")
	}

	return trace.Wrap(kubeconfig.Save(k.KubeConfigPath(), *k.kubeconfig))
}

// getCertReissuer returns a function that reissues the user certificate for a Kubernetes cluster,
// used by the local proxy middleware when a cert it serves expires.
// The issuer performs a relogin if required.
func (k *kubeLocalProxy) getCertReissuer() func(ctx context.Context, teleportCluster, kubeCluster string) (tls.Certificate, error) {
	// mu serializes the reissues, which are not safe for concurrent use, and the ephemeral kubeconfig load and rewrite.
	var mu sync.Mutex

	return func(ctx context.Context, teleportCluster, kubeCluster string) (tls.Certificate, error) {
		mu.Lock()
		defer mu.Unlock()

		// We save user's current context in case there is a relogin, which
		// will delete our ephemeral kubeconfig and we'll need to recreate it.
		cfg, err := kubeconfig.Load(k.KubeConfigPath())
		if err != nil {
			return tls.Certificate{}, trace.Wrap(err, "could not load ephemeral kubeconfig at %q", k.KubeConfigPath())
		}
		currentContext := cfg.CurrentContext

		cert, err := k.certIssuer.IssueCert(ctx, teleportCluster, kubeCluster, nil /*mfaCheck*/)
		if err != nil {
			return tls.Certificate{}, trace.Wrap(err)
		}

		// We recreate ephemeral kubeconfig to make sure it's there even after relogin.
		k.kubeconfig.CurrentContext = currentContext
		if err := k.WriteKubeConfig(); err != nil {
			return tls.Certificate{}, trace.Wrap(err)
		}
		return *cert, nil
	}
}

// checkMultipleClusterSelections takes a map of name selectors to matched
// clusters and checks that each matching is valid.
func checkMultipleClusterSelections(cf *CLIConf, matchMap map[string]types.KubeClusters) error {
	for name, clusters := range matchMap {
		err := checkClusterSelection(cf, clusters, name)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// combineMatchedClusters combineMatchedClusters takes a map from name selector
// to matched clusters and combines all the matched clusters into a deduplicated
// slice.
func combineMatchedClusters(matchMap map[string]types.KubeClusters) types.KubeClusters {
	var out types.KubeClusters
	for _, clusters := range matchMap {
		out = append(out, clusters...)
	}
	return types.DeduplicateKubeClusters(out)
}

// matchClustersByNames maps each name to the clusters it matches by exact name
// or by discovered name.
func matchClustersByNames(clusters types.KubeClusters, names ...string) map[string]types.KubeClusters {
	matchesForNames := make(map[string]types.KubeClusters)
	for _, name := range names {
		matchesForNames[name] = matchClustersByNameOrDiscoveredName(name, clusters)
	}
	return matchesForNames
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
{{envVarCommand .format "KUBECONFIG" .kubeConfigPath}}
kubectl version

`))

// proxyKubeHeadlessTemplate is the message that gets printed to a user when a kube proxy is started with --headless.
var proxyKubeHeadlessTemplate = template.Must(template.New("").
	Parse(fmt.Sprintf(`Started local proxy for Kubernetes Access in the background.

%v Teleport will initiate a new shell configured with kubectl for local proxy access.
To conclude the session, simply use the "exit" command. Upon exiting, your original shell will be restored,
the local proxy will be closed, and future access through this headless session won't be possible.

{{ if .multipleContexts}} To work with different contexts use "kubectl --context", for example:
"kubectl --context='staging' get pods".
"kubectl --context='dev' get pods".
{{end}}
Try issuing a command, for example "kubectl version".
`, utils.Color(utils.Yellow, "Warning!"))))
