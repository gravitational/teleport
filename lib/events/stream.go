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

package events

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/summarizer"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// Int32Size is a constant for 32 bit integer byte size
	Int32Size = 4

	// Int64Size is a constant for 64 bit integer byte size
	Int64Size = 8

	// ConcurrentUploadsPerStream limits the amount of concurrent uploads
	// per stream
	ConcurrentUploadsPerStream = 1

	// MaxProtoMessageSizeBytes is maximum protobuf marshaled message size
	MaxProtoMessageSizeBytes = 64 * 1024

	// MinUploadPartSizeBytes is the minimum allowed part size when uploading a part to
	// Amazon S3.
	MinUploadPartSizeBytes = 1024 * 1024 * 5

	// ProtoStreamV1 is a version of the binary protocol
	ProtoStreamV1 = 1

	// ProtoStreamV1PartHeaderSize is the size of the part of the protocol stream
	// on disk format, it consists of
	// * 8 bytes for the format version
	// * 8 bytes for meaningful size of the part
	// * 8 bytes for optional padding size at the end of the slice
	ProtoStreamV1PartHeaderSize = Int64Size * 3

	// ProtoStreamV1RecordHeaderSize is the size of the header
	// of the record header, it consists of the record length
	ProtoStreamV1RecordHeaderSize = Int32Size

	// uploaderReservePartErrorMessage error message present when
	// `ReserveUploadPart` fails.
	uploaderReservePartErrorMessage = "uploader failed to reserve upload part"
)

// ProtoStreamerConfig specifies configuration for the part
type ProtoStreamerConfig struct {
	Uploader MultipartUploader
	// MinUploadBytes submits upload when they have reached min bytes (could be more,
	// but not less), due to the nature of gzip writer
	MinUploadBytes int64
	// ConcurrentUploads sets concurrent uploads per stream
	ConcurrentUploads int
	// ForceFlush is used in tests to force a flush of an in-progress slice. Note that
	// sending on this channel just forces a single flush for whichever upload happens
	// to receive the signal first, so this may not be suitable for concurrent tests.
	ForceFlush chan struct{}
	// RetryConfig defines how to retry on a failed upload
	RetryConfig *retryutils.LinearConfig
}

// CheckAndSetDefaults checks and sets streamer defaults
func (cfg *ProtoStreamerConfig) CheckAndSetDefaults() error {
	if cfg.Uploader == nil {
		return trace.BadParameter("missing parameter Uploader")
	}
	if cfg.MinUploadBytes == 0 {
		cfg.MinUploadBytes = MinUploadPartSizeBytes
	}
	if cfg.ConcurrentUploads == 0 {
		cfg.ConcurrentUploads = ConcurrentUploadsPerStream
	}
	return nil
}

// NewProtoStreamer creates protobuf-based streams
func NewProtoStreamer(cfg ProtoStreamerConfig) (*ProtoStreamer, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &ProtoStreamer{
		cfg: cfg,
		// Min upload bytes + some overhead to prevent buffer growth (gzip writer is not precise)
		bufferPool: utils.NewBufferSyncPool(cfg.MinUploadBytes + cfg.MinUploadBytes/3),
		// MaxProtoMessage size + length of the message record
		slicePool: utils.NewSliceSyncPool(MaxProtoMessageSizeBytes + ProtoStreamV1RecordHeaderSize),
	}, nil
}

// ProtoStreamer creates protobuf-based streams uploaded to the storage
// backends, for example S3 or GCS
type ProtoStreamer struct {
	cfg        ProtoStreamerConfig
	bufferPool *utils.BufferSyncPool
	slicePool  *utils.SliceSyncPool
}

// CreateAuditStreamForUpload creates audit stream for existing upload,
// this function is useful in tests
func (s *ProtoStreamer) CreateAuditStreamForUpload(ctx context.Context, sid session.ID, upload StreamUpload) (apievents.Stream, error) {
	return NewProtoStream(ProtoStreamConfig{
		Upload:            upload,
		BufferPool:        s.bufferPool,
		SlicePool:         s.slicePool,
		Uploader:          s.cfg.Uploader,
		MinUploadBytes:    s.cfg.MinUploadBytes,
		ConcurrentUploads: s.cfg.ConcurrentUploads,
		ForceFlush:        s.cfg.ForceFlush,
		RetryConfig:       s.cfg.RetryConfig,
	})
}

// CreateAuditStream creates audit stream and upload
func (s *ProtoStreamer) CreateAuditStream(ctx context.Context, sid session.ID) (apievents.Stream, error) {
	upload, err := s.cfg.Uploader.CreateUpload(ctx, sid)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return s.CreateAuditStreamForUpload(ctx, sid, *upload)
}

