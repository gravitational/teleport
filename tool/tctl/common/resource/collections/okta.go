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

package collections

import (
	"io"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/tool/tctl/common/oktaassignment"
)

type oktaImportRuleCollection struct {
	importRules []types.OktaImportRule
}

func NewOktaImportRuleCollection(importRules []types.OktaImportRule) ResourceCollection {
	return &oktaImportRuleCollection{importRules: importRules}
}

func (c *oktaImportRuleCollection) Resources() []types.Resource {
	r := make([]types.Resource, len(c.importRules))
	for i, resource := range c.importRules {
		r[i] = resource
	}
	return r
}

func (c *oktaImportRuleCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name"})
	for _, importRule := range c.importRules {
		t.AddRow([]string{importRule.GetName()})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type oktaAssignmentCollection struct {
	assignments []types.OktaAssignment
}

func NewOktaAssignmentCollection(assignments []types.OktaAssignment) ResourceCollection {
	return &oktaAssignmentCollection{assignments: assignments}
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

type userGroupCollection struct {
	userGroups []types.UserGroup
}

func NewUserGroupCollection(userGroups []types.UserGroup) ResourceCollection {
	return &userGroupCollection{userGroups: userGroups}
}

func (c *userGroupCollection) Resources() []types.Resource {
	r := make([]types.Resource, len(c.userGroups))
	for i, resource := range c.userGroups {
		r[i] = resource
	}
	return r
}

func (c *userGroupCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Origin"})
	for _, userGroup := range c.userGroups {
		t.AddRow([]string{
			userGroup.GetName(),
			userGroup.Origin(),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}
