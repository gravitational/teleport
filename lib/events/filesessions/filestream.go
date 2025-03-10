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

package filesessions

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
)

// TODO(gabrielcorado): remove this global variable.
var globalOpenFile = struct {
	mu       sync.Mutex // guards openFile
	openFile utils.OpenFileWithFlagsFunc
}{
	openFile: os.OpenFile,
}

// SetOpenFileFunc sets the OpenFileWithFlagsFunc used by the package.
//
// TODO(gabrielcorado): remove this global variable.
func SetOpenFileFunc(f utils.OpenFileWithFlagsFunc) {
	globalOpenFile.mu.Lock()
	globalOpenFile.openFile = f
	globalOpenFile.mu.Unlock()
}

// GetOpenFileFunc gets the OpenFileWithFlagsFunc set in the package.
//
// TODO(gabrielcorado): remove this global variable.
func GetOpenFileFunc() utils.OpenFileWithFlagsFunc {
	globalOpenFile.mu.Lock()
	fn := globalOpenFile.openFile
	globalOpenFile.mu.Unlock()
	return fn
}

// minUploadBytes is the minimum part file size required to trigger its upload.
const minUploadBytes = events.MaxProtoMessageSizeBytes * 2

// NewStreamer creates a streamer sending uploads to disk
func NewStreamer(dir string) (*events.ProtoStreamer, error) {
	handler, err := NewHandler(Config{Directory: dir})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return events.NewProtoStreamer(events.ProtoStreamerConfig{
		Uploader:       handler,
		MinUploadBytes: minUploadBytes,
	})
}

// CreateUpload creates a multipart upload
func (h *Handler) CreateUpload(ctx context.Context, sessionID session.ID) (*events.StreamUpload, error) {
	if err := os.MkdirAll(h.uploadsPath(), teleport.PrivateDirMode); err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	upload := events.StreamUpload{
		SessionID: sessionID,
		ID:        uuid.New().String(),
	}
	if err := upload.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := os.MkdirAll(h.uploadPath(upload), teleport.PrivateDirMode); err != nil {
		return nil, trace.Wrap(err)
	}

	return &upload, nil
}

// UploadPart uploads part
func (h *Handler) UploadPart(ctx context.Context, upload events.StreamUpload, partNumber int64, partBody io.ReadSeeker) (*events.StreamPart, error) {
	if err := checkUpload(upload); err != nil {
		return nil, trace.Wrap(err)
	}

	reservationPath := h.reservationPath(upload, partNumber)
	if err := h.fileOps.WriteReservation(reservationPath, partBody); err != nil {
		// TODO(codingllama): Move Remove into fileOps?
		if rmErr := os.Remove(reservationPath); rmErr != nil {
			h.logger.WarnContext(ctx, "Failed to remove part file", "file", reservationPath, "error", rmErr)
		}
		return nil, trace.Wrap(err)
	}

	// Rename reservation to part file.
	partPath := h.partPath(upload, partNumber)
	if err := os.Rename(reservationPath, partPath); err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	var lastModified time.Time
	fi, err := os.Stat(partPath)
	if err == nil {
		lastModified = fi.ModTime()
	}
	return &events.StreamPart{Number: partNumber, LastModified: lastModified}, nil
}

