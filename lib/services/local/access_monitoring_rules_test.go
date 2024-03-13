/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package local

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types/accessmonitoringrule"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

// TestAccessMonitoringRulesCRUD tests backend operations with AccessMonitoringRule resources.
func TestAccessMonitoringRulesCRUD(t *testing.T) {
	ctx := context.Background()

	mem, err := memory.New(memory.Config{
		Context: ctx,
	})
	require.NoError(t, err)
	t.Cleanup(func() { mem.Close() })

	service, err := NewAccessMonitoringRulesService(mem)
	require.NoError(t, err)

	// Define two AccessMonitoringRules
	AccessMonitoringRule1 := accessmonitoringrule.NewAccessMonitoringRule(
		header.Metadata{Name: "p1"},
		accessmonitoringrule.Spec{},
	)
	AccessMonitoringRule2 := accessmonitoringrule.NewAccessMonitoringRule(
		header.Metadata{Name: "p2"},
		accessmonitoringrule.Spec{},
	)

	// Create both AccessMonitoringRules.
	_, err = service.CreateAccessMonitoringRule(ctx, AccessMonitoringRule1)
	require.NoError(t, err)
	_, err = service.CreateAccessMonitoringRule(ctx, AccessMonitoringRule2)
	require.NoError(t, err)

	// Fetch a specific AccessMonitoringRule.
	rule, err := service.GetAccessMonitoringRule(ctx, AccessMonitoringRule2.Metadata.Name)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(rule, AccessMonitoringRule2,
		cmpopts.IgnoreFields(header.Metadata{}, "ID", "Revision"),
	))

	// Try to fetch a AccessMonitoringRule that doesn't exist.
	_, err = service.GetAccessMonitoringRule(ctx, "doesnotexist")
	require.IsType(t, trace.NotFound(""), err)

	// Try to create a duplicate AccessMonitoringRule.
	_, err = service.CreateAccessMonitoringRule(ctx, AccessMonitoringRule1)
	require.IsType(t, trace.AlreadyExists(""), err)

	// Delete a AccessMonitoringRule.
	err = service.DeleteAccessMonitoringRule(ctx, AccessMonitoringRule1.Metadata.Name)
	require.NoError(t, err)
	_, err = service.GetAccessMonitoringRule(ctx, AccessMonitoringRule1.Metadata.Name)
	require.IsType(t, trace.NotFound(""), err)

	// Try to delete a AccessMonitoringRule that doesn't exist.
	err = service.DeleteAccessMonitoringRule(ctx, "doesnotexist")
	require.IsType(t, trace.NotFound(""), err)

	// Delete all AccessMonitoringRule.
	err = service.DeleteAllAccessMonitoringRules(ctx)
	require.NoError(t, err)
	_, err = service.GetAccessMonitoringRule(ctx, AccessMonitoringRule1.Metadata.Name)
	require.IsType(t, trace.NotFound(""), err)
	_, err = service.GetAccessMonitoringRule(ctx, AccessMonitoringRule2.Metadata.Name)
	require.IsType(t, trace.NotFound(""), err)
}

func TestListAccessMonitoringRules(t *testing.T) {
	const pageSize = 5
	const numAccessMonitoringRules = 2*pageSize + 1
	ctx := context.Background()

	mem, err := memory.New(memory.Config{
		Context: ctx,
	})
	require.NoError(t, err)
	t.Cleanup(func() { mem.Close() })

	service, err := NewAccessMonitoringRulesService(mem)
	require.NoError(t, err)

	var insertedAccessMonitoringRules []*accessmonitoringrule.AccessMonitoringRule
	for i := 0; i < numAccessMonitoringRules; i++ {
		AccessMonitoringRule := accessmonitoringrule.NewAccessMonitoringRule(
			header.Metadata{Name: fmt.Sprintf("p%02d", i+1)},
			accessmonitoringrule.Spec{})
		_,err := service.CreateAccessMonitoringRule(ctx, AccessMonitoringRule)
		require.NoError(t, err)
		insertedAccessMonitoringRules = append(insertedAccessMonitoringRules, AccessMonitoringRule)
	}

	t.Run("paginated", func(t *testing.T) {
		page1, nextKey, err := service.ListAccessMonitoringRules(ctx, pageSize, "")
		require.NoError(t, err)
		require.NotEmpty(t, nextKey)
		require.Len(t, page1, pageSize)

		page2, nextKey, err := service.ListAccessMonitoringRules(ctx, pageSize, nextKey)
		require.NoError(t, err)
		require.NotEmpty(t, nextKey)
		require.Len(t, page2, pageSize)

		page3, nextKey, err := service.ListAccessMonitoringRules(ctx, pageSize, nextKey)
		require.NoError(t, err)
		require.Empty(t, nextKey)
		require.Len(t, page3, 1)

		var fetchedAccessMonitoringRules []*accessmonitoringrule.AccessMonitoringRule
		fetchedAccessMonitoringRules = append(fetchedAccessMonitoringRules, page1...)
		fetchedAccessMonitoringRules = append(fetchedAccessMonitoringRules, page2...)
		fetchedAccessMonitoringRules = append(fetchedAccessMonitoringRules, page3...)

		require.Empty(t, cmp.Diff(insertedAccessMonitoringRules, fetchedAccessMonitoringRules,
			cmpopts.IgnoreFields(header.Metadata{}, "ID", "Revision"),
		))
	})

	t.Run("single", func(t *testing.T) {
		fetchedAccessMonitoringRules, nextKey, err := service.ListAccessMonitoringRules(ctx, apidefaults.DefaultChunkSize, "")
		require.NoError(t, err)
		require.Empty(t, nextKey)

		require.Empty(t, cmp.Diff(insertedAccessMonitoringRules, fetchedAccessMonitoringRules,
			cmpopts.IgnoreFields(header.Metadata{}, "ID", "Revision"),
		))
	})
}
