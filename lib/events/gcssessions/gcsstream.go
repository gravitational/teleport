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

package gcssessions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"google.golang.org/api/iterator"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
)

// CreateUpload creates a multipart upload
func (h *Handler) CreateUpload(ctx context.Context, sessionID session.ID) (*events.StreamUpload, error) {
	upload := events.StreamUpload{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		Initiated: time.Now().UTC(),
	}
	if err := upload.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	uploadPath := h.uploadPath(upload)

	h.logger.DebugContext(ctx, "Creating upload", "path", uploadPath)
	// Make sure we don't overwrite an existing upload
	_, err := h.gcsClient.Bucket(h.Config.Bucket).Object(uploadPath).Attrs(ctx)
	if !errors.Is(err, storage.ErrObjectNotExist) {
		if err != nil {
			return nil, convertGCSError(err)
		}
		return nil, trace.AlreadyExists("upload %v for session %q already exists in GCS", upload.ID, sessionID)
	}

	writer := h.gcsClient.Bucket(h.Config.Bucket).Object(uploadPath).NewWriter(ctx)
	start := time.Now()
	_, err = io.Copy(writer, strings.NewReader(string(sessionID)))
	// Always close the writer, even if upload failed.
	closeErr := writer.Close()
	if err == nil {
		err = closeErr
	}
	uploadLatencies.Observe(time.Since(start).Seconds())
	uploadRequests.Inc()
	if err != nil {
		return nil, convertGCSError(err)
	}
	return &upload, nil
}

// UploadPart uploads part
func (h *Handler) UploadPart(ctx context.Context, upload events.StreamUpload, partNumber int64, partBody io.ReadSeeker) (*events.StreamPart, error) {
	if err := upload.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	partPath := h.partPath(upload, partNumber)
	writer := h.gcsClient.Bucket(h.Config.Bucket).Object(partPath).NewWriter(ctx)
	start := time.Now()
	_, err := io.Copy(writer, partBody)
	// Always close the writer, even if upload failed.
	closeErr := writer.Close()
	if err == nil {
		err = closeErr
	}
	uploadLatencies.Observe(time.Since(start).Seconds())
	uploadRequests.Inc()
	if err != nil {
		return nil, convertGCSError(err)
	}
	return &events.StreamPart{Number: partNumber, LastModified: writer.Attrs().Created}, nil
}

// CompleteUpload completes the upload
func (h *Handler) CompleteUpload(ctx context.Context, upload events.StreamUpload, parts []events.StreamPart) error {
	if err := upload.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	// If the session has been already created, move to cleanup
	sessionPath := h.path(upload.SessionID)
	_, err := h.gcsClient.Bucket(h.Config.Bucket).Object(sessionPath).Attrs(ctx)
	if !errors.Is(err, storage.ErrObjectNotExist) {
		if err != nil {
			return convertGCSError(err)
		}
		return h.cleanupUpload(ctx, upload)
	}

	// Makes sure that upload has been properly initiated,
	// checks the .upload file
	uploadPath := h.uploadPath(upload)
	bucket := h.gcsClient.Bucket(h.Config.Bucket)
	_, err = bucket.Object(uploadPath).Attrs(ctx)
	if err != nil {
		return convertGCSError(err)
	}

	// If there are no parts to complete, move to cleanup
	if len(parts) == 0 {
		return h.cleanupUpload(ctx, upload)
	}

	objects := h.partsToObjects(upload, parts)
	for len(objects) > maxParts {
		h.logger.DebugContext(ctx, "Merging multiple objects for upload", "objects", len(objects), "upload", upload)
		objectsToMerge := objects[:maxParts]
		mergeID := hashOfNames(objectsToMerge)
		mergePath := h.mergePath(upload, mergeID)
		mergeObject := bucket.Object(mergePath)
		composer := mergeObject.ComposerFrom(objectsToMerge...)
		_, err = h.OnComposerRun(ctx, composer)
		if err != nil {
			return convertGCSError(err)
		}
		objects = append([]*storage.ObjectHandle{mergeObject}, objects[maxParts:]...)
	}
	composer := bucket.Object(sessionPath).ComposerFrom(objects...)
	_, err = h.OnComposerRun(ctx, composer)
	if err != nil {
		return convertGCSError(err)
	}
	h.logger.DebugContext(ctx, "Completed upload after merging multiple objects", "objects", len(objects), "upload", upload)
	return h.cleanupUpload(ctx, upload)
}

