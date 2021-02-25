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

	"github.com/gravitational/trace"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gopkg.in/check.v1"
)

type CLISuite struct {
}

var _ = check.Suite(&CLISuite{})

func (s *CLISuite) TestUserMessageFromError(c *check.C) {
	c.Skip("Enable after https://drone.gravitational.io/gravitational/teleport/3517 is merged.")
	tests := []struct {
		comment   string
		inError   error
		outString string
	}{
		{
			comment:   "outputs x509-specific unknown authority message",
			inError:   trace.Wrap(x509.UnknownAuthorityError{}),
			outString: "WARNING:\n\n  The proxy you are connecting to has presented a",
		},
		{
			comment:   "outputs x509-specific invalid certificate message",
			inError:   trace.Wrap(x509.CertificateInvalidError{}),
			outString: "WARNING:\n\n  The certificate presented by the proxy is invalid",
		},
		{
			comment:   "outputs user message as provided",
			inError:   trace.Errorf("\x1b[1mWARNING\x1b[0m"),
			outString: `error: "\x1b[1mWARNING\x1b[0m"`,
		},
	}

	for _, tt := range tests {
		comment := check.Commentf(tt.comment)
		message := UserMessageFromError(tt.inError)
		c.Assert(strings.HasPrefix(message, tt.outString), check.Equals, true, comment)
	}
}

// Regressions test - Consolef used to panic when component name was longer
// than 8 bytes.
func TestConsolefLongComponent(t *testing.T) {
	require.NotPanics(t, func() {
		component := strings.Repeat("na ", 10) + "batman!"
		Consolef(ioutil.Discard, logrus.New(), component, "test message")
	})
}
