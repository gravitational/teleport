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
		err      bool
	}{
		{
			desc: "fail-old-releases",
			tag:  "v7.3.3",
			releases: []string{
				"v8.0.0",
				"v7.3.2",
				"v7.0.0",
			},
			err: true,
		},
		{
			desc: "fail-same-releases",
			tag:  "v8.0.0",
			releases: []string{
				"v8.0.0",
				"v7.3.2",
				"v7.0.0",
			},
			err: true,
		},
		{
			desc: "pass-new-releases",
			tag:  "v8.0.1",
			releases: []string{
				"v8.0.0",
				"v7.3.2",
				"v7.0.0",
			},
			err: false,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			gh := &fakeGitHub{
				releases: test.releases,
			}
			err := check(context.Background(), gh, "", "", test.tag)
			if test.err && err == nil {
				t.Fatalf("Expected an error, got nil.")
			}
			if !test.err && err != nil {
				t.Fatalf("Did not expect and error, got: %v", err)
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
