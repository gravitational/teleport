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
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/component-base/cli"
	"k8s.io/kubectl/pkg/cmd"
	"k8s.io/kubectl/pkg/cmd/plugin"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/gravitational/teleport/api/types"
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
func onKubectlCommand(cf *CLIConf, args []string) error {
	if os.Getenv(tshKubectlReexecEnvVar) == "" {
		err := runKubectlAndCollectRun(cf, args)
		return trace.Wrap(err)
	}

	runKubectlCode(args)
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
func runKubectlReexec(selfExec string, args []string, collector io.Writer) error {
	cmd := exec.Command(selfExec, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = io.MultiWriter(os.Stderr, collector)
	cmd.Env = append(os.Environ(), fmt.Sprintf("%s=yes", tshKubectlReexecEnvVar))
	return trace.Wrap(cmd.Run())
}

// runKubectlCode runs the actual kubectl package code with the default options.
// This code is only executed when `tshKubectlReexec` env is present. This happens
// because we need to retry kubectl calls and `kubectl` calls os.Exit in multiple
// paths.
func runKubectlCode(args []string) {
	// These values are the defaults used by kubectl and can be found here:
	// https://github.com/kubernetes/kubectl/blob/3612c18ed86fc0a2f4467ca355b3e21569fabe0a/pkg/cmd/cmd.go#L94
	defaultConfigFlags := genericclioptions.NewConfigFlags(true).
		WithDeprecatedPasswordFlag().
		WithDiscoveryBurst(300).
		WithDiscoveryQPS(50.0)

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
	// override args without kubectl to avoid errors.
	command.SetArgs(args[1:])
	// run command until it finishes.
	if err := cli.RunNoErrOutput(command); err != nil {
		// Pretty-print the error and exit with an error.
		cmdutil.CheckErr(err)
	}
	os.Exit(0)
}

func runKubectlAndCollectRun(cf *CLIConf, args []string) error {
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

		err := runKubectlReexec(cf.executablePath, args, writer)
		writer.CloseWithError(io.EOF)

		if scanErr := group.Wait(); scanErr != nil {
			log.WithError(scanErr).Warn("unable to scan stderr payload")
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

// extractKubeConfigAndContext parses the args and extracts - if present -
// the --kubeconfig flag that overrrides the default kubeconfig location
// and the --context flag that overrides the default context to use.
func extractKubeConfigAndContext(args []string) (kubeconfig string, context string) {
	if len(args) <= 2 {
		return
	}
	command := cmd.NewDefaultKubectlCommandWithArgs(
		cmd.KubectlOptions{
			Arguments: args[2:],
		},
	)

	if err := command.ParseFlags(args[2:]); err != nil {
		return
	}

	kubeconfig = command.Flag("kubeconfig").Value.String()
	context = command.Flag("context").Value.String()

	return
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
	kubeName := kubeconfig.KubeClusterFromContext(selectedContext, teleportClusterName)
	if kubeName == "" {
		return "", trace.BadParameter("selected context %q does not belong to Teleport cluster %q", selectedContext, teleportClusterName)
	}
	return kubeName, nil
}
