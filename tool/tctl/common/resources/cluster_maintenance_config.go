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

package resources

import (
	"context"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

type clusterMaintenanceConfigCollection struct {
	cmc types.ClusterMaintenanceConfig
}

func (c *clusterMaintenanceConfigCollection) Resources() (r []types.Resource) {
	if c.cmc == nil {
		return nil
	}
	return []types.Resource{c.cmc}
}

func (c *clusterMaintenanceConfigCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Type", "Params"})

	agentUpgradeParams := "none"

	if c.cmc != nil {
		if win, ok := c.cmc.GetAgentUpgradeWindow(); ok {
			agentUpgradeParams = fmt.Sprintf("utc_start_hour=%d", win.UTCStartHour)
			if len(win.Weekdays) != 0 {
				agentUpgradeParams = fmt.Sprintf("%s, weekdays=%s", agentUpgradeParams, strings.Join(win.Weekdays, ","))
			}
		}
	}

	t.AddRow([]string{"Agent Upgrades", agentUpgradeParams})

	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func clusterMaintenanceConfigHandler() Handler {
	return Handler{
		getHandler:    getClusterMaintenanceConfig,
		createHandler: createClusterMaintenanceConfig,
		updateHandler: updateClusterMaintenanceConfig,
		deleteHandler: deleteClusterMaintenanceConfig,
		singleton:     true,
		mfaRequired:   false,
		description:   "Configures when agent managed updates v1 happen. Cannot be edited in Teleport Cloud.",
	}
}

func getClusterMaintenanceConfig(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	cmc, err := client.GetClusterMaintenanceConfig(ctx)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	return &clusterMaintenanceConfigCollection{cmc}, nil
}

func createClusterMaintenanceConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	var cmc types.ClusterMaintenanceConfigV1
	if err := utils.FastUnmarshal(raw.Raw, &cmc); err != nil {
		return trace.Wrap(err)
	}

	if err := cmc.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if opts.Force {
		// max nonce forces "upsert" behavior
		cmc.Nonce = math.MaxUint64
	}

	if err := client.UpdateClusterMaintenanceConfig(ctx, &cmc); err != nil {
		return trace.Wrap(err)
	}

	fmt.Println("cluster maintenance config created")
	return nil
}

func updateClusterMaintenanceConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	var cmc types.ClusterMaintenanceConfigV1
	if err := utils.FastUnmarshal(raw.Raw, &cmc); err != nil {
		return trace.Wrap(err)
	}

	if err := cmc.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if err := client.UpdateClusterMaintenanceConfig(ctx, &cmc); err != nil {
		return trace.Wrap(err)
	}

	fmt.Println("cluster maintenance config updated")
	return nil
}

func deleteClusterMaintenanceConfig(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteClusterMaintenanceConfig(ctx); err != nil {
		return trace.Wrap(err)
	}
	fmt.Println("cluster maintenance config deleted")
	return nil
}
