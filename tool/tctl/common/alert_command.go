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
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	libclient "github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// AlertCommand implements the `tctl alerts` family of commands.
type AlertCommand struct {
	config *servicecfg.Config

	message  string
	labels   string
	severity string
	ttl      time.Duration

	format  string
	verbose bool

	alertList   *kingpin.CmdClause
	alertCreate *kingpin.CmdClause

	alertAck *kingpin.CmdClause

	reason  string
	alertID string
	clear   bool
}

// Initialize allows AlertCommand to plug itself into the CLI parser
func (c *AlertCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	c.config = config
	alert := app.Command("alerts", "Manage cluster alerts.").Alias("alert")

	c.alertList = alert.Command("list", "List cluster alerts.").Alias("ls")
	c.alertList.Flag("verbose", "Show detailed alert info, including acknowledged alerts.").Short('v').BoolVar(&c.verbose)
	c.alertList.Flag("labels", labelHelp).StringVar(&c.labels)
	c.alertList.Flag("format", "Output format, 'text' or 'json'").Default(teleport.Text).EnumVar(&c.format, teleport.Text, teleport.JSON)

	c.alertCreate = alert.Command("create", "Create cluster alerts.")
	c.alertCreate.Arg("message", "Alert body message.").Required().StringVar(&c.message)
	c.alertCreate.Flag("ttl", "Time duration after which the alert expires (default 24h).").DurationVar(&c.ttl)
	c.alertCreate.Flag("severity", "Severity of the alert (low, medium, or high).").Default("low").EnumVar(&c.severity, "low", "medium", "high")
	c.alertCreate.Flag("labels", "List of labels to attach to the alert. For example: key1=value1,key2=value2.").StringVar(&c.labels)

	c.alertAck = alert.Command("ack", "Acknowledge cluster alerts.")
	// Be wary of making any of these flags required. Because `tctl alerts ack ls` is not an actual
	// command but is handled by alertAck, any flag that is required for `tctl alerts ack` will be
	// required for `tctl alerts ack ls` as well.
	c.alertAck.Flag("ttl", "Time duration to acknowledge the cluster alert for.").DurationVar(&c.ttl)
	c.alertAck.Flag("clear", "Clear the acknowledgment for the cluster alert.").BoolVar(&c.clear)
	c.alertAck.Flag("reason", "The reason for acknowledging the cluster alert.").StringVar(&c.reason)
	c.alertAck.Arg("id", "The cluster alert ID.").Required().StringVar(&c.alertID)

	// We add "ack ls" as a command so kingpin shows it in the help dialog - as there is a space, `tctl ack xyz` will always be
	// handled by the ack command above
	// This allows us to be consistent with our other `tctl xyz ls` commands
	alert.Command("ack ls", "List acknowledged cluster alerts.")
}

