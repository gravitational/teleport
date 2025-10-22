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

package resources

import (
	"context"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"

	"github.com/gravitational/trace"

	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

type autoUpdateAgentReportCollection struct {
	reports []*autoupdatev1pb.AutoUpdateAgentReport
}

func (c *autoUpdateAgentReportCollection) Resources() []types.Resource {
	resources := make([]types.Resource, len(c.reports))
	for i, report := range c.reports {
		resources[i] = types.ProtoResource153ToLegacy(report)
	}
	return resources
}

func (c *autoUpdateAgentReportCollection) WriteText(w io.Writer, verbose bool) error {
	groupSet := make(map[string]any)
	versionsSet := make(map[string]any)
	for _, report := range c.reports {
		for groupName, group := range report.GetSpec().GetGroups() {
			groupSet[groupName] = struct{}{}
			for versionName := range group.GetVersions() {
				versionsSet[versionName] = struct{}{}
			}
		}
	}

	groupNames := slices.Collect(maps.Keys(groupSet))
	versionNames := slices.Collect(maps.Keys(versionsSet))
	slices.Sort(groupNames)
	slices.Sort(versionNames)

	t := asciitable.MakeTable(append([]string{"Auth Server ID", "Agent Version"}, groupNames...))
	for _, report := range c.reports {
		for i, versionName := range versionNames {
			row := make([]string, len(groupNames)+2)
			if i == 0 {
				row[0] = report.GetMetadata().GetName()
			}
			row[1] = versionName
			for j, groupName := range groupNames {
				row[j+2] = strconv.Itoa(int(report.GetSpec().GetGroups()[groupName].GetVersions()[versionName].GetCount()))
			}
			t.AddRow(row)
		}
		t.AddRow(make([]string, len(versionNames)+2))
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func autoUpdateAgentReportHandler() Handler {
	return Handler{
		getHandler:    getAutoUpdateAgentReport,
		createHandler: upsertAutoUpdateAgentReport,
		singleton:     false,
		mfaRequired:   false,
		description:   "Reports which agent versions are connected to which Teleport auth server.",
	}
}

func getAutoUpdateAgentReport(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name != "" {
		report, err := client.GetAutoUpdateAgentReport(ctx, ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &autoUpdateAgentReportCollection{reports: []*autoupdatev1pb.AutoUpdateAgentReport{report}}, nil
	}

	reports, err := stream.Collect(clientutils.Resources(ctx, client.ListAutoUpdateAgentReports))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &autoUpdateAgentReportCollection{reports: reports}, nil
}

func upsertAutoUpdateAgentReport(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	report, err := services.UnmarshalProtoResource[*autoupdatev1pb.AutoUpdateAgentReport](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = client.UpsertAutoUpdateAgentReport(ctx, report)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Println("autoupdate_agent_report has been created")
	return nil
}
