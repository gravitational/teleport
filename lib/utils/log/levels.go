// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package log

import (
	"log/slog"

	"github.com/sirupsen/logrus"
)

// SlogLevelToLogrusLevel converts a [slog.Level] to its equivalent
// [logrus.Level].
func SlogLevelToLogrusLevel(level slog.Level) logrus.Level {
	switch level {
	case TraceLevel:
		return logrus.TraceLevel
	case slog.LevelDebug:
		return logrus.DebugLevel
	case slog.LevelInfo:
		return logrus.InfoLevel
	case slog.LevelWarn:
		return logrus.WarnLevel
	case slog.LevelError:
		return logrus.ErrorLevel
	default:
		return logrus.FatalLevel
	}
}
