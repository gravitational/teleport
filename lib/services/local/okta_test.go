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

package local

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
)

// TestOktaImportRuleCRUD tests backend operations with Okta import rule resources.
func TestOktaImportRuleCRUD(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	backend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service, err := NewOktaService(backend, clock)
	require.NoError(t, err)

	// Create a couple Okta import rule.
	importRule1, err := types.NewOktaImportRule(
		types.Metadata{
			Name: "importRule1",
		},
		types.OktaImportRuleSpecV1{
			Mappings: []*types.OktaImportRuleMappingV1{
				{
					Match: []*types.OktaImportRuleMatchV1{
						{
							AppIDs: []string{"yes"},
						},
					},
					AddLabels: map[string]string{
						"label1": "value1",
					},
				},
				{
					Match: []*types.OktaImportRuleMatchV1{
						{
							GroupIDs: []string{"yes"},
						},
					},
					AddLabels: map[string]string{
						"label1": "value1",
					},
				},
			},
		},
	)
	require.NoError(t, err)
	importRule2, err := types.NewOktaImportRule(
		types.Metadata{
			Name: "importRule2",
		},
		types.OktaImportRuleSpecV1{
			Mappings: []*types.OktaImportRuleMappingV1{
				{
					Match: []*types.OktaImportRuleMatchV1{
						{
							AppIDs: []string{"yes"},
						},
					},
					AddLabels: map[string]string{
						"label1": "value1",
					},
				},
				{
					Match: []*types.OktaImportRuleMatchV1{
						{
							GroupIDs: []string{"yes"},
						},
					},
					AddLabels: map[string]string{
						"label1": "value1",
					},
				},
			},
		},
	)
	require.NoError(t, err)

	// Initially we expect no import rule.
	out, nextToken, err := service.ListOktaImportRules(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, out)

	// Create both import rules.
	importRule, err := service.CreateOktaImportRule(ctx, importRule1)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(importRule1, importRule,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))

	importRule, err = service.CreateOktaImportRule(ctx, importRule2)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(importRule2, importRule,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))

	// Fetch all import rules.
	out, nextToken, err = service.ListOktaImportRules(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, cmp.Diff([]types.OktaImportRule{importRule1, importRule2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))

	// Fetch a paginated list of import rules
	paginatedOut := make([]types.OktaImportRule, 0, 2)
	for {
		out, nextToken, err = service.ListOktaImportRules(ctx, 1, nextToken)
		require.NoError(t, err)

		paginatedOut = append(paginatedOut, out...)
		if nextToken == "" {
			break
		}
	}

	require.Len(t, paginatedOut, 2)
	require.Empty(t, cmp.Diff([]types.OktaImportRule{importRule1, importRule2}, paginatedOut,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))

	// Fetch a specific import rule.
	importRule, err = service.GetOktaImportRule(ctx, importRule2.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(importRule2, importRule,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))

	// Try to fetch an import rule that doesn't exist.
	_, err = service.GetOktaImportRule(ctx, "doesnotexist")
	require.True(t, trace.IsNotFound(err), "expected not found error, got %v", err)

	// Try to create the same import rule.
	_, err = service.CreateOktaImportRule(ctx, importRule1)
	require.True(t, trace.IsAlreadyExists(err), "expected already exists error, got %v", err)

	// Update an import rule.
	importRule1.SetExpiry(clock.Now().Add(30 * time.Minute))
	_, err = service.UpdateOktaImportRule(ctx, importRule1)
	require.NoError(t, err)
	importRule, err = service.GetOktaImportRule(ctx, importRule1.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(importRule1, importRule,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))

	// Delete an import rule
	err = service.DeleteOktaImportRule(ctx, importRule1.GetName())
	require.NoError(t, err)
	out, nextToken, err = service.ListOktaImportRules(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, cmp.Diff([]types.OktaImportRule{importRule2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))

	// Try to delete an import rule that doesn't exist.
	err = service.DeleteOktaImportRule(ctx, "doesnotexist")
	require.True(t, trace.IsNotFound(err), "expected not found error, got %v", err)

	// Delete all import rules.
	err = service.DeleteAllOktaImportRules(ctx)
	require.NoError(t, err)
	out, nextToken, err = service.ListOktaImportRules(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, out)
}

func TestValidateOktaImportRuleRegexes(t *testing.T) {
	t.Parallel()

	createRegexMatch := func(appNameRegex, groupNameRegex string) *types.OktaImportRuleMatchV1 {
		return &types.OktaImportRuleMatchV1{
			AppNameRegexes:   []string{appNameRegex},
			GroupNameRegexes: []string{groupNameRegex},
		}
	}

	tests := []struct {
		name    string
		spec    types.OktaImportRuleSpecV1
		wantErr require.ErrorAssertionFunc
	}{
		{
			name: "no regex validation issues",
			spec: types.OktaImportRuleSpecV1{
				Mappings: []*types.OktaImportRuleMappingV1{
					{
						Match:     []*types.OktaImportRuleMatchV1{createRegexMatch(".*", ".*")},
						AddLabels: map[string]string{"label1": "value1"},
					},
					{
						Match:     []*types.OktaImportRuleMatchV1{createRegexMatch(".*", ".*")},
						AddLabels: map[string]string{"label1": "value1"},
					},
				},
			},
			wantErr: require.NoError,
		},
		{
			name: "no regex present",
			spec: types.OktaImportRuleSpecV1{
				Mappings: []*types.OktaImportRuleMappingV1{
					{
						Match:     []*types.OktaImportRuleMatchV1{{AppIDs: []string{"1"}}},
						AddLabels: map[string]string{"label1": "value1"},
					},
					{
						Match:     []*types.OktaImportRuleMatchV1{{GroupIDs: []string{"1"}}},
						AddLabels: map[string]string{"label1": "value1"},
					},
				},
			},
			wantErr: require.NoError,
		},
		{
			name: "app regex validation issues",
			spec: types.OktaImportRuleSpecV1{
				Mappings: []*types.OktaImportRuleMappingV1{
					{
						Match:     []*types.OktaImportRuleMatchV1{createRegexMatch("^(bad$", ".*")},
						AddLabels: map[string]string{"label1": "value1"},
					},
					{
						Match:     []*types.OktaImportRuleMatchV1{createRegexMatch(".*", ".*")},
						AddLabels: map[string]string{"label1": "value1"},
					},
				},
			},
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "error parsing regexp")
			},
		},
		{
			name: "group regex validation issues",
			spec: types.OktaImportRuleSpecV1{
				Mappings: []*types.OktaImportRuleMappingV1{
					{
						Match:     []*types.OktaImportRuleMatchV1{createRegexMatch(".*", ".*")},
						AddLabels: map[string]string{"label1": "value1"},
					},
					{
						Match:     []*types.OktaImportRuleMatchV1{createRegexMatch(".*", "^(bad$")},
						AddLabels: map[string]string{"label1": "value1"},
					},
				},
			},
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "error parsing regexp")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			importRule, err := types.NewOktaImportRule(types.Metadata{
				Name: "test",
			}, test.spec)
			require.NoError(t, err)
			test.wantErr(t, validateOktaImportRuleRegexes(importRule))
		})
	}
}

