/*
Copyright 2020 Gravitational, Inc.

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

package pam

import (
	"bytes"
	"os/user"
	"strings"
	"testing"

	"github.com/gravitational/teleport/lib/utils"

	"gopkg.in/check.v1"
)

type Suite struct {
	username string
}

var _ = check.Suite(&Suite{})

func TestPAM(t *testing.T) { check.TestingT(t) }

func (s *Suite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()

	// Skip this test if the binary was not built with PAM support.
	if !BuildHasPAM() || !SystemHasPAM() {
		c.Skip("Skipping test: PAM support not enabled.")
	}

	local, err := user.Current()
	c.Assert(err, check.IsNil)
	s.username = local.Username
}

// TestEcho makes sure that the teleport env variables passed to a PAM module
// are correctly set
//
// The PAM module used, pam_teleport.so is called from the policy file
// teleport-acct-echo. The policy file instructs pam_teleport.so to echo the
// contents of TELEPORT_* to stdout where this test can read, parse, and
// validate it's output.
func (s *Suite) TestEcho(c *check.C) {
	var buf bytes.Buffer
	pamContext, err := Open(&Config{
		Enabled:     true,
		ServiceName: "teleport-acct-echo",
		Login:       s.username,
		Env: map[string]string{
			"TELEPORT_USERNAME": s.username + "@example.com",
			"TELEPORT_LOGIN":    s.username,
			"TELEPORT_ROLES":    "bar baz qux",
		},
		Stdin:  &discardReader{},
		Stdout: &buf,
		Stderr: &buf,
	})
	c.Assert(err, check.IsNil)
	defer pamContext.Close()

	assertOutput(c, buf.String(), []string{
		s.username + "@example.com",
		s.username,
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
func (s *Suite) TestEnvironment(c *check.C) {
	var buf bytes.Buffer
	pamContext, err := Open(&Config{
		Enabled:     true,
		ServiceName: "teleport-session-environment",
		Login:       s.username,
		Stdin:       &discardReader{},
		Stdout:      &buf,
		Stderr:      &buf,
	})
	c.Assert(err, check.IsNil)
	defer pamContext.Close()

	c.Assert(pamContext.Environment(), check.HasLen, 1)
	c.Assert(pamContext.Environment()[0], check.Equals, "foo=bar")
}

func (s *Suite) TestSuccess(c *check.C) {
	var buf bytes.Buffer
	pamContext, err := Open(&Config{
		Enabled:     true,
		ServiceName: "teleport-success",
		Login:       s.username,
		Stdin:       &discardReader{},
		Stdout:      &buf,
		Stderr:      &buf,
	})
	c.Assert(err, check.IsNil)
	defer pamContext.Close()

	assertOutput(c, buf.String(), []string{
		"pam_sm_acct_mgmt OK",
		"pam_sm_authenticate OK",
		"pam_sm_open_session OK",
	})
}

func (s *Suite) TestAccountFailure(c *check.C) {
	var buf bytes.Buffer
	_, err := Open(&Config{
		Enabled:     true,
		ServiceName: "teleport-acct-failure",
		Login:       s.username,
		Stdin:       &discardReader{},
		Stdout:      &buf,
		Stderr:      &buf,
	})
	c.Assert(err, check.NotNil)
}

func (s *Suite) TestAuthFailure(c *check.C) {
	var buf bytes.Buffer
	_, err := Open(&Config{
		Enabled:     true,
		ServiceName: "teleport-auth-failure",
		Login:       s.username,
		Stdin:       &discardReader{},
		Stdout:      &buf,
		Stderr:      &buf,
	})
	c.Assert(err, check.NotNil)
}

func (s *Suite) TestSessionFailure(c *check.C) {
	var buf bytes.Buffer
	_, err := Open(&Config{
		Enabled:     true,
		ServiceName: "teleport-session-failure",
		Login:       s.username,
		Stdin:       &discardReader{},
		Stdout:      &buf,
		Stderr:      &buf,
	})
	c.Assert(err, check.NotNil)
}

func assertOutput(c *check.C, got string, want []string) {
	got = strings.TrimSpace(got)
	lines := strings.Split(got, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(l)
	}
	c.Assert(lines, check.DeepEquals, want)
}

type discardReader struct {
}

func (d *discardReader) Read(p []byte) (int, error) {
	return len(p), nil
}
