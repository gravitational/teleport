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
	"sync"

	"github.com/gravitational/trace"
	"github.com/spf13/pflag"
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
)

// resourceKind identifies which resource the access was denied.
type resourceKind struct {
	kind            string
	subResourceName string
}

// onKubectlCommand re-execs itself if env var `tshKubectlRexec` is not set
// in order to execute the `kubectl` portion of the code. This is a requirement because
// `kubectl` calls `os.Exit(.)` in every code path and we need to intercept them
// to validate if the request was denied.
// When executing `tsh kubectl get pods`, tsh checks if `tshKubectlRexec`. Since
// it's the user call and the flag is not present, tsh reexecs the same exact
// the user executed and uses an io.MultiWritter to write the os.Stderr output
// from the kubectl command into an io.Pipe for analysis. It also sets the env
// `tshKubectlRexec` in the exec.Cmd.Env and runs the command. When running the
// command, `tsh` will be recalled, and since `tshKubectlRexec` is set only the
// kubectl portion of code is executed.
// On the caller side, once the callee execution finishes, tsh inspects the sterr
// outputs and decides if creating an access request is appropriate.
// If the access request is created, tsh waits for the approval and runs the expected
// command again.
func onKubectlCommand(cf *CLIConf, args []string) error {
	// if
	if len(os.Getenv(tshKubectlRexec)) > 0 {
		runKubectlCode(args)
		return nil
	}
	err := runKubectlAndCollectRun(cf, args)

	return trace.Wrap(err)
}

const (
	// tshKubectlRexec is used to controll if the tsh call is the re-exec one
	// of the one issued by the user.
	tshKubectlRexec = "tsh_kube_reexec"
)

// runKubectlReexec reexecs itself and collects the `strderr` outputs also into
// the provided collector.
// It also forces the tshKubectlRexec for the reexec command so a exec loop
// does not exist.
func runKubectlReexec(selfExec string, args []string, collector io.Writer) error {
	cmd := exec.Command(selfExec, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = io.MultiWriter(os.Stderr, collector)
	cmd.Env = append(os.Environ(), fmt.Sprintf("%s=yes", tshKubectlRexec))
	return cmd.Run()
}

// runKubectlCode runs the actual kubectl package code with the default options.
// This code is only executed when `tsh_kube_reexec` env is present. This happens
// because we need to retry kubectl calls and `kubectl` calls os.Exit in multiple
// paths.
func runKubectlCode(args []string) {
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
		resources := make([]resourceKind, 50)
		reader, writer := io.Pipe()
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			// This goroutine scans each line of log emmited to stderr by kubectl
			// and parses it in order to check if the returned error was a problem with
			// missing access level. If it's the case, tsh kubectl will create automatically
			// the access request for the user to access the resource.
			// Current supported resources:
			// - pod
			// - kube_cluster
			defer wg.Done()

			scanner := bufio.NewScanner(reader)
			scanner.Split(bufio.ScanLines)
			for scanner.Scan() {
				line := scanner.Text()
				// check if the request targeting a pod endpoint was denied due to the
				// Pod RBAC logic of in case of Teleport allowing the resource, check if
				// the operation was denied by Kubernetes RBAC. In the second case, we should
				// also create a Resource Access Request to allow the user to exec/read logs
				// using different Kubernetes RBAC principals.
				if podForbiddenRe.MatchString(line) {
					results := podForbiddenRe.FindStringSubmatch(line)
					resources = append(resources, resourceKind{kind: types.KindKubePod, subResourceName: filepath.Join(results[2], results[1])})
					// check if cluster access was dennied. If denied we should create
					// an resource access request to the cluster and not a pod.
				} else if strings.Contains(line, clusterForbidden) {
					resources = append(resources, resourceKind{kind: types.KindKubernetesCluster})
				}
			}
		}()

		err = runKubectlReexec(cf.executablePath, args, writer)
		if err == nil {
			break
		} else if !errors.Is(err, exitErr) {
			return trace.Wrap(err)
		} else if errors.As(err, &exitErr) && exitErr.ExitCode() != cmdutil.DefaultErrorExitCode {
			// if the exit code is not 1, it was emmited by pod exec code and we should
			// ignore it since the user was allowed to execute the command in the pod.
			break
		}
		writer.CloseWithError(io.EOF)
		wg.Wait()

		if len(resources) > 0 && !alreadyRequestedAccess {
			// create the access requests for the user and wait for approval.
			if err := createKubeAccessRequest(cf, resources, args); err != nil {
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

// createKubeAccessRequest creates an access request to the dennied resources
// if the user's roles allow search_as_role.
func createKubeAccessRequest(cf *CLIConf, resources []resourceKind, args []string) error {
	tc, err := makeClient(cf, true)
	if err != nil {
		return trace.Wrap(err)
	}
	proxyClient, err := tc.ConnectToProxy(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}
	kubeName, err := getKubeClusterName(args, proxyClient.ClusterName())
	if err != nil {
		return trace.Wrap(err)
	}
	for _, rec := range resources {
		cf.RequestedResourceIDs = append(
			cf.RequestedResourceIDs,
			filepath.Join("/", proxyClient.ClusterName(), rec.kind, kubeName, rec.subResourceName),
		)
	}
	cf.Reason = fmt.Sprintf("Resource request automatically created on %v", args)
	if err := executeAccessRequest(cf, tc); err != nil {
		// TODO: intercept the error to validate the origin
		return trace.Wrap(err)
	}
	return nil
}

// extractKubeConfigAndContext parses the args and extracts - if present -
// the --kubeconfig flag that overrrides the default kubeconfig location
// and the --context flag that overrides the default context to use.
func extractKubeConfigAndContext(args []string) (kubeconfig string, context string) {
	// ignore any error on parse since we are only interested in two flag values.
	fs := pflag.NewFlagSet("kubectl", pflag.ContinueOnError)
	fs.StringVar(&kubeconfig, "kubeconfig", "", "")
	fs.StringVar(&context, "context", "", "")
	// ignore returned error.
	_ = fs.Parse(args)
	return
}

// getKubeClusterName extracts the Kubernetes Cluster name if the Kube belongs to
// the teleportClusterName cluster. It parses the args to extract the `--kubeconfig`
// and `--context` flag values and to use them if any was overriten.
func getKubeClusterName(args []string, teleportClusterName string) (string, error) {
	kubeconfigLocation, selectedContext := extractKubeConfigAndContext(args)
	if len(selectedContext) == 0 {
		kubeName, err := kubeconfig.SelectedKubeCluster(kubeconfigLocation, teleportClusterName)
		return kubeName, trace.Wrap(err)
	}
	kubeName := kubeconfig.KubeClusterFromContext(selectedContext, teleportClusterName)
	if len(kubeName) == 0 {
		return "", trace.BadParameter("selected context %q does not belong to Teleport cluster %q", selectedContext, teleportClusterName)
	}
	return kubeName, nil
}
