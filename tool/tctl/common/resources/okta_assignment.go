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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/oktaassignment"
)

type oktaAssignmentCollection struct {
	assignments []types.OktaAssignment
}

func (c *oktaAssignmentCollection) Resources() []types.Resource {
	r := make([]types.Resource, len(c.assignments))
	for i, resource := range c.assignments {
		r[i] = oktaassignment.ToResource(resource)
	}
	return r
}

func (c *oktaAssignmentCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name"})
	for _, assignment := range c.assignments {
		t.AddRow([]string{assignment.GetName()})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func oktaAssignmentHandler() Handler {
	return Handler{
		getHandler:    getOktaAssignment,
		deleteHandler: deleteOktaAssignment,
		description:   "Represents a user assignment imported from Okta.",
	}
}

func getOktaAssignment(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name != "" {
		assignment, err := client.OktaClient().GetOktaAssignment(ctx, ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &oktaAssignmentCollection{assignments: []types.OktaAssignment{assignment}}, nil
	}

	assignments, err := stream.Collect(clientutils.Resources(ctx, client.OktaClient().ListOktaAssignments))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &oktaAssignmentCollection{assignments: assignments}, nil
}

func deleteOktaAssignment(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.OktaClient().DeleteOktaAssignment(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Okta assignment %q has been deleted\n", ref.Name)
	return nil
}
