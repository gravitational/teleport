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

package pagerduty

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/backoff"
	"github.com/gravitational/teleport/integrations/lib/logger"
	"github.com/gravitational/teleport/integrations/lib/watcherjob"
)

const (
	// minServerVersion is the minimal teleport version the plugin supports.
	minServerVersion = "6.1.0"
	// pluginName is used to tag PluginData and as a Delegator in Audit log.
	pluginName = "pagerduty"
	// initTimeout is used to bound execution time of health check and teleport version check.
	initTimeout = time.Second * 10
	// handlerTimeout is used to bound the execution time of watcher event handler.
	handlerTimeout = time.Second * 30
	// modifyPluginDataBackoffBase is an initial (minimum) backoff value.
	modifyPluginDataBackoffBase = time.Millisecond
	// modifyPluginDataBackoffMax is a backoff threshold
	modifyPluginDataBackoffMax = time.Second
)

// Special kind of error that can be ignored.
var errSkip = errors.New("")

// App contains global application state of the PagerDuty plugin.
type App struct {
	conf Config

	teleport   teleport.Client
	pagerduty  Pagerduty
	statusSink common.StatusSink
	mainJob    lib.ServiceJob

	*lib.Process
}

// NewApp constructs a new PagerDuty App.
func NewApp(conf Config) (*App, error) {
	app := &App{
		conf:       conf,
		teleport:   conf.Client,
		statusSink: conf.StatusSink,
	}
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

func (a *App) run(ctx context.Context) error {
	var err error

	log := logger.Get(ctx)
	log.Infof("Starting Teleport Access PagerDuty Plugin")

	if err = a.init(ctx); err != nil {
		return trace.Wrap(err)
	}

	watcherJob, err := watcherjob.NewJob(
		a.teleport,
		watcherjob.Config{
			Watch:            types.Watch{Kinds: []types.WatchKind{{Kind: types.KindAccessRequest}}},
			EventFuncTimeout: handlerTimeout,
		},
		a.onWatcherEvent,
	)
	if err != nil {
		return trace.Wrap(err)
	}
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

func (a *App) init(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, initTimeout)
	defer cancel()
	log := logger.Get(ctx)

	var (
		err  error
		pong proto.PingResponse
	)

	if a.teleport == nil {
		if a.teleport, err = common.GetTeleportClient(ctx, a.conf.Teleport); err != nil {
			return trace.Wrap(err)
		}
	}

	if pong, err = a.checkTeleportVersion(ctx); err != nil {
		return trace.Wrap(err)
	}

	var webProxyAddr string
	if pong.ServerFeatures.AdvancedAccessWorkflows {
		webProxyAddr = pong.ProxyPublicAddr
	}
	a.pagerduty, err = NewPagerdutyClient(a.conf.Pagerduty, pong.ClusterName, webProxyAddr, a.statusSink)
	if err != nil {
		return trace.Wrap(err)
	}

	log.Debug("Starting PagerDuty API health check...")
	if err = a.pagerduty.HealthCheck(ctx); err != nil {
		return trace.Wrap(err, "api health check failed. check your credentials and service_id settings")
	}
	log.Debug("PagerDuty API health check finished ok")

	return nil
}

func (a *App) checkTeleportVersion(ctx context.Context) (proto.PingResponse, error) {
	log := logger.Get(ctx)
	log.Debug("Checking Teleport server version")

	pong, err := a.teleport.Ping(ctx)
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

func (a *App) onWatcherEvent(ctx context.Context, event types.Event) error {
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
		case req.GetState().IsResolved():
			err = a.onResolvedRequest(ctx, req)
		default:
			log.WithField("event", event).Warn("Unknown request state")
			return nil
		}

		if err != nil {
			log.WithError(err).Error("Failed to process request")
			return trace.Wrap(err)
		}

		return nil
	case types.OpDelete:
		ctx, log := logger.WithField(ctx, "request_op", "delete")

		if err := a.onDeletedRequest(ctx, reqID); err != nil {
			log.WithError(err).Error("Failed to process deleted request")
			return trace.Wrap(err)
		}
		return nil
	default:
		return trace.BadParameter("unexpected event operation %s", op)
	}
}

