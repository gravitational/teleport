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
	f.Fuzz(func(t *testing.T, in string) {
		require.NotPanics(t, func() {
			ParseProxyJump(in)
		})
	})
}

func FuzzParseWebLinks(f *testing.F) {
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
	f.Fuzz(func(t *testing.T, dataBytes []byte) {
		data := bytes.NewReader(dataBytes)

		require.NotPanics(t, func() {
			ReadYAML(data)
		})
	})
}