// cleanupUpload iterates through all upload related objects
// and deletes them in parallel
func (h *Handler) cleanupUpload(ctx context.Context, upload events.StreamUpload) error {
	prefixes := []string{
		h.partsPrefix(upload),
		h.mergesPrefix(upload),
		h.uploadPrefix(upload),
	}

	bucket := h.gcsClient.Bucket(h.Config.Bucket)
	var objects []*storage.ObjectHandle
	for _, prefix := range prefixes {
		i := bucket.Objects(ctx, &storage.Query{Prefix: prefix, Versions: false})
		for {
			attrs, err := i.Next()
			if errors.Is(err, iterator.Done) {
				break
			}
			if err != nil {
				return convertGCSError(err)
			}
			objects = append(objects, bucket.Object(attrs.Name))
		}
	}

	// batch delete objects to speed up the process
	semCh := make(chan struct{}, maxParts)
	errorsCh := make(chan error, maxParts)
	for i := range objects {
		select {
		case semCh <- struct{}{}:
			go func(object *storage.ObjectHandle) {
				defer func() { <-semCh }()
				err := h.AfterObjectDelete(ctx, object, object.Delete(ctx))
				select {
				case errorsCh <- convertGCSError(err):
				case <-ctx.Done():
				}
			}(objects[i])
		case <-ctx.Done():
			return trace.ConnectionProblem(ctx.Err(), "context closed")
		}
	}

	var errors []error
	for range objects {
		select {
		case err := <-errorsCh:
			if !trace.IsNotFound(err) {
				errors = append(errors, err)
			}
		case <-ctx.Done():
			return trace.ConnectionProblem(ctx.Err(), "context closed")
		}
	}
	return trace.NewAggregate(errors...)
}

func (h *Handler) partsToObjects(upload events.StreamUpload, parts []events.StreamPart) []*storage.ObjectHandle {
	objects := make([]*storage.ObjectHandle, len(parts))
	bucket := h.gcsClient.Bucket(h.Config.Bucket)
	for i := 0; i < len(parts); i++ {
		objects[i] = bucket.Object(h.partPath(upload, parts[i].Number))
	}
	return objects
}

// ListParts lists upload parts
func (h *Handler) ListParts(ctx context.Context, upload events.StreamUpload) ([]events.StreamPart, error) {
	if err := upload.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	i := h.gcsClient.Bucket(h.Config.Bucket).Objects(ctx, &storage.Query{
		Prefix: h.partsPrefix(upload),
	})
	var parts []events.StreamPart
	for {
		attrs, err := i.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, convertGCSError(err)
		}
		// Skip entries that are not parts
		if path.Ext(attrs.Name) != partExt {
			continue
		}
		part, err := partFromPath(attrs.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		part.LastModified = attrs.Updated
		parts = append(parts, *part)
	}
	return parts, nil
}

// ListUploads lists uploads that have been initiated but not completed with
// earlier uploads returned first
func (h *Handler) ListUploads(ctx context.Context) ([]events.StreamUpload, error) {
	i := h.gcsClient.Bucket(h.Config.Bucket).Objects(ctx, &storage.Query{
		Prefix: h.uploadsPrefix(),
	})
	var uploads []events.StreamUpload
	for {
		attrs, err := i.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, convertGCSError(err)
		}
		// Skip entries that are not uploads
		if path.Ext(attrs.Name) != uploadExt {
			continue
		}
		upload, err := uploadFromPath(attrs.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		upload.Initiated = attrs.Created
		uploads = append(uploads, *upload)
	}
	return uploads, nil
}

// GetUploadMetadata gets the metadata for session upload
func (h *Handler) GetUploadMetadata(s session.ID) events.UploadMetadata {
	return events.UploadMetadata{
		URL:       fmt.Sprintf("%v://%v/%v", teleport.SchemeGCS, h.path(s), string(s)),
		SessionID: s,
	}
}

