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

package common

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	vc "github.com/gravitational/teleport/lib/versioncontrol"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
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

	upgrader    string
	updateGroup string

	inventoryStatus *kingpin.CmdClause
	inventoryList   *kingpin.CmdClause
	inventoryPing   *kingpin.CmdClause
}

// Initialize allows AccessRequestCommand to plug itself into the CLI parser
func (c *InventoryCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	c.config = config
	inventory := app.Command("inventory", "Manage Teleport instance inventory.").Hidden()

	c.inventoryStatus = inventory.Command("status", "Show inventory status summary.")
	c.inventoryStatus.Flag("connected", "Show locally connected instances summary").BoolVar(&c.getConnected)
	c.inventoryStatus.Flag("format", "Output format, 'text' or 'json'").Default(teleport.Text).StringVar(&c.format)

	c.inventoryList = inventory.Command("list", "List Teleport instance inventory.").Alias("ls")
	c.inventoryList.Flag("older-than", "Filter for older teleport versions").StringVar(&c.olderThan)
	c.inventoryList.Flag("newer-than", "Filter for newer teleport versions").StringVar(&c.newerThan)
	c.inventoryList.Flag("exact-version", "Filter output by teleport version").StringVar(&c.version)
	c.inventoryList.Flag("services", "Filter output by service (node,kube,proxy,etc)").StringVar(&c.services)
	c.inventoryList.Flag("format", "Output format, 'text' or 'json'").Default(teleport.Text).StringVar(&c.format)
	c.inventoryList.Flag("upgrader", "Filter output by upgrader (kube,unit,none)").StringVar(&c.upgrader)
	c.inventoryList.Flag("update-group", "Filter output by update group").StringVar(&c.updateGroup)

	c.inventoryPing = inventory.Command("ping", "Ping locally connected instance.")
	c.inventoryPing.Arg("server-id", "ID of target server").Required().StringVar(&c.serverID)
	c.inventoryPing.Flag("control-log", "Use control log for ping").Hidden().BoolVar(&c.controlLog)
}

// TryRun takes the CLI command as an argument (like "inventory status") and executes it.
func (c *InventoryCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	var commandFunc func(ctx context.Context, client *authclient.Client) error
	switch cmd {
	case c.inventoryStatus.FullCommand():
		commandFunc = c.Status
	case c.inventoryList.FullCommand():
		commandFunc = c.List
	case c.inventoryPing.FullCommand():
		commandFunc = c.Ping
	default:
		return false, nil
	}
	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	err = commandFunc(ctx, client)
	closeFn(ctx)

	return true, trace.Wrap(err)
}

func (c *InventoryCommand) Status(ctx context.Context, client *authclient.Client) error {
	rsp, err := client.GetInventoryStatus(ctx, &proto.InventoryStatusRequest{
		Connected: c.getConnected,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	switch c.format {
	case teleport.Text:
		if c.getConnected {
			table := asciitable.MakeTable([]string{"Server ID", "Services", "Version", "Upgrader"})
			for _, h := range rsp.GetConnected() {
				table.AddRow([]string{
					h.GetServerID(),
					strings.Join(h.GetServices(), ","),
					h.GetVersion(),
					cmp.Or(h.GetExternalUpgrader(), "none"),
				})
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
	case teleport.JSON:
		output, err := json.Marshal(rsp)
		if err != nil {
			return trace.Wrap(err, "marshaling inventory status to json")
		}
		fmt.Println(string(output))
	default:
		return trace.BadParameter("unknown format: %q", c.format)
	}

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

func (c *InventoryCommand) List(ctx context.Context, client *authclient.Client) error {
	var services []types.SystemRole
	var err error
	var omitControlPlane bool
	if c.services != "" {
		services, err = types.ParseTeleportRoles(c.services)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		resp, err := client.Ping(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		omitControlPlane = resp.GetServerFeatures().GetCloud()
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
		UpdateGroup:      c.updateGroup,
	})

	switch c.format {
	case teleport.Text:
		table := asciitable.MakeTable([]string{"Server ID", "Hostname", "Services", "Agent Version", "Upgrader", "Upgrader Version", "Update Group"})
		for instances.Next() {
			instance := instances.Item()

			// The auth and proxy services should be omitted from the inventory list
			// on Managed Teleport Enterprise (Cloud-Hosted) instances.
			if omitControlPlane && (instance.HasService(types.RoleAuth) || instance.HasService(types.RoleProxy)) {
				continue
			}

			services := make([]string, 0, len(instance.GetServices()))
			for _, s := range instance.GetServices() {
				services = append(services, string(s))
			}

			upgrader := instance.GetExternalUpgrader()
			if upgrader == "" {
				upgrader = "none"
			}

			upgraderVersion := instance.GetExternalUpgraderVersion()
			if upgraderVersion == "" {
				upgraderVersion = "none"
			}

			var updateGroup string
			if updateInfo := instance.GetUpdaterInfo(); updateInfo != nil {
				updateGroup = updateInfo.UpdateGroup
			}

			table.AddRow([]string{
				instance.GetName(),
				instance.GetHostname(),
				strings.Join(services, ","),
				instance.GetTeleportVersion(),
				upgrader,
				upgraderVersion,
				updateGroup,
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

func (c *InventoryCommand) Ping(ctx context.Context, client *authclient.Client) error {
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