// CompleteUpload completes the upload
func (h *Handler) CompleteUpload(ctx context.Context, upload events.StreamUpload, parts []events.StreamPart) error {
	if err := checkUpload(upload); err != nil {
		return trace.Wrap(err)
	}

	uploadPath := h.path(upload.SessionID)

	// Prevent other processes from accessing this file until the write is completed
	f, err := h.openFile(uploadPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	unlock, err := utils.FSTryWriteLock(uploadPath)
Loop:
	for i := 0; i < 3; i++ {
		switch {
		case err == nil:
			break Loop
		case errors.Is(err, utils.ErrUnsuccessfulLockTry):
			// If unable to lock the file, try again with some backoff
			// to allow the UploadCompleter to finish and remove its
			// file lock before giving up.
			select {
			case <-ctx.Done():
				if err := f.Close(); err != nil {
					h.logger.ErrorContext(ctx, "Failed to close upload file", "file", uploadPath, "error", err)
				}

				return nil
			case <-time.After(50 * time.Millisecond):
				unlock, err = utils.FSTryWriteLock(uploadPath)
				continue
			}
		default:
			if err := f.Close(); err != nil {
				h.logger.ErrorContext(ctx, "Failed to close upload file", "file", uploadPath)
			}

			return trace.Wrap(err, "handler could not acquire file lock for %q", uploadPath)
		}
	}

	if unlock == nil {
		if err := f.Close(); err != nil {
			h.logger.ErrorContext(ctx, "Failed to close upload file", "file", uploadPath, "error", err)
		}

		return trace.Wrap(err, "handler could not acquire file lock for %q", uploadPath)
	}

	defer func() {
		if err := unlock(); err != nil {
			h.logger.ErrorContext(ctx, "Failed to unlock filesystem lock.", "error", err)
		}
		if err := f.Close(); err != nil {
			h.logger.ErrorContext(ctx, "Failed to close upload file", "file", uploadPath, "error", err)
		}
	}()

	// Collect part names in order.
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].Number < parts[j].Number
	})
	partNames := make([]string, len(parts))
	for i, part := range parts {
		partNames[i] = h.partPath(upload, part.Number)
	}

	// Combine parts into f.
	if err := h.fileOps.CombineParts(f, partNames); err != nil {
		return trace.Wrap(err)
	}

	err = h.Config.OnBeforeComplete(ctx, upload)
	if err != nil {
		return trace.Wrap(err)
	}

	err = os.RemoveAll(h.uploadRootPath(upload))
	if err != nil {
		h.logger.ErrorContext(ctx, "Failed to remove upload", "upload_id", upload.ID)
	}
	return nil
}

// ListParts lists upload parts
func (h *Handler) ListParts(ctx context.Context, upload events.StreamUpload) ([]events.StreamPart, error) {
	var parts []events.StreamPart
	if err := checkUpload(upload); err != nil {
		return nil, trace.Wrap(err)
	}
	err := filepath.Walk(h.uploadPath(upload), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			err = trace.ConvertSystemError(err)
			if trace.IsNotFound(err) {
				return nil
			}
			return err
		}
		if info.IsDir() {
			return nil
		}
		part, err := partFromFileName(path)
		if err != nil {
			h.logger.DebugContext(ctx, "Skipping upload file", "file", path, "error", err)

			return nil
		}
		parts = append(parts, events.StreamPart{
			Number:       part,
			LastModified: info.ModTime(),
		})
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Parts must be sorted in PartNumber order.
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].Number < parts[j].Number
	})
	return parts, nil
}

// ListUploads lists uploads that have been initiated but not completed with
// earlier uploads returned first
func (h *Handler) ListUploads(ctx context.Context) ([]events.StreamUpload, error) {
	var uploads []events.StreamUpload

	dirs, err := os.ReadDir(h.uploadsPath())
	if err != nil {
		err = trace.ConvertSystemError(err)
		// The upload folder may not exist if there are no uploads yet.
		if trace.IsNotFound(err) {
			return nil, nil
		}
		return nil, trace.Wrap(err)
	}

	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}
		uploadID := dir.Name()
		if err := checkUploadID(uploadID); err != nil {
			h.logger.WarnContext(ctx, "Skipping upload with bad format", "upload_id", uploadID, "error", err)
			continue
		}
		files, err := os.ReadDir(filepath.Join(h.uploadsPath(), dir.Name()))
		if err != nil {
			err = trace.ConvertSystemError(err)
			if trace.IsNotFound(err) {
				continue
			}
			return nil, trace.Wrap(err)
		}
		// expect just one subdirectory - session ID
		if len(files) != 1 {
			h.logger.WarnContext(ctx, "Skipping upload, missing subdirectory.", "upload_id", uploadID)
			continue
		}
		if !files[0].IsDir() {
			h.logger.WarnContext(ctx, "Skipping upload, not a directory.", "upload_id", uploadID)
			continue
		}

		info, err := dir.Info()
		if err != nil {
			h.logger.WarnContext(ctx, "Skipping upload: cannot read file info", "upload_id", uploadID, "error", err)
			continue
		}

		uploads = append(uploads, events.StreamUpload{
			SessionID: session.ID(filepath.Base(files[0].Name())),
			ID:        uploadID,
			Initiated: info.ModTime(),
		})
	}

	sort.Slice(uploads, func(i, j int) bool {
		return uploads[i].Initiated.Before(uploads[j].Initiated)
	})

	return uploads, nil
}

