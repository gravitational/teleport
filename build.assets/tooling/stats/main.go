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
	"fmt"
	"os"
	"time"

	"github.com/gravitational/kingpin"
	"github.com/jedib0t/go-pretty/v6/table"
)

var (
	num               = kingpin.Arg("num", "Number of latest PRs to fetch").Required().Int()
	latestErrorsCount = kingpin.Flag("failures", "Number of latest failures to show").Short('f').Default("10").Int()
	owner             = kingpin.Flag("owner", "Repository owner").Default("gravitational").String()
	repo              = kingpin.Flag("repo", "Repository name").Default("teleport").String()
)

func main() {
	kingpin.Parse()

	fmt.Println("Fetching latest commits and their checks from a merged PRs...")

	prs, err := fetch(*num)
	if err != nil {
		bail(err)
	}

	totals := newTotals(prs)

	fmt.Println()
	fmt.Printf("Time frame: %v - %v\n", totals.FirstRun.Format(time.UnixDate), totals.LastRun.Format(time.UnixDate))
	fmt.Println()

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"#", "App Name", "Name", "First run", "Last run", "Average", "Failures", "%"})

	for n, group := range totals.CheckRunTotals {
		t.AppendRow([]interface{}{
			n + 1,
			group.AppName,
			group.Name,
			group.FirstRun.Format(time.UnixDate),
			group.LastRun.Format(time.UnixDate),
			time.Duration.Round(group.AverageElapsed, time.Second).String(),
			group.Failures,
			fmt.Sprintf("%.2f", group.FailurePercentage),
		})
	}

	t.AppendSeparator()
	t.AppendFooter(table.Row{len(totals.CheckRunTotals), "", ""})
	t.Render()

	if *latestErrorsCount == 0 {
		return
	}
	fmt.Println()
	fmt.Println("Latest failures by type:")
	fmt.Println()

	for _, t := range totals.CheckRunTotals {
		if t.Failures == 0 {
			continue
		}

		fmt.Println(t.Name, " - ", t.AppName+":")

		failures := 0

		for _, run := range t.Runs {
			if run.Conclusion == "SUCCESS" {
				continue
			}

			fmt.Println("  - ", run.Permalink)
			failures++
			if failures > *latestErrorsCount {
				break
			}
		}

		fmt.Println()
	}
}

func bail(err error) {
	fmt.Printf("Error occurred: %v\n", err)
	os.Exit(-1)
}
