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
	"math"
	"net/http"
	"time"

	"github.com/Khan/genqlient/graphql"
	progressbar "github.com/schollz/progressbar/v3"
)

const (
	perPage      = 10
	httpTimeout  = 30 * time.Second
	gitHubAPIURL = "https://api.github.com/graphql"
)

// CheckRun represents the summary of a Check run
type CheckRun struct {
	Name       string
	AppName    string
	Conclusion string
	StartedAt  time.Time
	Elapsed    time.Duration
	Permalink  string
}

// PR represents the summary of a PR
type PR struct {
	Number   int
	MergedAt time.Time
	Runs     []CheckRun
}

// Key returns complex key for this run
func (r CheckRun) Key() string {
	return r.AppName + " - " + r.Name
}

// fetch fetches last num PRs, summarizes and returns them
func fetch(num int) ([]PR, error) {
	bar := progressbar.Default(int64(num))

	var cursor *string // query cursor value
	var prs = make([]PR, 0)

	numPages := int(math.Ceil(float64(num) / float64(perPage)))

	for i := 0; i < numPages; i++ {
		currentPageLen := num - (i * perPage)
		if currentPageLen > perPage {
			currentPageLen = perPage
		}

		ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
		defer cancel()

		client := graphql.NewClient(gitHubAPIURL, &http.Client{Transport: &githubAuthTransport{wrapped: http.DefaultTransport}})
		response, err := getLatestPRs(ctx, client, *owner, *repo, currentPageLen, cursor)
		if err != nil {
			return nil, err
		}

		bar.Add(currentPageLen)

		cursor = response.Repository.PullRequests.PageInfo.StartCursor
		if cursor == nil {
			break
		}

		prs = append(prs, parsePRs(response)...)
	}

	return prs, nil
}

// parsePRs transforms current response page into []PR array
func parsePRs(response *getLatestPRsResponse) []PR {
	var prs = make([]PR, 0)

	for _, pr := range response.Repository.PullRequests.Nodes {
		var runs = make([]CheckRun, 0)

		for _, commit := range pr.Commits.Nodes {
			for _, suite := range commit.Commit.CheckSuites.Nodes {
				for _, run := range suite.CheckRuns.Nodes {
					var appName, conclusion string
					var startedAt time.Time
					var elapsed time.Duration

					if suite.App != nil {
						appName = suite.App.Name
					}

					if run.Conclusion != nil {
						conclusion = string(*run.Conclusion)
					}

					if run.StartedAt != nil {
						startedAt = *run.StartedAt
						if run.CompletedAt != nil {
							elapsed = run.CompletedAt.Sub(startedAt)
						}
					}

					runs = append(runs, CheckRun{
						Name:       run.Name,
						AppName:    appName,
						Conclusion: conclusion,
						StartedAt:  startedAt,
						Elapsed:    elapsed,
						Permalink:  run.Permalink,
					})
				}
			}
		}

		var mergedAt time.Time
		if pr.MergedAt != nil {
			mergedAt = *pr.MergedAt
		}

		prs = append(prs, PR{
			Number:   pr.Number,
			MergedAt: mergedAt,
			Runs:     runs,
		})
	}

	return prs
}
