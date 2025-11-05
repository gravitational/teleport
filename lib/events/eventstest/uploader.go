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

package eventstest

import (
	"bytes"
	"context"
	"io"
	"iter"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
)

// MemoryUploaderConfig optional configuration for MemoryUploader.
type MemoryUploaderConfig struct {
	// EventsC is used by some tests to receive signal for completed uploads.
	EventsC chan events.UploadEvent
	// MinimumUploadBytes sets the minimum upload part size. The uploader will
	// add padding to smaller uploads to reach this minimum size.
	MinimumUploadBytes int
}

// NewMemoryUploader returns a new memory uploader implementing multipart
// upload
func NewMemoryUploader(cfg ...MemoryUploaderConfig) *MemoryUploader {
	up := &MemoryUploader{
		mtx:        &sync.RWMutex{},
		uploads:    make(map[string]*MemoryUpload),
		sessions:   make(map[session.ID][]byte),
		summaries:  make(map[session.ID][]byte),
		metadata:   make(map[session.ID][]byte),
		thumbnails: make(map[session.ID][]byte),
	}
	if len(cfg) != 0 {
		up.cfg = cfg[0]
	}
	return up
}

// MemoryUploader uploads all bytes to memory, used in tests
type MemoryUploader struct {
	cfg MemoryUploaderConfig

	mtx        *sync.RWMutex
	uploads    map[string]*MemoryUpload
	sessions   map[session.ID][]byte
	summaries  map[session.ID][]byte
	metadata   map[session.ID][]byte
	thumbnails map[session.ID][]byte

	// Clock is an optional [clockwork.Clock] to determine the time to associate
	// with uploads and parts.
	Clock clockwork.Clock
}

// MemoryUpload is used in tests
type MemoryUpload struct {
	// id is the upload ID
	id string
	// parts is the upload parts
	parts map[int64]part
	// sessionID is the session ID associated with the upload
	sessionID session.ID
	//completed specifies upload as completed
	completed bool
	// Initiated contains the timestamp of when the upload
	// was initiated, not always initialized
	Initiated time.Time
}

type part struct {
	data         []byte
	lastModified time.Time
}

func (m *MemoryUploader) trySendEvent(event events.UploadEvent) {
	if m.cfg.EventsC == nil {
		return
	}
	select {
	case m.cfg.EventsC <- event:
	default:
	}
}

// Reset resets all state, removes all uploads and objects
func (m *MemoryUploader) Reset() {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.uploads = make(map[string]*MemoryUpload)
	m.sessions = make(map[session.ID][]byte)
	m.summaries = make(map[session.ID][]byte)
	m.metadata = make(map[session.ID][]byte)
	m.thumbnails = make(map[session.ID][]byte)
}

// CreateUpload creates a multipart upload
func (m *MemoryUploader) CreateUpload(ctx context.Context, sessionID session.ID) (*events.StreamUpload, error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	upload := &events.StreamUpload{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		Initiated: time.Now(),
	}
	if m.Clock != nil {
		upload.Initiated = m.Clock.Now()
	}
	m.uploads[upload.ID] = &MemoryUpload{
		id:        upload.ID,
		sessionID: sessionID,
		parts:     make(map[int64]part),
		Initiated: upload.Initiated,
	}
	return upload, nil
}

// CompleteUpload completes the upload
func (m *MemoryUploader) CompleteUpload(ctx context.Context, upload events.StreamUpload, parts []events.StreamPart) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	up, ok := m.uploads[upload.ID]
	if !ok {
		return trace.NotFound("upload not found")
	}
	if up.completed {
		return trace.BadParameter("upload already completed")
	}
	// verify that all parts have been uploaded
	var result []byte
	partsSet := make(map[int64]bool, len(parts))
	for _, part := range parts {
		partsSet[part.Number] = true
		upPart, ok := up.parts[part.Number]
		if !ok {
			return trace.NotFound("part %v has not been uploaded", part.Number)
		}
		result = append(result, upPart.data...)
	}
	// exclude parts that are not requested to be completed
	for number := range up.parts {
		if !partsSet[number] {
			delete(up.parts, number)
		}
	}
	m.sessions[upload.SessionID] = result
	up.completed = true
	m.trySendEvent(events.UploadEvent{SessionID: string(upload.SessionID), UploadID: upload.ID})
	return nil
}

// UploadPart uploads part and returns the part
func (m *MemoryUploader) UploadPart(ctx context.Context, upload events.StreamUpload, partNumber int64, partBody io.ReadSeeker) (*events.StreamPart, error) {
	data, err := io.ReadAll(partBody)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	m.mtx.Lock()
	defer m.mtx.Unlock()
	up, ok := m.uploads[upload.ID]
	if !ok {
		return nil, trace.NotFound("upload %q is not found", upload.ID)
	}
	lastModified := time.Now()
	if m.Clock != nil {
		lastModified = m.Clock.Now()
	}
	up.parts[partNumber] = part{
		data:         data,
		lastModified: lastModified,
	}
	return &events.StreamPart{Number: partNumber, LastModified: lastModified}, nil
}

