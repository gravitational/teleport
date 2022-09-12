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
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestParseSessionsURI parses sessions URI
func TestParseSessionsURI(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		info string
		in   string
		url  *url.URL
	}{
		{info: "local default file system URI", in: "/home/log", url: &url.URL{Scheme: "file", Path: "/home/log"}},
		{info: "explicit filesystem URI", in: "file:///home/log", url: &url.URL{Scheme: "file", Path: "/home/log"}},
		{info: "other scheme", in: "other://my-bucket", url: &url.URL{Scheme: "other", Host: "my-bucket"}},
	}
	for _, testCase := range testCases {
		t.Run(testCase.info, func(t *testing.T) {
			out, err := ParseSessionsURI(testCase.in)
			require.NoError(t, err)
			require.Equal(t, testCase.url, out)
		})
	}
}
