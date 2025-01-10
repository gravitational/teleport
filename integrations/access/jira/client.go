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
	"net/http"
	"net/url"
	"strings"
	"text/template"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/google/go-querystring/query"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
)

const (
	RequestIDPropertyKey = "teleportAccessRequestId"

	jiraMaxConns    = 100
	jiraHTTPTimeout = 10 * time.Second
	// JiraReasonLimit is the max size for a reason in jira.
	// Teleport has a 4096-character limit for the reason field, so we
	// truncate all reasons to a generous but conservative limit
	JiraReasonLimit = 3000

	jiraStatusUpdateTimeout time.Duration = 10 * time.Second
)

var jiraRequiredPermissions = []string{"BROWSE_PROJECTS", "CREATE_ISSUES", "TRANSITION_ISSUES", "ADD_COMMENTS"}

// Jira is a wrapper around resty.Client.
type Jira struct {
	client           *resty.Client
	project          string
	issueType        string
	clusterName      string
	teleportProxyURL *url.URL
}

var descriptionTemplate = template.Must(template.New("description").Parse(
	`User *{{.User}}* requested an access on *{{.Created.Format .TimeFormat}}* with the following roles:
{{range .Roles}}
* {{ . }}
{{end}}
{{if .RequestReason}}
Reason: *{{.RequestReason}}*
{{end}}
Request ID: *{{.ID}}*
{{if .RequestLink}}To approve or deny the request, proceed to {{.RequestLink}}{{end}}`,
))
var reviewCommentTemplate = template.Must(template.New("review comment").Parse(
	`*{{.Author}}* reviewed the request at *{{.Created.Format .TimeFormat}}*.
Resolution: *{{.ProposedState}}*.
{{if .Reason}}Reason: {{.Reason}}.{{end}}`,
))
var resolutionCommentTemplate = template.Must(template.New("resolution comment").Parse(
	`Access request has been {{.Resolution}}
{{if .ResolveReason}}Reason: {{.ResolveReason}}{{end}}`,
))

