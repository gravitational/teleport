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

package common

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	vc "github.com/gravitational/teleport/lib/versioncontrol"
)

// InventoryCommand implements the `tctl inventory` family of commands.
type InventoryCommand struct {
	config *servicecfg.Config

	serverID string

	getConnected bool

	format string

	controlLog bool

	version string

	olderThan string
	newerThan string

	services string

	upgrader string

	inventoryStatus *kingpin.CmdClause
	inventoryList   *kingpin.CmdClause
	inventoryPing   *kingpin.CmdClause
}

// Initialize allows AccessRequestCommand to plug itself into the CLI parser
func (c *InventoryCommand) Initialize(app *kingpin.Application, config *servicecfg.Config) {
	c.config = config
	inventory := app.Command("inventory", "Manage Teleport instance inventory.").Hidden()

	c.inventoryStatus = inventory.Command("status", "Show inventory status summary.")
	c.inventoryStatus.Flag("connected", "Show locally connected instances summary").BoolVar(&c.getConnected)

	c.inventoryList = inventory.Command("list", "List Teleport instance inventory.").Alias("ls")
	c.inventoryList.Flag("older-than", "Filter for older teleport versions").StringVar(&c.olderThan)
	c.inventoryList.Flag("newer-than", "Filter for newer teleport versions").StringVar(&c.newerThan)
	c.inventoryList.Flag("exact-version", "Filter output by teleport version").StringVar(&c.version)
	c.inventoryList.Flag("services", "Filter output by service (node,kube,proxy,etc)").StringVar(&c.services)
	c.inventoryList.Flag("format", "Output format, 'text' or 'json'").Default(teleport.Text).StringVar(&c.format)
	c.inventoryList.Flag("upgrader", "Filter output by upgrader (kube,unit,none)").StringVar(&c.upgrader)

	c.inventoryPing = inventory.Command("ping", "Ping locally connected instance.")
	c.inventoryPing.Arg("server-id", "ID of target server").Required().StringVar(&c.serverID)
	c.inventoryPing.Flag("control-log", "Use control log for ping").Hidden().BoolVar(&c.controlLog)
}

// TryRun takes the CLI command as an argument (like "inventory status") and executes it.
func (c *InventoryCommand) TryRun(ctx context.Context, cmd string, client auth.ClientI) (match bool, err error) {
	switch cmd {
	case c.inventoryStatus.FullCommand():
		err = c.Status(ctx, client)
	case c.inventoryList.FullCommand():
		err = c.List(ctx, client)
	case c.inventoryPing.FullCommand():
		err = c.Ping(ctx, client)
	default:
		return false, nil
	}
	return true, trace.Wrap(err)
}

func (c *InventoryCommand) Status(ctx context.Context, client auth.ClientI) error {
	rsp, err := client.GetInventoryStatus(ctx, proto.InventoryStatusRequest{
		Connected: c.getConnected,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if c.getConnected {
		table := asciitable.MakeTable([]string{"Server ID", "Services", "Version", "Upgrader"})
		for _, h := range rsp.Connected {
			services := make([]string, 0, len(h.Services))
			for _, s := range h.Services {
				services = append(services, string(s))
			}
			upgrader := h.ExternalUpgrader
			if upgrader == "" {
				upgrader = "none"
			}
			table.AddRow([]string{h.ServerID, strings.Join(services, ","), h.Version, upgrader})
		}

		_, err := table.AsBuffer().WriteTo(os.Stdout)
		return trace.Wrap(err)
	}

	printHierarchicalData(map[string]any{
		"Versions":        toAnyMap(rsp.VersionCounts),
		"Upgraders":       toAnyMap(rsp.UpgraderCounts),
		"Services":        toAnyMap(rsp.ServiceCounts),
		"Total Instances": rsp.InstanceCount,
	}, "  ", 0)

	return nil
}

// toAnyMap converts a mapping with a concrete value type to an 'any' value type.
func toAnyMap[T any](m map[string]T) map[string]any {
	n := make(map[string]any, len(m))
	for key, val := range m {
		n[key] = val
	}

	return n
}

// printHierarchicalData is a helper for displaying nested mappings of data.
func printHierarchicalData(data map[string]any, indent string, depth int) {
	var longestKey int
	for key := range data {
		if longestKey == 0 || len(key) > longestKey {
			longestKey = len(key)
		}
	}

	for key, val := range data {
		if m, ok := val.(map[string]any); ok {
			if len(m) != 0 {
				fmt.Printf("%s%s:\n", strings.Repeat(indent, depth), key)
				printHierarchicalData(m, indent, depth+1)
				continue
			} else {
				val = "none"
			}
		}

		fmt.Printf("%s%s: %s%v\n",
			strings.Repeat(indent, depth),
			key,
			strings.Repeat(" ", longestKey-len(key)),
			val,
		)
	}
}

func (c *InventoryCommand) List(ctx context.Context, client auth.ClientI) error {
	var services []types.SystemRole
	var err error
	if c.services != "" {
		services, err = types.ParseTeleportRoles(c.services)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	upgrader := c.upgrader
	var noUpgrader bool
	if upgrader == "none" {
		// explicitly match instances with no upgrader defined
		upgrader = ""
		noUpgrader = true
	}

	instances := client.GetInstances(ctx, types.InstanceFilter{
		Services:         services,
		Version:          vc.Normalize(c.version),
		OlderThanVersion: vc.Normalize(c.olderThan),
		NewerThanVersion: vc.Normalize(c.newerThan),
		ExternalUpgrader: upgrader,
		NoExtUpgrader:    noUpgrader,
	})

	switch c.format {
	case teleport.Text:
		table := asciitable.MakeTable([]string{"Server ID", "Hostname", "Services", "Version", "Upgrader"})
		for instances.Next() {
			instance := instances.Item()
			services := make([]string, 0, len(instance.GetServices()))
			for _, s := range instance.GetServices() {
				services = append(services, string(s))
			}

			upgrader := instance.GetExternalUpgrader()
			if upgrader == "" {
				upgrader = "none"
			}

			table.AddRow([]string{
				instance.GetName(),
				instance.GetHostname(),
				strings.Join(services, ","),
				instance.GetTeleportVersion(),
				upgrader,
			})
		}

		if err := instances.Done(); err != nil {
			return trace.Wrap(err)
		}

		_, err := table.AsBuffer().WriteTo(os.Stdout)
		return trace.Wrap(err)
	case teleport.JSON:
		if err := utils.StreamJSONArray(instances, os.Stdout, true); err != nil {
			return trace.Wrap(err)
		}
		fmt.Fprintf(os.Stdout, "\n")
		return nil
	default:
		return trace.BadParameter("unknown format %q, must be one of [%q, %q]", c.format, teleport.Text, teleport.JSON)
	}
}

func (c *InventoryCommand) Ping(ctx context.Context, client auth.ClientI) error {
	rsp, err := client.PingInventory(ctx, proto.InventoryPingRequest{
		ServerID:   c.serverID,
		ControlLog: c.controlLog,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Successfully pinged server %q (~%s).\n", c.serverID, rsp.Duration)
	return nil
}
