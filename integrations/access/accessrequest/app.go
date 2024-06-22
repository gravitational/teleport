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
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/accessrequest"
	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
	pd "github.com/gravitational/teleport/integrations/lib/plugindata"
	"github.com/gravitational/teleport/integrations/lib/watcherjob"
)

const (
	// handlerTimeout is used to bound the execution time of watcher event handler.
	handlerTimeout = time.Second * 5
	// defaultAccessMonitoringRulePageSize is the default number of rules to retrieve per request
	defaultAccessMonitoringRulePageSize = 10
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

	accessMonitoringRules amrMap
}

type amrMap struct {
	sync.RWMutex
	rules map[string]*accessmonitoringrulesv1.AccessMonitoringRule
}

// NewApp will create a new access request application.
func NewApp(bot MessagingBot) common.App {
	app := &App{accessMonitoringRules: amrMap{
		rules: make(map[string]*accessmonitoringrulesv1.AccessMonitoringRule),
	}}
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

	job, err := watcherjob.NewJob(
		a.apiClient,
		watcherjob.Config{
			Watch: types.Watch{Kinds: []types.WatchKind{
				{Kind: types.KindAccessRequest},
				{Kind: types.KindAccessMonitoringRule},
			}},
			EventFuncTimeout: handlerTimeout,
		},
		a.onWatcherEvent,
	)
	if err != nil {
		return trace.Wrap(err)
	}

	process.SpawnCriticalJob(job)

	ok, err := job.WaitReady(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := a.initAccessMonitoringRulesCache(ctx); err != nil {
		return trace.Wrap(err)
	}

	a.job.SetReady(ok)
	if !ok {
		return trace.BadParameter("job not ready")
	}

	<-job.Done()
	return nil
}

func (a *App) amrAppliesToThisPlugin(amr *accessmonitoringrulesv1.AccessMonitoringRule) bool {
	if amr.Spec.Notification.Name != a.pluginName {
		return false
	}
	return slices.ContainsFunc(amr.Spec.Subjects, func(subject string) bool {
		return subject == types.KindAccessRequest
	})
}

// onWatcherEvent is called for every cluster Event. It will filter out non-access-request events and
// call onPendingRequest, onResolvedRequest and on DeletedRequest depending on the event.
func (a *App) onWatcherEvent(ctx context.Context, event types.Event) error {
	switch event.Resource.GetKind() {
	case types.KindAccessMonitoringRule:
		return trace.Wrap(a.handleAccessMonitoringRule(ctx, event))
	case types.KindAccessRequest:
		return trace.Wrap(a.handleAcessRequest(ctx, event))
	}
	return trace.BadParameter("unexpected kind %s", event.Resource.GetKind())
}

func (a *App) handleAcessRequest(ctx context.Context, event types.Event) error {
	op := event.Type
	reqID := event.Resource.GetName()
	ctx, _ = logger.WithField(ctx, "request_id", reqID)

	switch op {
	case types.OpPut:
		ctx, _ = logger.WithField(ctx, "request_op", "put")
		req, ok := event.Resource.(types.AccessRequest)
		if !ok {
			return trace.BadParameter("unexpected resource type %T", event.Resource)
		}
		ctx, log := logger.WithField(ctx, "request_state", req.GetState().String())

		var err error
		switch {
		case req.GetState().IsPending():
			err = a.onPendingRequest(ctx, req)
		case req.GetState().IsResolved():
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

func (a *App) handleAccessMonitoringRule(ctx context.Context, event types.Event) error {
	if kind := event.Resource.GetKind(); kind != types.KindAccessMonitoringRule {
		return trace.BadParameter("expected %s resource kind, got %s", types.KindAccessMonitoringRule, kind)
	}

	a.accessMonitoringRules.Lock()
	defer a.accessMonitoringRules.Unlock()
	switch op := event.Type; op {
	case types.OpPut:
		e, ok := event.Resource.(types.Resource153Unwrapper)
		if !ok {
			return trace.BadParameter("expected Resource153Unwrapper resource type, got %T", event.Resource)
		}
		req, ok := e.Unwrap().(*accessmonitoringrulesv1.AccessMonitoringRule)
		if !ok {
			return trace.BadParameter("expected AccessMonitoringRule resource type, got %T", event.Resource)
		}

		// In the event an existing rule no longer applies we must remove it.
		if !a.amrAppliesToThisPlugin(req) {
			delete(a.accessMonitoringRules.rules, event.Resource.GetName())
			return nil
		}
		a.accessMonitoringRules.rules[req.Metadata.Name] = req
		return nil
	case types.OpDelete:
		delete(a.accessMonitoringRules.rules, event.Resource.GetName())
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

	reqData := pd.AccessRequestData{
		User:              req.GetUser(),
		Roles:             req.GetRoles(),
		RequestReason:     req.GetRequestReason(),
		SystemAnnotations: req.GetSystemAnnotations(),
		Resources:         resourceNames,
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
		logger.Get(ctx).Warningf("Unknown state %v (%s)", state, state.String())
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
		logger.Get(ctx).WithFields(logger.Fields{
			"channel_id": data.ChannelID,
			"message_id": data.MessageID,
		}).Info("Successfully posted messages")
	}
	if err != nil {
		logger.Get(ctx).WithError(err).Error("Failed to post one or more messages")
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
func (a *App) getMessageRecipients(ctx context.Context, req types.AccessRequest) []common.Recipient {
	log := logger.Get(ctx)

	// We receive a set from GetRawRecipientsFor but we still might end up with duplicate channel names.
	// This can happen if this set contains the channel `C` and the email for channel `C`.
	recipientSet := common.NewRecipientSet()

	recipients := a.recipientsFromAccessMonitoringRules(ctx, req)
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
				log.Warningf("Failed to fetch Opsgenie recipient: %v", err)
				continue
			}
			recipientSet.Add(*rec)
		}
		return recipientSet.ToSlice()
	}

	validEmailSuggReviewers := []string{}
	for _, reviewer := range req.GetSuggestedReviewers() {
		if !lib.IsEmail(reviewer) {
			log.Warningf("Failed to notify a suggested reviewer: %q does not look like a valid email", reviewer)
			continue
		}

		validEmailSuggReviewers = append(validEmailSuggReviewers, reviewer)
	}
	rawRecipients := a.recipients.GetRawRecipientsFor(req.GetRoles(), validEmailSuggReviewers)
	for _, rawRecipient := range rawRecipients {
		recipient, err := a.bot.FetchRecipient(ctx, rawRecipient)
		if err != nil {
			log.WithError(err).Warn("Failure when fetching recipient, continuing anyway")
		} else {
			recipientSet.Add(*recipient)
		}
	}

	return recipientSet.ToSlice()
}

func (a *App) recipientsFromAccessMonitoringRules(ctx context.Context, req types.AccessRequest) *common.RecipientSet {
	log := logger.Get(ctx)
	recipientSet := common.NewRecipientSet()

	// This switch is used to determine which plugins we are enabling access monitoring notification rules for.
	switch a.pluginType {
	// Enabled plugins are added to this case.
	case types.PluginTypeSlack:
		log.Debug("Applying access monitoring rules to request")
	default:
		return &recipientSet
	}

	for _, rule := range a.getAccessMonitoringRules() {
		match, err := matchAccessRequest(rule.Spec.Condition, req)
		if err != nil {
			log.WithError(err).WithField("rule", rule.Metadata.Name).
				Warn("Failed to parse access monitoring notification rule")
		}
		if !match {
			continue
		}
		for _, recipient := range rule.Spec.Notification.Recipients {
			rec, err := a.bot.FetchRecipient(ctx, recipient)
			if err != nil {
				log.WithError(err).Warn("Failed to fetch plugin recipients based on Access moniotring rule recipients")
				continue
			}
			recipientSet.Add(*rec)
		}
	}
	return &recipientSet
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

	if err := a.bot.NotifyUser(ctx, reqID, reqData); err != nil && !trace.IsNotImplemented(err) {
		return trace.Wrap(err)
	}

	log.Infof("Successfully notified user %s request marked as %s", reqData.User, tag)

	return nil
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

func (a *App) initAccessMonitoringRulesCache(ctx context.Context) error {
	accessMonitoringRules, err := a.getAllAccessMonitoringRules(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	a.accessMonitoringRules.Lock()
	defer a.accessMonitoringRules.Unlock()
	for _, amr := range accessMonitoringRules {
		if !a.amrAppliesToThisPlugin(amr) {
			continue
		}
		a.accessMonitoringRules.rules[amr.GetMetadata().Name] = amr
	}
	return nil
}

func (a *App) getAllAccessMonitoringRules(ctx context.Context) ([]*accessmonitoringrulesv1.AccessMonitoringRule, error) {
	var resources []*accessmonitoringrulesv1.AccessMonitoringRule
	var nextToken string
	for {
		var page []*accessmonitoringrulesv1.AccessMonitoringRule
		var err error
		page, nextToken, err = a.apiClient.ListAccessMonitoringRulesWithFilter(ctx, defaultAccessMonitoringRulePageSize, nextToken, []string{types.KindAccessRequest}, a.pluginName)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		resources = append(resources, page...)

		if nextToken == "" {
			break
		}
	}
	return resources, nil
}

func (a *App) getAccessMonitoringRules() map[string]*accessmonitoringrulesv1.AccessMonitoringRule {
	a.accessMonitoringRules.RLock()
	defer a.accessMonitoringRules.RUnlock()
	return maps.Clone(a.accessMonitoringRules.rules)
}
