/*
Copyright 2022 Gravitational, Inc.

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

package common

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
	pd "github.com/gravitational/teleport/integrations/lib/plugindata"
	"github.com/gravitational/teleport/integrations/lib/watcherjob"
)

const (
	// minServerVersion is the minimal teleport version the plugin supports.
	minServerVersion = "6.1.0-beta.1"
	// grpcBackoffMaxDelay is a maximum time GRPC client waits before reconnection attempt.
	grpcBackoffMaxDelay = time.Second * 2
	// InitTimeout is used to bound execution time of health check and teleport version check.
	initTimeout = time.Second * 10
	// handlerTimeout is used to bound the execution time of watcher event handler.
	handlerTimeout = time.Second * 5
)

// BaseApp is responsible for handling all the access-request logic.
// It will start a Teleport client, listen for events and treat them.
// It also handles signals and watches its thread.
// To instantiate a new BaseApp, use NewApp()
type BaseApp struct {
	PluginName string
	apiClient  teleport.Client
	bot        MessagingBot
	mainJob    lib.ServiceJob
	pluginData *pd.CompareAndSwap[GenericPluginData]
	Conf       PluginConfiguration

	*lib.Process
}

// NewApp creates a new BaseApp and initialize its main job
func NewApp(conf PluginConfiguration, pluginName string) *BaseApp {
	app := BaseApp{
		PluginName: pluginName,
		Conf:       conf,
	}
	app.mainJob = lib.NewServiceJob(app.run)
	return &app
}

// Run initializes and runs a watcher and a callback server
func (a *BaseApp) Run(ctx context.Context) error {
	// Initialize the process.
	a.Process = lib.NewProcess(ctx)
	a.SpawnCriticalJob(a.mainJob)
	<-a.Process.Done()
	return a.Err()
}

// Err returns the error app finished with.
func (a *BaseApp) Err() error {
	return trace.Wrap(a.mainJob.Err())
}

// WaitReady waits for http and watcher service to start up.
func (a *BaseApp) WaitReady(ctx context.Context) (bool, error) {
	return a.mainJob.WaitReady(ctx)
}

func (a *BaseApp) checkTeleportVersion(ctx context.Context) (proto.PingResponse, error) {
	log := logger.Get(ctx)
	log.Debug("Checking Teleport server version")

	pong, err := a.apiClient.Ping(ctx)
	if err != nil {
		if trace.IsNotImplemented(err) {
			return pong, trace.Wrap(err, "server version must be at least %s", minServerVersion)
		}
		return pong, trace.Wrap(err, "Unable to get Teleport server version")
	}
	err = lib.AssertServerVersion(pong, minServerVersion)
	return pong, trace.Wrap(err)
}

// initTeleport creates a Teleport client and validates Teleport connectivity.
func (a *BaseApp) initTeleport(ctx context.Context, conf PluginConfiguration) (clusterName, webProxyAddr string, err error) {
	clt, err := conf.GetTeleportClient(ctx)
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	a.apiClient = clt
	pong, err := a.checkTeleportVersion(ctx)
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	if pong.ServerFeatures.AdvancedAccessWorkflows {
		webProxyAddr = pong.ProxyPublicAddr
	}

	return pong.ClusterName, webProxyAddr, nil
}

// onWatcherEvent is called for every cluster Event. It will filter out non-access-request events and
// call onPendingRequest, onResolvedRequest and on DeletedRequest depending on the event.
func (a *BaseApp) onWatcherEvent(ctx context.Context, event types.Event) error {
	if kind := event.Resource.GetKind(); kind != types.KindAccessRequest {
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
			err = a.onPendingRequest(ctx, req)
		case req.GetState().IsApproved(), req.GetState().IsDenied():
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

		if err := a.onDeletedRequest(ctx, reqID); err != nil {
			log.WithError(err).Errorf("Failed to process deleted request")
			return trace.Wrap(err)
		}
		return nil
	default:
		return trace.BadParameter("unexpected event operation %s", op)
	}
}

// run starts the event watcher job and blocks utils it stops
func (a *BaseApp) run(ctx context.Context) error {
	log := logger.Get(ctx)

	if err := a.init(ctx); err != nil {
		return trace.Wrap(err)
	}
	watcherJob := watcherjob.NewJob(
		a.apiClient,
		watcherjob.Config{
			Watch:            types.Watch{Kinds: []types.WatchKind{{Kind: types.KindAccessRequest}}},
			EventFuncTimeout: handlerTimeout,
		},
		a.onWatcherEvent,
	)
	a.SpawnCriticalJob(watcherJob)
	ok, err := watcherJob.WaitReady(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	a.mainJob.SetReady(ok)
	if ok {
		log.Info("Plugin is ready")
	} else {
		log.Error("Plugin is not ready")
	}

	<-watcherJob.Done()

	return trace.Wrap(watcherJob.Err())
}

func (a *BaseApp) init(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, initTimeout)
	defer cancel()
	log := logger.Get(ctx)

	clusterName, webProxyAddr, err := a.initTeleport(ctx, a.Conf)
	if err != nil {
		return trace.Wrap(err)
	}

	a.bot, err = a.Conf.NewBot(clusterName, webProxyAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	a.pluginData = pd.NewCAS(
		a.apiClient,
		a.PluginName,
		types.KindAccessRequest,
		EncodePluginData,
		DecodePluginData,
	)

	log.Debug("Starting API health check...")
	if err = a.bot.CheckHealth(ctx); err != nil {
		return trace.Wrap(err, "API health check failed")
	}

	log.Debug("API health check finished ok")
	return nil
}

func (a *BaseApp) onPendingRequest(ctx context.Context, req types.AccessRequest) error {
	log := logger.Get(ctx)

	reqID := req.GetName()
	reqData := pd.AccessRequestData{
		User:               req.GetUser(),
		Roles:              req.GetRoles(),
		RequestReason:      req.GetRequestReason(),
		ResolveAnnotations: req.GetResolveAnnotations(),
	}

	_, err := a.pluginData.Create(ctx, reqID, GenericPluginData{AccessRequestData: reqData})
	switch {
	case err == nil:
		// This is a new access-request, we have to broadcast it first.
		if recipients := a.getMessageRecipients(ctx, req); len(recipients) > 0 {
			if err := a.broadcastMessages(ctx, recipients, reqID, reqData); err != nil {
				return trace.Wrap(err)
			}
		} else {
			log.Warning("No channel to post")
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

func (a *BaseApp) onResolvedRequest(ctx context.Context, req types.AccessRequest) error {
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
	default:
		logger.Get(ctx).Warningf("Unknown state %v (%s)", state, state.String())
		return replyErr
	}
	err := trace.Wrap(a.updateMessages(ctx, req.GetName(), tag, reason, req.GetReviews()))
	return trace.NewAggregate(replyErr, err)
}

func (a *BaseApp) onDeletedRequest(ctx context.Context, reqID string) error {
	return a.updateMessages(ctx, reqID, pd.ResolvedExpired, "", nil)
}

// broadcastMessages sends nessages to each recipient for an access-request.
// This method is only called when for new access-requests.
func (a *BaseApp) broadcastMessages(ctx context.Context, recipients []Recipient, reqID string, reqData pd.AccessRequestData) error {
	sentMessages, err := a.bot.Broadcast(ctx, recipients, reqID, reqData)
	if len(sentMessages) == 0 && err != nil {
		return trace.Wrap(err)
	}
	for _, data := range sentMessages {
		logger.Get(ctx).WithFields(logger.Fields{
			"channel_id": data.ChannelID,
			"message_id": data.MessageID,
		}).Info("Successfully posted messages")
	}
	if err != nil {
		logger.Get(ctx).WithError(err).Error("Failed to post one or more messages")
	}

	_, err = a.pluginData.Update(ctx, reqID, func(existing GenericPluginData) (GenericPluginData, error) {
		existing.SentMessages = sentMessages
		return existing, nil
	})

	return trace.Wrap(err)
}

// postReviewReplies lists and updates existing messages belonging to an access request.
// Posting reviews is done both by updating the original message and by replying in thread if possible.
func (a *BaseApp) postReviewReplies(ctx context.Context, reqID string, reqReviews []types.AccessReview) error {
	var oldCount int

	pd, err := a.pluginData.Update(ctx, reqID, func(existing GenericPluginData) (GenericPluginData, error) {
		sentMessages := existing.SentMessages
		if len(sentMessages) == 0 {
			// wait for the plugin data to be updated with SentMessages
			return GenericPluginData{}, trace.CompareFailed("existing sentMessages is empty")
		}

		count := len(reqReviews)
		oldCount = existing.ReviewsCount
		if oldCount >= count {
			return GenericPluginData{}, trace.AlreadyExists("reviews are sent already")
		}

		existing.ReviewsCount = count
		return existing, nil
	})
	if trace.IsAlreadyExists(err) {
		logger.Get(ctx).Debug("Failed to post reply: replies are already sent")
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
		ctx, _ = logger.WithFields(ctx, logger.Fields{"channel_id": data.ChannelID, "message_id": data.MessageID})
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
func (a *BaseApp) getMessageRecipients(ctx context.Context, req types.AccessRequest) []Recipient {
	log := logger.Get(ctx)

	// We receive a set from GetRawRecipientsFor but we still might end up with duplicate channel names.
	// This can happen if this set contains the channel `C` and the email for channel `C`.
	recipientSet := NewRecipientSet()

	switch a.Conf.GetPluginType() {
	case types.PluginTypeOpsgenie:
		if recipients, ok := req.GetResolveAnnotations()[types.TeleportNamespace+types.ReqAnnotationSchedulesLabel]; ok {
			for _, recipient := range recipients {
				rec, err := a.bot.FetchRecipient(ctx, recipient)
				if err != nil {
					log.Warning(err)
				}
				recipientSet.Add(*rec)
			}
			return recipientSet.ToSlice()
		}
	}

	validEmailSuggReviewers := []string{}
	for _, reviewer := range req.GetSuggestedReviewers() {
		if !lib.IsEmail(reviewer) {
			log.Warningf("Failed to notify a suggested reviewer: %q does not look like a valid email", reviewer)
			continue
		}

		validEmailSuggReviewers = append(validEmailSuggReviewers, reviewer)
	}
	rawRecipients := a.Conf.GetRecipients().GetRawRecipientsFor(req.GetRoles(), validEmailSuggReviewers)
	for _, rawRecipient := range rawRecipients {
		recipient, err := a.bot.FetchRecipient(ctx, rawRecipient)
		if err != nil {
			// Something wrong happened, we log the error and continue to treat valid rawRecipients
			log.Warning(err)
		} else {
			recipientSet.Add(*recipient)
		}
	}

	return recipientSet.ToSlice()
}

// updateMessages updates the messages status and adds the resolve reason.
func (a *BaseApp) updateMessages(ctx context.Context, reqID string, tag pd.ResolutionTag, reason string, reviews []types.AccessReview) error {
	log := logger.Get(ctx)

	pluginData, err := a.pluginData.Update(ctx, reqID, func(existing GenericPluginData) (GenericPluginData, error) {
		if len(existing.SentMessages) == 0 {
			return GenericPluginData{}, trace.NotFound("plugin data not found")
		}

		// If resolution field is not empty then we already resolved the incident before. In this case we just quit.
		if existing.AccessRequestData.ResolutionTag != pd.Unresolved {
			return GenericPluginData{}, trace.AlreadyExists("request is already resolved")
		}

		// Mark plugin data as resolved.
		existing.ResolutionTag = tag
		existing.ResolutionReason = reason

		return existing, nil
	})
	if trace.IsNotFound(err) {
		log.Debug("Failed to update messages: plugin data is missing")
		return nil
	}
	if trace.IsAlreadyExists(err) {
		if tag != pluginData.ResolutionTag {
			return trace.WrapWithMessage(err,
				"cannot change the resolution tag of an already resolved request, existing: %s, event: %s",
				pluginData.ResolutionTag, tag)
		}
		log.Debug("Request is already resolved, ignoring event")
		return nil
	}
	if err != nil {
		return trace.Wrap(err)
	}

	reqData, sentMessages := pluginData.AccessRequestData, pluginData.SentMessages
	if err := a.bot.UpdateMessages(ctx, reqID, reqData, sentMessages, reviews); err != nil {
		return trace.Wrap(err)
	}

	log.Infof("Successfully marked request as %s in all messages", tag)

	return nil
}
