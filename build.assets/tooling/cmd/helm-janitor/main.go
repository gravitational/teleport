/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/gravitational/trace"
)

type Chart struct {
	Name          string
	Path          string
	ReferencePath string
}

var charts = []Chart{
	{
		Name: "teleport-cluster",
		Path: "examples/chart/teleport-cluster",
		// teleport-cluster reference is still hand-written.
		ReferencePath: "",
	},
	{
		Name:          "teleport-kube-agent",
		Path:          "examples/chart/teleport-kube-agent",
		ReferencePath: "docs/pages/includes/helm-reference/zz_generated.teleport-kube-agent.mdx",
	},
	{
		Name:          "teleport-relay",
		Path:          "examples/chart/teleport-relay",
		ReferencePath: "docs/pages/includes/helm-reference/zz_generated.teleport-relay.mdx",
	},
	{
		Name:          "teleport-operator",
		Path:          "examples/chart/teleport-cluster/charts/teleport-operator",
		ReferencePath: "docs/pages/includes/helm-reference/zz_generated.teleport-operator.mdx",
	},
	{
		Name:          "access-email",
		Path:          "examples/chart/access/email",
		ReferencePath: "docs/pages/includes/helm-reference/zz_generated.access-email.mdx",
	},
	{
		Name:          "access-jira",
		Path:          "examples/chart/access/jira",
		ReferencePath: "docs/pages/includes/helm-reference/zz_generated.access-jira.mdx",
	},
	{
		Name:          "access-mattermost",
		Path:          "examples/chart/access/mattermost",
		ReferencePath: "docs/pages/includes/helm-reference/zz_generated.access-mattermost.mdx",
	},
	{
		Name:          "access-msteams",
		Path:          "examples/chart/access/msteams",
		ReferencePath: "docs/pages/includes/helm-reference/zz_generated.access-msteams.mdx",
	},
	{
		Name:          "access-pagerduty",
		Path:          "examples/chart/access/pagerduty",
		ReferencePath: "docs/pages/includes/helm-reference/zz_generated.access-pagerduty.mdx",
	},
	{
		Name:          "access-slack",
		Path:          "examples/chart/access/slack",
		ReferencePath: "docs/pages/includes/helm-reference/zz_generated.access-slack.mdx",
	},
	{
		Name:          "event-handler",
		Path:          "examples/chart/event-handler",
		ReferencePath: "docs/pages/includes/helm-reference/zz_generated.event-handler.mdx",
	},
	{
		Name:          "tbot",
		Path:          "examples/chart/tbot",
		ReferencePath: "docs/pages/includes/helm-reference/zz_generated.tbot.mdx",
	},
	{
		Name:          "tbot-spiffe-daemon-set",
		Path:          "examples/chart/tbot-spiffe-daemon-set",
		ReferencePath: "docs/pages/includes/helm-reference/zz_generated.tbot-spiffe-daemon-set.mdx",
	},
}

const usage = `Usage:
  helm-janitor all [--charts=<names>]
  helm-janitor test [--charts=<names>]
  helm-janitor lint [--charts=<names>]
  helm-janitor reference [--check] [--charts=<names>]

  helm-janitor list
  helm-janitor update-version <version>

<names> is a comma-separated list of chart names.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	command := os.Args[1]

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// TODO: capture signals and cancel contexts

	switch command {
	case "all":
		allCmd := flag.NewFlagSet("all", flag.ExitOnError)
		chartsFlag := allCmd.String("charts", "", "Comma-separated list of chart names")
		if err := allCmd.Parse(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
		selectedCharts := selectCharts(*chartsFlag)
		if err := runAll(ctx, selectedCharts); err != nil {
			log.Fatal(err)
		}

	case "test":
		testCmd := flag.NewFlagSet("test", flag.ExitOnError)
		chartsFlag := testCmd.String("charts", "", "Comma-separated list of chart names")
		updateSnapshotsFlag := testCmd.Bool("update-snapshots", false, "Update Helm test snapshots")
		if err := testCmd.Parse(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
		selectedCharts := selectCharts(*chartsFlag)
		if err := runTest(ctx, selectedCharts, *updateSnapshotsFlag); err != nil {
			log.Fatal(err)
		}

	case "lint":
		lintCmd := flag.NewFlagSet("lint", flag.ExitOnError)
		chartsFlag := lintCmd.String("charts", "", "Comma-separated list of chart names")
		if err := lintCmd.Parse(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
		selectedCharts := selectCharts(*chartsFlag)
		if err := runLint(ctx, selectedCharts); err != nil {
			log.Fatal(err)
		}

	case "reference", "ref":
		referenceCmd := flag.NewFlagSet("reference", flag.ExitOnError)
		checkFlag := referenceCmd.Bool("check", false, "Check if references are up to date")
		chartsFlag := referenceCmd.String("charts", "", "Comma-separated list of chart names")
		if err := referenceCmd.Parse(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
		selectedCharts := selectCharts(*chartsFlag)
		if err := runReference(ctx, selectedCharts, *checkFlag); err != nil {
			log.Fatal(err)
		}

	case "list":
		if err := listCharts(); err != nil {
			log.Fatal(err)
		}

	case "update-version":
		versionCmd := flag.NewFlagSet("update-version", flag.ExitOnError)
		if err := versionCmd.Parse(os.Args[2:]); err != nil {
			log.Fatal(err)
		}

		args := versionCmd.Args()
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "Error: update-version requires exactly one argument (version)")
			fmt.Fprint(os.Stderr, usage)
			os.Exit(1)
		}
		version := args[0]

		if err := updateVersion(ctx, version); err != nil {
			log.Fatal(err)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}
}

func selectCharts(chartNames string) []Chart {
	if chartNames == "" {
		return charts
	}

	names := strings.Split(chartNames, ",")
	nameSet := make(map[string]bool)
	for _, name := range names {
		nameSet[strings.TrimSpace(name)] = true
	}

	var selected []Chart
	for _, chart := range charts {
		if nameSet[chart.Name] {
			selected = append(selected, chart)
		}
	}

	return selected
}

func runAll(ctx context.Context, charts []Chart) error {
	fmt.Println("Running all operations...")
	if err := runLint(ctx, charts); err != nil {
		return trace.Wrap(err)
	}
	const updateSnapshots = false
	if err := runTest(ctx, charts, updateSnapshots); err != nil {
		return trace.Wrap(err)
	}
	if err := runReference(ctx, charts, false); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func listCharts() error {
	fmt.Println("Available charts:")
	paths := make([]string, len(charts))
	for i, chart := range charts {
		paths[i] = chart.Path
	}
	sort.Strings(paths)
	out, err := yaml.Marshal(paths)
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}
