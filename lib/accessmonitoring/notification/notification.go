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

package notification

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/accessrequest"
	"github.com/gravitational/teleport/api/client/proto"
	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
	pd "github.com/gravitational/teleport/integrations/lib/plugindata"
	"github.com/gravitational/teleport/lib/accessmonitoring"
	"github.com/gravitational/teleport/lib/services"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// Client aggregates the parts of Teleport API client interface
// (as implemented by github.com/gravitational/teleport/api/client.Client)
// that are used by the access review handler.
type Client interface {
	GetPluginData(context.Context, types.PluginDataFilter) ([]types.PluginData, error)
	UpdatePluginData(context.Context, types.PluginDataUpdateParams) error

	ListAccessMonitoringRulesWithFilter(ctx context.Context, req *accessmonitoringrulesv1.ListAccessMonitoringRulesWithFilterRequest) ([]*accessmonitoringrulesv1.AccessMonitoringRule, string, error)
	GetUser(ctx context.Context, name string, withSecrets bool) (types.User, error)
	ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error)
	GetRole(ctx context.Context, name string) (types.Role, error)
}

type Bot interface {
	CheckHealth(ctx context.Context) error
	FetchRecipient(ctx context.Context, recipient string) (*common.Recipient, error)

	NotifyApprover(ctx context.Context, recipient common.Recipient, reqData pd.AccessRequestData) (data SentMessage, err error)
	NotifyRequestor(ctx context.Context, recipient common.Recipient, reqData pd.AccessRequestData) (data SentMessage, err error)

	PostReview(ctx context.Context, originalMessage SentMessage, review types.AccessReview) (SentReview, error)
	UpdateMessage(ctx context.Context, originalMessage SentMessage, reviews []types.AccessReview, reqData pd.AccessRequestData, canReview bool) error

	// NotifyApproverResolved(ctx context.Context, message SentMessage, reqData pd.AccessRequestData) (data SentMessage, err error)
	// NotifyReviewerResolved(ctx context.Context, message SentMessage, reqData pd.AccessRequestData) (data SentMessage, err error)
}

// Config specifies access review handler configuration.
type Config struct {
	// Logger is the logger for the handler.
	Logger *slog.Logger

	// HandlerName specifies the handler name.
	HandlerName string

	// Client is the auth service client interface.
	Client Client

	// Bot is the messaging service client interface.
	Bot Bot

	// Cache is the access monitoring rules cache.
	Cache *accessmonitoring.Cache

	StaticRecipients common.RawRecipientsMap
}

// CheckAndSetDefaults checks and sets default configuration.
func (cfg *Config) CheckAndSetDefaults() error {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Client == nil {
		return trace.BadParameter("teleport client is required")
	}
	if cfg.Bot == nil {
		return trace.BadParameter("notification bot is required")
	}
	if cfg.Cache == nil {
		cfg.Cache = accessmonitoring.NewCache()
	}
	return nil
}

// Handler handles automatic reviews of access requests.
type Handler struct {
	Config

	rules         *accessmonitoring.Cache
	notifications *pd.CompareAndSwap[Notification]
}

// NewHandler returns a new access review handler.
func NewHandler(cfg Config) (*Handler, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	notifications := pd.NewCAS(
		cfg.Client,
		cfg.HandlerName,
		types.KindAccessRequest,
		EncodeNotification,
		DecodeNotification,
	)

	return &Handler{
		Config:        cfg,
		rules:         cfg.Cache,
		notifications: notifications,
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
			PageSize:         pageSize,
			PageToken:        pageToken,
			Subjects:         []string{types.KindAccessRequest},
			NotificationName: handler.HandlerName,
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
	if rule.GetSpec().GetNotification().GetName() != handler.HandlerName {
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
			return trace.Wrap(handler.handleRequest(ctx, req))
		case req.GetState().IsResolved():
			return trace.Wrap(handler.handleRequest(ctx, req))
		default:
			return trace.BadParameter("unknown request state")
		}
	case types.OpDelete:
		err := handler.updateNotification(ctx,
			event.Resource.GetName(),
			pd.ResolvedExpired,
			"",
			nil,
		)
		return trace.Wrap(err)
	default:
		return trace.BadParameter("unexpected event operation %s", event.Type)
	}
}

