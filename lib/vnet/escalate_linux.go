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

package vnet

import (
	"context"
	"os"
	"os/exec"
	"slices"
	"time"

	systemddbus "github.com/coreos/go-systemd/v22/dbus"
	"github.com/godbus/dbus/v5"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
)

const (
	terminateTimeout = 30 * time.Second
)

func execAdminProcess(ctx context.Context, cfg LinuxAdminProcessConfig) error {
	if err := checkDBusServiceAvailability(ctx); err != nil {
		if os.Geteuid() == 0 {
			log.WarnContext(ctx, "VNet daemon not available via D-Bus, running daemon as a child process", "error", err)
			return trace.Wrap(runAdminSubcommand(ctx, cfg))
		} else {
			return trace.Wrap(err)
		}
	}

	return trace.Wrap(runService(ctx, cfg))
}

func runService(ctx context.Context, cfg LinuxAdminProcessConfig) error {
	err := startService(ctx, cfg)
	if err != nil {
		return trace.Wrap(err)
	}
	log.InfoContext(ctx, "Started systemd service", "service", vnetSystemdUnitName)

	conn, err := systemddbus.NewWithContext(ctx)
	if err != nil {
		return trace.NotFound("systemd D-Bus is unavailable: %v", err)
	}
	defer conn.Close()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

loop:
	for {
		select {
		case <-ctx.Done():
			log.InfoContext(ctx, "Context canceled, stopping systemd service")
			err := stopService(ctx)
			if err != nil {
				return trace.Wrap(err)
			}
			break loop
		case <-ticker.C:
			state, err := systemdUnitActiveState(ctx, conn, vnetSystemdUnitName)
			if err != nil {
				return trace.Wrap(err, "querying systemd service %s", vnetSystemdUnitName)
			}
			if state != "active" && state != "activating" {
				return trace.Errorf("service stopped running prematurely, status: %s", state)
			}
		}
	}

	// Wait for the service to actually stop
	deadline := time.After(terminateTimeout + 5*time.Second)
	for {
		select {
		case <-deadline:
			return trace.Errorf("systemd service %s failed to stop with %v", vnetSystemdUnitName, terminateTimeout)
		case <-ticker.C:
			state, err := systemdUnitActiveState(ctx, conn, vnetSystemdUnitName)
			if err != nil {
				return trace.Wrap(err, "querying systemd service %s", vnetSystemdUnitName)
			}
			if state == "inactive" {
				return nil
			}
		}
	}
}

func systemdUnitActiveState(ctx context.Context, conn *systemddbus.Conn, unit string) (string, error) {
	props, err := conn.GetUnitPropertiesContext(ctx, unit)
	if err != nil {
		return "", err
	}
	state, ok := props["ActiveState"].(string)
	if !ok || state == "" {
		return "", trace.Errorf("systemd ActiveState is missing for %s", unit)
	}
	return state, nil
}

func checkDBusServiceAvailability(ctx context.Context) error {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return trace.Wrap(err, "system D-Bus is unavailable")
	}
	defer conn.Close()

	// Check if the service is already running.
	var hasOwner bool
	err = conn.Object("org.freedesktop.DBus", "/org/freedesktop/DBus").
		CallWithContext(ctx, "org.freedesktop.DBus.NameHasOwner", 0, vnetDBusServiceName).
		Store(&hasOwner)
	if err != nil {
		return trace.Wrap(err, "checking D-Bus service owner")
	}
	if hasOwner {
		return nil
	}

	// If it is not running, check that it can be activated via D-Bus.
	var services []string
	err = conn.Object("org.freedesktop.DBus", "/org/freedesktop/DBus").
		CallWithContext(ctx, "org.freedesktop.DBus.ListActivatableNames", 0).
		Store(&services)
	if err != nil {
		return trace.Wrap(err, "listing activatable D-Bus names")
	}

	if slices.Contains(services, vnetDBusServiceName) {
		return nil
	} else {
		return trace.Errorf("D-Bus service %s is not available", vnetDBusServiceName)
	}
	// TODO: Maybe also check the systemd unit file exists. D-Bus can report a name
	// as activatable even if the corresponding systemd unit is missing.
}

func runAdminSubcommand(ctx context.Context, cfg LinuxAdminProcessConfig) error {
	executableName, err := os.Executable()
	if err != nil {
		return trace.Wrap(err, "getting executable path")
	}

	cmd := exec.CommandContext(ctx, executableName, "-d",
		teleport.VnetAdminSetupSubCommand,
		"--addr", cfg.ClientApplicationServiceAddr,
		"--cred-path", cfg.ServiceCredentialPath,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return trace.Wrap(cmd.Run(), "running %s", teleport.VnetAdminSetupSubCommand)
}
