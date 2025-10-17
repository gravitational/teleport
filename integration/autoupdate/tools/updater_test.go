/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package tools_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/autoupdate/tools/updater"
	"github.com/gravitational/teleport/lib/autoupdate"
	"github.com/gravitational/teleport/lib/autoupdate/tools"
	"github.com/gravitational/teleport/lib/modules"
)

var (
	// pattern is template for response on version command for client tools {tsh, tctl}.
	pattern = regexp.MustCompile(`(?m)Teleport v(.*) git`)
)

// TestUpdate verifies the basic update logic. We first download a lower version, then request
// an update to a newer version, expecting it to re-execute with the updated version.
func TestUpdate(t *testing.T) {
	t.Setenv(types.HomeEnvVar, t.TempDir())
	ctx := context.Background()

	// Verify that the installed version is equal to requested one.
	cmd := exec.CommandContext(ctx, tctlPath, "version")
	out, err := cmd.Output()
	require.NoError(t, err)

	matchVersion(t, string(out), testVersions[0])

	// Execute version command again with setting the new version which must
	// trigger re-execution of the same command after downloading requested version.
	cmd = exec.CommandContext(ctx, tshPath, "version")
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("%s=%s", teleportToolsVersion, testVersions[1]),
	)
	out, err = cmd.Output()
	require.NoError(t, err)

	matchVersion(t, string(out), testVersions[1])
}

// TestUpdateDifferentOSArch verifies the update logic for matching operating system
// and architecture. If they differ from the current system, a new download must be
// initiated even when the same version is already installed.
func TestUpdateDifferentOSArch(t *testing.T) {
	home := t.TempDir()
	t.Setenv(types.HomeEnvVar, home)
	ctx := context.Background()

	// Execute version command with setting the new version which must trigger update and
	// re-execution of the same command after downloading requested version.
	cmd := exec.CommandContext(ctx, tshPath, "version")
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("%s=%s", teleportToolsVersion, testVersions[1]),
	)
	out, err := cmd.Output()
	require.NoError(t, err)
	matchVersion(t, string(out), testVersions[1])

	configPath := filepath.Join(home, "bin")

	ctc, err := tools.GetToolsConfig(configPath)
	require.NoError(t, err)
	require.Len(t, ctc.Tools, 1)
	require.Equal(t, runtime.GOOS, ctc.Tools[0].OS)
	require.Equal(t, runtime.GOARCH, ctc.Tools[0].Arch)

	// Update the architecture to a non-existing value.
	err = tools.UpdateToolsConfig(configPath, func(ctc *tools.ClientToolsConfig) error {
		ctc.Tools[0].Arch = "unknown"
		return nil
	})
	require.NoError(t, err)

	// After executing the version command, we should not match the architecture of the
	// previously installed tool version. Since the package does not match, we must
	// re-download the package for the required architecture and re-execute.
	cmd = exec.CommandContext(ctx, tshPath, "version")
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("%s=%s", teleportToolsVersion, testVersions[1]),
	)
	out, err = cmd.Output()
	require.NoError(t, err)
	matchVersion(t, string(out), testVersions[1])

	ctc, err = tools.GetToolsConfig(configPath)
	require.NoError(t, err)
	// The second call to the version command installs another package with the required
	// OS and architecture, and we should then see two packages in the list.
	require.Len(t, ctc.Tools, 2)
}

// TestParallelUpdate launches multiple updater commands in parallel while defining a new version.
// The first process should acquire a lock and block execution for the other processes. After the
// first update is complete, other processes should acquire the lock one by one and re-execute
// the command with the updated version without any new downloads.
func TestParallelUpdate(t *testing.T) {
	t.Setenv(types.HomeEnvVar, t.TempDir())
	ctx := context.Background()

	tCtx, cancel := context.WithTimeout(ctx, time.Minute)
	t.Cleanup(cancel)

	// Spawn three parallel processes with an environment variable to request a version update.
	// Only one process should initiate the update, while the other two must be locked and wait
	// until the first process finishes downloading and unpacking the update.
	outputs := make([]bytes.Buffer, 3)
	errChan := make(chan error, 3)
	for i := range outputs {
		cmd := exec.CommandContext(tCtx, tshPath, "version")
		cmd.Stdout = &outputs[i]
		cmd.Stderr = &outputs[i]
		cmd.Env = append(
			os.Environ(),
			fmt.Sprintf("%s=%s", teleportToolsVersion, testVersions[1]),
		)
		err := cmd.Start()
		require.NoError(t, err, "failed to start updater")

		go func(cmd *exec.Cmd) {
			errChan <- cmd.Wait()
		}(cmd)
	}

	// Wait till process finished with exit code 0, but we still should get progress
	// bar in output content.
	for range cap(outputs) {
		require.NoError(t, <-errChan)
	}

	// Verify the output of all spawned processes to ensure that only one process
	// indicates the client tools were updating. As a result, all outputs must show
	// the updated version: the first process performs the update and re-executes,
	// while the other two wait until the first process finishes before re-executing
	// to the desired version.
	var progressCount int
	for i := range cap(outputs) {
		matchVersion(t, outputs[i].String(), testVersions[1])
		if strings.Contains(outputs[i].String(), "Update progress:") {
			progressCount++
		}
	}
	assert.Equal(t, 1, progressCount, "we should have only one progress bar downloading new version")
}

