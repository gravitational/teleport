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
//     checks should protect in cases like merge builds, where every build is
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
// The app authenticates to github via a GitHub app.
//
// Note that killing the tool WILL NOT cancel the corresponding GitHub
// workflow run.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	ghinst "github.com/bradleyfalzon/ghinstallation/v2"
	ghapi "github.com/google/go-github/v41/github"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	retryablehttp "github.com/hashicorp/go-retryablehttp"

	"github.com/gravitational/teleport/build.assets/tooling/lib/github"
)

const wokflowTagInput = "workflow-tag"

func main() {
	args, err := parseCommandLine()
	if err != nil {
		flag.Usage()
		log.Fatal(err.Error())
	}

	ctx := context.Background()
	if args.timeout != 0 {
		log.Printf("Setting %v timeout", args.timeout)

		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, args.timeout)
		defer cancel()
	}

	installationID, err := lookupInstallationID(ctx, args)
	if err != nil {
		log.Fatalf("Failed to fetch installation ID for app #%d on account %s: %s", args.appID, args.owner, err)
	}

	tx, err := ghinst.New(&retryablehttp.RoundTripper{}, args.appID, installationID, args.appKey)
	if err != nil {
		log.Fatalf("Failed creating authenticated transport: %s", err)
	}

	gh := ghapi.NewClient(&http.Client{Transport: tx})

	if args.seriesRun {
		err := waitForActiveWorkflowRuns(ctx, gh, args)
		if err != nil {
			log.Fatalf("Failed to wait for existing workflow runs: %s", err)
		}
	}

	dispatchCtx, cancelDispatch := context.WithTimeout(ctx, 1*time.Minute)
	defer cancelDispatch()

	// Determine what workflows runs have already been created before we start, so
	// we can exclude them when trying to detect a new run started in response to
	// our dispatch event. Note that we pick a time slightly in the past to handle
	// any clock skew.
	baselineTime := time.Now().Add(-2 * time.Minute)
	oldRuns, err := github.ListWorkflowRunIDs(dispatchCtx, gh.Actions, args.owner, args.repo, args.workflow, getBranchForRef(args.workflowRef), baselineTime)
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

	conclusion, err := github.WaitForRun(ctx, gh.Actions, args.owner, args.repo, args.workflow, run.GetID())
	if err != nil {
		log.Fatalf("Failed to wait for run to exit %s", err)
	}

	if conclusion != "success" {
		log.Fatalf("Build failed: %s", conclusion)
	}

	log.Printf("Workflow succeeded")
}

// Returns either the branch name for the provided reference (if it refers to a branch), or an empty string.
func getBranchForRef(ref string) string {
	branchRefPrefix := "refs/heads/"
	if strings.HasPrefix(ref, branchRefPrefix) {
		return strings.TrimPrefix(ref, branchRefPrefix)
	}

	return ""
}

// lookupInstallationID attempts to find an installation of the interface app
// we're using to authenticate.
func lookupInstallationID(ctx context.Context, args args) (int64, error) {
	// Because we don't know the Installtion ID yet (otherwise we wouldn't be
	// here at all) we have to uses a special, short-lived github client that
	// can authenticate without it.
	tx, err := ghinst.NewAppsTransport(&retryablehttp.RoundTripper{}, args.appID, args.appKey)
	if err != nil {
		return 0, trace.Wrap(err, "Failed creating authenticated transport")
	}
	gh := ghapi.NewClient(&http.Client{Transport: tx})

	installationID, err := github.FindAppInstallID(ctx, gh.Apps, args.owner)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	return installationID, nil
}

// Returns the first incomplete matching workflow run found. If none are found, returns nil.
func getIncompleteWorkflowRunID(ctx context.Context, gh *ghapi.Client, args args) (*ghapi.WorkflowRun, error) {
	// If there are runs lasting longer than one hour then there is a probably a much larger problem at play
	recentRuns, err := github.ListWorkflowRuns(ctx, gh.Actions, args.owner, args.repo, args.workflow, "", time.Now().Add(-time.Hour))
	if err != nil {
		return nil, trace.Wrap(err, "failed to get a list of current workflow runs")
	}

	regex, err := regexp.Compile(args.seriesRunFilter)
	if err != nil {
		return nil, trace.Wrap(err, "failed to compile series run regex %q", args.seriesRunFilter)
	}

	for _, recentRun := range recentRuns {
		runStatus := recentRun.GetStatus()
		if runStatus == "" {
			return nil, trace.Errorf("failed to get status for run ID %q", recentRun.GetID())
		}

		if !regex.MatchString(*recentRun.Name) {
			continue
		}

		if runStatus != "completed" {
			return recentRun, nil
		}
	}

	return nil, nil
}

func waitForActiveWorkflowRuns(ctx context.Context, gh *ghapi.Client, args args) error {
	for {
		incompleteWorkflowRun, err := getIncompleteWorkflowRunID(ctx, gh, args)
		if err != nil {
			return trace.Wrap(err, "failed to check if workflow has pending runs")
		}

		if incompleteWorkflowRun == nil {
			return nil
		}

		workflowID := incompleteWorkflowRun.GetID()
		log.Printf("Waiting on pre-existing incomplete run: %s", incompleteWorkflowRun.GetHTMLURL())
		_, err = github.WaitForRun(ctx, gh.Actions, args.owner, args.repo, args.workflow, workflowID)
		if err != nil {
			return trace.Wrap(err, "failed to wait for workflow run %d to complete", workflowID)
		}
	}
}

func waitForNewWorkflowRun(ctx context.Context, gh *ghapi.Client, args args, tag string, baselineTime time.Time, existingRuns github.RunIDSet) (*ghapi.WorkflowRun, error) {
	// Now we need to wait and see if a new workflow is spawned
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Fatal("Timed out waiting for workflow run to start")

		case <-ticker.C:
			newRuns, err := github.ListWorkflowRunIDs(ctx, gh.Actions, args.owner, args.repo, args.workflow, getBranchForRef(args.workflowRef), baselineTime)
			if err != nil {
				return nil, trace.Wrap(err, "Failed polling for new workflow runs")
			}

			for candidate := range newRuns.NotIn(existingRuns) {
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
