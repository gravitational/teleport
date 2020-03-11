/*
Copyright 2018 Gravitational, Inc.

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
	"os"
	"path/filepath"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// Config is a file uploader configuration
type Config struct {
	// Directory is a directory with files
	Directory string
}

// CheckAndSetDefaults checks and sets default values of file handler config
func (s *Config) CheckAndSetDefaults() error {
	if s.Directory == "" {
		return trace.BadParameter("missing parameter Directory")
	}
	if !utils.IsDir(s.Directory) {
		return trace.BadParameter("path %q does not exist or is not a directory", s.Directory)
	}
	return nil
}

// NewHandler returns new file sessions handler
func NewHandler(cfg Config) (*Handler, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	h := &Handler{
		Entry: log.WithFields(log.Fields{
			trace.Component: teleport.Component(teleport.SchemeFile),
		}),
		Config: cfg,
	}
	return h, nil
}

// Handler uploads and downloads sessions archives by reading
// and writing files to directory, useful for NFS setups and tests
type Handler struct {
	// Config is a file sessions config
	Config
	// Entry is a file entry
	*log.Entry
}

// Closer releases connection and resources associated with log if any
func (l *Handler) Close() error {
	return nil
}

// Download downloads session recording from storage, in case of file handler reads the
// file from local directory
func (l *Handler) Download(ctx context.Context, sessionID session.ID, writer io.WriterAt) error {
	path := l.path(sessionID)
	_, err := os.Stat(filepath.Dir(path))
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
	defer f.Close()

	_, err = io.Copy(f, reader)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// If the upload context was canceled, clear out the session recording.
	select {
	case <-ctx.Done():
		err = os.Remove(path)
		if err != nil {
			return "", trace.Wrap(err)
		}
	default:
	}

	return fmt.Sprintf("%v://%v", teleport.SchemeFile, path), nil
}

func (l *Handler) path(sessionID session.ID) string {
	return filepath.Join(l.Directory, string(sessionID)+".tar")
}
