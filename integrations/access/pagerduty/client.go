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
	"github.com/gravitational/teleport/integrations/lib/stringset"
)

const (
	pdMaxConns    = 100
	pdHTTPTimeout = 10 * time.Second
	pdListLimit   = uint(100)

	PdIncidentKeyPrefix = "teleport-access-request"

	pdStatusUpdateTimeout time.Duration = 10 * time.Second
)

var incidentBodyTemplate = template.Must(template.New("incident body").Parse(
	`{{.User}} requested permissions for roles {{range $index, $element := .Roles}}{{if $index}}, {{end}}{{ . }}{{end}} on Teleport at {{.Created.Format .TimeFormat}}.
{{if .RequestReason}}Reason: {{.RequestReason}}{{end}}
{{if .RequestLink}}To approve or deny the request, proceed to {{.RequestLink}}{{end}}
`,
))
var reviewNoteTemplate = template.Must(template.New("review note").Parse(
	`{{.Author}} reviewed the request at {{.Created.Format .TimeFormat}}.
Resolution: {{.ProposedState}}.
{{if .Reason}}Reason: {{.Reason}}.{{end}}`,
))
var resolutionNoteTemplate = template.Must(template.New("resolution note").Parse(
	`Access request has been {{.Resolution}}
{{if .ResolveReason}}Reason: {{.ResolveReason}}{{end}}`,
))

// Pagerduty is a wrapper around resty.Client.
type Pagerduty struct {
	client *resty.Client
	from   string

	clusterName string
	webProxyURL *url.URL
}

func NewPagerdutyClient(conf PagerdutyConfig, clusterName, webProxyAddr string, statusSink common.StatusSink) (Pagerduty, error) {
	var (
		webProxyURL *url.URL
		err         error
	)
	if webProxyAddr != "" {
		if webProxyURL, err = lib.AddrToURL(webProxyAddr); err != nil {
			return Pagerduty{}, trace.Wrap(err)
		}
	}

	client := resty.NewWithClient(&http.Client{
		Timeout: pdHTTPTimeout,
		Transport: &http.Transport{
			MaxConnsPerHost:     pdMaxConns,
			MaxIdleConnsPerHost: pdMaxConns,
			Proxy:               http.ProxyFromEnvironment,
		}}).
		SetBaseURL(conf.APIEndpoint).
		SetHeader("Accept", "application/vnd.pagerduty+json;version=2").
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", "Token token="+conf.APIKey).
		OnBeforeRequest(func(_ *resty.Client, req *resty.Request) error {
			req.SetError(&ErrorResult{})
			return nil
		})

	if statusSink != nil {
		client.OnAfterResponse(onAfterPagerDutyResponse(statusSink))
	}

	return Pagerduty{
		client:      client,
		clusterName: clusterName,
		webProxyURL: webProxyURL,
		from:        conf.UserEmail,
	}, nil
}

func onAfterPagerDutyResponse(sink common.StatusSink) resty.ResponseMiddleware {
	return func(_ *resty.Client, resp *resty.Response) error {
		log := logger.Get(resp.Request.Context())
		status := common.StatusFromStatusCode(resp.StatusCode())

		// No usable context in scope, use background with a reasonable timeout
		ctx, cancel := context.WithTimeout(context.Background(), pdStatusUpdateTimeout)
		defer cancel()

		if err := sink.Emit(ctx, status); err != nil {
			log.ErrorContext(ctx, "Error while emitting PagerDuty plugin status", "error", err)
		}

		errorFn := trace.Errorf
		if status.GetCode() == types.PluginStatusCode_UNAUTHORIZED {
			errorFn = func(msg string, args ...any) error {
				return trace.AccessDenied(msg, args...)
			}
		}

		if resp.IsError() {
			switch result := resp.Error().(type) {
			case *ErrorResult:
				// Do we have a formatted PagerDuty API error response? We set
				// an empty `ErrorResult` in the pre-request hook, and if the
				// HTTP server returns an error, the `resty` middleware will
				// attempt to unmarshal the error response into it.
				return errorFn("http error code=%v, err_code=%v, message=%v, errors=[%v]", resp.StatusCode(), result.Code, result.Message, strings.Join(result.Errors, ", "))
			default:
				return errorFn("unknown error result %#v", result)
			}
		}
		return nil
	}
}

