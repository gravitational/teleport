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

package externalauditstorage

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

const (
	eventEmitFailureClusterAlert                  = "event-emit"
	eventEmitFailureClusterAlertMsgTemplate       = "External Audit Storage: experiencing elevated error rate emitting events. Recent error message: %s"
	eventSearchFailureClusterAlert                = "event-search"
	eventSearchFailureClusterAlertMsgTemplate     = "External Audit Storage: experiencing elevated error rate searching events. Recent error message: %s"
	sessionUploadFailureClusterAlert              = "session-upload"
	sessionUploadFailureClusterAlertMsgTemplate   = "External Audit Storage: experiencing elevated error rate uploading session recordings. Recent error message: %s"
	sessionDownloadFailureClusterAlert            = "session-download"
	sessionDownloadFailureClusterAlertMsgTemplate = "External Audit Storage: experiencing elevated error rate downloading session recordings. Recent error message: %s"

	syncInterval = 30 * time.Second
)

var log = logutils.NewPackageLogger(teleport.ComponentKey, "ExternalAuditStorage")

// ClusterAlertService abstracts a service providing Upsert and Delete
// operations for cluster alerts.
type ClusterAlertService interface {
	// UpsertClusterAlert creates the specified alert, overwriting any preexising alert with the same ID.
	UpsertClusterAlert(ctx context.Context, alert types.ClusterAlert) error
	// DeleteClusterAlert deletes the cluster alert with the specified ID.
	DeleteClusterAlert(ctx context.Context, alertID string) error
}

// ErrorCounter is used when the External Audit Storage feature is enabled to
// store audit events and session recordings on external infrastructure. It
// effectively provides audit middlewares that count errors and raise or clear
// cluster alerts based on recent error rates. Cluster alerts are used to
// surface this information because Cloud customers don't have access to their
// own Auth server logs.
type ErrorCounter struct {
	emits     errorCount
	searches  errorCount
	uploads   errorCount
	downloads errorCount

	alertService ClusterAlertService
	clock        clockwork.Clock

	categories []errorCategory
}

// NewErrorCounter takes a ClusterAlertService that will be used to raise or
// clear cluster alerts and returns a new ErrorCounter.
func NewErrorCounter(alertService ClusterAlertService) *ErrorCounter {
	c := &ErrorCounter{
		alertService: alertService,
		clock:        clockwork.NewRealClock(),
	}
	c.categories = []errorCategory{
		{
			alertName:    eventEmitFailureClusterAlert,
			alertMessage: eventEmitFailureClusterAlertMsgTemplate,
			// Raise a cluster alert if the recent error rate reaches 40%.
			alertThreshold: 0.4,
			// Don't alert until there have been at least 10 errors.
			minErrorsForAlert: 10,
			// Clear the alert if the recent error rate is below 5%.
			clearAlertThreshold: 0.05,
			// Don't clear the alert unless there have been at least 10
			// successes.
			minSuccessesForClear: 10,
			count:                &c.emits,
		},
		{
			alertName:            eventSearchFailureClusterAlert,
			alertMessage:         eventSearchFailureClusterAlertMsgTemplate,
			minErrorsForAlert:    4,
			alertThreshold:       0.4,
			minSuccessesForClear: 4,
			clearAlertThreshold:  0.05,
			count:                &c.searches,
		},
		{
			alertName:            sessionUploadFailureClusterAlert,
			alertMessage:         sessionUploadFailureClusterAlertMsgTemplate,
			minErrorsForAlert:    4,
			alertThreshold:       0.4,
			minSuccessesForClear: 8,
			clearAlertThreshold:  0.05,
			count:                &c.uploads,
		},
		{
			alertName:            sessionDownloadFailureClusterAlert,
			alertMessage:         sessionDownloadFailureClusterAlertMsgTemplate,
			minErrorsForAlert:    4,
			alertThreshold:       0.4,
			minSuccessesForClear: 2,
			clearAlertThreshold:  0.05,
			count:                &c.downloads,
		},
	}
	return c
}

