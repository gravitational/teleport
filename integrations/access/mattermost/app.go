// Copyright 2023 Gravitational, Inc
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

package mattermost

import (
	"time"

	"github.com/gravitational/teleport/integrations/access/common"
)

const (
	// minServerVersion is the minimal teleport version the plugin supports.
	minServerVersion = "6.1.0-beta.1"
	// pluginName is used to tag PluginData and as a Delegator in Audit log.
	pluginName = "mattermost"
	// grpcBackoffMaxDelay is a maximum time GRPC client waits before reconnection attempt.
	grpcBackoffMaxDelay = time.Second * 2
	// initTimeout is used to bound execution time of health check and teleport version check.
	initTimeout = time.Second * 10
	// handlerTimeout is used to bound the execution time of watcher event handler.
	handlerTimeout = time.Second * 5
	// modifyPluginDataBackoffBase is an initial (minimum) backoff value.
	modifyPluginDataBackoffBase = time.Millisecond
	// modifyPluginDataBackoffMax is a backoff threshold
	modifyPluginDataBackoffMax = time.Second
)

func NewMattermostApp(conf *MattermostConfig) *common.BaseApp {
	return common.NewApp(conf, pluginName)
}

// // App contains global application state.
// type App struct {
// 	conf Config

// 	apiClient *client.Client
// 	bot       Bot
// 	mainJob   lib.ServiceJob

// 	*lib.Process
// }

// func NewApp(conf Config) (*App, error) {
// 	app := &App{conf: conf}
// 	app.mainJob = lib.NewServiceJob(app.run)
// 	return app, nil
// }

// // Run initializes and runs a watcher and a callback server
// func (a *App) Run(ctx context.Context) error {
// 	// Initialize the process.
// 	a.Process = lib.NewProcess(ctx)
// 	a.SpawnCriticalJob(a.mainJob)
// 	<-a.Process.Done()
// 	return a.Err()
// }

// // Err returns the error app finished with.
// func (a *App) Err() error {
// 	return trace.Wrap(a.mainJob.Err())
// }

// // WaitReady waits for http and watcher service to start up.
// func (a *App) WaitReady(ctx context.Context) (bool, error) {
// 	return a.mainJob.WaitReady(ctx)
// }

// func (a *App) run(ctx context.Context) error {
// 	var err error

// 	log := logger.Get(ctx)
// 	log.Infof("Starting Teleport Access Mattermost Plugin")

// 	if err = a.init(ctx); err != nil {
// 		return trace.Wrap(err)
// 	}

// 	watcherJob := watcherjob.NewJob(
// 		a.apiClient,
// 		watcherjob.Config{
// 			Watch:            types.Watch{Kinds: []types.WatchKind{types.WatchKind{Kind: types.KindAccessRequest}}},
// 			EventFuncTimeout: handlerTimeout,
// 		},
// 		a.onWatcherEvent,
// 	)
// 	a.SpawnCriticalJob(watcherJob)
// 	ok, err := watcherJob.WaitReady(ctx)
// 	if err != nil {
// 		return trace.Wrap(err)
// 	}

// 	a.mainJob.SetReady(ok)
// 	if ok {
// 		log.Info("Plugin is ready")
// 	} else {
// 		log.Error("Plugin is not ready")
// 	}

// 	<-watcherJob.Done()

// 	return trace.Wrap(watcherJob.Err())
// }

// func (a *App) init(ctx context.Context) error {
// 	ctx, cancel := context.WithTimeout(ctx, initTimeout)
// 	defer cancel()
// 	log := logger.Get(ctx)

// 	if validCred, err := credentials.CheckIfExpired(a.conf.Teleport.Credentials()); err != nil {
// 		log.Warn(err)
// 		if !validCred {
// 			return trace.BadParameter(
// 				"No valid credentials found, this likely means credentials are expired. In this case, please sign new credentials and increase their TTL if needed.",
// 			)
// 		}
// 		log.Info("At least one non-expired credential has been found, continuing startup")
// 	}

// 	var (
// 		err  error
// 		pong proto.PingResponse
// 	)

