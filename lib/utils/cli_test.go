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

package utils

import (
	"crypto/x509"
	"fmt"
	"io"
	"log/slog"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestUserMessageFromError(t *testing.T) {
	// Behavior is different in debug
	defaultLogger := slog.Default()

	var leveler slog.LevelVar
	leveler.Set(slog.LevelInfo)
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: &leveler})))
	t.Cleanup(func() {
		slog.SetDefault(defaultLogger)
	})

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
			inError:   trace.Errorf("bad thing occurred"),
			outString: "\x1b[31mERROR: \x1b[0mbad thing occurred",
		},
	}

	for _, tt := range tests {
		message := UserMessageFromError(tt.inError)
		require.Contains(t, message, tt.outString)
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

// TestAllowWhitespace tests escape control that allows (some) whitespace characters.
func TestAllowWhitespace(t *testing.T) {
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
			in:  "\thello, world!",
			out: "\thello, world!",
		},
		{
			in:  "\t\thello, world!",
			out: "\t\thello, world!",
		},
		{
			in:  "hello, world!\n",
			out: "hello, world!\n",
		},
		{
			in:  "hello, world!\n\n",
			out: "hello, world!\n\n",
		},
		{
			in:  string([]byte{0x68, 0x00, 0x68}),
			out: "\"h\\x00h\"",
		},
		{
			in:  string([]byte{0x68, 0x08, 0x68}),
			out: "\"h\\bh\"",
		},
		{
			in:  string([]int32{0x00000008, 0x00000009, 0x00000068}),
			out: "\"\\b\"\th",
		},
		{
			in:  string([]int32{0x00000090}),
			out: "\"\\u0090\"",
		},
		{
			in:  "hello,\r\tworld!",
			out: `"hello,\r"` + "\tworld!",
		},
		{
			in:  "hello,\n\r\tworld!",
			out: "hello,\n" + `"\r"` + "\tworld!",
		},
		{
			in:  "hello,\t\n\r\tworld!",
			out: "hello,\t\n" + `"\r"` + "\tworld!",
		},
	}

	for i, tt := range tests {
		require.Equal(t, tt.out, AllowWhitespace(tt.in), fmt.Sprintf("test case %v", i))
	}
}
