/*
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

package review

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/accessmonitoring"
)

// Client aggregates the parts of Teleport API client interface
// (as implemented by github.com/gravitational/teleport/api/client.Client)
// that are used by the access review handler.
type Client interface {
	SubmitAccessReview(ctx context.Context, params types.AccessReviewSubmission) (types.AccessRequest, error)
	ListAccessMonitoringRulesWithFilter(ctx context.Context, req *accessmonitoringrulesv1.ListAccessMonitoringRulesWithFilterRequest) ([]*accessmonitoringrulesv1.AccessMonitoringRule, string, error)
	GetUser(ctx context.Context, name string, withSecrets bool) (types.User, error)
}

// Config specifies access review handler configuration.
type Config struct {
	// Logger is the logger for the handler.
	Logger *slog.Logger

	// HandlerName specifies the handler name.
	HandlerName string

	// Client is the auth service client interface.
	Client Client

	// Cache is the access monitoring rules cache.
	Cache *accessmonitoring.Cache
}

// CheckAndSetDefaults checks and sets default configuration.
func (cfg *Config) CheckAndSetDefaults() error {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Client == nil {
		return trace.BadParameter("teleport client is required")
	}
	if cfg.Cache == nil {
		cfg.Cache = accessmonitoring.NewCache()
	}
	return nil
}

// Handler handles automatic reviews of access requests.
type Handler struct {
	Config

	rules *accessmonitoring.Cache
}

// NewHandler returns a new access review handler.
func NewHandler(cfg Config) (*Handler, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Handler{
		Config: cfg,
		rules:  cfg.Cache,
	}, nil
}

// initialize the access monitoring rules cache.
func (handler *Handler) initialize(ctx context.Context) error {
	err := handler.rules.Initialize(ctx, func(ctx context.Context, pageSize int64, pageToken string) (
		[]*accessmonitoringrulesv1.AccessMonitoringRule,
		string,
		error,
	) {
		req := &accessmonitoringrulesv1.ListAccessMonitoringRulesWithFilterRequest{
			PageSize:            pageSize,
			PageToken:           pageToken,
			Subjects:            []string{types.KindAccessRequest},
			AutomaticReviewName: handler.HandlerName,
		}
		page, next, err := handler.Client.ListAccessMonitoringRulesWithFilter(ctx, req)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}

		rules := []*accessmonitoringrulesv1.AccessMonitoringRule{}
		for _, rule := range page {
			if handler.ruleApplies(rule) {
				rules = append(rules, rule)
			}
		}
		return rules, next, nil
	})
	return trace.Wrap(err)
}

// HandleAccessMonitoringRule handles access monitoring rule events.
func (handler *Handler) HandleAccessMonitoringRule(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpInit:
		if err := handler.initialize(ctx); err != nil {
			return trace.Wrap(err)
		}
	case types.OpPut:
		e, ok := event.Resource.(types.Resource153UnwrapperT[*accessmonitoringrulesv1.AccessMonitoringRule])
		if !ok {
			return trace.BadParameter("expected resource type, got %T", event.Resource)
		}
		rule := e.UnwrapT()

		// In the event an existing rule no longer applies we must remove it.
		if !handler.ruleApplies(rule) {
			handler.rules.Delete(rule.GetMetadata().GetName())
			return nil
		}
		handler.rules.Put([]*accessmonitoringrulesv1.AccessMonitoringRule{rule})
	case types.OpDelete:
		handler.rules.Delete(event.Resource.GetName())
	default:
		return trace.BadParameter("unexpected event operation %s", event.Type)
	}
	return nil
}

// ruleApplies returns true if the rule applies to this handler.
func (handler *Handler) ruleApplies(rule *accessmonitoringrulesv1.AccessMonitoringRule) bool {
	// Automatic review rule is only applied if the desired state is "reviewed".
	if rule.GetSpec().GetDesiredState() != types.AccessMonitoringRuleStateReviewed {
		return false
	}
	if rule.GetSpec().GetAutomaticReview().GetIntegration() != handler.HandlerName {
		return false
	}
	return slices.Contains(rule.GetSpec().GetSubjects(), types.KindAccessRequest)
}

// HandleAccessRequest handles access request events.
func (handler *Handler) HandleAccessRequest(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpPut:
		req, ok := event.Resource.(types.AccessRequest)
		if !ok {
			return trace.BadParameter("unexpected resource type %T", event.Resource)
		}
		switch {
		case req.GetState().IsPending():
			return trace.Wrap(handler.onPendingRequest(ctx, req))
		case req.GetState().IsResolved():
			// Nothing to do when access request is resolved.
			return nil
		default:
			return trace.BadParameter("unknown request state")
		}
	case types.OpDelete:
		// Nothing to do when access request is deleted.
		return nil
	default:
		return trace.BadParameter("unexpected event operation %s", event.Type)
	}
}

func (handler *Handler) onPendingRequest(ctx context.Context, req types.AccessRequest) error {
	log := handler.Logger.With(
		"req_id", req.GetName(),
		"user", req.GetUser())

	// Automatic reviews are only supported with role requests.
	if len(req.GetRequestedResourceIDs()) > 0 {
		return trace.BadParameter("cannot automatically review access requests for resources other than 'roles'")
	}

	const withSecretsFalse = false
	user, err := handler.Client.GetUser(ctx, req.GetUser(), withSecretsFalse)
	if err != nil {
		return trace.Wrap(err)
	}

	env := getAccessRequestExpressionEnv(req, user.GetTraits())
	reviewRule := handler.getMatchingRule(ctx, env)
	if reviewRule == nil {
		// This access request does not match any access monitoring rules.
		return nil
	}

	review, err := newAccessReview(
		req.GetUser(),
		reviewRule.GetMetadata().GetName(),
		reviewRule.GetSpec().GetAutomaticReview().GetDecision())
	if err != nil {
		return trace.Wrap(err, "failed to create new access review")
	}

	_, err = handler.Client.SubmitAccessReview(ctx, types.AccessReviewSubmission{
		RequestID: req.GetName(),
		Review:    review,
	})

	switch {
	case isAlreadyReviewedError(err):
		log.DebugContext(ctx, "Already reviewed the request.", "error", err)
		return nil
	case err != nil:
		return trace.Wrap(err, "submitting access review")
	}

	log.InfoContext(ctx, "Successfully submitted an access review.")
	return nil
}

// getMatchingRule returns the first access monitoring rule that matches the
// given access request environment. If multiple rules match, `DENIED` rules
// take precedence.
func (handler *Handler) getMatchingRule(
	ctx context.Context,
	env accessmonitoring.AccessRequestExpressionEnv,
) *accessmonitoringrulesv1.AccessMonitoringRule {
	var reviewRule *accessmonitoringrulesv1.AccessMonitoringRule

	for _, rule := range handler.rules.Get() {
		conditionMatch, err := accessmonitoring.EvaluateCondition(rule.GetSpec().GetCondition(), env)
		if err != nil {
			handler.Logger.WarnContext(ctx, "Failed to evaluate access monitoring rule",
				"error", err,
				"rule", rule.GetMetadata().GetName(),
			)
			continue
		}

		if !conditionMatch {
			continue
		}

		if rule.GetSpec().GetAutomaticReview().GetDecision() == types.RequestState_DENIED.String() {
			return rule
		}

		if reviewRule == nil {
			reviewRule = rule
		}
	}
	return reviewRule
}

func newAccessReview(userName, ruleName, state string) (types.AccessReview, error) {
	var proposedState types.RequestState
	switch state {
	case types.RequestState_APPROVED.String():
		proposedState = types.RequestState_APPROVED
	case types.RequestState_DENIED.String():
		proposedState = types.RequestState_DENIED
	default:
		return types.AccessReview{}, trace.BadParameter("proposed state is unsupported: %s", state)
	}

	return types.AccessReview{
		Author:        teleport.SystemAccessApproverUserName,
		ProposedState: proposedState,
		Reason: fmt.Sprintf("Access request has been automatically %[4]s by %[1]q. "+
			"User %[2]q is %[4]s by access_monitoring_rule %[3]q.",
			teleport.SystemAccessApproverUserName, userName, ruleName, strings.ToLower(state)),
		Created: time.Now(),
	}, nil
}

func isAlreadyReviewedError(err error) bool {
	if err == nil {
		return false
	}

	return trace.IsAlreadyExists(err) || strings.HasSuffix(err.Error(), "has already reviewed this request")
}

// getAccessRequestExpressionEnv returns the expression env of the access request.
func getAccessRequestExpressionEnv(req types.AccessRequest, traits map[string][]string) accessmonitoring.AccessRequestExpressionEnv {
	return accessmonitoring.AccessRequestExpressionEnv{
		Roles:              req.GetRoles(),
		SuggestedReviewers: req.GetSuggestedReviewers(),
		Annotations:        req.GetSystemAnnotations(),
		User:               req.GetUser(),
		RequestReason:      req.GetRequestReason(),
		CreationTime:       req.GetCreationTime(),
		Expiry:             req.Expiry(),
		UserTraits:         traits,
	}
}
