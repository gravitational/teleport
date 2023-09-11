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

package client

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func FuzzParseProxyHost(f *testing.F) {
	for _, tc := range parseProxyHostTestCases {
		f.Add(tc.input)
	}

	f.Fuzz(func(t *testing.T, proxyHost string) {
		require.NotPanics(t, func() {
			_, _ = ParseProxyHost(proxyHost)
		})
	})
}

func FuzzParseLabelSpec(f *testing.F) {
	f.Add("XXXX=YYYY")
	f.Add(`type="database";" role"=master,ver="mongoDB v1,2"`)

	f.Fuzz(func(t *testing.T, spec string) {
		require.NotPanics(t, func() {
			_, _ = ParseLabelSpec(spec)
		})
	})
}

func FuzzParseSearchKeywords(f *testing.F) {
	f.Add("XXXX,YYYY", ',')
	f.Add(`XXXX"YYYY`, '"')
	f.Add(`"XXXX"`, '"')
	f.Add(`XXXX "YYYY" " ZZZZ  "`, ' ')
	for _, tc := range parseSearchKeywordsTestCases {
		f.Add(tc.spec, ',')
	}

	f.Fuzz(func(t *testing.T, spec string, customDelimiter rune) {
		require.NotPanics(t, func() {
			_ = ParseSearchKeywords(spec, customDelimiter)
		})
	})
}

func FuzzParsePortForwardSpec(f *testing.F) {
	f.Add("80:XXXX:180")
	f.Add("10.0.10.1:443:XXXX:1443")

	f.Fuzz(func(t *testing.T, spec string) {
		require.NotPanics(t, func() {
			_, _ = ParsePortForwardSpec([]string{spec})
		})
	})
}

func FuzzParseDynamicPortForwardSpec(f *testing.F) {
	for _, tc := range dynamicPortForwardParsingTestCases {
		if len(tc.spec) == 1 {
			f.Add(tc.spec[0])
		}
	}

	f.Fuzz(func(t *testing.T, spec string) {
		require.NotPanics(t, func() {
			_, _ = ParseDynamicPortForwardSpec([]string{spec})
		})
	})
}
