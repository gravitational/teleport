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

func TestCheck(t *testing.T) {
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
			err := check(context.Background(), gh, "", "", test.tag)
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
