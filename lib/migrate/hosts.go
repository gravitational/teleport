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

package migrate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	libclient "github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/identityfile"
	"github.com/gravitational/teleport/lib/migrate/hoststep"
	"github.com/gravitational/teleport/lib/modules"
	sliceutils "github.com/gravitational/teleport/lib/utils/slices"
	"github.com/gravitational/teleport/lib/web/scripts/oneoff"
)

// runResult is the outcome of one remote command.
type runResult struct {
	// Stdout is the command's standard output.
	Stdout string
	// Stderr is the command's standard error.
	Stderr string
	// ExitCode is the command's exit status.
	ExitCode int
}

// hostRunner runs commands as root on the hosts of one cluster. Secrets and
// file bodies ride in stdin, never in argv: the argv is written to that
// cluster's audit log and is visible in ps. It exists so tests can substitute
// a fake for SSH.
type hostRunner interface {
	// Run executes argv on the host with the given host UUID. A non-zero
	// remote exit is reported through runResult.ExitCode with a nil error; err
	// is only for transport failures.
	Run(ctx context.Context, hostID string, argv []string, stdin []byte) (runResult, error)
}

// shellSafe is the set of characters that never need quoting for POSIX sh.
var shellSafe = regexp.MustCompile(`^[a-zA-Z0-9_@%+=:,./-]+$`)

// sgrEscape matches the ANSI color sequences teleport writes around its
// "ERROR: " prefix on Linux.
var sgrEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// shellQuote single-quotes s unless every character is in shellSafe. It
// allowlists safe characters rather than blocklisting dangerous ones, because
// a stale blocklist passes unquoted whatever it missed.
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if shellSafe.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// hostLogin is the login every command runs as on a host.
const hostLogin = "root"

// sshHosts runs commands on one cluster's hosts over Teleport SSH. One
// TeleportClient and one ClusterClient are shared; each command dials its own
// NodeClient, because lib/client registers per-connection channel handlers
// per session.
type sshHosts struct {
	tc      *libclient.TeleportClient
	cluster *libclient.ClusterClient
}

// dialHosts reaches cluster's proxy as root with its identity file and opens
// the cluster connection every Run shares. ctx bounds the dial only; the
// connection stays open until Close.
func dialHosts(ctx context.Context, cluster ClusterConfig) (*sshHosts, error) {
	store := libclient.NewMemClientStore()
	if err := identityfile.LoadIdentityFileIntoClientStore(store, cluster.IdentityFile, cluster.ProxyServer, ""); err != nil {
		return nil, trace.Wrap(err, "loading identity %q", cluster.IdentityFile)
	}
	cfg := &libclient.Config{
		ClientStore:    store,
		WebProxyAddr:   cluster.ProxyServer,
		HostLogin:      hostLogin,
		NonInteractive: true,
		// If Stdin is left unset, CheckAndSetDefaults wires os.Stdin (the
		// operator's terminal) into every session.
		Stdin: bytes.NewReader(nil),
	}
	if err := cfg.LoadProfile(cluster.ProxyServer); err != nil {
		return nil, trace.Wrap(err, "loading the profile for %s", cluster.ProxyServer)
	}
	tc, err := libclient.NewClient(cfg)
	if err != nil {
		return nil, trace.Wrap(err, "building the SSH client for %s", cluster.ProxyServer)
	}
	// The placeholder profile lacks cluster details; Ping fills them in.
	if _, err := tc.Ping(ctx); err != nil {
		return nil, trace.Wrap(err, "pinging %s", cluster.ProxyServer)
	}
	clusterClient, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "connecting to %s", cluster.ProxyServer)
	}
	return &sshHosts{tc: tc, cluster: clusterClient}, nil
}

// maxOutput caps each output stream of a remote command, so a host that
// streams forever cannot take the tool down with it.
const maxOutput = 1 << 20

// cappedBuffer keeps the first maxOutput bytes and drops the rest without
// erroring, so the remote command still runs to completion and Run can
// report the overflow instead of a stalled session. The buffer is a named
// field: an embedded bytes.Buffer would give it a ReadFrom, which io.Copy
// (and so the SSH session) would call instead of Write.
type cappedBuffer struct {
	buf     bytes.Buffer
	dropped bool
}

// Write appends what fits under maxOutput and drops the rest, reporting the
// whole of p as written.
func (b *cappedBuffer) Write(p []byte) (int, error) {
	if room := maxOutput - b.buf.Len(); len(p) > room {
		b.dropped = true
		b.buf.Write(p[:room])
		return len(p), nil
	}
	return b.buf.Write(p)
}

