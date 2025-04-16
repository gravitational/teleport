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

package notificationsv1

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport/api/client"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// ServiceConfig holds configuration options for the notifications gRPC service.
type ServiceConfig struct {
	// Backend is the interface containing all methods for backend read/writes.
	Backend Backend

	// Authorizer is the authorizer used to check access to resources.
	Authorizer authz.Authorizer

	// UserNotificationCache is a custom cache for user-specific notifications,
	// this is to allow fetching notifications by date in descending order.
	UserNotificationCache *services.UserNotificationCache

	// GlobalNotificationCache is a custom cache for user-specific notifications,
	// this is to allow fetching notifications by date in descending order.
	GlobalNotificationCache *services.GlobalNotificationCache

	Clock clockwork.Clock
}

// Backend contains the getters required for notification states and user last seen notifications,
// as well as the methods required by the ReviewPermissionChecker.
type Backend interface {
	services.Notifications
	// Needed by the ReviewPermissionChecker
	services.UserLoginStatesGetter
	services.UserGetter
	services.RoleGetter
	client.ListResourcesClient
	GetRoles(ctx context.Context) ([]types.Role, error)
	GetClusterName(ctx context.Context) (types.ClusterName, error)
}

// Service implements the teleport.notifications.v1.NotificationsService RPC Service.
type Service struct {
	notificationsv1.UnimplementedNotificationServiceServer

	authorizer              authz.Authorizer
	backend                 Backend
	userNotificationCache   *services.UserNotificationCache
	globalNotificationCache *services.GlobalNotificationCache
	clock                   clockwork.Clock
}

