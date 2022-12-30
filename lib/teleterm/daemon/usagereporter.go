// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package daemon

import (
	"context"
	"github.com/gravitational/teleport/lib/usagereporter"
	"net/http"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/google/uuid"
	"github.com/gravitational/teleport/lib/defaults"
	prehogapi "github.com/gravitational/teleport/lib/prehog/gen/prehog/v1alpha"
	prehogclient "github.com/gravitational/teleport/lib/prehog/gen/prehog/v1alpha/prehogv1alphaconnect"
	api "github.com/gravitational/teleport/lib/teleterm/api/protogen/golang/v1"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

const (
	// minBatchSize determines the size at which a batch is sent
	// regardless of elapsed time
	minBatchSize = 25

	// maxBatchSize is the largest batch size that will be sent to
	// the server; batches larger than this will be split into multiple
	// requests.
	maxBatchSize = 50

	// maxBatchAge is the maximum age a batch may reach before
	// being flushed, regardless of the batch size
	maxBatchAge = time.Minute * 30

	// maxBufferSize is the maximum size to which the event buffer
	// may grow. Events submitted once this limit is reached will be discarded.
	// Events that were in the submission queue that fail to submit may also be
	// discarded when requeued.
	maxBufferSize = 100

	// submitDelay is a mandatory delay added to each batch submission
	// to avoid spamming the prehog instance.
	submitDelay = time.Second * 1

	// usageReporterRetryAttempts is the max number of attempts that
	// should be made to submit a particular event before it's dropped
	retryAttempts = 2
)

func NewConnectUsageReporter(ctx context.Context) (*usagereporter.UsageReporter[prehogapi.SubmitConnectEventRequest], error) {
	submitter, err := newRealPrehogSubmitter(ctx, "https://localhost:8443") // TODO: change the address before merge
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return usagereporter.NewUsageReporter(&usagereporter.Options[prehogapi.SubmitConnectEventRequest]{
		Submit:        submitter,
		MinBatchSize:  minBatchSize,
		MaxBatchSize:  maxBatchSize,
		MaxBufferSize: maxBufferSize,
		MaxBatchAge:   maxBatchAge,
		SubmitDelay:   submitDelay,
		RetryAttempts: retryAttempts,
	}), nil
}

func newRealPrehogSubmitter(ctx context.Context, prehogEndpoint string) (usagereporter.SubmitFunc[prehogapi.SubmitConnectEventRequest], error) {
	httpClient := &http.Client{
		Transport: &http.Transport{
			ForceAttemptHTTP2:   true,
			Proxy:               http.ProxyFromEnvironment,
			IdleConnTimeout:     defaults.HTTPIdleTimeout,
			MaxIdleConns:        defaults.HTTPMaxIdleConns,
			MaxIdleConnsPerHost: defaults.HTTPMaxConnsPerHost,
		},
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: 5 * time.Second,
	}

	client := prehogclient.NewConnectReportingServiceClient(httpClient, prehogEndpoint)

	return func(reporter *usagereporter.UsageReporter[prehogapi.SubmitConnectEventRequest], events []*usagereporter.SubmittedEvent[prehogapi.SubmitConnectEventRequest]) ([]*usagereporter.SubmittedEvent[prehogapi.SubmitConnectEventRequest], error) {
		var failed []*usagereporter.SubmittedEvent[prehogapi.SubmitConnectEventRequest]
		var errors []error

		// Note: the backend doesn't support batching at the moment.
		for _, event := range events {
			// Note: this results in retrying the entire batch, which probably
			// isn't ideal.
			req := connect.NewRequest(event.Event)
			if _, err := client.SubmitConnectEvent(ctx, req); err != nil {
				errors = append(errors, err)
			}
		}

		return failed, trace.NewAggregate(errors...)
	}, nil
}

func (s *Service) ReportUsageEvent(req *api.ReportEventRequest) error {
	event, err := convertAndAnonymizeApiEvent(req)
	if err != nil {
		return trace.Wrap(err)
	}
	s.usageReporter.AddEventsToQueue(event)
	return nil
}

func convertAndAnonymizeApiEvent(event *api.ReportEventRequest) (*prehogapi.SubmitConnectEventRequest, error) {
	convertedEvent := &prehogapi.SubmitConnectEventRequest{
		DistinctId: event.GetDistinctId(),
		Timestamp:  event.GetTimestamp(),
	}
	switch e := event.GetEvent().GetEvent().(type) {
	//anonymized
	case *api.ConnectUsageEventOneOf_LoginEvent:
		anonymizer, err := newClusterAnonymizer(e.LoginEvent.GetClusterProperties().GetAuthClusterId()) // authClusterId is sent with each event that needs to be anonymized
		if err != nil {
			return nil, trace.Wrap(err)
		}
		convertedEvent.Event = anonymizeLoginEvent(e.LoginEvent, anonymizer)
	case *api.ConnectUsageEventOneOf_ProtocolRunEvent:
		anonymizer, err := newClusterAnonymizer(e.ProtocolRunEvent.GetClusterProperties().GetAuthClusterId())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		convertedEvent.Event = anonymizeProtocolRunEvent(e.ProtocolRunEvent, anonymizer)
	case *api.ConnectUsageEventOneOf_AccessRequestCreateEvent:
		anonymizer, err := newClusterAnonymizer(e.AccessRequestCreateEvent.GetClusterProperties().GetAuthClusterId())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		convertedEvent.Event = anonymizeAccessRequestCreateEvent(e.AccessRequestCreateEvent, anonymizer)
	case *api.ConnectUsageEventOneOf_AccessRequestReviewEvent:
		anonymizer, err := newClusterAnonymizer(e.AccessRequestReviewEvent.GetClusterProperties().GetAuthClusterId())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		convertedEvent.Event = anonymizeAccessRequestReviewEvent(e.AccessRequestReviewEvent, anonymizer)
	case *api.ConnectUsageEventOneOf_AccessRequestAssumeRoleEvent:
		anonymizer, err := newClusterAnonymizer(e.AccessRequestAssumeRoleEvent.GetClusterProperties().GetAuthClusterId())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		convertedEvent.Event = anonymizeAccessRequestAssumeRoleEvent(e.AccessRequestAssumeRoleEvent, anonymizer)
	case *api.ConnectUsageEventOneOf_FileTransferRunEvent:
		anonymizer, err := newClusterAnonymizer(e.FileTransferRunEvent.GetClusterProperties().GetAuthClusterId())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		convertedEvent.Event = anonymizeFileTransferRunEvent(e.FileTransferRunEvent, anonymizer)

	// non-anonymized
	case *api.ConnectUsageEventOneOf_UserJobRoleUpdateEvent:
		convertedEvent.Event = convertUserJobRoleUpdateEvent(e.UserJobRoleUpdateEvent)

	default:
		return nil, trace.BadParameter("unexpected Event usage type %T", event)
	}

	return convertedEvent, nil
}

func newClusterAnonymizer(authClusterId string) (utils.Anonymizer, error) {
	_, err := uuid.Parse(authClusterId)
	if err != nil {
		return nil, trace.BadParameter("Invalid auth cluster ID %s", authClusterId)
	}
	return utils.NewHMACAnonymizer(authClusterId)
}

func anonymizeLoginEvent(event *api.LoginEvent, anonymizer utils.Anonymizer) *prehogapi.SubmitConnectEventRequest_ConnectLogin {
	return &prehogapi.SubmitConnectEventRequest_ConnectLogin{
		ConnectLogin: &prehogapi.ConnectLoginEvent{
			ClusterName:    anonymizer.AnonymizeString(event.GetClusterProperties().GetClusterName()),
			UserName:       anonymizer.AnonymizeString(event.GetClusterProperties().GetUserName()),
			Arch:           event.GetArch(),
			Os:             event.GetOs(),
			OsVersion:      event.GetOsVersion(),
			ConnectVersion: event.GetConnectVersion(),
		},
	}
}

func anonymizeProtocolRunEvent(event *api.ProtocolRunEvent, anonymizer utils.Anonymizer) *prehogapi.SubmitConnectEventRequest_ConnectProtocolRun {
	return &prehogapi.SubmitConnectEventRequest_ConnectProtocolRun{
		ConnectProtocolRun: &prehogapi.ConnectProtocolRunEvent{
			ClusterName: anonymizer.AnonymizeString(event.GetClusterProperties().GetClusterName()),
			UserName:    anonymizer.AnonymizeString(event.GetClusterProperties().GetUserName()),
			Protocol:    event.GetProtocol(),
		},
	}
}

func anonymizeAccessRequestCreateEvent(event *api.AccessRequestCreateEvent, anonymizer utils.Anonymizer) *prehogapi.SubmitConnectEventRequest_ConnectAccessRequestCreate {
	return &prehogapi.SubmitConnectEventRequest_ConnectAccessRequestCreate{
		ConnectAccessRequestCreate: &prehogapi.ConnectAccessRequestCreateEvent{
			ClusterName: anonymizer.AnonymizeString(event.GetClusterProperties().GetClusterName()),
			UserName:    anonymizer.AnonymizeString(event.GetClusterProperties().GetUserName()),
			Kind:        event.GetKind(),
		},
	}
}

func anonymizeAccessRequestReviewEvent(event *api.AccessRequestReviewEvent, anonymizer utils.Anonymizer) *prehogapi.SubmitConnectEventRequest_ConnectAccessRequestReview {
	return &prehogapi.SubmitConnectEventRequest_ConnectAccessRequestReview{
		ConnectAccessRequestReview: &prehogapi.ConnectAccessRequestReviewEvent{
			ClusterName: anonymizer.AnonymizeString(event.GetClusterProperties().GetClusterName()),
			UserName:    anonymizer.AnonymizeString(event.GetClusterProperties().GetUserName()),
		},
	}
}

func anonymizeAccessRequestAssumeRoleEvent(event *api.AccessRequestAssumeRoleEvent, anonymizer utils.Anonymizer) *prehogapi.SubmitConnectEventRequest_ConnectAccessRequestAssumeRole {
	return &prehogapi.SubmitConnectEventRequest_ConnectAccessRequestAssumeRole{
		ConnectAccessRequestAssumeRole: &prehogapi.ConnectAccessRequestAssumeRoleEvent{
			ClusterName: anonymizer.AnonymizeString(event.GetClusterProperties().GetClusterName()),
			UserName:    anonymizer.AnonymizeString(event.GetClusterProperties().GetUserName()),
		},
	}
}

func anonymizeFileTransferRunEvent(event *api.FileTransferRunEvent, anonymizer utils.Anonymizer) *prehogapi.SubmitConnectEventRequest_ConnectFileTransferRunEvent {
	return &prehogapi.SubmitConnectEventRequest_ConnectFileTransferRunEvent{
		ConnectFileTransferRunEvent: &prehogapi.ConnectFileTransferRunEvent{
			ClusterName: anonymizer.AnonymizeString(event.GetClusterProperties().GetClusterName()),
			UserName:    anonymizer.AnonymizeString(event.GetClusterProperties().GetUserName()),
			Direction:   event.GetDirection(),
		},
	}
}

func convertUserJobRoleUpdateEvent(event *api.UserJobRoleUpdateEvent) *prehogapi.SubmitConnectEventRequest_ConnectUserJobRoleUpdateEvent {
	return &prehogapi.SubmitConnectEventRequest_ConnectUserJobRoleUpdateEvent{
		ConnectUserJobRoleUpdateEvent: &prehogapi.ConnectUserJobRoleUpdateEvent{
			JobRole: event.GetJobRole(),
		},
	}
}
