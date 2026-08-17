/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gravitational/trace"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/client"
)

// beamsDevShellQuote single-quotes a value for safe interpolation into a
// remote command line. The SSH exec path joins argv into one command string
// that the remote shell re-parses, so every non-literal value (paths, branch
// names, anything from a workspace record) must go through this.
func beamsDevShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// beamRunner runs commands on a single beam and copies files to/from it. It
// owns its TeleportClient: output-capturing calls temporarily repoint the
// client's stdio, so the client must not be shared with interactive use.
type beamRunner struct {
	cf   *CLIConf
	tc   *client.TeleportClient
	beam *beamsv1.Beam
}

func newBeamRunner(cf *CLIConf, beam *beamsv1.Beam) (*beamRunner, error) {
	tc, err := makeClient(cf)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tc.AllowHeadless = true
	return &beamRunner{cf: cf, tc: tc, beam: beam}, nil
}

// setBeam repoints the runner at a different beam (used after a handoff).
func (r *beamRunner) setBeam(beam *beamsv1.Beam) {
	r.beam = beam
}

// output runs a shell command on the beam and returns its combined stdout.
// stderr is captured separately and folded into the returned error.
func (r *beamRunner) output(ctx context.Context, command string) (string, error) {
	var stdout, stderr bytes.Buffer

	prevStdout, prevStderr, prevStdin := r.tc.Stdout, r.tc.Stderr, r.tc.Stdin
	r.tc.Stdout = &stdout
	r.tc.Stderr = &stderr
	r.tc.Stdin = bytes.NewReader(nil)
	defer func() {
		r.tc.Stdout, r.tc.Stderr, r.tc.Stdin = prevStdout, prevStderr, prevStdin
	}()

	err := r.ssh(ctx, command)
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		return stdout.String(), trace.Wrap(err, "remote command failed on beam %q: %s (%s)",
			r.beam.GetStatus().GetAlias(), firstLine(msg), firstLine(command))
	}
	return stdout.String(), nil
}

// run executes a shell command on the beam with the user's stdio attached.
func (r *beamRunner) run(ctx context.Context, command string) error {
	return trace.Wrap(r.ssh(ctx, command))
}

func (r *beamRunner) ssh(ctx context.Context, command string) error {
	if r.beam.GetStatus().GetNodeId() == "" {
		return trace.Errorf("beam %q is not ready to accept SSH connections", r.beam.GetStatus().GetAlias())
	}
	target := fmt.Sprintf("%s:0", r.beam.GetStatus().GetNodeId())
	r.tc.HostLogin = types.BeamsLogin

	// A freshly created beam's node record may not have reached the proxy
	// cache yet, and its network may still be coming up — same race
	// sshBeam() handles, so retry the same way.
	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		First:  100 * time.Millisecond,
		Step:   100 * time.Millisecond,
		Max:    time.Second,
		Jitter: retryutils.HalfJitter,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	var lastErr error
	for range 10 {
		// The command is passed as a single element: the server side joins
		// argv with spaces, so pre-quoted compound commands survive intact.
		lastErr = r.tc.SSH(ctx, []string{command}, client.WithHostAddress(target))
		if lastErr == nil {
			return nil
		}
		switch {
		case trace.IsNotFound(lastErr):
			// Cache may not have received the node write yet.
		case trace.IsConnectionProblem(lastErr):
			// Beam network may not be ready yet.
		default:
			return trace.Wrap(lastErr)
		}
		select {
		case <-ctx.Done():
			return trace.Wrap(lastErr)
		case <-retry.After():
			retry.Inc()
		}
	}
	return trace.Wrap(lastErr)
}

// upload copies a local file to the beam via SFTP.
func (r *beamRunner) upload(ctx context.Context, localPath, remotePath string) error {
	return trace.Wrap(r.sftp(ctx, localPath, r.sftpTarget(remotePath)))
}

// download copies a file from the beam to the local filesystem via SFTP.
func (r *beamRunner) download(ctx context.Context, remotePath, localPath string) error {
	return trace.Wrap(r.sftp(ctx, r.sftpTarget(remotePath), localPath))
}

func (r *beamRunner) sftp(ctx context.Context, src, dest string) error {
	return trace.Wrap(r.tc.SFTP(ctx, client.SFTPRequest{
		Sources:     []string{src},
		Destination: dest,
	}))
}

func (r *beamRunner) sftpTarget(path string) string {
	return fmt.Sprintf("%s@%s:%s", types.BeamsLogin, r.beam.GetStatus().GetNodeId(), path)
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// runLocalGit runs git in the given local directory and returns trimmed
// stdout. stderr is folded into the error.
func runLocalGit(ctx context.Context, dir string, args ...string) (string, error) {
	out, err := runLocalGitRaw(ctx, dir, args...)
	return strings.TrimSpace(out), trace.Wrap(err)
}

// runLocalGitRaw is runLocalGit without output trimming, for positional
// formats like `status --porcelain -z` where a leading space in the first
// entry's status code is significant.
func runLocalGitRaw(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", trace.Wrap(err, "git %s: %s", strings.Join(args, " "), firstLine(strings.TrimSpace(stderr.String())))
	}
	return stdout.String(), nil
}

// runLocal runs an arbitrary local command, returning trimmed stdout.
func runLocal(ctx context.Context, dir string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	// Stop macOS bsdtar from injecting AppleDouble (._*) metadata entries
	// into archives: extracted on a Linux beam they materialize as real junk
	// files that then bounce around the sync loop. Harmless elsewhere.
	cmd.Env = append(os.Environ(), "COPYFILE_DISABLE=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", trace.Wrap(err, "%s: %s", name, firstLine(strings.TrimSpace(stderr.String())))
	}
	return strings.TrimSpace(stdout.String()), nil
}
