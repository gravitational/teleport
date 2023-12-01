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

package github

import (
	"context"

	"github.com/google/go-github/v41/github"
	"github.com/gravitational/trace"
	"golang.org/x/oauth2"
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

// NewGitHub returns a new instance of the minimal GitHub client that
// authenticates to GitHub with a Personal Access Token (or other
// oauth token).
func NewGitHubWithToken(ctx context.Context, token string) GitHub {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	authClient := oauth2.NewClient(ctx, ts)

	return &ghClient{
		client: github.NewClient(authClient),
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
