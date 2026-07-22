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
	"strconv"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

type sessionRecordingConfigCollection struct {
	recConfig types.SessionRecordingConfig
}

func (c *sessionRecordingConfigCollection) Resources() (r []types.Resource) {
	return []types.Resource{c.recConfig}
}

func (c *sessionRecordingConfigCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Mode", "Proxy Checks Host Keys"})
	t.AddRow([]string{c.recConfig.GetMode(), strconv.FormatBool(c.recConfig.GetProxyChecksHostKeys())})
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func sessionRecordingConfigHandler() Handler {
	return Handler{
		getHandler:    getSessionRecordingConfig,
		createHandler: createSessionRecordingConfig,
		updateHandler: updateSessionRecordingConfig,
		deleteHandler: deleteSessionRecordingConfig,
		singleton:     true,
		mfaRequired:   false,
		description:   "Configures session recording for the cluster. Can only be used when session recording is not configured in teleport.yaml.",
	}
}

func getSessionRecordingConfig(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name != "" {
		return nil, trace.BadParameter("only simple `tctl get %v` can be used", types.KindSessionRecordingConfig)
	}
	recConfig, err := client.GetSessionRecordingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &sessionRecordingConfigCollection{recConfig}, nil
}

func createSessionRecordingConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	newRecConfig, err := services.UnmarshalSessionRecordingConfig(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	storedRecConfig, err := client.GetSessionRecordingConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := checkCreateResourceWithOrigin(storedRecConfig, "session recording configuration", opts.Force, opts.Confirm); err != nil {
		return trace.Wrap(err)
	}

	if _, err := client.UpsertSessionRecordingConfig(ctx, newRecConfig); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("session recording configuration has been updated\n")
	return nil
}

func updateSessionRecordingConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	newRecConfig, err := services.UnmarshalSessionRecordingConfig(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	storedRecConfig, err := client.GetSessionRecordingConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := checkUpdateResourceWithOrigin(storedRecConfig, "session recording configuration", opts.Confirm); err != nil {
		return trace.Wrap(err)
	}

	if _, err := client.UpdateSessionRecordingConfig(ctx, newRecConfig); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("session recording configuration has been updated\n")
	return nil
}

func deleteSessionRecordingConfig(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	storedRecConfig, err := client.GetSessionRecordingConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	managedByStaticConfig := storedRecConfig.Origin() == types.OriginConfigFile
	if managedByStaticConfig {
		return trace.BadParameter("%s", managedByStaticDeleteMsg)
	}

	if err := client.ResetSessionRecordingConfig(ctx); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("cluster networking configuration has been reset to defaults\n")
	return nil
}