// 	bk := grpcbackoff.DefaultConfig
// 	bk.MaxDelay = grpcBackoffMaxDelay
// 	if a.apiClient, err = client.New(ctx, client.Config{
// 		Addrs:       a.conf.Teleport.GetAddrs(),
// 		Credentials: a.conf.Teleport.Credentials(),
// 		DialOpts: []grpc.DialOption{
// 			grpc.WithConnectParams(grpc.ConnectParams{Backoff: bk, MinConnectTimeout: initTimeout}),
// 			grpc.WithDefaultCallOptions(
// 				grpc.WaitForReady(true),
// 			),
// 			grpc.WithReturnConnectionError(),
// 		},
// 	}); err != nil {
// 		return trace.Wrap(err)
// 	}

// 	if pong, err = a.checkTeleportVersion(ctx); err != nil {
// 		return trace.Wrap(err)
// 	}

// 	var webProxyAddr string
// 	if pong.ServerFeatures.AdvancedAccessWorkflows {
// 		webProxyAddr = pong.ProxyPublicAddr
// 	}
// 	a.bot, err = NewBot(a.conf.Mattermost, pong.ClusterName, webProxyAddr)
// 	if err != nil {
// 		return trace.Wrap(err)
// 	}

// 	log.Debug("Starting Mattermost API health check...")
// 	if err = a.bot.CheckHealth(ctx); err != nil {
// 		return trace.Wrap(err, "api health check failed. Check your token and make sure that bot is added to your team")
// 	}

// 	log.Debug("Mattermost API health check finished ok")
// 	return nil
// }

// func (a *App) checkTeleportVersion(ctx context.Context) (proto.PingResponse, error) {
// 	log := logger.Get(ctx)
// 	log.Debug("Checking Teleport server version")
// 	pong, err := a.apiClient.Ping(ctx)
// 	if err != nil {
// 		if trace.IsNotImplemented(err) {
// 			return pong, trace.Wrap(err, "server version must be at least %s", minServerVersion)
// 		}
// 		log.Error("Unable to get Teleport server version")
// 		return pong, trace.Wrap(err)
// 	}
// 	err = lib.AssertServerVersion(pong, minServerVersion)
// 	return pong, trace.Wrap(err)
// }

// func (a *App) onWatcherEvent(ctx context.Context, event types.Event) error {
// 	if kind := event.Resource.GetKind(); kind != types.KindAccessRequest {
// 		return trace.Errorf("unexpected kind %s", kind)
// 	}
// 	op := event.Type
// 	reqID := event.Resource.GetName()
// 	ctx, _ = logger.WithField(ctx, "request_id", reqID)

// 	switch op {
// 	case types.OpPut:
// 		ctx, _ = logger.WithField(ctx, "request_op", "put")
// 		req, ok := event.Resource.(types.AccessRequest)
// 		if !ok {
// 			return trace.Errorf("unexpected resource type %T", event.Resource)
// 		}
// 		ctx, log := logger.WithField(ctx, "request_state", req.GetState().String())

// 		var err error
// 		switch {
// 		case req.GetState().IsPending():
// 			err = a.onPendingRequest(ctx, req)
// 		case req.GetState().IsApproved():
// 			err = a.onResolvedRequest(ctx, req)
// 		case req.GetState().IsDenied():
// 			err = a.onResolvedRequest(ctx, req)
// 		default:
// 			log.WithField("event", event).Warn("Unknown request state")
// 			return nil
// 		}

// 		if err != nil {
// 			log.WithError(err).Errorf("Failed to process request")
// 			return trace.Wrap(err)
// 		}

// 		return nil
// 	case types.OpDelete:
// 		ctx, log := logger.WithField(ctx, "request_op", "delete")

// 		if err := a.onDeletedRequest(ctx, reqID); err != nil {
// 			log.WithError(err).Errorf("Failed to process deleted request")
// 			return trace.Wrap(err)
// 		}
// 		return nil
// 	default:
// 		return trace.BadParameter("unexpected event operation %s", op)
// 	}
// }

// func (a *App) onPendingRequest(ctx context.Context, req types.AccessRequest) error {
// 	log := logger.Get(ctx)

