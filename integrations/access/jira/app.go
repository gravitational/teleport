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

package jira

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/retryutils"
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
	// minServerVersion is the minimal teleport version the plugin supports.
	minServerVersion = "6.1.0"
	// pluginName is used to tag PluginData and as a Delegator in Audit log.
	pluginName = "jira"
	// initTimeout is used to bound execution time of health check and teleport version check.
	initTimeout = time.Second * 10
	// handlerTimeout is used to bound the execution time of watcher event handler.
	handlerTimeout = time.Second * 5
	// modifyPluginDataBackoffBase is an initial (minimum) backoff value.
	modifyPluginDataBackoffBase = time.Millisecond
	// modifyPluginDataBackoffMax is a backoff threshold
	modifyPluginDataBackoffMax = time.Second
	// webhookIssueAPIRetryInterval is the time the plugin will wait before grabbing,
	// the jira issue again if the webhook payload and jira API disagree on the issue status.
	webhookIssueAPIRetryInterval = 5 * time.Second
	// webhookIssueAPIRetryTimeout the timeout for retrying check that webhook payload matches issue API response.
	webhookIssueAPIRetryTimeout = 5 * time.Minute
)

var resolveReasonInlineRegex = regexp.MustCompile(`(?im)^ *(resolution|reason) *: *(.+)$`)
var resolveReasonSeparatorRegex = regexp.MustCompile(`(?im)^ *(resolution|reason) *: *$`)

// App contains global application state.
type App struct {
	conf Config

	teleport    teleport.Client
	jira        *Jira
	webhookSrv  *WebhookServer
	mainJob     lib.ServiceJob
	statusSink  common.StatusSink
	retryConfig retryutils.LinearConfig

	*lib.Process
}