// WrapAuditLogger returns an [events.AuditLogger] that will forward all calls
// to [wrapped] and observe all errors encountered.
func (c *ErrorCounter) WrapAuditLogger(wrapped events.AuditLogger) *ErrorCountingLogger {
	return newErrorCountingLogger(wrapped, &c.emits, &c.searches)
}

// WrapSessionHandler returns an [events.MultipartHandler] that will forward all
// calls to [wrapped] and observe all errors encountered.
func (c *ErrorCounter) WrapSessionHandler(wrapped events.MultipartHandler) *ErrorCountingSessionHandler {
	return newErrorCountingSessionHandler(wrapped, &c.uploads, &c.downloads)
}

// ObserveEmitError can be called to observe relevant event emit errors not
// captured by WrapAuditLogger. In particular this should be used by the Athena
// consumer which batches event writes to S3.
func (c *ErrorCounter) ObserveEmitError(err error) {
	c.emits.observe(err)
}

type errorCategory struct {
	alertName            string
	alertMessage         string
	alertThreshold       float32
	clearAlertThreshold  float32
	minErrorsForAlert    uint64
	minSuccessesForClear uint64
	count                *errorCount
}

func (c *ErrorCounter) run(exitContext context.Context) {
	ticker := c.clock.NewTicker(syncInterval)
	defer ticker.Stop()
	for {
		select {
		case <-exitContext.Done():
			return
		case <-ticker.Chan():
		}
		c.sync(exitContext)
	}
}

func (c *ErrorCounter) sync(ctx context.Context) {
	var allAlertActions alertActions
	for _, category := range c.categories {
		allAlertActions.merge(category.sync())
	}

	for _, newAlert := range allAlertActions.newAlerts {
		alert, err := types.NewClusterAlert(newAlert.name, newAlert.message,
			types.WithAlertSeverity(types.AlertSeverity_HIGH),
			types.WithAlertLabel(types.AlertOnLogin, "yes"),
			types.WithAlertLabel(types.AlertVerbPermit, "external_audit_storage:create"))
		if err != nil {
			log.InfoContext(ctx, "ErrorCounter failed to create cluster alert",
				"alert_name", newAlert.name,
				"error", err,
			)
			continue
		}
		if err := c.alertService.UpsertClusterAlert(ctx, alert); err != nil {
			log.InfoContext(ctx, "ErrorCounter failed to upsert cluster alert",
				"alert_name", newAlert.name,
				"error", err,
			)
		}
	}
	for _, alertToClear := range allAlertActions.clearAlerts {
		if err := c.alertService.DeleteClusterAlert(ctx, alertToClear); err != nil && !trace.IsNotFound(err) {
			log.InfoContext(ctx, "ErrorCounter failed to delete cluster alert",
				"alert_name", alertToClear,
				"error", err,
			)
		}
	}
}

func (c *errorCategory) sync() alertActions {
	errorCount, successCount := c.count.errors.Load(), c.count.successes.Load()
	var errorRate float32
	// Avoid NaN
	if errorCount > 0 {
		errorRate = float32(errorCount) / float32(errorCount+successCount)
	}
	err := c.count.recentError.Load()
	if errorCount >= c.minErrorsForAlert && successCount >= c.minSuccessesForClear {
		// A good number of observations have happened since the last reset and
		// the results are mixed, reset the count to bias in favor of more
		// recent observations.
		// This doesn't prevent raising or clearing an alert below if a
		// threshold has been met.
		// This condition avoids resetting the count prematurely if errors are
		// happening slowly.
		c.count.reset()
	}
	if errorCount >= c.minErrorsForAlert &&
		errorRate >= c.alertThreshold {
		// Raising an alert, reset the count so that recovery can be detected
		// quickly.
		c.count.reset()
		return alertActions{
			newAlerts: []alert{{
				name:    c.alertName,
				message: fmt.Sprintf(c.alertMessage, sanitizeErrForAlert(*err)),
			}},
		}
	}
	if successCount >= c.minSuccessesForClear &&
		errorRate <= c.clearAlertThreshold {
		// Counted sufficient successes to clear any alert, reset the count so
		// that future errors can be detected quickly.
		c.count.reset()
		return alertActions{
			clearAlerts: []string{c.alertName},
		}
	}
	return alertActions{}
}

