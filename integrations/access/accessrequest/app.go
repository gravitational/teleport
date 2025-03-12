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

package accessrequest

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/accessrequest"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/accessmonitoring"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
	pd "github.com/gravitational/teleport/integrations/lib/plugindata"
	"github.com/gravitational/teleport/integrations/lib/watcherjob"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

const (
	// handlerTimeout is used to bound the execution time of watcher event handler.
	handlerTimeout = time.Second * 5
)

// App is the access request application for plugins. This will notify when access requests
// are created and reviewed.
type App struct {
	pluginName string
	pluginType string
	apiClient  teleport.Client
	recipients common.RawRecipientsMap
	pluginData *pd.CompareAndSwap[PluginData]
	bot        MessagingBot
	job        lib.ServiceJob

	accessMonitoringRules *accessmonitoring.RuleHandler
	// teleportUser is the name of the Teleport user that will act as the
	// access request approver.
	teleportUser string
}

// NewApp will create a new access request application.
func NewApp(bot MessagingBot) common.App {
	app := &App{}
	app.job = lib.NewServiceJob(app.run)
	return app
}

func (a *App) Init(baseApp *common.BaseApp) error {
	a.pluginName = baseApp.PluginName
	a.pluginType = string(baseApp.Conf.GetPluginType())
	a.apiClient = baseApp.APIClient
	a.recipients = baseApp.Conf.GetRecipients()
	a.pluginData = pd.NewCAS(
		a.apiClient,
		a.pluginName,
		types.KindAccessRequest,
		EncodePluginData,
		DecodePluginData,
	)

	var ok bool
	a.bot, ok = baseApp.Bot.(MessagingBot)
	if !ok {
		return trace.BadParameter("bot does not implement access request bot methods")
	}

	a.accessMonitoringRules = accessmonitoring.NewRuleHandler(accessmonitoring.RuleHandlerConfig{
		Client:                 a.apiClient,
		PluginType:             a.pluginType,
		PluginName:             a.pluginName,
		FetchRecipientCallback: a.bot.FetchRecipient,
	})
	a.teleportUser = baseApp.Conf.GetTeleportUser()

	return nil
}

// Start will start the application.
func (a *App) Start(process *lib.Process) {
	process.SpawnCriticalJob(a.job)
}

// WaitReady will block until the job is ready.
func (a *App) WaitReady(ctx context.Context) (bool, error) {
	return a.job.WaitReady(ctx)
}

// WaitForDone will wait until the job has completed.
func (a *App) WaitForDone() {
	<-a.job.Done()
}

// Err will return the error associated with the underlying job.
func (a *App) Err() error {
	if a.job != nil {
		return a.job.Err()
	}

	return nil
}

