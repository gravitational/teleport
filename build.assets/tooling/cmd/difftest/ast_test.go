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

	require.Equal(t, runners[filepath.Join("testdata", "ast", "head")], map[string]string{
		"JiraSuite":   "TestJira",
		"SingleSuite": "TestSingleSuite",
	})
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
