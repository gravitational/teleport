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
	"errors"
	"io/ioutil"
	"os"
	"testing"

	"github.com/gravitational/teleport/.github/workflows/ci"

	"github.com/google/go-github/v37/github"
	"github.com/stretchr/testify/require"
)

func TestNewPullRequestEnvironment(t *testing.T) {
	pr := &Metadata{Author: "Codertocat",
		RepoName:   "Hello-World",
		RepoOwner:  "Codertocat",
		Number:     2,
		HeadSHA:    "ec26c3e57ca3a959ca5aad62de7213c562f8c821",
		BaseSHA:    "f95f852bd8fca8fcc58a9a2d6c842781e32a215e",
		BranchName: "changes",
	}
	tests := []struct {
		cfg        Config
		checkErr   require.ErrorAssertionFunc
		expected   *PullRequestEnvironment
		desc       string
		createFile bool
	}{
		{
			cfg: Config{
				Client:    github.NewClient(nil),
				EventPath: "",
				users:     &mockUserGetter{},
			},
			checkErr:   require.Error,
			desc:       "invalid PullRequestEnvironment config without Reviewers parameter",
			expected:   nil,
			createFile: true,
		},
		{
			cfg: Config{
				Client:    github.NewClient(nil),
				Reviewers: `{"foo": ["bar", "baz"], "*":["admin"]}`,
				users:     &mockUserGetter{},
			},
			checkErr: require.NoError,
			desc:     "valid PullRequestEnvironment config",
			expected: &PullRequestEnvironment{
				reviewers:        map[string][]string{"foo": {"bar", "baz"}, "*": {"admin"}},
				Client:           github.NewClient(nil),
				Metadata:         pr,
				defaultReviewers: []string{"admin"},
			},
			createFile: true,
		},
		{
			cfg: Config{
				Client:    github.NewClient(nil),
				Reviewers: `{"foo": ["bar", "baz"], "*":["admin"]}`,
				users:     &mockUserGetter{},
			},
			checkErr: require.NoError,
			desc:     "valid PullRequestEnvironment config",
			expected: &PullRequestEnvironment{
				reviewers:        map[string][]string{"foo": {"bar", "baz"}, "*": {"admin"}},
				Client:           github.NewClient(nil),
				Metadata:         pr,
				defaultReviewers: []string{"admin"},
			},
			createFile: true,
		},
		{
			cfg: Config{
				Client:    github.NewClient(nil),
				Reviewers: `{"foo": ["bar", "baz"]}`,
				users:     &mockUserGetter{},
			},
			checkErr:   require.Error,
			desc:       "invalid PullRequestEnvironment config, has no default reviewers key",
			expected:   nil,
			createFile: true,
		},
		{
			cfg: Config{
				Reviewers: `{"foo": "baz", "*":["admin"]}`,
				Client:    github.NewClient(nil),
			},
			checkErr:   require.Error,
			desc:       "invalid reviewers object format",
			expected:   nil,
			createFile: true,
		},
		{
			cfg:        Config{},
			checkErr:   require.Error,
			desc:       "invalid config with no client",
			expected:   nil,
			createFile: true,
		},
		{
			cfg: Config{
				Client:    github.NewClient(nil),
				Reviewers: `{"invalidUser": ["bar", "baz"], "*":["admin"]}`,
				users:     &mockUserGetter{},
			},
			checkErr:   require.Error,
			desc:       "invalid PullRequestEnvironment config, user does not exist",
			expected:   nil,
			createFile: true,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			if test.createFile {
				f, err := ioutil.TempFile("", "payload")
				require.NoError(t, err)
				filePath := f.Name()
				defer os.Remove(f.Name())
				_, err = f.Write([]byte(pullRequest))
				require.NoError(t, err)
				test.cfg.EventPath = filePath
			}
			env, err := New(test.cfg)
			test.checkErr(t, err)
			require.Equal(t, test.expected, env)
		})
	}
}

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

type mockUserGetter struct {
}

func (m *mockUserGetter) Get(ctx context.Context, userLogin string) (*github.User, *github.Response, error) {
	if userLogin == "invalidUser" {
		return nil, nil, errors.New("invalid user")
	}
	return nil, nil, nil
}