// ResumeAuditStream resumes the stream that has not been completed yet
func (s *ProtoStreamer) ResumeAuditStream(ctx context.Context, sid session.ID, uploadID string) (apievents.Stream, error) {
	// Note, that if the session ID does not match the upload ID,
	// the request will fail
	upload := StreamUpload{SessionID: sid, ID: uploadID}
	parts, err := s.cfg.Uploader.ListParts(ctx, upload)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return NewProtoStream(ProtoStreamConfig{
		Upload:         upload,
		BufferPool:     s.bufferPool,
		SlicePool:      s.slicePool,
		Uploader:       s.cfg.Uploader,
		MinUploadBytes: s.cfg.MinUploadBytes,
		CompletedParts: parts,
		RetryConfig:    s.cfg.RetryConfig,
	})
}

// ProtoStreamConfig configures proto stream
type ProtoStreamConfig struct {
	// Upload is the upload this stream is handling
	Upload StreamUpload
	// Uploader handles upload to the storage
	Uploader MultipartUploader
	// BufferPool is a sync pool with buffers
	BufferPool *utils.BufferSyncPool
	// SlicePool is a sync pool with allocated slices
	SlicePool *utils.SliceSyncPool
	// MinUploadBytes submits upload when they have reached min bytes (could be more,
	// but not less), due to the nature of gzip writer
	MinUploadBytes int64
	// CompletedParts is a list of completed parts, used for resuming stream
	CompletedParts []StreamPart
	// InactivityFlushPeriod sets inactivity period
	// after which streamer flushes the data to the uploader
	// to avoid data loss
	InactivityFlushPeriod time.Duration
	// ForceFlush is used in tests to force a flush of an in-progress slice. Note that
	// sending on this channel just forces a single flush for whichever upload happens
	// to receive the signal first, so this may not be suitable for concurrent tests.
	ForceFlush chan struct{}
	// Clock is used to override time in tests
	Clock clockwork.Clock
	// ConcurrentUploads sets concurrent uploads per stream
	ConcurrentUploads int
	// RetryConfig defines how to retry on a failed upload
	RetryConfig *retryutils.LinearConfig
}

