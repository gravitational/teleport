/*
Copyright 2023 Gravitational, Inc.

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
	"sort"
	"strings"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/ghodss/yaml"
	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"
)

type oktaCommands struct {
	apps   *oktaAppsCommand
	groups *oktaGroupsCommand
	//labelRules  *oktaLabelRulesCommands
	//requests  *oktaRequestCommands
}

func newOktaCommand(app *kingpin.Application) oktaCommands {
	okta := app.Command("okta", "Query and request access to Okta applications and groups.")
	return oktaCommands{
		apps:   newOktaAppsCommand(okta),
		groups: newOktaGroupsCommand(okta),
	}
}

type oktaAppsCommand struct {
	*kingpin.CmdClause

	lsApps *oktaAppsLSCommand
}

func newOktaAppsCommand(parent *kingpin.CmdClause) *oktaAppsCommand {
	c := &oktaAppsCommand{
		CmdClause: parent.Command("apps", "Commands for querying Okta applications."),
	}

	c.lsApps = newOktaAppsLSCommand(c.CmdClause)
	return c
}

type oktaAppsLSCommand struct {
	*kingpin.CmdClause

	format string
}

func newOktaAppsLSCommand(parent *kingpin.CmdClause) *oktaAppsLSCommand {
	c := &oktaAppsLSCommand{
		CmdClause: parent.Command("ls", "List Okta applications."),
	}
	c.Flag("format", defaults.FormatFlagDescription(defaults.DefaultFormats...)).Short('f').Default(teleport.Text).EnumVar(&c.format, defaults.DefaultFormats...)
	return c
}

func (c *oktaAppsLSCommand) run(cf *CLIConf) error {
	tc, err := makeClient(cf, true)
	if err != nil {
		return trace.Wrap(err)
	}

	var apps []types.OktaApplication
	if err := client.RetryWithRelogin(cf.Context, tc, func() error {
		pc, err := tc.ConnectToProxy(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer pc.Close()
		aci, err := pc.ConnectToRootCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer aci.Close()

		resp, err := aci.ListOktaApplications(cf.Context, &proto.ListOktaApplicationsRequest{})
		if err != nil {
			return trace.Wrap(err)
		}

		apps = make([]types.OktaApplication, len(resp.Applications))
		for i, app := range resp.Applications {
			apps[i] = app
		}
		return nil
	}); err != nil {
		return trace.Wrap(err)
	}

	// Sort by name before printing.
	sort.Slice(apps, func(i, j int) bool { return apps[i].GetName() < apps[j].GetName() })

	format := strings.ToLower(c.format)
	switch format {
	case teleport.Text, "":
		printOktaApplications(apps)
	case teleport.JSON, teleport.YAML:
		out, err := serializeOktaApplications(apps, format)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Println(out)
	default:
		return trace.BadParameter("unsupported format %q", c.format)
	}

	return nil
}

type oktaGroupsCommand struct {
	*kingpin.CmdClause

	lsGroups *oktaGroupsLSCommand
}

func newOktaGroupsCommand(parent *kingpin.CmdClause) *oktaGroupsCommand {
	c := &oktaGroupsCommand{
		CmdClause: parent.Command("groups", "Commands for querying Okta groups."),
	}

	c.lsGroups = newOktaGroupsLSCommand(c.CmdClause)
	return c
}

func newOktaGroupsLSCommand(parent *kingpin.CmdClause) *oktaGroupsLSCommand {
	c := &oktaGroupsLSCommand{
		CmdClause: parent.Command("ls", "List Okta groups."),
	}
	c.Flag("format", defaults.FormatFlagDescription(defaults.DefaultFormats...)).Short('f').Default(teleport.Text).EnumVar(&c.format, defaults.DefaultFormats...)
	return c
}

type oktaGroupsLSCommand struct {
	*kingpin.CmdClause

	format string
}

func (c *oktaGroupsLSCommand) run(cf *CLIConf) error {
	tc, err := makeClient(cf, true)
	if err != nil {
		return trace.Wrap(err)
	}

	var groups []types.OktaGroup
	if err := client.RetryWithRelogin(cf.Context, tc, func() error {
		pc, err := tc.ConnectToProxy(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer pc.Close()
		aci, err := pc.ConnectToRootCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer aci.Close()

		resp, err := aci.ListOktaGroups(cf.Context, &proto.ListOktaGroupsRequest{})
		if err != nil {
			return trace.Wrap(err)
		}

		groups = make([]types.OktaGroup, len(resp.Groups))
		for i, group := range resp.Groups {
			groups[i] = group
		}
		return nil
	}); err != nil {
		return trace.Wrap(err)
	}

	sort.Slice(groups, func(i, j int) bool { return groups[i].GetName() < groups[j].GetName() })

	format := strings.ToLower(c.format)
	switch format {
	case teleport.Text, "":
		printOktaGroups(groups)
	case teleport.JSON, teleport.YAML:
		out, err := serializeOktaGroups(groups, format)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Println(out)
	default:
		return trace.BadParameter("unsupported format %q", c.format)
	}

	return nil
}

func serializeOktaApplications(apps []types.OktaApplication, format string) (string, error) {
	var out []byte
	var err error
	if format == teleport.JSON {
		out, err = utils.FastMarshalIndent(apps, "", "  ")
	} else {
		out, err = yaml.Marshal(apps)
	}
	return string(out), trace.Wrap(err)
}

func printOktaApplications(apps []types.OktaApplication) {
	t := asciitable.MakeTable([]string{"Name", "Description", "Labels"})
	for _, app := range apps {
		t.AddRow([]string{
			app.GetName(),
			app.GetDescription(),
			labelsToString(app.GetAllLabels()),
		})
	}
	fmt.Println(t.AsBuffer().String())
}

func serializeOktaGroups(groups []types.OktaGroup, format string) (string, error) {
	var out []byte
	var err error
	if format == teleport.JSON {
		out, err = utils.FastMarshalIndent(groups, "", "  ")
	} else {
		out, err = yaml.Marshal(groups)
	}
	return string(out), trace.Wrap(err)
}

func printOktaGroups(groups []types.OktaGroup) {
	t := asciitable.MakeTable([]string{"Name", "Labels"})
	for _, group := range groups {
		t.AddRow([]string{
			group.GetName(),
			labelsToString(group.GetAllLabels()),
		})
	}
	fmt.Println(t.AsBuffer().String())
}

func labelsToString(labels map[string]string) string {
	labelsAndValues := []string{}
	for label, value := range labels {
		labelsAndValues = append(labelsAndValues, fmt.Sprintf("%s:%s", label, value))
	}

	return strings.Join(labelsAndValues, ",")
}