// TestOktaAssignmentCRUD tests backend operations with Okta assignment resources.
func TestOktaAssignmentCRUD(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	backend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service, err := NewOktaService(backend, clock)
	require.NoError(t, err)

	// Create a couple Okta assignments.
	assignment1 := oktaAssignment(t, "assignment1", "test-user@test.user", constants.OktaAssignmentStatusPending, clock.Now(),
		oktaTarget(t, types.OktaAssignmentTargetV1_APPLICATION, "123456"),
		oktaTarget(t, types.OktaAssignmentTargetV1_GROUP, "234567"),
	)
	assignment2 := oktaAssignment(t, "assignment2", "test-user@test.user", constants.OktaAssignmentStatusPending, clock.Now(),
		oktaTarget(t, types.OktaAssignmentTargetV1_APPLICATION, "123456"),
		oktaTarget(t, types.OktaAssignmentTargetV1_GROUP, "234567"),
	)

	// Initially we expect no assignments.
	out, nextToken, err := service.ListOktaAssignments(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, out)

	// Create both assignments.
	assignment, err := service.CreateOktaAssignment(ctx, assignment1)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(assignment1, assignment,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))

	assignment, err = service.CreateOktaAssignment(ctx, assignment2)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(assignment2, assignment,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))

	// Fetch all assignments.
	out, nextToken, err = service.ListOktaAssignments(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, cmp.Diff([]types.OktaAssignment{assignment1, assignment2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))

	// Fetch a paginated list of assignments
	paginatedOut := make([]types.OktaAssignment, 0, 2)
	numPages := 0
	for {
		numPages++
		out, nextToken, err = service.ListOktaAssignments(ctx, 1, nextToken)
		require.NoError(t, err)

		paginatedOut = append(paginatedOut, out...)
		if nextToken == "" {
			break
		}
	}

	require.Equal(t, 2, numPages)
	require.Empty(t, cmp.Diff([]types.OktaAssignment{assignment1, assignment2}, paginatedOut,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))

	// Fetch a specific assignment.
	assignment, err = service.GetOktaAssignment(ctx, assignment2.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(assignment2, assignment,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))

	// Try to fetch an assignment that doesn't exist.
	_, err = service.GetOktaAssignment(ctx, "doesnotexist")
	require.True(t, trace.IsNotFound(err), "expected not found error, got %v", err)

	// Try to create the same assignment.
	_, err = service.CreateOktaAssignment(ctx, assignment1)
	require.True(t, trace.IsAlreadyExists(err), "expected already exists error, got %v", err)

	// Update the assignment.
	assignment1 = oktaAssignment(t, "assignment1", "test-user@test.user", constants.OktaAssignmentStatusProcessing, clock.Now(),
		oktaTarget(t, types.OktaAssignmentTargetV1_APPLICATION, "123456"),
		oktaTarget(t, types.OktaAssignmentTargetV1_GROUP, "234567"),
	)
	_, err = service.UpdateOktaAssignment(ctx, assignment1)
	require.NoError(t, err)

	assignment, err = service.GetOktaAssignment(ctx, assignment1.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(assignment1, assignment,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))

	// Fail to update the status for an assignment due to a bad transition.
	err = service.UpdateOktaAssignmentStatus(ctx, assignment1.GetName(), constants.OktaAssignmentStatusPending, 0)
	require.ErrorIs(t, err, trace.BadParameter("invalid transition: processing -> pending"))

	// Fail to update the status because not enough time has passed.
	err = service.UpdateOktaAssignmentStatus(ctx, assignment1.GetName(), constants.OktaAssignmentStatusPending, time.Hour)
	require.ErrorIs(t, err, trace.BadParameter("only 0s has passed since last transition"))

	// Successfully update the status for an assignment.
	require.NoError(t, assignment1.SetStatus(constants.OktaAssignmentStatusSuccessful))
	err = service.UpdateOktaAssignmentStatus(ctx, assignment1.GetName(), constants.OktaAssignmentStatusSuccessful, 0)
	require.NoError(t, err)
	assignment, err = service.GetOktaAssignment(ctx, assignment1.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(assignment1, assignment,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))

	// Delete an assignment
	err = service.DeleteOktaAssignment(ctx, assignment1.GetName())
	require.NoError(t, err)
	out, nextToken, err = service.ListOktaAssignments(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, cmp.Diff([]types.OktaAssignment{assignment2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))

	// Try to delete an assignment that doesn't exist.
	err = service.DeleteOktaAssignment(ctx, "doesnotexist")
	require.True(t, trace.IsNotFound(err), "expected not found error, got %v", err)

	// Delete all assignments.
	err = service.DeleteAllOktaAssignments(ctx)
	require.NoError(t, err)
	out, nextToken, err = service.ListOktaAssignments(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, out)
}

func oktaAssignment(t *testing.T, name, username, status string, lastTransition time.Time, targets ...*types.OktaAssignmentTargetV1) types.OktaAssignment {
	assignment, err := types.NewOktaAssignment(
		types.Metadata{
			Name: name,
		},
		types.OktaAssignmentSpecV1{
			User:    username,
			Targets: targets,
		},
	)
	require.NoError(t, err)
	require.NoError(t, assignment.SetStatus(status))
	assignment.SetLastTransition(lastTransition)

	return assignment
}

func oktaTarget(t *testing.T, targetType types.OktaAssignmentTargetV1_OktaAssignmentTargetType,
	id string) *types.OktaAssignmentTargetV1 {

	target := &types.OktaAssignmentTargetV1{
		Type: targetType,
		Id:   id,
	}

	return target
}
