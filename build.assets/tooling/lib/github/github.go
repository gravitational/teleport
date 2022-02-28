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

package github

import (
	"context"

	"github.com/google/go-github/v41/github"
	"github.com/gravitational/trace"
)

// GitHub is a minimal GitHub client for ease of use
type GitHub interface {
	ListReleases(ctx context.Context, organization, repository string) ([]github.RepositoryRelease, error)
}

type ghClient struct {
	client *github.Client
}

// NewGitHub returns a new instance of the minimal GitHub client
func NewGitHub() GitHub {
	return &ghClient{
		client: github.NewClient(nil),
	}
}

// ListReleases lists all releases associated with a repository
func (c *ghClient) ListReleases(ctx context.Context, organization, repository string) (releases []github.RepositoryRelease, err error) {
	opt := &github.ListOptions{
		Page:    0,
		PerPage: 100,
	}
	for n := 0; n < 100; n++ {
		page, resp, err := c.client.Repositories.ListReleases(ctx,
			organization,
			repository,
			opt)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, r := range page {
			releases = append(releases, *r)
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return releases, nil
}