// TryRun takes the CLI command as an argument (like "alerts ls") and executes it.
func (c *AlertCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	var commandFunc func(ctx context.Context, client *authclient.Client) error
	switch cmd {
	case c.alertList.FullCommand():
		commandFunc = c.List
	case c.alertCreate.FullCommand():
		commandFunc = c.Create
	case c.alertAck.FullCommand():
		commandFunc = c.Ack
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

func (c *AlertCommand) ListAck(ctx context.Context, client *authclient.Client) error {
	acks, err := client.GetAlertAcks(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	table := asciitable.MakeTable([]string{"ID", "Reason", "Expires"})

	for _, ack := range acks {
		expires := apiutils.HumanTimeFormat(ack.Expires)
		table.AddRow([]string{ack.AlertID, fmt.Sprintf("%q", ack.Reason), expires})
	}

	fmt.Println(table.AsBuffer().String())

	return nil
}

func (c *AlertCommand) Ack(ctx context.Context, client *authclient.Client) error {
	if c.clear {
		return c.ClearAck(ctx, client)
	}

	if c.alertID == "ls" {
		return c.ListAck(ctx, client)
	}

	ack := types.AlertAcknowledgement{
		AlertID: c.alertID,
		Reason:  c.reason,
	}

	if c.ttl.Seconds() == 0 {
		c.ttl = 24 * time.Hour
	}

	ack.Expires = time.Now().UTC().Add(c.ttl)

	if err := client.CreateAlertAck(ctx, ack); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("Successfully acknowledged alert %q. Alerts with this ID won't be pushed for %s.\n", c.alertID, c.ttl)

	return nil
}

func (c *AlertCommand) ClearAck(ctx context.Context, client *authclient.Client) error {
	req := proto.ClearAlertAcksRequest{
		AlertID: c.alertID,
	}

	if err := client.ClearAlertAcks(ctx, req); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("Successfully cleared acknowledgement for alert %q. Alerts with this ID will resume being pushed.\n", c.alertID)

	return nil
}

func (c *AlertCommand) List(ctx context.Context, client *authclient.Client) error {
	labels, err := libclient.ParseLabelSpec(c.labels)
	if err != nil {
		return trace.Wrap(err)
	}

	alerts, err := client.GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
		Labels:           labels,
		WithAcknowledged: c.verbose,
		WithUntargeted:   true, // include alerts not specifically targeted toward this user
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if len(alerts) == 0 && c.format == teleport.Text {
		fmt.Println("no alerts")
		return nil
	}

	// sort so that newer/high-severity alerts show up higher.
	types.SortClusterAlerts(alerts)

	switch c.format {
	case teleport.Text:
		displayAlertsText(alerts, c.verbose)
		return nil
	case teleport.JSON:
		return trace.Wrap(displayAlertsJSON(alerts))
	default:
		// technically unreachable since kingpin validates the EnumVar
		return trace.BadParameter("invalid format %q", c.format)
	}
}

func displayAlertsText(alerts []types.ClusterAlert, verbose bool) {
	if verbose {
		table := asciitable.MakeTable([]string{"ID", "Severity", "Expires In", "Message", "Created", "Labels"})
		for _, alert := range alerts {
			var labelPairs []string
			for key, val := range alert.Metadata.Labels {
				// alert labels can be displayed unquoted because we enforce a
				// very limited charset.
				labelPairs = append(labelPairs, fmt.Sprintf("%s=%s", key, val))
			}
			table.AddRow([]string{
				alert.GetName(),
				alert.Spec.Severity.String(),
				calculateTTL(alert.GetMetadata().Expires).String(),
				fmt.Sprintf("%q", alert.Spec.Message),
				alert.Spec.Created.Format(time.RFC822),
				strings.Join(labelPairs, ", "),
			})
		}
		fmt.Println(table.AsBuffer().String())
	} else {
		table := asciitable.MakeTable([]string{"ID", "Severity", "Expires In", "Message"})
		for _, alert := range alerts {
			table.AddRow([]string{
				alert.GetName(),
				alert.Spec.Severity.String(),
				calculateTTL(alert.GetMetadata().Expires).String(),
				fmt.Sprintf("%q", alert.Spec.Message),
			})
		}
		fmt.Println(table.AsBuffer().String())
	}
}

// calculateTTL returns the remaining TTL of the alert.
func calculateTTL(expiration *time.Time) time.Duration {
	if expiration == nil {
		return time.Duration(0)
	}
	remainingDuration := time.Until(*expiration)
	if remainingDuration < 0 {
		return time.Duration(0)
	}

	return remainingDuration.Round(time.Minute)
}

func displayAlertsJSON(alerts []types.ClusterAlert) error {
	out, err := json.MarshalIndent(alerts, "", "  ")
	if err != nil {
		return trace.Wrap(err, "failed to marshal alerts")
	}
	fmt.Println(string(out))
	return nil
}

func (c *AlertCommand) Create(ctx context.Context, client *authclient.Client) error {
	labels, err := libclient.ParseLabelSpec(c.labels)
	if err != nil {
		return trace.Wrap(err)
	}

	var sev types.AlertSeverity
	switch c.severity {
	case "low":
		sev = types.AlertSeverity_LOW
	case "medium":
		sev = types.AlertSeverity_MEDIUM
	case "high":
		sev = types.AlertSeverity_HIGH
	}

	alert, err := types.NewClusterAlert(uuid.New().String(), c.message, types.WithAlertSeverity(sev))
	if err != nil {
		return trace.Wrap(err)
	}

	if len(labels) == 0 {
		labels[types.AlertOnLogin] = "yes"
		labels[types.AlertPermitAll] = "yes"
	}
	alert.Metadata.Labels = labels

	if c.ttl > 0 {
		alert.SetExpiry(time.Now().UTC().Add(c.ttl))
	}

	return trace.Wrap(client.UpsertClusterAlert(ctx, alert))
}
