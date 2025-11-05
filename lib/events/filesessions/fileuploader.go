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
	"fmt"
	"io"
	"iter"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
)

// Config is a file uploader configuration
type Config struct {
	// Directory is a directory with files
	Directory string
	// OnBeforeComplete can be used to inject failures during tests
	OnBeforeComplete func(ctx context.Context, upload events.StreamUpload) error
	// OpenFile is used by session recording to open OS files
	OpenFile utils.OpenFileWithFlagsFunc
}

// nopBeforeComplete does nothing
func nopBeforeComplete(ctx context.Context, upload events.StreamUpload) error {
	return nil
}

// CheckAndSetDefaults checks and sets default values of file handler config
func (s *Config) CheckAndSetDefaults() error {
	if s.Directory == "" {
		return trace.BadParameter("missing parameter Directory")
	}
	if !utils.IsDir(s.Directory) {
		return trace.BadParameter("path %q does not exist or is not a directory", s.Directory)
	}
	if s.OnBeforeComplete == nil {
		s.OnBeforeComplete = nopBeforeComplete
	}
	if s.OpenFile == nil {
		s.OpenFile = os.OpenFile
	}
	return nil
}

// sessionFileRecorder captures file operations performed as part of saving session recordings as files.
type sessionFileRecorder interface {
	ReservePart(ctx context.Context, name string, size int64) error
	WritePart(ctx context.Context, name string, data io.Reader) error
	CombineParts(ctx context.Context, dst io.Writer, parts iter.Seq[string]) error
}

// NewHandler returns new file sessions handler
func NewHandler(cfg Config) (*Handler, error) {
	if err := os.MkdirAll(cfg.Directory, teleport.SharedDirMode); err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	logger := slog.With(teleport.ComponentKey, teleport.SchemeFile)
	h := &Handler{
		logger:       logger,
		Config:       cfg,
		fileRecorder: NewPlainFileRecorder(logger, cfg.OpenFile),
	}
	return h, nil
}

// Handler uploads and downloads sessions archives by reading
// and writing files to directory, useful for NFS setups and tests
type Handler struct {
	// Config is a file sessions config
	Config
	// logger emits logs messages
	logger *slog.Logger
	// fileRecorder is the interface for "low-level" file operations
	fileRecorder sessionFileRecorder
}

// Closer releases connection and resources associated with log if any
func (l *Handler) Close() error {
	return nil
}

// Download reads a session recording from a local directory.
func (l *Handler) Download(ctx context.Context, sessionID session.ID, writer events.RandomAccessWriter) error {
	return downloadFile(l.recordingPath(sessionID), writer)
}

// DownloadSummary reads a session summary from a local directory.
func (l *Handler) DownloadSummary(ctx context.Context, sessionID session.ID, writer events.RandomAccessWriter) error {
	return downloadFile(l.summaryPath(sessionID), writer)
}

// DownloadMetadata reads session metadata from a local directory.
func (l *Handler) DownloadMetadata(ctx context.Context, sessionID session.ID, writer events.RandomAccessWriter) error {
	return downloadFile(l.metadataPath(sessionID), writer)
}

// DownloadThumbnail reads a session thumbnail from a local directory.
func (l *Handler) DownloadThumbnail(ctx context.Context, sessionID session.ID, writer events.RandomAccessWriter) error {
	return downloadFile(l.thumbnailPath(sessionID), writer)
}

func downloadFile(path string, writer events.RandomAccessWriter) error {
	f, err := os.Open(path)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer f.Close()
	_, err = io.Copy(writer, f)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Upload writes a session recording to a local directory.
func (l *Handler) Upload(ctx context.Context, sessionID session.ID, reader io.Reader) (string, error) {
	return uploadFile(l.recordingPath(sessionID), reader)
}

// UploadSummary writes a session summary to a local directory.
func (l *Handler) UploadSummary(ctx context.Context, sessionID session.ID, reader io.Reader) (string, error) {
	return uploadFile(l.summaryPath(sessionID), reader)
}

// UploadMetadata writes session metadata to a local directory.
func (l *Handler) UploadMetadata(ctx context.Context, sessionID session.ID, reader io.Reader) (string, error) {
	return uploadFile(l.metadataPath(sessionID), reader)
}

// UploadThumbnail writes a session thumbnail to a local directory.
func (l *Handler) UploadThumbnail(ctx context.Context, sessionID session.ID, reader io.Reader) (string, error) {
	return uploadFile(l.thumbnailPath(sessionID), reader)
}

func uploadFile(path string, reader io.Reader) (string, error) {
	f, err := os.Create(path)
	if err != nil {
		return "", trace.ConvertSystemError(err)
	}
	_, err = io.Copy(f, reader)
	if err = trace.NewAggregate(err, f.Close()); err != nil {
		return "", trace.Wrap(err)
	}
	return fmt.Sprintf("%v://%v", teleport.SchemeFile, path), nil
}

func (l *Handler) recordingPath(sessionID session.ID) string {
	return filepath.Join(l.Directory, string(sessionID)+tarExt)
}

func (l *Handler) summaryPath(sessionID session.ID) string {
	return filepath.Join(l.Directory, string(sessionID)+summaryExt)
}

func (l *Handler) metadataPath(sessionID session.ID) string {
	return filepath.Join(l.Directory, string(sessionID)+metadataExt)
}

func (l *Handler) thumbnailPath(sessionID session.ID) string {
	return filepath.Join(l.Directory, string(sessionID)+thumbnailExt)
}

// sessionIDFromPath extracts session ID from the filename
func sessionIDFromPath(path string) (session.ID, error) {
	base := filepath.Base(path)
	if filepath.Ext(base) != tarExt {
		return session.ID(""), trace.BadParameter("expected extension %v, got %v", tarExt, base)
	}
	sid := session.ID(strings.TrimSuffix(base, tarExt))
	if err := sid.Check(); err != nil {
		return session.ID(""), trace.Wrap(err)
	}
	return sid, nil
}