// CheckAndSetDefaults checks and sets default values
func (cfg *ProtoStreamConfig) CheckAndSetDefaults() error {
	if err := cfg.Upload.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if cfg.Uploader == nil {
		return trace.BadParameter("missing parameter Uploader")
	}
	if cfg.BufferPool == nil {
		return trace.BadParameter("missing parameter BufferPool")
	}
	if cfg.SlicePool == nil {
		return trace.BadParameter("missing parameter SlicePool")
	}
	if cfg.MinUploadBytes == 0 {
		return trace.BadParameter("missing parameter MinUploadBytes")
	}
	if cfg.InactivityFlushPeriod == 0 {
		cfg.InactivityFlushPeriod = InactivityFlushPeriod
	}
	if cfg.ConcurrentUploads == 0 {
		cfg.ConcurrentUploads = ConcurrentUploadsPerStream
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.RetryConfig == nil {
		cfg.RetryConfig = &retryutils.LinearConfig{
			Step: NetworkRetryDuration,
			Max:  NetworkBackoffDuration,
		}
	}
	return nil
}

// NewProtoStream uploads session recordings in the protobuf format.
//
// The individual session stream is represented by continuous globally
// ordered sequence of events serialized to binary protobuf format.
//
// The stream is split into ordered slices of gzipped audit events.
//
// Each slice is composed of three parts:
//
// 1. Slice starts with 24 bytes version header
//
// * 8 bytes for the format version (used for future expansion)
// * 8 bytes for meaningful size of the part
// * 8 bytes for padding at the end of the slice (if present)
//
// 2. V1 body of the slice is gzipped protobuf messages in binary format.
//
// 3. Optional padding (if specified in the header), required
// to bring slices to minimum slice size.
//
// The slice size is determined by S3 multipart upload requirements:
//
// https://docs.aws.amazon.com/AmazonS3/latest/dev/qfacts.html
//
// This design allows the streamer to upload slices using S3-compatible APIs
// in parallel without buffering to disk.
func NewProtoStream(cfg ProtoStreamConfig) (*ProtoStream, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	cancelCtx, cancel := context.WithCancel(context.Background())
	completeCtx, complete := context.WithCancel(context.Background())
	stream := &ProtoStream{
		cfg:      cfg,
		eventsCh: make(chan protoEvent),

		cancelCtx: cancelCtx,
		cancel:    cancel,
		cancelMtx: &sync.RWMutex{},

		completeCtx:      completeCtx,
		complete:         complete,
		completeMtx:      &sync.RWMutex{},
		uploadLoopDoneCh: make(chan struct{}),

		// Buffered channel gives consumers
		// a chance to get an early status update.
		statusCh: make(chan apievents.StreamStatus, 1),
	}

	writer := &sliceWriter{
		proto:             stream,
		activeUploads:     make(map[int64]*activeUpload),
		completedUploadsC: make(chan *activeUpload, cfg.ConcurrentUploads),
		semUploads:        make(chan struct{}, cfg.ConcurrentUploads),
		lastPartNumber:    0,
		retryConfig:       *cfg.RetryConfig,
	}
	if len(cfg.CompletedParts) > 0 {
		// skip 2 extra parts as a protection from accidental overwrites.
		// the following is possible between processes 1 and 2 (P1 and P2)
		// P1: * start stream S
		// P1: * receive some data from stream S
		// C:  * disconnect from P1
		// P2: * resume stream, get all committed parts (0) and start writes
		// P2: * write part 1
		// P1: * flush the data to part 1 before closure
		//
		// In this scenario stream data submitted by P1 flush will be lost
		// unless resume will resume at part 2.
		//
		// On the other hand, it's ok if resume of P2 overwrites
		// any data of P1, because it will replay non committed
		// events, which could potentially lead to duplicate events.
		writer.lastPartNumber = cfg.CompletedParts[len(cfg.CompletedParts)-1].Number + 1
		writer.completedParts = cfg.CompletedParts
	}

	// Generate the first slice. This is done in the initialization process to
	// return any critical errors synchronously instead of having to emit the
	// first event.
	var err error
	writer.current, err = writer.newSlice()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Start writer events receiver.
	go func() {
		if err := writer.receiveAndUpload(); err != nil {
			slog.DebugContext(cancelCtx, "slice writer ended with error", "error", err)
			stream.setCancelError(err)
		}

		stream.cancel()
	}()

	return stream, nil
}

// ProtoStream implements concurrent safe event emitter
// that uploads the parts in parallel to S3
type ProtoStream struct {
	cfg ProtoStreamConfig

	eventsCh chan protoEvent

	// cancelCtx is used to signal closure
	cancelCtx context.Context
	cancel    context.CancelFunc
	cancelErr error
	cancelMtx *sync.RWMutex

	// completeCtx is used to signal completion of the operation
	completeCtx    context.Context
	complete       context.CancelFunc
	completeType   atomic.Uint32
	completeResult error
	completeMtx    *sync.RWMutex

	// uploadLoopDoneCh is closed when the slice exits the upload loop.
	// The exit might be an indication of completion or a cancelation
	uploadLoopDoneCh chan struct{}

	// statusCh sends updates on the stream status
	statusCh chan apievents.StreamStatus
}

const (
	// completeTypeComplete means that proto stream
	// should complete all in flight uploads and complete the upload itself
	completeTypeComplete = 0
	// completeTypeFlush means that proto stream
	// should complete all in flight uploads but do not complete the upload
	completeTypeFlush = 1
)

type protoEvent struct {
	index int64
	oneof *apievents.OneOf
}

func (s *ProtoStream) setCompleteResult(err error) {
	s.completeMtx.Lock()
	defer s.completeMtx.Unlock()
	s.completeResult = err
}

func (s *ProtoStream) getCompleteResult() error {
	s.completeMtx.RLock()
	defer s.completeMtx.RUnlock()
	return s.completeResult
}

// Done returns channel closed when streamer is closed
// should be used to detect sending errors
func (s *ProtoStream) Done() <-chan struct{} {
	return s.cancelCtx.Done()
}

// RecordEvent emits a single audit event to the stream
func (s *ProtoStream) RecordEvent(ctx context.Context, pe apievents.PreparedSessionEvent) error {
	event := pe.GetAuditEvent()
	messageSize := event.Size()
	if messageSize > MaxProtoMessageSizeBytes {
		event = event.TrimToMaxSize(MaxProtoMessageSizeBytes)
		if event.Size() > MaxProtoMessageSizeBytes {
			return trace.BadParameter("record size %v exceeds max message size of %v bytes", messageSize, MaxProtoMessageSizeBytes)
		}
	}

	oneof, err := apievents.ToOneOf(event)
	if err != nil {
		return trace.Wrap(err)
	}

	start := time.Now()
	select {
	case s.eventsCh <- protoEvent{index: event.GetIndex(), oneof: oneof}:
		diff := time.Since(start)
		if diff > 100*time.Millisecond {
			slog.DebugContext(ctx, "[SLOW] RecordEvent took", "duration", diff)
		}
		return nil
	case <-s.cancelCtx.Done():
		cancelErr := s.getCancelError()
		if cancelErr != nil {
			return trace.Wrap(cancelErr)
		}

		return trace.ConnectionProblem(s.cancelCtx.Err(), "emitter has been closed")
	case <-s.completeCtx.Done():
		return trace.ConnectionProblem(nil, "emitter is completed")
	case <-ctx.Done():
		return trace.ConnectionProblem(ctx.Err(), "context is closed")
	}
}

// Complete completes the upload, waits for completion and returns all allocated resources.
func (s *ProtoStream) Complete(ctx context.Context) error {
	s.complete()
	select {
	case <-s.uploadLoopDoneCh:
		s.cancel()
		go s.summarize()
		return s.getCompleteResult()
	case <-ctx.Done():
		return trace.ConnectionProblem(ctx.Err(), "context has canceled before complete could succeed")
	}
}

func (s *ProtoStream) summarize() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	summary, err := summarizer.Summarize(ctx, s.cfg.Upload.SessionID)
	if err != nil {
		slog.ErrorContext(ctx, "=============== Summarization error", "error", err)
		return
	}

	slog.DebugContext(ctx, "===================== SUMMARY =================", "summary", summary)
}

