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
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func FuzzParseProxyJump(f *testing.F) {
	f.Add("@:,")
	f.Add("user@host:port,bob@host:port")

	f.Fuzz(func(t *testing.T, in string) {
		require.NotPanics(t, func() {
			ParseProxyJump(in)
		})
	})
}

func FuzzParseWebLinks(f *testing.F) {
	f.Add("|")
	f.Add(",")
	f.Add("<foo>|<bar>")
	f.Add("<foo>,<bar>")
	f.Add(`<foo>; rel="next"|<bar>; rel="prev"`)
	f.Add(`<foo>; rel="first",<bar>; rel="last"`)

	f.Fuzz(func(t *testing.T, s string) {
		links := strings.Split(s, "|")
		require.NotPanics(t, func() {
			inResponse := &http.Response{
				Header: http.Header{
					"Link": links,
				},
			}
			ParseWebLinks(inResponse)
		})
	})
}

func FuzzReadYAML(f *testing.F) {
	f.Add([]byte("name: Example\nage: 30\nskills:\n  - Python\n  - JavaScript"))
	f.Add([]byte("---\nname: Document1\n---\nname: Document2"))
	f.Add([]byte(`name: "John Doe`))
	f.Add([]byte("level1:\n  level2:\n    level3:\n      level4:\n        key: value"))
	f.Add([]byte("default: &DEFAULT\n  name: Example\n  age: 30\nperson1:\n  <<: *DEFAULT\n  name: Another Example"))
	f.Add([]byte("123:\n  name: Example"))

	f.Fuzz(func(t *testing.T, dataBytes []byte) {
		data := bytes.NewReader(dataBytes)

		require.NotPanics(t, func() {
			_, _ = ReadYAML(data)
		})
	})
}
