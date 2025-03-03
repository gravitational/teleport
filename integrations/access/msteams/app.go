// Copyright 2024 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package msteams

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/accessmonitoring"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
	"github.com/gravitational/teleport/integrations/lib"
	pd "github.com/gravitational/teleport/integrations/lib/plugindata"
	"github.com/gravitational/teleport/integrations/lib/stringset"
	"github.com/gravitational/teleport/integrations/lib/watcherjob"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// pluginName used as Teleport plugin identifier
	pluginName = "msteams"
	// minServerVersion is the minimal teleport version the plugin supports.
	minServerVersion = "8.0.0"
	// initTimeout is used to bound execution time of health check and teleport version check.
	initTimeout = time.Second * 10
	// handlerTimeout is used to bound the execution time of watcher event handler.
	handlerTimeout = time.Second * 15
)

// App contains global application state.
type App struct {
	conf Config

	apiClient             teleport.Client
	bot                   *Bot
	mainJob               lib.ServiceJob
	watcherJob            lib.ServiceJob
	pd                    *pd.CompareAndSwap[PluginData]
	log                   *slog.Logger
	accessMonitoringRules *accessmonitoring.RuleHandler

	*lib.Process
}

// NewApp initializes a new teleport-msteams app and returns it.
func NewApp(conf Config) (*App, error) {
	app := &App{
		conf: conf,
		log:  slog.With("plugin", pluginName),
	}

	app.mainJob = lib.NewServiceJob(app.run)

	return app, nil
}

