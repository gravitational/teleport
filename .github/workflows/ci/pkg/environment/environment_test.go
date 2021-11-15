/*
Copyright 2021 Gravitational, Inc.
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
package environment

import (
	"context"
	"os"
	"testing"

	"github.com/google/go-github/v37/github"
	"github.com/gravitational/teleport/.github/workflows/ci"

	"github.com/stretchr/testify/require"
)

func TestSetPullRequest(t *testing.T) {
	tests := []struct {
		checkErr require.ErrorAssertionFunc
		env      *PullRequestEnvironment
		input    []byte
		desc     string
		value    *Metadata
		action   string
	}{
		{
			env:      &PullRequestEnvironment{},
			checkErr: require.NoError,
			input:    []byte(synchronize),
			value: &Metadata{Author: "quinqu",
				RepoName:   "gh-actions-poc",
				RepoOwner:  "gravitational",
				Number:     28,
				HeadSHA:    "ecabd9d97b218368ea47d17cd23815590b76e196",
				BaseSHA:    "cbb23161d4c33d70189430d07957d2d66d42fc30",
				BranchName: "jane/ci",
			},
			desc:   "sync, no error",
			action: ci.Synchronize,
		},
		{
			env:      &PullRequestEnvironment{},
			checkErr: require.NoError,
			input:    []byte(pullRequest),
			value: &Metadata{Author: "Codertocat",
				RepoName:   "Hello-World",
				RepoOwner:  "Codertocat",
				Number:     2,
				HeadSHA:    "ec26c3e57ca3a959ca5aad62de7213c562f8c821",
				BaseSHA:    "f95f852bd8fca8fcc58a9a2d6c842781e32a215e",
				BranchName: "changes",
			},
			desc:   "pull request, no error",
			action: ci.Opened,
		},
		{
			env:      &PullRequestEnvironment{action: "submitted"},
			checkErr: require.NoError,
			input:    []byte(submitted),
			value: &Metadata{Author: "Codertocat",
				RepoName:   "Hello-World",
				RepoOwner:  "Codertocat",
				Number:     2,
				HeadSHA:    "ec26c3e57ca3a959ca5aad62de7213c562f8c821",
				BaseSHA:    "f95f852bd8fca8fcc58a9a2d6c842781e32a215e",
				BranchName: "changes",
				Reviewer:   "Codertocat",
			},
			desc:   "review, no error",
			action: ci.Submitted,
		},

		{
			env:      &PullRequestEnvironment{},
			checkErr: require.Error,
			input:    []byte(submitted),
			value:    nil,
			desc:     "sync, error",
			action:   ci.Synchronize,
		},
		{
			env:      &PullRequestEnvironment{},
			checkErr: require.Error,
			input:    []byte(submitted),
			value:    nil,
			desc:     "pull request, error",
			action:   ci.Opened,
		},
		{
			env:      &PullRequestEnvironment{},
			checkErr: require.Error,
			input:    []byte(pullRequest),
			value:    nil,
			desc:     "review, error",
			action:   ci.Submitted,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			pr, err := getMetadata(test.input, test.action)
			test.checkErr(t, err)
			require.Equal(t, test.value, pr)
		})
	}

}

func TestGetReviewersForAuthors(t *testing.T) {
	testReviewerMap := map[string][]string{
		"*":   {"foo"},
		"foo": {"bar", "baz"},
	}
	tests := []struct {
		env      *PullRequestEnvironment
		desc     string
		user     string
		expected []string
	}{
		{
			env:      &PullRequestEnvironment{HasDocsChanges: true, HasCodeChanges: true, reviewers: testReviewerMap},
			desc:     "pull request has both code and docs changes",
			user:     "foo",
			expected: []string{"klizhentas", "bar", "baz"},
		},
		{
			env:      &PullRequestEnvironment{HasDocsChanges: false, HasCodeChanges: true, reviewers: testReviewerMap},
			desc:     "pull request has only code changes",
			user:     "foo",
			expected: []string{"bar", "baz"},
		},
		{
			env:      &PullRequestEnvironment{HasDocsChanges: true, HasCodeChanges: false, reviewers: testReviewerMap},
			desc:     "pull request has only docs changes",
			user:     "foo",
			expected: []string{"klizhentas"},
		},
		{
			env:      &PullRequestEnvironment{HasDocsChanges: false, HasCodeChanges: false, reviewers: testReviewerMap},
			desc:     "pull request has no changes",
			user:     "foo",
			expected: []string{"bar", "baz"},
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			reviewerSlice := test.env.GetReviewersForAuthor(test.user)
			require.Equal(t, test.expected, reviewerSlice)
		})
	}
}

func TestHasDocsChanges(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{input: "docs/some-file.txt", expected: true},
		{input: "lib/auth/auth.go", expected: false},
		{input: "lib/some-file.mdx", expected: true},
		{input: "some/random/path.md", expected: true},
		{input: "rfd/new-proposal.txt", expected: true},
		{input: "doc/file.txt", expected: false},
		{input: "", expected: false},
		{input: "vendor/file.md", expected: false},
	}

	for _, test := range tests {
		result := hasDocChanges(test.input)
		require.Equal(t, test.expected, result)
	}
}

func TestCheckAndSetDefaults(t *testing.T) {
	testReviewerMapValid := map[string][]string{
		"*":   {"foo"},
		"foo": {"bar", "baz"},
	}
	testReviewerMapInvalid := map[string][]string{
		"foo": {"bar", "baz"},
	}

	client := github.NewClient(nil)
	ctx := context.Background()
	os.Setenv(ci.GithubEventPath, "path/to/event.json")
	tests := []struct {
		cfg      Config
		desc     string
		expected Config
		checkErr require.ErrorAssertionFunc
	}{
		{
			cfg:      Config{Client: nil, Reviewers: testReviewerMapValid, Context: ctx, EventPath: "test/path"},
			desc:     "Invalid config, Client is nil.",
			expected: Config{Client: nil, Reviewers: testReviewerMapValid, Context: ctx, EventPath: "test/path"},
			checkErr: require.Error,
		},
		{
			cfg:      Config{Client: client, Reviewers: testReviewerMapInvalid, Context: ctx, EventPath: "test/path"},
			desc:     "Invalid config, invalid Reviewer map, missing wildcard key.",
			expected: Config{Client: client, Reviewers: testReviewerMapInvalid, Context: ctx, EventPath: "test/path"},
			checkErr: require.Error,
		},
		{
			cfg:      Config{Client: client, Context: ctx, EventPath: "test/path"},
			desc:     "Invalid config, missing Reviewer map.",
			expected: Config{Client: client, Context: ctx, EventPath: "test/path"},
			checkErr: require.Error,
		},
		{
			cfg:      Config{Client: client, Context: ctx, Reviewers: testReviewerMapValid},
			desc:     "Valid config, EventPath not set.  ",
			expected: Config{Client: client, Context: ctx, EventPath: "path/to/event.json", Reviewers: testReviewerMapValid},
			checkErr: require.NoError,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := test.cfg.CheckAndSetDefaults()
			test.checkErr(t, err)
			require.Equal(t, test.expected, test.cfg)
		})
	}
}
