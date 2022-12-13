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
// repository and awaits the result. The target workflow must respond to a
// repository_dispatch event.
//
// There is no natively-supported way to positively identify the workflow
// runs triggered by a given workflow_dispatch event, so this tool offers
// two options:
//
//  1. The first workflow run that starts after the event is triggered
//     that matches both the workflow file and source code revision targeted
//     by the event. Use this method if you have no control over the target
//     workflow. This method is inherently racy, but the workflow file and ref
//     checks should protect it cases like merge builds, where every build is
//     from a different source revision.
//
//  2. Adding an extra `workflow-tag` key to the event inputs. The tool will
//     then examine any workflow run that starts after the event is triggered
//     that matches both the workflow file and source code revision targeted
//     by the event. The tool polls the steps for each candidate workflow run
//     until it finds a step with the tag value, and deems the workflow run
//     enclosing that step to be the target run. This method does not suffer
//     from the raciness of option one, but requires that you have control
//     of the target workflow.
//
// The tool uses the more-generic/more-racy option 1 by default. Run with the
// `-tag-workflow` option to use the more-specific/less-racy option 2.
//
// Note that killing the tool WILL NOT cancel the corresponding GitHub
// workflow run.
package main

import (
	"context"
	"log"
	"time"

	ghapi "github.com/google/go-github/v41/github"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"golang.org/x/oauth2"

	"github.com/gravitational/teleport/build.assets/tooling/lib/github"
)

const wokflowTagInput = "workflow-tag"

func main() {
	args := parseCommandLine()
	ctx, cancel := context.WithTimeout(context.Background(), args.timeout)
	defer cancel()

	// Create a GitHub client that authenticates with a Personal Access Token
	authClient := oauth2.NewClient(ctx, oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: args.token},
	))
	gh := ghapi.NewClient(authClient)

	dispatchCtx, cancelDispatch := context.WithTimeout(ctx, 1*time.Minute)
	defer cancelDispatch()

	// Determine what workflows runs have already been created before we start, so
	// we can exclude them when trying to detect a new run started in response to
	// our dispatch event. Note that we pick a time slightly in the past to handle
	// any clock skew.
	baselineTime := time.Now().Add(-2 * time.Minute)
	oldRuns, err := github.ListWorkflowRuns(dispatchCtx, gh.Actions, args.owner, args.repo, args.workflow, args.workflowRef, baselineTime)
	if err != nil {
		log.Fatalf("Failed to fetch initial task list: %s", err)
	}

	// If we're tagging the workflow run, then add the tag input to the request so that it
	// will be piped through to the workflow run
	var tag string
	if args.useWorkflowTag {
		tag = uuid.NewString()
		args.inputs[wokflowTagInput] = tag
	}

	log.Printf("Attempting to trigger %s/%s/**/%s @ %s\n", args.owner, args.repo, args.workflow, args.workflowRef)
	triggerArgs := ghapi.CreateWorkflowDispatchEventRequest{
		Ref:    args.workflowRef,
		Inputs: args.inputs,
	}

	// Issue the workflow_dispatch event.
	_, err = gh.Actions.CreateWorkflowDispatchEventByFileName(dispatchCtx, args.owner, args.repo, args.workflow, triggerArgs)
	if err != nil {
		log.Fatalf("Failed to issue workflow dispatch event: %s", err)
	}

	run, err := waitForNewWorkflowRun(ctx, gh, args, tag, baselineTime, oldRuns)
	if err != nil {
		log.Fatalf("Failed to start workflow run %s", err)
	}
	log.Printf("Workflow run: %s", run.GetHTMLURL())

	conclusion, err := github.WaitForRun(ctx, gh.Actions, args.owner, args.repo, args.workflow, args.workflowRef, run.GetID())
	if err != nil {
		log.Fatalf("Failed to waiting for run to exit %s", err)
	}

	if conclusion != "success" {
		log.Fatalf("Build failed: %s", conclusion)
	}

	log.Printf("Build succeeded")
}

func waitForNewWorkflowRun(ctx context.Context, gh *ghapi.Client, args args, tag string, baselineTime time.Time, runs github.RunIDSet) (*ghapi.WorkflowRun, error) {
	// Now we need to wait and see if a new workflow is spawned
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Fatal("Timed out waiting for workflow run to start")

		case <-ticker.C:
			newRuns, err := github.ListWorkflowRuns(ctx, gh.Actions, args.owner, args.repo, args.workflow, args.workflowRef, baselineTime)
			if err != nil {
				return nil, trace.Wrap(err, "Failed polling for new workflow runs")
			}

			for candidate := range newRuns.NotIn(runs) {
				run, _, err := gh.Actions.GetWorkflowRunByID(ctx, args.owner, args.repo, candidate)
				if err != nil {
					return nil, trace.Wrap(err, "Failed fetching workflow run by id", candidate)
				}

				// If we're not looking for a tagged workflow....
				if tag == "" {
					// just return the first one we find
					return run, nil
				}

				// if we're looking for a tagged workflow on the other hand, we need to
				// fetch the workflow jobs and examine every step in them for a step with
				// a name that matches our tag.
				log.Printf("Examining jobs for workflow run %d from %s", run.GetID(), run.GetHTMLURL())
				jobs, err := github.ListWorkflowJobs(ctx, gh.Actions, args.owner, args.repo, candidate)
				if err != nil {
					return nil, trace.Wrap(err, "Failed polling workflow jobs")
				}

				for _, job := range jobs {
					for _, step := range job.Steps {
						if step.GetName() == tag {
							return run, nil
						}
					}
				}

				// Note that the list of jobs and steps changes over time as the workflows
				// start to execute, so unfortunately we can't simply exclude any rejected
				// workflow runs from further testing. We will have to re-check them next
				// time around in case the appropriate jobs & steps have appeared in the
				// meantime.
			}
		}
	}
}