// Run starts the main job process
func (a *App) Run(ctx context.Context) error {
	a.log.InfoContext(ctx, "Starting Teleport MS Teams Plugin")

	err := a.init(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	a.Process = lib.NewProcess(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	a.SpawnCriticalJob(a.mainJob)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-a.Process.Done():
		return a.Err()
	}
}

// Err returns the error app finished with.
func (a *App) Err() error {
	return trace.Wrap(a.mainJob.Err())
}

// WaitReady waits for http and watcher service to start up
func (a *App) WaitReady(ctx context.Context) (bool, error) {
	return a.mainJob.WaitReady(ctx)
}

// init initializes the application
func (a *App) init(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, initTimeout)
	defer cancel()

	if a.conf.Client != nil {
		a.apiClient = a.conf.Client
	} else {
		var err error
		a.apiClient, err = common.GetTeleportClient(ctx, a.conf.Teleport)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	a.pd = pd.NewCAS(
		a.apiClient,
		pluginName,
		types.KindAccessRequest,
		EncodePluginData,
		DecodePluginData,
	)

	pong, err := a.checkTeleportVersion(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	var webProxyAddr string
	if pong.ServerFeatures.AdvancedAccessWorkflows {
		webProxyAddr = pong.ProxyPublicAddr
	}

	a.bot, err = NewBot(&a.conf, pong.ClusterName, webProxyAddr, a.log)
	if err != nil {
		return trace.Wrap(err)
	}

	a.accessMonitoringRules = accessmonitoring.NewRuleHandler(accessmonitoring.RuleHandlerConfig{
		Client:     a.apiClient,
		PluginName: pluginName,
	})

	return a.initBot(ctx)
}

// initBot initializes bot
func (a *App) initBot(ctx context.Context) error {
	teamsApp, err := a.bot.GetTeamsApp(ctx)
	if trace.IsNotFound(err) {
		return trace.Wrap(err, "MS Teams app not found in org app store.")
	}
	if err != nil {
		return trace.Wrap(err)
	}

	a.log.InfoContext(ctx, "MS Teams app found in org app store",
		"name", teamsApp.DisplayName,
		"id", teamsApp.ID)

	if err := a.bot.CheckHealth(ctx); err != nil {

		a.log.WarnContext(ctx, "MS Teams healthcheck failed",
			"name", teamsApp.DisplayName,
			"id", teamsApp.ID)
	}

	if !a.conf.Preload {
		return nil
	}

	a.log.InfoContext(ctx, "Preloading recipient data...")

	for _, recipient := range a.conf.Recipients.GetAllRawRecipients() {
		recipientData, err := a.bot.FetchRecipient(ctx, recipient)
		if err != nil {
			return trace.Wrap(err)
		}
		a.log.InfoContext(ctx, "Recipient and chat found",
			slog.Group("recipient",
				"raw", recipient,
				"recipient_chat_id", recipientData.Chat.ID,
				"recipient_kind", recipientData.Kind,
			),
		)
	}

	a.log.InfoContext(ctx, "Recipient data preloaded and cached")

	return nil
}

// run starts the main process
func (a *App) run(ctx context.Context) error {
	watchKinds := []types.WatchKind{
		{Kind: types.KindAccessRequest},
		{Kind: types.KindAccessMonitoringRule},
	}
	acceptedWatchKinds := make([]string, 0, len(watchKinds))
	watcherJob, err := watcherjob.NewJobWithConfirmedWatchKinds(
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

	process := lib.MustGetProcess(ctx)
	process.SpawnCriticalJob(watcherJob)

	ok, err := watcherJob.WaitReady(ctx)
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

	a.watcherJob = watcherJob
	a.watcherJob.SetReady(ok)
	if ok {
		a.log.InfoContext(ctx, "Plugin is ready")
	} else {
		a.log.ErrorContext(ctx, "Plugin is not ready")
	}

	a.mainJob.SetReady(ok)

	<-a.watcherJob.Done()

	return trace.Wrap(a.watcherJob.Err())
}

// checkTeleportVersion loads Teleport version and checks that it meets the minimal required
func (a *App) checkTeleportVersion(ctx context.Context) (proto.PingResponse, error) {
	a.log.DebugContext(ctx, "Checking Teleport server version")

	pong, err := a.apiClient.Ping(ctx)
	if err != nil {
		if trace.IsNotImplemented(err) {
			return pong, trace.Wrap(err, "server version must be at least %s", minServerVersion)
		}

		a.log.ErrorContext(ctx, "Unable to get Teleport server version")
		return pong, trace.Wrap(err)
	}

	err = utils.CheckMinVersion(pong.ServerVersion, minServerVersion)

	return pong, trace.Wrap(err)
}

// onWatcherEvent called when an access request event is received
func (a *App) onWatcherEvent(ctx context.Context, event types.Event) error {
	kind := event.Resource.GetKind()
	if kind == types.KindAccessMonitoringRule {
		return trace.Wrap(a.accessMonitoringRules.HandleAccessMonitoringRule(ctx, event))
	}

	if kind != types.KindAccessRequest {
		return trace.Errorf("unexpected kind %s", kind)
	}

	op := event.Type
	reqID := event.Resource.GetName()
	log := a.log.With("request_id", reqID, "request_op", op.String())

	switch op {
	case types.OpPut:
		req, ok := event.Resource.(types.AccessRequest)
		if !ok {
			return trace.Errorf("unexpected resource type %T", event.Resource)
		}
		log = log.With("request_state", req.GetState().String())
		var err error

		switch {
		case req.GetState().IsPending():
			log.DebugContext(ctx, "Pending request received")
			err = a.onPendingRequest(ctx, req)
		case req.GetState().IsApproved():
			log.DebugContext(ctx, "Approval request received")
			err = a.onResolvedRequest(ctx, req)
		case req.GetState().IsDenied():
			log.DebugContext(ctx, "Denial request received")
			err = a.onResolvedRequest(ctx, req)
		default:
			log.WarnContext(ctx, "Unknown request state", "event", event)
			return nil
		}

		if err != nil {
			log.ErrorContext(ctx, "Failed to process request", "error", err)
			return trace.Wrap(err)
		}

		return nil

	case types.OpDelete:
		log.DebugContext(ctx, "Expiration request received")

		if err := a.onDeletedRequest(ctx, reqID); err != nil {
			log.ErrorContext(ctx, "Failed to process delete request", "error", err)
			return trace.Wrap(err)
		}
		return nil
	default:
		return trace.BadParameter("unexpected event operation %s", op)
	}
}

// onPendingRequest is called when there's a new request or a review
func (a *App) onPendingRequest(ctx context.Context, req types.AccessRequest) error {
	id := req.GetName()
	data := pd.AccessRequestData{
		User:          req.GetUser(),
		Roles:         req.GetRoles(),
		RequestReason: req.GetRequestReason(),
	}

	log := a.log.With("request_id", id)
	log.DebugContext(ctx, "Claiming access request", "user", req.GetUser(), "roles", req.GetRoles(), "reason", req.GetRequestReason())

	// Let's try to create PluginData. This equals to locking AccessRequest to this
	// instance of a plugin.
	_, err := a.pd.Create(ctx, id, PluginData{AccessRequestData: data})

	// If we succeeded to create PluginData, let's post a messages and save created Teams messages
	if !trace.IsAlreadyExists(err) {
		if err != nil {
			return trace.Wrap(err)
		}
		log.DebugContext(ctx, "Access request claimed")

		recipients := a.getMessageRecipients(ctx, req)

		if len(recipients) == 0 {
			log.WarnContext(ctx, "No recipients to notify")
		} else {
			err = a.postMessages(ctx, recipients, id, data)
			if err != nil {
				return trace.Wrap(err)
			}
		}
	} else {
		log.DebugContext(ctx, "Access request already claimed, skipping initial message posting")
	}

	// Update the received reviews
	reviews := req.GetReviews()
	if len(reviews) == 0 {
		log.DebugContext(ctx, "No access request reviews to process")
		return nil
	}

	log.DebugContext(ctx, "Processing access request reviews", "review_count", len(reviews))
	err = a.postReviews(ctx, id, reviews)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// onResolvedRequest is called when a request is resolved
func (a *App) onResolvedRequest(ctx context.Context, req types.AccessRequest) error {
	var tag pd.ResolutionTag
	state := req.GetState()

	log := a.log.With("request_id", req.GetName(), "request_state", state.String())
	switch state {
	case types.RequestState_APPROVED:
		tag = pd.ResolvedApproved
	case types.RequestState_DENIED:
		tag = pd.ResolvedDenied
	default:
		log.WarnContext(ctx, "Unknown request state")
		return trace.Errorf("Unknown state")
	}
	log.DebugContext(ctx, "Updating messages to mark the request resolved")
	err := a.updateMessages(ctx, req.GetName(), tag, req.GetResolveReason(), req.GetReviews())
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// onDeleteRequest gets called when a request is deleted
func (a *App) onDeletedRequest(ctx context.Context, reqID string) error {
	a.log.DebugContext(ctx, "Updating messages to mark the request deleted/expired", "request_id", reqID, "request_state", pd.ResolvedExpired)
	return a.updateMessages(ctx, reqID, pd.ResolvedExpired, "", nil)
}

// postMessages posts initial Teams messages
func (a *App) postMessages(ctx context.Context, recipients []string, id string, data pd.AccessRequestData) error {
	teamsData, err := a.bot.PostMessages(ctx, recipients, id, data)
	if err != nil {
		if len(teamsData) == 0 {
			a.log.ErrorContext(ctx, "Failed to post all messages to MS Teams")
			return trace.Wrap(err)
		}
		a.log.ErrorContext(ctx, "Failed to post one or more messages to MS Teams, continuing")
	}

	for _, data := range teamsData {
		a.log.InfoContext(ctx, "Successfully posted to MS Teams",
			"id", data.ID,
			"timestamp", data.Timestamp,
			"recipient", data.RecipientID)
	}

	// Let's update sent messages data
	_, err = a.pd.Update(ctx, id, func(existing PluginData) (PluginData, error) {
		existing.TeamsData = teamsData
		return existing, nil
	})

	return trace.Wrap(err)
}

// postReviews updates a message with reviews
func (a *App) postReviews(ctx context.Context, id string, reviews []types.AccessReview) error {
	a.log.DebugContext(ctx, "Looking for reviews that need to be posted", "review_count", len(reviews))
	pluginData, err := a.pd.Update(ctx, id, func(existing PluginData) (PluginData, error) {
		teamsData := existing.TeamsData
		if len(teamsData) == 0 {
			// No teamsData found in the plugin data. This might be because of a race condition
			// (messages not sent yet) or because sending failed (msapi error or no recipient)
			// We don't know which one is true, so we'll still return `CompareFailed` to retry
			// TODO: find a better way to handle failures
			return existing, trace.CompareFailed("existing teamsData is empty, no messages were sent about this access request")
		}

		count := len(reviews)
		oldCount := existing.ReviewsCount
		if oldCount >= count {
			return existing, trace.AlreadyExists("reviews are sent already")
		}

		existing.ReviewsCount = count
		return existing, nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	err = a.bot.UpdateMessages(ctx, id, pluginData, reviews)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// updateMessages updates the messages status and adds the resolve reason.
func (a *App) updateMessages(ctx context.Context, reqID string, tag pd.ResolutionTag, reason string, reviews []types.AccessReview) error {
	pluginData, err := a.pd.Update(ctx, reqID, func(existing PluginData) (PluginData, error) {
		// No teamsData found in the plugin data. This might be because of a race condition
		// (messages not sent yet) or because sending failed (msapi error or no recipient)
		// We don't know which one is true, so we'll still return `CompareFailed` to retry
		// TODO: find a better way to handle failures
		if len(existing.TeamsData) == 0 {
			return existing, trace.CompareFailed("existing teamsData is empty")
		}

		// If resolution field is not empty then we already resolved the incident before. In this case we just quit.
		if existing.AccessRequestData.ResolutionTag != pd.Unresolved {
			return existing, trace.AlreadyExists("request has already been resolved, skipping message update")
		}

		// Mark plugin data as resolved.
		existing.ResolutionTag = tag
		existing.ResolutionReason = reason

		return existing, nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if err := a.bot.UpdateMessages(ctx, reqID, pluginData, reviews); err != nil {
		return trace.Wrap(err)
	}

	a.log.InfoContext(ctx, "Successfully updated all messages with the resolution", "resolution_tag", tag)

	return nil
}

// getMessageRecipients returns a recipients list for the access request
func (a *App) getMessageRecipients(ctx context.Context, req types.AccessRequest) []string {
	// We receive a set from GetRawRecipientsFor but we still might end up with duplicate channel names.
	// This can happen if this set contains the channel `C` and the email for channel `C`.
	recipientSet := stringset.New()
	a.log.DebugContext(ctx, "Getting suggested reviewer recipients")
	accessRuleRecipients := a.accessMonitoringRules.RawRecipientsFromAccessMonitoringRules(ctx, req)
	if len(accessRuleRecipients) != 0 {
		return accessRuleRecipients
	}

	var validEmailsSuggReviewers []string
	for _, reviewer := range req.GetSuggestedReviewers() {
		if !lib.IsEmail(reviewer) {
			a.log.WarnContext(ctx, "Failed to notify a suggested reviewer, does not look like a valid email", "reviewer", reviewer)
			continue
		}

		validEmailsSuggReviewers = append(validEmailsSuggReviewers, reviewer)
	}

	a.log.DebugContext(ctx, "Getting recipients for role", "role", req.GetRoles())
	recipients := a.conf.Recipients.GetRawRecipientsFor(req.GetRoles(), validEmailsSuggReviewers)
	for _, recipient := range recipients {
		if recipient != "" {
			recipientSet.Add(recipient)
		}
	}
	return recipientSet.ToSlice()
}
