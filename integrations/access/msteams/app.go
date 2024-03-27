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
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
	pd "github.com/gravitational/teleport/integrations/lib/plugindata"
	"github.com/gravitational/teleport/integrations/lib/stringset"
	"github.com/gravitational/teleport/integrations/lib/watcherjob"
)

const (
	// pluginName used as Teleport plugin identifier
	pluginName = "msteams"
	// minServerVersion is the minimal teleport version the plugin supports.
	minServerVersion = "8.0.0"
	// initTimeout is used to bound execution time of health check and teleport version check.
	initTimeout = time.Second * 10
	// handlerTimeout is used to bound the execution time of watcher event handler.
	handlerTimeout = time.Second * 5
)

// App contains global application state.
type App struct {
	conf Config

	apiClient  teleport.Client
	bot        *Bot
	mainJob    lib.ServiceJob
	watcherJob lib.ServiceJob
	pd         *pd.CompareAndSwap[PluginData]

	*lib.Process
}

// NewApp initializes a new teleport-msteams app and returns it.
func NewApp(conf Config) (*App, error) {
	app := &App{conf: conf}

	app.mainJob = lib.NewServiceJob(app.run)

	return app, nil
}

// Run starts the main job process
func (a *App) Run(ctx context.Context) error {
	log := logger.Get(ctx)
	log.Info("Starting Teleport MS Teams Plugin")

	err := a.init(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	a.Process = lib.NewProcess(ctx)
	a.watcherJob, err = a.newWatcherJob()
	if err != nil {
		return trace.Wrap(err)
	}

	a.SpawnCriticalJob(a.mainJob)
	a.SpawnCriticalJob(a.watcherJob)

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

	var err error
	a.apiClient, err = common.GetTeleportClient(ctx, a.conf.Teleport)
	if err != nil {
		return trace.Wrap(err)
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

	a.bot, err = NewBot(a.conf.MSAPI, pong.ClusterName, webProxyAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	return a.initBot(ctx)
}

// initBot initializes bot
func (a *App) initBot(ctx context.Context) error {
	log := logger.Get(ctx)

	teamsApp, err := a.bot.GetTeamsApp(ctx)
	if trace.IsNotFound(err) {
		return trace.Wrap(err, "MS Teams app not found in org app store.")
	}
	if err != nil {
		return trace.Wrap(err)
	}

	log.WithField("name", teamsApp.DisplayName).
		WithField("id", teamsApp.ID).
		Info("MS Teams app found in org app store")

	if !a.conf.Preload {
		return nil
	}

	log.Info("Preloading recipient data...")

	for _, recipient := range a.conf.Recipients.GetAllRawRecipients() {
		recipientData, err := a.bot.FetchRecipient(ctx, recipient)
		if err != nil {
			return trace.Wrap(err)
		}
		log.WithField("recipient", recipient).
			WithField("chat_id", recipientData.Chat.ID).
			WithField("kind", recipientData.Kind).
			Info("Recipient found, chat found")
	}

	log.Info("Recipient data preloaded and cached.")

	return nil
}

// newWatcherJob creates WatcherJob
func (a *App) newWatcherJob() (lib.ServiceJob, error) {
	return watcherjob.NewJob(
		a.apiClient,
		watcherjob.Config{
			Watch: types.Watch{
				Kinds: []types.WatchKind{{Kind: types.KindAccessRequest}},
			},
			EventFuncTimeout: handlerTimeout,
		},
		a.onWatcherEvent,
	)
}

// run starts the main process
func (a *App) run(ctx context.Context) error {
	log := logger.Get(ctx)

	ok, err := a.watcherJob.WaitReady(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	if ok {
		log.Info("Plugin is ready")
	} else {
		log.Error("Plugin is not ready")
	}

	a.mainJob.SetReady(ok)

	<-a.watcherJob.Done()

	return trace.Wrap(a.watcherJob.Err())
}

// checkTeleportVersion loads Teleport version and checks that it meets the minimal required
func (a *App) checkTeleportVersion(ctx context.Context) (proto.PingResponse, error) {
	log := logger.Get(ctx)
	log.Debug("Checking Teleport server version")

	pong, err := a.apiClient.Ping(ctx)
	if err != nil {
		if trace.IsNotImplemented(err) {
			return pong, trace.Wrap(err, "server version must be at least %s", minServerVersion)
		}

		log.Error("Unable to get Teleport server version")
		return pong, trace.Wrap(err)
	}

	err = lib.AssertServerVersion(pong, minServerVersion)

	return pong, trace.Wrap(err)
}

// onWatcherEvent called when an access request event is received
func (a *App) onWatcherEvent(ctx context.Context, event types.Event) error {
	kind := event.Resource.GetKind()
	if kind != types.KindAccessRequest {
		return trace.Errorf("unexpected kind %s", kind)
	}

	op := event.Type
	reqID := event.Resource.GetName()
	ctx, _ = logger.WithField(ctx, "request_id", reqID)

	switch op {
	case types.OpPut:
		ctx, _ = logger.WithField(ctx, "request_op", "put")
		req, ok := event.Resource.(types.AccessRequest)
		if !ok {
			return trace.Errorf("unexpected resource type %T", event.Resource)
		}
		ctx, log := logger.WithField(ctx, "request_state", req.GetState().String())

		var err error

		switch {
		case req.GetState().IsPending():
			log.Debug("Pending request received")
			err = a.onPendingRequest(ctx, req)
		case req.GetState().IsApproved():
			log.Debug("Approval request received")
			err = a.onResolvedRequest(ctx, req)
		case req.GetState().IsDenied():
			log.Debug("Denial request received")
			err = a.onResolvedRequest(ctx, req)
		default:
			log.WithField("event", event).Warn("Unknown request state")
			return nil
		}

		if err != nil {
			log.WithError(err).Errorf("Failed to process request")
			return trace.Wrap(err)
		}

		return nil

	case types.OpDelete:
		ctx, log := logger.WithField(ctx, "request_op", "delete")

		log.Debug("Expiration request received")

		if err := a.onDeletedRequest(ctx, reqID); err != nil {
			log.WithError(err).Errorf("Failed to process deleted request")
			return trace.Wrap(err)
		}
		return nil
	default:
		return trace.BadParameter("unexpected event operation %s", op)
	}
}

// onPendingRequest is called when there's a new request or a review
func (a *App) onPendingRequest(ctx context.Context, req types.AccessRequest) error {
	log := logger.Get(ctx)

	id := req.GetName()
	data := pd.AccessRequestData{
		User:          req.GetUser(),
		Roles:         req.GetRoles(),
		RequestReason: req.GetRequestReason(),
	}

	// Let's try to create PluginData. This equals to locking AccessRequest to this
	// instance of a plugin.
	_, err := a.pd.Create(ctx, id, PluginData{AccessRequestData: data})

	// If we succeeded to create PluginData, let's post a messages and save created Teams messages
	if !trace.IsAlreadyExists(err) {
		if err != nil {
			return trace.Wrap(err)
		}

		recipients := a.getMessageRecipients(ctx, req)

		if len(recipients) == 0 {
			log.Warning("No recipients to notify")
		} else {
			err = a.postMessages(ctx, recipients, id, data)
			if err != nil {
				return trace.Wrap(err)
			}
		}
	}

	// Update the received reviews
	reviews := req.GetReviews()
	if len(reviews) == 0 {
		return nil
	}

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
	switch state {
	case types.RequestState_APPROVED:
		tag = pd.ResolvedApproved
	case types.RequestState_DENIED:
		tag = pd.ResolvedDenied
	default:
		logger.Get(ctx).Warningf("Unknown state %v (%s)", state, state.String())
		return trace.Errorf("Unknown state")
	}
	err := a.updateMessages(ctx, req.GetName(), tag, req.GetResolveReason(), req.GetReviews())
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// onDeleteRequest gets called when a request is deleted
func (a *App) onDeletedRequest(ctx context.Context, reqID string) error {
	return a.updateMessages(ctx, reqID, pd.ResolvedExpired, "", nil)
}

// postMessages posts initial Teams messages
func (a *App) postMessages(ctx context.Context, recipients []string, id string, data pd.AccessRequestData) error {
	teamsData, err := a.bot.PostMessages(ctx, recipients, id, data)
	if err != nil {
		if len(teamsData) == 0 {
			// TODO: add better logging here
			return trace.Wrap(err)
		}

		logger.Get(ctx).WithError(err).Error("Failed to post one or more messages to MS Teams")
	}

	for _, data := range teamsData {
		logger.Get(ctx).WithFields(logger.Fields{
			"id":        data.ID,
			"timestamp": data.Timestamp,
			"recipient": data.RecipientID,
		}).Info("Successfully posted to MS Teams")
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
	log := logger.Get(ctx)

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

	log.Infof("Successfully marked request as %s in all messages", tag)

	return nil
}

// getMessageRecipients returns a recipients list for the access request
func (a *App) getMessageRecipients(ctx context.Context, req types.AccessRequest) []string {
	log := logger.Get(ctx)

	// We receive a set from GetRawRecipientsFor but we still might end up with duplicate channel names.
	// This can happen if this set contains the channel `C` and the email for channel `C`.
	recipientSet := stringset.New()

	var validEmailsSuggReviewers []string
	for _, reviewer := range req.GetSuggestedReviewers() {
		if !lib.IsEmail(reviewer) {
			log.Warningf("Failed to notify a suggested reviewer: %q does not look like a valid email", reviewer)
			continue
		}

		validEmailsSuggReviewers = append(validEmailsSuggReviewers, reviewer)
	}

	recipients := a.conf.Recipients.GetRawRecipientsFor(req.GetRoles(), validEmailsSuggReviewers)
	for _, recipient := range recipients {
		if recipient != "" {
			recipientSet.Add(recipient)
		}
	}

	return recipientSet.ToSlice()
}
