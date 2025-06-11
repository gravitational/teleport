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
	pd "github.com/gravitational/teleport/integrations/lib/plugindata"
	"github.com/gravitational/teleport/lib/accessmonitoring"
	"github.com/gravitational/teleport/lib/services"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// Client aggregates the parts of Teleport API client interface
// (as implemented by github.com/gravitational/teleport/api/client.Client)
// that are used by the access review handler.
type Client interface {
	GetUser(ctx context.Context, name string, withSecrets bool) (types.User, error)
	GetRole(ctx context.Context, name string) (types.Role, error)
	ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error)
}

type Notifier interface {
	CheckHealth(context.Context) error
	FetchRecipient(context.Context, string) (*common.Recipient, error)

	NotifyReviewer(ctx context.Context, recipient *common.Recipient, ar pd.AccessRequestData) (Message, error)
	NotifyRequester(ctx context.Context, recipient *common.Recipient, ar pd.AccessRequestData) (Message, error)

	UpdateReviewer(ctx context.Context, original Message, ar pd.AccessRequestData) (Message, error)
	UpdateRequester(ctx context.Context, original Message, ar pd.AccessRequestData) (Message, error)
}

// type CAS interface {
// 	Create(context.Context, string, Notification) (Notification, error)
// 	Update(context.Context, string, func(Notification) (Notification, error)) (Notification, error)
// }