func (handler *Handler) handleRequest(ctx context.Context, req types.AccessRequest) error {
	notification, err := handler.newNotification(ctx, req)
	if err != nil {
		return trace.Wrap(err)
	}

	notification, err = handler.createNotification(ctx, notification)
	if err != nil {
		return trace.Wrap(err)
	}

	var tag pd.ResolutionTag
	switch req.GetState() {
	case types.RequestState_PENDING:
		tag = pd.Unresolved
	case types.RequestState_APPROVED:
		tag = pd.ResolvedApproved
	case types.RequestState_DENIED:
		tag = pd.ResolvedDenied
	case types.RequestState_PROMOTED:
		tag = pd.ResolvedPromoted
	default:
		return trace.BadParameter("Unknown state: %v", logutils.StringerAttr(req.GetState()))
	}

	// 3. Update notifications with reviews
	err = handler.updateNotification(ctx,
		req.GetName(),
		tag,
		req.GetResolveReason(),
		req.GetReviews(),
	)
	return trace.Wrap(err)
}

func (handler *Handler) getRecipients(ctx context.Context, req types.AccessRequest) ([]common.Recipient, error) {
	recipientSet := common.NewRecipientSet()

	// Fetch reviewer recipients from Access Monitoring Rules
	traits := handler.getUserTraits(ctx, req.GetUser())
	env := getAccessRequestExpressionEnv(req, traits)
	rules := handler.getMatchingRules(ctx, env)

	for _, rule := range rules {
		for _, recipient := range rule.GetSpec().GetNotification().GetRecipients() {
			recipient, err := handler.Bot.FetchRecipient(ctx, recipient)
			if err != nil {
				handler.Logger.WarnContext(ctx, "Failed to fetch plugin recipients based on Access monitoring rule recipients", "error", err)
				continue
			}
			recipientSet.Add(*recipient)
		}
	}

	if recipientSet.Len() != 0 {
		return recipientSet.ToSlice(), nil
	}

	// Fallback to static recipients if Access Monitoring Rule recipients are
	// not configured

	validEmailSuggReviewers := []string{}
	for _, reviewer := range req.GetSuggestedReviewers() {
		if !lib.IsEmail(reviewer) {
			handler.Logger.WarnContext(ctx, "Failed to notify a suggested reviewer with an invalid email address", "reviewer", reviewer)
			continue
		}
		validEmailSuggReviewers = append(validEmailSuggReviewers, reviewer)
	}

	rawRecipients := handler.StaticRecipients.GetRawRecipientsFor(req.GetRoles(), validEmailSuggReviewers)
	for _, rawRecipient := range rawRecipients {
		recipient, err := handler.Bot.FetchRecipient(ctx, rawRecipient)
		if err != nil {
			handler.Logger.WarnContext(ctx, "Failure when fetching recipient, continuing anyway", "error", err)
		} else {
			recipientSet.Add(*recipient)
		}
	}

	if recipientSet.Len() == 0 {
		return nil, trace.BadParameter("unable to get any recipients")
	}
	return recipientSet.ToSlice(), nil
}

