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

	"github.com/gravitational/trace"

	"github.com/google/go-github/v37/github"
)

// Assign assigns reviewers to the pull request in the
// current context.
func (a *Bot) Assign(ctx context.Context) error {
	pullReq := a.Environment.Metadata
	// Getting reviewers for author of pull request
	r := a.Environment.GetReviewersForAuthor(pullReq.Author)
	client := a.Environment.Client
	// Assigning reviewers to pull request
	_, _, err := client.PullRequests.RequestReviewers(ctx,
		pullReq.RepoOwner,
		pullReq.RepoName, pullReq.Number,
		github.ReviewersRequest{Reviewers: r})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