// Config specifies access review handler configuration.
type Config struct {
	// Logger is the logger for the handler.
	Logger *slog.Logger

	// Client is the auth service client interface.
	Client Client

	Notifier Notifier

	// Cache is the access monitoring rules cache.
	Cache *accessmonitoring.Cache

	// CAS is the CompareAndSwap for notifications.
	CAS *NotificationCAS

	// StaticRecipients is a map of statically configured recipients.
	// Deprecated: Prefer Access Monitoring Rules.
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
	if cfg.Notifier == nil {
		return trace.BadParameter("notifier client is required")
	}
	if cfg.CAS == nil {
		return trace.BadParameter("notification CAS is required")
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
	notifications *NotificationCAS
}

// NewHandler returns a new access review handler.
func NewHandler(cfg Config) (*Handler, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Handler{
		Config:        cfg,
		rules:         cfg.Cache,
		notifications: cfg.CAS,
	}, nil
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

func (handler *Handler) newNotification(ctx context.Context, req types.AccessRequest) (Notification, error) {
	recipients, err := handler.getRecipients(ctx, req)
	if err != nil {
		return Notification{}, trace.Wrap(err)
	}

	resourceNames, err := handler.getResourceNames(ctx, req)
	if err != nil {
		return Notification{}, trace.Wrap(err)
	}

	loginsByRole, err := handler.getLoginsByRole(ctx, req)
	if err != nil {
		return Notification{}, trace.Wrap(err)
	}

	return Notification{
		ID: req.GetName(),
		AccessRequestData: pd.AccessRequestData{
			User:              req.GetUser(),
			Roles:             req.GetRoles(),
			RequestReason:     req.GetRequestReason(),
			SystemAnnotations: req.GetSystemAnnotations(),
			Reviews:           req.GetReviews(),
			Resources:         resourceNames,
			LoginsByRole:      loginsByRole,
		},
		Recipients: recipients,
	}, nil
}

// TODO: Prefer notification routing using Access Monitoring Rules instead of
// static recipients.
func (handler *Handler) getStaticRecipients(req types.AccessRequest) []string {
	var reviewers []string
	rawRecipients := handler.StaticRecipients.GetRawRecipientsFor(req.GetRoles(), req.GetSuggestedReviewers())
	reviewers = append(reviewers, rawRecipients...)
	return reviewers
}

func (handler *Handler) getRecipientsFromRules(ctx context.Context, req types.AccessRequest) []string {
	var reviewers []string
	traits := handler.getUserTraits(ctx, req.GetUser())
	env := getAccessRequestExpressionEnv(req, traits)
	rules := handler.getMatchingRules(ctx, env)
	for _, rule := range rules {
		reviewers = append(reviewers, rule.GetSpec().GetNotification().GetRecipients()...)
	}
	return reviewers
}

func (handler *Handler) getRecipients(ctx context.Context, req types.AccessRequest) ([]string, error) {
	// Fetch reviewer recipients from Access Monitoring Rules.
	recipients := handler.getRecipientsFromRules(ctx, req)
	if len(recipients) != 0 {
		return append([]string{req.GetUser()}, recipients...), nil
	}

	// Fallback to static recipients if Access Monitoring Rule recipients are
	// not found.
	recipients = handler.getStaticRecipients(req)
	if len(recipients) != 0 {
		return append([]string{req.GetUser()}, recipients...), nil
	}

	return nil, trace.NotFound("unable to get any reviewer recipients")
}

func isRequester(recipient string, notification Notification) bool {
	return recipient == notification.AccessRequestData.User
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
		for _, recipient := range notification.Recipients {
			fetched, err := handler.Notifier.FetchRecipient(ctx, recipient)
			if err != nil {
				handler.Logger.ErrorContext(ctx, "Failed to fetch recipient", "error", err, "recipient", recipient)
				continue
			}

			var sent Message
			if isRequester(recipient, notification) {
				sent, err = handler.Notifier.NotifyRequester(ctx, fetched, notification.AccessRequestData)
			} else {
				sent, err = handler.Notifier.NotifyReviewer(ctx, fetched, notification.AccessRequestData)
			}
			if err != nil {
				handler.Logger.ErrorContext(ctx, "Failed to post message", "error", err, "recipient", recipient)
				continue
			}
			handler.Logger.InfoContext(ctx, "Successfully posted messages", "message_id", sent.ID())

			notification.Messages[recipient] = sent
		}

		// Update plugin data with sent messages
		notification, err = handler.notifications.Update(ctx, notification.ID, func(existing Notification) (Notification, error) {
			existing.Messages = notification.Messages
			return existing, nil
		})
		return notification, trace.Wrap(err)
	}
}

func (handler *Handler) updateNotification(
	ctx context.Context,
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

		existing.AccessRequestData.ResolutionTag = tag
		existing.AccessRequestData.ResolutionReason = reason
		existing.AccessRequestData.Reviews = reviews

		return existing, nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	for _, recipient := range notification.Recipients {
		var updated Message
		var err error

		// TODO: Ensure messages exists
		// The message may have previously failed to send
		original := notification.Messages[recipient]
		if recipient == notification.AccessRequestData.User {
			updated, err = handler.Notifier.UpdateRequester(ctx, original, notification.AccessRequestData)
		} else {
			updated, err = handler.Notifier.UpdateReviewer(ctx, original, notification.AccessRequestData)
		}
		if err != nil {
			handler.Logger.WarnContext(ctx, "Failed to update message", "error", err)
		}
		notification.Messages[recipient] = updated
	}

	_, err = handler.notifications.Update(ctx, notification.ID, func(existing Notification) (Notification, error) {
		existing.Messages = notification.Messages
		return existing, nil
	})
	return trace.Wrap(err)
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

const (
	accessDeniedRoleRead = "Missing permissions to read role, please add role.read to the associated role"
	accessDeniedUserRead = "Missing permissions to read user, please add user.read to the associated role"
)

func (handler *Handler) getUserTraits(ctx context.Context, userName string) map[string][]string {
	const withSecretsFalse = false
	user, err := handler.Client.GetUser(ctx, userName, withSecretsFalse)
	switch {
	case trace.IsAccessDenied(err):
		handler.Logger.WarnContext(ctx, accessDeniedUserRead, "error", err)
		return nil
	case err != nil:
		handler.Logger.WarnContext(ctx, "Failed to read user.traits", "error", err)
		return nil
	default:
		return user.GetTraits()
	}
}

// TODO: We should reconsider how we handle displaying the logins by role.
// A concern is that logins by roles are partially returned, giving reviewers
// a incomplete view of allowed logins. This could encourage a reviewer to
// approve access to privileged logins unintentially.
func (handler *Handler) getLoginsByRole(ctx context.Context, req types.AccessRequest) (map[string][]string, error) {
	loginsByRole := make(map[string][]string, len(req.GetRoles()))
	traits := handler.getUserTraits(ctx, req.GetUser())

	for _, role := range req.GetRoles() {
		currentRole, err := handler.Client.GetRole(ctx, role)
		switch {
		case trace.IsAccessDenied(err):
			handler.Logger.WarnContext(ctx, accessDeniedUserRead, "error", err)
			continue
		case err != nil:
			handler.Logger.WarnContext(ctx, "Failed to get role", "error", err)
			continue
		default:
			if len(traits) != 0 {
				currentRole, err = services.ApplyTraits(currentRole, traits)
				if err != nil {
					return nil, trace.Wrap(err)
				}
			}
			loginsByRole[role] = slices.Clone(currentRole.GetLogins(types.Allow))
		}
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
