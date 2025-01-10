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

package servicenow

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	tp "github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/accessrequest"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/accessmonitoring"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/backoff"
	"github.com/gravitational/teleport/integrations/lib/logger"
	"github.com/gravitational/teleport/integrations/lib/watcherjob"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

const (
	// pluginName is used to tag Servicenow GenericPluginData and as a Delegator in Audit log.
	pluginName = "servicenow"
	// minServerVersion is the minimal teleport version the plugin supports.
	minServerVersion = "13.0.0"
	// initTimeout is used to bound execution time of health check and teleport version check.
	initTimeout = time.Second * 10
	// modifyPluginDataBackoffBase is an initial (minimum) backoff value.
	modifyPluginDataBackoffBase = time.Millisecond
	// modifyPluginDataBackoffMax is a backoff threshold
	modifyPluginDataBackoffMax = time.Second
)

// App is a wrapper around the base app to allow for extra functionality.
type App struct {
	*lib.Process
	common.BaseApp

	PluginName            string
	teleport              teleport.Client
	serviceNow            ServiceNowClient
	mainJob               lib.ServiceJob
	conf                  Config
	accessMonitoringRules *accessmonitoring.RuleHandler
}

// NewServicenowApp initializes a new teleport-servicenow app and returns it.
func NewServiceNowApp(ctx context.Context, conf *Config) (*App, error) {
	serviceNowApp := &App{
		PluginName: pluginName,
		conf:       *conf,
	}
	teleClient, err := conf.GetTeleportClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	serviceNowApp.accessMonitoringRules = accessmonitoring.NewRuleHandler(accessmonitoring.RuleHandlerConfig{
		Client:     teleClient,
		PluginType: string(conf.PluginType),
		PluginName: pluginName,
		FetchRecipientCallback: func(_ context.Context, name string) (*common.Recipient, error) {
			return &common.Recipient{
				Name: name,
				ID:   name,
				Kind: common.RecipientKindSchedule,
			}, nil
		},
	})
	serviceNowApp.mainJob = lib.NewServiceJob(serviceNowApp.run)
	return serviceNowApp, nil
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

// WaitReady waits for access request watcher to start up.
func (a *App) WaitReady(ctx context.Context) (bool, error) {
	return a.mainJob.WaitReady(ctx)
}

func (a *App) run(ctx context.Context) error {
	log := logger.Get(ctx)
	log.InfoContext(ctx, "Starting Teleport Access Servicenow Plugin")

	if err := a.init(ctx); err != nil {
		return trace.Wrap(err)
	}
	watchKinds := []types.WatchKind{
		{Kind: types.KindAccessRequest},
		{Kind: types.KindAccessMonitoringRule},
	}

	acceptedWatchKinds := make([]string, 0, len(watchKinds))
	watcherJob, err := watcherjob.NewJobWithConfirmedWatchKinds(
		a.teleport,
		watcherjob.Config{
			Watch: types.Watch{Kinds: watchKinds, AllowPartialSuccess: true},
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
	if err := a.accessMonitoringRules.InitAccessMonitoringRulesCache(ctx); err != nil {
		return trace.Wrap(err)
	}
	a.mainJob.SetReady(ok)
	if ok {
		log.InfoContext(ctx, "ServiceNow plugin is ready")
	} else {
		log.ErrorContext(ctx, "ServiceNow plugin is not ready")
	}

	<-watcherJob.Done()

	return trace.Wrap(watcherJob.Err())
}

func (a *App) init(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, initTimeout)
	defer cancel()
	log := logger.Get(ctx)

	var err error
	a.teleport, err = a.conf.GetTeleportClient(ctx)
	if err != nil {
		return trace.Wrap(err, "getting teleport client")
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
	a.serviceNow, err = NewClient(a.conf.ClientConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	log.DebugContext(ctx, "Starting API health check")
	if err = a.serviceNow.CheckHealth(ctx); err != nil {
		return trace.Wrap(err, "API health check failed")
	}
	log.DebugContext(ctx, "API health check finished ok")

	return nil
}

func (a *App) checkTeleportVersion(ctx context.Context) (proto.PingResponse, error) {
	log := logger.Get(ctx)
	log.DebugContext(ctx, "Checking Teleport server version")

	pong, err := a.teleport.Ping(ctx)
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
	reqID := req.GetName()
	log := logger.Get(ctx).With("req_id", reqID)

	resourceNames, err := a.getResourceNames(ctx, req)
	if err != nil {
		return trace.Wrap(err)
	}

	reqData := RequestData{
		User:              req.GetUser(),
		Roles:             req.GetRoles(),
		RequestReason:     req.GetRequestReason(),
		SystemAnnotations: req.GetSystemAnnotations(),
		Resources:         resourceNames,
	}

	// Create plugin data if it didn't exist before.
	isNew, err := a.modifyPluginData(ctx, reqID, func(existing *PluginData) (PluginData, bool) {
		if existing != nil {
			return PluginData{}, false
		}
		return PluginData{RequestData: reqData}, true
	})
	if err != nil {
		return trace.Wrap(err, "updating plugin data")
	}

	if isNew {
		log.InfoContext(ctx, "Creating servicenow incident")
		recipientAssignee := a.accessMonitoringRules.RecipientsFromAccessMonitoringRules(ctx, req)
		assignees := []string{}
		recipientAssignee.ForEach(func(r common.Recipient) {
			assignees = append(assignees, r.Name)
		})
		if len(assignees) > 0 {
			reqData.SuggestedReviewers = assignees
		}
		if err = a.createIncident(ctx, reqID, reqData); err != nil {
			// Even if we failed to create the incident we try to auto-approve
			return trace.NewAggregate(
				trace.WrapWithMessage(err, "creating ServiceNow incident"),
				trace.Wrap(a.tryApproveRequest(ctx, req)),
			)
		}
	}
	if reqReviews := req.GetReviews(); len(reqReviews) > 0 {
		if err = a.postReviewNotes(ctx, reqID, reqReviews); err != nil {
			return trace.NewAggregate(
				trace.WrapWithMessage(err, "posting review notes"),
				trace.Wrap(a.tryApproveRequest(ctx, req)),
			)
		}
	}
	// To minimize the count of auto-approval tries, let's only attempt it only when we have just created an incident.
	if !isNew {
		return nil
	}
	// Try to approve the request if user is currently on-call.
	return trace.Wrap(a.tryApproveRequest(ctx, req))
}

func (a *App) onResolvedRequest(ctx context.Context, req types.AccessRequest) error {
	var notifyErr error
	if err := a.postReviewNotes(ctx, req.GetName(), req.GetReviews()); err != nil {
		notifyErr = trace.Wrap(err)
	}

	resolution := Resolution{Reason: req.GetResolveReason()}

	var state string

	switch req.GetState() {
	case types.RequestState_APPROVED:
		state = ResolutionStateResolved
	case types.RequestState_DENIED:
		state = ResolutionStateClosed
	default:
		return trace.BadParameter("onResolvedRequest called with non resolved request")
	}
	resolution.State = state

	err := trace.Wrap(a.resolveIncident(ctx, req.GetName(), resolution))
	return trace.NewAggregate(notifyErr, err)
}

func (a *App) onDeletedRequest(ctx context.Context, reqID string) error {
	return a.resolveIncident(ctx, reqID, Resolution{State: ResolutionStateResolved})
}

func (a *App) getOnCallServiceNames(req types.AccessRequest) ([]string, error) {
	annotationKey := types.TeleportNamespace + types.ReqAnnotationApproveSchedulesLabel
	return common.GetNamesFromAnnotations(req, annotationKey)
}

// createIncident posts an incident with request information.
func (a *App) createIncident(ctx context.Context, reqID string, reqData RequestData) error {
	data, err := a.serviceNow.CreateIncident(ctx, reqID, reqData)
	if err != nil {
		return trace.Wrap(err)
	}
	ctx, log := logger.With(ctx, "servicenow_incident_id", data.IncidentID)
	log.InfoContext(ctx, "Successfully created Servicenow incident")

	// Save servicenow incident info in plugin data.
	_, err = a.modifyPluginData(ctx, reqID, func(existing *PluginData) (PluginData, bool) {
		var pluginData PluginData
		if existing != nil {
			pluginData = *existing
		} else {
			// It must be impossible but lets handle it just in case.
			pluginData = PluginData{RequestData: reqData}
		}
		pluginData.ServiceNowData = ServiceNowData{IncidentID: data.IncidentID}
		return pluginData, true
	})
	return trace.Wrap(err)
}

// postReviewNotes posts incident notes about new reviews appeared for request.
func (a *App) postReviewNotes(ctx context.Context, reqID string, reqReviews []types.AccessReview) error {
	var oldCount int
	var data ServiceNowData

	// Increase the review counter in plugin data.
	ok, err := a.modifyPluginData(ctx, reqID, func(existing *PluginData) (PluginData, bool) {
		if existing == nil {
			return PluginData{}, false
		}

		if data = existing.ServiceNowData; data.IncidentID == "" {
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
		logger.Get(ctx).DebugContext(ctx, "Failed to post the note: plugin data is missing")
		return nil
	}
	ctx, _ = logger.With(ctx, "servicenow_incident_id", data.IncidentID)

	slice := reqReviews[oldCount:]
	if len(slice) == 0 {
		return nil
	}

	errors := make([]error, 0, len(slice))
	for _, review := range slice {
		if err := a.serviceNow.PostReviewNote(ctx, data.IncidentID, review); err != nil {
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
		logger.Get(ctx).DebugContext(ctx, "Skipping the approval", "error", err)
		return nil
	}
	log.DebugContext(ctx, "Checking the shifts to see if the requester is on-call", "shifts", serviceNames)

	onCallUsers, err := a.getOnCallUsers(ctx, serviceNames)
	if err != nil {
		return trace.Wrap(err)
	}
	log.DebugContext(ctx, "Users on-call are", "on_call_users", onCallUsers)

	if userIsOnCall := slices.Contains(onCallUsers, req.GetUser()); !userIsOnCall {
		log.DebugContext(ctx, "User is not on-call, not approving the request",
			"user", req.GetUser(),
			"request", req.GetName(),
		)
		return nil
	}
	log.DebugContext(ctx, "User is on-call, auto-approving the request",
		"user", req.GetUser(),
		"request", req.GetName(),
	)
	if _, err := a.teleport.SubmitAccessReview(ctx, types.AccessReviewSubmission{
		RequestID: req.GetName(),
		Review: types.AccessReview{
			Author:        a.conf.TeleportUser,
			ProposedState: types.RequestState_APPROVED,
			Reason: fmt.Sprintf("Access requested by user %s who is on call on service(s) %s",
				tp.SystemAccessApproverUserName,
				strings.Join(serviceNames, ","),
			),
			Created: time.Now(),
		},
	}); err != nil {
		if strings.HasSuffix(err.Error(), "has already reviewed this request") {
			log.DebugContext(ctx, "Already reviewed the request")
			return nil
		}
		return trace.Wrap(err, "submitting access request")
	}
	log.InfoContext(ctx, "Successfully submitted a request approval")
	return nil
}

func (a *App) getOnCallUsers(ctx context.Context, serviceNames []string) ([]string, error) {
	log := logger.Get(ctx)
	onCallUsers := []string{}
	for _, scheduleName := range serviceNames {
		respondersResult, err := a.serviceNow.GetOnCall(ctx, scheduleName)
		if err != nil {
			if trace.IsNotFound(err) {
				log.ErrorContext(ctx, "Failed to retrieve responder from schedule", "error", err)
				continue
			}
			return nil, trace.Wrap(err)
		}
		onCallUsers = append(onCallUsers, respondersResult...)
	}
	return onCallUsers, nil
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

		// If state field is not empty then we already resolved the incident before. In this case we just quit.
		if existing.RequestData.Resolution.State != "" {
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
		logger.Get(ctx).DebugContext(ctx, "Failed to resolve the incident: plugin data is missing")
		return nil
	}

	ctx, log := logger.With(ctx, "servicenow_incident_id", incidentID)
	if err := a.serviceNow.ResolveIncident(ctx, incidentID, resolution); err != nil {
		return trace.Wrap(err)
	}
	log.InfoContext(ctx, "Successfully resolved the incident")

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

// getResourceNames returns the names of the requested resources.
func (a *App) getResourceNames(ctx context.Context, req types.AccessRequest) ([]string, error) {
	resourceNames := make([]string, 0, len(req.GetRequestedResourceIDs()))
	resourcesByCluster := accessrequest.GetResourceIDsByCluster(req)

	for cluster, resources := range resourcesByCluster {
		resourceDetails, err := accessrequest.GetResourceDetails(ctx, cluster, a.teleport, resources)
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
