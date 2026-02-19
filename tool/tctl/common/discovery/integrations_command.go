// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package discovery

import (
	"context"
	"io"

	usertasksapi "github.com/gravitational/teleport/api/types/usertasks"
	"github.com/gravitational/teleport/api/utils/clientutils"

	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/trace"
)

func (c *Command) runIntegrationList(ctx context.Context, client discoveryClient) error {
	integrations, err := listIntegrations(ctx, client)
	if err != nil {
		return trace.Wrap(err)
	}

	discoveryConfigs, err := stream.Collect(clientutils.Resources(ctx, client.DiscoveryConfigClient().ListDiscoveryConfigs))
	if err != nil {
		return trace.Wrap(err)
	}

	tasks, err := listUserTasks(ctx, client, "", usertasksapi.TaskStateOpen)
	if err != nil {
		return trace.Wrap(err)
	}

	statsMap := buildIntegrationStatsMap(discoveryConfigs)
	taskCountMap := countTasksByIntegration(tasks)
	items := toIntegrationListItems(integrations, statsMap, taskCountMap)

	listOutput := integrationListOutput{
		Total: len(items),
		Items: items,
	}
	return trace.Wrap(writeOutputByFormat(c.output(), c.integrationListFormat, listOutput, func(w io.Writer) error {
		return renderIntegrationListText(w, items)
	}))
}

func (c *Command) runIntegrationShow(ctx context.Context, client discoveryClient) error {
	ig, err := client.GetIntegration(ctx, c.integrationShowName)
	if err != nil {
		return trace.Wrap(err)
	}

	discoveryConfigs, err := stream.Collect(clientutils.Resources(ctx, client.DiscoveryConfigClient().ListDiscoveryConfigs))
	if err != nil {
		return trace.Wrap(err)
	}

	tasks, err := listUserTasks(ctx, client, c.integrationShowName, usertasksapi.TaskStateOpen)
	if err != nil {
		return trace.Wrap(err)
	}

	detail := buildIntegrationDetail(ig, discoveryConfigs, tasks)
	return trace.Wrap(writeOutputByFormat(c.output(), c.integrationShowFormat, detail, func(w io.Writer) error {
		return renderIntegrationShowText(w, detail)
	}))
}
