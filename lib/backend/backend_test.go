/*
Copyright 2015 Gravitational, Inc.

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

package backend

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParams(t *testing.T) {
	const (
		expectedPath  = "/usr/bin"
		expectedCount = 200
	)
	p := Params{
		"path":    expectedPath,
		"enabled": true,
		"count":   expectedCount,
	}
	path := p.GetString("path")
	if path != expectedPath {
		t.Errorf("expected 'path' to be '%v', got '%v'", expectedPath, path)
	}
}

func TestRangeEnd(t *testing.T) {
	for _, test := range []struct {
		key, expected string
	}{
		{"abc", "abd"},
		{"/foo/bar", "/foo/bas"},
		{"/xyz", "/xy{"},
		{"\xFF", "\x00"},
		{"\xFF\xFF\xFF", "\x00"},
	} {
		t.Run(test.key, func(t *testing.T) {
			end := RangeEnd([]byte(test.key))
			require.Equal(t, test.expected, string(end))
		})
	}
}

func TestParamsCleanse(t *testing.T) {
	source := Params{
		"Addr": "localhost:345",
		"TLS": map[interface{}]interface{}{
			"CAFile": "/path/to/file",
			"Certs": map[interface{}]interface{}{
				"Cert": "cert.crt",
				"Key":  "key.crt",
			},
		},
	}
	expect := Params{
		"Addr": "localhost:345",
		"TLS": map[string]interface{}{
			"CAFile": "/path/to/file",
			"Certs": map[string]interface{}{
				"Cert": "cert.crt",
				"Key":  "key.crt",
			},
		},
	}
	source.Cleanse()
	require.Equal(t, source, expect)
}