// ReserveUploadPart reserves an upload part.
func (h *Handler) ReserveUploadPart(ctx context.Context, upload events.StreamUpload, partNumber int64) error {
	return nil
}

const (
	// uploadsKey is a key that holds all upload-related objects
	uploadsKey = "uploads"
	// partsKey is a key that holds all part-related objects
	partsKey = "parts"
	// mergesKey is a key that holds temp merges to workaround
	// google max parts limit
	mergesKey = "merges"
	// partExt is a part extension
	partExt = ".part"
	// mergeExt is a merge extension
	mergeExt = ".merge"
	// uploadExt is upload extension
	uploadExt = ".upload"
	// slash is a forward slash
	slash = "/"
	// Maximum parts per compose as set by
	// https://cloud.google.com/storage/docs/composite-objects
	maxParts = 32
)

// uploadsPrefix is "path/uploads"
func (h *Handler) uploadsPrefix() string {
	return strings.TrimPrefix(path.Join(h.Path, uploadsKey), slash)
}

// uploadPrefix is "path/uploads/<upload-id>"
func (h *Handler) uploadPrefix(upload events.StreamUpload) string {
	return path.Join(h.uploadsPrefix(), upload.ID)
}

// uploadPath is "path/uploads/<upload-id>/<session-id>.upload"
func (h *Handler) uploadPath(upload events.StreamUpload) string {
	return path.Join(h.uploadPrefix(upload), string(upload.SessionID)) + uploadExt
}

// partsPrefix is "path/parts/<upload-id>"
// this path is under different tree from upload to make prefix
// iteration of uploads more efficient (that otherwise would have
// scan and skip the parts that could be 5K parts per upload)
func (h *Handler) partsPrefix(upload events.StreamUpload) string {
	return strings.TrimPrefix(path.Join(h.Path, partsKey, upload.ID), slash)
}

// partPath is "path/parts/<upload-id>/<part-number>.part"
func (h *Handler) partPath(upload events.StreamUpload, partNumber int64) string {
	return path.Join(h.partsPrefix(upload), fmt.Sprintf("%v%v", partNumber, partExt))
}

// mergesPrefix is "path/merges/<upload-id>"
// this path is under different tree from upload to make prefix
// iteration of uploads more efficient (that otherwise would have
// scan and skip the parts that could be 5K parts per upload)
func (h *Handler) mergesPrefix(upload events.StreamUpload) string {
	return strings.TrimPrefix(path.Join(h.Path, mergesKey, upload.ID), slash)
}

// mergePath is "path/merges/<upload-id>/<merge-id>.merge"
func (h *Handler) mergePath(upload events.StreamUpload, mergeID string) string {
	return path.Join(h.mergesPrefix(upload), fmt.Sprintf("%v%v", mergeID, mergeExt))
}

// hashOfNames creates an object with hash of names
// to avoid generating new objects for consecutive merge attempts
func hashOfNames(objects []*storage.ObjectHandle) string {
	hash := sha256.New()
	for _, object := range objects {
		hash.Write([]byte(object.ObjectName()))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func uploadFromPath(uploadPath string) (*events.StreamUpload, error) {
	dir, file := path.Split(uploadPath)
	if path.Ext(file) != uploadExt {
		return nil, trace.BadParameter("expected extension %v, got %v", uploadExt, file)
	}
	sessionID := session.ID(strings.TrimSuffix(file, uploadExt))
	if err := sessionID.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	parts := strings.Split(strings.TrimSuffix(dir, slash), slash)
	if len(parts) < 2 {
		return nil, trace.BadParameter("expected format uploads/<upload-id>, got %v", dir)
	}
	uploadID := parts[len(parts)-1]
	return &events.StreamUpload{
		SessionID: sessionID,
		ID:        uploadID,
	}, nil
}

func partFromPath(uploadPath string) (*events.StreamPart, error) {
	base := path.Base(uploadPath)
	if path.Ext(base) != partExt {
		return nil, trace.BadParameter("expected extension %v, got %v", partExt, base)
	}
	numberString := strings.TrimSuffix(base, partExt)
	partNumber, err := strconv.ParseInt(numberString, 10, 0)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &events.StreamPart{Number: partNumber}, nil
}