// ListUploads lists uploads that have been initiated but not completed with
// earlier uploads returned first.
func (m *MemoryUploader) ListUploads(ctx context.Context) ([]events.StreamUpload, error) {
	m.mtx.RLock()
	defer m.mtx.RUnlock()
	uploads := make([]events.StreamUpload, 0, len(m.uploads))
	for id, upload := range m.uploads {
		uploads = append(uploads, events.StreamUpload{
			ID:        id,
			SessionID: upload.sessionID,
			Initiated: upload.Initiated,
		})
	}
	sort.Slice(uploads, func(i, j int) bool {
		return uploads[i].Initiated.Before(uploads[j].Initiated)
	})
	return uploads, nil
}

// GetParts returns upload parts uploaded up to date, sorted by part number
func (m *MemoryUploader) GetParts(uploadID string) ([][]byte, error) {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	up, ok := m.uploads[uploadID]
	if !ok {
		return nil, trace.NotFound("upload %q is not found", uploadID)
	}

	partNumbers := make([]int64, 0, len(up.parts))
	sortedParts := make([][]byte, 0, len(up.parts))
	for partNumber := range up.parts {
		partNumbers = append(partNumbers, partNumber)
	}
	slices.Sort(partNumbers)
	for _, partNumber := range partNumbers {
		sortedParts = append(sortedParts, up.parts[partNumber].data)
	}
	return sortedParts, nil
}

func (m *MemoryUploader) IsCompleted(uploadID string) bool {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	u := m.uploads[uploadID]
	return u != nil && u.completed
}

// ListParts returns all uploaded parts for the completed upload in sorted order
func (m *MemoryUploader) ListParts(ctx context.Context, upload events.StreamUpload) ([]events.StreamPart, error) {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	up, ok := m.uploads[upload.ID]
	if !ok {
		return nil, trace.NotFound("upload %v is not found", upload.ID)
	}

	partNumbers := make([]int64, 0, len(up.parts))
	sortedParts := make([]events.StreamPart, 0, len(up.parts))
	for partNumber := range up.parts {
		partNumbers = append(partNumbers, partNumber)
	}
	slices.Sort(partNumbers)
	for _, partNumber := range partNumbers {
		sortedParts = append(sortedParts, events.StreamPart{Number: partNumber})
	}
	return sortedParts, nil
}

// Upload uploads session tarball and returns URL with uploaded file
// in case of success.
func (m *MemoryUploader) Upload(ctx context.Context, sessionID session.ID, readCloser io.Reader) (string, error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	_, ok := m.sessions[sessionID]
	if ok {
		return "", trace.AlreadyExists("session %q already exists", sessionID)
	}
	data, err := io.ReadAll(readCloser)
	if err != nil {
		return "", trace.ConvertSystemError(err)
	}
	m.sessions[sessionID] = data
	return string(sessionID), nil
}

// UploadSummary uploads session summary and returns URL with uploaded file in
// case of success.
func (m *MemoryUploader) UploadSummary(ctx context.Context, sessionID session.ID, readCloser io.Reader) (string, error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	_, ok := m.summaries[sessionID]
	if ok {
		return "", trace.AlreadyExists("summary %q already exists", sessionID)
	}
	data, err := io.ReadAll(readCloser)
	if err != nil {
		return "", trace.ConvertSystemError(err)
	}
	m.summaries[sessionID] = data
	return string(sessionID), nil
}

// UploadMetadata uploads session metadata and returns URL with uploaded file in
// case of success.
func (m *MemoryUploader) UploadMetadata(ctx context.Context, sessionID session.ID, readCloser io.Reader) (string, error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	_, ok := m.metadata[sessionID]
	if ok {
		return "", trace.AlreadyExists("metadata %q already exists", sessionID)
	}
	data, err := io.ReadAll(readCloser)
	if err != nil {
		return "", trace.ConvertSystemError(err)
	}
	m.metadata[sessionID] = data
	return string(sessionID), nil
}

// UploadThumbnail uploads session thumbnail and returns URL with uploaded file in
// case of success.
func (m *MemoryUploader) UploadThumbnail(ctx context.Context, sessionID session.ID, readCloser io.Reader) (string, error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	_, ok := m.thumbnails[sessionID]
	if ok {
		return "", trace.AlreadyExists("thumbnail %q already exists", sessionID)
	}
	data, err := io.ReadAll(readCloser)
	if err != nil {
		return "", trace.ConvertSystemError(err)
	}
	m.thumbnails[sessionID] = data
	return string(sessionID), nil
}

// Download downloads session tarball and writes it to writer
func (m *MemoryUploader) Download(ctx context.Context, sessionID session.ID, writer events.RandomAccessWriter) error {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	data, ok := m.sessions[sessionID]
	if !ok {
		return trace.NotFound("session %q is not found", sessionID)
	}
	_, err := io.Copy(writer, bytes.NewReader(data))
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}

func (m *MemoryUploader) DownloadSummary(ctx context.Context, sessionID session.ID, writer events.RandomAccessWriter) error {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	data, ok := m.summaries[sessionID]
	if !ok {
		return trace.NotFound("summary %q is not found", sessionID)
	}
	_, err := io.Copy(writer, bytes.NewReader(data))
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}

