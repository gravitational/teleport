/*
Copyright 2015-2023 Gravitational, Inc.

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

package opsgenie

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"text/template"
	"time"

	"github.com/aws/aws-sdk-go/aws/defaults"
	"github.com/go-resty/resty/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/lib"
)

const (
	// alertKeyPrefix is the prefix for Alert's alias field used when creating an Alert.
	alertKeyPrefix = "teleport-access-request"
)

var alertBodyTemplate = template.Must(template.New("alert body").Parse(
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

// Client is a wrapper around resty.Client.
type Client struct {
	ClientConfig

	client *resty.Client
}

// ClientConfig is the config for the opsgenie client.
type ClientConfig struct {
	// APIKey is the API key for Opsgenie
	APIKey string
	// APIEndpoint is the endpoint for the Opsgenie API
	// api url of the form https://api.opsgenie.com/v2/ with optional trailing '/'
	APIEndpoint string
	// DefaultSchedules are the default on-call schedules to check for auto approval
	DefaultSchedules []string
	// Priority is the priority alerts are to be created with
	Priority string

	// WebProxyURL is the Teleport address used when building the bodies of the alerts
	// allowing links to the access requests to be built
	WebProxyURL *url.URL
	// ClusterName is the name of the Teleport cluster
	ClusterName string
}

// NewClient creates a new Opsgenie client for managing alerts.
func NewClient(conf ClientConfig) (*Client, error) {
	client := resty.NewWithClient(defaults.Config().HTTPClient)
	client.SetHeader("Authorization", "GenieKey "+conf.APIKey)
	client.SetBaseURL(conf.APIEndpoint)
	return &Client{
		client:       client,
		ClientConfig: conf,
	}, nil
}

func errWrapper(statusCode int) error {
	switch statusCode {
	case http.StatusForbidden:
		return trace.AccessDenied("opsgenie API access denied: status code %v", statusCode)
	case http.StatusRequestTimeout:
		return trace.ConnectionProblem(trace.Errorf("status code %v", statusCode),
			"connecting to opsgenie API")
	}
	return trace.Errorf("connecting to opsgenie API status code %v", statusCode)
}

// CreateAlert creates an opsgenie alert.
func (og Client) CreateAlert(ctx context.Context, reqID string, reqData RequestData) (OpsgenieData, error) {
	bodyDetails, err := buildAlertBody(og.WebProxyURL, reqID, reqData)
	if err != nil {
		return OpsgenieData{}, trace.Wrap(err)
	}

	body := AlertBody{
		Message:     fmt.Sprintf("Access request from %s", reqData.User),
		Alias:       fmt.Sprintf("%s/%s", alertKeyPrefix, reqID),
		Description: bodyDetails,
		Responders:  og.getResponders(reqData),
		Priority:    og.Priority,
	}

	var result AlertResult
	resp, err := og.client.NewRequest().
		SetContext(ctx).
		SetBody(body).
		SetResult(&result).
		Post("alerts")

	if err != nil {
		return OpsgenieData{}, trace.Wrap(err)
	}
	defer resp.RawResponse.Body.Close()
	if resp.IsError() {
		return OpsgenieData{}, errWrapper(resp.StatusCode())
	}
	return OpsgenieData{
		AlertID: result.Alert.ID,
	}, nil
}

func (og Client) getResponders(reqData RequestData) []Responder {
	schedules := og.DefaultSchedules
	if reqSchedules, ok := reqData.RequestAnnotations[ReqAnnotationRespondersKey]; ok {
		schedules = reqSchedules
	}
	responders := make([]Responder, 0, len(schedules))
	for _, s := range schedules {
		responders = append(responders, Responder{
			Type: "schedule",
			ID:   s,
		})
	}
	return responders
}

// PostReviewNote posts a note once a new request review appears.
func (og Client) PostReviewNote(ctx context.Context, alertID string, review types.AccessReview) error {
	note, err := buildReviewNoteBody(review)
	if err != nil {
		return trace.Wrap(err)
	}
	body := AlertNote{
		Note: note,
	}
	resp, err := og.client.NewRequest().
		SetContext(ctx).
		SetBody(body).
		SetPathParams(map[string]string{"alertID": alertID}).
		SetQueryParams(map[string]string{"identifierType": "id"}).
		Post("alerts/{alertID}/notes")

	if err != nil {
		return trace.Wrap(err)
	}
	defer resp.RawResponse.Body.Close()
	if resp.IsError() {
		return errWrapper(resp.StatusCode())
	}
	return nil
}

// ResolveAlert resolves an alert and posts a note with resolution details.
func (og Client) ResolveAlert(ctx context.Context, alertID string, resolution Resolution) error {
	note, err := buildResolutionNoteBody(resolution)
	if err != nil {
		return trace.Wrap(err)
	}
	body := AlertNote{
		Note: note,
	}
	resp, err := og.client.NewRequest().
		SetContext(ctx).
		SetBody(body).
		SetPathParams(map[string]string{"alertID": alertID}).
		SetQueryParams(map[string]string{"identifierType": "id"}).
		Post("alerts/{alertID}/close")
	if err != nil {
		return trace.Wrap(err)
	}
	defer resp.RawResponse.Body.Close()
	if resp.IsError() {
		return errWrapper(resp.StatusCode())
	}
	return nil
}

// GetOnCall returns the list of responders on-call for a schedule.
func (og Client) GetOnCall(ctx context.Context, scheduleName string) (RespondersResult, error) {
	var result RespondersResult
	resp, err := og.client.NewRequest().
		SetContext(ctx).
		SetPathParams(map[string]string{"scheduleName": scheduleName}).
		SetQueryParams(map[string]string{
			"scheduleIdentifierType": "name",
			// When flat is enabled it returns the email addresses of on-call participants.
			"flat": "true",
		}).
		SetResult(&result).
		Post("schedules/{scheduleName}/on-calls")
	if err != nil {
		return RespondersResult{}, trace.Wrap(err)
	}
	defer resp.RawResponse.Body.Close()
	if resp.IsError() {
		return RespondersResult{}, errWrapper(resp.StatusCode())
	}
	return result, nil
}

func buildAlertBody(webProxyURL *url.URL, reqID string, reqData RequestData) (string, error) {
	var requestLink string
	if webProxyURL != nil {
		reqURL := *webProxyURL
		reqURL.Path = lib.BuildURLPath("web", "requests", reqID)
		requestLink = reqURL.String()
	}

	var builder strings.Builder
	err := alertBodyTemplate.Execute(&builder, struct {
		ID          string
		TimeFormat  string
		RequestLink string
		RequestData
	}{
		ID:          reqID,
		TimeFormat:  time.RFC822,
		RequestLink: requestLink,
		RequestData: reqData,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}
	return builder.String(), nil
}

func buildReviewNoteBody(review types.AccessReview) (string, error) {
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

func buildResolutionNoteBody(resolution Resolution) (string, error) {
	var builder strings.Builder
	err := resolutionNoteTemplate.Execute(&builder, struct {
		Resolution    string
		ResolveReason string
	}{
		Resolution:    string(resolution.Tag),
		ResolveReason: resolution.Reason,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}
	return builder.String(), nil
}
