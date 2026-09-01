//go:build e2e
// +build e2e

/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package main_test

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// configPath path of the configuration file used by `tctl`
	// to configure client tools managed update configuration.
	configPath = "/etc/teleport-local.yaml"
	// proxy is the cluster address used in the e2e test.
	proxy = "localhost:9443"

	tshV1  = "./test-packages/v17.5.1/tsh"
	tctlV1 = "./test-packages/v17.5.1/tctl"
	tshV2  = "./test-packages/v18.0.0/tsh"
	tctlV2 = "./test-packages/v18.0.0/tctl"
)

var (
	// pattern is template for response on version command for client tools {tsh, tctl}.
	pattern = regexp.MustCompile(`(?m)Teleport v(.*) git`)
	// user is username registered in cluster for the tests.
	user = os.Getenv("TELEPORT_TEST_USER")
	// secret used for the `user` created in cluster to generate TOTP.
	secret = os.Getenv("TELEPORT_TEST_SECRET")
	// password is default password for `user`.
	password = os.Getenv("TELEPORT_TEST_PASSWORD")
)

func TestMain(m *testing.M) {
	// If test environment variables is not set, we have to skip tests.
	if user == "" || secret == "" || password == "" {
		return
	}

	enableMU(tctlV1)

	code := m.Run()
	os.Exit(code)
}

// TestSameVersionCheck: v1 -> v2(env) -> v2(re-install) -> v2(login to same version).
func TestSameVersionCheck(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TELEPORT_CDN_BASE_URL", "http://localhost:8080")
	t.Setenv("TELEPORT_HOME", tempDir)
	t.Setenv("TELEPORT_TOOLS_DIR", filepath.Join(tempDir, ".tsh", "bin"))

	v := checkVersion(t, tshV1, []string{"TELEPORT_TOOLS_VERSION=18.5.5"})
	assert.Equal(t, "18.5.5", v)

	v = checkVersion(t, tshV1, []string{})
	assert.Equal(t, "18.5.5", v)

	// Trigger migration.
	v = checkVersion(t, tshV2, []string{})
	assert.Equal(t, "18.0.0", v)
	printConfig(t, tempDir)

	// Login to the cluster
	setClusterVersion(t, tctlV2, "18.0.0")
	time.Sleep(time.Second)
	login(t, tshV2, proxy, []string{})
	v = checkVersion(t, tshV2, []string{})
	assert.Equal(t, "18.0.0", v)

	// Show the configuration generated after test execution.
	printConfig(t, tempDir)
}

// TestV1UpgradeDowngrade: v1 -> v2(env) -> v1(login) -> v1(login).
func TestV1UpgradeDowngrade(t *testing.T) {
	proxy := "localhost:9443"
	tempDir := t.TempDir()
	t.Setenv("TELEPORT_CDN_BASE_URL", "http://localhost:8080")
	t.Setenv("TELEPORT_HOME", tempDir)
	t.Setenv("TELEPORT_TOOLS_DIR", filepath.Join(tempDir, ".tsh", "bin"))

	v := checkVersion(t, tshV1, []string{"TELEPORT_TOOLS_VERSION=18.0.0"})
	require.Equal(t, "18.0.0", v)

	// Login to the cluster
	setClusterVersion(t, tctlV1, "17.5.4")
	time.Sleep(time.Second)
	login(t, tshV1, proxy, []string{})
	v = checkVersion(t, tshV1, []string{})
	require.Equal(t, "17.5.4", v)

	// Login to the cluster.
	setClusterVersion(t, tctlV1, "17.5.7")

	login(t, tshV1, proxy, []string{})
	v = checkVersion(t, tshV1, []string{})
	require.Equal(t, "17.5.7", v)

	// Show the configuration generated after test execution.
	printConfig(t, tempDir)
}

// TestV1UpgradeToV2: v1 -> v1(env) -> v1(login) -> v2(login) -> v2(login).
func TestV1UpgradeToV2(t *testing.T) {
	proxy := "localhost:9443"
	tempDir := t.TempDir()
	t.Setenv("TELEPORT_CDN_BASE_URL", "http://localhost:8080")
	t.Setenv("TELEPORT_HOME", tempDir)
	t.Setenv("TELEPORT_TOOLS_DIR", filepath.Join(tempDir, ".tsh", "bin"))

	v := checkVersion(t, tshV1, []string{"TELEPORT_TOOLS_VERSION=17.5.4"})
	require.Equal(t, "17.5.4", v)

	setClusterVersion(t, tctlV1, "17.5.7")
	login(t, tshV1, proxy, []string{})
	v = checkVersion(t, tshV1, []string{})
	require.Equal(t, "17.5.7", v)

	// Login to the cluster
	setClusterVersion(t, tctlV1, "18.0.0")
	login(t, tshV1, proxy, []string{})
	v = checkVersion(t, tshV1, []string{})
	require.Equal(t, "18.0.0", v)

	// Login to the cluster
	setClusterVersion(t, tctlV1, "18.5.5")

	login(t, tshV1, proxy, []string{})
	v = checkVersion(t, tshV1, []string{})
	require.Equal(t, "18.5.5", v)

	// Login to the cluster
	setClusterVersion(t, tctlV1, "18.5.7")

	login(t, tshV1, proxy, []string{})
	v = checkVersion(t, tshV1, []string{})
	require.Equal(t, "18.5.7", v)

	// Show the configuration generated after test execution.
	printConfig(t, tempDir)
}

