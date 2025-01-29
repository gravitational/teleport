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
	"github.com/gravitational/trace"
)

// TODO: Replace logger when pyroscope uses slog
type pyroscopeLogger struct {
	l *slog.Logger
}

func (l pyroscopeLogger) Infof(format string, args ...interface{}) {
	//nolint:sloglint // msg cannot be constant
	l.l.Info(fmt.Sprintf(format, args...))
}

func (l pyroscopeLogger) Debugf(format string, args ...interface{}) {
	//nolint:sloglint // msg cannot be constant
	l.l.Debug(fmt.Sprintf(format, args...))
}

func (l pyroscopeLogger) Errorf(format string, args ...interface{}) {
	//nolint:sloglint // msg cannot be constant
	l.l.Error(fmt.Sprintf(format, args...))
}

// createPyroscopeConfig generates the Pyroscope configuration for the Teleport process.
func createPyroscopeConfig(ctx context.Context, address string) (pyroscope.Config, error) {
	if address == "" {
		return pyroscope.Config{}, trace.BadParameter("pyroscope address is empty")
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

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
		slog.InfoContext(ctx, "No profile types enabled, using default")
	} else {
		config.ProfileTypes = p
		slog.InfoContext(ctx, "Pyroscope will configure profiles from env")
	}

	var uploadRate *time.Duration
	if rate := os.Getenv("TELEPORT_PYROSCOPE_UPLOAD_RATE"); rate != "" {
		parsedRate, err := time.ParseDuration(rate)
		if err != nil {
			slog.InfoContext(ctx, "invalid TELEPORT_PYROSCOPE_UPLOAD_RATE, ignoring value", "provided_value", rate, "error", err)
		} else {
			uploadRate = &parsedRate
		}
	} else {
		slog.InfoContext(ctx, "TELEPORT_PYROSCOPE_UPLOAD_RATE not specified, using default")
	}

	// Set UploadRate or fall back to defaults
	if uploadRate != nil {
		config.UploadRate = *uploadRate
	}

	if value, isSet := os.LookupEnv("TELEPORT_PYROSCOPE_KUBE_COMPONENT"); isSet {
		config.Tags["component"] = value
	}

	if value, isSet := os.LookupEnv("TELEPORT_PYROSCOPE_KUBE_NAMESPACE"); isSet {
		config.Tags["namespace"] = value
	}

	return config, nil
}

// initPyroscope instruments Teleport to run with continuous profiling for Pyroscope
func (process *TeleportProcess) initPyroscope(address string) {
	config, err := createPyroscopeConfig(process.ExitContext(), address)
	if err != nil {
		slog.ErrorContext(process.ExitContext(), "failed to create Pyroscope config", "address", address, "error", err)
		return
	}

	profiler, err := pyroscope.Start(config)
	if err != nil {
		slog.ErrorContext(process.ExitContext(), "error starting pyroscope profiler", "address", address, "error", err)
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
