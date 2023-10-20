/*
Copyright 2022 Gravitational, Inc.

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