// NewJiraClient builds a new Jira client.
func NewJiraClient(conf JiraConfig, clusterName, teleportProxyAddr string, statusSink common.StatusSink) (*Jira, error) {
	var (
		teleportProxyURL *url.URL
		err              error
	)
	if teleportProxyAddr != "" {
		if teleportProxyURL, err = lib.AddrToURL(teleportProxyAddr); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	client := resty.NewWithClient(&http.Client{
		Timeout: jiraHTTPTimeout,
		Transport: &http.Transport{
			MaxConnsPerHost:     jiraMaxConns,
			MaxIdleConnsPerHost: jiraMaxConns,
			Proxy:               http.ProxyFromEnvironment,
		}}).
		SetBaseURL(conf.URL).
		SetBasicAuth(conf.Username, conf.APIToken).
		SetHeader("Content-Type", "application/json").
		OnBeforeRequest(
			func(_ *resty.Client, req *resty.Request) error {
				req.SetError(&ErrorResult{})
				return nil
			}).
		OnAfterResponse(
			func(_ *resty.Client, resp *resty.Response) error {
				log := logger.Get(resp.Request.Context())

				if statusSink != nil {
					status := statusFromStatusCode(resp.StatusCode())

					// No usable context in scope. We can't use the context from the Resty response,
					// as that could already be canceled, which would block us from emitting a status
					// update showing that the plugin is currently broken.
					//
					// Using the background context with a reasonable timeout seems the least-bad option.
					ctx, cancel := context.WithTimeout(context.Background(), jiraStatusUpdateTimeout)
					defer cancel()

					if err := statusSink.Emit(ctx, status); err != nil {
						log.ErrorContext(ctx, "Error while emitting Jira plugin status", "error", err)
					}
				}

				if resp.IsError() {
					switch result := resp.Error().(type) {
					case *ErrorResult:
						return trace.Errorf("http error code=%v, errors=[%s]", resp.StatusCode(), result)
					case nil:
						return nil
					default:
						return trace.Errorf("unknown error result %#v", result)
					}
				}
				return nil
			})

	return &Jira{
		client:           client,
		project:          conf.Project,
		issueType:        conf.IssueType,
		clusterName:      clusterName,
		teleportProxyURL: teleportProxyURL,
	}, nil
}

func statusFromStatusCode(httpCode int) types.PluginStatus {
	var code types.PluginStatusCode
	switch {
	case httpCode == http.StatusUnauthorized:
		code = types.PluginStatusCode_UNAUTHORIZED
	case httpCode >= 200 && httpCode < 400:
		code = types.PluginStatusCode_RUNNING
	default:
		code = types.PluginStatusCode_OTHER_ERROR
	}
	return &types.PluginStatusV1{Code: code}
}

// buildSummary creates the Issue's summary by using the user name and roles.
// No official docs seem to exist, but it's _known_ that summary field must be less than 255 chars:
// Eg https://community.atlassian.com/t5/Jira-questions/Summary-must-be-less-than-255-characters/qaq-p/989632
func buildSummary(reqData RequestData) string {
	summary := fmt.Sprintf("%s requested %s", reqData.User, strings.Join(reqData.Roles, ", "))
	if len(summary) <= 254 {
		return summary
	}
	return fmt.Sprintf("%s requested access to %d roles", reqData.User, len(reqData.Roles))
}

// HealthCheck checks Jira endpoint for validity and also checks the project permissions.
func (j *Jira) HealthCheck(ctx context.Context) error {
	log := logger.Get(ctx)
	var emptyError *ErrorResult
	resp, err := j.client.NewRequest().
		SetContext(ctx).
		SetError(emptyError).
		Get("rest/api/2/myself")
	if err != nil {
		return trace.Wrap(err)
	}
	if !strings.HasPrefix(resp.Header().Get("Content-Type"), "application/json") {
		return trace.AccessDenied("got non-json response from API endpoint, perhaps Jira URL is not configured well")
	}
	if resp.IsError() {
		if resp.StatusCode() == 404 {
			return trace.AccessDenied("got %s from API endpoint, perhaps Jira URL is not configured well", resp.Status())
		} else if resp.StatusCode() == 403 || resp.StatusCode() == 401 {
			return trace.AccessDenied("got %s from API endpoint, perhaps Jira credentials are not configured well", resp.Status())
		} else {
			return trace.AccessDenied("got %s from API endpoint", resp.Status())
		}
	}

	log.DebugContext(ctx, "Checking out Jira project")
	var project Project
	_, err = j.client.NewRequest().
		SetContext(ctx).
		SetPathParams(map[string]string{"projectID": j.project}).
		SetResult(&project).
		Get("rest/api/2/project/{projectID}")
	if err != nil {
		return trace.Wrap(err)
	}
	log.DebugContext(ctx, "Found Jira project",
		"project", project.Key,
		"project_name", project.Name,
	)

	log.DebugContext(ctx, "Checking out Jira project permissions")
	queryOptions, err := query.Values(GetMyPermissionsQueryOptions{
		ProjectKey:  j.project,
		Permissions: jiraRequiredPermissions,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	var permissions Permissions
	_, err = j.client.NewRequest().
		SetContext(ctx).
		SetQueryParamsFromValues(queryOptions).
		SetResult(&permissions).
		Get("rest/api/2/mypermissions")
	if err != nil {
		return trace.Wrap(err)
	}

	for _, key := range jiraRequiredPermissions {
		if !permissions.Permissions[key].HavePermission {
			return trace.AccessDenied("plugin jira user does not have %s permission", key)
		}
	}

	return nil
}

// CreateIssue creates an issue with "Pending" status
func (j *Jira) CreateIssue(ctx context.Context, reqID string, reqData RequestData) (JiraData, error) {
	reqData = truncateReasonFields(reqData)
	description, err := j.buildIssueDescription(reqID, reqData)
	if err != nil {
		return JiraData{}, trace.Wrap(err)
	}

	input := IssueInput{
		Properties: []EntityProperty{
			{
				Key:   RequestIDPropertyKey,
				Value: reqID,
			},
		},
		Fields: IssueFieldsInput{
			Type:        &IssueType{Name: j.issueType},
			Project:     &Project{Key: j.project},
			Summary:     buildSummary(reqData),
			Description: description,
		},
	}
	var issue CreatedIssue
	_, err = j.client.NewRequest().
		SetContext(ctx).
		SetBody(&input).
		SetResult(&issue).
		Post("rest/api/2/issue")
	if err != nil {
		return JiraData{}, trace.Wrap(err)
	}

	return JiraData{
		IssueID:  issue.ID,
		IssueKey: issue.Key,
	}, nil
}

func (j *Jira) buildIssueDescription(reqID string, reqData RequestData) (string, error) {
	reqData = truncateReasonFields(reqData)
	var requestLink string
	if j.teleportProxyURL != nil {
		reqURL := *j.teleportProxyURL
		reqURL.Path = lib.BuildURLPath("web", "requests", reqID)
		requestLink = reqURL.String()
	}

	var builder strings.Builder
	err := descriptionTemplate.Execute(&builder, struct {
		ID          string
		TimeFormat  string
		RequestLink string
		RequestData
	}{
		reqID,
		time.RFC822,
		requestLink,
		reqData,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}
	return builder.String(), nil
}

// GetIssue loads the issue with all necessary nested data.
func (j *Jira) GetIssue(ctx context.Context, id string) (Issue, error) {
	queryOptions, err := query.Values(GetIssueQueryOptions{
		Fields:     []string{"status", "comment"},
		Expand:     []string{"changelog", "transitions"},
		Properties: []string{RequestIDPropertyKey},
	})
	if err != nil {
		return Issue{}, trace.Wrap(err)
	}
	var jiraIssue Issue
	_, err = j.client.NewRequest().
		SetContext(ctx).
		SetPathParams(map[string]string{"issueID": id}).
		SetQueryParamsFromValues(queryOptions).
		SetResult(&jiraIssue).
		Get("rest/api/2/issue/{issueID}")
	if err != nil {
		return Issue{}, trace.Wrap(err)
	}

	return jiraIssue, nil
}

// AddIssueReviewComment posts an issue comment about access review added to a request.
func (j *Jira) AddIssueReviewComment(ctx context.Context, id string, review types.AccessReview) error {
	var builder strings.Builder
	err := reviewCommentTemplate.Execute(&builder, struct {
		types.AccessReview
		ProposedState string
		TimeFormat    string
	}{
		review,
		review.ProposedState.String(),
		time.RFC822,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = j.client.NewRequest().
		SetContext(ctx).
		SetPathParams(map[string]string{"issueID": id}).
		SetBody(CommentInput{Body: builder.String()}).
		Post("rest/api/2/issue/{issueID}/comment")
	return trace.Wrap(err)
}

// RangeIssueCommentsDescending iterates over pages of comments of an issue.
func (j *Jira) RangeIssueCommentsDescending(ctx context.Context, id string, fn func(PageOfComments) bool) error {
	startAt := 0
	for {
		queryOptions, err := query.Values(GetIssueCommentQueryOptions{
			StartAt: startAt,
			OrderBy: "-created",
		})
		if err != nil {
			return trace.Wrap(err)
		}

		var pageOfComments PageOfComments
		_, err = j.client.NewRequest().
			SetContext(ctx).
			SetPathParams(map[string]string{"issueID": id}).
			SetQueryParamsFromValues(queryOptions).
			SetResult(&pageOfComments).
			Get("rest/api/2/issue/{issueID}/comment")
		if err != nil {
			return trace.Wrap(err)
		}

		nComments := len(pageOfComments.Comments)

		if nComments == 0 {
			break
		}

		if !fn(pageOfComments) {
			break
		}

		if nComments < pageOfComments.MaxResults {
			break
		}

		startAt = startAt + nComments
	}

	return nil
}

// TransitionIssue moves an issue by transition ID.
func (j *Jira) TransitionIssue(ctx context.Context, issueID, transitionID string) error {
	payload := IssueTransitionInput{
		Transition: IssueTransition{
			ID: transitionID,
		},
	}
	_, err := j.client.NewRequest().
		SetContext(ctx).
		SetPathParams(map[string]string{"issueID": issueID}).
		SetBody(&payload).
		Post("rest/api/2/issue/{issueID}/transitions")
	return trace.Wrap(err)
}

// ResolveIssue sets a final status e.g. "approved", "denied" or "expired" to the issue and posts the comment.
func (j *Jira) ResolveIssue(ctx context.Context, issueID string, resolution Resolution) error {
	if resolution.Tag == Unresolved {
		return trace.BadParameter("resolution is empty")
	}
	issue, err := j.GetIssue(ctx, issueID)
	if err != nil {
		return trace.Wrap(err)
	}

	// Try to add a comment.
	err1 := trace.Wrap(j.AddResolutionComment(ctx, issue.ID, resolution))

	// Try to transition the issue.
	fromStatus, toStatus := strings.ToLower(issue.Fields.Status.Name), string(resolution.Tag)
	if fromStatus == toStatus {
		return trace.Wrap(err1)
	}
	transition, err2 := GetTransition(issue, toStatus)
	if err2 != nil {
		return trace.NewAggregate(err1, err2)
	}
	if err2 := trace.Wrap(j.TransitionIssue(ctx, issue.ID, transition.ID)); err2 != nil {
		return trace.NewAggregate(err1, err2)
	}
	logger.Get(ctx).DebugContext(ctx, "Successfully moved the issue to the target status", "target_status", toStatus)

	return trace.Wrap(err1)
}

// AddResolutionComment posts an issue comment about request resolution.
func (j *Jira) AddResolutionComment(ctx context.Context, id string, resolution Resolution) error {
	var builder strings.Builder
	err := resolutionCommentTemplate.Execute(&builder, struct {
		Resolution    string
		ResolveReason string
	}{
		string(resolution.Tag),
		resolution.Reason,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = j.client.NewRequest().
		SetContext(ctx).
		SetPathParams(map[string]string{"issueID": id}).
		SetBody(CommentInput{Body: builder.String()}).
		Post("rest/api/2/issue/{issueID}/comment")
	if err == nil {
		logger.Get(ctx).DebugContext(ctx, "Successfully added a resolution comment to the issue")
	}
	return trace.Wrap(err)
}

func truncateReasonFields(reqData RequestData) RequestData {
	if reqData.Resolution.Reason != "" && len(reqData.Resolution.Reason) > JiraReasonLimit {
		reqData.Resolution.Reason = reqData.Resolution.Reason[:JiraReasonLimit]
	}
	if reqData.RequestReason != "" && len(reqData.RequestReason) > JiraReasonLimit {
		reqData.RequestReason = reqData.RequestReason[:JiraReasonLimit]
	}
	return reqData
}