// 	reqID := req.GetName()
// 	reqData := RequestData{User: req.GetUser(), Roles: req.GetRoles(), RequestReason: req.GetRequestReason()}

// 	isNew, err := a.modifyPluginData(ctx, reqID, func(existing *PluginData) (PluginData, bool) {
// 		if existing != nil {
// 			return PluginData{}, false
// 		}
// 		return PluginData{RequestData: reqData}, true
// 	})
// 	if err != nil {
// 		return trace.Wrap(err)
// 	}

// 	if isNew {
// 		if channels := a.getPostRecipients(ctx, req.GetSuggestedReviewers()); len(channels) > 0 {
// 			if err := a.broadcastMessages(ctx, channels, reqID, reqData); err != nil {
// 				return trace.Wrap(err)
// 			}
// 		} else {
// 			log.Warning("No channel to post")
// 		}
// 	}

// 	if reqReviews := req.GetReviews(); len(reqReviews) > 0 {
// 		if err := a.postReviewComments(ctx, reqID, reqReviews); err != nil {
// 			return trace.Wrap(err)
// 		}
// 	}

// 	return nil
// }

// func (a *App) onResolvedRequest(ctx context.Context, req types.AccessRequest) error {
// 	var commentErr error
// 	if err := a.postReviewComments(ctx, req.GetName(), req.GetReviews()); err != nil {
// 		commentErr = trace.Wrap(err)
// 	}
// 	resolution := Resolution{Reason: req.GetResolveReason()}
// 	state := req.GetState()
// 	switch state {
// 	case types.RequestState_APPROVED:
// 		resolution.Tag = ResolvedApproved
// 	case types.RequestState_DENIED:
// 		resolution.Tag = ResolvedDenied
// 	default:
// 		logger.Get(ctx).Warningf("Unknown state %v (%s)", state, state.String())
// 		return commentErr
// 	}
// 	err := trace.Wrap(a.updatePosts(ctx, req.GetName(), resolution))
// 	return trace.NewAggregate(commentErr, err)
// }

// func (a *App) onDeletedRequest(ctx context.Context, reqID string) error {
// 	return a.updatePosts(ctx, reqID, Resolution{Tag: ResolvedExpired})
// }

// func (a *App) broadcastMessages(ctx context.Context, channels []string, reqID string, reqData plugindata.AccessRequestData) error {
// 	mmData, err := a.bot.Broadcast(ctx, channels, reqID, reqData)
// 	if len(mmData) == 0 && err != nil {
// 		return trace.Wrap(err)
// 	}
// 	for _, data := range mmData {
// 		logger.Get(ctx).WithFields(logger.Fields{
// 			"mm_channel_id": data.ChannelID,
// 			"mm_post_id":    data.MessageID,
// 		}).Info("Successfully posted to Mattermost")
// 	}
// 	if err != nil {
// 		logger.Get(ctx).WithError(err).Error("Failed to post one or more messages to Mattermost")
// 	}

// 	_, err = a.modifyPluginData(ctx, reqID, func(existing *PluginData) (PluginData, bool) {
// 		var pluginData PluginData
// 		if existing != nil {
// 			pluginData = *existing
// 		} else {
// 			// It must be impossible but lets handle it just in case.
// 			pluginData = PluginData{RequestData: reqData}
// 		}
// 		pluginData.MattermostData = mmData
// 		return pluginData, true
// 	})
// 	return trace.Wrap(err)
// }

// func (a *App) postReviewComments(ctx context.Context, reqID string, reqReviews []types.AccessReview) error {
// 	var oldCount int
// 	var mmData MattermostData
// 	ok, err := a.modifyPluginData(ctx, reqID, func(existing *PluginData) (PluginData, bool) {
// 		if existing == nil {
// 			return PluginData{}, false
// 		}

// 		if mmData = existing.MattermostData; len(mmData) == 0 {
// 			return PluginData{}, false
// 		}