// Status returns channel receiving updates about stream status
// last event index that was uploaded and upload ID
func (s *ProtoStream) Status() <-chan apievents.StreamStatus {
	return s.statusCh
}

// Close flushes non-uploaded flight stream data without marking
// the stream completed and closes the stream instance
func (s *ProtoStream) Close(ctx context.Context) error {
	s.completeType.Store(completeTypeFlush)
	s.complete()
	select {
	case <-s.uploadLoopDoneCh:
		return ctx.Err()
	case <-ctx.Done():
		return trace.ConnectionProblem(ctx.Err(), "context has canceled before complete could succeed")
	}
}

// setCancelError sets the cancelErr with lock.
func (s *ProtoStream) setCancelError(err error) {
	s.cancelMtx.Lock()
	defer s.cancelMtx.Unlock()
	s.cancelErr = err
}

// getCancelError gets the cancelErr with lock.
func (s *ProtoStream) getCancelError() error {
	s.cancelMtx.RLock()
	defer s.cancelMtx.RUnlock()
	return s.cancelErr
}

// sliceWriter is a helper struct that coordinates
// writing slices and checkpointing
type sliceWriter struct {
	proto *ProtoStream
	// current is the current slice being written to
	current *slice
	// lastPartNumber is the last assigned part number
	lastPartNumber int64
	// activeUploads tracks active uploads
	activeUploads map[int64]*activeUpload
	// completedUploadsC receives uploads that have been completed
	completedUploadsC chan *activeUpload
	// semUploads controls concurrent uploads that are in flight
	semUploads chan struct{}
	// completedParts is the list of completed parts
	completedParts []StreamPart
	// emptyHeader is used to write empty header
	// to preserve some bytes
	emptyHeader [ProtoStreamV1PartHeaderSize]byte
	// retryConfig  defines how to retry on a failed upload
	retryConfig retryutils.LinearConfig
}

func (w *sliceWriter) updateCompletedParts(part StreamPart, lastEventIndex int64) {
	w.completedParts = append(w.completedParts, part)
	w.trySendStreamStatusUpdate(lastEventIndex)
}

func (w *sliceWriter) trySendStreamStatusUpdate(lastEventIndex int64) {
	status := apievents.StreamStatus{
		UploadID:       w.proto.cfg.Upload.ID,
		LastEventIndex: lastEventIndex,
		LastUploadTime: w.proto.cfg.Clock.Now().UTC(),
	}
	select {
	case w.proto.statusCh <- status:
	default:
	}
}

// receiveAndUpload receives and uploads serialized events
func (w *sliceWriter) receiveAndUpload() error {
	defer close(w.proto.uploadLoopDoneCh)
	// on the start, send stream status with the upload ID and negative
	// index so that remote party can get an upload ID
	w.trySendStreamStatusUpdate(-1)

	clock := w.proto.cfg.Clock

	var lastEvent time.Time
	var flushCh <-chan time.Time
	for {
		select {
		case <-w.proto.cancelCtx.Done():
			// cancel stops all operations without waiting
			return nil
		case <-w.proto.completeCtx.Done():
			// if present, send remaining data for upload
			if w.current != nil && !w.current.isEmpty() {
				// mark that the current part is last (last parts are allowed to be
				// smaller than the certain size, otherwise the padding
				// have to be added (this is due to S3 API limits)
				if w.proto.completeType.Load() == completeTypeComplete {
					w.current.isLast = true
				}
				if err := w.startUploadCurrentSlice(); err != nil {
					return trace.Wrap(err)
				}
			}

			w.completeStream()
			return nil
		case upload := <-w.completedUploadsC:
			part, err := upload.getPart()
			if err != nil {
				return trace.Wrap(err)
			}

			delete(w.activeUploads, part.Number)
			w.updateCompletedParts(*part, upload.lastEventIndex)
		case <-w.proto.cfg.ForceFlush:
			if w.current != nil {
				if err := w.startUploadCurrentSlice(); err != nil {
					return trace.Wrap(err)
				}
			}
		case <-flushCh:
			now := clock.Now().UTC()
			inactivityPeriod := now.Sub(lastEvent)
			if inactivityPeriod < 0 {
				inactivityPeriod = 0
			}
			if inactivityPeriod >= w.proto.cfg.InactivityFlushPeriod {
				// inactivity period exceeded threshold,
				// there is no need to schedule a timer until the next
				// event occurs, set the timer channel to nil
				flushCh = nil
				if w.current != nil && !w.current.isEmpty() {
					slog.DebugContext(w.proto.completeCtx, "Inactivity timer ticked and exceeded threshold and have data. Flushing.", "tick", now, "inactivity_period", inactivityPeriod)
					if err := w.startUploadCurrentSlice(); err != nil {
						return trace.Wrap(err)
					}
				} else {
					slog.DebugContext(w.proto.completeCtx, "Inactivity timer ticked and exceeded threshold but have no data. Nothing to do.", "tick", now, "inactivity_period", inactivityPeriod)
				}
			} else {
				slog.DebugContext(w.proto.completeCtx, "Inactivity timer ticked and did not exceeded threshold. Resetting ticker.", "tick", now, "inactivity_period", inactivityPeriod, "next_tick", w.proto.cfg.InactivityFlushPeriod-inactivityPeriod)
				flushCh = clock.After(w.proto.cfg.InactivityFlushPeriod - inactivityPeriod)
			}
		case event := <-w.proto.eventsCh:
			lastEvent = clock.Now().UTC()
			// flush timer is set up only if any event was submitted
			// after last flush or system start
			if flushCh == nil {
				flushCh = clock.After(w.proto.cfg.InactivityFlushPeriod)
			}
			if err := w.submitEvent(event); err != nil {
				slog.ErrorContext(w.proto.cancelCtx, "Lost event.", "error", err)
				// Failure on `newSlice` indicates that the streamer won't be
				// able to process events. Close the streamer and set the
				// returned error so that event emitters can proceed.
				if isReserveUploadPartError(err) {
					return trace.Wrap(err)
				}

				continue
			}
			if w.shouldUploadCurrentSlice() {
				// this logic blocks the EmitAuditEvent in case if the
				// upload has not completed and the current slice is out of capacity
				if err := w.startUploadCurrentSlice(); err != nil {
					return trace.Wrap(err)
				}
			}
		}
	}
}

