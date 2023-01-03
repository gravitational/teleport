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
				failed = append(failed, event)
				errors = append(errors, err)
			}
		}

		return failed, trace.NewAggregate(errors...)
	}, nil
}

func (s *Service) ReportUsageEvent(req *api.ReportUsageEventRequest) error {
	event, err := convertAndAnonymizeApiEvent(req)
	if err != nil {
		return trace.Wrap(err)
	}
	s.usageReporter.AddEventsToQueue(event)
	return nil
}

func convertAndAnonymizeApiEvent(event *api.ReportUsageEventRequest) (*prehogapi.SubmitConnectEventRequest, error) {
	prehogReq := event.PrehogReq

	// non-anonymized
	switch prehogReq.GetEvent().(type) {
	case *prehogapi.SubmitConnectEventRequest_UserJobRoleUpdate:
		return prehogReq, nil
	}

	// anonymized
	anonymizer, err := newClusterAnonymizer(event.GetAuthClusterId()) // authClusterId is sent with each event that needs to be anonymized
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch e := prehogReq.GetEvent().(type) {
	case *prehogapi.SubmitConnectEventRequest_UserLogin:
		e.UserLogin.ClusterName = anonymizer.AnonymizeString(e.UserLogin.ClusterName)
		e.UserLogin.UserName = anonymizer.AnonymizeString(e.UserLogin.UserName)
		return prehogReq, nil
	case *prehogapi.SubmitConnectEventRequest_ProtocolUse:
		e.ProtocolUse.ClusterName = anonymizer.AnonymizeString(e.ProtocolUse.ClusterName)
		e.ProtocolUse.UserName = anonymizer.AnonymizeString(e.ProtocolUse.UserName)
		return prehogReq, nil
	case *prehogapi.SubmitConnectEventRequest_AccessRequestCreate:
		e.AccessRequestCreate.ClusterName = anonymizer.AnonymizeString(e.AccessRequestCreate.ClusterName)
		e.AccessRequestCreate.UserName = anonymizer.AnonymizeString(e.AccessRequestCreate.UserName)
		return prehogReq, nil
	case *prehogapi.SubmitConnectEventRequest_AccessRequestReview:
		e.AccessRequestReview.ClusterName = anonymizer.AnonymizeString(e.AccessRequestReview.ClusterName)
		e.AccessRequestReview.UserName = anonymizer.AnonymizeString(e.AccessRequestReview.UserName)
		return prehogReq, nil
	case *prehogapi.SubmitConnectEventRequest_AccessRequestAssumeRole:
		e.AccessRequestAssumeRole.ClusterName = anonymizer.AnonymizeString(e.AccessRequestAssumeRole.ClusterName)
		e.AccessRequestAssumeRole.UserName = anonymizer.AnonymizeString(e.AccessRequestAssumeRole.UserName)
		return prehogReq, nil
	case *prehogapi.SubmitConnectEventRequest_FileTransferRun:
		e.FileTransferRun.ClusterName = anonymizer.AnonymizeString(e.FileTransferRun.ClusterName)
		e.FileTransferRun.UserName = anonymizer.AnonymizeString(e.FileTransferRun.UserName)
		return prehogReq, nil
	}

	return nil, trace.BadParameter("unexpected Event usage type %T", event)
}

func newClusterAnonymizer(authClusterId string) (utils.Anonymizer, error) {
	_, err := uuid.Parse(authClusterId)
	if err != nil {
		return nil, trace.BadParameter("Invalid auth cluster ID %s", authClusterId)
	}
	return utils.NewHMACAnonymizer(authClusterId)
}
