// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Command trigger-workflow triggers a GigHub Actions workflow in a given
// repository and awaits the result. The target workflow must repond to a
// repository_dispatch event.
//
// WARNING: this tool can only handle waiting for a single workflow
// spawned by repository_dispatch event. If multiple wokflows are
// configured to respond, then this script will  the first one it finds.
// The choice is essentially random.
package main

import (
	"context"
	"log"
	"time"

	"github.com/gravitational/teleport/build.assets/tooling/lib/github"
)

func main() {
	args := parseCommandLine()
	ctx, cancel := context.WithTimeout(context.Background(), args.timeout)
	defer cancel()

	// Create a GitHub client that authenticates witha Personal Access Token
	gh := github.NewGitHubWithToken(ctx, args.token)

	dispatchCtx, cancelDispatch := context.WithTimeout(ctx, 1*time.Minute)
	defer cancelDispatch()

	run, err := gh.TriggerDispatchEvent(dispatchCtx, args.owner, args.repo, args.workflow, args.workflowRef, args.inputs)
	if err != nil {
		log.Fatalf("Failed to start workflow run %s", err)
	}
	log.Printf("Workflow run: %s", run.GetHTMLURL())

	conclusion, err := gh.WaitForRun(ctx, args.owner, args.repo, args.workflow, args.workflowRef, run.GetID())
	if err != nil {
		log.Fatalf("Failed to waiting for run to exit %s", err)
	}

	if conclusion != "success" {
		log.Fatalf("Build failed: %s", conclusion)
	}

	log.Printf("Build succeeded")
}
