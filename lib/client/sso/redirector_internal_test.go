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

package sso

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestClickableURL tests clickable URL conversions
func TestClickableURL(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		info string
		in   string
		out  string
	}{
		{info: "original URL is OK", in: "http://127.0.0.1:3000/hello", out: "http://127.0.0.1:3000/hello"},
		{info: "unspecified IPV6", in: "http://[::]:5050/howdy", out: "http://127.0.0.1:5050/howdy"},
		{info: "unspecified IPV4", in: "http://0.0.0.0:5050/howdy", out: "http://127.0.0.1:5050/howdy"},
		{info: "specified IPV4", in: "http://192.168.1.1:5050/howdy", out: "http://192.168.1.1:5050/howdy"},
		{info: "specified IPV6", in: "http://[2001:0db8:85a3:0000:0000:8a2e:0370:7334]:5050/howdy", out: "http://[2001:0db8:85a3:0000:0000:8a2e:0370:7334]:5050/howdy"},
		{info: "hostname", in: "http://example.com:3000/howdy", out: "http://example.com:3000/howdy"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.info, func(t *testing.T) {
			out := clickableURL(testCase.in)
			require.Equal(t, testCase.out, out)
		})
	}
}