func (handler *Handler) newNotification(ctx context.Context, req types.AccessRequest) (Notification, error) {
	reviewerRecipients, err := handler.getRecipients(ctx, req)
	if err != nil {
		return Notification{}, trace.Wrap(err)
	}
	resourceNames, err := handler.getResourceNames(ctx, req)
	if err != nil {
		return Notification{}, trace.Wrap(err)
	}

	loginsByRole, err := handler.getLoginsByRole(ctx, req)
	if trace.IsAccessDenied(err) {
		handler.Logger.WarnContext(ctx, "Missing permissions to get logins by role, please add role.read to the associated role", "error", err)
	} else if err != nil {
		return Notification{}, trace.Wrap(err)
	}

	notification := Notification{
		ID:                 req.GetName(),
		ReviewerRecipients: reviewerRecipients,
		AccessRequestData: pd.AccessRequestData{
			User:              req.GetUser(),
			Roles:             req.GetRoles(),
			RequestReason:     req.GetRequestReason(),
			SystemAnnotations: req.GetSystemAnnotations(),
			Resources:         resourceNames,
			LoginsByRole:      loginsByRole,
		},
		ReviewerMessages: make(map[MessageID]Message),
	}

	recipient, err := handler.Bot.FetchRecipient(ctx, req.GetUser())
	if err != nil {
		handler.Logger.WarnContext(ctx, "Failed to fetch requester recipient", "error", err)
	} else {
		notification.RequesterRecipient = *recipient
	}

	return notification, nil
}

func (handler *Handler) createNotification(ctx context.Context, notification Notification) (Notification, error) {
	notification, err := handler.notifications.Create(ctx, notification.ID, notification)
	switch {
	case trace.IsAlreadyExists(err):
		// The messages have already been sent, nothing to do
		return notification, nil
	case err != nil:
		// This is an unexpected error, returning
		return Notification{}, trace.Wrap(err)
	default:
		for _, recipient := range notification.ReviewerRecipients {
			sent, err := handler.Bot.NotifyApprover(ctx, recipient, notification.AccessRequestData)
			if err != nil {
				handler.Logger.ErrorContext(ctx, "Failed to post message", "error", err, "recipient", recipient)
				continue
			}
			handler.Logger.InfoContext(ctx, "Successfully posted messages", "message_id", sent.ID())
			notification.ReviewerMessages[sent.ID()] = Message{
				SentMessage: sent,
				Reviews:     make(map[ReviewID]SentReview),
			}
		}

		sent, err := handler.Bot.NotifyRequestor(ctx, notification.RequesterRecipient, notification.AccessRequestData)
		if err != nil {
			handler.Logger.ErrorContext(ctx, "Failed to post message", "error", err, "recipient", notification.RequesterRecipient)
		} else {
			handler.Logger.InfoContext(ctx, "Successfully posted messages", "message_id", sent.ID())
			notification.RequesterMessage = Message{
				SentMessage: sent,
				Reviews:     make(map[ReviewID]SentReview),
			}
		}

		// Update plugin data with sent messages
		notification, err = handler.notifications.Update(ctx, notification.ID, func(existing Notification) (Notification, error) {
			existing.ReviewerMessages = notification.ReviewerMessages
			existing.RequesterMessage = notification.RequesterMessage
			return existing, nil
		})
		return notification, trace.Wrap(err)
	}
}

