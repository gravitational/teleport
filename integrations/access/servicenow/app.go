/*
Copyright 2023 Gravitational, Inc.

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

package servicenow

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/grpc"
	grpcbackoff "google.golang.org/grpc/backoff"

	tp "github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/backoff"
	"github.com/gravitational/teleport/integrations/lib/credentials"
	"github.com/gravitational/teleport/integrations/lib/logger"
	"github.com/gravitational/teleport/integrations/lib/watcherjob"
)

const (
	// pluginName is used to tag Servicenow GenericPluginData and as a Delegator in Audit log.
	pluginName = "servicenow"
	// minServerVersion is the minimal teleport version the plugin supports.
	minServerVersion = "6.1.0"
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

// errMissingAnnotation is used for cases where request annotations are not set
var errMissingAnnotation = errors.New("access request is missing annotations")

// App is a wrapper around the base app to allow for extra functionality.
type App struct {
	*lib.Process

	PluginName string
	teleport   teleport.Client
	servicenow *Client
	mainJob    lib.ServiceJob
	conf       Config
}

// NewServicenowApp initializes a new teleport-servicenow app and returns it.
func NewServicenowApp(ctx context.Context, conf *Config) (*App, error) {
	teleportClient, err := conf.GetTeleportClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	servicenowApp := &App{
		PluginName: pluginName,
		teleport:   teleportClient,
		conf:       *conf,
	}
	servicenowApp.mainJob = lib.NewServiceJob(servicenowApp.run)
	return servicenowApp, nil
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
	log.Infof("Starting Teleport Access Servicenow Plugin")

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

	if validCred, err := credentials.CheckIfExpired(a.conf.Teleport.Credentials()); err != nil {
		log.Warnf("Invalid Teleport credentials: %v", err)
		if !validCred {
			return trace.BadParameter(
				"No valid credentials found, this likely means credentials are expired. In this case, please sign new credentials and increase their TTL if needed.",
			)
		}
		log.Info("At least one non-expired credential has been found, continuing startup")
	}

	var err error

	if a.teleport == nil {
		bk := grpcbackoff.DefaultConfig
		bk.MaxDelay = grpcBackoffMaxDelay
		if a.teleport, err = client.New(ctx, client.Config{
			Addrs:       a.conf.Teleport.GetAddrs(),
			Credentials: a.conf.Teleport.Credentials(),
			DialOpts: []grpc.DialOption{
				grpc.WithConnectParams(grpc.ConnectParams{Backoff: bk, MinConnectTimeout: initTimeout}),
				grpc.WithReturnConnectionError(),
			},
		}); err != nil {
			return trace.Wrap(err)
		}
	}

	pong, err := a.checkTeleportVersion(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	webProxyURL, err := url.Parse(pong.ProxyPublicAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	a.conf.ClientConfig.WebProxyURL = webProxyURL
	a.servicenow, err = NewClient(a.conf.ClientConfig)
	if err != nil {
		return trace.Wrap(err)
	}
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
		case req.GetState().IsApproved():
			err = a.onResolvedRequest(ctx, req)
		case req.GetState().IsDenied():
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

	// To minimize the count of auto-approval tries, let's only attempt it only when we have just created an incident.
	// But if there's an error, we can't really know if the incident is new or not so lets just try.
	if !isNew && notifyErr == nil {
		return nil
	}
	// Don't show the error if the annotation is just missing.
	if trace.Unwrap(notifyErr) == errMissingAnnotation {
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
	case types.RequestState_APPROVED, types.RequestState_DENIED:
		resolution.State = ResolutionStateResolved
	}

	// TODO: Add in close codes

	err := trace.Wrap(a.resolveIncident(ctx, req.GetName(), resolution))
	return trace.NewAggregate(notifyErr, err)
}

func (a *App) onDeletedRequest(ctx context.Context, reqID string) error {
	return a.resolveIncident(ctx, reqID, Resolution{State: ResolutionStateResolved})
}

func (a *App) getNotifyServiceNames(req types.AccessRequest) ([]string, error) {
	services, ok := req.GetSystemAnnotations()[types.TeleportNamespace+types.ReqAnnotationNotifyServicesLabel]
	if !ok {
		return nil, trace.NotFound("notify services not specified")
	}
	return services, nil
}

func (a *App) getOnCallServiceNames(req types.AccessRequest) ([]string, error) {
	services, ok := req.GetSystemAnnotations()[types.TeleportNamespace+types.ReqAnnotationSchedulesLabel]
	if !ok {
		return nil, trace.NotFound("on-call schedules not specified")
	}
	return services, nil
}

func (a *App) tryNotifyService(ctx context.Context, req types.AccessRequest) (bool, error) {
	log := logger.Get(ctx)

	serviceNames, err := a.getNotifyServiceNames(req)
	if err != nil {
		log.Debugf("Skipping the notification: %s", err)
		return false, trace.Wrap(errMissingAnnotation)
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
		for _, serviceName := range serviceNames {
			incidentCtx, _ := logger.WithField(ctx, "servicenow_service_name", serviceName)

			if err = a.createIncident(incidentCtx, serviceName, reqID, reqData); err != nil {
				return isNew, trace.Wrap(err, "creating Servicenow incident")
			}
		}

		if reqReviews := req.GetReviews(); len(reqReviews) > 0 {
			if err = a.postReviewNotes(ctx, reqID, reqReviews); err != nil {
				return isNew, trace.Wrap(err)
			}
		}
	}
	return isNew, nil
}

// createIncident posts an incident with request information.
func (a *App) createIncident(ctx context.Context, serviceID, reqID string, reqData RequestData) error {
	data, err := a.servicenow.CreateIncident(ctx, reqID, reqData)
	if err != nil {
		return trace.Wrap(err)
	}
	ctx, log := logger.WithField(ctx, "servicenow_incident_id", data.IncidentID)
	log.Info("Successfully created Servicenow incident")

	// Save servicenow incident info in plugin data.
	_, err = a.modifyPluginData(ctx, reqID, func(existing *PluginData) (PluginData, bool) {
		var pluginData PluginData
		if existing != nil {
			pluginData = *existing
		} else {
			// It must be impossible but lets handle it just in case.
			pluginData = PluginData{RequestData: reqData}
		}
		pluginData.ServicenowData = ServicenowData{IncidentID: data.IncidentID}
		return pluginData, true
	})
	return trace.Wrap(err)
}

// postReviewNotes posts incident notes about new reviews appeared for request.
func (a *App) postReviewNotes(ctx context.Context, reqID string, reqReviews []types.AccessReview) error {
	var oldCount int
	var data ServicenowData

	// Increase the review counter in plugin data.
	ok, err := a.modifyPluginData(ctx, reqID, func(existing *PluginData) (PluginData, bool) {
		if existing == nil {
			return PluginData{}, false
		}

		if data = existing.ServicenowData; data.IncidentID == "" {
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
	ctx, _ = logger.WithField(ctx, "servicenow_incident_id", data.IncidentID)

	slice := reqReviews[oldCount:]
	if len(slice) == 0 {
		return nil
	}

	errors := make([]error, 0, len(slice))
	for _, review := range slice {
		if err := a.servicenow.PostReviewNote(ctx, data.IncidentID, review); err != nil {
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

	onCallUsers := []string{}
	for _, scheduleName := range serviceNames {
		respondersResult, err := a.servicenow.GetOnCall(ctx, scheduleName)
		if err != nil {
			return trace.Wrap(err)
		}
		onCallUsers = append(onCallUsers, respondersResult...)
	}

	userIsOnCall := false
	for _, user := range onCallUsers {
		if req.GetUser() == user {
			userIsOnCall = true
		}
	}
	if userIsOnCall {
		if _, err := a.teleport.SubmitAccessReview(ctx, types.AccessReviewSubmission{
			RequestID: req.GetName(),
			Review: types.AccessReview{
				Author:        tp.SystemAccessApproverUserName,
				ProposedState: types.RequestState_APPROVED,
				Reason: fmt.Sprintf("Access requested by user %s who is on call on service(s) %s",
					tp.SystemAccessApproverUserName,
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

		// If close notes field is not empty then we already resolved the incident before. In this case we just quit.
		if existing.RequestData.Resolution.CloseCode != "" {
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

	ctx, log := logger.WithField(ctx, "servicenow_incident_id", incidentID)
	if err := a.servicenow.ResolveIncident(ctx, incidentID, resolution); err != nil {
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
	data, err := DecodePluginData(entry.Data)
	if err != nil {
		return nil, trace.Wrap(err)
	}
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
