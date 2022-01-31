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

package bot

import (
	"context"
	"testing"

	"github.com/gravitational/teleport/.github/workflows/robot/internal/env"
	"github.com/gravitational/teleport/.github/workflows/robot/internal/github"

	"github.com/stretchr/testify/require"
)

// TestParseChanges checks that PR contents are correctly parsed for docs and
// code changes.
func TestParseChanges(t *testing.T) {
	tests := []struct {
		desc  string
		files []string
		docs  bool
		code  bool
	}{
		{
			desc: "code-only",
			files: []string{
				"file.go",
				"examples/README.md",
			},
			docs: false,
			code: true,
		},
		{
			desc: "docs-only",
			files: []string{
				"docs/docs.md",
			},
			docs: true,
			code: false,
		},
		{
			desc: "code-and-code",
			files: []string{
				"file.go",
				"docs/docs.md",
			},
			docs: true,
			code: true,
		},
		{
			desc:  "no-docs-no-code",
			files: []string{},
			docs:  false,
			code:  false,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			b := &Bot{
				c: &Config{
					Environment: &env.Environment{
						Organization: "foo",
						Repository:   "bar",
						Number:       0,
					},
					GitHub: &fakeGithub{
						test.files,
					},
				},
			}
			docs, code, err := b.parseChanges(context.Background())
			require.NoError(t, err)
			require.Equal(t, docs, test.docs)
			require.Equal(t, code, test.code)
		})
	}
}

type fakeGithub struct {
	files []string
}

func (f *fakeGithub) RequestReviewers(ctx context.Context, organization string, repository string, number int, reviewers []string) error {
	return nil
}

func (f *fakeGithub) ListReviews(ctx context.Context, organization string, repository string, number int) (map[string]*github.Review, error) {
	return nil, nil
}

func (f *fakeGithub) ListPullRequests(ctx context.Context, organization string, repository string, state string) ([]github.PullRequest, error) {
	return nil, nil
}

func (f *fakeGithub) ListFiles(ctx context.Context, organization string, repository string, number int) ([]string, error) {
	return f.files, nil
}

func (f *fakeGithub) AddLabels(ctx context.Context, organization string, repository string, number int, labels []string) error {
	return nil
}

func (f *fakeGithub) ListWorkflows(ctx context.Context, organization string, repository string) ([]github.Workflow, error) {
	return nil, nil
}

func (f *fakeGithub) ListWorkflowRuns(ctx context.Context, organization string, repository string, branch string, workflowID int64) ([]github.Run, error) {
	return nil, nil
}

func (f *fakeGithub) DeleteWorkflowRun(ctx context.Context, organization string, repository string, runID int64) error {
	return nil
}
