/*
Copyright 2015-2021 Gravitational, Inc.

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

package email

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
	"github.com/gravitational/teleport/integrations/lib/logger"
	"github.com/gravitational/teleport/integrations/lib/watcherjob"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

const (
	// minServerVersion is the minimal teleport version the plugin supports.
	minServerVersion = "6.1.0-beta.1"
	// pluginName is used to tag PluginData and as a Delegator in Audit log.
	pluginName = "email"
	// initTimeout is used to bound execution time of health check and teleport version check.
	initTimeout = time.Second * 10
	// handlerTimeout is used to bound the execution time of watcher event handler.
	handlerTimeout = time.Second * 15
	// maxModifyPluginDataTries is a maximum number of compare-and-swap tries when modifying plugin data.
	maxModifyPluginDataTries = 5
)

// App contains global application state.
type App struct {
	conf Config

	apiClient             teleport.Client
	client                Client
	mainJob               lib.ServiceJob
	accessMonitoringRules *accessmonitoring.RuleHandler

	*lib.Process
}

// NewApp initializes a new teleport-email app and returns it.
func NewApp(conf Config) (*App, error) {
	app := &App{conf: conf}
	app.mainJob = lib.NewServiceJob(app.run)
	return app, nil
}

// Run initializes and runs a watcher and a callback server
func (a *App) Run(ctx context.Context) error {
	// Initialize the process.
	a.Process = lib.NewProcess(ctx)
	a.SpawnCriticalJob(a.mainJob)
	<-a.Process.Done()
	return a.Err()
}

// Err returns the error app finished with.
func (a *App) Err() error {
	return trace.Wrap(a.mainJob.Err())
}

// WaitReady waits for http and watcher service to start up.
func (a *App) WaitReady(ctx context.Context) (bool, error) {
	return a.mainJob.WaitReady(ctx)
}

// run starts plugin
func (a *App) run(ctx context.Context) error {
	var err error

	log := logger.Get(ctx)

	if err = a.init(ctx); err != nil {
		return trace.Wrap(err)
	}

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
	a.SpawnCriticalJob(watcherJob)
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

	a.mainJob.SetReady(ok)
	if ok {
		log.InfoContext(ctx, "Plugin is ready")
	} else {
		log.ErrorContext(ctx, "Plugin is not ready")
	}

	<-watcherJob.Done()

	return trace.Wrap(watcherJob.Err())
}

// init inits plugin
func (a *App) init(ctx context.Context) error {
	log := logger.Get(ctx)
	ctx, cancel := context.WithTimeout(ctx, initTimeout)
	defer cancel()

	var err error
	if a.apiClient, err = a.conf.GetTeleportClient(ctx); err != nil {
		return trace.Wrap(err)
	}

	pong, err := a.checkTeleportVersion(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	var webProxyAddr string
	if pong.ServerFeatures.AdvancedAccessWorkflows {
		webProxyAddr = pong.ProxyPublicAddr
	}

	a.client, err = NewClient(ctx, a.conf, pong.ClusterName, webProxyAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	a.accessMonitoringRules = accessmonitoring.NewRuleHandler(accessmonitoring.RuleHandlerConfig{
		Client:     a.apiClient,
		PluginType: types.PluginTypeEmail,
		PluginName: pluginName,
		FetchRecipientCallback: func(_ context.Context, recipient string) (*common.Recipient, error) {
			return &common.Recipient{
				Name: recipient,
				ID:   recipient,
				Kind: common.RecipientKindEmail,
			}, nil
		},
	})

	log.DebugContext(ctx, "Starting client connection health check")
	if err = a.client.CheckHealth(ctx); err != nil {
		return trace.Wrap(err, "client connection health check failed")
	}
	log.DebugContext(ctx, "Client connection health check finished ok")
	return nil
}

// checkTeleportVersion checks that Teleport version is not lower than required
func (a *App) checkTeleportVersion(ctx context.Context) (proto.PingResponse, error) {
	log := logger.Get(ctx)
	log.DebugContext(ctx, "Checking Teleport server version")
	pong, err := a.apiClient.Ping(ctx)
	if err != nil {
		if trace.IsNotImplemented(err) {
			return pong, trace.Wrap(err, "server version must be at least %s", minServerVersion)
		}
		log.ErrorContext(ctx, "Unable to get Teleport server version")
		return pong, trace.Wrap(err)
	}
	err = utils.CheckMinVersion(pong.ServerVersion, minServerVersion)
	return pong, trace.Wrap(err)
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

// handleAccessRequest processes new incoming access request
func (a *App) handleAccessRequest(ctx context.Context, event types.Event) error {
	if kind := event.Resource.GetKind(); kind != types.KindAccessRequest {
		return trace.Errorf("unexpected kind %s", kind)
	}
	op := event.Type
	reqID := event.Resource.GetName()
	ctx, _ = logger.With(ctx, "request_id", reqID)

	switch op {
	case types.OpPut:
		ctx, _ = logger.With(ctx, "request_op", "put")
		req, ok := event.Resource.(types.AccessRequest)
		if !ok {
			return trace.Errorf("unexpected resource type %T", event.Resource)
		}
		ctx, log := logger.With(ctx, "request_state", req.GetState().String())

		var err error
		switch {
		case req.GetState().IsPending():
			err = a.onPendingRequest(ctx, req)
		case req.GetState().IsApproved():
			err = a.onResolvedRequest(ctx, req)
		case req.GetState().IsDenied():
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

			log.With("event", event).WarnContext(ctx, "Unknown request state")
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

// onPendingRequest is called when an access request is created or reviewed (with thresholds > 1)
func (a *App) onPendingRequest(ctx context.Context, req types.AccessRequest) error {
	log := logger.Get(ctx)

	reqID := req.GetName()
	reqData := NewRequestData(req)

	isNew, err := a.modifyPluginData(ctx, reqID, func(existing *PluginData) (PluginData, bool) {
		if existing != nil {
			return PluginData{}, false
		}
		return PluginData{RequestData: reqData}, true
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if isNew {
		recipients := a.getRecipients(ctx, req)
		if len(recipients) == 0 {
			log.WarnContext(ctx, "No recipients to send")
			return nil
		}

		if err := a.sendNewThreads(ctx, recipients, reqID, reqData); err != nil {
			return trace.Wrap(err)
		}
	}

	if reqReviews := req.GetReviews(); len(reqReviews) > 0 {
		if err := a.sendReviews(ctx, reqID, reqData, reqReviews); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// onResolvedRequest is called when request has been resolved or denied
func (a *App) onResolvedRequest(ctx context.Context, req types.AccessRequest) error {
	var replyErr error

	reqID := req.GetName()
	reqData := NewRequestData(req)

	if err := a.sendReviews(ctx, reqID, reqData, req.GetReviews()); err != nil {
		replyErr = trace.Wrap(err)
	}

	resolution := Resolution{Reason: req.GetResolveReason()}
	state := req.GetState()
	switch state {
	case types.RequestState_APPROVED:
		resolution.Tag = ResolvedApproved
	case types.RequestState_DENIED:
		resolution.Tag = ResolvedDenied
	default:
		logger.Get(ctx).WarnContext(ctx, "Unknown state", "state", logutils.StringerAttr(state))
		return replyErr
	}
	err := trace.Wrap(a.sendResolution(ctx, req.GetName(), resolution))
	return trace.NewAggregate(replyErr, err)
}

// onResolvedRequest is called when request has been deleted
func (a *App) onDeletedRequest(ctx context.Context, reqID string) error {
	return a.sendResolution(ctx, reqID, Resolution{Tag: ResolvedExpired})
}

func (a *App) getRecipients(ctx context.Context, req types.AccessRequest) []common.Recipient {
	log := logger.Get(ctx)

	recipientSet := common.NewRecipientSet()
	recipients := a.accessMonitoringRules.RecipientsFromAccessMonitoringRules(ctx, req)
	recipients.ForEach(func(r common.Recipient) {
		recipientSet.Add(r)
	})

	// Return the set of recipients if it is not empty.
	// Otherwise, use the legacy role to recipients map to search for recipients.
	if recipientSet.Len() != 0 {
		return recipientSet.ToSlice()
	}

	rawRecipients := a.conf.RoleToRecipients.GetRawRecipientsFor(req.GetRoles(), req.GetSuggestedReviewers())
	for _, rawRecipient := range rawRecipients {
		if !lib.IsEmail(rawRecipient) {
			log.WarnContext(ctx, "Failed to notify a suggested reviewer with an invalid email address", "reviewer", rawRecipient)
			continue
		}
		recipientSet.Add(common.Recipient{
			ID:   rawRecipient,
			Name: rawRecipient,
			Kind: common.RecipientKindEmail,
		})
	}
	return recipientSet.ToSlice()
}

// broadcastNewThreads sends notifications on a new request
func (a *App) sendNewThreads(ctx context.Context, recipients []common.Recipient, reqID string, reqData RequestData) error {
	threadsSent, err := a.client.SendNewThreads(ctx, recipients, reqID, reqData)

	if len(threadsSent) == 0 && err != nil {
		return trace.Wrap(err)
	}

	logSentThreads(ctx, threadsSent, "new threads")

	if err != nil {
		logger.Get(ctx).ErrorContext(ctx, "Failed send one or more messages", "error", err)
	}

	_, err = a.modifyPluginData(ctx, reqID, func(existing *PluginData) (PluginData, bool) {
		var pluginData PluginData
		if existing != nil {
			pluginData = *existing
		} else {
			// It must be impossible but lets handle it just in case.
			pluginData = PluginData{RequestData: reqData}
		}
		pluginData.EmailThreads = threadsSent
		return pluginData, true
	})
	return trace.Wrap(err)
}

// sendReviews sends notifications on a request updates (new accept/decline review, review expired)
func (a *App) sendReviews(ctx context.Context, reqID string, reqData RequestData, reqReviews []types.AccessReview) error {
	var oldCount int
	var threads []EmailThread

	ok, err := a.modifyPluginData(ctx, reqID, func(existing *PluginData) (PluginData, bool) {
		if existing == nil {
			return PluginData{}, false
		}

		if threads = existing.EmailThreads; len(threads) == 0 {
			return PluginData{}, false
		}

		count := len(reqReviews)
		if oldCount = existing.ReviewsCount; oldCount >= count {
			return PluginData{}, false
		}
		pluginData := *existing
		pluginData.ReviewsCount = count
		return pluginData, true
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if !ok {
		logger.Get(ctx).DebugContext(ctx, "Failed to post reply: plugin data is missing")
		return nil
	}
	reviews := reqReviews[oldCount:]
	if len(reviews) == 0 {
		return nil
	}

	errors := make([]error, 0, len(reviews))
	for _, review := range reviews {
		threadsSent, err := a.client.SendReview(ctx, threads, reqID, reqData, review)
		if err != nil {
			errors = append(errors, err)
		}
		logger.Get(ctx).InfoContext(ctx, "New review for request",
			"request_id", reqID,
			"author", review.Author,
			"state", logutils.StringerAttr(review.ProposedState),
		)
		logSentThreads(ctx, threadsSent, "new review")
	}

	return trace.NewAggregate(errors...)
}

// sendResolution updates the messages status and sends message when request has been resolved
func (a *App) sendResolution(ctx context.Context, reqID string, resolution Resolution) error {
	log := logger.Get(ctx)

	var pluginData PluginData
	ok, err := a.modifyPluginData(ctx, reqID, func(existing *PluginData) (PluginData, bool) {
		// If plugin data is empty or missing email message timestamps, we cannot do anything.
		if existing == nil {
			return PluginData{}, false
		}
		if pluginData = *existing; len(pluginData.EmailThreads) == 0 {
			return PluginData{}, false
		}

		// If resolution field is not empty then we already resolved the incident before. In this case we just quit.
		if pluginData.RequestData.Resolution.Tag != Unresolved {
			return PluginData{}, false
		}

		// Mark plugin data as resolved.
		pluginData.Resolution = resolution
		return pluginData, true
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if !ok {
		log.DebugContext(ctx, "Failed to update messages: plugin data is missing")
		return nil
	}

	reqData, threads := pluginData.RequestData, pluginData.EmailThreads

	threadsSent, err := a.client.SendResolution(ctx, threads, reqID, reqData)
	logSentThreads(ctx, threadsSent, "request resolved")

	log.InfoContext(ctx, "Marked request with resolution and sent emails", "resolution", resolution.Tag)

	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// modifyPluginData performs a compare-and-swap update of access request's plugin data.
//
// Callback function parameter is nil if plugin data hasn't been created yet.
//
// Otherwise, callback function parameter is a pointer to current plugin data contents.
// Callback function return value is an updated plugin data contents plus the boolean flag
// indicating whether it should be written or not.
//
// Note that callback function fn might be called more than once due to retry mechanism baked in
// so make sure that the function is "pure" i.e. it doesn't interact with the outside world:
// it doesn't perform any sort of I/O operations so even things like Go channels must be avoided.
//
// Indeed, this limitation is not that ultimate at least if you know what you're doing.
func (a *App) modifyPluginData(ctx context.Context, reqID string, fn func(data *PluginData) (PluginData, bool)) (bool, error) {
	var lastErr error
	for range maxModifyPluginDataTries {
		oldData, err := a.getPluginData(ctx, reqID)
		if err != nil && !trace.IsNotFound(err) {
			return false, trace.Wrap(err)
		}
		newData, ok := fn(oldData)
		if !ok {
			return false, nil
		}
		var expectData PluginData
		if oldData != nil {
			expectData = *oldData
		}
		err = trace.Wrap(a.updatePluginData(ctx, reqID, newData, expectData))
		if err == nil {
			return true, nil
		}
		if trace.IsCompareFailed(err) {
			lastErr = err
			continue
		}
		return false, err
	}
	return false, lastErr
}

// getPluginData loads a plugin data for a given access request. It returns nil if it's not found.
func (a *App) getPluginData(ctx context.Context, reqID string) (*PluginData, error) {
	dataMaps, err := a.apiClient.GetPluginData(ctx, types.PluginDataFilter{
		Kind:     types.KindAccessRequest,
		Resource: reqID,
		Plugin:   pluginName,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(dataMaps) == 0 {
		return nil, trace.NotFound("plugin data not found")
	}
	entry := dataMaps[0].Entries()[pluginName]
	if entry == nil {
		return nil, trace.NotFound("plugin data not found")
	}
	data := DecodePluginData(entry.Data)
	return &data, nil
}

// updatePluginData updates an existing plugin data or sets a new one if it didn't exist.
func (a *App) updatePluginData(ctx context.Context, reqID string, data PluginData, expectData PluginData) error {
	return a.apiClient.UpdatePluginData(ctx, types.PluginDataUpdateParams{
		Kind:     types.KindAccessRequest,
		Resource: reqID,
		Plugin:   pluginName,
		Set:      EncodePluginData(data),
		Expect:   EncodePluginData(expectData),
	})
}

// logSentThreads logs successfully sent emails
func logSentThreads(ctx context.Context, threads []EmailThread, kind string) {
	for _, thread := range threads {
		logger.Get(ctx).InfoContext(ctx, "Successfully sent",
			"email", thread.Email,
			"timestamp", thread.Timestamp,
			"message_id", thread.MessageID,
			"kind", kind,
		)
	}
}
