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

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/moby/term"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/observability/tracing"
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/sshutils/sftp"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/socks"
)

// NodeClient implements ssh client to a ssh node (teleport or any regular ssh node)
// NodeClient can run shell and commands or upload and download files.
type NodeClient struct {
	Namespace   string
	Tracer      oteltrace.Tracer
	Client      *tracessh.Client
	TC          *TeleportClient
	OnMFA       func()
	FIPSEnabled bool

	mu      sync.Mutex
	closers []io.Closer

	// ProxyPublicAddr is the web proxy public addr, as opposed to the local proxy
	// addr set in TC.WebProxyAddr. This is needed to report the correct address
	// to SSH_TELEPORT_WEBPROXY_ADDR used by some features like "teleport status".
	ProxyPublicAddr string

	// hostname is the node's hostname, for more user-friendly logging.
	hostname string

	// sshLogDir is the directory to log the output of multiple SSH commands to.
	// If not set, no logs will be created.
	sshLogDir string
}

// AddCloser adds an [io.Closer] that will be closed when the
// client is closed.
func (c *NodeClient) AddCloser(closer io.Closer) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.closers = append(c.closers, closer)
}

type closerFunc func() error

func (f closerFunc) Close() error {
	return f()
}

// AddCancel adds a [context.CancelFunc] that will be canceled when the
// client is closed.
func (c *NodeClient) AddCancel(cancel context.CancelFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.closers = append(c.closers, closerFunc(func() error {
		cancel()
		return nil
	}))
}

// ReissueParams encodes optional parameters for
// user certificate reissue.
type ReissueParams struct {
	RouteToCluster    string
	NodeName          string
	KubernetesCluster string
	AccessRequests    []string
	// See [proto.UserCertsRequest.DropAccessRequests].
	DropAccessRequests    []string
	RouteToDatabase       proto.RouteToDatabase
	RouteToApp            proto.RouteToApp
	RouteToWindowsDesktop proto.RouteToWindowsDesktop

	// ExistingCreds is a gross hack for lib/web/terminal.go to pass in
	// existing user credentials. The TeleportClient in lib/web/terminal.go
	// doesn't have a real LocalKeystore and keeps all certs in memory.
	// Normally, existing credentials are loaded from
	// TeleportClient.localAgent.
	//
	// TODO(awly): refactor lib/web to use a Keystore implementation that
	// mimics LocalKeystore and remove this.
	ExistingCreds *Key

	// MFACheck is optional parameter passed if MFA check was already done.
	// It can be nil.
	MFACheck *proto.IsMFARequiredResponse
	// AuthClient is the client used for the MFACheck that can be reused
	AuthClient authclient.ClientI
	// RequesterName identifies who is sending the cert reissue request.
	RequesterName proto.UserCertsRequest_Requester
}

func (p ReissueParams) usage() proto.UserCertsRequest_CertUsage {
	switch {
	case p.NodeName != "":
		// SSH means a request for an SSH certificate for access to a specific
		// SSH node, as specified by NodeName.
		return proto.UserCertsRequest_SSH
	case p.KubernetesCluster != "":
		// Kubernetes means a request for a TLS certificate for access to a
		// specific Kubernetes cluster, as specified by KubernetesCluster.
		return proto.UserCertsRequest_Kubernetes
	case p.RouteToDatabase.ServiceName != "":
		// Database means a request for a TLS certificate for access to a
		// specific database, as specified by RouteToDatabase.
		return proto.UserCertsRequest_Database
	case p.RouteToApp.Name != "":
		// App means a request for a TLS certificate for access to a specific
		// web app, as specified by RouteToApp.
		return proto.UserCertsRequest_App
	case p.RouteToWindowsDesktop.WindowsDesktop != "":
		return proto.UserCertsRequest_WindowsDesktop
	default:
		// All means a request for both SSH and TLS certificates for the
		// overall user session. These certificates are not specific to any SSH
		// node, Kubernetes cluster, database or web app.
		return proto.UserCertsRequest_All
	}
}

