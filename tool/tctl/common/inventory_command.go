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
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"
)

// InventoryCommand implements the `tctl inventory` family of commands.
type InventoryCommand struct {
	config *service.Config

	serverID string

	getConnected bool

	format string

	controlLog bool

	inventoryStatus *kingpin.CmdClause
	inventoryList   *kingpin.CmdClause
	inventoryPing   *kingpin.CmdClause
}

// Initialize allows AccessRequestCommand to plug itself into the CLI parser
func (c *InventoryCommand) Initialize(app *kingpin.Application, config *service.Config) {
	c.config = config
	inventory := app.Command("inventory", "Manage Teleport instance inventory").Hidden()

	c.inventoryStatus = inventory.Command("status", "Show inventory status summary")
	c.inventoryStatus.Flag("connected", "Show locally connected instances summary").BoolVar(&c.getConnected)

	c.inventoryList = inventory.Command("list", "List teleport instance inventory").Alias("ls")
	c.inventoryList.Flag("format", "Output format, 'text' or 'json'").Default(teleport.Text).StringVar(&c.format)

	c.inventoryPing = inventory.Command("ping", "Ping locally connected instance")
	c.inventoryPing.Arg("server-id", "ID of target server").Required().StringVar(&c.serverID)
	c.inventoryPing.Flag("control-log", "Use control log for ping").BoolVar(&c.controlLog)
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
	if !c.getConnected {
		// intention is for the status command to eventually display cluster-wide inventory
		// info by default, but we only have access to info specific to this auth right now,
		// so we can only display the locally connected instances. in order to avoid confusion
		// we simply don't support any default output right now.
		fmt.Println("Nothing to display.\n\nhint: try using the --connected flag to see a summary of locally connected instances.")
		return nil
	}
	rsp, err := client.GetInventoryStatus(ctx, proto.InventoryStatusRequest{
		Connected: c.getConnected,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if c.getConnected {
		table := asciitable.MakeTable([]string{"ServerID", "Services", "Version"})
		for _, h := range rsp.Connected {
			services := make([]string, 0, len(h.Services))
			for _, s := range h.Services {
				services = append(services, string(s))
			}
			table.AddRow([]string{h.ServerID, strings.Join(services, ","), h.Version})
		}
		fmt.Println(table.AsBuffer().String())
	}
	return nil
}

func (c *InventoryCommand) List(ctx context.Context, client auth.ClientI) error {

	instanceIter := client.GetInstances(ctx, types.InstanceFilter{})

	switch c.format {

	case teleport.Text:
		table := asciitable.MakeTable([]string{"ServerID", "Hostname", "Services", "Version", "Status"})
		now := time.Now()
		for instanceIter.Next() {
			instance := instanceIter.Item()
			services := make([]string, 0, len(instance.GetServices()))
			for _, s := range instance.GetServices() {
				services = append(services, string(s))
			}
			status := fmt.Sprintf("%s ago", now.Sub(instance.GetLastSeen()).Round(time.Second))
			table.AddRow([]string{instance.GetName(), instance.GetHostname(), strings.Join(services, ","), instance.GetTeleportVersion(), status})
		}

		if err := instanceIter.Done(); err != nil {
			return trace.Wrap(err)
		}

		fmt.Println(table.AsBuffer().String())
		return nil
	case teleport.JSON:
		if err := utils.StreamJSONArrayFromIter(instanceIter, os.Stdout, true); err != nil {
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
