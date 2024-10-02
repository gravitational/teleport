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
	return nil
}

// NewHandler returns new file sessions handler
func NewHandler(cfg Config) (*Handler, error) {
	if err := os.MkdirAll(cfg.Directory, teleport.SharedDirMode); err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	h := &Handler{
		logger: slog.With(teleport.ComponentKey, teleport.SchemeFile),
		Config: cfg,
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
}

// Closer releases connection and resources associated with log if any
func (l *Handler) Close() error {
	return nil
}

// Download downloads session recording from storage, in case of file handler reads the
// file from local directory
func (l *Handler) Download(ctx context.Context, sessionID session.ID, writer io.WriterAt) error {
	path := l.path(sessionID)
	f, err := os.Open(path)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer f.Close()
	_, err = io.Copy(writer.(io.Writer), f)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Upload uploads session recording to file storage, in case of file handler,
// writes the file to local directory
func (l *Handler) Upload(ctx context.Context, sessionID session.ID, reader io.Reader) (string, error) {
	path := l.path(sessionID)
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

func (l *Handler) path(sessionID session.ID) string {
	return filepath.Join(l.Directory, string(sessionID)+tarExt)
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
