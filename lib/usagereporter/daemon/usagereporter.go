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

package usagereporter

import (
	"context"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/gravitational/trace"

	prehogv1a "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	prehogv1ac "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha/prehogv1alphaconnect"
	teletermv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/usagereporter"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// minBatchSize determines the size at which a batch is sent
	// regardless of elapsed time
	minBatchSize = 15

	// maxBatchSize is the largest batch size that will be sent to
	// the server; batches larger than this will be split into multiple
	// requests.
	maxBatchSize = 30

	// maxBatchAge is the maximum age a batch may reach before
	// being flushed, regardless of the batch size
	maxBatchAge = time.Hour

	// maxBufferSize is the maximum size to which the event buffer
	// may grow. Events submitted once this limit is reached will be discarded.
	// Events that were in the submission queue that fail to submit may also be
	// discarded when requeued.
	maxBufferSize = 60

	// submitDelay is a mandatory delay added to each batch submission
	// to avoid spamming the prehog instance.
	submitDelay = time.Second * 1

	// usageReporterRetryAttempts is the max number of attempts that
	// should be made to submit a particular event before it's dropped
	retryAttempts = 2
)

type (
	UsageReporter  = usagereporter.UsageReporter[prehogv1a.SubmitConnectEventRequest]
	SubmitFunc     = usagereporter.SubmitFunc[prehogv1a.SubmitConnectEventRequest]
	SubmittedEvent = usagereporter.SubmittedEvent[prehogv1a.SubmitConnectEventRequest]
)

func NewConnectUsageReporter(ctx context.Context, prehogAddr string) (*UsageReporter, error) {
	submitter, err := newPrehogSubmitter(ctx, prehogAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return usagereporter.NewUsageReporter(&usagereporter.Options[prehogv1a.SubmitConnectEventRequest]{
		Submit:        submitter,
		MinBatchSize:  minBatchSize,
		MaxBatchSize:  maxBatchSize,
		MaxBufferSize: maxBufferSize,
		MaxBatchAge:   maxBatchAge,
		SubmitDelay:   submitDelay,
		RetryAttempts: retryAttempts,
	}), nil
}

func newPrehogSubmitter(ctx context.Context, prehogEndpoint string) (SubmitFunc, error) {
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

	client := prehogv1ac.NewConnectReportingServiceClient(httpClient, prehogEndpoint)

	return func(reporter *UsageReporter, events []*SubmittedEvent) ([]*SubmittedEvent, error) {
		var failed []*SubmittedEvent
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

func GetAnonymizedPrehogEvent(req *teletermv1.ReportUsageEventRequest) (*prehogv1a.SubmitConnectEventRequest, error) {
	prehogEvent := req.PrehogReq

	// non-anonymized
	switch prehogEvent.GetEvent().(type) {
	case *prehogv1a.SubmitConnectEventRequest_UserJobRoleUpdate:
		return prehogEvent, nil
	}

	// anonymized
	// NOTE: for simplicity reasons the teleterm anonymization doesn't match the on-prem teleport anonymization.
	// see https://github.com/gravitational/teleport/pull/35652#issuecomment-1865135970 for details
	anonymizer, err := newClusterAnonymizer(req.GetAuthClusterId()) // authClusterId is sent with each event that needs to be anonymized
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch e := prehogEvent.GetEvent().(type) {
	case *prehogv1a.SubmitConnectEventRequest_ClusterLogin:
		e.ClusterLogin.ClusterName = anonymizer.AnonymizeString(e.ClusterLogin.ClusterName)
		e.ClusterLogin.UserName = anonymizer.AnonymizeString(e.ClusterLogin.UserName)
		return prehogEvent, nil
	case *prehogv1a.SubmitConnectEventRequest_ProtocolUse:
		e.ProtocolUse.ClusterName = anonymizer.AnonymizeString(e.ProtocolUse.ClusterName)
		e.ProtocolUse.UserName = anonymizer.AnonymizeString(e.ProtocolUse.UserName)
		return prehogEvent, nil
	case *prehogv1a.SubmitConnectEventRequest_AccessRequestCreate:
		e.AccessRequestCreate.ClusterName = anonymizer.AnonymizeString(e.AccessRequestCreate.ClusterName)
		e.AccessRequestCreate.UserName = anonymizer.AnonymizeString(e.AccessRequestCreate.UserName)
		return prehogEvent, nil
	case *prehogv1a.SubmitConnectEventRequest_AccessRequestReview:
		e.AccessRequestReview.ClusterName = anonymizer.AnonymizeString(e.AccessRequestReview.ClusterName)
		e.AccessRequestReview.UserName = anonymizer.AnonymizeString(e.AccessRequestReview.UserName)
		return prehogEvent, nil
	case *prehogv1a.SubmitConnectEventRequest_AccessRequestAssumeRole:
		e.AccessRequestAssumeRole.ClusterName = anonymizer.AnonymizeString(e.AccessRequestAssumeRole.ClusterName)
		e.AccessRequestAssumeRole.UserName = anonymizer.AnonymizeString(e.AccessRequestAssumeRole.UserName)
		return prehogEvent, nil
	case *prehogv1a.SubmitConnectEventRequest_FileTransferRun:
		e.FileTransferRun.ClusterName = anonymizer.AnonymizeString(e.FileTransferRun.ClusterName)
		e.FileTransferRun.UserName = anonymizer.AnonymizeString(e.FileTransferRun.UserName)
		return prehogEvent, nil
	case *prehogv1a.SubmitConnectEventRequest_ConnectMyComputerSetup:
		e.ConnectMyComputerSetup.ClusterName = anonymizer.AnonymizeString(e.ConnectMyComputerSetup.ClusterName)
		e.ConnectMyComputerSetup.UserName = anonymizer.AnonymizeString(e.ConnectMyComputerSetup.UserName)
		return prehogEvent, nil
	case *prehogv1a.SubmitConnectEventRequest_ConnectMyComputerAgentStart:
		e.ConnectMyComputerAgentStart.ClusterName = anonymizer.AnonymizeString(e.ConnectMyComputerAgentStart.ClusterName)
		e.ConnectMyComputerAgentStart.UserName = anonymizer.AnonymizeString(e.ConnectMyComputerAgentStart.UserName)
		return prehogEvent, nil
	}

	return nil, trace.BadParameter("unexpected Event usage type %T", req)
}

func newClusterAnonymizer(authClusterID string) (utils.Anonymizer, error) {
	_, err := uuid.Parse(authClusterID)
	if err != nil {
		return nil, trace.BadParameter("Invalid auth cluster ID %s", authClusterID)
	}
	return utils.NewHMACAnonymizer(authClusterID)
}