func (p ReissueParams) isMFARequiredRequest(sshLogin string) *proto.IsMFARequiredRequest {
	req := new(proto.IsMFARequiredRequest)
	switch {
	case p.NodeName != "":
		req.Target = &proto.IsMFARequiredRequest_Node{Node: &proto.NodeLogin{Node: p.NodeName, Login: sshLogin}}
	case p.KubernetesCluster != "":
		req.Target = &proto.IsMFARequiredRequest_KubernetesCluster{KubernetesCluster: p.KubernetesCluster}
	case p.RouteToDatabase.ServiceName != "":
		req.Target = &proto.IsMFARequiredRequest_Database{Database: &p.RouteToDatabase}
	case p.RouteToWindowsDesktop.WindowsDesktop != "":
		req.Target = &proto.IsMFARequiredRequest_WindowsDesktop{WindowsDesktop: &p.RouteToWindowsDesktop}
	case p.RouteToApp.Name != "":
		req.Target = &proto.IsMFARequiredRequest_App{App: &p.RouteToApp}
	}
	return req
}

// CertCachePolicy describes what should happen to the certificate cache when a
// user certificate is re-issued
type CertCachePolicy int

const (
	// CertCacheDrop indicates that all user certificates should be dropped as
	// part of the re-issue process. This can be necessary if the roles
	// assigned to the user are expected to change as a part of the re-issue.
	CertCacheDrop CertCachePolicy = 0

	// CertCacheKeep indicates that all user certificates (except those
	// explicitly updated by the re-issue) should be preserved across the
	// re-issue process.
	CertCacheKeep CertCachePolicy = 1
)

// makeDatabaseClientPEM returns appropriate client PEM file contents for the
// specified database type. Some databases only need certificate in the PEM
// file, others both certificate and key.
func makeDatabaseClientPEM(proto string, cert []byte, pk *Key) ([]byte, error) {
	// MongoDB expects certificate and key pair in the same pem file.
	if proto == defaults.ProtocolMongoDB {
		rsaKeyPEM, err := pk.PrivateKey.RSAPrivateKeyPEM()
		if err == nil {
			return append(cert, rsaKeyPEM...), nil
		} else if !trace.IsBadParameter(err) {
			return nil, trace.Wrap(err)
		}
		log.WithError(err).Warn("MongoDB integration is not supported when logging in with a non-rsa private key.")
	}
	return cert, nil
}

// PromptMFAChallengeHandler is a handler for MFA challenges.
//
// The challenge c from proxyAddr should be presented to the user, asking to
// use one of their registered MFA devices. User's response should be returned,
// or an error if anything goes wrong.
type PromptMFAChallengeHandler func(ctx context.Context, proxyAddr string, c *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error)

// sharedAuthClient is a wrapper around auth.ClientI which
// prevents the underlying client from being closed.
type sharedAuthClient struct {
	authclient.ClientI
}

// Close is a no-op
func (a sharedAuthClient) Close() error {
	return nil
}

// nodeName removes the port number from the hostname, if present
func nodeName(node targetNode) string {
	if node.hostname != "" {
		return node.hostname
	}
	n, _, err := net.SplitHostPort(node.addr)
	if err != nil {
		return node.addr
	}
	return n
}

// NodeDetails provides connection information for a node
type NodeDetails struct {
	// Addr is an address to dial
	Addr string
	// Namespace is the node namespace
	Namespace string
	// Cluster is the name of the target cluster
	Cluster string

	// MFACheck is optional parameter passed if MFA check was already done.
	// It can be nil.
	MFACheck *proto.IsMFARequiredResponse

	// hostname is the node's hostname, for more user-friendly logging.
	hostname string
}

// String returns a user-friendly name
func (n NodeDetails) String() string {
	parts := []string{nodeName(targetNode{addr: n.Addr})}
	if n.Cluster != "" {
		parts = append(parts, "on cluster", n.Cluster)
	}
	return strings.Join(parts, " ")
}

// ProxyFormat returns the address in the format
// used by the proxy subsystem
func (n *NodeDetails) ProxyFormat() string {
	parts := []string{n.Addr}
	if n.Namespace != "" {
		parts = append(parts, n.Namespace)
	}
	if n.Cluster != "" {
		parts = append(parts, n.Cluster)
	}
	return strings.Join(parts, "@")
}

