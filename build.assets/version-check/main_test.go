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
)

func TestCheckPrerelease(t *testing.T) {
	tests := []struct {
		desc     string
		tag      string
		releases []string
		wantErr  bool
	}{
		{
			desc:    "fail-rc",
			tag:     "v9.0.0-rc.1",
			wantErr: true,
		},
		{ // this build was published to the deb repos on 2021-10-06
			desc:    "fail-debug",
			tag:     "v6.2.14-debug.4",
			wantErr: true,
		},
		{
			desc:    "fail-metadata",
			tag:     "v8.0.7+1a2b3c4d",
			wantErr: true,
		},
		{
			desc:    "pass",
			tag:     "v8.0.1",
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := checkPrerelease(test.tag)
			if test.wantErr && err == nil {
				t.Errorf("Expected an error, got nil.")
			}
			if !test.wantErr && err != nil {
				t.Errorf("Did not expect and error, got: %v", err)
			}
		})
	}

}

func TestCheckLatest(t *testing.T) {
	tests := []struct {
		desc     string
		tag      string
		releases []string
		wantErr  bool
	}{
		{
			desc: "fail-old-releases",
			tag:  "v7.3.3",
			releases: []string{
				"v8.0.0",
				"v7.3.2",
				"v7.0.0",
			},
			wantErr: true,
		},
		{
			desc: "fail-same-releases",
			tag:  "v8.0.0",
			releases: []string{
				"v8.0.0",
				"v7.3.2",
				"v7.0.0",
			},
			wantErr: true,
		},
		{
			desc: "pass-new-releases",
			tag:  "v8.0.1",
			releases: []string{
				"v8.0.0",
				"v7.3.2",
				"v7.0.0",
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			gh := &fakeGitHub{
				releases: test.releases,
			}
			err := checkLatest(context.Background(), test.tag, gh)
			if test.wantErr && err == nil {
				t.Errorf("Expected an error, got nil.")
			}
			if !test.wantErr && err != nil {
				t.Errorf("Did not expect and error, got: %v", err)
			}
		})
	}

}

type fakeGitHub struct {
	releases []string
}

func (f *fakeGitHub) ListReleases(ctx context.Context, organization string, repository string) ([]string, error) {
	return f.releases, nil
}
