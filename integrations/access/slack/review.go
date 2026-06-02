/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package slack

import (
	"context"
	"strings"
	"time"

	usersv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

const (
	actionsBlockID  = "review_block"
	approveButtonID = "approve_button"
	denyButtonID    = "deny_button"
	// cacheTTL is the cache TTL for resolving Slack user ID to Teleport username.
	cacheTTL = 10 * time.Minute
)

// ReviewBot is the bot interface defining the methods required to support Slack native review.
type ReviewBot interface {
	common.MessagingBot

	GenerateWebSocketURL(ctx context.Context) (string, error)
	LookupEmailByUserID(ctx context.Context, userID string) (string, error)
	PostReviewErrorReply(ctx context.Context, channelID, userID string, reviewErr error) error
}

// ReviewApp is the access review application for the Slack plugin. This will send review requests
// to the Teleport Auth Server on behalf of Slack users when they interact with an access request
// notification.
type ReviewApp struct {
	apiClient        teleport.Client
	socketModeClient *SocketModeClient
	bot              ReviewBot
	conf             ReviewConfig
	userCache        *utils.FnCache
	clock            clockwork.Clock
	job              lib.ServiceJob
}

// NewReviewApp creates a new access review application.
func NewReviewApp(reviewConfig ReviewConfig) *ReviewApp {
	reviewApp := &ReviewApp{
		conf: reviewConfig,
	}

	reviewApp.job = lib.NewServiceJob(reviewApp.run)
	return reviewApp
}

// Init initializes the Teleport client and Socket Mode client for the review app.
func (a *ReviewApp) Init(baseApp *common.BaseApp) error {
	a.apiClient = baseApp.APIClient
	a.clock = baseApp.Clock

	var ok bool
	a.bot, ok = baseApp.Bot.(ReviewBot)
	if !ok {
		return trace.BadParameter("bot does not support native review")
	}
	a.socketModeClient = NewSocketModeClient(a.bot)

	return nil
}

// Start will start the application.
func (a *ReviewApp) Start(process *lib.Process) {
	process.SpawnCriticalJob(a.job)
}

// WaitReady will block until the job is ready.
func (a *ReviewApp) WaitReady(ctx context.Context) (bool, error) {
	return a.job.WaitReady(ctx)
}

// WaitForDone will wait until the job has completed.
func (a *ReviewApp) WaitForDone() {
	<-a.job.Done()
}

// Err will return the error associated with the underlying job.
func (a *ReviewApp) Err() error {
	if a.job != nil {
		return a.job.Err()
	}

	return nil
}

func (a *ReviewApp) run(ctx context.Context) error {
	process := lib.MustGetProcess(ctx)
	ctx, cancel := context.WithCancel(ctx)

	process.OnTerminate(func(_ context.Context) error {
		cancel()
		return nil
	})

	log := logger.Get(ctx)

	userCache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:         cacheTTL,
		Context:     ctx,
		Clock:       a.clock,
		ReloadOnErr: true,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	a.userCache = userCache

	log.InfoContext(ctx, "Review app is running")

	a.job.SetReady(true)

	// Ingest interaction events from Socket Mode client.
	// Currently, we only handle "block_actions" interactions.
	go func() {
		for {
			select {
			case interaction := <-a.socketModeClient.Interactions():
				switch interaction.Type() {
				case BlockActionsEvent{}.Type():
					blockActionsEvent, ok := interaction.(BlockActionsEvent)
					if !ok {
						log.ErrorContext(ctx, "Error unmarshaling `block_actions` interaction event, this is a bug")
						continue
					}
					a.handleBlockActionsEvent(ctx, blockActionsEvent)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return a.socketModeClient.Run(ctx)
}

// handleBlockActionsEvent extracts information from the event payload to prepare the review request.
func (a *ReviewApp) handleBlockActionsEvent(ctx context.Context, blockActionsEvent BlockActionsEvent) {
	log := logger.Get(ctx)

	// Validate the `block_actions` event payload.
	if len(blockActionsEvent.Actions) != 1 {
		log.WarnContext(ctx, "Skipping unknown interaction event, invalid actions data")
		return
	}
	action := blockActionsEvent.Actions[0]
	if action.BlockID != actionsBlockID {
		log.WarnContext(ctx, "Skipping unknown interaction event, action does not match actions block_id")
		return
	}

	var reqID, slackUserID string
	var proposedState types.RequestState

	reqID = action.Value
	slackUserID = blockActionsEvent.User.ID
	if reqID == "" || slackUserID == "" {
		log.WarnContext(ctx, "Skipping unknown interaction event, payload is missing data")
		return
	}

	switch action.ID {
	case approveButtonID:
		proposedState = types.RequestState_APPROVED
	case denyButtonID:
		proposedState = types.RequestState_DENIED
	default:
		log.WarnContext(ctx, "Skipping unknown interaction event, unknown action_id", "action_id", action.ID)
		return
	}

	log.DebugContext(ctx, "Resolving review request", "req_id", reqID, "proposed_state", proposedState)

	// On review request errors, return an ephemeral reply to the user in the same channel.
	if err := a.resolveReview(ctx, reqID, slackUserID, proposedState); err != nil {
		replyErr := a.bot.PostReviewErrorReply(ctx, blockActionsEvent.Channel.ID, slackUserID, err)
		if replyErr != nil {
			log.ErrorContext(ctx, "Error posting review error reply", "error", replyErr.Error())
		}
	}
}

// resolveReview resolves the review request associated with the Access Request ID.
func (a *ReviewApp) resolveReview(ctx context.Context, reqID, slackUserID string, proposedState types.RequestState) error {
	log := logger.Get(ctx).With(
		"req_id", reqID,
		"proposed_state", proposedState,
	)

	username, err := utils.FnCacheGet(ctx, a.userCache, slackUserID, func(ctx context.Context) (string, error) {
		return a.resolveTeleportUser(ctx, slackUserID)
	})
	if err != nil {
		log.DebugContext(ctx, "Failed to resolve Slack user to Teleport user", "error", err)
		return trace.Wrap(err)
	}

	log = log.With("teleport_username", username)
	log.DebugContext(ctx, "Submitting access review")

	if _, err := a.apiClient.SubmitAccessReview(ctx, types.AccessReviewSubmission{
		RequestID: reqID,
		Review: types.AccessReview{
			Author:              username,
			IsSubmittedByPlugin: true,
			ProposedState:       proposedState,
			Created:             time.Now(),
		},
	}); err != nil {
		log.DebugContext(ctx, "Error submitting access review", "error", err.Error())

		if strings.Contains(err.Error(), "the access request has been already") {
			// Convert error to NotFound to simplify user-facing error replies.
			return trace.NotFound("request has already been resolved")
		}
		return trace.Wrap(err)
	}

	log.InfoContext(ctx, "Successfully submitted a request review")
	return nil
}

// resolveTeleportUser resolves the Slack user ID to a local Teleport user.
func (a *ReviewApp) resolveTeleportUser(ctx context.Context, slackUserID string) (string, error) {
	log := logger.Get(ctx).With(
		"slack_user_id_trait", a.conf.SlackUserIDTrait,
		"allow_email_username_match", a.conf.AllowEmailUsernameMatch,
	)

	// First try resolving to a unique Teleport user by matching the trait set by `review.slack_user_id_trait`.
	users, err := a.getUsersByTrait(ctx, a.conf.SlackUserIDTrait, slackUserID)
	if err != nil {
		return "", trace.Wrap(err)
	}

	switch len(users) {
	case 1:
		return users[0], nil
	case 0:
		// Otherwise, we fallback to an exact match of Slack email to Teleport username
		// This assumes that Slack email is trusted and unmodifiable, so must be
		// manually enabled via `review.allow_email_username_match`.
		if a.conf.AllowEmailUsernameMatch {
			slackEmail, err := a.bot.LookupEmailByUserID(ctx, slackUserID)
			if err != nil {
				log.DebugContext(ctx, "Failed to lookup Slack email by Slack user ID", "error", err)
				return "", trace.Wrap(err)
			}

			user, err := a.apiClient.GetUser(ctx, slackEmail, false)
			if err != nil {
				log.DebugContext(ctx, "Failed to fetch user from local store", "error", err)
				return "", trace.Wrap(err)
			}

			return user.GetName(), nil
		}
		return "", trace.AccessDenied("no Teleport users match slack_user_id_trait %q", a.conf.SlackUserIDTrait)
	default:
		return "", trace.AccessDenied("multiple Teleport users match slack_user_id_trait %q", a.conf.SlackUserIDTrait)
	}
}

// getUsersByTrait filters Teleport users by trait name and value.
func (a *ReviewApp) getUsersByTrait(ctx context.Context, trait, value string) ([]string, error) {
	// Don't bother listing if trait name or value are empty.
	if trait == "" || value == "" {
		return []string{}, nil
	}

	userstream := clientutils.Resources(
		ctx,
		func(ctx context.Context, limit int, token string) ([]*types.UserV2, string, error) {
			rsp, err := a.apiClient.ListUsers(ctx, &usersv1.ListUsersRequest{
				PageToken: token,
				PageSize:  int32(limit),
				Filter: &types.UserFilter{
					Traits: map[string][]string{trait: {value}},
				},
			})

			if err != nil {
				return nil, "", trace.Wrap(err)
			}

			return rsp.Users, rsp.NextPageToken, nil
		})

	var usernames []string
	for user, err := range userstream {
		if err != nil {
			return nil, trace.Wrap(err)
		}

		usernames = append(usernames, user.GetName())
	}
	return usernames, nil
}

// MsgReviewErr returns a user-facing error for bad review requests.
func MsgReviewErr(reviewErr error) string {
	switch {
	case trace.IsAccessDenied(reviewErr):
		return "Insufficient permissions to review request"
	case trace.IsAlreadyExists(reviewErr):
		return "Request has already been reviewed by user"
	case trace.IsNotFound(reviewErr):
		return "Request has already been resolved or expired"
	default:
		return "Unknown error"
	}
}
