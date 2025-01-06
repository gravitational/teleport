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
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/gravitational/trace"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/component-base/cli"
	"k8s.io/kubectl/pkg/cmd"
	"k8s.io/kubectl/pkg/cmd/plugin"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/gravitational/teleport"
	tracehttp "github.com/gravitational/teleport/api/observability/tracing/http"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
)

var (
	podForbiddenRe   = regexp.MustCompile(`(?m)Error from server \(Forbidden\): pods "(.*)" is forbidden: User ".*" cannot get resource "pods" in API group "" in the namespace "(.*)"`)
	clusterForbidden = "[00] access denied"
	// clusterObjectDiscoveryFailed is printed when kubectl tries to do API discovery
	// - calling /apis endpoint - but Teleport denies the request. Since it cannot
	// discover the resources available in the cluster, it prints this message saying
	// that the cluster does not have pod(s). Since every Kubernetes cluster supports
	// pods, it's safe to create a resource access request.
	clusterObjectDiscoveryFailed = regexp.MustCompile(`(?m)the server doesn't have a resource type "pods?"`)
)

// resourceKind identifies a Kubernetes resource.
type resourceKind struct {
	kind            string
	subResourceName string
}

// onKubectlCommand re-execs itself if env var `tshKubectlRexec` is not set
// in order to execute the `kubectl` portion of the code. This is a requirement because
// `kubectl` calls `os.Exit()` in every code path, and we need to intercept the
// exit code to validate if the request was denied.
// When executing `tsh kubectl get pods`, tsh checks if `tshKubectlReexec`. Since
// it's the user call and the flag is not present, tsh reexecs the same exact
// the user executed and uses an io.MultiWriter to write the os.Stderr output
// from the kubectl command into an io.Pipe for analysis. It also sets the env
// `tshKubectlReexec` in the exec.Cmd.Env and runs the command. When running the
// command, `tsh` will be recalled, and since `tshKubectlReexec` is set only the
// kubectl portion of code is executed.
// On the caller side, once the callee execution finishes, tsh inspects the stderr
// outputs and decides if creating an access request is appropriate.
// If the access request is created, tsh waits for the approval and runs the expected
// command again.
func onKubectlCommand(cf *CLIConf, fullArgs []string, args []string) error {
	if os.Getenv(tshKubectlReexecEnvVar) == "" {
		err := runKubectlAndCollectRun(cf, fullArgs, args)
		return trace.Wrap(err)
	}
	runKubectlCode(cf, args)
	return nil
}

const (
	// tshKubectlReexecEnvVar is the name of the environment variable used to control if
	// tsh should re-exec or execute a kubectl command.
	tshKubectlReexecEnvVar = "TSH_KUBE_REEXEC"
)

