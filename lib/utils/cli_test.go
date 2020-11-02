/*
Copyright 2019 Gravitational, Inc.

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

package utils

import (
	"crypto/x509"
	"io/ioutil"
	"strings"
	"testing"

	"gopkg.in/check.v1"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

type CLISuite struct {
}

var _ = check.Suite(&CLISuite{})

func (s *CLISuite) SetUpSuite(c *check.C) {
	InitLoggerForTests()
}
func (s *CLISuite) TearDownSuite(c *check.C) {}
func (s *CLISuite) SetUpTest(c *check.C)     {}
func (s *CLISuite) TearDownTest(c *check.C)  {}

func (s *CLISuite) TestUserMessageFromError(c *check.C) {
	tests := []struct {
		inError   error
		outString string
	}{
		{
			inError:   trace.Wrap(x509.UnknownAuthorityError{}),
			outString: "WARNING:\n\n  The proxy you are connecting to has presented a",
		},
		{
			inError:   trace.Wrap(x509.CertificateInvalidError{}),
			outString: "WARNING:\n\n  The certificate presented by the proxy is invalid",
		},
		{
			inError:   trace.Errorf("\x1b[1mWARNING\x1b[0m"),
			outString: `error: "\x1b[1mWARNING\x1b[0m"`,
		},
	}

	for i, tt := range tests {
		comment := check.Commentf("Test %v", i)
		message := UserMessageFromError(tt.inError)
		c.Assert(strings.HasPrefix(message, tt.outString), check.Equals, true, comment)
	}
}

// Regressions test - Consolef used to panic when component name was longer
// than 8 bytes.
func TestConsolefLongComponent(t *testing.T) {
	require.NotPanics(t, func() {
		component := strings.Repeat("na ", 10) + "batman!"
		Consolef(ioutil.Discard, component, "test message")
	})
}