type alertActions struct {
	clearAlerts []string
	newAlerts   []alert
}

func (a *alertActions) merge(o alertActions) {
	a.clearAlerts = append(a.clearAlerts, o.clearAlerts...)
	a.newAlerts = append(a.newAlerts, o.newAlerts...)
}

type alert struct {
	name    string
	message string
}

type errorCount struct {
	recentError atomic.Pointer[error]
	errors      atomic.Uint64
	successes   atomic.Uint64
}

func (c *errorCount) observe(err error) {
	if err != nil {
		c.recentError.Store(&err)
		c.errors.Add(1)
	} else {
		c.successes.Add(1)
	}
}

func (c *errorCount) reset() {
	c.errors.Store(0)
	c.successes.Store(0)
}

// ErrorCountingLogger wraps an AuditLogger and counts errors on emit and search
// operations.
type ErrorCountingLogger struct {
	wrapped events.AuditLogger

	emits    *errorCount
	searches *errorCount
}

func newErrorCountingLogger(wrapped events.AuditLogger, emits, searches *errorCount) *ErrorCountingLogger {
	return &ErrorCountingLogger{
		wrapped:  wrapped,
		emits:    emits,
		searches: searches,
	}
}

// Close calls [c.wrapped.Close]
func (c *ErrorCountingLogger) Close() error {
	return c.wrapped.Close()
}

// EmitAuditEvent calls [c.wrapped.EmitAuditEvent] and counts the error or
// success.
func (c *ErrorCountingLogger) EmitAuditEvent(ctx context.Context, e apievents.AuditEvent) error {
	err := c.wrapped.EmitAuditEvent(ctx, e)
	c.emits.observe(err)
	return err
}

// SearchEvents calls [c.wrapped.SearchEvents] and counts the error or
// success.
func (c *ErrorCountingLogger) SearchEvents(ctx context.Context, req events.SearchEventsRequest) ([]apievents.AuditEvent, string, error) {
	events, key, err := c.wrapped.SearchEvents(ctx, req)
	c.searches.observe(err)
	return events, key, err
}

func (c *ErrorCountingLogger) SearchUnstructuredEvents(ctx context.Context, req events.SearchEventsRequest) ([]*auditlogpb.EventUnstructured, string, error) {
	events, key, err := c.wrapped.SearchUnstructuredEvents(ctx, req)
	c.searches.observe(err)
	return events, key, err
}

func (c *ErrorCountingLogger) ExportUnstructuredEvents(ctx context.Context, req *auditlogpb.ExportUnstructuredEventsRequest) stream.Stream[*auditlogpb.ExportEventUnstructured] {
	return stream.MapErr(c.wrapped.ExportUnstructuredEvents(ctx, req), func(err error) error {
		c.searches.observe(err)
		return err
	})
}

func (c *ErrorCountingLogger) GetEventExportChunks(ctx context.Context, req *auditlogpb.GetEventExportChunksRequest) stream.Stream[*auditlogpb.EventExportChunk] {
	return stream.MapErr(c.wrapped.GetEventExportChunks(ctx, req), func(err error) error {
		c.searches.observe(err)
		return err
	})
}

// SearchSessionEvents calls [c.wrapped.SearchSessionEvents] and counts the error or
// success.
func (c *ErrorCountingLogger) SearchSessionEvents(ctx context.Context, req events.SearchSessionEventsRequest) ([]apievents.AuditEvent, string, error) {
	events, key, err := c.wrapped.SearchSessionEvents(ctx, req)
	c.searches.observe(err)
	return events, key, err
}

