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
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	oktaImportRulePrefix      = "okta_import_rule"
	oktaImportRuleMaxPageSize = 200
	oktaAssignmentPrefix      = "okta_assignment"
	oktaAssignmentMaxPageSize = 200
)

// OktaService manages Okta resources in the Backend.
type OktaService struct {
	clock         clockwork.Clock
	importRuleSvc *generic.Service[types.OktaImportRule]
	assignmentSvc *generic.Service[types.OktaAssignment]
}

// NewOktaService creates a new OktaService.
func NewOktaService(b backend.Backend, clock clockwork.Clock) (*OktaService, error) {
	importRuleSvc, err := generic.NewService(&generic.ServiceConfig[types.OktaImportRule]{
		Backend:       b,
		PageLimit:     oktaImportRuleMaxPageSize,
		ResourceKind:  types.KindOktaImportRule,
		BackendPrefix: backend.NewKey(oktaImportRulePrefix),
		MarshalFunc:   services.MarshalOktaImportRule,
		UnmarshalFunc: services.UnmarshalOktaImportRule,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	assignmentSvc, err := generic.NewService(&generic.ServiceConfig[types.OktaAssignment]{
		Backend:       b,
		PageLimit:     oktaAssignmentMaxPageSize,
		ResourceKind:  types.KindOktaAssignment,
		BackendPrefix: backend.NewKey(oktaAssignmentPrefix),
		MarshalFunc:   services.MarshalOktaAssignment,
		UnmarshalFunc: services.UnmarshalOktaAssignment,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &OktaService{
		clock:         clock,
		importRuleSvc: importRuleSvc,
		assignmentSvc: assignmentSvc,
	}, nil
}

// ListOktaImportRules returns a paginated list of all Okta import rule resources.
func (o *OktaService) ListOktaImportRules(ctx context.Context, pageSize int, nextToken string) ([]types.OktaImportRule, string, error) {
	return o.importRuleSvc.ListResources(ctx, pageSize, nextToken)
}

// GetOktaImportRule returns the specified Okta import rule resources.
func (o *OktaService) GetOktaImportRule(ctx context.Context, name string) (types.OktaImportRule, error) {
	return o.importRuleSvc.GetResource(ctx, name)
}

// CreateOktaImportRule creates a new Okta import rule resource.
func (o *OktaService) CreateOktaImportRule(ctx context.Context, importRule types.OktaImportRule) (types.OktaImportRule, error) {
	if err := validateOktaImportRuleRegexes(importRule); err != nil {
		return nil, trace.Wrap(err)
	}
	created, err := o.importRuleSvc.CreateResource(ctx, importRule)
	return created, trace.Wrap(err)
}

// UpdateOktaImportRule updates an existing Okta import rule resource.
func (o *OktaService) UpdateOktaImportRule(ctx context.Context, importRule types.OktaImportRule) (types.OktaImportRule, error) {
	if err := validateOktaImportRuleRegexes(importRule); err != nil {
		return nil, trace.Wrap(err)
	}
	updated, err := o.importRuleSvc.UpdateResource(ctx, importRule)
	return updated, trace.Wrap(err)
}

// DeleteOktaImportRule removes the specified Okta import rule resource.
func (o *OktaService) DeleteOktaImportRule(ctx context.Context, name string) error {
	return o.importRuleSvc.DeleteResource(ctx, name)
}

// DeleteAllOktaImportRules removes all Okta import rules.
func (o *OktaService) DeleteAllOktaImportRules(ctx context.Context) error {
	return o.importRuleSvc.DeleteAllResources(ctx)
}

// validateOktaImportRuleRegexes will validate all of the regexes present in an import rule.
func validateOktaImportRuleRegexes(importRule types.OktaImportRule) error {
	var errs []error
	for _, mapping := range importRule.GetMappings() {
		for _, match := range mapping.GetMatches() {
			if ok, regexes := match.GetAppNameRegexes(); ok {
				for _, regex := range regexes {
					if _, err := utils.CompileExpression(regex); err != nil {
						errs = append(errs, err)
					}
				}
			}

			if ok, regexes := match.GetGroupNameRegexes(); ok {
				for _, regex := range regexes {
					if _, err := utils.CompileExpression(regex); err != nil {
						errs = append(errs, err)
					}
				}
			}
		}
	}

	return trace.Wrap(trace.NewAggregate(errs...), "error compiling regexes for Okta import rule %s", importRule.GetName())
}

// ListOktaAssignments returns a paginated list of all Okta assignment resources.
func (o *OktaService) ListOktaAssignments(ctx context.Context, pageSize int, nextToken string) ([]types.OktaAssignment, string, error) {
	return o.assignmentSvc.ListResources(ctx, pageSize, nextToken)
}

// GetOktaAssignment returns the specified Okta assignment resources.
func (o *OktaService) GetOktaAssignment(ctx context.Context, name string) (types.OktaAssignment, error) {
	return o.assignmentSvc.GetResource(ctx, name)
}

// CreateOktaAssignment creates a new Okta assignment resource.
func (o *OktaService) CreateOktaAssignment(ctx context.Context, assignment types.OktaAssignment) (types.OktaAssignment, error) {
	created, err := o.assignmentSvc.CreateResource(ctx, assignment)
	return created, trace.Wrap(err)
}

// UpdateOktaAssignment updates an existing Okta assignment resource.
func (o *OktaService) UpdateOktaAssignment(ctx context.Context, assignment types.OktaAssignment) (types.OktaAssignment, error) {
	updated, err := o.assignmentSvc.UpdateResource(ctx, assignment)
	return updated, trace.Wrap(err)
}

// UpdateOktaAssignmentStatus will update the status for an Okta assignment if the given time has passed
// since the last transition.
func (o *OktaService) UpdateOktaAssignmentStatus(ctx context.Context, name, status string, timeHasPassed time.Duration) error {
	_, err := o.assignmentSvc.UpdateAndSwapResource(ctx, name, func(currentAssignment types.OktaAssignment) error {
		// Only update the status if the given duration has passed.
		sinceLastTransition := o.clock.Since(currentAssignment.GetLastTransition())
		if sinceLastTransition < timeHasPassed {
			return trace.BadParameter("only %s has passed since last transition", sinceLastTransition)
		}

		if err := currentAssignment.SetStatus(status); err != nil {
			return trace.Wrap(err)
		}
		currentAssignment.SetLastTransition(o.clock.Now())

		return nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// DeleteOktaAssignment removes the specified Okta assignment resource.
func (o *OktaService) DeleteOktaAssignment(ctx context.Context, name string) error {
	return o.assignmentSvc.DeleteResource(ctx, name)
}

// DeleteAllOktaAssignments removes all Okta assignments.
func (o *OktaService) DeleteAllOktaAssignments(ctx context.Context) error {
	return o.assignmentSvc.DeleteAllResources(ctx)
}