// NodeClientOption is a functional argument for NewNodeClient.
type NodeClientOption func(nc *NodeClient)

// WithNodeHostname sets the hostname to display for the connected node.
func WithNodeHostname(hostname string) NodeClientOption {
	return func(nc *NodeClient) {
		nc.hostname = hostname
	}
}

// WithSSHLogDir sets the directory to write command output to when running
// commands on multiple nodes.
func WithSSHLogDir(logDir string) NodeClientOption {
	return func(nc *NodeClient) {
		nc.sshLogDir = logDir
	}
}

// NewNodeClient constructs a NodeClient that is connected to the node at nodeAddress.
// The nodeName field is optional and is used only to present better error messages.
func NewNodeClient(ctx context.Context, sshConfig *ssh.ClientConfig, conn net.Conn, nodeAddress, nodeName string, tc *TeleportClient, fipsEnabled bool, opts ...NodeClientOption) (*NodeClient, error) {
	ctx, span := tc.Tracer.Start(
		ctx,
		"NewNodeClient",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("node", nodeAddress),
		),
	)
	defer span.End()

	if nodeName == "" {
		nodeName = nodeAddress
	}

	sshconn, chans, reqs, err := newClientConn(ctx, conn, nodeAddress, sshConfig)
	if err != nil {
		if utils.IsHandshakeFailedError(err) {
			conn.Close()
			// TODO(codingllama): Improve error message below for device trust.
			//  An alternative we have here is querying the cluster to check if device
			//  trust is required, a check similar to `IsMFARequired`.
			log.Infof("Access denied to %v connecting to %v: %v", sshConfig.User, nodeName, err)
			return nil, trace.AccessDenied(`access denied to %v connecting to %v`, sshConfig.User, nodeName)
		}
		return nil, trace.Wrap(err)
	}

	// We pass an empty channel which we close right away to ssh.NewClient
	// because the client need to handle requests itself.
	emptyCh := make(chan *ssh.Request)
	close(emptyCh)

	nc := &NodeClient{
		Client:          tracessh.NewClient(sshconn, chans, emptyCh),
		Namespace:       apidefaults.Namespace,
		TC:              tc,
		Tracer:          tc.Tracer,
		FIPSEnabled:     fipsEnabled,
		ProxyPublicAddr: tc.WebProxyAddr,
		hostname:        nodeName,
	}

	for _, opt := range opts {
		opt(nc)
	}

	// Start a goroutine that will run for the duration of the client to process
	// global requests from the client. Teleport clients will use this to update
	// terminal sizes when the remote PTY size has changed.
	go nc.handleGlobalRequests(ctx, reqs)

	return nc, nil
}