// shouldUploadCurrentSlice returns true when it's time to upload
// the current slice (it has reached upload bytes)
func (w *sliceWriter) shouldUploadCurrentSlice() bool {
	return w.current.shouldUpload()
}

// startUploadCurrentSlice starts uploading current slice
// and adds it to the waiting list
func (w *sliceWriter) startUploadCurrentSlice() error {
	activeUpload, err := w.startUpload(w.lastPartNumber, w.current)
	if err != nil {
		return trace.Wrap(err)
	}
	w.activeUploads[w.lastPartNumber] = activeUpload
	w.current = nil
	w.lastPartNumber++
	return nil
}

type bufferCloser struct {
	*bytes.Buffer
}

func (b *bufferCloser) Close() error {
	return nil
}

func (w *sliceWriter) newSlice() (*slice, error) {
	w.lastPartNumber++
	// This buffer will be returned to the pool by slice.Close
	buffer := w.proto.cfg.BufferPool.Get()
	buffer.Reset()
	// reserve bytes for version header
	buffer.Write(w.emptyHeader[:])

	err := w.proto.cfg.Uploader.ReserveUploadPart(w.proto.cancelCtx, w.proto.cfg.Upload, w.lastPartNumber)
	if err != nil {
		// Return the unused buffer to the pool.
		w.proto.cfg.BufferPool.Put(buffer)
		return nil, trace.ConnectionProblem(err, uploaderReservePartErrorMessage)
	}

	return &slice{
		proto:  w.proto,
		buffer: buffer,
		writer: newGzipWriter(&bufferCloser{Buffer: buffer}),
	}, nil
}

