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
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
)

// NewStreamer creates a streamer sending uploads to disk
func NewStreamer(dir string) (*events.ProtoStreamer, error) {
	handler, err := NewHandler(Config{
		Directory: dir,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return events.NewProtoStreamer(events.ProtoStreamerConfig{
		Uploader:       handler,
		MinUploadBytes: events.MaxProtoMessageSizeBytes * 2,
	})
}

// CreateUpload creates a multipart upload
func (h *Handler) CreateUpload(ctx context.Context, sessionID session.ID) (*events.StreamUpload, error) {
	if err := os.MkdirAll(h.uploadsPath(), teleport.PrivateDirMode); err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	upload := events.StreamUpload{
		SessionID: sessionID,
		ID:        uuid.New(),
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

	partPath := h.partPath(upload, partNumber)
	file, err := os.OpenFile(partPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	_, err = io.Copy(file, partBody)
	if err = trace.NewAggregate(err, file.Close()); err != nil {
		if rmErr := os.Remove(partPath); rmErr != nil {
			h.WithError(rmErr).Warningf("Failed to remove file %q.", partPath)
		}
		return nil, trace.Wrap(err)
	}

	return &events.StreamPart{Number: partNumber}, nil
}

// CompleteUpload completes the upload
func (h *Handler) CompleteUpload(ctx context.Context, upload events.StreamUpload, parts []events.StreamPart) error {
	if len(parts) == 0 {
		return trace.BadParameter("need at least one part to complete the upload")
	}
	if err := checkUpload(upload); err != nil {
		return trace.Wrap(err)
	}

	// Parts must be sorted in PartNumber order.
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].Number < parts[j].Number
	})

	uploadPath := h.path(upload.SessionID)

	// Prevent other processes from accessing this file until the write is completed
	f, err := os.OpenFile(uploadPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	if err := utils.FSTryWriteLock(f); err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if err := utils.FSUnlock(f); err != nil {
			h.WithError(err).Errorf("Failed to unlock filesystem lock.")
		}
		if err := f.Close(); err != nil {
			h.WithError(err).Errorf("Failed to close file %q.", uploadPath)
		}
	}()

	files := make([]*os.File, 0, len(parts))
	readers := make([]io.Reader, 0, len(parts))

	defer func() {
		for i := 0; i < len(files); i++ {
			if err := files[i].Close(); err != nil {
				h.WithError(err).Errorf("Failed to close file %q.", files[i].Name())
			}
		}
	}()

	for _, part := range parts {
		partPath := h.partPath(upload, part.Number)
		file, err := os.Open(partPath)
		if err != nil {
			return trace.ConvertSystemError(err)
		}
		files = append(files, file)
		readers = append(readers, file)
	}

	_, err = io.Copy(f, io.MultiReader(readers...))
	if err != nil {
		return trace.Wrap(err)
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
			Number: part,
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

	dirs, err := ioutil.ReadDir(h.uploadsPath())
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
		files, err := ioutil.ReadDir(filepath.Join(h.uploadsPath(), dir.Name()))
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
		uploads = append(uploads, events.StreamUpload{
			SessionID: session.ID(filepath.Base(files[0].Name())),
			ID:        uploadID,
			Initiated: dir.ModTime(),
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

func partFileName(partNumber int64) string {
	return fmt.Sprintf("%v%v", partNumber, partExt)
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
	out := uuid.Parse(uploadID)
	if out == nil {
		return trace.BadParameter("bad format of upload ID")
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
)
