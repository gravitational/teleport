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

	"github.com/gravitational/trace"

	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

type autoUpdateAgentRolloutCollection struct {
	rollout *autoupdatev1pb.AutoUpdateAgentRollout
}

func (c *autoUpdateAgentRolloutCollection) Resources() []types.Resource {
	return []types.Resource{types.ProtoResource153ToLegacy(c.rollout)}
}

func (c *autoUpdateAgentRolloutCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Start Version", "Target Version", "Mode", "Schedule", "Strategy"})
	t.AddRow([]string{
		c.rollout.GetMetadata().GetName(),
		fmt.Sprintf("%v", c.rollout.GetSpec().GetStartVersion()),
		fmt.Sprintf("%v", c.rollout.GetSpec().GetTargetVersion()),
		fmt.Sprintf("%v", c.rollout.GetSpec().GetAutoupdateMode()),
		fmt.Sprintf("%v", c.rollout.GetSpec().GetSchedule()),
		fmt.Sprintf("%v", c.rollout.GetSpec().GetStrategy()),
	})
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func autoUpdateAgentRolloutHandler() Handler {
	return Handler{
		getHandler:    getAutoUpdateAgentRollout,
		createHandler: createAutoUpdateAgentRollout,
		updateHandler: updateAutoUpdateAgentRollout,
		deleteHandler: deleteAutoUpdateAgentRollout,
		singleton:     true,
		mfaRequired:   false,
		description:   "Tracks the current state of the managed agent update rollout.",
	}
}

func getAutoUpdateAgentRollout(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	rollout, err := client.GetAutoUpdateAgentRollout(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &autoUpdateAgentRolloutCollection{rollout}, nil
}
func createAutoUpdateAgentRollout(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	rollout, err := services.UnmarshalProtoResource[*autoupdatev1pb.AutoUpdateAgentRollout](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if rollout.GetMetadata() == nil {
		rollout.Metadata = &headerv1.Metadata{}
	}
	if rollout.GetMetadata().GetName() == "" {
		rollout.Metadata.Name = types.MetaNameAutoUpdateAgentRollout
	}

	if opts.Force {
		_, err = client.UpsertAutoUpdateAgentRollout(ctx, rollout)
	} else {
		_, err = client.CreateAutoUpdateAgentRollout(ctx, rollout)
	}
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Println("autoupdate_agent_rollout has been created")
	return nil
}

func updateAutoUpdateAgentRollout(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	rollout, err := services.UnmarshalProtoResource[*autoupdatev1pb.AutoUpdateAgentRollout](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if rollout.GetMetadata() == nil {
		rollout.Metadata = &headerv1.Metadata{}
	}
	if rollout.GetMetadata().GetName() == "" {
		rollout.Metadata.Name = types.MetaNameAutoUpdateAgentRollout
	}

	if _, err := client.UpdateAutoUpdateAgentRollout(ctx, rollout); err != nil {
		return trace.Wrap(err)
	}
	fmt.Println("autoupdate_version has been updated")
	return nil
}

func deleteAutoUpdateAgentRollout(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteAutoUpdateAgentRollout(ctx); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("AutoUpdateAgentRollout has been deleted\n")
	return nil
}
