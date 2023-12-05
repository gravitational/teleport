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

package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseAST(t *testing.T) {
	methods, err := parseMethodMap(filepath.Join("testdata", "ast", "head", "a_simple_test.go"), nil, nil)
	require.NoError(t, err)

	require.Len(t, methods, 3)

	require.Contains(t, methods, Method{
		Name:    "TestFirst",
		SHA1:    "378619fb4a74ce4d139e6c31a6469eae56c6ceb5",
		RefName: "TestFirst",
	})
	require.Contains(t, methods, Method{
		Name:    "TestSecond",
		SHA1:    "2df8f8035f9691e07b489e8af5a052561d3a7c85",
		RefName: "TestSecond",
	})
	require.Contains(t, methods, Method{
		Name:    "TestFourth",
		SHA1:    "035a07a1e38e5387cd682b2c6b37114d187fa3d2",
		RefName: "TestFourth",
	})
}

func TestParseASTTestifySuite(t *testing.T) {
	runners, err := findAllSuiteRunners(".", []string{filepath.Join("testdata", "ast", "head", "testify_suite_test.go")})
	require.NoError(t, err)

	methods, err := parseMethodMap(filepath.Join("testdata", "ast", "head", "testify_suite_test.go"), nil, runners)
	require.NoError(t, err)

	require.Contains(t, methods, Method{
		Name:    "TestIssueCreation",
		SHA1:    "02c613a2ecfa4fa782120dfec2178a0e8ac1d9b9",
		RefName: "TestJira/TestIssueCreation",
	})
}

func TestFindAllSuiteRunners(t *testing.T) {
	runners, err := findAllSuiteRunners(".", []string{filepath.Join("testdata", "ast", "head", "testify_single_suite_b_test.go")})
	require.NoError(t, err)

	expected := map[string]string{
		"JiraSuite":   "TestJira",
		"SingleSuite": "TestSingleSuite",
	}
	require.Equal(t, expected, runners[filepath.Join("testdata", "ast", "head")])
}

func TestParseASTTestifySuiteMulti(t *testing.T) {
	runners, err := findAllSuiteRunners(".", []string{filepath.Join("testdata", "ast", "head", "testify_single_suite_b_test.go")})
	require.NoError(t, err)

	methods, err := parseMethodMap(filepath.Join("testdata", "ast", "head", "testify_single_suite_b_test.go"), nil, runners)
	require.NoError(t, err)

	require.Contains(t, methods, Method{
		Name:    "TestIssueDeletion",
		SHA1:    "1bb6b31ec037fbd00715088e4dcccde4be8f055a",
		RefName: "TestSingleSuite/TestIssueDeletion",
	})
}
