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
	"context"
	"testing"
	"time"

	"github.com/google/go-github/v41/github"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestCheckPrerelease(t *testing.T) {
	tests := []struct {
		desc    string
		tag     string
		wantErr require.ErrorAssertionFunc
	}{
		{
			desc:    "fail-rc",
			tag:     "v9.0.0-rc.1",
			wantErr: require.Error,
		},
		{ // this build was published to the deb repos on 2021-10-06
			desc:    "fail-debug",
			tag:     "v6.2.14-debug.4",
			wantErr: require.Error,
		},
		{
			desc:    "fail-metadata",
			tag:     "v8.0.7+1a2b3c4d",
			wantErr: require.Error,
		},
		{
			desc:    "pass",
			tag:     "v8.0.1",
			wantErr: require.NoError,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			test.wantErr(t, checkPrerelease(test.tag))
		})
	}

}

func TestCheckLatest(t *testing.T) {
	tests := []struct {
		desc     string
		tag      string
		releases []string
		wantErr  require.ErrorAssertionFunc
	}{
		{
			desc: "fail-old-releases",
			tag:  "v7.3.3",
			releases: []string{
				"v8.0.0",
				"v7.3.2",
				"v7.0.0",
			},
			wantErr: require.Error,
		},
		{
			desc: "fail-same-releases",
			tag:  "v8.0.0",
			releases: []string{
				"v8.0.0",
				"v7.3.2",
				"v7.0.0",
			},
			wantErr: require.Error,
		},
		{
			desc: "fail-lexicographic",
			tag:  "v8.0.9",
			releases: []string{
				"v8.0.8",
				"v8.0.10",
			},
			wantErr: require.Error,
		},
		{
			desc: "pass-new-releases",
			tag:  "v8.0.1",
			releases: []string{
				"v8.0.0",
				"v7.3.2",
				"v7.0.0",
			},
			wantErr: require.NoError,
		},
		{ // see https://github.com/gravitational/teleport/issues/10800
			desc: "pass-pre-release",
			tag:  "v8.3.3",
			releases: []string{
				"v9.0.0-beta.1",
				"v8.3.2",
			},
			wantErr: require.NoError,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			gh := &fakeGitHub{releases: test.releases}
			test.wantErr(t, checkLatest(context.Background(), test.tag, gh))
		})
	}

}

type fakeGitHub struct {
	releases []string
}

func (f *fakeGitHub) ListReleases(ctx context.Context, organization, repository string) ([]github.RepositoryRelease, error) {
	ghReleases := make([]github.RepositoryRelease, 0)
	for _, r := range f.releases {
		tag := r
		ghReleases = append(ghReleases, github.RepositoryRelease{TagName: &tag})
	}
	return ghReleases, nil
}

func (f *fakeGitHub) ListWorkflowRuns(ctx context.Context, owner, repo, path, ref string, since time.Time) (map[int64]struct{}, error) {
	return nil, trace.NotImplemented("Not required for test")
}

func (f *fakeGitHub) TriggerDispatchEvent(ctx context.Context, owner, repo, path, ref string, inputs map[string]interface{}) (*github.WorkflowRun, error) {
	return nil, trace.NotImplemented("Not required for test")
}

func (f *fakeGitHub) WaitForRun(ctx context.Context, owner, repo, path, ref string, runID int64) (string, error) {
	return "", trace.NotImplemented("Not required for test")
}