// TestV2DowngradeUpgrade: v2 -> v1(env) -> v2(login) -> v2(login).
func TestV2DowngradeUpgrade(t *testing.T) {
	proxy := "localhost:9443"
	tempDir := t.TempDir()
	t.Setenv("TELEPORT_CDN_BASE_URL", "http://localhost:8080")
	t.Setenv("TELEPORT_HOME", tempDir)
	t.Setenv("TELEPORT_TOOLS_DIR", filepath.Join(tempDir, ".tsh", "bin"))

	v := checkVersion(t, tshV2, []string{"TELEPORT_TOOLS_VERSION=17.5.4"})
	require.Equal(t, "17.5.4", v)

	// Login to the cluster
	setClusterVersion(t, tctlV2, "18.5.5")
	login(t, tshV2, proxy, []string{})
	v = checkVersion(t, tshV2, []string{})
	require.Equal(t, "18.5.5", v)

	setClusterVersion(t, tctlV2, "18.5.7")
	//logout(t, tshV2, []string{})

	// Login to the cluster
	login(t, tshV2, proxy, []string{})
	v = checkVersion(t, tshV2, []string{})
	require.Equal(t, "18.5.7", v)

	// Show the configuration generated after test execution.
	printConfig(t, tempDir)
}

// TestV2RegularUpgrade: v2 -> v2(login) -> v2(login).
func TestV2RegularUpgrade(t *testing.T) {
	proxy := "localhost:9443"
	tempDir := t.TempDir()
	t.Setenv("TELEPORT_CDN_BASE_URL", "http://localhost:8080")
	t.Setenv("TELEPORT_HOME", tempDir)
	t.Setenv("TELEPORT_TOOLS_DIR", filepath.Join(tempDir, ".tsh", "bin"))

	v := checkVersion(t, tshV2, []string{})
	require.Equal(t, "18.0.0", v)

	// Login to the cluster
	setClusterVersion(t, tctlV2, "18.5.5")
	login(t, tshV2, proxy, []string{})
	v = checkVersion(t, tshV2, []string{})
	require.Equal(t, "18.5.5", v)

	setClusterVersion(t, tctlV2, "18.5.7")
	login(t, tshV2, proxy, []string{})
	v = checkVersion(t, tshV2, []string{})
	require.Equal(t, "18.5.7", v)

	// Show the configuration generated after test execution.
	printConfig(t, tempDir)
}

func checkVersion(t *testing.T, tsh string, env []string) string {
	t.Helper()
	cmd := exec.Command(tsh, "version", "--insecure")
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.Output()
	require.NoError(t, err)
	matches := pattern.FindStringSubmatch(string(out))
	require.Len(t, matches, 2)
	fmt.Println(string(out))

	return matches[1]
}

func enableMU(tctl string) {
	cmd := exec.Command(tctl, "-c", configPath, "--insecure", "autoupdate", "client-tools", "enable")
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	if err != nil {
		fmt.Printf("Error enabling MU: %s\n", err)
	}
	fmt.Println(string(out))
}

func setClusterVersion(t *testing.T, tctl string, version string) {
	t.Helper()
	cmd := exec.Command(tctl, "-c", configPath, "--insecure", "autoupdate", "client-tools", "target", version)
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	require.NoError(t, err)
	fmt.Println(string(out))
	time.Sleep(time.Second)
}

func logout(t *testing.T, tsh string, env []string) {
	t.Helper()
	cmd := exec.Command(tsh, "logout", "--insecure")
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.Output()
	require.NoError(t, err)
	fmt.Println(string(out))
}

func login(t *testing.T, tsh, proxy string, env []string) {
	t.Helper()

	var cmdErr error
	for range 3 {
		cmd := exec.Command(tsh, "login", "--proxy", proxy, "--user", user, "--insecure", "--skip-version-check")
		cmd.Env = append(os.Environ(), env...)

		ptmx, err := pty.Start(cmd)
		require.NoError(t, err)
		defer ptmx.Close()

		scanner := bufio.NewScanner(ptmx)

		go func() {
			for scanner.Scan() {
				line := scanner.Text()
				fmt.Println(line)
				switch {
				case strings.Contains(line, fmt.Sprintf("Enter password for Teleport user %s:", user)):
					_, _ = ptmx.Write([]byte(password + "\n"))
				case strings.Contains(line, "Enter an OTP code from a device:"):
					code, err := totp.GenerateCode(secret, time.Now())
					require.NoError(t, err)
					_, _ = ptmx.Write([]byte(code + "\n"))
				}
			}
		}()

		if cmdErr = cmd.Wait(); cmdErr == nil {
			return
		}
	}
	require.NoError(t, cmdErr)
}

func printConfig(t *testing.T, tempDir string) {
	t.Helper()
	f, err := os.ReadFile(filepath.Join(tempDir, ".tsh", "bin", ".config.json"))
	if os.IsNotExist(err) {
		fmt.Println("Config file not found.")
		return
	}
	require.NoError(t, err)
	fmt.Println(string(f))
}
