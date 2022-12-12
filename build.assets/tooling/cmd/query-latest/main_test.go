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

func TestGetLatest(t *testing.T) {
	tests := []struct {
		desc     string
		spec     string
		releases []string
		wantErr  require.ErrorAssertionFunc
		latest   string
	}{
		{
			desc: "pass",
			spec: "v8",
			releases: []string{
				"v8.1.9",
				"v8.1.10",
				"v8.0.11",
			},
			wantErr: require.NoError,
			latest:  "v8.1.10",
		},
		{
			desc: "fail-bad-spec",
			spec: "v9",
			releases: []string{
				"v8.1.9",
				"v8.1.10",
				"v8.0.11",
			},
			wantErr: require.Error,
			latest:  "",
		},
		{
			desc: "pass-prerelease",
			spec: "v8",
			releases: []string{
				"v8.1.10-rc.1",
				"v8.1.10",
				"v8.1.10-alpha.1",
			},
			wantErr: require.NoError,
			latest:  "v8.1.10",
		},
		{
			desc: "pass-major-minor",
			spec: "v8.1",
			releases: []string{
				"v8.1.9",
				"v8.2.1",
				"v8.1.10",
				"v8.0.11",
			},
			wantErr: require.NoError,
			latest:  "v8.1.10",
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			gh := &fakeGitHub{
				releases: test.releases,
			}
			latest, err := getLatest(context.Background(), test.spec, gh)
			test.wantErr(t, err)
			require.Equal(t, test.latest, latest)
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