// NewService returns a new notifications gRPC service.
func NewService(cfg ServiceConfig) (*Service, error) {
	switch {
	case cfg.Backend == nil:
		return nil, trace.BadParameter("backend is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	case cfg.UserNotificationCache == nil:
		return nil, trace.BadParameter("user notification cache is required")
	case cfg.GlobalNotificationCache == nil:
		return nil, trace.BadParameter("global notification cache is required")
	case cfg.Clock == nil:
		cfg.Clock = clockwork.NewRealClock()
	}

	return &Service{
		authorizer:              cfg.Authorizer,
		backend:                 cfg.Backend,
		userNotificationCache:   cfg.UserNotificationCache,
		globalNotificationCache: cfg.GlobalNotificationCache,
		clock:                   cfg.Clock,
	}, nil
}

// ListNotifications returns a paginated list of notifications which match the user.
func (s *Service) ListNotifications(ctx context.Context, req *notificationsv1.ListNotificationsRequest) (*notificationsv1.ListNotificationsResponse, error) {
	labelsMatch := func(resourceLabels map[string]string) bool {
		if req.Filters == nil || len(req.Filters.Labels) == 0 {
			// no labels to match against
			return true
		}
		for k, v := range req.Filters.Labels {
			if resourceLabels[k] != v {
				return false
			}
		}
		return true
	}

	if req.Filters != nil {
		if req.Filters.GlobalOnly {
			return s.listGlobalNotifications(ctx, req)
		}
		if req.Filters.Username != "" {
			return s.listUserSpecificNotificationsForUser(ctx, req)
		}

		return nil, trace.BadParameter("Invalid filters were provided, exactly one of GlobalOnly or Username must be defined.")
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	username := authCtx.User.GetName()

	// Fetch all of the user's notification states. We do this upfront to filter out dismissed notifications.
	notificationStatesMap := make(map[string]notificationsv1.NotificationState)
	for startKey := ""; ; {
		notificationStates, nextKey, err := s.backend.ListUserNotificationStates(ctx, username, apidefaults.DefaultChunkSize, startKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, notificationState := range notificationStates {
			if notificationState.Spec != nil && notificationState.Status != nil {
				notificationStatesMap[notificationState.Spec.NotificationId] = notificationState.Status.GetNotificationState()
			}
		}
		if nextKey == "" {
			break
		}
		startKey = nextKey
	}

	currentTime := s.clock.Now()
	var hasNotificationExpired = func(n *notificationsv1.Notification) bool {
		notificationExpiryTime := n.GetMetadata().GetExpires().AsTime()
		return currentTime.After(notificationExpiryTime)
	}

	var userNotifMatchFn = func(n *notificationsv1.Notification) bool {
		// Return true if the user hasn't dismissed this notification
		return notificationStatesMap[n.GetMetadata().GetName()] != notificationsv1.NotificationState_NOTIFICATION_STATE_DISMISSED
	}
	userKey, globalKey, found := strings.Cut(req.PageToken, ",")
	if !found && req.PageToken != "" {
		return nil, trace.BadParameter("invalid page token provided")
	}
	pageSize := int(req.PageSize)
	userNotifsStream := stream.Slice[*notificationsv1.Notification](nil)
	if userKey != "" || !found {
		userNotifsStream = stream.FilterMap(
			s.userNotificationCache.StreamUserNotifications(ctx, username, userKey),
			func(n *notificationsv1.Notification) (*notificationsv1.Notification, bool) {
				// If the notification is expired, return false right away.
				if hasNotificationExpired(n) {
					return nil, false
				}

				if !labelsMatch(n.GetMetadata().GetLabels()) {
					return nil, false
				}

				if !userNotifMatchFn(n) {
					return nil, false
				}
				return n, true
			})
	}
	globalNotifsStream := stream.Slice[*notificationsv1.GlobalNotification](nil)
	if globalKey != "" || !found {
		globalNotifsStream = stream.FilterMap(
			s.globalNotificationCache.StreamGlobalNotifications(ctx, globalKey),
			func(gn *notificationsv1.GlobalNotification) (*notificationsv1.GlobalNotification, bool) {
				// If the notification is expired, return false right away.
				if hasNotificationExpired(gn.GetSpec().GetNotification()) {
					return nil, false
				}

				if !labelsMatch(gn.GetMetadata().GetLabels()) {
					return nil, false
				}

				if !s.matchGlobalNotification(ctx, authCtx, gn, notificationStatesMap) {
					return nil, false
				}
				return gn, true
			})
	}
	notifStream := stream.MergeStreams(
		userNotifsStream,
		globalNotifsStream,
		isMoreRecent,
		func(notification *notificationsv1.Notification) *notificationsv1.Notification {
			return notification
		},
		func(globalNotification *notificationsv1.GlobalNotification) *notificationsv1.Notification {
			notification := globalNotification.GetSpec().GetNotification()
			notification.Metadata.Name = globalNotification.GetMetadata().GetName()
			return notification
		},
	)
	var notifications []*notificationsv1.Notification
	var nextGlobalKey, nextUserKey string
	for notifStream.Next() {
		item := notifStream.Item()
		if item != nil {
			notifications = append(notifications, item)
		}
		if len(notifications) == pageSize {
			// The nextKeys should represent the next unconsumed items in their respective lists.
			// If the last item in this page (ie. the current item in the stream) was a user-specific notification, then the userNotificationsNextKey should be the next item in the userNotifsStream, and
			// the globalNotificationsNextKey should be the current (and unconsumed) item in the globalNotifsStream. And vice-versa.
			if item.GetMetadata().GetLabels()[types.NotificationScope] == "user" {
				// If the provided globalKey was "", then return that as the nextGlobalKey again.
				if globalKey != "" || !found {
					nextGlobalKey = globalNotifsStream.Item().GetMetadata().GetName()
				} else {
					nextGlobalKey = ""
				}
				// Advance to the next user-specific notification.
				ok := userNotifsStream.Next()
				if ok {
					// If it exists, set it as the userNotificationsNextKey, otherwise set it to ""
					nextUserKey = userNotifsStream.Item().GetMetadata().GetName()
				} else {
					nextUserKey = ""
				}
			} else {
				// If the provided userKey was "", then return that as the nextUserKey again.
				if userKey != "" || !found {
					nextUserKey = userNotifsStream.Item().GetMetadata().GetName()
				} else {
					nextUserKey = ""
				}
				// Advance to the next global notification.
				ok := globalNotifsStream.Next()
				if ok {
					// If it exists, set it as the globalNotificationsNextKey.
					nextGlobalKey = globalNotifsStream.Item().GetMetadata().GetName()
				} else {
					nextGlobalKey = ""
				}
			}
			break
		}
	}
	userLastSeenNotification, err := s.backend.GetUserLastSeenNotification(ctx, username)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	// Add label to indicate notifications that the user has clicked.
	for _, notification := range notifications {
		if (notificationStatesMap[notification.GetMetadata().GetName()]) == notificationsv1.NotificationState_NOTIFICATION_STATE_CLICKED {
			notification.GetMetadata().Labels[types.NotificationClickedLabel] = "true"
		}
	}
	var nextPageToken string
	if nextUserKey != "" || nextGlobalKey != "" {
		nextPageToken = fmt.Sprintf("%s,%s", nextUserKey, nextGlobalKey)
	}
	response := &notificationsv1.ListNotificationsResponse{
		Notifications:                     notifications,
		NextPageToken:                     nextPageToken,
		UserLastSeenNotificationTimestamp: userLastSeenNotification.GetStatus().GetLastSeenTime(),
	}
	return response, nil
}

func (s *Service) matchGlobalNotification(ctx context.Context, authCtx *authz.Context, gn *notificationsv1.GlobalNotification, notificationStatesMap map[string]notificationsv1.NotificationState) bool {
	// If the user has dismissed this notification, return false.
	if notificationStatesMap[gn.GetMetadata().GetName()] == notificationsv1.NotificationState_NOTIFICATION_STATE_DISMISSED {
		return false
	}

	// If the user is explicitly excluded by the notification, return false early.
	if gn.Spec.ExcludeUsers != nil && slices.Contains(gn.Spec.ExcludeUsers, authCtx.User.GetName()) {
		return false
	}

	switch matcher := gn.Spec.Matcher.(type) {
	case *notificationsv1.GlobalNotificationSpec_All:
		// Always return true if the matcher is "all."
		return true

	case *notificationsv1.GlobalNotificationSpec_ByUsers:
		userList := matcher.ByUsers.GetUsers()
		return slices.Contains(userList, authCtx.User.GetName())

	case *notificationsv1.GlobalNotificationSpec_ByRoles:
		matcherRoles := matcher.ByRoles.GetRoles()
		userRoles := authCtx.User.GetRoles()

		// If MatchAllConditions is true, then userRoles must contain every role in matcherRoles.
		if gn.Spec.MatchAllConditions {
			for _, matcherRole := range matcherRoles {
				// Return false if there is any role missing.
				if !slices.Contains(userRoles, matcherRole) {
					return false
				}
			}
			return true
		}

		// Return true if it matches at least one matcherRole.
		for _, matcherRole := range matcherRoles {
			if slices.Contains(userRoles, matcherRole) {
				return true
			}
		}

		return false

	case *notificationsv1.GlobalNotificationSpec_ByPermissions:
		roleConditionsList := matcher.ByPermissions.GetRoleConditions()

		var results []bool
		for _, roleConditions := range roleConditionsList {
			match, err := s.matchRoleConditions(ctx, authCtx, roleConditions)
			if err != nil {
				slog.WarnContext(ctx, "Encountered error while matching RoleConditions", "role_conditions", roleConditions, "error", err)
				return false
			}

			// If MatchAllConditions is false, we can exit at the first match.
			if !gn.Spec.MatchAllConditions && match {
				return true
			}

			// If MatchAllConditions is true, we exit at the first non-match.
			if gn.Spec.MatchAllConditions && !match {
				return false
			}

			results = append(results, match)
		}

		// Return false if any of the roleConditions didn't match.
		if gn.Spec.MatchAllConditions {
			return !slices.Contains(results, false)
		}

		return false
	}

	return false
}

func (s *Service) matchRoleConditions(ctx context.Context, authCtx *authz.Context, rc *types.RoleConditions) (bool, error) {
	if len(rc.Logins) > 0 {
		userLogins := authCtx.Checker.GetAllLogins()
		var matchedLogin bool
		for _, login := range rc.Logins {
			// If at least one  of the logins match, this is a match.
			if slices.Contains(userLogins, login) {
				matchedLogin = true
				break
			}
		}
		if !matchedLogin {
			return false, nil
		}
	}

	if len(rc.Rules) > 0 {
		var matchedRule bool
		for _, rule := range rc.Rules {
			hasAccess, err := checkAccessToRule(authCtx, rule)
			if err != nil {
				return false, trace.Wrap(err, "encountered unexpected error when checking access to rule")
			}

			// If the user has permissions for at least one of the rules, this is a match.
			if hasAccess {
				matchedRule = true
				break
			}
		}
		if !matchedRule {
			return false, nil
		}
	}

	if rc.ReviewRequests != nil {
		identity := authCtx.Identity.GetIdentity()
		checker, err := services.NewReviewPermissionChecker(
			ctx,
			s.backend,
			authCtx.User.GetName(),
			&identity,
		)
		if err != nil {
			return false, trace.Wrap(err)
		}

		// unless the user has allow directives for reviewing, they will never be able to
		// see any requests other than their own.
		if !checker.HasAllowDirectives() {
			return false, nil
		}

		// We instantiate a fake access request with the defined roles, this allows us to use our existing AccessReviewChecker to check if the
		// user is allowed to review requests for them.
		fakeAccessRequest, err := types.NewAccessRequest("fake", "fake", rc.ReviewRequests.Roles...)
		if err != nil {
			return false, trace.Wrap(err)
		}

		canReview, err := checker.CanReviewRequest(fakeAccessRequest)
		if err != nil {
			return false, trace.Wrap(err, "failed to evaluate request review permissions")
		}

		if !canReview {
			return false, nil
		}
	}

	// This RoleConditions object matches if there were no failed matches that returned prior to this.
	return true, nil
}

// checkAccessToRule returns true if the user has the permissions defined in a rule.
func checkAccessToRule(authCtx *authz.Context, rule types.Rule) (bool, error) {
	for _, resourceKind := range rule.Resources {
		for _, verb := range rule.Verbs {
			if err := authCtx.CheckAccessToKind(resourceKind, verb); err != nil {
				// If the user doesn't have access for the verbs for any one of the resources in the rule, then just return false.
				if trace.IsAccessDenied(err) {
					return false, nil
					// If the error is due to something else, then return it.
				} else {
					return false, trace.Wrap(err)
				}
			}
		}
	}

	return true, nil
}

// isMoreRecent returns true if the userNotification is more recent than the globalNotification.
func isMoreRecent(userNotification *notificationsv1.Notification, globalNotification *notificationsv1.GlobalNotification) bool {
	userNotificationTime := userNotification.GetSpec().GetCreated().AsTime()
	globalNotificationTime := globalNotification.GetSpec().GetNotification().GetSpec().GetCreated().AsTime()

	return userNotificationTime.After(globalNotificationTime)
}

// UpsertUserNotificationState creates or updates a user notification state which records whether the user has clicked on or dismissed a notification.
func (s *Service) UpsertUserNotificationState(ctx context.Context, req *notificationsv1.UpsertUserNotificationStateRequest) (*notificationsv1.UserNotificationState, error) {
	if req.Username == "" {
		return nil, trace.BadParameter("missing username")
	}
	if req.UserNotificationState == nil {
		return nil, trace.BadParameter("missing notification state")
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	username := authCtx.User.GetName()
	if username != req.Username {
		return nil, trace.AccessDenied("a user may only update their own notification state")
	}

	out, err := s.backend.UpsertUserNotificationState(ctx, req.Username, req.UserNotificationState)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return out, nil
}

// UpsertUserLastSeenNotification creates or updates a user's last seen notification timestamp.
func (s *Service) UpsertUserLastSeenNotification(ctx context.Context, req *notificationsv1.UpsertUserLastSeenNotificationRequest) (*notificationsv1.UserLastSeenNotification, error) {
	if req.Username == "" {
		return nil, trace.BadParameter("missing username")
	}
	if req.UserLastSeenNotification == nil {
		return nil, trace.BadParameter("missing user last seen notification")
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	username := authCtx.User.GetName()
	if username != req.Username {
		return nil, trace.AccessDenied("a user may only update their own last seen notification timestamp")
	}

	out, err := s.backend.UpsertUserLastSeenNotification(ctx, req.Username, req.UserLastSeenNotification)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return out, nil
}

// CreateGlobalNotification creates a global notification.
func (s *Service) CreateGlobalNotification(ctx context.Context, req *notificationsv1.CreateGlobalNotificationRequest) (*notificationsv1.GlobalNotification, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindNotification, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	out, err := s.backend.CreateGlobalNotification(ctx, req.GlobalNotification)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return out, nil
}

// CreateUserNotification creates a user-specific notification.
func (s *Service) CreateUserNotification(ctx context.Context, req *notificationsv1.CreateUserNotificationRequest) (*notificationsv1.Notification, error) {
	if req.Username == "" {
		return nil, trace.BadParameter("missing username")
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindNotification, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	out, err := s.backend.CreateUserNotification(ctx, req.Notification)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return out, nil
}

// DeleteGlobalNotification deletes a global notification.
func (s *Service) DeleteGlobalNotification(ctx context.Context, req *notificationsv1.DeleteGlobalNotificationRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindNotification, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.backend.DeleteGlobalNotification(ctx, req.NotificationId)
	return nil, trace.Wrap(err)
}

// DeleteUserNotification deletes a user-specific notification.
func (s *Service) DeleteUserNotification(ctx context.Context, req *notificationsv1.DeleteUserNotificationRequest) (*emptypb.Empty, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindNotification, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.backend.DeleteUserNotification(ctx, req.Username, req.NotificationId)
	return nil, trace.Wrap(err)
}

// listUserSpecificNotificationsForUser returns a paginated list of all user-specific notifications for a user. This should only be used by admins.
func (s *Service) listUserSpecificNotificationsForUser(ctx context.Context, req *notificationsv1.ListNotificationsRequest) (*notificationsv1.ListNotificationsResponse, error) {
	if req.GetFilters().GetUsername() == "" {
		return nil, trace.BadParameter("missing username")
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authCtx, string(types.RoleAdmin)) {
		return nil, trace.AccessDenied("only RoleAdmin can list notifications for a specific user")
	}

	if err := authCtx.CheckAccessToKind(types.KindNotification, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	stream := stream.FilterMap(
		s.userNotificationCache.StreamUserNotifications(ctx, req.GetFilters().GetUsername(), req.GetPageToken()),
		func(n *notificationsv1.Notification) (*notificationsv1.Notification, bool) {
			// If only user-created notifications are requested, filter by the user-created subkinds.
			if req.GetFilters().GetUserCreatedOnly() &&
				n.GetSubKind() != types.NotificationUserCreatedInformationalSubKind &&
				n.GetSubKind() != types.NotificationUserCreatedWarningSubKind {
				return nil, false
			}

			for k, v := range req.GetFilters().GetLabels() {
				if n.GetMetadata().GetLabels()[k] != v {
					return nil, false
				}
			}

			return n, true
		})

	var notifications []*notificationsv1.Notification
	var nextKey string

	for stream.Next() {
		item := stream.Item()
		if item != nil {
			notifications = append(notifications, item)
			if len(notifications) == int(req.GetPageSize()) {
				nextKey = item.GetMetadata().GetName()
				break
			}
		}
	}

	return &notificationsv1.ListNotificationsResponse{
		Notifications: notifications,
		NextPageToken: nextKey,
	}, nil
}

// listGlobalNotifications returns a paginated list of all global notifications. This should only be used by admins.
func (s *Service) listGlobalNotifications(ctx context.Context, req *notificationsv1.ListNotificationsRequest) (*notificationsv1.ListNotificationsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authCtx, string(types.RoleAdmin)) {
		return nil, trace.AccessDenied("only RoleAdmin can list all global notifications")
	}

	stream := stream.FilterMap(
		s.globalNotificationCache.StreamGlobalNotifications(ctx, req.GetPageToken()),
		func(gn *notificationsv1.GlobalNotification) (*notificationsv1.GlobalNotification, bool) {
			// If only user-created notifications are requested, filter by the user-creatd subkinds.
			if req.GetFilters().GetUserCreatedOnly() &&
				gn.GetSpec().GetNotification().GetSubKind() != types.NotificationUserCreatedInformationalSubKind &&
				gn.GetSpec().GetNotification().GetSubKind() != types.NotificationUserCreatedWarningSubKind {
				return nil, false
			}

			// Pay special attention to the fact that we match against labels on the
			// inner-notification resource spec, not the labels in the GlobalNotification's
			// resource metadata.
			for k, v := range req.GetFilters().GetLabels() {
				if gn.GetSpec().GetNotification().GetMetadata().GetLabels()[k] != v {
					return nil, false
				}
			}

			return gn, true
		})

	var notifications []*notificationsv1.Notification
	var nextKey string

	for stream.Next() {
		item := stream.Item()
		if item != nil {
			notification := item.GetSpec().GetNotification()
			notification.Metadata.Name = item.GetMetadata().GetName()

			notifications = append(notifications, notification)

			if len(notifications) == int(req.GetPageSize()) {
				nextKey = item.GetMetadata().GetName()
				break
			}
		}
	}

	return &notificationsv1.ListNotificationsResponse{
		Notifications: notifications,
		NextPageToken: nextKey,
	}, nil
}
