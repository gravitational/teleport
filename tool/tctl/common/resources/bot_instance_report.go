// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>

package resources

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/gravitational/trace"

	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

type autoUpdateBotInstanceReportCollection struct {
	report *autoupdatev1pb.AutoUpdateBotInstanceReport
}

func (c *autoUpdateBotInstanceReportCollection) Resources() []types.Resource {
	return []types.Resource{types.ProtoResource153ToLegacy(c.report)}
}

func (c *autoUpdateBotInstanceReportCollection) WriteText(w io.Writer, _ bool) error {
	t := asciitable.MakeTable([]string{"Update Group", "Version", "Count"})
	for groupName, groupMetrics := range c.report.GetSpec().GetGroups() {
		if groupName == "" {
			groupName = "<no update group>"
		}
		for versionName, versionMetrics := range groupMetrics.GetVersions() {
			t.AddRow([]string{groupName, versionName, strconv.Itoa(int(versionMetrics.Count))})
		}
	}
	t.SortRowsBy([]int{0, 1}, true)

	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func autoUpdateBotInstanceReportHandler() Handler {
	return Handler{
		getHandler:    getAutoUpdateBotInstanceReport,
		deleteHandler: deleteAutoUpdateBotInstanceReport,
		singleton:     true,
		mfaRequired:   false,
		description:   "Report of bot instance counts by update group and version.",
	}
}

func getAutoUpdateBotInstanceReport(
	ctx context.Context,
	client *authclient.Client,
	_ services.Ref,
	_ GetOpts,
) (Collection, error) {
	report, err := client.GetAutoUpdateBotInstanceReport(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &autoUpdateBotInstanceReportCollection{report}, nil
}

func deleteAutoUpdateBotInstanceReport(
	ctx context.Context,
	client *authclient.Client,
	ref services.Ref,
) error {
	if err := client.DeleteAutoUpdateBotInstanceReport(ctx); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("%s has been deleted\n", types.KindAutoUpdateBotInstanceReport)
	return nil
}
