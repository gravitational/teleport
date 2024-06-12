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

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gravitational/trace"
)

// inputMap represents the input values to a workflow. The odd `interface{}`
// value type (rather than the more obvious `string`) is so that we match the
// type that the GitHub API expects, and we can plug an `inputMap` straight
// in without any further conversion.
type inputMap map[string]interface{}

func (m inputMap) String() string {
	text := strings.Builder{}
	for k, v := range m {
		fmt.Fprintf(&text, "%s=%v, ", k, v)
	}
	return text.String()
}

func (m inputMap) Set(s string) error {
	parts := strings.SplitN(s, "=", 2)

	if len(parts) != 2 {
		return trace.BadParameter("Invalid input. Must be name=value")
	}

	key := parts[0]
	value := parts[1]

	m[key] = value
	return nil
}

// key holds the bytes of a GutHub app key, stored as a slice of bytes so that
// it can be passed into the ghinstance library without further conversion.
type key []byte

func (k *key) String() string {
	return string(*k)
}

func (k *key) Set(s string) error {
	*k = []byte(s)
	return nil
}

// args holds the parsed command-line arguments for the command.
type args struct {
	appID           int64
	appKey          key
	owner           string
	repo            string
	workflow        string
	workflowRef     string
	useWorkflowTag  bool
	seriesRun       bool
	seriesRunFilter string
	timeout         time.Duration
	inputs          inputMap
}

func parseCommandLine() (args, error) {

	cliArgs := args{
		workflowRef: "main",
		inputs:      make(inputMap),
	}

	// 274696 is the Github-assigned app id for the default Drone interface app.
	flag.Int64Var(&cliArgs.appID, "app-id", 274696, "ID of the Drone interface GitHub App")
	flag.Var(&cliArgs.appKey, "app-key", "App key in PEM format for the Drone interface GitHub App. Can also be supplied via $GHA_APP_KEY.")
	flag.StringVar(&cliArgs.owner, "owner", "", "Owner of the repo to target")
	flag.StringVar(&cliArgs.repo, "repo", "", "Repo to target")
	flag.StringVar(&cliArgs.workflow, "workflow", "", "Path to workflow")
	flag.StringVar(&cliArgs.workflowRef, "workflow-ref", cliArgs.workflowRef, "Revision reference")
	flag.BoolVar(&cliArgs.useWorkflowTag, "tag-workflow", false, "Use a workflow input to tag and ID workflows spawned by the event")
	flag.BoolVar(&cliArgs.seriesRun, "series-run", false, "Attempts to wait for any workflows scheduled but not completed before starting this one")
	flag.StringVar(&cliArgs.seriesRunFilter, "series-run-filter", ".*", "Regexp filter to apply when determining what workflow runs should should be waited on, defaulting to \".*\"")
	flag.DurationVar(&cliArgs.timeout, "timeout", time.Duration(0), "Timeout. If not specified, waits forever.")
	flag.Var(cliArgs.inputs, "input", "Input to target workflow")

	flag.Parse()

	if cliArgs.appKey == nil {
		keyText := os.Getenv("GHA_APP_KEY")
		if keyText == "" {
			return args{}, trace.BadParameter("No app key supplied")
		}
		cliArgs.appKey = key(keyText)
	}

	if cliArgs.appID == 0 {
		return args{}, trace.BadParameter("No app ID supplied")
	}

	return cliArgs, nil
}
