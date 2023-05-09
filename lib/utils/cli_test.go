/*
Copyright 2019-2021 Gravitational, Inc.

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
	"fmt"
	"strings"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestUserMessageFromError(t *testing.T) {
	t.Parallel()

	t.Skip("Enable after https://drone.gravitational.io/gravitational/teleport/3517 is merged.")
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
		message := UserMessageFromError(tt.inError)
		require.True(t, strings.HasPrefix(message, tt.outString), tt.comment)
	}
}

// TestEscapeControl tests escape control
func TestEscapeControl(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in  string
		out string
	}{
		{
			in:  "hello, world!",
			out: "hello, world!",
		},
		{
			in:  "hello,\nworld!",
			out: `"hello,\nworld!"`,
		},
		{
			in:  "hello,\r\tworld!",
			out: `"hello,\r\tworld!"`,
		},
	}

	for i, tt := range tests {
		require.Equal(t, tt.out, EscapeControl(tt.in), fmt.Sprintf("test case %v", i))
	}
}

// TestAllowNewlines tests escape control that allows newlines
func TestAllowNewlines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in  string
		out string
	}{
		{
			in:  "hello, world!",
			out: "hello, world!",
		},
		{
			in:  "hello,\nworld!",
			out: "hello,\nworld!",
		},
		{
			in:  "hello,\r\tworld!",
			out: `"hello,\r\tworld!"`,
		},
		{
			in:  "hello,\n\r\tworld!",
			out: "hello,\n" + `"\r\tworld!"`,
		},
		{
			in:  "hello,\t\n\r\tworld!",
			out: `"hello,\t"` + "\n" + `"\r\tworld!"`,
		},
	}

	for i, tt := range tests {
		require.Equal(t, tt.out, AllowNewlines(tt.in), fmt.Sprintf("test case %v", i))
	}
}
