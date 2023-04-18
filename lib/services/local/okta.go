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

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
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
	importRuleSvc *generic.Service[types.OktaImportRule]
	assignmentSvc *generic.Service[types.OktaAssignment]
}

// NewOktaService creates a new OktaService.
func NewOktaService(backend backend.Backend) (*OktaService, error) {
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
	return importRule, o.importRuleSvc.CreateResource(ctx, importRule)
}

// UpdateOktaImportRule updates an existing Okta import rule resource.
func (o *OktaService) UpdateOktaImportRule(ctx context.Context, importRule types.OktaImportRule) (types.OktaImportRule, error) {
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
	var previousAssignment types.OktaAssignment
	err := o.assignmentSvc.UpdateAndSwapResource(ctx, assignment.GetName(), func(currentAssignment types.OktaAssignment) error {
		previousAssignment = currentAssignment.Copy()
		currentActions := currentAssignment.GetActions()

		if len(currentActions) != len(assignment.GetActions()) {
			return trace.BadParameter("Update to Okta assignment %s failed because the previous version has a different number of actions", assignment.GetName())
		}

		// Make sure that the status transitions of the updated assignment are valid.
		for i, action := range assignment.GetActions() {
			currentAction := currentActions[i]

			// Ensure that the previous actions are equal
			if !actionsMatch(currentAction, action) {
				return trace.BadParameter("action mismatch when updating Okta assignment %s", assignment.GetName())
			}

			// Don't check the status transition if the statuses are equal and the last transitions are equal.
			if currentAction.GetStatus() == action.GetStatus() &&
				currentAction.GetLastTransition().Equal(action.GetLastTransition()) {
				continue
			}

			if err := currentAction.SetStatus(action.GetStatus()); err != nil {
				return trace.Wrap(err)
			}
			currentAction.SetLastTransition(action.GetLastTransition())
		}

		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return previousAssignment, nil
}

// UpdateOktaAssignmentActionStatuses will update the statuses for all actions in an Okta assignment if the
// status is a valid transition. If a transition is invalid, it will be logged and the rest of the action statuses
// will be updated if possible.
func (o *OktaService) UpdateOktaAssignmentActionStatuses(ctx context.Context, name, status string) (types.OktaAssignment, error) {
	var previousAssignment types.OktaAssignment
	err := o.assignmentSvc.UpdateAndSwapResource(ctx, name, func(assignment types.OktaAssignment) error {
		previousAssignment = assignment.Copy()
		for _, action := range assignment.GetActions() {
			if err := action.SetStatus(status); err != nil {
				o.log.Warnf("Unable to transition status from %s -> %s", action.GetStatus(), status)
			}
		}

		return nil
	})
	return previousAssignment, trace.Wrap(err)

}

// DeleteOktaAssignment removes the specified Okta assignment resource.
func (o *OktaService) DeleteOktaAssignment(ctx context.Context, name string) error {
	return o.assignmentSvc.DeleteResource(ctx, name)
}

// DeleteAllOktaAssignments removes all Okta assignments.
func (o *OktaService) DeleteAllOktaAssignments(ctx context.Context) error {
	return o.assignmentSvc.DeleteAllResources(ctx)
}

// actionsMatch returns true if two actions match minus the status and last transition.
func actionsMatch(a1, a2 types.OktaAssignmentAction) bool {
	return a1.GetTargetType() == a2.GetTargetType() &&
		a1.GetID() == a2.GetID()
}
