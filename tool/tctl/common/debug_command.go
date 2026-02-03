// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package common

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	debugpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/debug/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	debugclient "github.com/gravitational/teleport/lib/client/debug"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// DebugCommand implements `tctl debug` group of commands.
type DebugCommand struct {
	config *servicecfg.Config

	serverID string

	getLogLevel *kingpin.CmdClause
	setLogLevel *kingpin.CmdClause
	logLevel    string

	profile        *kingpin.CmdClause
	profileSeconds int
	profileNames   []string

	readyz             *kingpin.CmdClause
	metrics            *kingpin.CmdClause
	logStream          *kingpin.CmdClause
	logStreamLevel     string
	logStreamComponent string

	// stdout and stderr for output. If nil, defaults to os.Stdout/os.Stderr.
	stdout     io.Writer
	stderr     io.Writer
	profileDir string
}

func (c *DebugCommand) getStdout() io.Writer {
	if c.stdout != nil {
		return c.stdout
	}
	return os.Stdout
}

func (c *DebugCommand) getStderr() io.Writer {
	if c.stderr != nil {
		return c.stderr
	}
	return os.Stderr
}

// Initialize allows DebugCommand to plug itself into the CLI parser.
func (c *DebugCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	c.config = config

	debug := app.Command("debug", "Interact with a remote Teleport instance's debug service.")

	c.getLogLevel = debug.Command("get-log-level", "Get the current log level.")
	c.getLogLevel.Arg("target", "Server ID or hostname of the target instance.").Required().StringVar(&c.serverID)

	c.setLogLevel = debug.Command("set-log-level", "Set the log level.")
	c.setLogLevel.Arg("target", "Server ID or hostname of the target instance.").Required().StringVar(&c.serverID)
	c.setLogLevel.Arg("level", "Log level to set (e.g. DEBUG, INFO, WARN, ERROR).").Required().StringVar(&c.logLevel)

	c.logStream = debug.Command("log-stream", "Stream live logs from the instance.")
	c.logStream.Arg("target", "Server ID or hostname of the target instance.").Required().StringVar(&c.serverID)
	c.logStream.Flag("level", "Minimum log level to stream (TRACE, DEBUG, INFO, WARN, ERROR).").StringVar(&c.logStreamLevel)
	c.logStream.Flag("component", "Component filter (comma-separated, glob patterns, case-insensitive).").StringVar(&c.logStreamComponent)

	c.profile = debug.Command("profile", "Collect pprof profiles.")
	c.profile.Arg("target", "Server ID or hostname of the target instance.").Required().StringVar(&c.serverID)
	c.profile.Flag("seconds", "Duration of CPU/trace profiles in seconds.").Default("30").IntVar(&c.profileSeconds)
	c.profile.Arg("profiles", "Profile names to collect.").Default("goroutine", "heap", "profile").StringsVar(&c.profileNames)

	c.readyz = debug.Command("readyz", "Check readiness of the instance.")
	c.readyz.Arg("target", "Server ID or hostname of the target instance.").Required().StringVar(&c.serverID)

	c.metrics = debug.Command("metrics", "Fetch Prometheus metrics.")
	c.metrics.Arg("target", "Server ID or hostname of the target instance.").Required().StringVar(&c.serverID)
}

// TryRun takes the CLI command as an argument and executes it.
func (c *DebugCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	var commandFunc func(ctx context.Context, client *authclient.Client) error
	switch cmd {
	case c.getLogLevel.FullCommand():
		commandFunc = c.runGetLogLevel
	case c.setLogLevel.FullCommand():
		commandFunc = c.runSetLogLevel
	case c.logStream.FullCommand():
		commandFunc = c.runLogStream
	case c.profile.FullCommand():
		commandFunc = c.runProfile
	case c.readyz.FullCommand():
		commandFunc = c.runReadyz
	case c.metrics.FullCommand():
		commandFunc = c.runMetrics
	default:
		return false, nil
	}

	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	defer closeFn(ctx)

	// Resolve hostname to server ID if needed.
	if err := c.resolveServerID(ctx, client); err != nil {
		return true, trace.Wrap(err)
	}

	err = commandFunc(ctx, client)
	return true, trace.Wrap(err)
}

