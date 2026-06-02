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

package x11

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/godbus/dbus/v5"
	"github.com/gravitational/trace"
)

const (
	mutterDisplayConfigService = "org.gnome.Mutter.DisplayConfig"
	mutterDisplayConfigPath    = "/org/gnome/Mutter/DisplayConfig"

	// applyMethodTemporary applies the config for this session only (not
	// persisted to monitors.xml). Our Xvfb mode names are session-scoped, so a
	// persisted config would reference modes that won't exist next session.
	applyMethodTemporary uint32 = 1
)

// monitorID matches the (ssss) struct in mutter's DisplayConfig schema:
// (connector, vendor, product, serial).
type monitorID struct {
	Connector string
	Vendor    string
	Product   string
	Serial    string
}

// displayMode matches the (siiddada{sv}) mode struct from GetCurrentState:
// (id, width, height, refresh, preferred_scale, supported_scales, props).
type displayMode struct {
	ID              string
	Width           int32
	Height          int32
	RefreshRate     float64
	PreferredScale  float64
	SupportedScales []float64
	Properties      map[string]dbus.Variant
}

// physicalMonitor matches a single ((ssss)a(siiddada{sv})a{sv}) entry in
// the monitors array.
type physicalMonitor struct {
	ID         monitorID
	Modes      []displayMode
	Properties map[string]dbus.Variant
}

// logicalMonitor matches a single (iiduba(ssss)a{sv}) entry in
// GetCurrentState's logical_monitors array.
type logicalMonitor struct {
	X          int32
	Y          int32
	Scale      float64
	Transform  uint32
	Primary    bool
	Monitors   []monitorID
	Properties map[string]dbus.Variant
}

// monitorAssignment matches the (ssa{sv}) struct in
// ApplyMonitorsConfig's logical_monitors argument: (connector, mode_id,
// properties).
type monitorAssignment struct {
	Connector  string
	ModeID     string
	Properties map[string]dbus.Variant
}

// logicalMonitorConfig matches the (iiduba(ssa{sv})) struct that
// ApplyMonitorsConfig accepts (note the simpler nested monitor type
// compared to logicalMonitor above).
type logicalMonitorConfig struct {
	X         int32
	Y         int32
	Scale     float64
	Transform uint32
	Primary   bool
	Monitors  []monitorAssignment
}

// findSessionBusAddress walks /proc for a desktop-session process owned by
// login and returns its DBUS_SESSION_BUS_ADDRESS. dbus-run-session creates a
// private bus at a temp path and exports the address only to its descendants,
// so there's no other way to find it from outside the session.
func findSessionBusAddress(login string) (string, error) {
	u, err := user.Lookup(login)
	if err != nil {
		return "", trace.Wrap(err)
	}
	uidPrefix := fmt.Sprintf("Uid:\t%s\t", u.Uid)

	procs, err := filepath.Glob("/proc/[0-9]*")
	if err != nil {
		return "", trace.Wrap(err)
	}
	for _, p := range procs {
		status, err := os.ReadFile(filepath.Join(p, "status"))
		if err != nil || !strings.Contains(string(status), uidPrefix) {
			continue
		}
		cmdline, err := os.ReadFile(filepath.Join(p, "cmdline"))
		if err != nil {
			continue
		}
		argv0 := string(cmdline)
		if !strings.Contains(argv0, "gnome-session-binary") &&
			!strings.Contains(argv0, "gnome-shell") {
			continue
		}
		environ, err := os.ReadFile(filepath.Join(p, "environ"))
		if err != nil {
			continue
		}
		for _, kv := range strings.Split(string(environ), "\x00") {
			if v, ok := strings.CutPrefix(kv, "DBUS_SESSION_BUS_ADDRESS="); ok && v != "" {
				return v, nil
			}
		}
	}
	return "", trace.NotFound("no DBUS_SESSION_BUS_ADDRESS found for user %q", login)
}

