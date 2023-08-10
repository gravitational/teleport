/*
Copyright 2020-2021 Gravitational, Inc.

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
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"
	dockerterm "github.com/moby/term"
	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/pkg/apis/clientauthentication"
	clientauthv1beta1 "k8s.io/client-go/pkg/apis/clientauthentication/v1beta1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/cmd/util/podcmd"
	"k8s.io/kubectl/pkg/polymorphichelpers"
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/kubectl/pkg/util/term"

	"github.com/gravitational/teleport"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	kubeutils "github.com/gravitational/teleport/lib/kube/utils"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/common"
)

type kubeCommands struct {
	credentials *kubeCredentialsCommand
	ls          *kubeLSCommand
	login       *kubeLoginCommand
	sessions    *kubeSessionsCommand
	exec        *kubeExecCommand
	join        *kubeJoinCommand
}

func newKubeCommand(app *kingpin.Application) kubeCommands {
	kube := app.Command("kube", "Manage available Kubernetes clusters")
	cmds := kubeCommands{
		credentials: newKubeCredentialsCommand(kube),
		ls:          newKubeLSCommand(kube),
		login:       newKubeLoginCommand(kube),
		sessions:    newKubeSessionsCommand(kube),
		exec:        newKubeExecCommand(kube),
		join:        newKubeJoinCommand(kube),
	}
	return cmds
}

type kubeJoinCommand struct {
	*kingpin.CmdClause
	session  string
	mode     string
	siteName string
}

func newKubeJoinCommand(parent *kingpin.CmdClause) *kubeJoinCommand {
	c := &kubeJoinCommand{
		CmdClause: parent.Command("join", "Join an active Kubernetes session."),
	}

	c.Flag("mode", "Mode of joining the session, valid modes are observer, moderator and peer.").Short('m').Default("observer").EnumVar(&c.mode, "observer", "moderator", "peer")
	c.Flag("cluster", clusterHelp).Short('c').StringVar(&c.siteName)
	c.Arg("session", "The ID of the target session.").Required().StringVar(&c.session)
	return c
}

func (c *kubeJoinCommand) getSessionMeta(ctx context.Context, tc *client.TeleportClient) (types.SessionTracker, error) {
	proxy, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	site := proxy.CurrentCluster()

	return site.GetSessionTracker(ctx, c.session)
}

func (c *kubeJoinCommand) run(cf *CLIConf) error {
	if err := validateParticipantMode(types.SessionParticipantMode(c.mode)); err != nil {
		return trace.Wrap(err)
	}

	cf.SiteName = c.siteName
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	meta, err := c.getSessionMeta(cf.Context, tc)
	if trace.IsNotFound(err) {
		return trace.NotFound("Failed to find session %q. The ID may be incorrect.", c.session)
	} else if err != nil {
		return trace.Wrap(err)
	}

	cluster := meta.GetClusterName()
	kubeCluster := meta.GetKubeCluster()
	var k *client.Key

	// Try loading existing keys.
	k, err = tc.LocalAgent().GetKey(cluster, client.WithKubeCerts{})
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	// Loaded existing credentials and have a cert for this cluster? Return it
	// right away.
	if err == nil {
		crt, err := k.KubeX509Cert(kubeCluster)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		if crt != nil && time.Until(crt.NotAfter) > time.Minute {
			log.Debugf("Re-using existing TLS cert for kubernetes cluster %q", kubeCluster)
		} else {
			err = client.RetryWithRelogin(cf.Context, tc, func() error {
				var err error
				k, err = tc.IssueUserCertsWithMFA(cf.Context, client.ReissueParams{
					RouteToCluster:    cluster,
					KubernetesCluster: kubeCluster,
				})

				return trace.Wrap(err)
			})

			if err != nil {
				return trace.Wrap(err)
			}

			// Cache the new cert on disk for reuse.
			if err := tc.LocalAgent().AddKubeKey(k); err != nil {
				return trace.Wrap(err)
			}
		}
		// Otherwise, cert for this k8s cluster is missing or expired. Request
		// a new one.
	}

	if _, err := tc.Ping(cf.Context); err != nil {
		return trace.Wrap(err)
	}

	if tc.KubeProxyAddr == "" {
		// Kubernetes support disabled, don't touch kubeconfig.
		return trace.AccessDenied("this cluster does not support Kubernetes")
	}

	kubeStatus, err := fetchKubeStatus(cf.Context, tc)
	if err != nil {
		return trace.Wrap(err)
	}

	ciphers := utils.DefaultCipherSuites()
	tlsConfig, err := k.KubeClientTLSConfig(ciphers, kubeCluster)
	if err != nil {
		return trace.Wrap(err)
	}

	tlsConfig.InsecureSkipVerify = cf.InsecureSkipVerify
	session, err := client.NewKubeSession(cf.Context, tc, meta, tc.KubeProxyAddr, kubeStatus.tlsServerName, types.SessionParticipantMode(c.mode), tlsConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	session.Wait()
	return trace.Wrap(session.Detach())
}

// RemoteExecutor defines the interface accepted by the Exec command - provided for test stubbing
type RemoteExecutor interface {
	Execute(ctx context.Context, method string, url *url.URL, config *restclient.Config, stdin io.Reader, stdout, stderr io.Writer, tty bool, terminalSizeQueue remotecommand.TerminalSizeQueue) error
}

// DefaultRemoteExecutor is the standard implementation of remote command execution
type DefaultRemoteExecutor struct{}

func (*DefaultRemoteExecutor) Execute(ctx context.Context, method string, url *url.URL, config *restclient.Config, stdin io.Reader, stdout, stderr io.Writer, tty bool, terminalSizeQueue remotecommand.TerminalSizeQueue) error {
	exec, err := remotecommand.NewSPDYExecutor(config, method, url)
	if err != nil {
		return err
	}
	return exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:             stdin,
		Stdout:            stdout,
		Stderr:            stderr,
		Tty:               tty,
		TerminalSizeQueue: terminalSizeQueue,
	})
}

type StreamOptions struct {
	Namespace     string
	PodName       string
	ContainerName string
	Stdin         bool
	TTY           bool
	// minimize unnecessary output
	Quiet bool

	genericclioptions.IOStreams

	overrideStreams func() (io.ReadCloser, io.Writer, io.Writer)
	isTerminalIn    func(t term.TTY) bool
}

func (o *StreamOptions) SetupTTY() term.TTY {
	t := term.TTY{
		Out: o.Out,
	}

	if !o.Stdin {
		// need to nil out o.In to make sure we don't create a stream for stdin
		o.In = nil
		o.TTY = false
		return t
	}

	t.In = o.In
	if !o.TTY {
		return t
	}

	if o.isTerminalIn == nil {
		o.isTerminalIn = func(tty term.TTY) bool {
			return tty.IsTerminalIn()
		}
	}
	if !o.isTerminalIn(t) {
		o.TTY = false

		if !o.Quiet && o.ErrOut != nil {
			fmt.Fprintln(o.ErrOut, "Unable to use a TTY - input is not a terminal or the right kind of file")
		}

		return t
	}

	// if we get to here, the user wants to attach stdin, wants a TTY, and o.In is a terminal, so we
	// can safely set t.Raw to true
	t.Raw = true

	if o.overrideStreams == nil {
		// use dockerterm.StdStreams() to get the right I/O handles on Windows
		o.overrideStreams = dockerterm.StdStreams
	}

	stdin, stdout, _ := o.overrideStreams()
	o.In = stdin
	t.In = stdin
	if o.Out != nil {
		o.Out = stdout
		t.Out = stdout
	}

	return t
}

type ExecOptions struct {
	StreamOptions
	resource.FilenameOptions

	ResourceName     string
	Command          []string
	EnforceNamespace bool

	Builder          func() *resource.Builder
	ExecutablePodFn  polymorphichelpers.AttachablePodForObjectFunc
	restClientGetter genericclioptions.RESTClientGetter

	Pod                            *corev1.Pod
	Executor                       RemoteExecutor
	PodClient                      coreclient.PodsGetter
	GetPodTimeout                  time.Duration
	Config                         *restclient.Config
	displayParticipantRequirements bool
	// invited is a list of users that are invited to the session
	invited []string
	// reason is the reason for the session
	reason string
}

// Run executes a validated remote execution against a pod.
func (p *ExecOptions) Run(ctx context.Context) error {
	var err error
	if len(p.PodName) != 0 {
		p.Pod, err = p.PodClient.Pods(p.Namespace).Get(ctx, p.PodName, metav1.GetOptions{})
		if err != nil {
			return err
		}
	} else {
		builder := p.Builder().
			WithScheme(scheme.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
			FilenameParam(p.EnforceNamespace, &p.FilenameOptions).
			NamespaceParam(p.Namespace).DefaultNamespace()
		if len(p.ResourceName) > 0 {
			builder = builder.ResourceNames("pods", p.ResourceName)
		}

		obj, err := builder.Do().Object()
		if err != nil {
			return err
		}

		p.Pod, err = p.ExecutablePodFn(p.restClientGetter, obj, p.GetPodTimeout)
		if err != nil {
			return err
		}
	}

	pod := p.Pod

	if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
		return fmt.Errorf("cannot exec into a container in a completed pod; current phase is %s", pod.Status.Phase)
	}

	containerName := p.ContainerName
	if len(containerName) == 0 {
		container, err := podcmd.FindOrDefaultContainerByName(pod, containerName, p.Quiet, p.ErrOut)
		if err != nil {
			return err
		}
		containerName = container.Name
	}

	// ensure we can recover the terminal while attached
	t := p.SetupTTY()

	var sizeQueue remotecommand.TerminalSizeQueue
	if t.Raw {
		// this call spawns a goroutine to monitor/update the terminal size
		sizeQueue = t.MonitorSize(t.GetSize())

		// unset p.Err if it was previously set because both stdout and stderr go over p.Out when tty is
		// true
		p.ErrOut = nil
	}

	fn := func() error {
		restClient, err := restclient.RESTClientFor(p.Config)
		if err != nil {
			return err
		}

		req := restClient.Post().
			Resource("pods").
			Name(pod.Name).
			Namespace(pod.Namespace).
			SubResource("exec").
			Param(teleport.KubeSessionDisplayParticipantRequirementsQueryParam, strconv.FormatBool(p.displayParticipantRequirements)).
			Param(teleport.KubeSessionInvitedQueryParam, strings.Join(p.invited, ",")).
			Param(teleport.KubeSessionReasonQueryParam, p.reason)
		req.VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   p.Command,
			Stdin:     p.Stdin,
			Stdout:    p.Out != nil,
			Stderr:    p.ErrOut != nil,
			TTY:       t.Raw,
		}, scheme.ParameterCodec)

		return p.Executor.Execute(ctx, "POST", req.URL(), p.Config, p.In, p.Out, p.ErrOut, t.Raw, sizeQueue)
	}

	return trace.Wrap(t.Safe(fn))
}

type kubeExecCommand struct {
	*kingpin.CmdClause
	target                         string
	container                      string
	filename                       string
	quiet                          bool
	stdin                          bool
	tty                            bool
	reason                         string
	invited                        string
	command                        []string
	displayParticipantRequirements bool
}

func newKubeExecCommand(parent *kingpin.CmdClause) *kubeExecCommand {
	c := &kubeExecCommand{
		CmdClause: parent.Command("exec", "Execute a command in a Kubernetes pod."),
	}

	c.Flag("container", "Container name. If omitted, use the kubectl.kubernetes.io/default-container annotation for selecting the container to be attached or the first container in the pod will be chosen").Short('c').StringVar(&c.container)
	c.Flag("filename", "to use to exec into the resource").Short('f').StringVar(&c.filename)
	c.Flag("quiet", "Only print output from the remote session").Short('q').BoolVar(&c.quiet)
	c.Flag("stdin", "Pass stdin to the container").Short('s').BoolVar(&c.stdin)
	c.Flag("tty", "Stdin is a TTY").Short('t').BoolVar(&c.tty)
	c.Flag("reason", "The purpose of the session.").StringVar(&c.reason)
	c.Flag("invite", "A comma separated list of people to mark as invited for the session.").StringVar(&c.invited)
	c.Flag("participant-req", "Displays a verbose list of required participants in a moderated session.").BoolVar(&c.displayParticipantRequirements)
	c.Arg("target", "Pod or deployment name").Required().StringVar(&c.target)
	c.Arg("command", "Command to execute in the container").Required().StringsVar(&c.command)
	return c
}

func (c *kubeExecCommand) run(cf *CLIConf) error {
	closeFn, newKubeConfigLocation, err := maybeStartKubeLocalProxy(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	defer closeFn()

	f := c.kubeCmdFactory(newKubeConfigLocation)
	var p ExecOptions
	p.IOStreams = genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}
	p.ResourceName = c.target
	p.ContainerName = c.container
	p.Quiet = c.quiet
	p.Stdin = c.stdin
	p.TTY = c.tty
	p.Command = c.command
	p.ExecutablePodFn = polymorphichelpers.AttachablePodForObjectFn
	p.GetPodTimeout = time.Second * 5
	p.Builder = f.NewBuilder
	p.restClientGetter = f
	p.Executor = &DefaultRemoteExecutor{}
	p.displayParticipantRequirements = c.displayParticipantRequirements
	p.invited = strings.Split(c.invited, ",")
	p.reason = c.reason
	p.Namespace, p.EnforceNamespace, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return trace.Wrap(err)
	}

	p.Config, err = f.ToRESTConfig()
	if err != nil {
		return trace.Wrap(err)
	}

	clientset, err := f.KubernetesClientSet()
	if err != nil {
		return trace.Wrap(err)
	}

	p.PodClient = clientset.CoreV1()
	return trace.Wrap(p.Run(cf.Context))
}

func (c *kubeExecCommand) kubeCmdFactory(overwriteKubeConfigLocation string) cmdutil.Factory {
	kubeConfigFlags := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag()

	if overwriteKubeConfigLocation != "" {
		kubeConfigFlags.KubeConfig = &overwriteKubeConfigLocation
	}

	matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)
	return cmdutil.NewFactory(matchVersionKubeConfigFlags)
}

type kubeSessionsCommand struct {
	*kingpin.CmdClause
	format   string
	siteName string
}

func newKubeSessionsCommand(parent *kingpin.CmdClause) *kubeSessionsCommand {
	c := &kubeSessionsCommand{
		CmdClause: parent.Command("sessions", "Get a list of active Kubernetes sessions."),
	}
	c.Flag("format", defaults.FormatFlagDescription(defaults.DefaultFormats...)).Short('f').Default(teleport.Text).EnumVar(&c.format, defaults.DefaultFormats...)
	c.Flag("cluster", clusterHelp).Short('c').StringVar(&c.siteName)
	return c
}

func (c *kubeSessionsCommand) run(cf *CLIConf) error {
	if c.siteName != "" {
		cf.SiteName = c.siteName
	}
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	proxy, err := tc.ConnectToProxy(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}

	site := proxy.CurrentCluster()
	sessions, err := site.GetActiveSessionTrackers(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}

	filteredSessions := make([]types.SessionTracker, 0)
	for _, session := range sessions {
		if session.GetSessionKind() == types.KubernetesSessionKind {
			filteredSessions = append(filteredSessions, session)
		}
	}

	sort.Slice(filteredSessions, func(i, j int) bool {
		return filteredSessions[i].GetCreated().Before(filteredSessions[j].GetCreated())
	})

	format := strings.ToLower(c.format)
	switch format {
	case teleport.Text, "":
		printSessions(cf.Stdout(), filteredSessions)
	case teleport.JSON, teleport.YAML:
		out, err := serializeKubeSessions(sessions, format)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Fprintln(cf.Stdout(), out)
	default:
		return trace.BadParameter("unsupported format %q", c.format)
	}
	return nil
}

func serializeKubeSessions(sessions []types.SessionTracker, format string) (string, error) {
	var out []byte
	var err error
	if format == teleport.JSON {
		out, err = utils.FastMarshalIndent(sessions, "", "  ")
	} else {
		out, err = yaml.Marshal(sessions)
	}
	return string(out), trace.Wrap(err)
}

func printSessions(output io.Writer, sessions []types.SessionTracker) {
	table := asciitable.MakeTable([]string{"ID", "State", "Created", "Hostname", "Address", "Login", "Reason"})
	for _, s := range sessions {
		table.AddRow([]string{s.GetSessionID(), s.GetState().String(), s.GetCreated().Format(time.RFC3339), s.GetHostname(), s.GetAddress(), s.GetLogin(), s.GetReason()})
	}

	tableOutput := table.AsBuffer().String()
	fmt.Fprintln(output, tableOutput)
}

type kubeCredentialsCommand struct {
	*kingpin.CmdClause
	kubeCluster     string
	teleportCluster string
}

func newKubeCredentialsCommand(parent *kingpin.CmdClause) *kubeCredentialsCommand {
	c := &kubeCredentialsCommand{
		// This command is always hidden. It's called from the kubeconfig that
		// tsh generates and never by users directly.
		CmdClause: parent.Command("credentials", "Get credentials for kubectl access").Hidden(),
	}
	c.Flag("teleport-cluster", "Name of the Teleport cluster to get credentials for.").Required().StringVar(&c.teleportCluster)
	c.Flag("kube-cluster", "Name of the Kubernetes cluster to get credentials for.").Required().StringVar(&c.kubeCluster)
	return c
}

func getKubeCredLockfilePath(homePath, proxy string) (string, error) {
	profilePath := profile.FullProfilePath(homePath)
	// tsh stores the profiles using the proxy host as the profile name.
	profileName, err := utils.Host(proxy)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return keypaths.KubeCredLockfilePath(profilePath, profileName), nil
}

// errKubeCredLockfileFound is returned when kube credentials lockfile is found and user should resolve login problems manually.
var errKubeCredLockfileFound = trace.AlreadyExists("Having problems with relogin, please use 'tsh login/tsh kube login' manually")

func takeKubeCredLock(ctx context.Context, homePath, proxy string, lockTimeout time.Duration) (func(bool), error) {
	kubeCredLockfilePath, err := getKubeCredLockfilePath(homePath, proxy)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If kube credentials lockfile already exists, it means last time kube credentials was called
	// we had an error while trying to issue certificate, return an error asking user to login manually.
	if _, err := os.Stat(kubeCredLockfilePath); err == nil {
		log.Debugf("Kube credentials lockfile was found at %q, aborting.", kubeCredLockfilePath)
		return nil, trace.Wrap(errKubeCredLockfileFound)
	}

	if _, err := utils.EnsureLocalPath(kubeCredLockfilePath, "", ""); err != nil {
		return nil, trace.Wrap(err)
	}
	// Take a lock while we're trying to issue certificate and possibly relogin
	unlock, err := utils.FSTryWriteLockTimeout(ctx, kubeCredLockfilePath, lockTimeout)
	if err != nil {
		log.Debugf("could not take kube credentials lock: %v", err.Error())
		return nil, trace.Wrap(errKubeCredLockfileFound)
	}

	return func(removeFile bool) {
		// We must unlock the lockfile before removing it, otherwise unlock operation will fail
		// on Windows.
		if err := unlock(); err != nil {
			log.WithError(err).Warnf("could not unlock kube credentials lock")
		}
		if !removeFile {
			return
		}
		// Remove kube credentials lockfile.
		if err = os.Remove(kubeCredLockfilePath); err != nil && !os.IsNotExist(err) {
			log.WithError(err).Warnf("could not remove kube credentials lockfile %q", kubeCredLockfilePath)
		}
	}, nil
}

func (c *kubeCredentialsCommand) run(cf *CLIConf) error {
	profile, err := cf.GetProfile()
	if err != nil {
		// Cannot find the profile, continue to c.issueCert for a login.
		return trace.Wrap(c.issueCert(cf))
	}

	if err := c.checkLocalProxyRequirement(profile); err != nil {
		return trace.Wrap(err)
	}

	// client.LoadKeysToKubeFromStore function is used to speed up the credentials
	// loading process since Teleport Store transverses the entire store to find the keys.
	// This operation takes a long time when the store has a lot of keys and when
	// we call the function multiple times in parallel.
	// Although client.LoadKeysToKubeFromStore function speeds up the process since
	// it removes all transversals, it still has to read 2 different files from the disk:
	// - $TSH_HOME/keys/$PROXY/$USER-kube/$TELEPORT_CLUSTER/$KUBE_CLUSTER-x509.pem
	// - $TSH_HOME/keys/$PROXY/$USER
	//
	// In addition to these files, $TSH_HOME/$profile.yaml is also read from
	// cf.GetProfile call above.
	if kubeCert, privKey, err := client.LoadKeysToKubeFromStore(
		profile,
		cf.HomePath,
		c.teleportCluster,
		c.kubeCluster,
	); err == nil {
		crt, _ := tlsca.ParseCertificatePEM(kubeCert)
		if crt != nil && time.Until(crt.NotAfter) > time.Minute {
			log.Debugf("Re-using existing TLS cert for Kubernetes cluster %q", c.kubeCluster)
			return c.writeByteResponse(cf.Stdout(), kubeCert, privKey, crt.NotAfter)
		}
	}

	return trace.Wrap(c.issueCert(cf))
}

func (c *kubeCredentialsCommand) issueCert(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := c.checkLocalProxyRequirement(tc.Profile()); err != nil {
		return trace.Wrap(err)
	}

	_, span := tc.Tracer.Start(cf.Context, "tsh.kubeCredentials/GetKey")
	// Try loading existing keys.
	k, err := tc.LocalAgent().GetKey(c.teleportCluster, client.WithKubeCerts{})
	span.End()

	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	// Loaded existing credentials and have a cert for this cluster? Return it
	// right away.
	if err == nil {
		_, span := tc.Tracer.Start(cf.Context, "tsh.kubeCredentials/KubeX509Cert")
		crt, err := k.KubeX509Cert(c.kubeCluster)
		span.End()
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		if crt != nil && time.Until(crt.NotAfter) > time.Minute {
			log.Debugf("Re-using existing TLS cert for Kubernetes cluster %q", c.kubeCluster)

			return c.writeKeyResponse(cf.Stdout(), k, c.kubeCluster)
		}
		// Otherwise, cert for this k8s cluster is missing or expired. Request
		// a new one.
	}

	log.Debugf("Requesting TLS cert for Kubernetes cluster %q", c.kubeCluster)
	var unlockKubeCred func(bool)
	deleteKubeCredsLock := false
	defer func() {
		if unlockKubeCred != nil {
			unlockKubeCred(deleteKubeCredsLock) // by default (in case of an error) we don't delete lockfile.
		}
	}()

	ctx, span := tc.Tracer.Start(cf.Context, "tsh.kubeCredentials/RetryWithRelogin")
	err = client.RetryWithRelogin(
		ctx,
		tc,
		func() error {
			// The requirement may change after a new login so check again just in
			// case.
			if err := c.checkLocalProxyRequirement(tc.Profile()); err != nil {
				return trace.Wrap(err)
			}

			var err error
			k, err = tc.IssueUserCertsWithMFA(ctx, client.ReissueParams{
				RouteToCluster:    c.teleportCluster,
				KubernetesCluster: c.kubeCluster,
			})
			return err
		},
		client.WithBeforeLoginHook(
			// Before login we take a lock on the kube credentials file. This is
			// done to prevent multiple tsh processes from requesting login and
			// opening multiple browser tabs.
			func() error {
				var err error
				lockTimeout := 5 * time.Second
				// If we are under tests, MockSSOLogin is set and we want to allow just one try
				// to take the lock and fail if the lock is already taken. This is done to prevent
				// tests from hanging and continue to run once the lock is released.
				// FSLockRetryDelay is 10ms and we want to fail as fast as possible if the lock is
				// already taken by another process to validate that the lock is working as expected.
				if cf.mockSSOLogin != nil {
					lockTimeout = utils.FSLockRetryDelay
				}
				unlockKubeCred, err = takeKubeCredLock(cf.Context, cf.HomePath, cf.Proxy, lockTimeout)
				return trace.Wrap(err)
			},
		),
	)
	span.End()
	if err != nil {
		// If we've got network error we remove the lockfile, so we could restore from temporary connection
		// problems without requiring user intervention.
		if isNetworkError(err) {
			deleteKubeCredsLock = true
		}
		return trace.Wrap(err)
	}
	// Make sure the cert is allowed to access the cluster.
	// At this point we already know that the user has access to the cluster
	// via the RBAC rules, but we also need to make sure that the user has
	// access to the cluster with at least one kubernetes_user or kubernetes_group
	// defined.
	if err := checkIfCertsAreAllowedToAccessCluster(k, c.kubeCluster); err != nil {
		return trace.Wrap(err)
	}
	// Cache the new cert on disk for reuse.
	if err := tc.LocalAgent().AddKubeKey(k); err != nil {
		return trace.Wrap(err)
	}

	// Remove the lockfile so subsequent tsh kube credentials calls don't exit early
	deleteKubeCredsLock = true

	return c.writeKeyResponse(cf.Stdout(), k, c.kubeCluster)
}

func isNetworkError(err error) bool {
	var opErr *net.OpError
	return errors.As(err, &opErr) || trace.IsConnectionProblem(err)
}

func (c *kubeCredentialsCommand) checkLocalProxyRequirement(profile *profile.Profile) error {
	if profile.RequireKubeLocalProxy() {
		return trace.BadParameter("Cannot connect Kubernetes clients to Teleport Proxy directly. Please use `tsh proxy kube` or `tsh kubectl` instead.")
	}
	return nil
}

// checkIfCertsAreAllowedToAccessCluster evaluates if the new cert created by the user
// to access kubeCluster has at least one kubernetes_user or kubernetes_group
// defined. If not, it returns an error.
// This is a safety check in order to print a better message to the user even
// before hitting Teleport Kubernetes Proxy.
func checkIfCertsAreAllowedToAccessCluster(k *client.Key, kubeCluster string) error {
	for k8sCluster, cert := range k.KubeTLSCerts {
		if k8sCluster != kubeCluster {
			continue
		}
		log.Debugf("Got TLS cert for Kubernetes cluster %q", k8sCluster)
		exist, err := checkIfCertHasKubeGroupsAndUsers(cert)
		if err != nil {
			return trace.Wrap(err)
		} else if exist {
			return nil
		}
	}
	errMsg := "Your user's Teleport role does not allow Kubernetes access." +
		" Please ask cluster administrator to ensure your role has appropriate kubernetes_groups and kubernetes_users set."
	return trace.AccessDenied(errMsg)
}

// checkIfCertHasKubeGroupsAndUsers checks if the certificate has Kubernetes groups or users
// in the Subject Name. If it does, it returns true, otherwise false.
// Having no Kubernetes groups or users in the certificate means that the user
// is not allowed to access the Kubernetes cluster since Kubernetes Access enforces
// the presence of at least one of Kubernetes groups or users in the certificate.
// If the certificate does not have any Kubernetes groups or users, the
func checkIfCertHasKubeGroupsAndUsers(certB []byte) (bool, error) {
	cert, err := tlsca.ParseCertificatePEM(certB)
	if err != nil {
		return false, trace.Wrap(err)
	}
	for _, name := range cert.Subject.Names {
		if name.Type.Equal(tlsca.KubeGroupsASN1ExtensionOID) || name.Type.Equal(tlsca.KubeUsersASN1ExtensionOID) {
			return true, nil
		}
	}
	return false, nil
}

func (c *kubeCredentialsCommand) writeKeyResponse(output io.Writer, key *client.Key, kubeClusterName string) error {
	crt, err := key.KubeX509Cert(kubeClusterName)
	if err != nil {
		return trace.Wrap(err)
	}
	expiry := crt.NotAfter
	// Indicate slightly earlier expiration to avoid the cert expiring
	// mid-request, if possible.
	if time.Until(expiry) > time.Minute {
		expiry = expiry.Add(-1 * time.Minute)
	}

	// TODO (Joerger): Create a custom k8s Auth Provider or Exec Provider to use non-rsa
	// private keys for kube credentials (if possible)
	rsaKeyPEM, err := key.PrivateKey.RSAPrivateKeyPEM()
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(c.writeResponse(output, key.KubeTLSCerts[kubeClusterName], rsaKeyPEM, expiry))
}

// writeByteResponse writes the exec credential response to the output stream.
func (c *kubeCredentialsCommand) writeByteResponse(output io.Writer, kubeTLSCert, rsaKeyPEM []byte, expiry time.Time) error {
	// Indicate slightly earlier expiration to avoid the cert expiring
	// mid-request, if possible.
	if time.Until(expiry) > time.Minute {
		expiry = expiry.Add(-1 * time.Minute)
	}

	return trace.Wrap(c.writeResponse(output, kubeTLSCert, rsaKeyPEM, expiry))
}

// writeResponse writes the exec credential response to the output stream.
func (c *kubeCredentialsCommand) writeResponse(output io.Writer, kubeTLSCert, rsaKeyPEM []byte, expiry time.Time) error {
	resp := &clientauthentication.ExecCredential{
		Status: &clientauthentication.ExecCredentialStatus{
			ExpirationTimestamp:   &metav1.Time{Time: expiry},
			ClientCertificateData: string(kubeTLSCert),
			ClientKeyData:         string(rsaKeyPEM),
		},
	}
	data, err := runtime.Encode(kubeCodecs.LegacyCodec(kubeGroupVersion), resp)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Fprintln(output, string(data))
	return nil
}

type kubeLSCommand struct {
	*kingpin.CmdClause
	labels         string
	predicateExpr  string
	searchKeywords string
	format         string
	listAll        bool
	siteName       string
	verbose        bool
	quiet          bool
}

func newKubeLSCommand(parent *kingpin.CmdClause) *kubeLSCommand {
	c := &kubeLSCommand{
		CmdClause: parent.Command("ls", "Get a list of Kubernetes clusters."),
	}
	c.Flag("cluster", clusterHelp).Short('c').StringVar(&c.siteName)
	c.Flag("search", searchHelp).StringVar(&c.searchKeywords)
	c.Flag("query", queryHelp).StringVar(&c.predicateExpr)
	c.Flag("format", defaults.FormatFlagDescription(defaults.DefaultFormats...)).Short('f').Default(teleport.Text).EnumVar(&c.format, defaults.DefaultFormats...)
	c.Flag("all", "List Kubernetes clusters from all clusters and proxies.").Short('R').BoolVar(&c.listAll)
	c.Arg("labels", labelHelp).StringVar(&c.labels)
	c.Flag("verbose", "Show an untruncated list of labels.").Short('v').BoolVar(&c.verbose)
	c.Flag("quiet", "Quiet mode.").Short('q').BoolVar(&c.quiet)
	return c
}

type kubeListing struct {
	Proxy       string            `json:"proxy"`
	Cluster     string            `json:"cluster"`
	KubeCluster types.KubeCluster `json:"kube_cluster"`
}

type kubeListings []kubeListing

func (l kubeListings) Len() int {
	return len(l)
}

func (l kubeListings) Less(i, j int) bool {
	if l[i].Proxy != l[j].Proxy {
		return l[i].Proxy < l[j].Proxy
	}
	if l[i].Cluster != l[j].Cluster {
		return l[i].Cluster < l[j].Cluster
	}
	return l[i].KubeCluster.GetName() < l[j].KubeCluster.GetName()
}

func (l kubeListings) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func (c *kubeLSCommand) run(cf *CLIConf) error {
	cf.SearchKeywords = c.searchKeywords
	cf.Labels = c.labels
	cf.PredicateExpression = c.predicateExpr
	cf.SiteName = c.siteName

	if c.listAll {
		return trace.Wrap(c.runAllClusters(cf))
	}

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	currentTeleportCluster, kubeClusters, err := fetchKubeClusters(cf.Context, tc)
	if err != nil {
		return trace.Wrap(err)
	}

	// Ignore errors from fetching the current cluster, since it's not
	// mandatory to have a cluster selected or even to have a kubeconfig file.
	selectedCluster, _ := kubeconfig.SelectedKubeCluster(getKubeConfigPath(cf, ""), currentTeleportCluster)
	err = c.showKubeClusters(cf.Stdout(), kubeClusters, selectedCluster)
	return trace.Wrap(err)
}

func (c *kubeLSCommand) showKubeClusters(w io.Writer, kubeClusters types.KubeClusters, selectedCluster string) error {
	format := strings.ToLower(c.format)
	switch format {
	case teleport.Text, "":
		out := formatKubeClustersAsText(kubeClusters, selectedCluster, c.quiet, c.verbose)
		fmt.Fprintln(w, out)
	case teleport.JSON, teleport.YAML:
		sort.Sort(kubeClusters)
		out, err := serializeKubeClusters(kubeClusters, selectedCluster, format)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Fprintln(w, out)
	default:
		return trace.BadParameter("unsupported format %q", c.format)
	}
	return nil
}

func getKubeClusterTextRow(kc types.KubeCluster, selectedCluster string, verbose bool) []string {
	var selectedMark string
	var row []string
	if selectedCluster != "" && kc.GetName() == selectedCluster {
		selectedMark = "*"
	}
	displayName := common.FormatResourceName(kc, verbose)
	labels := common.FormatLabels(kc.GetAllLabels(), verbose)
	row = append(row, displayName, labels, selectedMark)
	return row
}

func formatKubeClustersAsText(kubeClusters types.KubeClusters, selectedCluster string, quiet, verbose bool) string {
	var (
		columns = []string{"Kube Cluster Name", "Labels", "Selected"}
		t       asciitable.Table
		rows    [][]string
	)

	for _, cluster := range kubeClusters {
		r := getKubeClusterTextRow(cluster, selectedCluster, verbose)
		rows = append(rows, r)
	}

	switch {
	case quiet:
		// no column headers and only include the cluster name and labels.
		t = asciitable.MakeHeadlessTable(2)
		for _, row := range rows {
			t.AddRow(row)
		}
	case verbose:
		t = asciitable.MakeTable(columns, rows...)
	default:
		t = asciitable.MakeTableWithTruncatedColumn(columns, rows, "Labels")
	}

	// stable sort by kube cluster name.
	t.SortRowsBy([]int{0}, true)
	return t.AsBuffer().String()
}

func serializeKubeClusters(kubeClusters []types.KubeCluster, selectedCluster, format string) (string, error) {
	type cluster struct {
		KubeClusterName string            `json:"kube_cluster_name"`
		Labels          map[string]string `json:"labels"`
		Selected        bool              `json:"selected"`
	}
	clusterInfo := make([]cluster, 0, len(kubeClusters))
	for _, cl := range kubeClusters {
		labels := cl.GetAllLabels()
		if len(labels) == 0 {
			labels = nil
		}
		clusterInfo = append(clusterInfo, cluster{
			KubeClusterName: cl.GetName(),
			Labels:          labels,
			Selected:        cl.GetName() == selectedCluster,
		})
	}
	var out []byte
	var err error
	if format == teleport.JSON {
		out, err = utils.FastMarshalIndent(clusterInfo, "", "  ")
	} else {
		out, err = yaml.Marshal(clusterInfo)
	}
	return string(out), trace.Wrap(err)
}

func (c *kubeLSCommand) runAllClusters(cf *CLIConf) error {
	var listings kubeListings

	err := forEachProfile(cf, func(tc *client.TeleportClient, profile *client.ProfileStatus) error {
		req := proto.ListResourcesRequest{
			SearchKeywords:      tc.SearchKeywords,
			PredicateExpression: tc.PredicateExpression,
			Labels:              tc.Labels,
		}

		kubeClusters, err := tc.ListKubernetesClustersWithFiltersAllClusters(cf.Context, req)
		if err != nil {
			return trace.Wrap(err)
		}
		for clusterName, kubeClusters := range kubeClusters {
			for _, kc := range kubeClusters {
				listings = append(listings, kubeListing{
					Proxy:       profile.ProxyURL.Host,
					Cluster:     clusterName,
					KubeCluster: kc,
				})
			}
		}
		return nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	format := strings.ToLower(c.format)
	switch format {
	case teleport.Text, "":
		out := formatKubeListingsAsText(listings, c.quiet, c.verbose)
		fmt.Fprintln(cf.Stdout(), out)
	case teleport.JSON, teleport.YAML:
		sort.Sort(listings)
		out, err := serializeKubeListings(listings, format)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Fprintln(cf.Stdout(), out)
	default:
		return trace.BadParameter("Unrecognized format %q", c.format)
	}

	return nil
}

func formatKubeListingsAsText(listings kubeListings, quiet, verbose bool) string {
	var (
		columns = []string{"Proxy", "Cluster", "Kube Cluster Name", "Labels"}
		t       asciitable.Table
		rows    [][]string
	)
	for _, listing := range listings {
		r := append([]string{
			listing.Proxy,
			listing.Cluster,
		}, getKubeClusterTextRow(listing.KubeCluster, "", verbose)...)
		rows = append(rows, r)
	}

	switch {
	case quiet:
		// quiet, so no column headers.
		t = asciitable.MakeHeadlessTable(4)
		for _, row := range rows {
			t.AddRow(row)
		}
	case verbose:
		t = asciitable.MakeTable(columns, rows...)
	default:
		t = asciitable.MakeTableWithTruncatedColumn(columns, rows, "Labels")
	}
	// stable sort by proxy, then cluster, then kube cluster name.
	t.SortRowsBy([]int{0, 1, 2}, true)
	return t.AsBuffer().String()
}

func serializeKubeListings(kubeListings []kubeListing, format string) (string, error) {
	var out []byte
	var err error
	if format == teleport.JSON {
		out, err = utils.FastMarshalIndent(kubeListings, "", "  ")
	} else {
		out, err = yaml.Marshal(kubeListings)
	}
	return string(out), trace.Wrap(err)
}

type kubeLoginCommand struct {
	*kingpin.CmdClause
	kubeCluster          string
	siteName             string
	impersonateUser      string
	impersonateGroups    []string
	namespace            string
	all                  bool
	overrideContextName  string
	disableAccessRequest bool
	requestReason        string
}

func newKubeLoginCommand(parent *kingpin.CmdClause) *kubeLoginCommand {
	c := &kubeLoginCommand{
		CmdClause: parent.Command("login", "Login to a Kubernetes cluster."),
	}
	c.Flag("cluster", clusterHelp).Short('c').StringVar(&c.siteName)
	c.Arg("kube-cluster", "Name of the Kubernetes cluster to login to. Check 'tsh kube ls' for a list of available clusters.").StringVar(&c.kubeCluster)
	c.Flag("as", "Configure custom Kubernetes user impersonation.").StringVar(&c.impersonateUser)
	c.Flag("as-groups", "Configure custom Kubernetes group impersonation.").StringsVar(&c.impersonateGroups)
	// TODO (tigrato): move this back to namespace once teleport drops the namespace flag.
	c.Flag("kube-namespace", "Configure the default Kubernetes namespace.").Short('n').StringVar(&c.namespace)
	c.Flag("all", "Generate a kubeconfig with every cluster the user has access to.").BoolVar(&c.all)
	c.Flag("set-context-name", "Define a custom context name. To use it with --all include \"{{.KubeName}}\"").
		// Use the default context name template if --set-context-name is not set.
		// This works as an hint to the user that the context name can be customized.
		Default(kubeconfig.ContextName("{{.ClusterName}}", "{{.KubeName}}")).
		StringVar(&c.overrideContextName)
	c.Flag("request-reason", "Reason for requesting access").StringVar(&c.requestReason)
	c.Flag("disable-access-request", "Disable automatic resource access requests").BoolVar(&c.disableAccessRequest)
	return c
}

func (c *kubeLoginCommand) run(cf *CLIConf) error {
	if c.kubeCluster == "" && !c.all {
		return trace.BadParameter("kube-cluster name is required. Check 'tsh kube ls' for a list of available clusters.")
	}
	// If --all and --set-context-name are set, ensure that the template is valid
	// and can produce distinct context names for each cluster before proceeding.
	if err := kubeconfig.CheckContextOverrideTemplate(c.overrideContextName); err != nil && c.all {
		return trace.Wrap(err)
	}

	// Set CLIConf.KubernetesCluster so that the kube cluster's context is automatically selected.
	cf.KubernetesCluster = c.kubeCluster
	cf.SiteName = c.siteName
	cf.kubernetesImpersonationConfig = impersonationConfig{
		kubernetesUser:   c.impersonateUser,
		kubernetesGroups: c.impersonateGroups,
	}
	cf.kubeNamespace = c.namespace
	cf.disableAccessRequest = c.disableAccessRequest
	cf.RequestReason = c.requestReason
	cf.ListAll = c.all
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	err = retryWithAccessRequest(cf, tc, func() error {
		// Check that this kube cluster exists.
		currentTeleportCluster, kubeClusters, err := fetchKubeClusters(cf.Context, tc)
		if err != nil {
			return trace.Wrap(err)
		}
		clusterNames := kubeClustersToStrings(kubeClusters)
		// If the user is trying to login to a specific cluster, check that it exists.
		switch {
		case c.kubeCluster != "" && !slices.Contains(clusterNames, c.kubeCluster):
			return trace.AccessDenied("kubernetes cluster %q not found, check 'tsh kube ls' for a list of known clusters", c.kubeCluster)
		case cf.ListAll && len(clusterNames) == 0:
			return trace.AccessDenied("no kubernetes clusters found, check 'tsh kube ls' for a list of known clusters")
		}

		// Update default kubeconfig file located at ~/.kube/config or the value of
		// KUBECONFIG env var even if the context exists.
		if err := updateKubeConfig(cf, tc, "", c.overrideContextName); err != nil {
			return trace.Wrap(err)
		}

		// Generate a profile specific kubeconfig which can be used
		// by setting the kubeconfig environment variable (with `tsh env`)
		profileKubeconfigPath := keypaths.KubeConfigPath(
			profile.FullProfilePath(cf.HomePath), tc.WebProxyHost(), tc.Username, currentTeleportCluster, c.kubeCluster,
		)
		if err := updateKubeConfig(cf, tc, profileKubeconfigPath, c.overrideContextName); err != nil {
			return trace.Wrap(err)
		}

		c.printUserMessage(cf, tc)
		return nil
	},
		accessRequestForKubeCluster,
		resourceNameOrWildcard(c.kubeCluster, c.all),
	)
	return trace.Wrap(err)
}

func resourceNameOrWildcard(clusterName string, listAll bool) string {
	if clusterName != "" {
		return clusterName
	} else if listAll {
		return "*"
	}
	return ""
}

func (c *kubeLoginCommand) printUserMessage(cf *CLIConf, tc *client.TeleportClient) {
	if tc.Profile().RequireKubeLocalProxy() {
		c.printLocalProxyUserMessage(cf)
		return
	}

	if c.kubeCluster != "" {
		fmt.Fprintf(cf.Stdout(), "Logged into Kubernetes cluster %q. Try 'kubectl version' to test the connection.\n", c.kubeCluster)
	} else {
		fmt.Fprintf(cf.Stdout(), "Created kubeconfig with every Kubernetes cluster available. Select a context and try 'kubectl version' to test the connection.\n")
	}
}

func (c *kubeLoginCommand) printLocalProxyUserMessage(cf *CLIConf) {
	switch {
	case c.kubeCluster != "":
		fmt.Fprintf(cf.Stdout(), `Logged into Kubernetes cluster %q.`, c.kubeCluster)

	default:
		fmt.Fprintf(cf.Stdout(), "Logged into all Kubernetes clusters available.")
	}

	fmt.Fprintf(cf.Stdout(), `

Your Teleport cluster runs behind a layer 7 load balancer or reverse proxy.

To access the cluster, use "tsh kubectl" which is a fully featured "kubectl"
command that works when the Teleport cluster is behind layer 7 load balancer or
reverse proxy. To run the Kubernetes client, use:
  tsh kubectl version

Or, start a local proxy with "tsh proxy kube" and use the kubeconfig
provided by the local proxy with your native Kubernetes clients:
  tsh proxy kube -p 8443

Learn more at https://goteleport.com/docs/architecture/tls-routing/#working-with-layer-7-load-balancers-or-reverse-proxies-preview
`)
}

func fetchKubeClusters(ctx context.Context, tc *client.TeleportClient) (teleportCluster string, kubeClusters []types.KubeCluster, err error) {
	err = client.RetryWithRelogin(ctx, tc, func() error {
		pc, err := tc.ConnectToProxy(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		defer pc.Close()

		ac := pc.CurrentCluster()
		defer ac.Close()

		teleportCluster = pc.ClusterName()
		kubeClusters, err = kubeutils.ListKubeClustersWithFilters(ctx, ac, proto.ListResourcesRequest{
			SearchKeywords:      tc.SearchKeywords,
			PredicateExpression: tc.PredicateExpression,
			Labels:              tc.Labels,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		return nil
	})
	if err != nil {
		return "", nil, trace.Wrap(err)
	}
	return teleportCluster, kubeClusters, nil
}

func kubeClustersToStrings(kubeClusters []types.KubeCluster) []string {
	names := make([]string, len(kubeClusters))
	for i, cluster := range kubeClusters {
		names[i] = cluster.GetName()
	}

	return names
}

// kubernetesStatus holds teleport client information necessary to populate the user's kubeconfig.
type kubernetesStatus struct {
	clusterAddr         string
	teleportClusterName string
	kubeClusters        []types.KubeCluster
	credentials         *client.Key
	tlsServerName       string
}

// fetchKubeStatus returns a kubernetesStatus populated from the given TeleportClient.
func fetchKubeStatus(ctx context.Context, tc *client.TeleportClient) (*kubernetesStatus, error) {
	var err error
	kubeStatus := &kubernetesStatus{
		clusterAddr: tc.KubeClusterAddr(),
	}
	kubeStatus.credentials, err = tc.LocalAgent().GetCoreKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	kubeStatus.teleportClusterName, kubeStatus.kubeClusters, err = fetchKubeClusters(ctx, tc)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if tc.TLSRoutingEnabled {
		k8host, _ := tc.KubeProxyHostPort()
		kubeStatus.tlsServerName = client.GetKubeTLSServerName(k8host)
	}

	return kubeStatus, nil
}

// buildKubeConfigUpdate returns a kubeconfig.Values suitable for updating the user's kubeconfig
// based on the CLI parameters and the given kubernetesStatus.
func buildKubeConfigUpdate(cf *CLIConf, kubeStatus *kubernetesStatus, overrideContextName string) (*kubeconfig.Values, error) {
	v := &kubeconfig.Values{
		ClusterAddr:         kubeStatus.clusterAddr,
		TeleportClusterName: kubeStatus.teleportClusterName,
		Credentials:         kubeStatus.credentials,
		ProxyAddr:           cf.Proxy,
		TLSServerName:       kubeStatus.tlsServerName,
		Impersonate:         cf.kubernetesImpersonationConfig.kubernetesUser,
		ImpersonateGroups:   cf.kubernetesImpersonationConfig.kubernetesGroups,
		Namespace:           cf.kubeNamespace,
		// Only switch the current context if kube-cluster is explicitly set on the command line.
		SelectCluster:   cf.KubernetesCluster,
		OverrideContext: overrideContextName,
	}

	if cf.executablePath == "" {
		// Don't know tsh path.
		// Fall back to the old kubeconfig, with static credentials from v.Credentials.
		return v, nil
	}

	if len(kubeStatus.kubeClusters) == 0 {
		// If there are no registered k8s clusters, we may have an older teleport cluster.
		// Fall back to the old kubeconfig, with static credentials from v.Credentials.
		log.Debug("Disabling exec plugin mode for kubeconfig because this Teleport cluster has no Kubernetes clusters.")
		return v, nil
	}

	clusterNames := kubeClustersToStrings(kubeStatus.kubeClusters)

	// Validate if cf.KubernetesCluster is part of the returned list of clusters
	if cf.KubernetesCluster != "" && !slices.Contains(clusterNames, cf.KubernetesCluster) {
		return nil, trace.NotFound("Kubernetes cluster %q is not registered in this Teleport cluster; you can list registered Kubernetes clusters using 'tsh kube ls'.", cf.KubernetesCluster)
	}
	// If ListAll is not enabled, update only cf.KubernetesCluster cluster.
	if cf.KubernetesCluster != "" && !cf.ListAll {
		clusterNames = []string{cf.KubernetesCluster}
	}

	v.KubeClusters = clusterNames
	v.Exec = &kubeconfig.ExecValues{
		TshBinaryPath:     cf.executablePath,
		TshBinaryInsecure: cf.InsecureSkipVerify,
		Env:               make(map[string]string),
	}

	if cf.HomePath != "" {
		v.Exec.Env[types.HomeEnvVar] = cf.HomePath
	}

	return v, nil
}

// impersonationConfig allows to configure custom kubernetes impersonation values.
type impersonationConfig struct {
	// kubernetesUser specifies the kubernetes user to impersonate request as.
	kubernetesUser string
	// kubernetesGroups specifies the kubernetes groups to impersonate request as.
	kubernetesGroups []string
}

// updateKubeConfig adds Teleport configuration to the users's kubeconfig based on the CLI
// parameters and the kubernetes services in the current Teleport cluster. If no path for
// the kubeconfig is given, it will use environment values or known defaults to get a path.
func updateKubeConfig(cf *CLIConf, tc *client.TeleportClient, path string, overrideContext string) error {
	// Fetch proxy's advertised ports to check for k8s support.
	if _, err := tc.Ping(cf.Context); err != nil {
		return trace.Wrap(err)
	}
	if tc.KubeProxyAddr == "" {
		// Kubernetes support disabled, don't touch kubeconfig.
		return nil
	}

	kubeStatus, err := fetchKubeStatus(cf.Context, tc)
	if err != nil {
		return trace.Wrap(err)
	}

	if cf.Proxy == "" {
		cf.Proxy = tc.WebProxyAddr
	}

	values, err := buildKubeConfigUpdate(cf, kubeStatus, overrideContext)
	if err != nil {
		return trace.Wrap(err)
	}

	path = getKubeConfigPath(cf, path)

	// If this is a profile specific kubeconfig, we only need
	// to put the selected kube cluster into the kubeconfig.
	isKubeConfig, err := keypaths.IsProfileKubeConfigPath(path)
	if err != nil {
		return trace.Wrap(err)
	}
	if isKubeConfig {
		if !strings.Contains(path, cf.KubernetesCluster) {
			return trace.BadParameter("profile specific kubeconfig is in use, run 'eval $(tsh env --unset)' to switch contexts to another kube cluster")
		}
		values.KubeClusters = []string{cf.KubernetesCluster}
	}

	return trace.Wrap(kubeconfig.Update(path, *values, tc.LoadAllCAs))
}

func getKubeConfigPath(cf *CLIConf, path string) string {
	// cf.kubeConfigPath is used in tests to allow Teleport to run tsh login commands
	// in parallel. If defined, it should take precedence over kubeconfig.PathFromEnv().
	if path == "" && cf.kubeConfigPath != "" {
		path = cf.kubeConfigPath
	} else if path == "" {
		path = kubeconfig.PathFromEnv()
	}
	return path
}

// Required magic boilerplate to use the k8s encoder.

var (
	kubeScheme       = runtime.NewScheme()
	kubeCodecs       = serializer.NewCodecFactory(kubeScheme)
	kubeGroupVersion = schema.GroupVersion{
		Group:   "client.authentication.k8s.io",
		Version: "v1beta1",
	}
)

func init() {
	metav1.AddToGroupVersion(kubeScheme, schema.GroupVersion{Version: "v1"})
	clientauthv1beta1.AddToScheme(kubeScheme)
	clientauthentication.AddToScheme(kubeScheme)
}

// accessRequestForKubeCluster attempts to create a resource access request for the case
// where "tsh kube login" was attempted and access was denied
func accessRequestForKubeCluster(ctx context.Context, cf *CLIConf, tc *client.TeleportClient) (types.AccessRequest, error) {
	if tc.KubernetesCluster == "" && !cf.ListAll {
		return nil, trace.BadParameter("no KubernetesCluster specified")
	}
	clt, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer clt.Close()

	// Match on cluster name
	expr := ""
	if !cf.ListAll {
		expr = fmt.Sprintf(`resource.metadata.name == "%s"`, tc.KubernetesCluster)
	}
	kubes, err := apiclient.GetAllResources[types.KubeCluster](ctx, clt.AuthClient, &proto.ListResourcesRequest{
		Namespace:           apidefaults.Namespace,
		ResourceType:        types.KindKubernetesCluster,
		UseSearchAsRoles:    true,
		PredicateExpression: expr,
	})
	switch {
	case err != nil:
		return nil, trace.Wrap(err)
	case len(kubes) == 0:
		return nil, trace.NotFound("kubernetes cluster %q not found, unable to request access", tc.KubernetesCluster)
	case len(kubes) > 1 && !cf.ListAll:
		return nil, trace.BadParameter("more than one kubernetes cluster matched %q", tc.KubernetesCluster)
	}

	requestResourceIDs := make([]types.ResourceID, len(kubes))
	for i, kube := range kubes {
		requestResourceIDs[i] = types.ResourceID{
			ClusterName: tc.SiteName,
			Kind:        types.KindKubernetesCluster,
			Name:        kube.GetName(),
		}
	}

	// Roles to request will be automatically determined on the backend.
	req, err := services.NewAccessRequestWithResources(tc.Username, nil /* roles */, requestResourceIDs)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Set the DryRun flag and send the request to auth for full validation. If
	// the user has no search_as_roles or is not allowed to connect to the Kube cluster
	// we will get an error here.
	req.SetDryRun(true)
	req.SetRequestReason("Dry run, this request will not be created. If you see this, there is a bug.")
	if err := tc.WithRootClusterClient(ctx, func(clt auth.ClientI) error {
		return trace.Wrap(clt.CreateAccessRequest(ctx, req))
	}); err != nil {
		return nil, trace.Wrap(err)
	}
	req.SetDryRun(false)
	req.SetRequestReason("")

	return req, nil
}
