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

package pam

import (
	"bytes"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()

	// Skip this test if the binary was not built with PAM support.
	if !BuildHasPAM() || !SystemHasPAM() {
		fmt.Println("PAM support not enabled, skipping tests")
		os.Exit(0)
	}

	_, err := os.Stat("../../build.assets/pam/pam_teleport.so")
	if os.IsNotExist(err) {
		fmt.Println("PAM test module is not installed, you can install it with 'sudo make -C build.assets/pam install'")
		os.Exit(0)
	}

	os.Exit(m.Run())
}

// TestEcho makes sure that the teleport env variables passed to a PAM module
// are correctly set
//
// The PAM module used, pam_teleport.so is called from the policy file
// teleport-acct-echo. The policy file instructs pam_teleport.so to echo the
// contents of TELEPORT_* to stdout where this test can read, parse, and
// validate its output.
func TestEcho(t *testing.T) {
	t.Parallel()
	checkTestModule(t, "teleport-acct-echo")
	username := currentUser(t)

	var buf bytes.Buffer
	pamContext, err := Open(&servicecfg.PAMConfig{
		Enabled:     true,
		ServiceName: "teleport-acct-echo",
		Login:       username,
		Env: map[string]string{
			"TELEPORT_USERNAME": username + "@example.com",
			"TELEPORT_LOGIN":    username,
			"TELEPORT_ROLES":    "bar baz qux",
		},
		Stdin:      &discardReader{},
		Stdout:     &buf,
		Stderr:     &buf,
		UsePAMAuth: true,
	})
	require.NoError(t, err)
	defer pamContext.Close()

	assertOutput(t, buf.String(), []string{
		username + "@example.com",
		username,
		"bar baz qux",
		"pam_sm_acct_mgmt OK",
		"pam_sm_authenticate OK",
		"pam_sm_open_session OK",
	})
}

// TestEnvironment makes sure that PAM environment variables (environment
// variables set by a PAM module) can be accessed from the PAM handle/context
// in Go code.
//
// The PAM module used, pam_teleport.so is called from the policy file
// teleport-session-environment. The policy file instructs pam_teleport.so to
// read in the first argument and set it as a PAM environment variable. This
// test then validates it matches what was set in the policy file.
func TestEnvironment(t *testing.T) {
	t.Parallel()
	checkTestModule(t, "teleport-session-environment")
	username := currentUser(t)

	var buf bytes.Buffer
	pamContext, err := Open(&servicecfg.PAMConfig{
		Enabled:     true,
		ServiceName: "teleport-session-environment",
		Login:       username,
		Stdin:       &discardReader{},
		Stdout:      &buf,
		Stderr:      &buf,
	})
	require.NoError(t, err)
	defer pamContext.Close()

	require.ElementsMatch(t, pamContext.Environment(), []string{"foo=bar"})
}

func TestSuccess(t *testing.T) {
	t.Parallel()
	checkTestModule(t, "teleport-success")
	username := currentUser(t)

	var buf bytes.Buffer
	pamContext, err := Open(&servicecfg.PAMConfig{
		Enabled:     true,
		ServiceName: "teleport-success",
		Login:       username,
		Stdin:       &discardReader{},
		Stdout:      &buf,
		Stderr:      &buf,
		UsePAMAuth:  true,
	})
	require.NoError(t, err)
	defer pamContext.Close()

	assertOutput(t, buf.String(), []string{
		"pam_sm_acct_mgmt OK",
		"pam_sm_authenticate OK",
		"pam_sm_open_session OK",
	})
}

func TestAccountFailure(t *testing.T) {
	t.Parallel()
	checkTestModule(t, "teleport-acct-failure")
	username := currentUser(t)

	var buf bytes.Buffer
	_, err := Open(&servicecfg.PAMConfig{
		Enabled:     true,
		ServiceName: "teleport-acct-failure",
		Login:       username,
		Stdin:       &discardReader{},
		Stdout:      &buf,
		Stderr:      &buf,
	})
	require.Error(t, err)
}

func TestAuthFailure(t *testing.T) {
	t.Parallel()
	checkTestModule(t, "teleport-auth-failure")
	username := currentUser(t)

	var buf bytes.Buffer
	_, err := Open(&servicecfg.PAMConfig{
		Enabled:     true,
		ServiceName: "teleport-auth-failure",
		Login:       username,
		Stdin:       &discardReader{},
		Stdout:      &buf,
		Stderr:      &buf,
		UsePAMAuth:  true,
	})
	require.Error(t, err)
}

func TestAuthDisabled(t *testing.T) {
	t.Parallel()
	checkTestModule(t, "teleport-auth-failure")
	username := currentUser(t)

	var buf bytes.Buffer
	pamContext, err := Open(&servicecfg.PAMConfig{
		Enabled:     true,
		ServiceName: "teleport-auth-failure",
		Login:       username,
		Stdin:       &discardReader{},
		Stdout:      &buf,
		Stderr:      &buf,
		UsePAMAuth:  false,
	})
	require.NoError(t, err)
	defer pamContext.Close()

	assertOutput(t, buf.String(), []string{
		"pam_sm_acct_mgmt OK",
		"pam_sm_open_session OK",
	})
}

func TestSessionFailure(t *testing.T) {
	t.Parallel()
	checkTestModule(t, "teleport-session-failure")
	username := currentUser(t)

	var buf bytes.Buffer
	_, err := Open(&servicecfg.PAMConfig{
		Enabled:     true,
		ServiceName: "teleport-session-failure",
		Login:       username,
		Stdin:       &discardReader{},
		Stdout:      &buf,
		Stderr:      &buf,
	})
	require.Error(t, err)
}

func assertOutput(t *testing.T, got string, want []string) {
	got = strings.TrimSpace(got)
	lines := strings.Split(got, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(l)
	}
	require.ElementsMatch(t, lines, want)
}

type discardReader struct {
}

func (d *discardReader) Read(p []byte) (int, error) {
	return len(p), nil
}

func checkTestModule(t *testing.T, name string) {
	_, err := os.Stat(filepath.Join("/etc/pam.d", name))
	if os.IsNotExist(err) {
		t.Skipf("PAM test service %q is not installed, you can install it with 'sudo make -C build.assets/pam install'", name)
	}
}

func currentUser(t *testing.T) string {
	usr, err := user.Current()
	require.NoError(t, err)
	return usr.Username
}