func (a *App) onPendingRequest(ctx context.Context, req types.AccessRequest) error {
	if len(req.GetSystemAnnotations()) == 0 {
		logger.Get(ctx).Debug("Cannot proceed further. Request is missing any annotations")
		return nil
	}

	// First, try to create a notification incident.
	isNew, notifyErr := a.tryNotifyService(ctx, req)
	notifyErr = trace.Wrap(notifyErr)

	// To minimize the count of auto-approval tries, lets attempt it only when we just created an incident.
	// But if there's an error, we can't really know is the incident new or not so lets just try.
	if !isNew && notifyErr == nil {
		return nil
	}
	// Don't show the error if the annotation is just missing.
	if trace.Unwrap(notifyErr) == errSkip {
		notifyErr = nil
	}

	// Then, try to approve the request if user is currently on-call.
	approveErr := trace.Wrap(a.tryApproveRequest(ctx, req))
	return trace.NewAggregate(notifyErr, approveErr)
}

func (a *App) onResolvedRequest(ctx context.Context, req types.AccessRequest) error {
	var notifyErr error
	if err := a.postReviewNotes(ctx, req.GetName(), req.GetReviews()); err != nil {
		notifyErr = trace.Wrap(err)
	}

	resolution := Resolution{Reason: req.GetResolveReason()}
	switch req.GetState() {
	case types.RequestState_APPROVED:
		resolution.Tag = ResolvedApproved
	case types.RequestState_DENIED:
		resolution.Tag = ResolvedDenied
	case types.RequestState_PROMOTED:
		resolution.Tag = ResolvedPromoted
	}
	err := trace.Wrap(a.resolveIncident(ctx, req.GetName(), resolution))
	return trace.NewAggregate(notifyErr, err)
}

func (a *App) onDeletedRequest(ctx context.Context, reqID string) error {
	return a.resolveIncident(ctx, reqID, Resolution{Tag: ResolvedExpired})
}

func (a *App) getNotifyServiceName(req types.AccessRequest) (string, error) {
	annotationKey := a.conf.Pagerduty.RequestAnnotations.NotifyService
	// We cannot use common.GetServiceNamesFromAnnotations here as it sorts the
	// list and might change the first element.
	// The proper way would be to support notifying multiple services
	slice, ok := req.GetSystemAnnotations()[annotationKey]
	if !ok {
		return "", trace.Errorf("request annotation %s is missing", annotationKey)
	}
	var serviceName string
	if len(slice) > 0 {
		serviceName = slice[0]
	}
	if serviceName == "" {
		return "", trace.Errorf("request annotation %s is present but empty", annotationKey)
	}
	return serviceName, nil
}

func (a *App) getOnCallServiceNames(req types.AccessRequest) ([]string, error) {
	annotationKey := a.conf.Pagerduty.RequestAnnotations.Services
	return common.GetServiceNamesFromAnnotations(req, annotationKey)
}

func (a *App) tryNotifyService(ctx context.Context, req types.AccessRequest) (bool, error) {
	log := logger.Get(ctx)

	serviceName, err := a.getNotifyServiceName(req)
	if err != nil {
		log.Debugf("Skipping the notification: %s", err)
		return false, trace.Wrap(errSkip)
	}

	ctx, _ = logger.WithField(ctx, "pd_service_name", serviceName)
	service, err := a.pagerduty.FindServiceByName(ctx, serviceName)
	if err != nil {
		return false, trace.Wrap(err, "finding pagerduty service %s", serviceName)
	}

	reqID := req.GetName()
	reqData := RequestData{
		User:          req.GetUser(),
		Roles:         req.GetRoles(),
		Created:       req.GetCreationTime(),
		RequestReason: req.GetRequestReason(),
	}

	// Create plugin data if it didn't exist before.
	isNew, err := a.modifyPluginData(ctx, reqID, func(existing *PluginData) (PluginData, bool) {
		if existing != nil {
			return PluginData{}, false
		}
		return PluginData{RequestData: reqData}, true
	})
	if err != nil {
		return isNew, trace.Wrap(err, "updating plugin data")
	}

	if isNew {
		if err = a.createIncident(ctx, service.ID, reqID, reqData); err != nil {
			return isNew, trace.Wrap(err, "creating PagerDuty incident")
		}
	}

	if reqReviews := req.GetReviews(); len(reqReviews) > 0 {
		if err = a.postReviewNotes(ctx, reqID, reqReviews); err != nil {
			return isNew, trace.Wrap(err)
		}
	}

	return isNew, nil
}

