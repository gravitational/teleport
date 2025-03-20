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

package approval

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

const (
	// componentName specifies the access approval handler component name used for debugging.
	componentName = "access_approval_handler"

	// stateApproved specifies the approved state.
	stateApproved = "approved"
)

// Client aggregates the parts of Teleport API client interface
// (as implemented by github.com/gravitational/teleport/api/client.Client)
// that are used by the access approval handler.
type Client interface {
	SubmitAccessReview(ctx context.Context, params types.AccessReviewSubmission) (types.AccessRequest, error)
	ListAccessMonitoringRulesWithFilter(ctx context.Context, req *accessmonitoringrulesv1.ListAccessMonitoringRulesWithFilterRequest) ([]*accessmonitoringrulesv1.AccessMonitoringRule, string, error)
	GetUser(ctx context.Context, name string, withSecrets bool) (types.User, error)
}

// Config specifies approval handler configuration.
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

// Handler handles automatic approvals of access requests.
type Handler struct {
	Config

	rules *accessmonitoring.Cache
}

// NewHandler returns a new access approval handler.
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
			PageSize:              pageSize,
			PageToken:             pageToken,
			Subjects:              []string{types.KindAccessRequest},
			AutomaticApprovalName: handler.HandlerName,
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
		e, ok := event.Resource.(types.Resource153Unwrapper)
		if !ok {
			return trace.BadParameter("expected Resource153Unwrapper resource type, got %T", event.Resource)
		}
		rule, ok := e.Unwrap().(*accessmonitoringrulesv1.AccessMonitoringRule)
		if !ok {
			return trace.BadParameter("expected AccessMonitoringRule resource type, got %T", event.Resource)
		}

		// In the event an existing rule no longer applies we must remove it.
		if !handler.ruleApplies(rule) {
			handler.rules.Delete(rule.GetMetadata().GetName())
			return nil
		}
		handler.rules.Put(rule)
	case types.OpDelete:
		handler.rules.Delete(event.Resource.GetName())
	default:
		return trace.BadParameter("unexpected event operation %s", event.Type)
	}
	return nil
}

// ruleApplies returns true if the rule applies to this handler.
func (handler *Handler) ruleApplies(rule *accessmonitoringrulesv1.AccessMonitoringRule) bool {
	// Automatic approval rule is only applied if the desired state is "approved".
	if !slices.Contains(rule.GetSpec().GetStates(), stateApproved) {
		return false
	}
	if rule.GetSpec().GetAutomaticApproval().GetName() != handler.HandlerName {
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

	const withSecretsFalse = false
	user, err := handler.Client.GetUser(ctx, req.GetUser(), withSecretsFalse)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, rule := range handler.rules.Get() {
		// Check if any access monitoring rule enables automatic approval for the access request.
		approved, err := accessmonitoring.EvaluateCondition(
			rule.GetSpec().GetCondition(),
			getAccessRequestExpressionEnv(req, user.GetTraits()))
		if err != nil {
			log.WarnContext(ctx, "Failed to evaluate access monitoring rule",
				"error", err,
				"rule", rule.GetMetadata().GetName(),
			)
		}

		if !approved {
			continue
		}

		// If the the request is pre-approved, then submimt an access request approval.
		_, err = handler.Client.SubmitAccessReview(ctx, types.AccessReviewSubmission{
			RequestID: req.GetName(),
			Review:    newAccessReview(req.GetUser(), rule.GetMetadata().GetName()),
		})

		switch {
		case isAlreadyReviewedError(err):
			log.DebugContext(ctx, "Already reviewed the request.", "error", err)
			return nil
		case err != nil:
			return trace.Wrap(err, "submitting access request")
		}

		log.InfoContext(ctx, "Successfully submitted a request approval.")
		return nil
	}
	return nil
}

func newAccessReview(userName, ruleName string) types.AccessReview {
	return types.AccessReview{
		Author:        teleport.SystemAccessApproverUserName,
		ProposedState: types.RequestState_APPROVED,
		Reason: fmt.Sprintf("Access request has been automatically approved by %q. "+
			"User %q is pre-approved by access_monitoring_rule %q.",
			componentName, userName, ruleName),
		Created: time.Now(),
	}
}

func isAlreadyReviewedError(err error) bool {
	if err == nil {
		return false
	}
	return strings.HasSuffix(err.Error(), "has already reviewed this request")
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