// ApplyGNOMEDisplayScale switches the GNOME session to the given mode
// (width x height px) at the given integer scale via mutter's DisplayConfig
// D-Bus API, letting mutter drive the RandR switch (the same path as
// gnome-control-center's live rescale). We avoid our own backend.Resize /
// SetScreenConfig here: that makes mutter autonomously activate HiDPI on the
// ScreenChangeNotify, which on Xvfb severs gnome-shell during 1x to 2x.
//
// Callers must first add a (width, height) RandR mode via Backend.EnsureMode
// so mutter has a mode_id to select.
func ApplyGNOMEDisplayScale(ctx context.Context, logger *slog.Logger, login string, width, height uint16, scale float64) error {
	busAddr, err := findSessionBusAddress(login)
	if err != nil {
		return trace.Wrap(err)
	}
	logger.DebugContext(ctx, "ApplyGNOMEDisplayScale: connecting",
		"login", login, "width", width, "height", height, "scale", scale, "bus", busAddr)

	conn, err := dbus.Dial(busAddr, dbus.WithContext(ctx))
	if err != nil {
		return trace.Wrap(err, "dialing session bus")
	}
	defer conn.Close()

	if err := conn.Auth(nil); err != nil {
		return trace.Wrap(err, "authenticating to session bus")
	}
	if err := conn.Hello(); err != nil {
		return trace.Wrap(err, "saying hello on session bus")
	}

	obj := conn.Object(mutterDisplayConfigService, dbus.ObjectPath(mutterDisplayConfigPath))

	var (
		serial   uint32
		monitors []physicalMonitor
		logical  []logicalMonitor
		props    map[string]dbus.Variant
	)
	err = obj.CallWithContext(ctx, mutterDisplayConfigService+".GetCurrentState", 0).
		Store(&serial, &monitors, &logical, &props)
	if err != nil {
		return trace.Wrap(err, "GetCurrentState")
	}
	logger.DebugContext(ctx, "ApplyGNOMEDisplayScale: GetCurrentState",
		"serial", serial, "monitors", len(monitors), "logical", len(logical))
	if len(logical) == 0 {
		return trace.BadParameter("no logical monitors in current state")
	}
	for _, mon := range monitors {
		for _, m := range mon.Modes {
			logger.DebugContext(ctx, "ApplyGNOMEDisplayScale: mode",
				"connector", mon.ID.Connector, "id", m.ID,
				"w", m.Width, "h", m.Height,
				"preferred_scale", m.PreferredScale, "supported_scales", m.SupportedScales)
		}
	}

	newLogical := make([]logicalMonitorConfig, 0, len(logical))
	for _, lm := range logical {
		if len(lm.Monitors) == 0 {
			continue
		}
		connector := lm.Monitors[0].Connector
		modeID := modeIDForSize(monitors, connector, int32(width), int32(height))
		if modeID == "" {
			return trace.NotFound("no mode for connector %q at %dx%d (EnsureMode not yet visible to mutter?)", connector, width, height)
		}
		logger.DebugContext(ctx, "ApplyGNOMEDisplayScale: logical monitor",
			"connector", connector, "mode_id", modeID,
			"old_scale", lm.Scale, "new_scale", scale)
		newLogical = append(newLogical, logicalMonitorConfig{
			X:         lm.X,
			Y:         lm.Y,
			Scale:     scale,
			Transform: lm.Transform,
			Primary:   lm.Primary,
			Monitors: []monitorAssignment{{
				Connector:  connector,
				ModeID:     modeID,
				Properties: map[string]dbus.Variant{},
			}},
		})
	}

	err = obj.CallWithContext(ctx, mutterDisplayConfigService+".ApplyMonitorsConfig", 0,
		serial, applyMethodTemporary, newLogical, map[string]dbus.Variant{}).Err
	if err != nil {
		return trace.Wrap(err, "ApplyMonitorsConfig")
	}
	logger.DebugContext(ctx, "ApplyGNOMEDisplayScale: applied",
		"width", width, "height", height, "scale", scale)
	return nil
}

// modeIDForSize returns the mode ID for the physical monitor on the
// given connector whose pixel dimensions match (width, height), or ""
// if no such mode is exposed by mutter.
func modeIDForSize(monitors []physicalMonitor, connector string, width, height int32) string {
	for _, mon := range monitors {
		if mon.ID.Connector != connector {
			continue
		}
		for _, m := range mon.Modes {
			if m.Width == width && m.Height == height {
				return m.ID
			}
		}
		return ""
	}
	return ""
}
