/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"os/signal"
	"sort"
	"syscall"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"
)

// Chart represents a chart we want to test, lint, publish a reference for, and update.
type Chart struct {
	// Name of the chart.
	Name string
	// Path of the chart, relative to the teleport repo root.
	Path string
	// ReferencePath is where the generated reference is stored.
	// When it's empty, no reference is generated.
	ReferencePath string
	// IsLibrary describes if the chart is a library chart.
	// Library charts cannot be installed and are not directly tested, nor linted.
	IsLibrary bool
}

// charts is the source of truth for the list of charts we maintain.
// If you need to introduce a new chart, add to this list.
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
		Name:          "access-discord",
		Path:          "examples/chart/access/discord",
		ReferencePath: "docs/pages/includes/helm-reference/zz_generated.access-discord.mdx",
	},
	{
		Name:          "access-datadog",
		Path:          "examples/chart/access/datadog",
		ReferencePath: "docs/pages/includes/helm-reference/zz_generated.access-datadog.mdx",
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
	{
		Name:          "teleport-kube-updater",
		Path:          "examples/chart/teleport-kube-updater",
		ReferencePath: "",
		IsLibrary:     true,
	},
}

const usage = `Usage:
  helm-janitor all [--charts=<names>] [--root-dir=<path>]
  helm-janitor test [--charts=<names>] [--root-dir=<path>]
  helm-janitor lint [--charts=<names>] [--root-dir=<path>]
  helm-janitor reference [--check] [--charts=<names>] [--root-dir=<path>]

  helm-janitor list [--root-dir=<path>]
  helm-janitor update-version <version> [--root-dir=<path>]

<names> is a comma-separated list of chart names.
<path> is the path to the teleport repo root.
`

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case <-ch:
			cancel()
		case <-ctx.Done():
		}
	}()

	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}
	command := os.Args[1]

	fs := flag.NewFlagSet("helm-janitor", flag.ExitOnError)
	chartsFlag := fs.String("charts", "", "Comma-separated list of chart names")
	updateSnapshotsFlag := fs.Bool("update-snapshots", false, "Update Helm test snapshots")
	checkFlag := fs.Bool("check", false, "Check if references are up to date")
	rootDirFlag := fs.String("root-dir", "", "Root directory of the teleport repo.")

	if err := fs.Parse(os.Args[2:]); err != nil {
		log.Fatal(err)
	}

	selectedCharts, err := selectCharts(*chartsFlag, *rootDirFlag)
	if err != nil {
		log.Fatal(err)
	}

	switch command {
	case "all":
		if err := runAll(ctx, selectedCharts, *rootDirFlag); err != nil {
			log.Fatal(err)
		}

	case "test":
		if err := runTest(ctx, selectedCharts, *updateSnapshotsFlag); err != nil {
			log.Fatal(err)
		}

	case "lint":
		if err := runLint(ctx, selectedCharts, *rootDirFlag); err != nil {
			log.Fatal(err)
		}

	case "reference", "ref":
		if err := runReference(ctx, selectedCharts, *checkFlag); err != nil {
			log.Fatal(err)
		}

	case "list":
		if err := listCharts(ctx, selectedCharts); err != nil {
			log.Fatal(err)
		}

	case "update-version":
		args := fs.Args()
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "Error: update-version requires exactly one argument (version)")
			fmt.Fprint(os.Stderr, usage)
			os.Exit(1)
		}
		version := args[0]

		if err := updateVersion(ctx, version, selectedCharts); err != nil {
			log.Fatal(err)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}
}

func runAll(ctx context.Context, charts []Chart, rootDir string) error {
	fmt.Println("Running all operations...")
	if err := runLint(ctx, charts, rootDir); err != nil {
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

func listCharts(ctx context.Context, charts []Chart) error {
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
