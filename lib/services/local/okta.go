/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package local

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

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
	log           logrus.FieldLogger
	clock         clockwork.Clock
	importRuleSvc *generic.Service[types.OktaImportRule]
	assignmentSvc *generic.Service[types.OktaAssignment]
}

// NewOktaService creates a new OktaService.
func NewOktaService(backend backend.Backend, clock clockwork.Clock) (*OktaService, error) {
	importRuleSvc, err := generic.NewService(&generic.ServiceConfig[types.OktaImportRule]{
		Backend:       backend,
		PageLimit:     oktaImportRuleMaxPageSize,
		ResourceKind:  types.KindOktaImportRule,
		BackendPrefix: oktaImportRulePrefix,
		MarshalFunc:   services.MarshalOktaImportRule,
		UnmarshalFunc: services.UnmarshalOktaImportRule,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	assignmentSvc, err := generic.NewService(&generic.ServiceConfig[types.OktaAssignment]{
		Backend:       backend,
		PageLimit:     oktaAssignmentMaxPageSize,
		ResourceKind:  types.KindOktaAssignment,
		BackendPrefix: oktaAssignmentPrefix,
		MarshalFunc:   services.MarshalOktaAssignment,
		UnmarshalFunc: services.UnmarshalOktaAssignment,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &OktaService{
		log:           logrus.WithFields(logrus.Fields{trace.Component: "okta:local-service"}),
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
	return importRule, o.importRuleSvc.CreateResource(ctx, importRule)
}

// UpdateOktaImportRule updates an existing Okta import rule resource.
func (o *OktaService) UpdateOktaImportRule(ctx context.Context, importRule types.OktaImportRule) (types.OktaImportRule, error) {
	if err := validateOktaImportRuleRegexes(importRule); err != nil {
		return nil, trace.Wrap(err)
	}
	return importRule, o.importRuleSvc.UpdateResource(ctx, importRule)
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
	return assignment, o.assignmentSvc.CreateResource(ctx, assignment)
}

// UpdateOktaAssignment updates an existing Okta assignment resource.
func (o *OktaService) UpdateOktaAssignment(ctx context.Context, assignment types.OktaAssignment) (types.OktaAssignment, error) {
	return assignment, o.assignmentSvc.UpdateResource(ctx, assignment)
}

// UpdateOktaAssignmentStatus will update the status for an Okta assignment if the given time has passed
// since the last transition.
func (o *OktaService) UpdateOktaAssignmentStatus(ctx context.Context, name, status string, timeHasPassed time.Duration) error {
	err := o.assignmentSvc.UpdateAndSwapResource(ctx, name, func(currentAssignment types.OktaAssignment) error {
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