// ErrorCountingSessionHandler wraps a MultipartHandler and counts errors on all
// operations.
type ErrorCountingSessionHandler struct {
	wrapped events.MultipartHandler

	uploads   *errorCount
	downloads *errorCount
}

func newErrorCountingSessionHandler(wrapped events.MultipartHandler, uploads, downloads *errorCount) *ErrorCountingSessionHandler {
	return &ErrorCountingSessionHandler{
		wrapped:   wrapped,
		uploads:   uploads,
		downloads: downloads,
	}
}

// Upload calls [c.wrapped.Upload] and counts the error or success.
func (c *ErrorCountingSessionHandler) Upload(ctx context.Context, sessionID session.ID, reader io.Reader) (string, error) {
	res, err := c.wrapped.Upload(ctx, sessionID, reader)
	c.uploads.observe(err)
	return res, err
}

// Download calls [c.wrapped.Download] and counts the error or success.
func (c *ErrorCountingSessionHandler) Download(ctx context.Context, sessionID session.ID, writer io.WriterAt) error {
	err := c.wrapped.Download(ctx, sessionID, writer)
	c.downloads.observe(err)
	return err
}

// CreateUpload calls [c.wrapped.CreateUpload] and counts the error or success.
func (c *ErrorCountingSessionHandler) CreateUpload(ctx context.Context, sessionID session.ID) (*events.StreamUpload, error) {
	res, err := c.wrapped.CreateUpload(ctx, sessionID)
	c.uploads.observe(err)
	return res, err
}

// CompleteUpload calls [c.wrapped.CompleteUpload] and counts the error or success.
func (c *ErrorCountingSessionHandler) CompleteUpload(ctx context.Context, upload events.StreamUpload, parts []events.StreamPart) error {
	err := c.wrapped.CompleteUpload(ctx, upload, parts)
	c.uploads.observe(err)
	return err
}

// ReserveUploadPart calls [c.wrapped.ReserveUploadPart] and counts the error or success.
func (c *ErrorCountingSessionHandler) ReserveUploadPart(ctx context.Context, upload events.StreamUpload, partNumber int64) error {
	err := c.wrapped.ReserveUploadPart(ctx, upload, partNumber)
	c.uploads.observe(err)
	return err
}

// UploadPart calls [c.wrapped.UploadPart] and counts the error or success.
func (c *ErrorCountingSessionHandler) UploadPart(ctx context.Context, upload events.StreamUpload, partNumber int64, partBody io.ReadSeeker) (*events.StreamPart, error) {
	part, err := c.wrapped.UploadPart(ctx, upload, partNumber, partBody)
	c.uploads.observe(err)
	return part, err
}

// ListParts calls [c.wrapped.ListParts] and counts the error or success.
func (c *ErrorCountingSessionHandler) ListParts(ctx context.Context, upload events.StreamUpload) ([]events.StreamPart, error) {
	parts, err := c.wrapped.ListParts(ctx, upload)
	c.uploads.observe(err)
	return parts, err
}

// ListUploads calls [c.wrapped.ListUploads] and counts the error or success.
func (c *ErrorCountingSessionHandler) ListUploads(ctx context.Context) ([]events.StreamUpload, error) {
	uploads, err := c.wrapped.ListUploads(ctx)
	c.uploads.observe(err)
	return uploads, err
}

// GetUploadMetadata calls [c.wrapped.GetUploadMetadata] and counts the error or success.
func (c *ErrorCountingSessionHandler) GetUploadMetadata(sessionID session.ID) events.UploadMetadata {
	return c.wrapped.GetUploadMetadata(sessionID)
}

func sanitizeErrForAlert(err error) string {
	return strings.Map(func(r rune) rune {
		// Cluster alerts do not allow control characters.
		if !unicode.IsPrint(r) {
			return ' '
		}
		return r
	}, truncateErrForAlert(err))
}

func truncateErrForAlert(err error) string {
	s := err.Error()
	const maxLength = 256 // arbitrary
	if len(s) < maxLength {
		return s
	}
	return s[:maxLength]
}
