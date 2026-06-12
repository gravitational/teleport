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