// 		count := len(reqReviews)
// 		if oldCount = existing.ReviewsCount; oldCount >= count {
// 			return PluginData{}, false
// 		}
// 		pluginData := *existing
// 		pluginData.ReviewsCount = count
// 		return pluginData, true
// 	})
// 	if err != nil {
// 		return trace.Wrap(err)
// 	}
// 	if !ok {
// 		logger.Get(ctx).Debug("Failed to post comment: plugin data is missing")
// 		return nil
// 	}

// 	slice := reqReviews[oldCount:]
// 	if len(slice) == 0 {
// 		return nil
// 	}

// 	errors := make([]error, 0, len(slice))
// 	for _, data := range mmData {
// 		ctx, _ = logger.WithFields(ctx, logger.Fields{"mm_channel_id": data.ChannelID, "mm_post_id": data.PostID})
// 		for _, review := range slice {
// 			if err := a.bot.PostReviewComment(ctx, data.ChannelID, data.PostID, review); err != nil {
// 				errors = append(errors, err)
// 			}
// 		}
// 	}
// 	return trace.NewAggregate(errors...)
// }

// func (a *App) tryLookupDirectChannel(ctx context.Context, userEmail string) string {
// 	log := logger.Get(ctx).WithField("mm_user_email", userEmail)
// 	channel, err := a.bot.LookupDirectChannel(ctx, userEmail)
// 	if err != nil {
// 		if errResult, ok := trace.Unwrap(err).(*ErrorResult); ok {
// 			log.Warningf("Failed to lookup direct channel info: %q", errResult.Message)
// 		} else {
// 			log.WithError(err).Error("Failed to lookup direct channel info")
// 		}
// 		return ""
// 	}
// 	return channel
// }

// func (a *App) tryLookupChannel(ctx context.Context, team, name string) string {
// 	log := logger.Get(ctx).WithFields(logger.Fields{
// 		"mm_team":    team,
// 		"mm_channel": name,
// 	})
// 	channel, err := a.bot.LookupChannel(ctx, team, name)
// 	if err != nil {
// 		if errResult, ok := trace.Unwrap(err).(*ErrorResult); ok {
// 			log.Warningf("Failed to lookup channel info: %q", errResult.Message)
// 		} else {
// 			log.WithError(err).Error("Failed to lookup channel info")
// 		}
// 		return ""
// 	}
// 	return channel
// }

// func (a *App) getPostRecipients(ctx context.Context, suggestedReviewers []string) []string {
// 	log := logger.Get(ctx)

// 	channelSet := stringset.NewWithCap(len(suggestedReviewers) + len(a.conf.Mattermost.Recipients))

// 	for _, recipient := range suggestedReviewers {
// 		// We require SuggestedReviewers to contain email-like data. Anything else is not supported.
// 		if !lib.IsEmail(recipient) {
// 			log.Warningf("Failed to notify a suggested reviewer: %q does not look like a valid email", recipient)
// 			continue
// 		}
// 		channel := a.tryLookupDirectChannel(ctx, recipient)
// 		if channel == "" {
// 			continue
// 		}
// 		channelSet.Add(channel)
// 	}

// 	for _, recipient := range a.conf.Mattermost.Recipients {
// 		var channel string
// 		// Recipients from config file could contain either email or team and channel names separated by '/' symbol. It's up to user what format to use.
// 		if lib.IsEmail(recipient) {
// 			channel = a.tryLookupDirectChannel(ctx, recipient)
// 		} else {
// 			parts := strings.Split(recipient, "/")
// 			if len(parts) == 2 {
// 				channel = a.tryLookupChannel(ctx, parts[0], parts[1])
// 			} else {
// 				log.Warningf("Recipient must be either a user email or a channel in the format \"team/channel\" but got %q", recipient)
// 			}
// 		}
// 		if channel == "" {
// 			continue
// 		}
// 		channelSet.Add(channel)
// 	}

// 	return channelSet.ToSlice()
// }

// func (a *App) updatePosts(ctx context.Context, reqID string, resolution Resolution) error {
// 	log := logger.Get(ctx)

// 	var pluginData PluginData
// 	ok, err := a.modifyPluginData(ctx, reqID, func(existing *PluginData) (PluginData, bool) {
// 		// If plugin data is empty or missing mattermost post IDs, we cannot do anything.
// 		if existing == nil {
// 			return PluginData{}, false
// 		}
// 		if pluginData = *existing; len(pluginData.MattermostData) == 0 {
// 			return PluginData{}, false
// 		}