// runKubectlReexec reexecs itself and copies the `stderr` output into
// the provided collector.
// It also sets tshKubectlReexec for the command to prevent
// an exec loop
func runKubectlReexec(cf *CLIConf, fullArgs, args []string, collector io.Writer) error {
	closeFn, newKubeConfigLocation, err := maybeStartKubeLocalProxy(cf, withKubectlArgs(args))
	if err != nil {
		return trace.Wrap(err)
	}
	defer closeFn()

	cmdEnv := append(os.Environ(), fmt.Sprintf("%s=yes", tshKubectlReexecEnvVar))

	// Update kubeconfig location.
	if newKubeConfigLocation != "" {
		cmdEnv = overwriteKubeconfigInEnv(cmdEnv, newKubeConfigLocation)
		fullArgs = overwriteKubeconfigFlagInArgs(fullArgs, newKubeConfigLocation)
	}

	// Execute.
	cmd := exec.Command(cf.executablePath, fullArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = io.MultiWriter(os.Stderr, collector)
	cmd.Env = cmdEnv
	return trace.Wrap(cmd.Run())
}

// wrapConfigFn wraps the rest.Config with a custom RoundTripper if the user
// wants to sample traces.
func wrapConfigFn(cf *CLIConf) func(c *rest.Config) *rest.Config {
	return func(c *rest.Config) *rest.Config {
		c.Wrap(
			func(rt http.RoundTripper) http.RoundTripper {
				if cf.SampleTraces {
					// If the user wants to sample traces, wrap the transport with a trace
					// transport.
					return tracehttp.NewTransport(rt)
				}
				return rt
			},
		)
		return c
	}
}

// runKubectlCode runs the actual kubectl package code with the default options.
// This code is only executed when `tshKubectlReexec` env is present. This happens
// because we need to retry kubectl calls and `kubectl` calls os.Exit in multiple
// paths.
func runKubectlCode(cf *CLIConf, args []string) {
	closeTracer := initializeTracing(cf)
	// If the user opted to not sample traces, cf.TracingProvider is pre-initialized
	// with a noop provider.
	ctx, span := cf.TracingProvider.Tracer("kubectl").Start(cf.Context, "kubectl")
	closeSpanAndTracer := func() {
		span.End()
		closeTracer()
	}
	// These values are the defaults used by kubectl and can be found here:
	// https://github.com/kubernetes/kubectl/blob/3612c18ed86fc0a2f4467ca355b3e21569fabe0a/pkg/cmd/cmd.go#L94
	defaultConfigFlags := genericclioptions.NewConfigFlags(true).
		WithDeprecatedPasswordFlag().
		WithDiscoveryBurst(300).
		WithDiscoveryQPS(50.0).
		WithWrapConfigFn(wrapConfigFn(cf))

	command := cmd.NewDefaultKubectlCommandWithArgs(
		cmd.KubectlOptions{
			// init the default plugin handler.
			PluginHandler: cmd.NewDefaultPluginHandler(plugin.ValidPluginFilenamePrefixes),
			Arguments:     args,
			ConfigFlags:   defaultConfigFlags,
			// init the IOSStreams.
			IOStreams: genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
		},
	)
	command.SetContext(ctx)
	// override args without kubectl to avoid errors.
	command.SetArgs(args[1:])

	// run command until it finishes.
	if err := cli.RunNoErrOutput(command); err != nil {
		closeSpanAndTracer()
		// Pretty-print the error and exit with an error.
		cmdutil.CheckErr(err)
	}

	closeSpanAndTracer()
	os.Exit(0)
}

func runKubectlAndCollectRun(cf *CLIConf, fullArgs, args []string) error {
	var (
		alreadyRequestedAccess bool
		err                    error
		exitErr                *exec.ExitError
	)
	for {
		// missingKubeResources will include the Kubernetes Resources whose access
		// was rejected in this kubectl call.
		missingKubeResources := make([]resourceKind, 0, 50)
		reader, writer := io.Pipe()
		group, _ := errgroup.WithContext(cf.Context)
		group.Go(
			func() error {
				// This goroutine scans each line of output emitted to stderr by kubectl
				// and parses it in order to check if the returned error was a problem with
				// missing access level. If it's the case, tsh kubectl will create automatically
				// the access request for the user to access the resource.
				// Current supported resources:
				// - pod
				// - kube_cluster

				scanner := bufio.NewScanner(reader)
				scanner.Split(bufio.ScanLines)
				for scanner.Scan() {
					line := scanner.Text()

					// Check if the request targeting a pod endpoint was denied due to
					// Teleport Pod RBAC or if the operation was denied by Kubernetes RBAC.
					// In the second case, we should create a Resource Access Request to allow
					// the user to exec/read logs using different Kubernetes RBAC principals.
					// using different Kubernetes RBAC principals.
					if podForbiddenRe.MatchString(line) {
						results := podForbiddenRe.FindStringSubmatch(line)
						missingKubeResources = append(missingKubeResources, resourceKind{kind: types.KindKubePod, subResourceName: filepath.Join(results[2], results[1])})
						// Check if cluster access was denied. If denied we should create
						// a Resource Access Request for the cluster and not a pod.
					} else if strings.Contains(line, clusterForbidden) || clusterObjectDiscoveryFailed.MatchString(line) {
						missingKubeResources = append(missingKubeResources, resourceKind{kind: types.KindKubernetesCluster})
					}
				}
				return trace.Wrap(scanner.Err())
			},
		)

		err := runKubectlReexec(cf, fullArgs, args, writer)
		writer.CloseWithError(io.EOF)

		if scanErr := group.Wait(); scanErr != nil {
			logger.WarnContext(cf.Context, "unable to scan stderr payload", "error", scanErr)
		}

		if err == nil {
			break
		} else if !errors.As(err, &exitErr) {
			return trace.Wrap(err)
		} else if errors.As(err, &exitErr) && exitErr.ExitCode() != cmdutil.DefaultErrorExitCode {
			// if the exit code is not 1, it was emitted by pod exec code and we should
			// ignore it since the user was allowed to execute the command in the pod.
			break
		}

		if len(missingKubeResources) > 0 && !alreadyRequestedAccess {
			// create the access requests for the user and wait for approval.
			if err := createKubeAccessRequest(cf, missingKubeResources, args); err != nil {
				return trace.Wrap(err)
			}
			alreadyRequestedAccess = true
			continue
		}
		break
	}
	// exit with the kubectl exit code to keep compatibility.
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.ExitCode())
	}
	return nil
}