// createIncident posts an incident with request information.
func (a *App) createIncident(ctx context.Context, serviceID, reqID string, reqData RequestData) error {
	data, err := a.pagerduty.CreateIncident(ctx, serviceID, reqID, reqData)
	if err != nil {
		return trace.Wrap(err)
	}
	ctx, log := logger.WithField(ctx, "pd_incident_id", data.IncidentID)
	log.Info("Successfully created PagerDuty incident")

	// Save pagerduty incident info in plugin data.
	_, err = a.modifyPluginData(ctx, reqID, func(existing *PluginData) (PluginData, bool) {
		var pluginData PluginData
		if existing != nil {
			pluginData = *existing
		} else {
			// It must be impossible but lets handle it just in case.
			pluginData = PluginData{RequestData: reqData}
		}
		pluginData.PagerdutyData = data
		return pluginData, true
	})
	return trace.Wrap(err)
}

// postReviewNotes posts incident notes about new reviews appeared for request.
func (a *App) postReviewNotes(ctx context.Context, reqID string, reqReviews []types.AccessReview) error {
	var oldCount int
	var data PagerdutyData

	// Increase the review counter in plugin data.
	ok, err := a.modifyPluginData(ctx, reqID, func(existing *PluginData) (PluginData, bool) {
		if existing == nil {
			return PluginData{}, false
		}

		if data = existing.PagerdutyData; data.IncidentID == "" {
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
		logger.Get(ctx).Debug("Failed to post the note: plugin data is missing")
		return nil
	}
	ctx, _ = logger.WithField(ctx, "pd_incident_id", data.IncidentID)

	slice := reqReviews[oldCount:]
	if len(slice) == 0 {
		return nil
	}

	errors := make([]error, 0, len(slice))
	for _, review := range slice {
		if err := a.pagerduty.PostReviewNote(ctx, data.IncidentID, review); err != nil {
			errors = append(errors, err)
		}
	}
	return trace.NewAggregate(errors...)
}

// tryApproveRequest attempts to submit an approval if the requesting user is on-call in one of the services provided in request annotation.
func (a *App) tryApproveRequest(ctx context.Context, req types.AccessRequest) error {
	log := logger.Get(ctx)

	serviceNames, err := a.getOnCallServiceNames(req)
	if err != nil {
		logger.Get(ctx).Debugf("Skipping the approval: %s", err)
		return nil
	}

	userName := req.GetUser()
	if !lib.IsEmail(userName) {
		logger.Get(ctx).Warningf("Skipping the approval: %q does not look like a valid email", userName)
		return nil
	}

	user, err := a.pagerduty.FindUserByEmail(ctx, userName)
	if err != nil {
		if trace.IsNotFound(err) {
			log.WithError(err).WithField("pd_user_email", userName).Debug("Skipping the approval: email is not found")
			return nil
		}
		return trace.Wrap(err)
	}

	ctx, log = logger.WithFields(ctx, logger.Fields{
		"pd_user_email": user.Email,
		"pd_user_name":  user.Name,
	})

	services, err := a.pagerduty.FindServicesByNames(ctx, serviceNames)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(services) == 0 {
		log.WithField("pd_service_names", serviceNames).Warning("Failed to find any service")
		return nil
	}

	escalationPolicyMapping := make(map[string][]Service)
	for _, service := range services {
		escalationPolicyMapping[service.EscalationPolicy.ID] = append(escalationPolicyMapping[service.EscalationPolicy.ID], service)
	}
	var escalationPolicyIDs []string
	for id := range escalationPolicyMapping {
		escalationPolicyIDs = append(escalationPolicyIDs, id)
	}

	if escalationPolicyIDs, err = a.pagerduty.FilterOnCallPolicies(ctx, user.ID, escalationPolicyIDs); err != nil {
		return trace.Wrap(err)
	}
	if len(escalationPolicyIDs) == 0 {
		log.Debug("Skipping the approval: user is not on call")
		return nil
	}

	serviceNames = make([]string, 0, len(services))
	for _, policyID := range escalationPolicyIDs {
		for _, service := range escalationPolicyMapping[policyID] {
			serviceNames = append(serviceNames, service.Name)
		}
	}

	if _, err := a.teleport.SubmitAccessReview(ctx, types.AccessReviewSubmission{
		RequestID: req.GetName(),
		Review: types.AccessReview{
			Author:        a.conf.TeleportUser,
			ProposedState: types.RequestState_APPROVED,
			Reason: fmt.Sprintf("Access requested by user %s (%s) who is on call in service(s) %s",
				user.Name,
				user.Email,
				strings.Join(serviceNames, ","),
			),
			Created: time.Now(),
		},
	}); err != nil {
		if strings.HasSuffix(err.Error(), "has already reviewed this request") {
			log.Debug("Already reviewed the request")
			return nil
		}
		return trace.Wrap(err, "submitting access request")
	}

	log.Info("Successfully submitted a request approval")
	return nil
}

// resolveIncident resolves the notification incident created by plugin if the incident exists.
func (a *App) resolveIncident(ctx context.Context, reqID string, resolution Resolution) error {
	var incidentID string

	// Save request resolution info in plugin data.
	ok, err := a.modifyPluginData(ctx, reqID, func(existing *PluginData) (PluginData, bool) {
		// If plugin data is empty or missing incidentID, we cannot do anything.
		if existing == nil {
			return PluginData{}, false
		}
		if incidentID = existing.IncidentID; incidentID == "" {
			return PluginData{}, false
		}

		// If resolution field is not empty then we already resolved the incident before. In this case we just quit.
		if existing.RequestData.Resolution.Tag != Unresolved {
			return PluginData{}, false
		}

		// Mark incident as resolved.
		pluginData := *existing
		pluginData.Resolution = resolution
		return pluginData, true
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if !ok {
		logger.Get(ctx).Debug("Failed to resolve the incident: plugin data is missing")
		return nil
	}

	ctx, log := logger.WithField(ctx, "pd_incident_id", incidentID)
	if err := a.pagerduty.ResolveIncident(ctx, incidentID, resolution); err != nil {
		return trace.Wrap(err)
	}
	log.Info("Successfully resolved the incident")

	return nil
}

// modifyPluginData performs a compare-and-swap update of access request's plugin data.
// Callback function parameter is nil if plugin data hasn't been created yet.
// Otherwise, callback function parameter is a pointer to current plugin data contents.
// Callback function return value is an updated plugin data contents plus the boolean flag
// indicating whether it should be written or not.
// Note that callback function fn might be called more than once due to retry mechanism baked in
// so make sure that the function is "pure" i.e. it doesn't interact with the outside world:
// it doesn't perform any sort of I/O operations so even things like Go channels must be avoided.
// Indeed, this limitation is not that ultimate at least if you know what you're doing.
func (a *App) modifyPluginData(ctx context.Context, reqID string, fn func(data *PluginData) (PluginData, bool)) (bool, error) {
	backoff := backoff.NewDecorr(modifyPluginDataBackoffBase, modifyPluginDataBackoffMax, clockwork.NewRealClock())
	for {
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
		if !trace.IsCompareFailed(err) {
			return false, trace.Wrap(err)
		}
		if err := backoff.Do(ctx); err != nil {
			return false, trace.Wrap(err)
		}
	}
}

// getPluginData loads a plugin data for a given access request. It returns nil if it's not found.
func (a *App) getPluginData(ctx context.Context, reqID string) (*PluginData, error) {
	dataMaps, err := a.teleport.GetPluginData(ctx, types.PluginDataFilter{
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
		return nil, trace.NotFound("plugin data entry not found")
	}
	data := DecodePluginData(entry.Data)
	return &data, nil
}

// updatePluginData updates an existing plugin data or sets a new one if it didn't exist.
func (a *App) updatePluginData(ctx context.Context, reqID string, data PluginData, expectData PluginData) error {
	return a.teleport.UpdatePluginData(ctx, types.PluginDataUpdateParams{
		Kind:     types.KindAccessRequest,
		Resource: reqID,
		Plugin:   pluginName,
		Set:      EncodePluginData(data),
		Expect:   EncodePluginData(expectData),
	})
}
