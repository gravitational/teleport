/*
Copyright 2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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

var (
	// openFileFunc this is the `OpenFileWithFlagsFunc` used by the handler.
	//
	// TODO(gabrielcorado): remove this global variable.
	openFileFunc utils.OpenFileWithFlagsFunc = os.OpenFile

	// flagLock protects access to all globals declared in this file
	flagLock sync.Mutex
)

// SetOpenFileFunc sets the OpenFileWithFlagsFunc used by the package.
//
// TODO(gabrielcorado): remove this global variable.
func SetOpenFileFunc(f utils.OpenFileWithFlagsFunc) {
	flagLock.Lock()
	defer flagLock.Unlock()
	openFileFunc = f
}

// GetOpenFileFunc gets the OpenFileWithFlagsFunc set in the package.
//
// TODO(gabrielcorado): remove this global variable.
func GetOpenFileFunc() utils.OpenFileWithFlagsFunc {
	flagLock.Lock()
	defer flagLock.Unlock()
	return openFileFunc
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

	file, reservationPath, err := h.openReservationPart(upload, partNumber)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	size, err := io.Copy(file, partBody)
	if err = trace.NewAggregate(err, file.Truncate(size), file.Close()); err != nil {
		if rmErr := os.Remove(reservationPath); rmErr != nil {
			h.WithError(rmErr).Warningf("Failed to remove file %q.", reservationPath)
		}
		return nil, trace.Wrap(err)
	}

	// Rename reservation to part file.
	partPath := h.partPath(upload, partNumber)
	err = os.Rename(reservationPath, partPath)
	if err != nil {
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

	// Parts must be sorted in PartNumber order.
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].Number < parts[j].Number
	})

	uploadPath := h.path(upload.SessionID)

	// Prevent other processes from accessing this file until the write is completed
	f, err := GetOpenFileFunc()(uploadPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
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
					h.WithError(err).Errorf("Failed to close file %q.", uploadPath)
				}

				return nil
			case <-time.After(50 * time.Millisecond):
				unlock, err = utils.FSTryWriteLock(uploadPath)
				continue
			}
		default:
			if err := f.Close(); err != nil {
				h.WithError(err).Errorf("Failed to close file %q.", uploadPath)
			}

			return trace.Wrap(err, "handler could not acquire file lock for %q", uploadPath)
		}
	}

	if unlock == nil {
		if err := f.Close(); err != nil {
			h.WithError(err).Errorf("Failed to close file %q.", uploadPath)
		}

		return trace.Wrap(err, "handler could not acquire file lock for %q", uploadPath)
	}

	defer func() {
		if err := unlock(); err != nil {
			h.WithError(err).Errorf("Failed to unlock filesystem lock.")
		}
		if err := f.Close(); err != nil {
			h.WithError(err).Errorf("Failed to close file %q.", uploadPath)
		}
	}()

	writePartToFile := func(path string) error {
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() {
			if err := file.Close(); err != nil {
				h.WithError(err).Errorf("failed to close file %q", path)
			}
		}()

		_, err = io.Copy(f, file)
		return err
	}

	for _, part := range parts {
		partPath := h.partPath(upload, part.Number)
		if err := writePartToFile(partPath); err != nil {
			return trace.Wrap(err)
		}
	}

	err = h.Config.OnBeforeComplete(ctx, upload)
	if err != nil {
		return trace.Wrap(err)
	}

	err = os.RemoveAll(h.uploadRootPath(upload))
	if err != nil {
		h.WithError(err).Errorf("Failed to remove upload %q.", upload.ID)
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
			h.WithError(err).Debugf("Skipping file %v.", path)
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
			h.WithError(err).Warningf("Skipping upload %v with bad format.", uploadID)
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
			h.Warningf("Skipping upload %v, missing subdirectory.", uploadID)
			continue
		}
		if !files[0].IsDir() {
			h.Warningf("Skipping upload %v, not a directory.", uploadID)
			continue
		}

		info, err := dir.Info()
		if err != nil {
			h.WithError(err).Warningf("Skipping upload %v: cannot read file info", uploadID)
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
	file, partPath, err := h.openReservationPart(upload, partNumber)
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	// Create a buffer with the max size that a part file can have.
	buf := make([]byte, minUploadBytes+events.MaxProtoMessageSizeBytes)

	_, err = file.Write(buf)
	if err = trace.NewAggregate(err, file.Close()); err != nil {
		if rmErr := os.Remove(partPath); rmErr != nil {
			h.WithError(rmErr).Warningf("Failed to remove file %q.", partPath)
		}

		return trace.ConvertSystemError(err)
	}

	return nil
}

// openReservationPart opens a reservation upload part file.
func (h *Handler) openReservationPart(upload events.StreamUpload, partNumber int64) (*os.File, string, error) {
	partPath := h.reservationPath(upload, partNumber)
	file, err := GetOpenFileFunc()(partPath, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, partPath, trace.ConvertSystemError(err)
	}

	return file, partPath, nil
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
