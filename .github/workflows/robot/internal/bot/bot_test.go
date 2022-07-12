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

	"github.com/gravitational/teleport/.github/workflows/robot/internal/github"

	"github.com/stretchr/testify/require"
)

// TestClassifyChanges checks that PR contents are correctly parsed for docs and
// code changes.
func TestClassifyChanges(t *testing.T) {
	tests := []struct {
		desc  string
		files []github.PullRequestFile
		docs  bool
		code  bool
	}{
		{
			desc: "code-only",
			files: []github.PullRequestFile{
				{Name: "file.go"},
				{Name: "examples/README.md"},
			},
			docs: false,
			code: true,
		},
		{
			desc: "docs-only",
			files: []github.PullRequestFile{
				{Name: "docs/docs.md"},
			},
			docs: true,
			code: false,
		},
		{
			desc: "code-and-code",
			files: []github.PullRequestFile{
				{Name: "file.go"},
				{Name: "docs/docs.md"},
			},
			docs: true,
			code: true,
		},
		{
			desc:  "no-docs-no-code",
			files: nil,
			docs:  false,
			code:  false,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			docs, code, err := classifyChanges(test.files)
			require.NoError(t, err)
			require.Equal(t, docs, test.docs)
			require.Equal(t, code, test.code)
		})
	}
}

func TestIsLargePR(t *testing.T) {
	tests := []struct {
		desc    string
		files   []github.PullRequestFile
		isLarge bool
	}{
		{
			desc: "single file large",
			files: []github.PullRequestFile{
				{Name: "file.go", Additions: 5555},
			},
			isLarge: true,
		},
		{
			desc: "single file not large",
			files: []github.PullRequestFile{
				{Name: "file.go", Additions: 5, Deletions: 2},
			},
			isLarge: false,
		},
		{
			desc: "multiple files large",
			files: []github.PullRequestFile{
				{Name: "file.go", Additions: 502, Deletions: 2},
				{Name: "file2.go", Additions: 10000, Deletions: 2000},
			},
			isLarge: true,
		},
		{
			desc: "with autogen, not large",
			files: []github.PullRequestFile{
				{Name: "file.go", Additions: 502, Deletions: 2},
				{Name: "file2.pb.go", Additions: 10000, Deletions: 2000},
				{Name: "file_pb.js", Additions: 10000, Deletions: 2000},
				{Name: "file2_pb.d.ts", Additions: 10000, Deletions: 2000},
				{Name: "webassets/12345/app.js", Additions: 10000, Deletions: 2000},
			},
			isLarge: false,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			require.Equal(t, test.isLarge, isLargePR(test.files))
		})
	}
}

type fakeGithub struct {
	files     []github.PullRequestFile
	pull      github.PullRequest
	reviewers []string
	reviews   []github.Review
}

func (f *fakeGithub) RequestReviewers(ctx context.Context, organization string, repository string, number int, reviewers []string) error {
	return nil
}

func (f *fakeGithub) DismissReviewers(ctx context.Context, organization string, repository string, number int, reviewers []string) error {
	return nil
}

func (f *fakeGithub) ListReviews(ctx context.Context, organization string, repository string, number int) ([]github.Review, error) {
	return f.reviews, nil
}

func (f *fakeGithub) ListReviewers(ctx context.Context, organization string, repository string, number int) ([]string, error) {
	return f.reviewers, nil
}

func (f *fakeGithub) GetPullRequest(ctx context.Context, organization string, repository string, number int) (github.PullRequest, error) {
	return f.pull, nil
}

func (f *fakeGithub) ListPullRequests(ctx context.Context, organization string, repository string, state string) ([]github.PullRequest, error) {
	return nil, nil
}

func (f *fakeGithub) ListFiles(ctx context.Context, organization string, repository string, number int) ([]github.PullRequestFile, error) {
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

func (f *fakeGithub) ListWorkflowJobs(ctx context.Context, organization string, repository string, runID int64) ([]github.Job, error) {
	return nil, nil
}

func (f *fakeGithub) DeleteWorkflowRun(ctx context.Context, organization string, repository string, runID int64) error {
	return nil
}

func (f *fakeGithub) CreateComment(ctx context.Context, organization string, repository string, number int, comment string) error {
	return nil
}

func (f *fakeGithub) CreatePullRequest(ctx context.Context, organization string, repository string, title string, head string, base string, body string, draft bool) (int, error) {
	return 0, nil
}