func (w *sliceWriter) submitEvent(event protoEvent) error {
	if w.current == nil {
		var err error
		w.current, err = w.newSlice()
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return w.current.recordEvent(event)
}

// completeStream waits for in-flight uploads to finish
// and completes the stream
func (w *sliceWriter) completeStream() {
	for range w.activeUploads {
		select {
		case upload := <-w.completedUploadsC:
			part, err := upload.getPart()
			if err != nil {
				slog.WarnContext(w.proto.cancelCtx, "Failed to upload part",
					"error", err,
					"upload", w.proto.cfg.Upload.ID,
					"session", w.proto.cfg.Upload.SessionID,
				)
				continue
			}
			w.updateCompletedParts(*part, upload.lastEventIndex)
		case <-w.proto.cancelCtx.Done():
			return
		}
	}
	if w.proto.completeType.Load() == completeTypeComplete {
		// part upload notifications could arrive out of order
		sort.Slice(w.completedParts, func(i, j int) bool {
			return w.completedParts[i].Number < w.completedParts[j].Number
		})
		err := w.proto.cfg.Uploader.CompleteUpload(w.proto.cancelCtx, w.proto.cfg.Upload, w.completedParts)
		w.proto.setCompleteResult(err)
		if err != nil {
			slog.WarnContext(w.proto.cancelCtx, "Failed to complete upload",
				"error", err,
				"upload", w.proto.cfg.Upload.ID,
				"session", w.proto.cfg.Upload.SessionID,
			)
		}
	}
}

// startUpload acquires upload semaphore and starts upload, returns error
// only if there is a critical error
func (w *sliceWriter) startUpload(partNumber int64, slice *slice) (*activeUpload, error) {
	// acquire semaphore limiting concurrent uploads
	select {
	case w.semUploads <- struct{}{}:
	case <-w.proto.cancelCtx.Done():
		return nil, trace.ConnectionProblem(w.proto.cancelCtx.Err(), "context is closed")
	}
	activeUpload := &activeUpload{
		partNumber:     partNumber,
		lastEventIndex: slice.lastEventIndex,
		start:          time.Now().UTC(),
	}

	go func() {
		defer func() {
			if err := slice.Close(); err != nil {
				slog.WarnContext(w.proto.cancelCtx, "Failed to close slice.", "error", err)
			}
		}()

		defer func() {
			select {
			case w.completedUploadsC <- activeUpload:
			case <-w.proto.cancelCtx.Done():
				return
			}
		}()

		defer func() {
			<-w.semUploads
		}()

		log := slog.With(
			"part", partNumber,
			"upload", w.proto.cfg.Upload.ID,
			"session", w.proto.cfg.Upload.SessionID,
		)

		var retry retryutils.Retry

		// create reader once before the retry loop. in the event of an error, the reader must
		// be reset via Seek rather than recreated.
		reader, err := slice.reader()
		if err != nil {
			activeUpload.setError(err)
			return
		}

		for i := 0; i < defaults.MaxIterationLimit; i++ {
			log := log.With("attempt", i)

			part, err := w.proto.cfg.Uploader.UploadPart(w.proto.cancelCtx, w.proto.cfg.Upload, partNumber, reader)
			if err == nil {
				activeUpload.setPart(*part)
				return
			}

			log.WarnContext(w.proto.cancelCtx, "failed to upload part", "error", err)

			// upload is not found is not a transient error, so abort the operation
			if errors.Is(trace.Unwrap(err), context.Canceled) || trace.IsNotFound(err) {
				log.InfoContext(w.proto.cancelCtx, "aborting part upload")
				activeUpload.setError(err)
				return
			}
			log.InfoContext(w.proto.cancelCtx, "will retry part upload")

			// retry is created on the first upload error
			if retry == nil {
				var rerr error
				retry, rerr = retryutils.NewLinear(w.retryConfig)
				if rerr != nil {
					activeUpload.setError(rerr)
					return
				}
			}
			retry.Inc()

			// reset reader to the beginning of the slice so it can be re-read
			if _, err := reader.Seek(0, 0); err != nil {
				activeUpload.setError(err)
				return
			}

			select {
			case <-retry.After():
				log.DebugContext(w.proto.cancelCtx, "Back off period for retry has passed. Retrying", "error", err)
			case <-w.proto.cancelCtx.Done():
				return
			}
		}
	}()

	return activeUpload, nil
}

type activeUpload struct {
	mtx            sync.RWMutex
	start          time.Time
	end            time.Time
	partNumber     int64
	part           *StreamPart
	err            error
	lastEventIndex int64
}

func (a *activeUpload) setError(err error) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	a.end = time.Now().UTC()
	a.err = err
}

func (a *activeUpload) setPart(part StreamPart) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	a.end = time.Now().UTC()
	a.part = &part
}

func (a *activeUpload) getPart() (*StreamPart, error) {
	a.mtx.RLock()
	defer a.mtx.RUnlock()
	if a.err != nil {
		return nil, trace.Wrap(a.err)
	}
	if a.part == nil {
		return nil, trace.NotFound("part is not set")
	}
	return a.part, nil
}

// slice contains serialized protobuf messages
type slice struct {
	proto          *ProtoStream
	writer         *gzipWriter
	buffer         *bytes.Buffer
	isLast         bool
	lastEventIndex int64
	eventCount     uint64
}

// reader returns a reader for the bytes written, no writes should be done after this
// method is called and this method should be called at most once per slice, otherwise
// the resulting recording will be corrupted.
func (s *slice) reader() (io.ReadSeeker, error) {
	if err := s.writer.Close(); err != nil {
		return nil, trace.Wrap(err)
	}
	wroteBytes := int64(s.buffer.Len())
	var paddingBytes int64
	// non last slices should be at least min upload bytes (as limited by S3 API spec)
	if !s.isLast && wroteBytes < s.proto.cfg.MinUploadBytes {
		paddingBytes = s.proto.cfg.MinUploadBytes - wroteBytes
		s.buffer.Grow(int(paddingBytes))
		padding := s.buffer.AvailableBuffer()[:paddingBytes]
		clear(padding)
		s.buffer.Write(padding)
	}
	data := s.buffer.Bytes()
	// when the slice was created, the first bytes were reserved
	// for the protocol version number and size of the slice in bytes
	binary.BigEndian.PutUint64(data[0:], ProtoStreamV1)
	binary.BigEndian.PutUint64(data[Int64Size:], uint64(wroteBytes-ProtoStreamV1PartHeaderSize))
	binary.BigEndian.PutUint64(data[Int64Size*2:], uint64(paddingBytes))
	return bytes.NewReader(data), nil
}

// Close closes buffer and returns all allocated resources
func (s *slice) Close() error {
	err := s.writer.Close()
	s.proto.cfg.BufferPool.Put(s.buffer)
	s.buffer = nil
	return trace.Wrap(err)
}