func (p Pagerduty) HealthCheck(ctx context.Context) error {
	var result ListAbilitiesResult
	_, err := p.client.NewRequest().
		SetContext(ctx).
		SetResult(&result).
		Get("abilities")
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// CreateIncident creates a notification incident.
func (p Pagerduty) CreateIncident(ctx context.Context, serviceID, reqID string, reqData RequestData) (PagerdutyData, error) {
	bodyDetails, err := p.buildIncidentBody(reqID, reqData)
	if err != nil {
		return PagerdutyData{}, trace.Wrap(err)
	}
	body := IncidentBody{
		Title:       fmt.Sprintf("Access request from %s", reqData.User),
		IncidentKey: fmt.Sprintf("%s/%s", PdIncidentKeyPrefix, reqID),
		Service: Reference{
			Type: "service_reference",
			ID:   serviceID,
		},
		Body: Details{
			Type:    "incident_body",
			Details: bodyDetails,
		},
	}
	var result IncidentResult
	_, err = p.client.NewRequest().
		SetContext(ctx).
		SetHeader("From", p.from).
		SetBody(IncidentBodyWrap{body}).
		SetResult(&result).
		Post("incidents")
	if err != nil {
		return PagerdutyData{}, trace.Wrap(err)
	}

	return PagerdutyData{
		ServiceID:  serviceID,
		IncidentID: result.Incident.ID,
	}, nil
}

// PostReviewNote posts a note once a new request review appears.
func (p Pagerduty) PostReviewNote(ctx context.Context, incidentID string, review types.AccessReview) error {
	noteContent, err := p.buildReviewNoteBody(review)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = p.client.NewRequest().
		SetContext(ctx).
		SetHeader("From", p.from).
		SetBody(IncidentNoteBodyWrap{IncidentNoteBody{Content: noteContent}}).
		SetPathParams(map[string]string{"incidentID": incidentID}).
		Post("incidents/{incidentID}/notes")
	return trace.Wrap(err)
}

// ResolveIncident resolves an incident and posts a note with resolution details.
func (p Pagerduty) ResolveIncident(ctx context.Context, incidentID string, resolution Resolution) error {
	noteContent, err := p.buildResolutionNoteBody(resolution)
	if err != nil {
		return trace.Wrap(err)
	}

	pathParams := map[string]string{"incidentID": incidentID}

	_, err = p.client.NewRequest().
		SetContext(ctx).
		SetHeader("From", p.from).
		SetBody(IncidentNoteBodyWrap{IncidentNoteBody{Content: noteContent}}).
		SetPathParams(pathParams).
		Post("incidents/{incidentID}/notes")
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = p.client.NewRequest().
		SetContext(ctx).
		SetHeader("From", p.from).
		SetBody(IncidentBodyWrap{IncidentBody{
			Type:   "incident_reference",
			Status: "resolved",
		}}).
		SetPathParams(pathParams).
		Put("incidents/{incidentID}")
	return trace.Wrap(err)
}

// GetUserInfo loads a user profile by id.
func (p Pagerduty) GetUserInfo(ctx context.Context, userID string) (User, error) {
	var result UserResult

	_, err := p.client.NewRequest().
		SetContext(ctx).
		SetResult(&result).
		SetPathParams(map[string]string{"userID": userID}).
		Get("users/{userID}")
	if err != nil {
		return User{}, trace.Wrap(err)
	}

	return result.User, nil
}

// GetUserByEmail finds a user by email.
func (p *Pagerduty) FindUserByEmail(ctx context.Context, userEmail string) (User, error) {
	userEmail = strings.ToLower(userEmail)
	usersQuery, err := query.Values(ListUsersQuery{
		Query: userEmail,
		PaginationQuery: PaginationQuery{
			Limit: pdListLimit,
		},
	})
	if err != nil {
		return User{}, trace.Wrap(err)
	}

	var result ListUsersResult
	_, err = p.client.NewRequest().
		SetContext(ctx).
		SetQueryParamsFromValues(usersQuery).
		SetResult(&result).
		Get("users")
	if err != nil {
		return User{}, trace.Wrap(err)
	}

	for _, user := range result.Users {
		if strings.ToLower(user.Email) == userEmail {
			return user, nil
		}
	}

	if len(result.Users) > 0 && result.More {
		logger.Get(ctx).WarnContext(ctx, "PagerDuty returned too many results when querying user email", "email", userEmail)
	}

	return User{}, trace.NotFound("failed to find pagerduty user by email %s", userEmail)
}

// FindServiceByName finds a service by its name (case-insensitive).
func (p *Pagerduty) FindServiceByName(ctx context.Context, serviceName string) (Service, error) {
	// In PagerDuty service names are unique and in fact case-insensitive.
	serviceName = strings.ToLower(serviceName)
	servicesQuery, err := query.Values(ListServicesQuery{Query: serviceName})
	if err != nil {
		return Service{}, trace.Wrap(err)
	}
	var result ListServicesResult
	_, err = p.client.NewRequest().
		SetContext(ctx).
		SetQueryParamsFromValues(servicesQuery).
		SetResult(&result).
		Get("services")
	if err != nil {
		return Service{}, trace.Wrap(err)
	}

	for _, service := range result.Services {
		if strings.ToLower(service.Name) == serviceName {
			return service, nil
		}
	}

	return Service{}, trace.NotFound("failed to find pagerduty service by name %s", serviceName)
}

// FindServicesByNames finds a bunch of services by its names making a query for each service.
func (p Pagerduty) FindServicesByNames(ctx context.Context, serviceNames []string) ([]Service, error) {
	services := make([]Service, 0, len(serviceNames))
	for _, serviceName := range serviceNames {
		service, err := p.FindServiceByName(ctx, serviceName)
		if err != nil {
			if trace.IsNotFound(err) {
				continue
			}
			return nil, trace.Wrap(err)
		}
		services = append(services, service)
	}
	return services, nil
}

// RangeOnCallPolicies iterates over the escalation policy IDs for which a given user is currently on-call.
func (p *Pagerduty) FilterOnCallPolicies(ctx context.Context, userID string, escalationPolicyIDs []string) ([]string, error) {
	policyIDSet := stringset.New(escalationPolicyIDs...)
	filteredIDSet := stringset.New()

	var offset uint
	more := true
	anyData := false
	for more {
		query, err := query.Values(ListOnCallsQuery{
			PaginationQuery:     PaginationQuery{Offset: offset},
			UserIDs:             []string{userID},
			EscalationPolicyIDs: escalationPolicyIDs,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		var result ListOnCallsResult

		_, err = p.client.NewRequest().
			SetContext(ctx).
			SetQueryParamsFromValues(query).
			SetResult(&result).
			Get("oncalls")
		if err != nil {
			return nil, trace.Wrap(err)
		}
		more = result.More
		offset += uint(len(result.OnCalls))
		anyData = anyData || len(result.OnCalls) > 0

		for _, onCall := range result.OnCalls {
			if onCall.User.Type != "user_reference" || onCall.User.ID != userID {
				continue
			}

			id := onCall.EscalationPolicy.ID
			if policyIDSet.Contains(id) {
				filteredIDSet.Add(id)
			}
		}

		if filteredIDSet.Len() == policyIDSet.Len() {
			more = false
		}
	}

	if len(filteredIDSet) == 0 {
		if anyData {
			logger.Get(ctx).WarnContext(ctx, "PagerDuty returned some oncalls array but none of them matched the query",
				"pd_user_id", userID,
				"pd_escalation_policy_ids", escalationPolicyIDs,
			)
		}

		return nil, nil
	}

	return filteredIDSet.ToSlice(), nil
}

func (p Pagerduty) buildIncidentBody(reqID string, reqData RequestData) (string, error) {
	var requestLink string
	if p.webProxyURL != nil {
		reqURL := *p.webProxyURL
		reqURL.Path = lib.BuildURLPath("web", "requests", reqID)
		requestLink = reqURL.String()
	}

	var builder strings.Builder
	err := incidentBodyTemplate.Execute(&builder, struct {
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

func (p Pagerduty) buildReviewNoteBody(review types.AccessReview) (string, error) {
	var builder strings.Builder
	err := reviewNoteTemplate.Execute(&builder, struct {
		types.AccessReview
		ProposedState string
		TimeFormat    string
	}{
		review,
		review.ProposedState.String(),
		time.RFC822,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}
	return builder.String(), nil
}

func (p Pagerduty) buildResolutionNoteBody(resolution Resolution) (string, error) {
	var builder strings.Builder
	err := resolutionNoteTemplate.Execute(&builder, struct {
		Resolution    string
		ResolveReason string
	}{
		string(resolution.Tag),
		resolution.Reason,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}
	return builder.String(), nil
}
