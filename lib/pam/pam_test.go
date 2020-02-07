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
	"fmt"
	"os"
	"os/user"
	"strings"
	"testing"

	"github.com/gravitational/teleport/lib/utils"

	"gopkg.in/check.v1"
)

type Suite struct{}

var _ = fmt.Printf
var _ = check.Suite(&Suite{})

func TestPAM(t *testing.T) { check.TestingT(t) }

func (s *Suite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
}
func (s *Suite) TearDownSuite(c *check.C) {}
func (s *Suite) SetUpTest(c *check.C)     {}
func (s *Suite) TearDownTest(c *check.C)  {}

// TestEcho makes sure that the PAM_RUSER variable passed to a PAM module
// is correctly set
//
// The PAM module used, pam_teleport.so is called from the policy file
// teleport-session-echo-ruser. The policy file instructs pam_teleport.so to
// echo the contents of PAM_RUSER to stdout where this test can read, parse,
// and validate it's output.
func (s *Suite) TestEcho(c *check.C) {
	// Skip this test if the binary was not built with PAM support.
	if !BuildHasPAM() || !SystemHasPAM() {
		c.Skip("Skipping test: PAM support not enabled.")
	}

	local, err := user.Current()
	c.Assert(err, check.IsNil)

	os.Setenv("TELEPORT_USERNAME", local.Username+"@example.com")
	os.Setenv("TELEPORT_LOGIN", local.Username)
	os.Setenv("TELEPORT_ROLES", "bar baz qux")

	var buf bytes.Buffer
	pamContext, err := Open(&Config{
		Enabled:     true,
		ServiceName: "teleport-acct-echo",
		Login:       local.Username,
		Stdin:       &discardReader{},
		Stdout:      &buf,
		Stderr:      &buf,
	})
	c.Assert(err, check.IsNil)
	defer pamContext.Close()

	parts := strings.FieldsFunc(buf.String(), func(c rune) bool { return c == '\n' })
	c.Assert(parts, check.HasLen, 3)
	c.Assert(strings.TrimSpace(parts[0]), check.Equals, local.Username+"@example.com")
	c.Assert(strings.TrimSpace(parts[1]), check.Equals, local.Username)
	c.Assert(strings.TrimSpace(parts[2]), check.Equals, "bar baz qux")
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
	// Skip this test if the binary was not built with PAM support.
	if !BuildHasPAM() || !SystemHasPAM() {
		c.Skip("Skipping test: PAM support not enabled.")
	}

	local, err := user.Current()
	c.Assert(err, check.IsNil)

	var buf bytes.Buffer
	pamContext, err := Open(&Config{
		Enabled:     true,
		ServiceName: "teleport-session-environment",
		Login:       local.Username,
		Stdin:       &discardReader{},
		Stdout:      &buf,
		Stderr:      &buf,
	})
	c.Assert(err, check.IsNil)
	defer pamContext.Close()

	c.Assert(pamContext.Environment(), check.HasLen, 1)
	c.Assert(pamContext.Environment()[0], check.Equals, "foo=bar")
}

type discardReader struct {
}

func (d *discardReader) Read(p []byte) (int, error) {
	return len(p), nil
}