// debugClient opens a tunneled HTTP connection to the target node's debug
// service via the auth server's Connect RPC and returns a debug.Client that
// speaks HTTP over the tunnel. The caller must call the returned cleanup
// function when done.
func (c *DebugCommand) debugClient(ctx context.Context, client *authclient.Client) (*debugclient.Client, func(), error) {
	grpcClt := debugpb.NewDebugServiceClient(client.GetConnection())
	stream, err := grpcClt.Connect(ctx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Send the target server ID as the first frame.
	if err := stream.Send(&debugpb.Frame{
		Payload: &debugpb.Frame_ServerId{ServerId: c.serverID},
	}); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	conn := newStreamConn(stream)
	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(context.Context, string, string) (net.Conn, error) {
				return conn, nil
			},
			// Use a single connection for all requests (the tunnel).
			MaxConnsPerHost:   1,
			DisableKeepAlives: false,
		},
	}

	debugClt := debugclient.NewClientWithHTTPClient(httpClient)
	cleanup := func() {
		stream.CloseSend()
	}
	return debugClt, cleanup, nil
}

func (c *DebugCommand) runGetLogLevel(ctx context.Context, client *authclient.Client) error {
	debugClt, cleanup, err := c.debugClient(ctx, client)
	if err != nil {
		return trace.Wrap(err)
	}
	defer cleanup()
	return debugclient.WriteLogLevel(ctx, c.getStdout(), debugClt)
}

func (c *DebugCommand) runSetLogLevel(ctx context.Context, client *authclient.Client) error {
	debugClt, cleanup, err := c.debugClient(ctx, client)
	if err != nil {
		return trace.Wrap(err)
	}
	defer cleanup()
	return debugclient.WriteSetLogLevel(ctx, c.getStdout(), debugClt, c.logLevel)
}

func (c *DebugCommand) runProfile(ctx context.Context, client *authclient.Client) error {
	debugClt, cleanup, err := c.debugClient(ctx, client)
	if err != nil {
		return trace.Wrap(err)
	}
	defer cleanup()

	for _, name := range c.profileNames {
		fmt.Fprintf(c.getStderr(), "Collecting profile %q...\n", name)

		data, err := debugClt.CollectProfile(ctx, name, c.profileSeconds)
		if err != nil {
			return trace.Wrap(err, "collecting profile %q", name)
		}

		filename := fmt.Sprintf("%s-%s.pb.gz", c.serverID, name)
		if c.profileDir != "" {
			filename = filepath.Join(c.profileDir, filename)
		}
		if err := os.WriteFile(filename, data, 0o600); err != nil {
			return trace.Wrap(err, "writing profile %q", name)
		}
		fmt.Fprintf(c.getStderr(), "Saved %s (%d bytes)\n", filename, len(data))
	}
	return nil
}

func (c *DebugCommand) runReadyz(ctx context.Context, client *authclient.Client) error {
	debugClt, cleanup, err := c.debugClient(ctx, client)
	if err != nil {
		return trace.Wrap(err)
	}
	defer cleanup()
	return debugclient.WriteReadiness(ctx, c.getStdout(), debugClt)
}

func (c *DebugCommand) runMetrics(ctx context.Context, client *authclient.Client) error {
	debugClt, cleanup, err := c.debugClient(ctx, client)
	if err != nil {
		return trace.Wrap(err)
	}
	defer cleanup()
	return debugclient.WriteMetrics(ctx, c.getStdout(), debugClt)
}

func (c *DebugCommand) runLogStream(ctx context.Context, client *authclient.Client) error {
	var backoff time.Duration
	for {
		err := c.doLogStream(ctx, client)
		if ctx.Err() != nil {
			return nil
		}
		if err == nil {
			backoff = 0
		}
		backoff = max(min(backoff*2, 10*time.Second), time.Second)
		fmt.Fprintf(c.getStderr(), "Connection lost: %v\nReconnecting in %s...\n", err, backoff)
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return nil
		}
	}
}

func (c *DebugCommand) doLogStream(ctx context.Context, client *authclient.Client) error {
	debugClt, cleanup, err := c.debugClient(ctx, client)
	if err != nil {
		return trace.Wrap(err)
	}
	defer cleanup()

	body, err := debugClt.GetLogStream(ctx, c.logStreamLevel)
	if err != nil {
		return trace.Wrap(err)
	}
	defer body.Close()

	patterns := c.parseComponentPatterns()
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Apply component filter if set.
		if len(patterns) > 0 {
			var obj map[string]any
			if json.Unmarshal(line, &obj) == nil {
				component, _ := obj["component"].(string)
				if !matchesComponent(component, patterns) {
					continue
				}
			}
		}

		if _, err := c.getStdout().Write(append(line, '\n')); err != nil {
			return trace.Wrap(err)
		}
	}
	return trace.Wrap(scanner.Err())
}