func NewApp(conf Config) (*App, error) {
	retryConfig := retryutils.LinearConfig{
		Step: webhookIssueAPIRetryInterval,
		Max:  webhookIssueAPIRetryTimeout,
	}
	app := &App{
		conf:        conf,
		teleport:    conf.Client,
		statusSink:  conf.StatusSink,
		retryConfig: retryConfig,
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
	return trace.Wrap(a.mainJob.Err())
}

// Err returns the error app finished with.
func (a *App) Err() error {
	return trace.Wrap(a.mainJob.Err())
}

// WaitReady waits for http and watcher service to start up.
func (a *App) WaitReady(ctx context.Context) (bool, error) {
	return a.mainJob.WaitReady(ctx)
}

// PublicURL returns a webhook base URL.
func (a *App) PublicURL() *url.URL {
	if !a.mainJob.IsReady() {
		panic("app is not running")
	}
	return a.webhookSrv.BaseURL()
}

func (a *App) run(ctx context.Context) error {
	var err error

	log := logger.Get(ctx)

	if err = a.init(ctx); err != nil {
		return trace.Wrap(err)
	}

	var httpOk bool
	var httpJob lib.ServiceJob
	var httpErr error

	if a.webhookSrv != nil {
		httpJob = a.webhookSrv.ServiceJob()
		a.SpawnCriticalJob(httpJob)
		httpOk, err = httpJob.WaitReady(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
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
	watcherOk, err := watcherJob.WaitReady(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	ok := (a.webhookSrv == nil || httpOk) && watcherOk
	a.mainJob.SetReady(ok)
	if ok {
		log.InfoContext(ctx, "Plugin is ready")
	} else {
		log.ErrorContext(ctx, "Plugin is not ready")
	}

	if httpJob != nil {
		<-httpJob.Done()
		httpErr = httpJob.Err()
	}
	<-watcherJob.Done()

	return trace.NewAggregate(httpErr, watcherJob.Err())
}

func (a *App) init(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, initTimeout)
	defer cancel()
	log := logger.Get(ctx)

	var err error
	if a.teleport == nil {
		if a.teleport, err = common.GetTeleportClient(ctx, a.conf.Teleport); err != nil {
			return trace.Wrap(err)
		}
	}

	pong, err := a.checkTeleportVersion(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	var teleportProxyAddr string
	if pong.ServerFeatures.AdvancedAccessWorkflows {
		teleportProxyAddr = pong.ProxyPublicAddr
	}

	a.jira, err = NewJiraClient(a.conf.Jira, pong.ClusterName, teleportProxyAddr, a.statusSink)
	if err != nil {
		return trace.Wrap(err)
	}

	log.DebugContext(ctx, "Starting Jira API health check")
	if err = a.jira.HealthCheck(ctx); err != nil {
		return trace.Wrap(err, "api health check failed")
	}
	log.DebugContext(ctx, "Jira API health check finished ok")

	if !a.conf.DisableWebhook {
		webhookSrv, err := NewWebhookServer(a.conf.HTTP, a.onJiraWebhook)
		if err != nil {
			return trace.Wrap(err)
		}
		if err = webhookSrv.EnsureCert(); err != nil {
			return trace.Wrap(err)
		}
		a.webhookSrv = webhookSrv
	}

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

func (a *App) onWatcherEvent(ctx context.Context, event types.Event) error {
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
		log.DebugContext(ctx, "Processing watcher event")

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

// onJiraWebhook processes Jira webhook and updates the status of an issue
func (a *App) onJiraWebhook(_ context.Context, webhook Webhook) error {
	ctx, cancel := context.WithTimeout(context.Background(), webhookIssueAPIRetryTimeout)
	defer cancel()

	webhookEvent := webhook.WebhookEvent
	issueEventTypeName := webhook.IssueEventTypeName
	if webhookEvent != "jira:issue_updated" || issueEventTypeName != "issue_generic" {
		return nil
	}

	ctx, log := logger.With(ctx, "jira_issue_id", webhook.Issue.ID)
	log.DebugContext(ctx, "Processing incoming webhook event",
		"event", webhookEvent,
		"event_type", issueEventTypeName,
	)

	if webhook.Issue == nil {
		return trace.Errorf("got webhook without issue info")
	}

	retry, err := retryutils.NewLinear(a.retryConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	var issue Issue
	var statusName string
	webHookStatusName := strings.ToLower(webhook.Issue.Fields.Status.Name)
	// Retry until API syncs up with webhook payload.
	err = retry.For(ctx, func() error {
		issue, err = a.jira.GetIssue(ctx, webhook.Issue.ID)
		if err != nil {
			return trace.Wrap(err)
		}
		statusName = strings.ToLower(issue.Fields.Status.Name)
		if webHookStatusName != statusName {
			return trace.CompareFailed("mismatch of webhook payload and issue API response: %q and %q",
				webHookStatusName, statusName)
		}
		return nil
	})
	if err != nil {
		if statusName == "" {
			return trace.Errorf("getting Jira issue status: %w", err)
		}
		log.WarnContext(ctx, "Using most recent successful getIssue response", "error", err)
	}

	ctx, log = logger.With(ctx,
		"jira_issue_id", issue.ID,
		"jira_issue_key", issue.Key,
	)

	switch {
	case statusName == "pending":
		log.DebugContext(ctx, "Issue has pending status, ignoring it")
		return nil
	case statusName == "expired":
		log.DebugContext(ctx, "Issue has expired status, ignoring it")
		return nil
	case statusName != "approved" && statusName != "denied":
		return trace.BadParameter("unknown Jira status %s", statusName)
	}

	reqID, err := GetRequestID(issue)
	if err != nil {
		return trace.Wrap(err)
	}
	if reqID == "" {
		log.DebugContext(ctx, "Missing teleportAccessRequestId issue property")
		return nil
	}

	ctx, log = logger.With(ctx, "request_id", reqID)

	reqs, err := a.teleport.GetAccessRequests(ctx, types.AccessRequestFilter{ID: reqID})
	if err != nil {
		return trace.Wrap(err)
	}

	var req types.AccessRequest
	if len(reqs) > 0 {
		req = reqs[0]
	}

	// Validate plugin data that it's matching with the webhook information
	pluginData, err := a.getPluginData(ctx, reqID)
	if err != nil {
		return trace.Wrap(err)
	}
	if pluginData.IssueID == "" {
		return trace.Errorf("plugin data is blank")
	}
	if pluginData.IssueID != issue.ID {
		log.DebugContext(ctx, "plugin_data.issue_id does not match issue.id",
			"plugin_data_issue_id", pluginData.IssueID,
		)
		return trace.Errorf("issue_id from request's plugin_data does not match")
	}

	if req == nil {
		return trace.Wrap(a.resolveIssue(ctx, reqID, Resolution{Tag: ResolvedExpired}))
	}

	var resolution Resolution
	state := req.GetState()
	switch {
	case state.IsPending():
		switch statusName {
		case "approved":
			resolution.Tag = ResolvedApproved
		case "denied":
			resolution.Tag = ResolvedDenied
		default:
			return trace.BadParameter("unknown status: %v", statusName)
		}

		author, reason, err := a.loadResolutionInfo(ctx, issue, statusName)
		if err != nil {
			log.ErrorContext(ctx, "Failed to load resolution info from the issue history", "error", err)
		}
		resolution.Reason = reason

		ctx, _ = logger.With(ctx,
			"jira_user_email", author.EmailAddress,
			"jira_user_name", author.DisplayName,
			"request_user", req.GetUser(),
			"request_roles", req.GetRoles(),
			"reason", reason,
		)
		if err := a.resolveRequest(ctx, reqID, author.EmailAddress, resolution); err != nil {
			return trace.Wrap(err)
		}
	case state.IsApproved():
		resolution.Tag = ResolvedApproved
	case state.IsDenied():
		resolution.Tag = ResolvedDenied
	case state.IsPromoted():
		resolution.Tag = ResolvedPromoted
	default:
		return trace.BadParameter("unknown request state %v (%s)", state, state)
	}

	return trace.Wrap(a.resolveIssue(ctx, reqID, resolution))
}

func (a *App) onPendingRequest(ctx context.Context, req types.AccessRequest) error {
	reqID := req.GetName()
	reqData := RequestData{
		User:          req.GetUser(),
		Roles:         req.GetRoles(),
		RequestReason: req.GetRequestReason(),
		Created:       req.GetCreationTime(),
	}

	// Create plugin data if it didn't exist before.
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
		if err := a.createIssue(ctx, reqID, reqData); err != nil {
			return trace.Wrap(err)
		}
	}

	if reqReviews := req.GetReviews(); len(reqReviews) > 0 {
		if err = a.addReviewComments(ctx, reqID, reqReviews); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func (a *App) onResolvedRequest(ctx context.Context, req types.AccessRequest) error {
	var commentErr error
	if err := a.addReviewComments(ctx, req.GetName(), req.GetReviews()); err != nil {
		commentErr = trace.Wrap(err)
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
	err := trace.Wrap(a.resolveIssue(ctx, req.GetName(), resolution))
	return trace.NewAggregate(commentErr, err)
}

func (a *App) onDeletedRequest(ctx context.Context, reqID string) error {
	return a.resolveIssue(ctx, reqID, Resolution{Tag: ResolvedExpired})
}

// createIssue posts a Jira issue with request information.
func (a *App) createIssue(ctx context.Context, reqID string, reqData RequestData) error {
	data, err := a.jira.CreateIssue(ctx, reqID, reqData)
	if err != nil {
		return trace.Wrap(err)
	}

	ctx, log := logger.With(ctx,
		"jira_issue_id", data.IssueID,
		"jira_issue_key", data.IssueKey,
	)
	log.InfoContext(ctx, "Jira Issue created")

	// Save jira issue info in plugin data.
	_, err = a.modifyPluginData(ctx, reqID, func(existing *PluginData) (PluginData, bool) {
		var pluginData PluginData
		if existing != nil {
			pluginData = *existing
		} else {
			// It must be impossible but lets handle it just in case.
			pluginData = PluginData{RequestData: reqData}
		}
		pluginData.JiraData = data
		return pluginData, true
	})
	return trace.Wrap(err)
}

// addReviewComments posts issue comments about new reviews appeared for request.
func (a *App) addReviewComments(ctx context.Context, reqID string, reqReviews []types.AccessReview) error {
	var oldCount int
	var issueID string

	// Increase the review counter in plugin data.
	ok, err := a.modifyPluginData(ctx, reqID, func(existing *PluginData) (PluginData, bool) {
		// If plugin data is empty or missing issueID, we cannot do anything.
		if existing == nil {
			issueID = ""
			return PluginData{}, false
		}

		issueID = existing.IssueID
		// If plugin data has blank issue identification info, we cannot do anything.
		if issueID == "" {
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
		if issueID == "" {
			logger.Get(ctx).DebugContext(ctx, "Failed to add the comment: plugin data is blank")
		}
		return nil
	}
	ctx, _ = logger.With(ctx, "jira_issue_id", issueID)

	slice := reqReviews[oldCount:]
	if len(slice) == 0 {
		return nil
	}

	errors := make([]error, 0, len(slice))
	for _, review := range slice {
		if err := a.jira.AddIssueReviewComment(ctx, issueID, review); err != nil {
			errors = append(errors, err)
		}
	}
	return trace.NewAggregate(errors...)
}

// loadResolutionInfo loads a user email of who has transitioned the issue and the contents of resolution comment.
func (a *App) loadResolutionInfo(ctx context.Context, issue Issue, statusName string) (UserDetails, string, error) {
	issueUpdate, err := GetLastUpdate(issue, statusName)
	if err != nil {
		return UserDetails{}, "", trace.Wrap(err, "failed to determine who updated the issue status")
	}
	author := issueUpdate.Author
	accountID := author.AccountID
	var reason string
	err = a.jira.RangeIssueCommentsDescending(ctx, issue.ID, func(page PageOfComments) bool {
		for _, comment := range page.Comments {
			if comment.Author.AccountID != accountID {
				continue
			}
			contents := comment.Body
			if submatch := resolveReasonInlineRegex.FindStringSubmatch(contents); len(submatch) > 0 {
				reason = strings.Trim(submatch[2], " \n")
				return false
			} else if locs := resolveReasonSeparatorRegex.FindStringIndex(contents); len(locs) > 0 {
				reason = strings.TrimLeft(contents[locs[1]:], "\n")
				return false
			}
		}
		return true
	})
	if err != nil {
		return UserDetails{}, "", trace.Wrap(err, "failed to load issue comments")
	}
	return author, reason, nil
}

// resolveRequest sets an access request state.
func (a *App) resolveRequest(ctx context.Context, reqID string, userEmail string, resolution Resolution) error {
	params := types.AccessRequestUpdate{RequestID: reqID, Reason: resolution.Reason}

	switch resolution.Tag {
	case ResolvedApproved:
		params.State = types.RequestState_APPROVED
	case ResolvedDenied:
		params.State = types.RequestState_DENIED
	default:
		return trace.BadParameter("unknown resolution tag %v", resolution.Tag)
	}

	delegator := fmt.Sprintf("%s:%s", pluginName, userEmail)

	if err := a.teleport.SetAccessRequestState(apiutils.WithDelegator(ctx, delegator), params); err != nil {
		return trace.Wrap(err)
	}

	logger.Get(ctx).InfoContext(ctx, "Jira user processed the request", "resolution", resolution.Tag)
	return nil
}

// resolveIssue transitions the issue to some final state.
func (a *App) resolveIssue(ctx context.Context, reqID string, resolution Resolution) error {
	var issueID string

	// Save request resolution info in plugin data.
	ok, err := a.modifyPluginData(ctx, reqID, func(existing *PluginData) (PluginData, bool) {
		// If plugin data is empty or missing issueID, we cannot do anything.
		if existing == nil {
			issueID = ""
			return PluginData{}, false
		}

		issueID = existing.IssueID
		// If plugin data has blank issue identification info, we cannot do anything.
		if issueID == "" {
			return PluginData{}, false
		}

		// If resolution field is not empty then we already resolved the issue before. In this case we just quit.
		if existing.RequestData.Resolution.Tag != Unresolved {
			return PluginData{}, false
		}

		// Mark issue as resolved.
		pluginData := *existing
		pluginData.Resolution = resolution
		return pluginData, true
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if !ok {
		if issueID == "" {
			logger.Get(ctx).DebugContext(ctx, "Failed to resolve the issue: plugin data is blank")
		}

		// Either plugin data is missing or issue is already resolved by us, just quit.
		return nil
	}

	ctx, log := logger.With(ctx, "jira_issue_id", issueID)
	if err := a.jira.ResolveIssue(ctx, issueID, resolution); err != nil {
		return trace.Wrap(err)
	}
	log.InfoContext(ctx, "Successfully resolved the issue")

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
