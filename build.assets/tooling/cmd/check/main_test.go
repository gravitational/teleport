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
	"context"
	"testing"

	"github.com/google/go-github/v41/github"
	"github.com/stretchr/testify/require"
)

func TestCheckIsBareRelease(t *testing.T) {
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
		{
			desc:    "fail-leading-zero",
			tag:     "v13.3.3-tcsc.02",
			wantErr: require.Error,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			test.wantErr(t, checkIsBareRelease(test.tag))
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
