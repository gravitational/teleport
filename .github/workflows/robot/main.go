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

package main

import (
	"context"
	"encoding/base64"
	"flag"
	"log"
	"time"

	"github.com/gravitational/teleport/.github/workflows/robot/internal/bot"
	"github.com/gravitational/teleport/.github/workflows/robot/internal/env"
	"github.com/gravitational/teleport/.github/workflows/robot/internal/github"
	"github.com/gravitational/teleport/.github/workflows/robot/internal/review"

	"github.com/gravitational/trace"
)

func main() {
	workflow, token, reviewers, err := parseFlags()
	if err != nil {
		log.Fatalf("Failed to parse flags: %v.", err)
	}

	// Cancel run if it takes longer than 1 minute.
	//
	// To re-run a job go to the Actions tab in the Github repo, go to the run
	// that failed, and click the "Re-run all jobs" button in the top right corner.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	b, err := createBot(ctx, token, reviewers)
	if err != nil {
		log.Fatalf("Failed to create bot: %v.", err)
	}

	log.Printf("Running %v.", workflow)

	switch workflow {
	case "assign":
		err = b.Assign(ctx)
	case "check":
		err = b.Check(ctx)
	case "dismiss":
		err = b.Dismiss(ctx)
	case "label":
		err = b.Label(ctx)
	default:
		err = trace.BadParameter("unknown workflow: %v", workflow)
	}
	if err != nil {
		log.Fatalf("Workflow %v failed: %v.", workflow, err)
	}

	log.Printf("Workflow %v complete.", workflow)
}

func parseFlags() (string, string, string, error) {
	var (
		workflow  = flag.String("workflow", "", "specific workflow to run [assign, check, dismiss]")
		token     = flag.String("token", "", "GitHub authentication token")
		reviewers = flag.String("reviewers", "", "reviewer assignments")
	)
	flag.Parse()

	if *workflow == "" {
		return "", "", "", trace.BadParameter("workflow missing")
	}
	if *token == "" {
		return "", "", "", trace.BadParameter("token missing")
	}
	if *reviewers == "" {
		return "", "", "", trace.BadParameter("reviewers required for assign and check")
	}

	data, err := base64.StdEncoding.DecodeString(*reviewers)
	if err != nil {
		return "", "", "", trace.Wrap(err)
	}

	return *workflow, *token, string(data), nil
}

func createBot(ctx context.Context, token string, reviewers string) (*bot.Bot, error) {
	gh, err := github.New(ctx, token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	environment, err := env.New()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	reviewer, err := review.FromString(reviewers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	b, err := bot.New(&bot.Config{
		GitHub:      gh,
		Environment: environment,
		Review:      reviewer,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return b, nil
}