// createKubeAccessRequest creates an access request to the denied resources
// if the user's roles allow search_as_role.
func createKubeAccessRequest(cf *CLIConf, resources []resourceKind, args []string) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	kubeName, err := getKubeClusterName(args, tc.SiteName)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, rec := range resources {
		cf.RequestedResourceIDs = append(
			cf.RequestedResourceIDs,
			filepath.Join("/", tc.SiteName, rec.kind, kubeName, rec.subResourceName),
		)
	}
	cf.Reason = fmt.Sprintf("Resource request automatically created for %v", args)
	if err := executeAccessRequest(cf, tc); err != nil {
		// TODO(tigrato): intercept the error to validate the origin
		return trace.Wrap(err)
	}
	return nil
}

// extractKubeConfigAndContext parses the args and extracts:
// - the "--context" flag that overrides the default context to use, if present
// - the "--kubeconfig" flag that overrides the default kubeconfig location, if
// present
func extractKubeConfigAndContext(args []string) (string, string) {
	if len(args) <= 2 {
		return "", ""
	}

	command := makeKubectlCobraCommand()
	return extractKubeConfigAndContextFromCommand(command, args)
}

// extractKubeConfigAndContextFromCommand parses the args using provided
// kubectl command and extracts:
// - the "--context" flag that overrides the default context to use, if present
// - the "--kubeconfig" flag that overrides the default kubeconfig location, if
// present
func extractKubeConfigAndContextFromCommand(command *cobra.Command, args []string) (kubeconfig string, context string) {
	if len(args) <= 2 {
		return
	}

	// Find subcommand.
	if subcommand, _, err := command.Find(args[1:]); err == nil {
		command = subcommand
	}

	// Ignore errors from ParseFlags.
	command.ParseFlags(args[1:])

	kubeconfig = command.Flag("kubeconfig").Value.String()
	context = command.Flag("context").Value.String()
	return
}

var makeKubectlCobraCommandLock sync.Mutex

// makeKubectlCobraCommand creates a cobra.Command for kubectl.
//
// Note that cmd.NewKubectlCommand is slow (15+ ms, 20k+ alloc), so avoid
// making/re-making it when possible.
//
// Also cmd.NewKubectlCommand is not goroutine-safe, thus using a lock.
func makeKubectlCobraCommand() *cobra.Command {
	makeKubectlCobraCommandLock.Lock()
	defer makeKubectlCobraCommandLock.Unlock()

	return cmd.NewKubectlCommand(cmd.KubectlOptions{
		// Use NewConfigFlags to avoid load existing values from
		// defaultConfigFlags.
		ConfigFlags: genericclioptions.NewConfigFlags(true),
	})
}

// getKubeClusterName extracts the Kubernetes Cluster name if the Kube belongs to
// the teleportClusterName cluster. It parses the args to extract the `--kubeconfig`
// and `--context` flag values and to use them if any was overriten.
func getKubeClusterName(args []string, teleportClusterName string) (string, error) {
	kubeconfigLocation, selectedContext := extractKubeConfigAndContext(args)
	if selectedContext == "" {
		kubeName, err := kubeconfig.SelectedKubeCluster(kubeconfigLocation, teleportClusterName)
		return kubeName, trace.Wrap(err)
	}
	kc, err := kubeconfig.Load(kubeconfigLocation)
	if err != nil {
		return "", trace.Wrap(err)
	}
	kubeName := kubeconfig.KubeClusterFromContext(selectedContext, kc.Contexts[selectedContext], teleportClusterName)
	if kubeName == "" {
		return "", trace.BadParameter("selected context %q does not belong to Teleport cluster %q", selectedContext, teleportClusterName)
	}
	return kubeName, nil
}

type kubeLocalProxyOpts struct {
	// kubectlArgs is a list of command arguments passed in for `tsh kubectl`.
	// used to decide if local proxy is required.
	kubectlArgs []string
	// makeAndStartKubeLocalProxyFunc is a callback function to create and
	// start a kube local proxy, when it is decided that a local proxy is
	// required. Default to makeAndStartKubeLocalProxy. Can be set another
	// function for testing.
	makeAndStartKubeLocalProxyFunc func(*CLIConf, *clientcmdapi.Config, kubeconfig.LocalProxyClusters) (func(), string, error)
}

type applyKubeLocalProxyOpts func(o *kubeLocalProxyOpts)

func withKubectlArgs(args []string) applyKubeLocalProxyOpts {
	return func(o *kubeLocalProxyOpts) {
		o.kubectlArgs = args
	}
}

func newKubeLocalProxyOpts(applyOpts ...applyKubeLocalProxyOpts) kubeLocalProxyOpts {
	opts := kubeLocalProxyOpts{
		makeAndStartKubeLocalProxyFunc: makeAndStartKubeLocalProxy,
	}
	for _, applyOpt := range applyOpts {
		applyOpt(&opts)
	}
	return opts
}

