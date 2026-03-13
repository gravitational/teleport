// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package debug

import (
	"fmt"
	"log/slog"

	"github.com/gravitational/trace"

	debugpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/debug/v1"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// ServiceConfig holds the dependencies for the debug service logic.
type ServiceConfig struct {
	Logger      *slog.Logger
	Leveler     LogLeveler
	Broadcaster *logutils.LogBroadcaster
}

// Service implements the shared debug service logic used by both
// HTTP handlers (local) and the gRPC service (remote).
type Service struct {
	logger      *slog.Logger
	leveler     LogLeveler
	broadcaster *logutils.LogBroadcaster
}

// NewService creates a new debug service.
func NewService(cfg ServiceConfig) *Service {
	return &Service{
		logger:      cfg.Logger,
		leveler:     cfg.Leveler,
		broadcaster: cfg.Broadcaster,
	}
}

// GetLogLevel returns the current log level as a string.
func (s *Service) GetLogLevel() string {
	return marshalLogLevel(s.leveler.GetLogLevel())
}

// SetLogLevel sets the log level from a string and returns a status message.
func (s *Service) SetLogLevel(levelStr string) (string, error) {
	level, err := unmarshalLogLevel([]byte(levelStr))
	if err != nil {
		return "", trace.Wrap(err)
	}
	curr := s.leveler.GetLogLevel()
	msg := fmt.Sprintf("Log level already set to %q.", marshalLogLevel(level))
	if level != curr {
		msg = fmt.Sprintf("Changed log level from %q to %q.", marshalLogLevel(curr), marshalLogLevel(level))
		s.leveler.SetLogLevel(level)
		s.logger.Info("Changed log level.", "old", marshalLogLevel(curr), "new", marshalLogLevel(level))
	}
	return msg, nil
}

// SubscribeLogs creates a log stream subscription at the given minimum level.
// Returns the channel, a cleanup function, and an error if too many subscribers.
func (s *Service) SubscribeLogs(levelStr string) (<-chan *debugpb.LogEntry, func(), error) {
	minLevel := logutils.TraceLevel
	if levelStr != "" {
		parsed, err := unmarshalLogLevel([]byte(levelStr))
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		minLevel = parsed
	}
	ch := s.broadcaster.Subscribe(minLevel)
	if ch == nil {
		return nil, nil, trace.LimitExceeded("too many active log stream subscribers")
	}
	cleanup := func() { s.broadcaster.Unsubscribe(ch) }
	return ch, cleanup, nil
}