// Run runs argv as root on the host with UUID hostID, reached through the
// proxy, with stdin on its standard input. A non-zero exit comes back in
// runResult.ExitCode with a nil error; output past maxOutput on either stream is
// a LimitExceeded error.
func (r *sshHosts) Run(ctx context.Context, hostID string, argv []string, stdin []byte) (runResult, error) {
	// Port 0 lets the proxy resolve the host by UUID, as tsh does.
	node, err := r.tc.ConnectToNode(ctx, r.cluster, libclient.NodeDetails{Addr: hostID + ":0", Cluster: r.cluster.ClusterName()}, hostLogin)
	if err != nil {
		return runResult{}, trace.Wrap(err)
	}
	defer node.Close()

	var stdout, stderr cappedBuffer
	// lib/client flattens argv into one string that the remote login shell
	// splits again, so every argument is quoted.
	err = node.RunCommand(ctx, sliceutils.Map(argv, shellQuote), libclient.WithOutput(&stdout, &stderr), libclient.WithInput(bytes.NewReader(stdin)))
	// A session cut short by ctx can return while its copy goroutines still
	// write the buffers, so they are read only after a session that ended.
	if err != nil && ctx.Err() != nil {
		return runResult{}, trace.Wrap(err)
	}
	result := runResult{Stdout: stdout.buf.String(), Stderr: stderr.buf.String()}
	if stdout.dropped || stderr.dropped {
		return result, trace.LimitExceeded("more than %d bytes of output", maxOutput)
	}
	var exitErr *ssh.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitStatus()
		return result, nil
	}
	return result, trace.Wrap(err)
}

// Close closes the cluster connection every Run shared.
func (r *sshHosts) Close() error { return trace.Wrap(r.cluster.Close()) }

// newHostBinary returns the oneoff script params that run this build's
// version, edition and FIPS mode of teleport from the host's cache, downloading
// it and teleport-update from baseURL first; EntrypointArgs is set per step.
// Like teleport-update, it refuses a baseURL without TLS: nothing else checks
// the binary hosts run as root.
func newHostBinary(baseURL string) (oneoff.OneOffScriptParams, error) {
	if !strings.HasPrefix(strings.ToLower(baseURL), "https://") {
		return oneoff.OneOffScriptParams{}, trace.BadParameter("TELEPORT_CDN_BASE_URL %s must use TLS (https://)", baseURL)
	}
	fips := modules.GetModules().IsFIPSBuild()
	flavor := types.PackageNameOSS
	if fips || modules.GetModules().BuildType() == modules.BuildEnterprise {
		flavor = types.PackageNameEnt
	}
	return oneoff.OneOffScriptParams{
		CDNBaseURL:      baseURL,
		TeleportVersion: "v" + teleport.Version,
		TeleportFlavor:  flavor,
		TeleportFIPS:    fips,
		Entrypoint:      "teleport",
		CacheDir:        hoststep.CacheRoot,
		// The step's JSON result is the only thing on stdout.
		LogToStderr: true,
	}, nil
}

// maxQuotedOutput bounds how much of an undecodable result an error quotes.
const maxQuotedOutput = 200

// stepExitError is a host step that ran and exited non-zero.
type stepExitError struct {
	step   string
	code   int
	stderr string
}

// Error names the step, its exit code and its stderr.
func (e *stepExitError) Error() string {
	return fmt.Sprintf("%s failed (exit %d): %s", e.step, e.code, e.stderr)
}

// runHostStep runs `teleport migrate host <step>` on hostID through the
// oneoff script, with request as JSON on stdin, and decodes the step's JSON
// output into result. A refused login stays an AccessDenied and an output
// overflow a LimitExceeded; a run cut short by ctx's deadline wraps
// context.DeadlineExceeded, and any other transport failure is a
// ConnectionProblem. A non-zero exit is a *stepExitError, its stderr without
// the oneoff script's lines before the step's own and with the CLI's colors
// and "ERROR: " prefix removed.
func (m *Migrator) runHostStep(ctx context.Context, hosts hostRunner, hostID, step string, request, result any) error {
	body, err := json.Marshal(request)
	if err != nil {
		return trace.Wrap(err)
	}
	params := m.binary
	params.EntrypointArgs = "migrate host " + step
	script, err := oneoff.BuildScript(params)
	if err != nil {
		return trace.Wrap(err)
	}
	// The step is the script's $0, which names it in the shell's own errors.
	argv := []string{"/bin/sh", "-c", script, step}
	run, err := hosts.Run(ctx, hostID, argv, body)
	switch {
	case err == nil:
	case trace.IsAccessDenied(err), trace.IsLimitExceeded(err):
		return trace.Wrap(err, "running %s", step)
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		// The run outlived its timeout, which says nothing about whether the
		// host is reachable.
		return trace.Wrap(ctx.Err(), "running %s", step)
	default:
		// Callers tell an unreachable host by this type.
		return trace.ConnectionProblem(err, "running %s: %v", step, err)
	}
	if run.ExitCode != 0 {
		stderr := run.Stderr
		// The oneoff script logs "> <command>" before running the step, so the
		// step's own stderr follows that line; a failed download has none.
		if i := strings.Index("\n"+stderr, "\n> "); i >= 0 {
			_, stderr, _ = strings.Cut(stderr[i:], "\n")
		}
		stderr = strings.TrimPrefix(strings.TrimSpace(sgrEscape.ReplaceAllString(stderr, "")), "ERROR: ")
		return trace.Wrap(&stepExitError{step: step, code: run.ExitCode, stderr: stderr})
	}
	// Decode only the step's JSON: a FIPS build's lib/client writes a screen
	// clear after the remote output when the session closes.
	if err := json.NewDecoder(strings.NewReader(run.Stdout)).Decode(result); err != nil {
		return trace.BadParameter("decoding the result of %s: %v; the host printed %q", step, err, run.Stdout[:min(len(run.Stdout), maxQuotedOutput)])
	}
	return nil
}