// 		// If resolution field is not empty then we already resolved the incident before. In this case we just quit.
// 		if pluginData.RequestData.Resolution.Tag != Unresolved {
// 			return PluginData{}, false
// 		}

// 		// Mark plugin data as resolved.
// 		pluginData.Resolution = resolution
// 		return pluginData, true
// 	})
// 	if err != nil {
// 		return trace.Wrap(err)
// 	}
// 	if !ok {
// 		log.Debug("Failed to update posts: plugin data is missing")
// 		return nil
// 	}

// 	reqData, mmData := pluginData.RequestData, pluginData.MattermostData
// 	if err := a.bot.UpdateMessages(ctx, reqID, reqData, mmData); err != nil {
// 		return trace.Wrap(err)
// 	}

// 	log.Infof("Successfully marked request as %s in all messages", resolution.Tag)

// 	return nil
// }

// // modifyPluginData performs a compare-and-swap update of access request's plugin data.
// // Callback function parameter is nil if plugin data hasn't been created yet.
// // Otherwise, callback function parameter is a pointer to current plugin data contents.
// // Callback function return value is an updated plugin data contents plus the boolean flag
// // indicating whether it should be written or not.
// // Note that callback function fn might be called more than once due to retry mechanism baked in
// // so make sure that the function is "pure" i.e. it doesn't interact with the outside world:
// // it doesn't perform any sort of I/O operations so even things like Go channels must be avoided.
// // Indeed, this limitation is not that ultimate at least if you know what you're doing.
// func (a *App) modifyPluginData(ctx context.Context, reqID string, fn func(data *PluginData) (PluginData, bool)) (bool, error) {
// 	backoff := backoff.NewDecorr(modifyPluginDataBackoffBase, modifyPluginDataBackoffMax, clockwork.NewRealClock())
// 	for {
// 		oldData, err := a.getPluginData(ctx, reqID)
// 		if err != nil && !trace.IsNotFound(err) {
// 			return false, trace.Wrap(err)
// 		}
// 		newData, ok := fn(oldData)
// 		if !ok {
// 			return false, nil
// 		}
// 		var expectData PluginData
// 		if oldData != nil {
// 			expectData = *oldData
// 		}
// 		err = trace.Wrap(a.updatePluginData(ctx, reqID, newData, expectData))
// 		if err == nil {
// 			return true, nil
// 		}
// 		if !trace.IsCompareFailed(err) {
// 			return false, trace.Wrap(err)
// 		}
// 		if err := backoff.Do(ctx); err != nil {
// 			return false, trace.Wrap(err)
// 		}
// 	}
// }

// // getPluginData loads a plugin data for a given access request. It returns nil if it's not found.
// func (a *App) getPluginData(ctx context.Context, reqID string) (*PluginData, error) {
// 	dataMaps, err := a.apiClient.GetPluginData(ctx, types.PluginDataFilter{
// 		Kind:     types.KindAccessRequest,
// 		Resource: reqID,
// 		Plugin:   pluginName,
// 	})
// 	if err != nil {
// 		return nil, trace.Wrap(err)
// 	}
// 	if len(dataMaps) == 0 {
// 		return nil, trace.NotFound("plugin data not found")
// 	}
// 	entry := dataMaps[0].Entries()[pluginName]
// 	if entry == nil {
// 		return nil, trace.NotFound("plugin data entry not found")
// 	}
// 	data := DecodePluginData(entry.Data)
// 	return &data, nil
// }

// // updatePluginData updates an existing plugin data or sets a new one if it didn't exist.
// func (a *App) updatePluginData(ctx context.Context, reqID string, data PluginData, expectData PluginData) error {
// 	return a.apiClient.UpdatePluginData(ctx, types.PluginDataUpdateParams{
// 		Kind:     types.KindAccessRequest,
// 		Resource: reqID,
// 		Plugin:   pluginName,
// 		Set:      EncodePluginData(data),
// 		Expect:   EncodePluginData(expectData),
// 	})
// }
