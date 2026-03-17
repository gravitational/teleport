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

// systemdUnitActiveState is the ActiveState property of a systemd unit.
// The values and descriptions below are copied from the official
// org.freedesktop.systemd1 D-Bus interface documentation.
type systemdUnitState string

const (
	// systemdUnitActive means started, bound, plugged in, …, depending on the unit type.
	systemdUnitActive systemdUnitState = "active"
	// systemdUnitInactive means stopped, unbound, unplugged, …, depending on the unit type.
	systemdUnitInactive systemdUnitState = "inactive"
	// systemdUnitFailed means similar to inactive, but the unit failed in some way (process returned error code on exit, crashed, an operation timed out, or after too many restarts).
	systemdUnitFailed systemdUnitState = "failed"
	// systemdUnitActivating means changing from inactive to active.
	systemdUnitActivating systemdUnitState = "activating"
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

	for {
		select {
		case <-ctx.Done():
			stopCtx, stopCancel := context.WithTimeout(context.Background(), terminateTimeout)
			defer stopCancel()
			log.InfoContext(stopCtx, "Context canceled, stopping systemd service")
			if err := stopService(stopCtx); err != nil {
				return trace.Wrap(err, "sending stop request to systemd service %s", vnetSystemdUnitName)
			}
			err := waitForServiceStop(stopCtx, vnetSystemdUnitName)
			if err != nil {
				return trace.Wrap(err, "systemd service %s failed to stop with %v", vnetSystemdUnitName, terminateTimeout)
			}
			log.InfoContext(stopCtx, "Successfully stopped systemd service")
			return nil
		case <-ticker.C:
			state, err := getSystemdUnitState(ctx, conn, vnetSystemdUnitName)
			if err != nil {
				return trace.Wrap(err, "querying systemd service %s", vnetSystemdUnitName)
			}
			if state != systemdUnitActive && state != systemdUnitActivating {
				return trace.Errorf("service stopped running prematurely, status: %s", state)
			}
		}
	}
}

func waitForServiceStop(ctx context.Context, unit string) error {
	// Open a fresh connection here because the main loop's connection is bound to
	// the original context and may be closed when that context is canceled.
	conn, err := systemddbus.NewWithContext(ctx)
	if err != nil {
		return trace.NotFound("systemd D-Bus is unavailable: %v", err)
	}
	defer conn.Close()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			state, err := getSystemdUnitState(ctx, conn, unit)
			if err != nil {
				return trace.Wrap(err, "querying systemd service %s", unit)
			}
			if state == systemdUnitInactive || state == systemdUnitFailed {
				return nil
			}
		}
	}
}

func getSystemdUnitState(ctx context.Context, conn *systemddbus.Conn, unit string) (systemdUnitState, error) {
	props, err := conn.GetUnitPropertiesContext(ctx, unit)
	if err != nil {
		return "", err
	}
	state, ok := props["ActiveState"].(string)
	if !ok || state == "" {
		return "", trace.Errorf("systemd ActiveState is missing for %s", unit)
	}
	return systemdUnitState(state), nil
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
	// TODO(tangyatsu): Maybe also check the systemd unit file exists. D-Bus can report a name
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