// shouldUpload returns true if it's time to write the slice
// (set to true when it has reached the min slice in bytes)
func (s *slice) shouldUpload() bool {
	return int64(s.buffer.Len()) >= s.proto.cfg.MinUploadBytes
}

// isEmpty returns true if the slice hasn't had any events written to
// it yet.
func (s *slice) isEmpty() bool {
	return s.eventCount == 0
}

// recordEvent emits a single session event to the stream
func (s *slice) recordEvent(event protoEvent) error {
	bytes := s.proto.cfg.SlicePool.Get()
	defer s.proto.cfg.SlicePool.Put(bytes)

	s.eventCount++

	messageSize := event.oneof.Size()
	recordSize := ProtoStreamV1RecordHeaderSize + messageSize

	if len(bytes) < recordSize {
		return trace.BadParameter(
			"error in buffer allocation, expected size to be >= %v, got %v", recordSize, len(bytes))
	}

	binary.BigEndian.PutUint32(bytes, uint32(messageSize))
	_, err := event.oneof.MarshalTo(bytes[Int32Size:])
	if err != nil {
		return trace.Wrap(err)
	}
	wroteBytes, err := s.writer.Write(bytes[:recordSize])
	if err != nil {
		return trace.Wrap(err)
	}
	if wroteBytes != recordSize {
		return trace.BadParameter("expected %v bytes to be written, got %v", recordSize, wroteBytes)
	}
	if event.index > s.lastEventIndex {
		s.lastEventIndex = event.index
	}
	return nil
}

// NewProtoReader returns a new proto reader with slice pool
func NewProtoReader(r io.Reader) *ProtoReader {
	return &ProtoReader{
		reader:    r,
		lastIndex: -1,
	}
}

// SessionReader provides method to read
// session events one by one
type SessionReader interface {
	// Read reads session events
	Read(context.Context) (apievents.AuditEvent, error)
}

const (
	// protoReaderStateInit is ready to start reading the next part
	protoReaderStateInit = 0
	// protoReaderStateCurrent will read the data from the current part
	protoReaderStateCurrent = iota
	// protoReaderStateEOF indicates that reader has completed reading
	// all parts
	protoReaderStateEOF = iota
	// protoReaderStateError indicates that reader has reached internal
	// error and should close
	protoReaderStateError = iota
)

// ProtoReader reads protobuf encoding from reader
type ProtoReader struct {
	gzipReader   *gzipReader
	padding      int64
	reader       io.Reader
	sizeBytes    [Int64Size]byte
	messageBytes [MaxProtoMessageSizeBytes]byte
	state        int
	error        error
	lastIndex    int64
	stats        ProtoReaderStats
}

// ProtoReaderStats contains some reader statistics
type ProtoReaderStats struct {
	// SkippedEvents is a counter with encountered
	// events recorded several times or events
	// that have been out of order as skipped
	SkippedEvents int64
	// OutOfOrderEvents is a counter with events
	// received out of order
	OutOfOrderEvents int64
	// TotalEvents contains total amount of
	// processed events (including duplicates)
	TotalEvents int64
}

// ToFields returns a copy of the stats to be used as log fields
func (p ProtoReaderStats) ToFields() map[string]any {
	return map[string]any{
		"skipped-events":      p.SkippedEvents,
		"out-of-order-events": p.OutOfOrderEvents,
		"total-events":        p.TotalEvents,
	}
}

// Close releases reader resources
func (r *ProtoReader) Close() error {
	if r.gzipReader != nil {
		return r.gzipReader.Close()
	}
	return nil
}

// Reset sets reader to read from the new reader
// without resetting the stats, could be used
// to deduplicate the events
func (r *ProtoReader) Reset(reader io.Reader) error {
	if r.error != nil {
		return r.error
	}
	if r.gzipReader != nil {
		if r.error = r.gzipReader.Close(); r.error != nil {
			return trace.Wrap(r.error)
		}
		r.gzipReader = nil
	}
	r.reader = reader
	r.state = protoReaderStateInit
	return nil
}

func (r *ProtoReader) setError(err error) error {
	r.state = protoReaderStateError
	r.error = err
	return err
}

// GetStats returns stats about processed events
func (r *ProtoReader) GetStats() ProtoReaderStats {
	return r.stats
}