// TestUpdateInterruptSignal verifies the interrupt signal send to the process must stop downloading.
func TestUpdateInterruptSignal(t *testing.T) {
	t.Setenv(types.HomeEnvVar, t.TempDir())

	var output bytes.Buffer
	multiOut := io.MultiWriter(&output, os.Stdout)
	cmd := newCommand(tshPath, "version")
	cmd.Stdout = multiOut
	cmd.Stderr = multiOut
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("%s=%s", teleportToolsVersion, testVersions[1]),
	)
	err := cmd.Start()
	if err != nil {
		t.Log(output.String())
	}
	require.NoError(t, err, "failed to start updater")
	pid := cmd.Process.Pid

	errChan := make(chan error)
	go func() {
		errChan <- cmd.Wait()
	}()

	// By setting the limit request next test http serving file going blocked until unlock is sent.
	lock := make(chan struct{})
	limitedWriter.SetLimitRequest(limitRequest{
		limit: 1024,
		lock:  lock,
	})

	select {
	case err := <-errChan:
		require.Fail(t, "we shouldn't receive any error", err)
	case <-time.After(10 * time.Second):
		require.Fail(t, "failed to wait till the download is started")
	case <-lock:
		time.Sleep(100 * time.Millisecond)
		t.Logf("sending signal to updater, pid: %d, test pid: %d", pid, os.Getpid())
		err := sendInterrupt(pid)
		require.NoError(t, err, "failed to send signal to updater")
		time.Sleep(100 * time.Millisecond)
		lock <- struct{}{}
	}

	// Wait till process finished with exit code 0, but we still should get progress
	// bar in output content.
	select {
	case <-time.After(10 * time.Second):
		require.Fail(t, "failed to wait till the process interrupted")
	case err := <-errChan:
		require.NoError(t, err)
	}
	assert.Contains(t, output.String(), "Update progress:")

	matches := pattern.FindStringSubmatch(output.String())
	require.Len(t, matches, 2)
	require.Equal(t, testVersions[0], matches[1])
}

// TestUpdateForOSSBuild verifies the update logic for AGPL editions of Teleport requires
// base URL environment variable.
func TestUpdateForOSSBuild(t *testing.T) {
	t.Setenv(types.HomeEnvVar, t.TempDir())
	ctx := context.Background()

	// Enable OSS build.
	t.Setenv(updater.TestBuild, modules.BuildOSS)
	t.Setenv(autoupdate.BaseURLEnvVar, "")

	// Verify that requested update is ignored by OSS build and version wasn't updated.
	cmd := exec.CommandContext(ctx, tshPath, "version")
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("%s=%s", teleportToolsVersion, testVersions[1]),
	)
	out, err := cmd.Output()
	if err != nil {
		t.Log(string(out))
	}
	require.NoError(t, err)

	matchVersion(t, string(out), testVersions[0])

	// Next update is set with the base URL env variable, must download new version.
	t.Setenv(autoupdate.BaseURLEnvVar, baseURL)
	cmd = exec.CommandContext(ctx, tshPath, "version")
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("%s=%s", teleportToolsVersion, testVersions[1]),
	)
	out, err = cmd.Output()
	require.NoError(t, err)

	matchVersion(t, string(out), testVersions[1])
}

func matchVersion(t *testing.T, output string, version string) {
	t.Helper()
	matches := pattern.FindStringSubmatch(output)
	require.Len(t, matches, 2)
	require.Equal(t, version, matches[1])
}