// RunInteractiveShell creates an interactive shell on the node and copies stdin/stdout/stderr
// to and from the node and local shell. This will block until the interactive shell on the node
// is terminated.
func (c *NodeClient) RunInteractiveShell(ctx context.Context, mode types.SessionParticipantMode, sessToJoin types.SessionTracker, chanReqCallback tracessh.ChannelRequestCallback, beforeStart func(io.Writer)) error {
	ctx, span := c.Tracer.Start(
		ctx,
		"nodeClient/RunInteractiveShell",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	env := c.TC.newSessionEnv()
	env[teleport.EnvSSHJoinMode] = string(mode)
	env[teleport.EnvSSHSessionReason] = c.TC.Config.Reason
	env[teleport.EnvSSHSessionDisplayParticipantRequirements] = strconv.FormatBool(c.TC.Config.DisplayParticipantRequirements)
	encoded, err := json.Marshal(&c.TC.Config.Invited)
	if err != nil {
		return trace.Wrap(err)
	}
	env[teleport.EnvSSHSessionInvited] = string(encoded)

	// Overwrite "SSH_SESSION_WEBPROXY_ADDR" with the public addr reported by the proxy. Otherwise,
	// this would be set to the localhost addr (tc.WebProxyAddr) used for Web UI client connections.
	if c.ProxyPublicAddr != "" && c.TC.WebProxyAddr != c.ProxyPublicAddr {
		env[teleport.SSHSessionWebProxyAddr] = c.ProxyPublicAddr
	}

	nodeSession, err := newSession(ctx, c, sessToJoin, env, c.TC.Stdin, c.TC.Stdout, c.TC.Stderr, c.TC.EnableEscapeSequences)
	if err != nil {
		return trace.Wrap(err)
	}

	if err = nodeSession.runShell(ctx, mode, c.TC.OnChannelRequest, beforeStart, c.TC.OnShellCreated); err != nil {
		var exitErr *ssh.ExitError
		var exitMissingErr *ssh.ExitMissingError
		switch err := trace.Unwrap(err); {
		case errors.As(err, &exitErr):
			c.TC.ExitStatus = exitErr.ExitStatus()
		case errors.As(err, &exitMissingErr):
			c.TC.ExitStatus = 1
		}

		return trace.Wrap(err)
	}

	if nodeSession.ExitMsg == "" {
		fmt.Fprintln(c.TC.Stderr, "the connection was closed on the remote side at ", time.Now().Format(time.RFC822))
	} else {
		fmt.Fprintln(c.TC.Stderr, nodeSession.ExitMsg)
	}

	return nil
}

// lineLabeledWriter is an io.Writer that prepends a label to each line it writes.
type lineLabeledWriter struct {
	linePrefix        []byte
	w                 io.Writer
	shouldWritePrefix bool
}

func newLineLabeledWriter(w io.Writer, label string) io.Writer {
	return &lineLabeledWriter{
		linePrefix:        []byte(fmt.Sprintf("[%v] ", label)),
		w:                 w,
		shouldWritePrefix: true,
	}
}

func (lw *lineLabeledWriter) writeChunk(b []byte, bytesWritten int, newline bool) (int, error) {
	n, err := lw.w.Write(b)
	bytesWritten += n
	if err != nil {
		return bytesWritten, trace.Wrap(err)
	}
	if newline {
		n, err = lw.w.Write([]byte("\n"))
		bytesWritten += n
	}
	return bytesWritten, trace.Wrap(err)
}

func (lw *lineLabeledWriter) Write(input []byte) (int, error) {
	bytesWritten := 0
	var line []byte
	rest := input
	var found bool
	for {
		line, rest, found = bytes.Cut(rest, []byte("\n"))
		// Write the prefix unless we're either continuing a line from the last
		// write or we're at the end.
		if lw.shouldWritePrefix && (len(line) > 0 || found) {
			// Write the prefix on its own to not mess with the eventual returned
			// number of bytes written.
			if _, err := lw.w.Write(lw.linePrefix); err != nil {
				return bytesWritten, trace.Wrap(err)
			}
		}
		var err error
		if bytesWritten, err = lw.writeChunk(line, bytesWritten, found); err != nil {
			return bytesWritten, trace.Wrap(err)
		}
		lw.shouldWritePrefix = true

		if !found {
			// If there were leftovers, the line will continue on the next write, so
			// skip the first prefix next time.
			lw.shouldWritePrefix = len(line) == 0
			break
		}
	}

	return bytesWritten, nil
}

// RunCommandOptions is a set of options for NodeClient.RunCommand.
type RunCommandOptions struct {
	labelLines bool
}

// RunCommandOption is a functional argument for NodeClient.RunCommand.
type RunCommandOption func(opts *RunCommandOptions)

// WithLabeledOutput labels each line of output from a command with the node's
// hostname.
func WithLabeledOutput() RunCommandOption {
	return func(opts *RunCommandOptions) {
		opts.labelLines = true
	}
}

// RunCommand executes a given bash command on the node.
func (c *NodeClient) RunCommand(ctx context.Context, command []string, opts ...RunCommandOption) error {
	ctx, span := c.Tracer.Start(
		ctx,
		"nodeClient/RunCommand",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	var options RunCommandOptions
	for _, opt := range opts {
		opt(&options)
	}

	// Set up output streams
	stdout := c.TC.Stdout
	stderr := c.TC.Stderr
	if c.hostname != "" {
		if options.labelLines {
			stdout = newLineLabeledWriter(c.TC.Stdout, c.hostname)
			stderr = newLineLabeledWriter(c.TC.Stderr, c.hostname)
		}

		if c.sshLogDir != "" {
			stdoutFile, err := os.Create(filepath.Join(c.sshLogDir, c.hostname+".stdout"))
			if err != nil {
				return trace.Wrap(err)
			}
			defer stdoutFile.Close()
			stderrFile, err := os.Create(filepath.Join(c.sshLogDir, c.hostname+".stderr"))
			if err != nil {
				return trace.Wrap(err)
			}
			defer stderrFile.Close()

			stdout = io.MultiWriter(stdout, stdoutFile)
			stderr = io.MultiWriter(stderr, stderrFile)
		}
	}

	nodeSession, err := newSession(ctx, c, nil, c.TC.newSessionEnv(), c.TC.Stdin, stdout, stderr, c.TC.EnableEscapeSequences)
	if err != nil {
		return trace.Wrap(err)
	}
	defer nodeSession.Close()
	if err := nodeSession.runCommand(ctx, types.SessionPeerMode, command, c.TC.OnChannelRequest, c.TC.OnShellCreated, c.TC.Config.InteractiveCommand); err != nil {
		originErr := trace.Unwrap(err)
		var exitErr *ssh.ExitError
		if errors.As(originErr, &exitErr) {
			c.TC.ExitStatus = exitErr.ExitStatus()
		} else {
			// if an error occurs, but no exit status is passed back, GoSSH returns
			// a generic error like this. in this case the error message is printed
			// to stderr by the remote process so we have to quietly return 1:
			if strings.Contains(originErr.Error(), "exited without exit status") {
				c.TC.ExitStatus = 1
			}
		}

		return trace.Wrap(err)
	}

	return nil
}

// AddEnv add environment variable to SSH session. This method needs to be called
// before the session is created.
func (c *NodeClient) AddEnv(key, value string) {
	if c.TC.extraEnvs == nil {
		c.TC.extraEnvs = make(map[string]string)
	}
	c.TC.extraEnvs[key] = value
}

func (c *NodeClient) handleGlobalRequests(ctx context.Context, requestCh <-chan *ssh.Request) {
	for {
		select {
		case r := <-requestCh:
			// When the channel is closing, nil is returned.
			if r == nil {
				return
			}

			switch r.Type {
			case teleport.MFAPresenceRequest:
				if c.OnMFA == nil {
					log.Warn("Received MFA presence request, but no callback was provided.")
					continue
				}

				go c.OnMFA()
			case teleport.SessionEvent:
				// Parse event and create events.EventFields that can be consumed directly
				// by caller.
				var e events.EventFields
				err := json.Unmarshal(r.Payload, &e)
				if err != nil {
					log.Warnf("Unable to parse event: %v: %v.", string(r.Payload), err)
					continue
				}

				// Send event to event channel.
				err = c.TC.SendEvent(ctx, e)
				if err != nil {
					log.Warnf("Unable to send event %v: %v.", string(r.Payload), err)
					continue
				}
			default:
				// This handles keep-alive messages and matches the behavior of OpenSSH.
				err := r.Reply(false, nil)
				if err != nil {
					log.Warnf("Unable to reply to %v request.", r.Type)
					continue
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

// newClientConn is a wrapper around ssh.NewClientConn
func newClientConn(
	ctx context.Context,
	conn net.Conn,
	nodeAddress string,
	config *ssh.ClientConfig,
) (ssh.Conn, <-chan ssh.NewChannel, <-chan *ssh.Request, error) {
	type response struct {
		conn   ssh.Conn
		chanCh <-chan ssh.NewChannel
		reqCh  <-chan *ssh.Request
		err    error
	}

	respCh := make(chan response, 1)
	go func() {
		// Use a noop text map propagator so that the tracing context isn't included in
		// the connection handshake. Since the provided conn will already include the tracing
		// context we don't want to send it again.
		conn, chans, reqs, err := tracessh.NewClientConn(ctx, conn, nodeAddress, config, tracing.WithTextMapPropagator(propagation.NewCompositeTextMapPropagator()))
		respCh <- response{conn, chans, reqs, err}
	}()

	select {
	case resp := <-respCh:
		if resp.err != nil {
			return nil, nil, nil, trace.Wrap(resp.err, "failed to connect to %q", nodeAddress)
		}
		return resp.conn, resp.chanCh, resp.reqCh, nil
	case <-ctx.Done():
		errClose := conn.Close()
		if errClose != nil {
			log.Error(errClose)
		}
		// drain the channel
		resp := <-respCh
		return nil, nil, nil, trace.ConnectionProblem(resp.err, "failed to connect to %q", nodeAddress)
	}
}

// TransferFiles transfers files over SFTP.
func (c *NodeClient) TransferFiles(ctx context.Context, cfg *sftp.Config) error {
	ctx, span := c.Tracer.Start(
		ctx,
		"nodeClient/TransferFiles",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	return trace.Wrap(cfg.TransferFiles(ctx, c.Client.Client))
}

type netDialer interface {
	DialContext(context.Context, string, string) (net.Conn, error)
}

func proxyConnection(ctx context.Context, conn net.Conn, remoteAddr string, dialer netDialer) error {
	defer conn.Close()
	defer log.Debugf("Finished proxy from %v to %v.", conn.RemoteAddr(), remoteAddr)

	var remoteConn net.Conn
	log.Debugf("Attempting to connect proxy from %v to %v.", conn.RemoteAddr(), remoteAddr)

	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		First:  100 * time.Millisecond,
		Step:   100 * time.Millisecond,
		Max:    time.Second,
		Jitter: retryutils.NewHalfJitter(),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	for attempt := 1; attempt <= 5; attempt++ {
		conn, err := dialer.DialContext(ctx, "tcp", remoteAddr)
		if err == nil {
			// Connection established, break out of the loop.
			remoteConn = conn
			break
		}

		log.Debugf("Proxy connection attempt %v: %v.", attempt, err)
		// Wait and attempt to connect again, if the context has closed, exit
		// right away.
		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case <-retry.After():
			retry.Inc()
			continue
		}
	}
	if remoteConn == nil {
		return trace.BadParameter("failed to connect to node: %v", remoteAddr)
	}
	defer remoteConn.Close()

	// Start proxying, close the connection if a problem occurs on either leg.
	return trace.Wrap(utils.ProxyConn(ctx, remoteConn, conn))
}

// acceptWithContext calls "Accept" on the listener but will unblock when the
// context is canceled.
func acceptWithContext(ctx context.Context, l net.Listener) (net.Conn, error) {
	acceptCh := make(chan net.Conn, 1)
	errorCh := make(chan error, 1)

	go func() {
		conn, err := l.Accept()
		if err != nil {
			errorCh <- err
			return
		}
		acceptCh <- conn
	}()

	select {
	case conn := <-acceptCh:
		return conn, nil
	case err := <-errorCh:
		return nil, trace.Wrap(err)
	case <-ctx.Done():
		return nil, trace.Wrap(ctx.Err())
	}
}

// listenAndForward listens on a given socket and forwards all incoming
// commands to the remote address through the SSH tunnel.
func (c *NodeClient) listenAndForward(ctx context.Context, ln net.Listener, localAddr string, remoteAddr string) {
	defer ln.Close()

	log := log.WithField("localAddr", localAddr).WithField("remoteAddr", remoteAddr)

	log.Infof("Starting port forwarding")

	for ctx.Err() == nil {
		// Accept connections from the client.
		conn, err := acceptWithContext(ctx, ln)
		if err != nil {
			if ctx.Err() == nil {
				log.WithError(err).Errorf("Port forwarding failed.")
			}
			continue
		}

		// Proxy the connection to the remote address.
		go func() {
			// `err` must be a fresh variable, hence `:=` instead of `=`.
			if err := proxyConnection(ctx, conn, remoteAddr, c.Client); err != nil {
				log.WithError(err).Warnf("Failed to proxy connection.")
			}
		}()
	}

	log.WithError(ctx.Err()).Infof("Shutting down port forwarding.")
}

// dynamicListenAndForward listens for connections, performs a SOCKS5
// handshake, and then proxies the connection to the requested address.
func (c *NodeClient) dynamicListenAndForward(ctx context.Context, ln net.Listener, localAddr string) {
	defer ln.Close()

	log := log.WithField("localAddr", localAddr)

	log.Infof("Starting dynamic port forwarding.")

	for ctx.Err() == nil {
		// Accept connection from the client. Here the client is typically
		// something like a web browser or other SOCKS5 aware application.
		conn, err := acceptWithContext(ctx, ln)
		if err != nil {
			if ctx.Err() == nil {
				log.WithError(err).Errorf("Dynamic port forwarding (SOCKS5) failed.")
			}
			continue
		}

		// Perform the SOCKS5 handshake with the client to find out the remote
		// address to proxy.
		remoteAddr, err := socks.Handshake(conn)
		if err != nil {
			log.WithError(err).Errorf("SOCKS5 handshake failed.")
			if err = conn.Close(); err != nil {
				log.WithError(err).Errorf("Error closing failed proxy connection.")
			}
			continue
		}
		log.Debugf("SOCKS5 proxy forwarding requests to %v.", remoteAddr)

		// Proxy the connection to the remote address.
		go func() {
			// `err` must be a fresh variable, hence `:=` instead of `=`.
			if err := proxyConnection(ctx, conn, remoteAddr, c.Client); err != nil {
				log.WithError(err).Warnf("Failed to proxy connection.")
				if err = conn.Close(); err != nil {
					log.WithError(err).Errorf("Error closing failed proxy connection.")
				}
			}
		}()
	}

	log.WithError(ctx.Err()).Infof("Shutting down dynamic port forwarding.")
}

// remoteListenAndForward requests a listening socket and forwards all incoming
// commands to the local address through the SSH tunnel.
func (c *NodeClient) remoteListenAndForward(ctx context.Context, ln net.Listener, localAddr, remoteAddr string) {
	defer ln.Close()
	log := log.WithField("localAddr", localAddr).WithField("remoteAddr", remoteAddr)
	log.Infof("Starting remote port forwarding")

	for ctx.Err() == nil {
		conn, err := acceptWithContext(ctx, ln)
		if err != nil {
			if ctx.Err() == nil {
				log.WithError(err).Errorf("Remote port forwarding failed.")
			}
			continue
		}

		go func() {
			if err := proxyConnection(ctx, conn, localAddr, &net.Dialer{}); err != nil {
				log.WithError(err).Warnf("Failed to proxy connection")
			}
		}()
	}
	log.WithError(ctx.Err()).Infof("Shutting down remote port forwarding.")
}

// GetRemoteTerminalSize fetches the terminal size of a given SSH session.
func (c *NodeClient) GetRemoteTerminalSize(ctx context.Context, sessionID string) (*term.Winsize, error) {
	ctx, span := c.Tracer.Start(
		ctx,
		"nodeClient/GetRemoteTerminalSize",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(attribute.String("session", sessionID)),
	)
	defer span.End()

	ok, payload, err := c.Client.SendRequest(ctx, teleport.TerminalSizeRequest, true, []byte(sessionID))
	if err != nil {
		return nil, trace.Wrap(err)
	} else if !ok {
		return nil, trace.BadParameter("failed to get terminal size")
	}

	ws := new(term.Winsize)
	err = json.Unmarshal(payload, ws)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ws, nil
}

// Close closes client and it's operations
func (c *NodeClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var errors []error
	for _, closer := range c.closers {
		errors = append(errors, closer.Close())
	}

	c.closers = nil

	errors = append(errors, c.Client.Close())

	return trace.NewAggregate(errors...)
}

// GetPaginatedSessions grabs up to 'max' sessions.
func GetPaginatedSessions(ctx context.Context, fromUTC, toUTC time.Time, pageSize int, order types.EventOrder, max int, authClient authclient.ClientI) ([]apievents.AuditEvent, error) {
	prevEventKey := ""
	var sessions []apievents.AuditEvent
	for {
		if remaining := max - len(sessions); remaining < pageSize {
			pageSize = remaining
		}
		nextEvents, eventKey, err := authClient.SearchSessionEvents(ctx, events.SearchSessionEventsRequest{
			From:     fromUTC,
			To:       toUTC,
			Limit:    pageSize,
			Order:    order,
			StartKey: prevEventKey,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		sessions = append(sessions, nextEvents...)
		if eventKey == "" || len(sessions) >= max {
			break
		}
		prevEventKey = eventKey
	}
	if max < len(sessions) {
		return sessions[:max], nil
	}
	return sessions, nil
}