// parseComponentPatterns splits the comma-separated --component flag
// into lowercase glob patterns.
func (c *DebugCommand) parseComponentPatterns() []string {
	var patterns []string
	for p := range strings.SplitSeq(c.logStreamComponent, ",") {
		if p = strings.TrimSpace(p); p != "" {
			patterns = append(patterns, strings.ToLower(p))
		}
	}
	return patterns
}

// matchesComponent returns true if the component matches any pattern.
func matchesComponent(component string, patterns []string) bool {
	lower := strings.ToLower(component)
	for _, p := range patterns {
		if matched, _ := path.Match(p, lower); matched {
			return true
		}
	}
	return false
}

// resolveServerID resolves c.serverID from a hostname to an actual server ID
// if the value doesn't look like a UUID. If the hostname matches multiple
// instances, it returns an error listing the ambiguous matches.
func (c *DebugCommand) resolveServerID(ctx context.Context, client *authclient.Client) error {
	// If it looks like a UUID, assume it's already a server ID.
	if looksLikeUUID(c.serverID) {
		return nil
	}

	hostname := c.serverID
	instances := client.GetInstances(ctx, types.InstanceFilter{})

	type match struct {
		serverID string
		hostname string
		services []string
	}
	var matches []match

	for instances.Next() {
		inst := instances.Item()
		if strings.EqualFold(inst.GetHostname(), hostname) {
			services := make([]string, 0, len(inst.GetServices()))
			for _, s := range inst.GetServices() {
				services = append(services, string(s))
			}
			matches = append(matches, match{
				serverID: inst.GetName(),
				hostname: inst.GetHostname(),
				services: services,
			})
		}
	}
	if err := instances.Done(); err != nil {
		return trace.Wrap(err)
	}

	switch len(matches) {
	case 0:
		return trace.NotFound("no instance found with hostname or server ID %q", hostname)
	case 1:
		c.serverID = matches[0].serverID
		return nil
	default:
		var buf strings.Builder
		fmt.Fprintf(&buf, "hostname %q matches multiple instances; use server ID instead:\n", hostname)
		for _, m := range matches {
			fmt.Fprintf(&buf, "  %s (%s) [%s]\n", m.serverID, m.hostname, strings.Join(m.services, ", "))
		}
		return trace.BadParameter("%s", buf.String())
	}
}

// looksLikeUUID returns true if s looks like a UUID (xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx).
func looksLikeUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
	}
	return true
}

// streamConn wraps a gRPC bidirectional stream as a net.Conn so that an
// http.Client can speak HTTP over the tunnel.
type streamConn struct {
	stream debugpb.DebugService_ConnectClient
	buf    bytes.Buffer
}

func newStreamConn(stream debugpb.DebugService_ConnectClient) *streamConn {
	return &streamConn{stream: stream}
}

func (c *streamConn) Read(p []byte) (int, error) {
	if c.buf.Len() > 0 {
		return c.buf.Read(p)
	}
	frame, err := c.stream.Recv()
	if err != nil {
		return 0, err
	}
	data := frame.GetData()
	n := copy(p, data)
	if n < len(data) {
		c.buf.Write(data[n:])
	}
	return n, nil
}

func (c *streamConn) Write(p []byte) (int, error) {
	cp := make([]byte, len(p))
	copy(cp, p)
	if err := c.stream.Send(&debugpb.Frame{
		Payload: &debugpb.Frame_Data{Data: cp},
	}); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *streamConn) Close() error                       { return c.stream.CloseSend() }
func (c *streamConn) LocalAddr() net.Addr                { return &net.TCPAddr{} }
func (c *streamConn) RemoteAddr() net.Addr               { return &net.TCPAddr{} }
func (c *streamConn) SetDeadline(time.Time) error        { return nil }
func (c *streamConn) SetReadDeadline(time.Time) error    { return nil }
func (c *streamConn) SetWriteDeadline(time.Time) error   { return nil }
