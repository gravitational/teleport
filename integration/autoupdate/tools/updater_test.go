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
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

	// Fetch compiled test binary with updater logic and install to $TELEPORT_HOME.
	updater := tools.NewUpdater(
		toolsDir,
		testVersions[0],
		tools.WithBaseURL(baseURL),
	)
	err := updater.Update(ctx, testVersions[0])
	require.NoError(t, err)

	// Verify that the installed version is equal to requested one.
	cmd := exec.CommandContext(ctx, filepath.Join(toolsDir, "tctl"), "version")
	out, err := cmd.Output()
	require.NoError(t, err)

	matches := pattern.FindStringSubmatch(string(out))
	require.Len(t, matches, 2)
	require.Equal(t, testVersions[0], matches[1])

	// Execute version command again with setting the new version which must
	// trigger re-execution of the same command after downloading requested version.
	cmd = exec.CommandContext(ctx, filepath.Join(toolsDir, "tsh"), "version")
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("%s=%s", teleportToolsVersion, testVersions[1]),
	)
	out, err = cmd.Output()
	require.NoError(t, err)

	matches = pattern.FindStringSubmatch(string(out))
	require.Len(t, matches, 2)
	require.Equal(t, testVersions[1], matches[1])
}

// TestParallelUpdate launches multiple updater commands in parallel while defining a new version.
// The first process should acquire a lock and block execution for the other processes. After the
// first update is complete, other processes should acquire the lock one by one and re-execute
// the command with the updated version without any new downloads.
func TestParallelUpdate(t *testing.T) {
	t.Setenv(types.HomeEnvVar, t.TempDir())
	ctx := context.Background()

	// Initial fetch the updater binary un-archive and replace.
	updater := tools.NewUpdater(
		toolsDir,
		testVersions[0],
		tools.WithBaseURL(baseURL),
	)
	err := updater.Update(ctx, testVersions[0])
	require.NoError(t, err)

	// By setting the limit request next test http serving file going blocked until unlock is sent.
	lock := make(chan struct{})
	limitedWriter.SetLimitRequest(limitRequest{
		limit: 1024,
		lock:  lock,
	})

	outputs := make([]bytes.Buffer, 3)
	errChan := make(chan error, 3)
	for i := range outputs {
		cmd := exec.Command(filepath.Join(toolsDir, "tsh"), "version")
		cmd.Stdout = &outputs[i]
		cmd.Stderr = &outputs[i]
		cmd.Env = append(
			os.Environ(),
			fmt.Sprintf("%s=%s", teleportToolsVersion, testVersions[1]),
		)
		err = cmd.Start()
		require.NoError(t, err, "failed to start updater")

		go func(cmd *exec.Cmd) {
			errChan <- cmd.Wait()
		}(cmd)
	}

	select {
	case err := <-errChan:
		require.Fail(t, "we shouldn't receive any error", err)
	case <-time.After(5 * time.Second):
		require.Fail(t, "failed to wait till the download is started")
	case <-lock:
		// Wait for a short period to allow other processes to launch and attempt to acquire the lock.
		time.Sleep(100 * time.Millisecond)
		lock <- struct{}{}
	}

	// Wait till process finished with exit code 0, but we still should get progress
	// bar in output content.
	for range cap(outputs) {
		select {
		case <-time.After(5 * time.Second):
			require.Fail(t, "failed to wait till the process is finished")
		case err := <-errChan:
			require.NoError(t, err)
		}
	}

	var progressCount int
	for i := range cap(outputs) {
		matches := pattern.FindStringSubmatch(outputs[i].String())
		require.Len(t, matches, 2)
		assert.Equal(t, testVersions[1], matches[1])
		if strings.Contains(outputs[i].String(), "Update progress:") {
			progressCount++
		}
	}
	assert.Equal(t, 1, progressCount, "we should have only one progress bar downloading new version")
}

// TestUpdateInterruptSignal verifies the interrupt signal send to the process must stop downloading.
func TestUpdateInterruptSignal(t *testing.T) {
	t.Setenv(types.HomeEnvVar, t.TempDir())
	ctx := context.Background()

	// Initial fetch the updater binary un-archive and replace.
	updater := tools.NewUpdater(
		toolsDir,
		testVersions[0],
		tools.WithBaseURL(baseURL),
	)
	err := updater.Update(ctx, testVersions[0])
	require.NoError(t, err)

	var output bytes.Buffer
	cmd := exec.Command(filepath.Join(toolsDir, "tsh"), "version")
	cmd.Stdout = &output
	cmd.Stderr = &output
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("%s=%s", teleportToolsVersion, testVersions[1]),
	)
	err = cmd.Start()
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
	case <-time.After(5 * time.Second):
		require.Fail(t, "failed to wait till the download is started")
	case <-lock:
		time.Sleep(100 * time.Millisecond)
		require.NoError(t, sendInterrupt(pid))
		lock <- struct{}{}
	}

	// Wait till process finished with exit code 0, but we still should get progress
	// bar in output content.
	select {
	case <-time.After(5 * time.Second):
		require.Fail(t, "failed to wait till the process interrupted")
	case err := <-errChan:
		require.NoError(t, err)
	}
	assert.Contains(t, output.String(), "Update progress:")
}

// TestUpdateForOSSBuild verifies the update logic for AGPL editions of Teleport requires
// base URL environment variable.
func TestUpdateForOSSBuild(t *testing.T) {
	t.Setenv(types.HomeEnvVar, t.TempDir())
	ctx := context.Background()

	// Enable OSS build.
	t.Setenv(updater.TestBuild, modules.BuildOSS)

	// Fetch compiled test binary with updater logic and install to $TELEPORT_HOME.
	updater := tools.NewUpdater(
		toolsDir,
		testVersions[0],
		tools.WithBaseURL(baseURL),
	)
	err := updater.Update(ctx, testVersions[0])
	require.NoError(t, err)

	// Verify that requested update is ignored by OSS build and version wasn't updated.
	cmd := exec.CommandContext(ctx, filepath.Join(toolsDir, "tsh"), "version")
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("%s=%s", teleportToolsVersion, testVersions[1]),
	)
	out, err := cmd.Output()
	require.NoError(t, err)

	matches := pattern.FindStringSubmatch(string(out))
	require.Len(t, matches, 2)
	require.Equal(t, testVersions[0], matches[1])

	// Next update is set with the base URL env variable, must download new version.
	t.Setenv(autoupdate.BaseURLEnvVar, baseURL)
	cmd = exec.CommandContext(ctx, filepath.Join(toolsDir, "tsh"), "version")
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("%s=%s", teleportToolsVersion, testVersions[1]),
	)
	out, err = cmd.Output()
	require.NoError(t, err)

	matches = pattern.FindStringSubmatch(string(out))
	require.Len(t, matches, 2)
	require.Equal(t, testVersions[1], matches[1])
}
