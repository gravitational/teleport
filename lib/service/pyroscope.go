// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package service

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/grafana/pyroscope-go"

	"github.com/gravitational/teleport"
)

// TODO: Replace logger when pyroscope uses slog
type pyroscopeLogger struct {
	l *slog.Logger
}

func (l pyroscopeLogger) Infof(format string, args ...interface{}) {
	if !l.l.Handler().Enabled(context.Background(), slog.LevelInfo) {
		return
	}
	//nolint:sloglint // msg cannot be constant
	l.l.Info(fmt.Sprintf(format, args...))
}

func (l pyroscopeLogger) Debugf(format string, args ...interface{}) {
	if !l.l.Handler().Enabled(context.Background(), slog.LevelDebug) {
		return
	}

	//nolint:sloglint // msg cannot be constant
	l.l.Debug(fmt.Sprintf(format, args...))
}

func (l pyroscopeLogger) Errorf(format string, args ...interface{}) {
	if !l.l.Handler().Enabled(context.Background(), slog.LevelError) {
		return
	}

	//nolint:sloglint // msg cannot be constant
	l.l.Error(fmt.Sprintf(format, args...))
}

// initPyroscope instruments Teleport to run with continuous profiling for Pyroscope
func (process *TeleportProcess) initPyroscope(address string) {
	if address == "" {
		return
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	// Build pyroscope config
	config := pyroscope.Config{
		ApplicationName: teleport.ComponentTeleport,
		ServerAddress:   address,
		Logger:          pyroscope.Logger(pyroscopeLogger{l: slog.Default()}),
		Tags: map[string]string{
			"host":    hostname,
			"version": teleport.Version,
			"git_ref": teleport.Gitref,
		},
	}

	// Evaluate if profile configuration is customized
	if p := getPyroscopeProfileTypesFromEnv(); len(p) == 0 {
		slog.InfoContext(process.ExitContext(), "No profile types enabled, using default")
	} else {
		config.ProfileTypes = p
		slog.InfoContext(process.ExitContext(), "Pyroscope will configure profiles from env")
	}

	var uploadRate *time.Duration
	if rate := os.Getenv("TELEPORT_PYROSCOPE_UPLOAD_RATE"); rate != "" {
		parsedRate, err := time.ParseDuration(rate)
		if err != nil {
			slog.InfoContext(process.ExitContext(), "invalid TELEPORT_PYROSCOPE_UPLOAD_RATE, ignoring value", "provided_value", rate, "error", err)
		} else {
			uploadRate = &parsedRate
		}
	} else {
		slog.InfoContext(process.ExitContext(), "TELEPORT_PYROSCOPE_UPLOAD_RATE not specified, using default")
	}

	// Set UploadRate or fall back to defaults
	if uploadRate != nil {
		config.UploadRate = *uploadRate
	}

	// If set, check for Kubernetes enrichment from downward API
	if os.Getenv("TELEPORT_PYROSCOPE_KUBE_ENV") == "true" {
		config.Tags = addKubeTagsFromEnv(config.Tags)
		slog.InfoContext(process.ExitContext(), "Pyroscope will configure tags for Kubernetes env if set.")
	}

	profiler, err := pyroscope.Start(config)
	if err != nil {
		slog.ErrorContext(process.ExitContext(), "error starting pyroscope profiler", "error", err)
	} else {
		process.OnExit("pyroscope.profiler", func(payload any) {
			profiler.Flush(payload == nil)
			_ = profiler.Stop()
		})
	}
	slog.InfoContext(process.ExitContext(), "Pyroscope has successfully started")
}

// getPyroscopeProfileTypesFromEnv sets the profile types based on environment variables.
func getPyroscopeProfileTypesFromEnv() []pyroscope.ProfileType {
	var profileTypes []pyroscope.ProfileType

	if os.Getenv("TELEPORT_PYROSCOPE_PROFILE_MEMORY_ENABLED") == "true" {
		profileTypes = append(profileTypes,
			pyroscope.ProfileAllocObjects,
			pyroscope.ProfileAllocSpace,
			pyroscope.ProfileInuseObjects,
			pyroscope.ProfileInuseSpace,
		)
	}

	if os.Getenv("TELEPORT_PYROSCOPE_PROFILE_CPU_ENABLED") == "true" {
		profileTypes = append(profileTypes, pyroscope.ProfileCPU)
	}

	if os.Getenv("TELEPORT_PYROSCOPE_PROFILE_GOROUTINES_ENABLED") == "true" {
		profileTypes = append(profileTypes, pyroscope.ProfileGoroutines)
	}

	return profileTypes
}

// getTagsFromKubeEnv extracts Kubernetes metadata passed from downward API and returns them if set.
func addKubeTagsFromEnv(tags map[string]string) map[string]string {

	env := map[string]string{
		"name":      "TELEPORT_PYROSCOPE_KUBE_NAME",
		"instance":  "TELEPORT_PYROSCOPE_KUBE_INSTANCE",
		"component": "TELEPORT_PYROSCOPE_KUBE_COMPONENT",
		"namespace": "TELEPORT_PYROSCOPE_KUBE_NAMESPACE",
		"region":    "TELEPORT_PYROSCOPE_KUBE_REGION",
	}

	for k, v := range env {
		if value, isSet := os.LookupEnv(v); isSet {
			tags[k] = value
		}
	}

	return tags
}