// DownloadMetadata downloads session metadata and writes it to writer
func (m *MemoryUploader) DownloadMetadata(ctx context.Context, sessionID session.ID, writer events.RandomAccessWriter) error {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	data, ok := m.metadata[sessionID]
	if !ok {
		return trace.NotFound("metadata %q is not found", sessionID)
	}
	_, err := io.Copy(writer, bytes.NewReader(data))
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}

// DownloadThumbnail downloads session thumbnail and writes it to writer
func (m *MemoryUploader) DownloadThumbnail(ctx context.Context, sessionID session.ID, writer events.RandomAccessWriter) error {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	data, ok := m.thumbnails[sessionID]
	if !ok {
		return trace.NotFound("thumbnail %q is not found", sessionID)
	}
	_, err := io.Copy(writer, bytes.NewReader(data))
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}

// GetUploadMetadata gets the session upload metadata
func (m *MemoryUploader) GetUploadMetadata(sid session.ID) events.UploadMetadata {
	return events.UploadMetadata{
		URL:       "memory",
		SessionID: sid,
	}
}

// ReserveUploadPart reserves an upload part.
func (m *MemoryUploader) ReserveUploadPart(ctx context.Context, upload events.StreamUpload, partNumber int64) error {
	return nil
}

// UploadEncryptedRecording uploads encrypted recordings.
func (m *MemoryUploader) UploadEncryptedRecording(ctx context.Context, sessionID string, parts iter.Seq2[[]byte, error]) error {
	sessID, err := session.ParseID(sessionID)
	if err != nil {
		return trace.Wrap(err)
	}
	upload, err := m.CreateUpload(ctx, *sessID)
	if err != nil {
		return trace.Wrap(err, "creating upload")
	}

	next, stop := iter.Pull2(parts)
	defer stop()

	part, err, ok := next()
	if err != nil {
		return trace.Wrap(err)
	} else if !ok {
		return trace.BadParameter("unexpected empty upload")
	}

	var streamParts []events.StreamPart
	// S3 requires that part numbers start at 1, so we do that by default regardless of which uploader is
	// configured for the auth service
	var partNumber int64 = 1
	for {
		if err := m.ReserveUploadPart(ctx, *upload, partNumber); err != nil {
			return trace.Wrap(err, "reserving upload part")
		}

		nextPart, err, hasNext := next()
		if err != nil {
			return trace.Wrap(err)
		}

		// If the upload part is not at least the minimum upload part size, and this isn't
		// the last part, append an empty part to pad up to the minimum upload size.
		if hasNext && len(part) < m.cfg.MinimumUploadBytes {
			part = events.PadUploadPart(part, m.cfg.MinimumUploadBytes)
		}

		streamPart, err := m.UploadPart(ctx, *upload, partNumber, bytes.NewReader(part))
		if err != nil {
			return trace.Wrap(err, "uploading part")
		}
		streamParts = append(streamParts, *streamPart)

		if !hasNext {
			break
		}

		part = nextPart
		partNumber++
	}

	return trace.Wrap(m.CompleteUpload(ctx, *upload, streamParts), "completing upload")
}

// MockUploader is a limited implementation of [events.MultipartUploader] that
// allows injecting errors for testing purposes. [MemoryUploader] is a more
// complete implementation and should be preferred for testing the happy path.
type MockUploader struct {
	events.MultipartUploader

	CreateUploadError      error
	ReserveUploadPartError error

	MockListParts      func(ctx context.Context, upload events.StreamUpload) ([]events.StreamPart, error)
	MockListUploads    func(ctx context.Context) ([]events.StreamUpload, error)
	MockCompleteUpload func(ctx context.Context, upload events.StreamUpload, parts []events.StreamPart) error
}

func (m *MockUploader) CreateUpload(ctx context.Context, sessionID session.ID) (*events.StreamUpload, error) {
	if m.CreateUploadError != nil {
		return nil, m.CreateUploadError
	}

	return &events.StreamUpload{
		ID:        uuid.New().String(),
		SessionID: sessionID,
	}, nil
}

func (m *MockUploader) ReserveUploadPart(_ context.Context, _ events.StreamUpload, _ int64) error {
	return m.ReserveUploadPartError
}

func (m *MockUploader) ListParts(ctx context.Context, upload events.StreamUpload) ([]events.StreamPart, error) {
	if m.MockListParts != nil {
		return m.MockListParts(ctx, upload)
	}

	return []events.StreamPart{}, nil
}

func (m *MockUploader) ListUploads(ctx context.Context) ([]events.StreamUpload, error) {
	if m.MockListUploads != nil {
		return m.MockListUploads(ctx)
	}

	return nil, nil
}

func (m *MockUploader) CompleteUpload(ctx context.Context, upload events.StreamUpload, parts []events.StreamPart) error {
	if m.MockCompleteUpload != nil {
		return m.MockCompleteUpload(ctx, upload, parts)
	}

	return nil
}