// GetUploadMetadata gets the metadata for session upload
func (h *Handler) GetUploadMetadata(s session.ID) events.UploadMetadata {
	return events.UploadMetadata{
		URL:       fmt.Sprintf("%v://%v/%v", teleport.SchemeFile, h.uploadsPath(), string(s)),
		SessionID: s,
	}
}

// ReserveUploadPart reserves an upload part.
func (h *Handler) ReserveUploadPart(ctx context.Context, upload events.StreamUpload, partNumber int64) error {
	reservationPath := h.reservationPath(upload, partNumber)
	const size = minUploadBytes + events.MaxProtoMessageSizeBytes
	if err := h.fileOps.CreateReservation(reservationPath, size); err != nil {
		// TODO(codingllama): Move Remove into fileOps?
		if rmErr := os.Remove(reservationPath); rmErr != nil {
			h.logger.WarnContext(ctx, "Failed to remove part file.", "file", reservationPath, "error", rmErr)
		}
		return trace.Wrap(err)
	}

	return nil
}

func (h *Handler) uploadsPath() string {
	return filepath.Join(h.Directory, uploadsDir)
}

func (h *Handler) uploadRootPath(upload events.StreamUpload) string {
	return filepath.Join(h.uploadsPath(), upload.ID)
}

func (h *Handler) uploadPath(upload events.StreamUpload) string {
	return filepath.Join(h.uploadRootPath(upload), string(upload.SessionID))
}

func (h *Handler) partPath(upload events.StreamUpload, partNumber int64) string {
	return filepath.Join(h.uploadPath(upload), partFileName(partNumber))
}

func (h *Handler) reservationPath(upload events.StreamUpload, partNumber int64) string {
	return filepath.Join(h.uploadPath(upload), reservationFileName(partNumber))
}

func partFileName(partNumber int64) string {
	return fmt.Sprintf("%v%v", partNumber, partExt)
}

func reservationFileName(partNumber int64) string {
	return fmt.Sprintf("%v%v", partNumber, reservationExt)
}

func partFromFileName(fileName string) (int64, error) {
	base := filepath.Base(fileName)
	if filepath.Ext(base) != partExt {
		return -1, trace.BadParameter("expected extension %v, got %v", partExt, base)
	}
	numberString := strings.TrimSuffix(base, partExt)
	partNumber, err := strconv.ParseInt(numberString, 10, 0)
	if err != nil {
		return -1, trace.Wrap(err)
	}
	return partNumber, nil
}

// checkUpload checks that upload IDs are valid
// and in addition verifies that upload ID is a valid UUID
// to avoid file scanning by passing bogus upload ID file paths
func checkUpload(upload events.StreamUpload) error {
	if err := upload.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if err := checkUploadID(upload.ID); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// checkUploadID checks that upload ID is a valid UUID
// to avoid path scanning or using local paths as upload IDs
func checkUploadID(uploadID string) error {
	_, err := uuid.Parse(uploadID)
	if err != nil {
		return trace.WrapWithMessage(err, "bad format of upload ID")
	}
	return nil
}

const (
	// uploadsDir is a directory with multipart uploads
	uploadsDir = "multi"
	// partExt is a part extension
	partExt = ".part"
	// tarExt is a suffix for file uploads
	tarExt = ".tar"
	// checkpointExt is a suffix for checkpoint extensions
	checkpointExt = ".checkpoint"
	// errorExt is a suffix for files storing session errors
	errorExt = ".error"
	// reservationExt is part reservation extension.
	reservationExt = ".reservation"
)
