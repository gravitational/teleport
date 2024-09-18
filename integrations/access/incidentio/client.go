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

package incidentio

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/aws/aws-sdk-go/aws/defaults"
	"github.com/go-resty/resty/v2"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/lib/logger"
	"github.com/gravitational/trace"
)

const (
	// alertKeyPrefix is the prefix for Alert's alias field used when creating an Alert.
	alertKeyPrefix = "teleport-access-request"
)

// AlertClient is a wrapper around resty.Client.
type AlertClient struct {
	ClientConfig

	client *resty.Client
}

type APIClient struct {
	ClientConfig

	client *resty.Client
}

// ClientConfig is the config for the incident.io client.
type ClientConfig struct {
	// AccessToken is the token for the incident.io Alert Source
	AccessToken string `toml:"access_token"`
	// APIKey is the API key for the incident.io API
	APIKey string `toml:"api_key"`
	// AlertSourceEndpoint is the endpoint for the incident.io Alert Source
	AlertSourceEndpoint string `toml:"alert_source_endpoint"`
	// APIEndpoint is the endpoint for the incident.io API
	// api url of the form https://api.incident.io/v2/ with optional trailing '/'
	APIEndpoint string `toml:"api_endpoint"`
	// DefaultSchedules are the default on-call schedules to check for auto approval
	DefaultSchedules []string `toml:"default_schedules"`

	// WebProxyURL is the Teleport address used when building the bodies of the alerts
	// allowing links to the access requests to be built
	WebProxyURL *url.URL `toml:"web_proxy_url"`
	// ClusterName is the name of the Teleport cluster
	ClusterName string `toml:"cluster_name"`

	// StatusSink receives any status updates from the plugin for
	// further processing. Status updates will be ignored if not set.
	StatusSink common.StatusSink
}

func (cfg *ClientConfig) CheckAndSetDefaults() error {
	if cfg.AccessToken == "" {
		return trace.BadParameter("missing required value AccessToken")
	}
	if cfg.AlertSourceEndpoint == "" {
		return trace.BadParameter("missing required value AlertSourceEndpoint")
	}
	if cfg.APIKey == "" {
		return trace.BadParameter("missing required value APIKey")
	}
	if cfg.APIEndpoint == "" {
		return trace.BadParameter("missing required value APIEndpoint")
	}
	return nil
}

// NewAlertClient creates a new incident.io client for sending alerts.
func NewAlertClient(conf ClientConfig) (*AlertClient, error) {
	client := resty.NewWithClient(defaults.Config().HTTPClient)
	client.SetHeader("Authorization", "Bearer "+conf.AccessToken)
	return &AlertClient{
		client:       client,
		ClientConfig: conf,
	}, nil
}

// NewAPIClient creates a new incident.io client for interacting with the incident.io API.
func NewAPIClient(conf ClientConfig) (*APIClient, error) {
	client := resty.NewWithClient(defaults.Config().HTTPClient)
	client.SetHeader("Authorization", "Bearer "+conf.APIKey)
	client.SetBaseURL(conf.APIEndpoint)
	return &APIClient{
		client:       client,
		ClientConfig: conf,
	}, nil
}

func errWrapper(statusCode int, body string) error {
	switch statusCode {
	case http.StatusForbidden:
		return trace.AccessDenied("incident.io API access denied: status code %v: %q", statusCode, body)
	case http.StatusRequestTimeout:
		return trace.ConnectionProblem(trace.Errorf("status code %v: %q", statusCode, body),
			"connecting to incident.io API")
	}
	return trace.Errorf("connecting to incident.io API status code %v: %q", statusCode, body)
}

// CreateAlert creates an incidentio alert.
func (inc AlertClient) CreateAlert(ctx context.Context, reqID string, reqData RequestData) (IncidentAlertData, error) {
	body := AlertBody{
		Title:            fmt.Sprintf("Access request from %s", reqData.User),
		DeduplicationKey: fmt.Sprintf("%s/%s", alertKeyPrefix, reqID),
		Description:      fmt.Sprintf("Access request from %s", reqData.User),
		Status:           "firing",
		Metadata: map[string]string{
			"request_id": reqID,
		},
	}

	var result AlertBody
	resp, err := inc.client.NewRequest().
		SetContext(ctx).
		SetBody(body).
		SetResult(&result).
		Post(inc.AlertSourceEndpoint)
	if err != nil {
		return IncidentAlertData{}, trace.Wrap(err)
	}
	defer resp.RawResponse.Body.Close()
	if resp.IsError() {
		return IncidentAlertData{}, errWrapper(resp.StatusCode(), string(resp.Body()))
	}

	return IncidentAlertData{
		DeduplicationKey: result.DeduplicationKey,
	}, nil
}

// ResolveAlert resolves an alert and posts a note with resolution details.
func (inc AlertClient) ResolveAlert(ctx context.Context, alertID string, resolution Resolution) error {
	alertBody := &AlertBody{
		Status:           "resolved",
		Title:            fmt.Sprintf("Access request resolved: %s", resolution.Tag),
		Description:      fmt.Sprintf("Access request has been %s", resolution.Tag),
		DeduplicationKey: fmt.Sprintf("%s/%s", alertKeyPrefix, alertID),
		Metadata: map[string]string{
			"request_id": alertID,
		},
	}

	resp, err := inc.client.NewRequest().
		SetContext(ctx).
		SetBody(alertBody).
		Post(inc.AlertSourceEndpoint)
	if err != nil {
		return trace.Wrap(err)
	}
	defer resp.RawResponse.Body.Close()
	if resp.IsError() {
		return errWrapper(resp.StatusCode(), string(resp.Body()))
	}
	return nil
}

// GetOnCall returns the list of responders on-call for a schedule.
func (inc APIClient) GetOnCall(ctx context.Context, scheduleID string) (GetScheduleResponse, error) {
	var result GetScheduleResponse
	resp, err := inc.client.NewRequest().
		SetContext(ctx).
		SetPathParams(map[string]string{"scheduleID": scheduleID}).
		SetResult(&result).
		Get("/v2/schedules/{scheduleID}")
	if err != nil {
		return GetScheduleResponse{}, trace.Wrap(err)
	}
	defer resp.RawResponse.Body.Close()
	if resp.IsError() {
		return GetScheduleResponse{}, errWrapper(resp.StatusCode(), string(resp.Body()))
	}
	return result, nil
}

// CheckHealth pings incident.io.
func (inc APIClient) CheckHealth(ctx context.Context) error {
	// The heartbeat pings will respond even if the heartbeat does not exist.
	resp, err := inc.client.NewRequest().
		SetContext(ctx).
		Get("v2/schedules")

	if err != nil {
		return trace.Wrap(err)
	}
	defer resp.RawResponse.Body.Close()

	if inc.StatusSink != nil {
		var code types.PluginStatusCode
		switch {
		case resp.StatusCode() == http.StatusUnauthorized:
			code = types.PluginStatusCode_UNAUTHORIZED
		case resp.StatusCode() >= 200 && resp.StatusCode() < 400:
			code = types.PluginStatusCode_RUNNING
		default:
			code = types.PluginStatusCode_OTHER_ERROR
		}
		if err := inc.StatusSink.Emit(ctx, &types.PluginStatusV1{Code: code}); err != nil {
			logger.Get(resp.Request.Context()).WithError(err).
				WithField("code", resp.StatusCode()).Errorf("Error while emitting servicenow plugin status: %v", err)
		}
	}

	if resp.IsError() {
		return errWrapper(resp.StatusCode(), string(resp.Body()))
	}
	return nil
}
