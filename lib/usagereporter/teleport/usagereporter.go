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
	"crypto/tls"
	"crypto/x509"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"connectrpc.com/connect"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	prehogv1a "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	prehogv1ac "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha/prehogv1alphaconnect"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/usagereporter"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// usageReporterMinBatchSize determines the size at which a batch is sent
	// regardless of elapsed time
	usageReporterMinBatchSize = 50

	// usageReporterMaxBatchSize is the largest batch size that will be sent to
	// the server; batches larger than this will be split into multiple
	// requests. Matches the limit enforced by the server side for a single RPC.
	usageReporterMaxBatchSize = 500

	// usageReporterMaxBatchAge is the maximum age a batch may reach before
	// being flushed, regardless of the batch size
	usageReporterMaxBatchAge = time.Second * 5

	// usageReporterMaxBufferSize is the maximum size to which the event buffer
	// may grow. Events submitted once this limit is reached will be discarded.
	// Events that were in the submission queue that fail to submit may also be
	// discarded when requeued.
	usageReporterMaxBufferSize = 2500

	// usageReporterSubmitDelay is a mandatory delay added to each batch submission
	// to avoid spamming the prehog instance.
	usageReporterSubmitDelay = time.Second * 1

	// usageReporterRetryAttempts is the max number of attempts that
	// should be made to submit a particular event before it's dropped
	usageReporterRetryAttempts = 5
)

// UsageReporter is a service that accepts Teleport usage events.
type UsageReporter interface {
	// AnonymizeAndSubmit submits a usage event. The payload will be
	// anonymized by the reporter implementation.
	AnonymizeAndSubmit(event ...Anonymizable)
}

// GracefulStopper is a UsageReporter that needs to do some work before
// stopping; this is a separate interface because [UsageReporter] is embedded in
// auth.Server.Services, and we don't want to expose extraneous methods as part
// of auth.Server.
type GracefulStopper interface {
	UsageReporter

	// GracefulStop gracefully closes and runs any finalization needed by the
	// UsageReporter; operations can run as long as the context is alive, but
	// must terminate quickly (even losing data) if the context is closed.
	// Returns nil if operations have completed cleanly.
	GracefulStop(context.Context) error
}

// StreamingUsageReporter submits all Teleport usage events anonymized with the
// cluster name, with a very short buffer for batches and no persistency.
type StreamingUsageReporter struct {
	// usageReporter is an actual reporter that batches and sends events
	usageReporter *usagereporter.UsageReporter[prehogv1a.SubmitEventRequest]
	// anonymizer is the anonymizer used for filtered audit events.
	anonymizer utils.Anonymizer
	// clusterName is the cluster's name, used for anonymization and as an event
	// field.
	clusterName types.ClusterName
	clock       clockwork.Clock
}

var _ UsageReporter = (*StreamingUsageReporter)(nil)

func (t *StreamingUsageReporter) AnonymizeAndSubmit(events ...Anonymizable) {
	for _, e := range events {
		req := e.Anonymize(t.anonymizer)
		req.Timestamp = timestamppb.New(t.clock.Now())
		req.ClusterName = t.anonymizer.AnonymizeString(t.clusterName.GetClusterName())
		req.TeleportVersion = teleport.Version
		t.usageReporter.AddEventsToQueue(&req)
	}
}

func (t *StreamingUsageReporter) Run(ctx context.Context) {
	t.usageReporter.Run(ctx)
}

type SubmitFunc = usagereporter.SubmitFunc[prehogv1a.SubmitEventRequest]

func NewStreamingUsageReporter(logger *slog.Logger, clusterName types.ClusterName, anonymizer utils.Anonymizer, submitter SubmitFunc) (*StreamingUsageReporter, error) {
	if anonymizer == nil {
		return nil, trace.BadParameter("missing anonymizer")
	}

	err := metrics.RegisterPrometheusCollectors(usagereporter.UsagePrometheusCollectors...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clock := clockwork.NewRealClock()

	reporter := usagereporter.NewUsageReporter(&usagereporter.Options[prehogv1a.SubmitEventRequest]{
		Logger:        logger,
		Submit:        submitter,
		MinBatchSize:  usageReporterMinBatchSize,
		MaxBatchSize:  usageReporterMaxBatchSize,
		MaxBatchAge:   usageReporterMaxBatchAge,
		MaxBufferSize: usageReporterMaxBufferSize,
		SubmitDelay:   usageReporterSubmitDelay,
		RetryAttempts: usageReporterRetryAttempts,
		Clock:         clock,
	})

	return &StreamingUsageReporter{
		usageReporter: reporter,
		anonymizer:    anonymizer,
		clusterName:   clusterName,
		clock:         clock,
	}, nil
}

func NewPrehogSubmitter(ctx context.Context, prehogEndpoint string, clientCert *tls.Certificate, caCertPEM []byte) (SubmitFunc, error) {
	tlsConfig := &tls.Config{
		// Self-signed test licenses may not have a proper issuer and won't be
		// used if just passed in via Certificates, so we'll use this to
		// explicitly set the client cert we want to use.
		GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return clientCert, nil
		},
	}

	if len(caCertPEM) > 0 {
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(caCertPEM)

		tlsConfig.RootCAs = pool
	}

	httpClient, err := defaults.HTTPClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	transport, ok := httpClient.Transport.(*http.Transport)
	if !ok {
		return nil, trace.BadParameter("invalid transport type %T", httpClient.Transport)
	}

	transport.Proxy = http.ProxyFromEnvironment
	transport.ForceAttemptHTTP2 = true
	transport.TLSClientConfig = tlsConfig

	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	httpClient.Timeout = 5 * time.Second

	client := prehogv1ac.NewTeleportReportingServiceClient(httpClient, prehogEndpoint)

	return func(reporter *usagereporter.UsageReporter[prehogv1a.SubmitEventRequest], events []*usagereporter.SubmittedEvent[prehogv1a.SubmitEventRequest]) ([]*usagereporter.SubmittedEvent[prehogv1a.SubmitEventRequest], error) {
		evs := make([]*prehogv1a.SubmitEventRequest, 0, len(events))
		for _, e := range events {
			evs = append(evs, e.Event)
		}

		req := connect.NewRequest(&prehogv1a.SubmitEventsRequest{
			Events: evs,
		})
		if _, err := client.SubmitEvents(ctx, req); err != nil {
			return events, trace.Wrap(err)
		}

		return nil, nil
	}, nil
}

// DiscardUsageReporter is a dummy usage reporter that drops all events.
type DiscardUsageReporter struct{}

var _ UsageReporter = DiscardUsageReporter{}

// AnonymizeAndSubmit implements [UsageReporter]
func (DiscardUsageReporter) AnonymizeAndSubmit(...Anonymizable) {
	// do nothing
}

// EmitEditorChangeEvent emits an editor change event if the editor role was added or removed.
func EmitEditorChangeEvent(username string, prevRoles, newRoles []string, submit func(...Anonymizable)) {
	prevEditor := slices.Contains(prevRoles, teleport.PresetEditorRoleName)
	newEditor := slices.Contains(newRoles, teleport.PresetEditorRoleName)

	// don't emit event if editor role wasn't added/removed
	if prevEditor == newEditor {
		return
	}

	eventType := prehogv1a.EditorChangeStatus_EDITOR_CHANGE_STATUS_ROLE_GRANTED
	if prevEditor {
		eventType = prehogv1a.EditorChangeStatus_EDITOR_CHANGE_STATUS_ROLE_REMOVED
	}

	submit(&EditorChangeEvent{
		UserName: username,
		Status:   eventType,
	})
}