// Read returns next event or io.EOF in case of the end of the parts
func (r *ProtoReader) Read(ctx context.Context) (apievents.AuditEvent, error) {
	// periodic checks of context after fixed amount of iterations
	// is an extra precaution to avoid
	// accidental endless loop due to logic error crashing the system
	// and allows ctx timeout to kick in if specified
	var checkpointIteration int64
	for {
		checkpointIteration++
		if checkpointIteration%defaults.MaxIterationLimit == 0 {
			select {
			case <-ctx.Done():
				if ctx.Err() != nil {
					return nil, trace.Wrap(ctx.Err())
				}
				return nil, trace.LimitExceeded("context has been canceled")
			default:
			}
		}
		switch r.state {
		case protoReaderStateEOF:
			return nil, io.EOF
		case protoReaderStateError:
			return nil, r.error
		case protoReaderStateInit:
			// read the part header that consists of the protocol version
			// and the part size (for the V1 version of the protocol)
			_, err := io.ReadFull(r.reader, r.sizeBytes[:Int64Size])
			if err != nil {
				// reached the end of the stream
				if errors.Is(err, io.EOF) {
					r.state = protoReaderStateEOF
					return nil, err
				}
				return nil, r.setError(trace.ConvertSystemError(err))
			}
			protocolVersion := binary.BigEndian.Uint64(r.sizeBytes[:Int64Size])
			if protocolVersion != ProtoStreamV1 {
				return nil, trace.BadParameter("unsupported protocol version %v", protocolVersion)
			}
			// read size of this gzipped part as encoded by V1 protocol version
			_, err = io.ReadFull(r.reader, r.sizeBytes[:Int64Size])
			if err != nil {
				return nil, r.setError(trace.ConvertSystemError(err))
			}
			partSize := binary.BigEndian.Uint64(r.sizeBytes[:Int64Size])
			// read padding size (could be 0)
			_, err = io.ReadFull(r.reader, r.sizeBytes[:Int64Size])
			if err != nil {
				return nil, r.setError(trace.ConvertSystemError(err))
			}
			r.padding = int64(binary.BigEndian.Uint64(r.sizeBytes[:Int64Size]))
			gzipReader, err := newGzipReader(io.NopCloser(io.LimitReader(r.reader, int64(partSize))))
			if err != nil {
				return nil, r.setError(trace.Wrap(err))
			}
			r.gzipReader = gzipReader
			r.state = protoReaderStateCurrent
			continue
			// read the next version from the gzip reader
		case protoReaderStateCurrent:
			// the record consists of length of the protobuf encoded
			// message and the message itself
			_, err := io.ReadFull(r.gzipReader, r.sizeBytes[:Int32Size])
			if err != nil {
				if !errors.Is(err, io.EOF) {
					return nil, r.setError(trace.ConvertSystemError(err))
				}

				// due to a bug in older versions of teleport it was possible that padding
				// bytes would end up inside of the gzip section of the archive. we should
				// skip any dangling data in the gzip secion.
				n, err := io.CopyBuffer(io.Discard, r.gzipReader.inner, r.messageBytes[:])
				if err != nil {
					return nil, r.setError(trace.ConvertSystemError(err))
				}

				if n != 0 {
					// log the number of bytes that were skipped
					slog.DebugContext(ctx, "skipped dangling data in session recording section", "length", n)
				}

				// reached the end of the current part, but not necessarily
				// the end of the stream
				if err := r.gzipReader.Close(); err != nil {
					return nil, r.setError(trace.ConvertSystemError(err))
				}
				if r.padding != 0 {
					skipped, err := io.CopyBuffer(io.Discard, io.LimitReader(r.reader, r.padding), r.messageBytes[:])
					if err != nil {
						return nil, r.setError(trace.ConvertSystemError(err))
					}
					if skipped != r.padding {
						return nil, r.setError(trace.BadParameter(
							"data truncated, expected to read %v bytes, but got %v", r.padding, skipped))
					}
				}
				r.padding = 0
				r.gzipReader = nil
				r.state = protoReaderStateInit
				continue
			}
			messageSize := binary.BigEndian.Uint32(r.sizeBytes[:Int32Size])
			// zero message size indicates end of the part
			// that sometimes is present in partially submitted parts
			// that have to be filled with zeroes for parts smaller
			// than minimum allowed size
			if messageSize == 0 {
				return nil, r.setError(trace.BadParameter("unexpected message size 0"))
			}
			_, err = io.ReadFull(r.gzipReader, r.messageBytes[:messageSize])
			if err != nil {
				return nil, r.setError(trace.ConvertSystemError(err))
			}
			oneof := apievents.OneOf{}
			err = oneof.Unmarshal(r.messageBytes[:messageSize])
			if err != nil {
				return nil, trace.Wrap(err)
			}
			event, err := apievents.FromOneOf(oneof)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			r.stats.TotalEvents++
			if event.GetIndex() <= r.lastIndex {
				r.stats.SkippedEvents++
				continue
			}
			if r.lastIndex > 0 && event.GetIndex() != r.lastIndex+1 {
				r.stats.OutOfOrderEvents++
			}
			r.lastIndex = event.GetIndex()
			return event, nil
		default:
			return nil, trace.BadParameter("unsupported reader size")
		}
	}
}

// ReadAll reads all events until EOF
func (r *ProtoReader) ReadAll(ctx context.Context) ([]apievents.AuditEvent, error) {
	var events []apievents.AuditEvent
	for {
		event, err := r.Read(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return events, nil
			}
			return nil, trace.Wrap(err)
		}
		events = append(events, event)
	}
}

// isReserveUploadPartError identifies uploader reserve part errors.
func isReserveUploadPartError(err error) bool {
	return strings.Contains(err.Error(), uploaderReservePartErrorMessage)
}