// maybeStartKubeLocalProxy starts a kube local proxy if local proxy is
// required. A closeFn and the new kubeconfig path are returned if local proxy
// is successfully created. Called by `tsh kubectl` and `tsh kube exec`.
func maybeStartKubeLocalProxy(cf *CLIConf, applyOpts ...applyKubeLocalProxyOpts) (func(), string, error) {
	opts := newKubeLocalProxyOpts(applyOpts...)

	config, clusters, useLocalProxy := shouldUseKubeLocalProxy(cf, opts.kubectlArgs)
	if !useLocalProxy {
		return func() {}, "", nil
	}

	closeFn, newKubeConfigLocation, err := opts.makeAndStartKubeLocalProxyFunc(cf, config, clusters)
	return closeFn, newKubeConfigLocation, trace.Wrap(err)
}

// makeAndStartKubeLocalProxy is a helper to create a kube local proxy and
// start it in a goroutine. If successful, a closeFn and the generated
// kubeconfig location are returned.
func makeAndStartKubeLocalProxy(cf *CLIConf, config *clientcmdapi.Config, clusters kubeconfig.LocalProxyClusters) (func(), string, error) {
	tc, err := makeClient(cf)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	localProxy, err := makeKubeLocalProxy(cf, tc, clusters, config, cf.LocalProxyPort, "")
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	if err := localProxy.WriteKubeConfig(); err != nil {
		return nil, "", trace.Wrap(err)
	}

	go localProxy.Start(cf.Context)

	closeFn := func() {
		localProxy.Close()
	}
	return closeFn, localProxy.KubeConfigPath(), nil
}

// shouldUseKubeLocalProxy checks if a local proxy is required for kube
// access for `tsh kubectl` or `tsh kube exec`.
//
// The local proxy is required when all of these conditions are met:
// - profile is loadable
// - kube access is enabled, and is accessed through web proxy address
// - ALPN connection upgrade is required (e.g. Proxy behind ALB)
// - not `kubectl config` commands
// - original/default kubeconfig is loadable
// - Selected cluster is a Teleport cluster that uses KubeClusterAddr
func shouldUseKubeLocalProxy(cf *CLIConf, kubectlArgs []string) (*clientcmdapi.Config, kubeconfig.LocalProxyClusters, bool) {
	// When failed to load profile, assume this CLI command is not running
	// against Teleport clusters.
	profile, err := cf.GetProfile()
	if err != nil {
		return nil, nil, false
	}

	if !profile.RequireKubeLocalProxy() {
		return nil, nil, false
	}

	// Skip "kubectl config" commands.
	var kubeconfigLocation, selectedContext string
	if len(kubectlArgs) > 0 {
		kubectlCommand := makeKubectlCobraCommand()
		if isKubectlConfigCommand(kubectlCommand, kubectlArgs) {
			return nil, nil, false
		}

		kubeconfigLocation, selectedContext = extractKubeConfigAndContextFromCommand(kubectlCommand, kubectlArgs)
	}

	// Nothing to do if cannot load original kubeconfig.
	defaultConfig, err := kubeconfig.Load(kubeconfigLocation)
	if err != nil {
		return nil, nil, false
	}

	// Prepare Teleport kube cluster based on selected context.
	kubeCluster, found := kubeconfig.FindTeleportClusterForLocalProxy(defaultConfig, kubeClusterAddrFromProfile(profile), selectedContext)
	if !found {
		return nil, nil, false
	}
	return defaultConfig, kubeconfig.LocalProxyClusters{kubeCluster}, true
}

func isKubectlConfigCommand(kubectlCommand *cobra.Command, args []string) bool {
	if len(args) < 2 || args[0] != "kubectl" {
		return false
	}

	find, _, _ := kubectlCommand.Find(args[1:])
	for ; find != nil; find = find.Parent() {
		if find.Name() == "config" {
			return true
		}
	}
	return false
}

func kubeClusterAddrFromProfile(profile *profile.Profile) string {
	partialClientConfig := client.Config{
		WebProxyAddr:      profile.WebProxyAddr,
		KubeProxyAddr:     profile.KubeProxyAddr,
		TLSRoutingEnabled: profile.TLSRoutingEnabled,
	}
	return partialClientConfig.KubeClusterAddr()
}

func overwriteKubeconfigFlagInArgs(args []string, newPath string) []string {
	// Make a clone to avoid changing the original args.
	args = slices.Clone(args)
	for i, arg := range args {
		switch {
		case strings.HasPrefix(arg, "--kubeconfig="):
			args[i] = fmt.Sprintf("--kubeconfig=%v", newPath)
		case arg == "--kubeconfig" && len(args) > i+1:
			args[i+1] = newPath
		}
	}
	return args
}

func overwriteKubeconfigInEnv(env []string, newPath string) (output []string) {
	kubeConfigEnvPrefix := teleport.EnvKubeConfig + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, kubeConfigEnvPrefix) {
			continue
		}
		output = append(output, entry)
	}
	output = append(output, kubeConfigEnvPrefix+newPath)
	return
}