func (a *App) run(ctx context.Context) error {
	process := lib.MustGetProcess(ctx)

	watchKinds := []types.WatchKind{
		{Kind: types.KindAccessRequest},
		{Kind: types.KindAccessMonitoringRule},
	}

	acceptedWatchKinds := make([]string, 0, len(watchKinds))
	job, err := watcherjob.NewJobWithConfirmedWatchKinds(
		a.apiClient,
		watcherjob.Config{
			Watch:            types.Watch{Kinds: watchKinds, AllowPartialSuccess: true},
			EventFuncTimeout: handlerTimeout,
		},
		a.onWatcherEvent,
		func(ws types.WatchStatus) {
			for _, watchKind := range ws.GetKinds() {
				acceptedWatchKinds = append(acceptedWatchKinds, watchKind.Kind)
			}
		},
	)
	if err != nil {
		return trace.Wrap(err)
	}

	process.SpawnCriticalJob(job)

	ok, err := job.WaitReady(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(acceptedWatchKinds) == 0 {
		return trace.BadParameter("failed to initialize watcher for all the required resources: %+v",
			watchKinds)
	}
	// Check if KindAccessMonitoringRule resources are being watched,
	// the role the plugin is running as may not have access.
	if slices.Contains(acceptedWatchKinds, types.KindAccessMonitoringRule) {
		if err := a.accessMonitoringRules.InitAccessMonitoringRulesCache(ctx); err != nil {
			return trace.Wrap(err, "initializing Access Monitoring Rule cache")
		}
	}

	a.job.SetReady(ok)
	if !ok {
		return trace.BadParameter("job not ready")
	}

	<-job.Done()
	return nil
}

// onWatcherEvent is called for every cluster Event. It will filter out non-access-request events and
// call onPendingRequest, onResolvedRequest and on DeletedRequest depending on the event.
func (a *App) onWatcherEvent(ctx context.Context, event types.Event) error {
	switch event.Resource.GetKind() {
	case types.KindAccessMonitoringRule:
		return trace.Wrap(a.accessMonitoringRules.HandleAccessMonitoringRule(ctx, event))
	case types.KindAccessRequest:
		return trace.Wrap(a.handleAccessRequest(ctx, event))
	}
	return trace.BadParameter("unexpected kind %s", event.Resource.GetKind())
}

func (a *App) handleAccessRequest(ctx context.Context, event types.Event) error {
	op := event.Type
	reqID := event.Resource.GetName()
	ctx, _ = logger.With(ctx, "request_id", reqID)

	switch op {
	case types.OpPut:
		ctx, _ = logger.With(ctx, "request_op", "put")
		req, ok := event.Resource.(types.AccessRequest)
		if !ok {
			return trace.BadParameter("unexpected resource type %T", event.Resource)
		}
		ctx, log := logger.With(ctx, "request_state", req.GetState().String())

		var err error
		switch {
		case req.GetState().IsPending():
			err = a.onPendingRequest(ctx, req)
		case req.GetState().IsResolved():
			err = a.onResolvedRequest(ctx, req)
		default:
			log.WarnContext(ctx, "Unknown request state",
				slog.Group("event",
					slog.Any("type", logutils.StringerAttr(event.Type)),
					slog.Group("resource",
						"kind", event.Resource.GetKind(),
						"name", event.Resource.GetName(),
					),
				),
			)
			return nil
		}

		if err != nil {
			log.ErrorContext(ctx, "Failed to process request", "error", err)
			return trace.Wrap(err)
		}

		return nil
	case types.OpDelete:
		ctx, log := logger.With(ctx, "request_op", "delete")

		if err := a.onDeletedRequest(ctx, reqID); err != nil {
			log.ErrorContext(ctx, "Failed to process deleted request", "error", err)
			return trace.Wrap(err)
		}
		return nil
	default:
		return trace.BadParameter("unexpected event operation %s", op)
	}
}

func (a *App) onPendingRequest(ctx context.Context, req types.AccessRequest) error {
	log := logger.Get(ctx)

	reqID := req.GetName()

	resourceNames, err := a.getResourceNames(ctx, req)
	if err != nil {
		return trace.Wrap(err)
	}

	loginsByRole, err := a.getLoginsByRole(ctx, req)
	if trace.IsAccessDenied(err) {
		log.WarnContext(ctx, "Missing permissions to get logins by role, please add role.read to the associated role", "error", err)
	} else if err != nil {
		return trace.Wrap(err)
	}

	reqData := pd.AccessRequestData{
		User:              req.GetUser(),
		Roles:             req.GetRoles(),
		RequestReason:     req.GetRequestReason(),
		SystemAnnotations: req.GetSystemAnnotations(),
		Resources:         resourceNames,
		LoginsByRole:      loginsByRole,
	}

	_, err = a.pluginData.Create(ctx, reqID, PluginData{AccessRequestData: reqData})
	switch {
	case err == nil:
		// This is a new access-request, we have to broadcast it first.
		if recipients := a.getMessageRecipients(ctx, req); len(recipients) > 0 {
			if err := a.broadcastAccessRequestMessages(ctx, recipients, reqID, reqData); err != nil {
				return trace.Wrap(err)
			}
		} else {
			log.WarnContext(ctx, "No channel to post")
		}

		// Try to approve the request if user is currently on-call.
		if err := a.tryApproveRequest(ctx, reqID, req); err != nil {
			log.WarnContext(ctx, "Failed to auto approve request", "error", err)
		}
	case trace.IsAlreadyExists(err):
		// The messages were already sent, nothing to do, we can update the reviews
	default:
		// This is an unexpected error, returning
		return trace.Wrap(err)
	}

	// This is an already existing access request, we post reviews and update its status
	if reqReviews := req.GetReviews(); len(reqReviews) > 0 {
		if err := a.postReviewReplies(ctx, reqID, reqReviews); err != nil {
			return trace.Wrap(err)
		}

		err := a.updateMessages(ctx, reqID, pd.Unresolved, "", reqReviews)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func (a *App) onResolvedRequest(ctx context.Context, req types.AccessRequest) error {
	// We always post review replies in thread. If the messaging service does not support
	// threading this will do nothing
	replyErr := a.postReviewReplies(ctx, req.GetName(), req.GetReviews())

	reason := req.GetResolveReason()
	state := req.GetState()
	var tag pd.ResolutionTag

	switch state {
	case types.RequestState_APPROVED:
		tag = pd.ResolvedApproved
	case types.RequestState_DENIED:
		tag = pd.ResolvedDenied
	case types.RequestState_PROMOTED:
		tag = pd.ResolvedPromoted
	default:
		logger.Get(ctx).WarnContext(ctx, "Unknown state", "state", logutils.StringerAttr(state))
		return replyErr
	}
	err := trace.Wrap(a.updateMessages(ctx, req.GetName(), tag, reason, req.GetReviews()))
	return trace.NewAggregate(replyErr, err)
}

func (a *App) onDeletedRequest(ctx context.Context, reqID string) error {
	return a.updateMessages(ctx, reqID, pd.ResolvedExpired, "", nil)
}

// broadcastAccessRequestMessages sends nessages to each recipient for an access-request.
// This method is only called when for new access-requests.
func (a *App) broadcastAccessRequestMessages(ctx context.Context, recipients []common.Recipient, reqID string, reqData pd.AccessRequestData) error {
	sentMessages, err := a.bot.BroadcastAccessRequestMessage(ctx, recipients, reqID, reqData)
	if len(sentMessages) == 0 && err != nil {
		return trace.Wrap(err)
	}
	for _, data := range sentMessages {
		logger.Get(ctx).InfoContext(ctx, "Successfully posted messages",
			"channel_id", data.ChannelID,
			"message_id", data.MessageID,
		)
	}
	if err != nil {
		logger.Get(ctx).ErrorContext(ctx, "Failed to post one or more messages", "error", err)
	}

	_, err = a.pluginData.Update(ctx, reqID, func(existing PluginData) (PluginData, error) {
		existing.SentMessages = sentMessages
		return existing, nil
	})

	return trace.Wrap(err)
}

// postReviewReplies lists and updates existing messages belonging to an access request.
// Posting reviews is done both by updating the original message and by replying in thread if possible.
func (a *App) postReviewReplies(ctx context.Context, reqID string, reqReviews []types.AccessReview) error {
	var oldCount int

	pd, err := a.pluginData.Update(ctx, reqID, func(existing PluginData) (PluginData, error) {
		sentMessages := existing.SentMessages
		if len(sentMessages) == 0 {
			// wait for the plugin data to be updated with SentMessages
			return PluginData{}, trace.CompareFailed("existing sentMessages is empty")
		}

		count := len(reqReviews)
		oldCount = existing.ReviewsCount
		if oldCount >= count {
			return PluginData{}, trace.AlreadyExists("reviews are sent already")
		}

		existing.ReviewsCount = count
		return existing, nil
	})
	if trace.IsAlreadyExists(err) {
		logger.Get(ctx).DebugContext(ctx, "Failed to post reply: replies are already sent")
		return nil
	}
	if err != nil {
		return trace.Wrap(err)
	}

	slice := reqReviews[oldCount:]
	if len(slice) == 0 {
		return nil
	}

	errors := make([]error, 0, len(slice))
	for _, data := range pd.SentMessages {
		ctx, _ = logger.With(ctx, "channel_id", data.ChannelID, "message_id", data.MessageID)
		for _, review := range slice {
			if err := a.bot.PostReviewReply(ctx, data.ChannelID, data.MessageID, review); err != nil {
				errors = append(errors, err)
			}
		}
	}
	return trace.NewAggregate(errors...)
}

// getMessageRecipients takes an access request and returns a list of channelIDs that should be messaged.
// channelIDs can represent any communication channel depending on the MessagingBot implementation:
// a public channel, a private one, or a user direct message channel.
func (a *App) getMessageRecipients(ctx context.Context, req types.AccessRequest) []common.Recipient {
	log := logger.Get(ctx)

	// We receive a set from GetRawRecipientsFor but we still might end up with duplicate channel names.
	// This can happen if this set contains the channel `C` and the email for channel `C`.
	recipientSet := common.NewRecipientSet()

	recipients := a.accessMonitoringRules.RecipientsFromAccessMonitoringRules(ctx, req)
	recipients.ForEach(func(r common.Recipient) {
		recipientSet.Add(r)
	})
	if recipientSet.Len() != 0 {
		return recipientSet.ToSlice()
	}

	switch a.pluginType {
	case types.PluginTypeServiceNow:
		// The ServiceNow plugin does not use recipients currently and create incidents in the incident table directly.
		// Recipients just needs to be non empty.
		recipientSet.Add(common.Recipient{})
		return recipientSet.ToSlice()
	case types.PluginTypeOpsgenie:
		recipients, ok := req.GetSystemAnnotations()[types.TeleportNamespace+types.ReqAnnotationNotifySchedulesLabel]
		if !ok {
			return recipientSet.ToSlice()
		}
		for _, recipient := range recipients {
			rec, err := a.bot.FetchRecipient(ctx, recipient)
			if err != nil {
				log.WarnContext(ctx, "Failed to fetch Opsgenie recipient", "error", err)
				continue
			}
			recipientSet.Add(*rec)
		}
		return recipientSet.ToSlice()
	}

	validEmailSuggReviewers := []string{}
	for _, reviewer := range req.GetSuggestedReviewers() {
		if !lib.IsEmail(reviewer) {
			log.WarnContext(ctx, "Failed to notify a suggested reviewer with an invalid email address", "reviewer", reviewer)
			continue
		}

		validEmailSuggReviewers = append(validEmailSuggReviewers, reviewer)
	}
	rawRecipients := a.recipients.GetRawRecipientsFor(req.GetRoles(), validEmailSuggReviewers)
	for _, rawRecipient := range rawRecipients {
		recipient, err := a.bot.FetchRecipient(ctx, rawRecipient)
		if err != nil {
			log.WarnContext(ctx, "Failure when fetching recipient, continuing anyway", "error", err)
		} else {
			recipientSet.Add(*recipient)
		}
	}

	return recipientSet.ToSlice()
}

// updateMessages updates the messages status and adds the resolve reason.
func (a *App) updateMessages(ctx context.Context, reqID string, tag pd.ResolutionTag, reason string, reviews []types.AccessReview) error {
	log := logger.Get(ctx)

	pluginData, err := a.pluginData.Update(ctx, reqID, func(existing PluginData) (PluginData, error) {
		if len(existing.SentMessages) == 0 {
			return PluginData{}, trace.NotFound("plugin data not found")
		}

		// If resolution field is not empty then we already resolved the incident before. In this case we just quit.
		if existing.AccessRequestData.ResolutionTag != pd.Unresolved {
			return PluginData{}, trace.AlreadyExists("request is already resolved")
		}

		// Mark plugin data as resolved.
		existing.ResolutionTag = tag
		existing.ResolutionReason = reason

		return existing, nil
	})
	if trace.IsNotFound(err) {
		log.DebugContext(ctx, "Failed to update messages: plugin data is missing")
		return nil
	}
	if trace.IsAlreadyExists(err) {
		if tag != pluginData.ResolutionTag {
			return trace.WrapWithMessage(err,
				"cannot change the resolution tag of an already resolved request, existing: %s, event: %s",
				pluginData.ResolutionTag, tag)
		}
		log.DebugContext(ctx, "Request is already resolved, ignoring event")
		return nil
	}
	if err != nil {
		return trace.Wrap(err)
	}

	reqData, sentMessages := pluginData.AccessRequestData, pluginData.SentMessages
	if err := a.bot.UpdateMessages(ctx, reqID, reqData, sentMessages, reviews); err != nil {
		return trace.Wrap(err)
	}

	log.InfoContext(ctx, "Marked request with resolution and sent emails!", "resolution", tag)

	if err := a.bot.NotifyUser(ctx, reqID, reqData); err != nil && !trace.IsNotImplemented(err) {
		return trace.Wrap(err)
	}

	log.InfoContext(ctx, "Successfully notified user",
		"user", reqData.User,
		"resolution", tag,
	)

	return nil
}

func (a *App) getLoginsByRole(ctx context.Context, req types.AccessRequest) (map[string][]string, error) {
	loginsByRole := make(map[string][]string, len(req.GetRoles()))

	for _, role := range req.GetRoles() {
		currentRole, err := a.apiClient.GetRole(ctx, role)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		loginsByRole[role] = currentRole.GetLogins(types.Allow)
	}

	return loginsByRole, nil
}

func (a *App) getResourceNames(ctx context.Context, req types.AccessRequest) ([]string, error) {
	resourceNames := make([]string, 0, len(req.GetRequestedResourceIDs()))
	resourcesByCluster := accessrequest.GetResourceIDsByCluster(req)

	for cluster, resources := range resourcesByCluster {
		resourceDetails, err := accessrequest.GetResourceDetails(ctx, cluster, a.apiClient, resources)
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

// tryApproveRequest attempts to automatically approve the access request if the
// user is on call for the configured service/team.
func (a *App) tryApproveRequest(ctx context.Context, reqID string, req types.AccessRequest) error {
	log := logger.Get(ctx).With("req_id", reqID, "user", req.GetUser())

	oncallUsers, err := a.bot.FetchOncallUsers(ctx, req)
	if trace.IsNotImplemented(err) {
		log.DebugContext(ctx, "Skipping auto-approval because bot does not support automatic approvals", "bot", a.pluginName)
		return nil
	}
	if err != nil {
		return trace.Wrap(err)
	}

	if !slices.Contains(oncallUsers, req.GetUser()) {
		log.DebugContext(ctx, "Skipping approval because user is not on-call")
		return nil
	}

	if _, err := a.apiClient.SubmitAccessReview(ctx, types.AccessReviewSubmission{
		RequestID: reqID,
		Review: types.AccessReview{
			Author:        a.teleportUser,
			ProposedState: types.RequestState_APPROVED,
			Reason:        fmt.Sprintf("Access request has been automatically approved by %q plugin because user %q is on-call.", a.pluginName, req.GetUser()),
			Created:       time.Now(),
		},
	}); err != nil {
		if strings.HasSuffix(err.Error(), "has already reviewed this request") {
			log.DebugContext(ctx, "Request has already been reviewed")
			return nil
		}
		return trace.Wrap(err)
	}

	log.InfoContext(ctx, "Successfully submitted a request approval")
	return nil
}