func (handler *Handler) updateNotification(
	ctx context.Context,
	// req types.AccessRequest,
	notificationID string,
	tag pd.ResolutionTag,
	reason string,
	reviews []types.AccessReview,
) error {
	notification, err := handler.notifications.Update(ctx, notificationID, func(existing Notification) (Notification, error) {
		// If resolution field is not empty then we already resolved the incident before. In this case we just quit.
		if existing.AccessRequestData.ResolutionTag != pd.Unresolved {
			return Notification{}, trace.AlreadyExists("request is already resolved")
		}

		// Mark plugin data as resolved.
		existing.AccessRequestData.ResolutionTag = tag
		existing.AccessRequestData.ResolutionReason = reason

		return existing, nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	for _, message := range notification.ReviewerMessages {
		for _, review := range reviews {
			_, ok := message.Reviews[review.Author]
			if ok {
				continue // Review has already been posted. Nothing to do.
			}

			// TODO: Handle unimplemented
			sentReview, err := handler.Bot.PostReview(ctx, message.SentMessage, review)
			if err != nil {
				handler.Logger.WarnContext(ctx, "Failed to post review", "error", err)
				continue
			}
			message.Reviews[sentReview.ID()] = sentReview
		}
		notification.ReviewerMessages[message.SentMessage.ID()] = message

		// TODO: handle unimplemented
		const canReview = true
		if err := handler.Bot.UpdateMessage(ctx, message.SentMessage, reviews, notification.AccessRequestData, canReview); err != nil {
			handler.Logger.WarnContext(ctx, "Failed to update message", "error", err)
		}
	}

	message := notification.RequesterMessage
	for _, review := range reviews {
		_, ok := message.Reviews[review.Author]
		if ok {
			continue // Review has already been posted. Nothing to do.
		}

		sentReview, err := handler.Bot.PostReview(ctx, message.SentMessage, review)
		if err != nil {
			handler.Logger.WarnContext(ctx, "Failed to post review", "error", err)
			continue
		}
		message.Reviews[sentReview.ID()] = sentReview
	}
	notification.RequesterMessage = message

	const canReview = false
	if err := handler.Bot.UpdateMessage(ctx, message.SentMessage, reviews, notification.AccessRequestData, canReview); err != nil {
		handler.Logger.WarnContext(ctx, "Failed to update message", "error", err)
	}

	_, err = handler.notifications.Update(ctx, notification.ID, func(existing Notification) (Notification, error) {
		existing.ReviewerMessages = notification.ReviewerMessages
		existing.RequesterMessage = notification.RequesterMessage
		return existing, nil
	})
	return trace.Wrap(err)
}

func (handler *Handler) getUserTraits(ctx context.Context, userName string) map[string][]string {
	log := logger.Get(ctx)
	const withSecretsFalse = false
	user, err := handler.Client.GetUser(ctx, userName, withSecretsFalse)
	if trace.IsAccessDenied(err) {
		log.WarnContext(ctx, "Missing permissions to read user.traits, please add user.read to the associated role", "error", err)
		return nil
	} else if err != nil {
		log.WarnContext(ctx, "Failed to read user.traits", "error", err)
		return nil
	}
	return user.GetTraits()
}

// getMatchingRules returns the list access monitoring rules that match the
// given access request environment.
func (handler *Handler) getMatchingRules(
	ctx context.Context,
	env accessmonitoring.AccessRequestExpressionEnv,
) []*accessmonitoringrulesv1.AccessMonitoringRule {
	rules := []*accessmonitoringrulesv1.AccessMonitoringRule{}

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
		rules = append(rules, rule)
	}
	return rules
}

func (handler *Handler) getLoginsByRole(ctx context.Context, req types.AccessRequest) (map[string][]string, error) {
	loginsByRole := make(map[string][]string, len(req.GetRoles()))

	user, err := handler.Client.GetUser(ctx, req.GetUser(), false)
	if err != nil {
		handler.Logger.WarnContext(ctx, "Missing permissions to apply user traits to login roles, please add user.read to the associated role", "error", err)
		for _, role := range req.GetRoles() {
			currentRole, err := handler.Client.GetRole(ctx, role)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			loginsByRole[role] = currentRole.GetLogins(types.Allow)
		}
		return loginsByRole, nil
	}
	for _, role := range req.GetRoles() {
		currentRole, err := handler.Client.GetRole(ctx, role)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		currentRole, err = services.ApplyTraits(currentRole, user.GetTraits())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		logins := currentRole.GetLogins(types.Allow)
		if logins == nil {
			logins = []string{}
		}
		loginsByRole[role] = logins
	}
	return loginsByRole, nil
}

func (handler *Handler) getResourceNames(ctx context.Context, req types.AccessRequest) ([]string, error) {
	resourceNames := make([]string, 0, len(req.GetRequestedResourceIDs()))
	resourcesByCluster := accessrequest.GetResourceIDsByCluster(req)

	for cluster, resources := range resourcesByCluster {
		resourceDetails, err := accessrequest.GetResourceDetails(ctx, cluster, handler.Client, resources)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, resource := range resources {
			resourceName := types.ResourceIDToString(resource)
			if details, ok := resourceDetails[resourceName]; ok && details.FriendlyName != "" {
				resourceName = fmt.Sprintf("%s/%s", resource.Kind, details.FriendlyName)
			}
			resourceNames = append(resourceNames, resourceName)
		}
	}
	return resourceNames, nil
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
