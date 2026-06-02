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
	"bufio"
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"math"
	"net"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/jezek/xgb"
	"github.com/jezek/xgb/damage"
	"github.com/jezek/xgb/randr"
	"github.com/jezek/xgb/xfixes"
	"github.com/jezek/xgb/xproto"
	"github.com/jezek/xgb/xtest"

	"github.com/gravitational/teleport/api/types"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var asciiRE = regexp.MustCompile("[[:^ascii:]]")

var modeCount atomic.Uint64

const (
	dpi = 96
)

func init() {
	xgb.Logger = log.New(io.Discard, "", 0)
}

// Config contains configuration for X11 backend.
type Config struct {
	// ClipboardDataReceiver is consumer for clipboard data received from X11.
	ClipboardDataReceiver func([]byte)
	// CursorReceiver consumes cursor sprite updates from the X server on every
	// display-cursor change. bgra is width*height*4 bytes, BGRA top-down
	// (little-endian ARGB uint32 layout).
	CursorReceiver func(width, height, hotspotX, hotspotY uint16, bgra []byte)
	Logger         *slog.Logger
}

// Backend is responsible for communication with selected X11 backend.
// It handles mouse and keyboard inputs, clipboard interactions, resizing and tracking changed regions.
type Backend struct {
	mu     sync.Mutex
	config Config

	ctx    context.Context
	cancel context.CancelFunc

	cmd    *exec.Cmd
	conn   *xgb.Conn
	setup  *xproto.SetupInfo
	damage damage.Damage

	// Display contains X11 display string (:N) for started backend
	Display string

	clipboardData   []byte
	clipboardWindow xproto.Window
	clipboardAtom   xproto.Atom
	targetsAtom     xproto.Atom
	utf8Atom        xproto.Atom
	selectionAtom   xproto.Atom

	// AuthorityFile is XAuthority file used for securing X11 socket, it'll be deleted when backend is closed
	AuthorityFile   string
	authorityCookie []byte

	// these fields are used for restoring target size after some other process (e.g. desktop environment) changes it
	width           uint16
	height          uint16
	resizeTimestamp xproto.Timestamp
}

// IsBackendPresent reports whether the binary required by selected backend is available in PATH.
func IsBackendPresent() bool {
	switch backend := os.Getenv("TELEPORT_LINUX_DESKTOP_BACKEND"); backend {
	case "", "xvfb", "xvfb-tcp":
		_, err := exec.LookPath("Xvfb")
		return err == nil
	case "xephyr":
		_, err := exec.LookPath("Xephyr")
		return err == nil
	default:
		return false
	}
}

func isBackendSafe() bool {
	switch backend := os.Getenv("TELEPORT_LINUX_DESKTOP_BACKEND"); backend {
	case "", "xvfb":
		return true
	default:
		return false
	}
}

func getBackendCommand(ctx context.Context, authorityFile string) (*exec.Cmd, error) {
	switch backend := os.Getenv("TELEPORT_LINUX_DESKTOP_BACKEND"); backend {
	case "", "xvfb":
		return exec.CommandContext(ctx, "Xvfb",
			"-displayfd", "1",
			"-screen", "0", fmt.Sprintf("%dx%dx24", types.MaxRDPScreenWidth, types.MaxRDPScreenHeight),
			"-nolock",
			"-dpi", fmt.Sprintf("%d", dpi),
			"-dpms",
			"-s", "off",
			"-nolisten", "tcp",
			"-auth", authorityFile), nil
	case "xvfb-tcp":
		// This backend allows to run multiple sessions on macOS, otherwise displayfd always return 0.
		// This will open TCP socket so it's less secure, and it can only handle around 64K display at
		// once - it's intended only for testing
		return exec.CommandContext(ctx, "Xvfb",
			"-displayfd", "1",
			"-screen", "0", fmt.Sprintf("%dx%dx24", types.MaxRDPScreenWidth, types.MaxRDPScreenHeight),
			"-nolock",
			"-dpi", fmt.Sprintf("%d", dpi),
			"-dpms",
			"-s", "off",
			"-listen", "tcp",
			"-audit", "4",
			"-auth", authorityFile), nil
	case "xephyr":
		// This backend starts nested X11 server using Xephyr. It requires already present server and
		// DISPLAY environment variable defined. It's intended only for testing and debugging to see
		// what's happening in the session
		return exec.CommandContext(ctx, "Xephyr",
			"-displayfd", "1",
			"-screen", fmt.Sprintf("%dx%dx24", types.MaxRDPScreenWidth, types.MaxRDPScreenHeight),
			"-nolock",
			"-listen", "tcp",
			"-auth", authorityFile), nil
	default:
		return nil, trace.BadParameter("unsupported backend: %q", backend)
	}
}

// pixelsToMm converts a pixel count to the physical mm the X server advertises.
// We always report a fixed 96 DPI: mutter watches the X-reported DPI and
// autonomously enables HiDPI at >= ~192, which fatally disconnects on Xvfb.
// HiDPI is instead driven via per-DE XSETTINGS (and ApplyMonitorsConfig for GNOME).
func pixelsToMm(pixels uint16) uint32 {
	return uint32(float64(pixels) / dpi * 25.4)
}

func internAtom(conn *xgb.Conn, atom string) (xproto.Atom, error) {
	reply, err := xproto.InternAtom(conn, false, uint16(len(atom)), atom).Reply()
	if err != nil {
		return xproto.AtomNone, trace.Wrap(err)
	}
	return reply.Atom, nil
}

// NewBackend starts a selected backend server and returns a connected client wrapper for interacting with the display.
func NewBackend(ctx context.Context, config Config) (*Backend, error) {
	if config.Logger == nil {
		return nil, trace.BadParameter("missing parameter config.Logger")
	}
	if !IsBackendPresent() {
		return nil, trace.NotFound("Backend is not installed")
	}

	if !isBackendSafe() {
		config.Logger.WarnContext(ctx, "Selected backend is not safe for production usage! Please use 'xvfb' instead.")
	}

	ctx, cancel := context.WithCancel(ctx)

	var success bool
	defer func() {
		if !success {
			cancel()
		}
	}()

	authorityFile, err := os.CreateTemp("", "teleport-x11-")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer func() {
		if !success {
			os.Remove(authorityFile.Name())
		}
	}()

	cmd, err := getBackendCommand(ctx, authorityFile.Name())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cmd.WaitDelay = 5 * time.Second

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			config.Logger.Log(ctx, logutils.TraceLevel, "backend output", "line", line)
		}
	}()

	if err := cmd.Start(); err != nil {
		return nil, trace.Wrap(err)
	}

	defer func() {
		if !success {
			cancel()
			cmd.Wait()
		}
	}()

	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() {
		if scanner.Err() != nil {
			return nil, trace.Wrap(scanner.Err(), "reading display string")
		}

		return nil, trace.BadParameter("no display string found")
	}
	display := scanner.Text()

	display = ":" + strings.TrimPrefix(display, ":")

	cookie := make([]byte, 16)
	if _, err := rand.Read(cookie); err != nil {
		return nil, trace.Wrap(err)
	}

	entry, err := generateXauthorityEntry(display, cookie)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if _, err := authorityFile.Write(entry); err != nil {
		return nil, trace.Wrap(err)
	}
	authorityFile.Close()

	conn, setup, err := connectToDisplay(display, cookie)
	if err != nil {
		return nil, trace.Wrap(err, "connecting to display")
	}

	defer func() {
		if !success {
			conn.Close()
		}
	}()

	id, err := conn.NewId()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	damageID := damage.Damage(id)

	err = damage.CreateChecked(conn, damageID, xproto.Drawable(setup.Roots[0].Root), damage.ReportLevelNonEmpty).Check()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clipboardAtom, err := internAtom(conn, "CLIPBOARD")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	root := setup.Roots[0].Root
	if err := xfixes.SelectSelectionInputChecked(conn, root, clipboardAtom, xfixes.SelectionEventMaskSetSelectionOwner).Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := xfixes.SelectCursorInputChecked(conn, root, xfixes.CursorNotifyMaskDisplayCursor).Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	clipWindow, err := xproto.NewWindowId(conn)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := xproto.CreateWindowChecked(conn, 0, clipWindow, root, -10, -10, 1, 1, 0, xproto.WindowClassInputOnly, xproto.WindowClassCopyFromParent, 0, nil).Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	selectionAtom, err := internAtom(conn, "TELEPORT_SELECTION")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	targetsAtom, err := internAtom(conn, "TARGETS")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	utf8Atom, err := internAtom(conn, "UTF8_STRING")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Set DPI
	widthMm := pixelsToMm(types.MaxRDPScreenWidth)
	heightMm := pixelsToMm(types.MaxRDPScreenHeight)
	if err := randr.SetScreenSizeChecked(conn, root, types.MaxRDPScreenWidth, types.MaxRDPScreenHeight, widthMm, heightMm).Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	randr.SelectInput(conn, root, randr.NotifyMaskCrtcChange)

	x := &Backend{
		ctx:             ctx,
		config:          config,
		cmd:             cmd,
		conn:            conn,
		setup:           setup,
		cancel:          cancel,
		Display:         display,
		damage:          damageID,
		clipboardWindow: clipWindow,
		clipboardAtom:   clipboardAtom,
		targetsAtom:     targetsAtom,
		utf8Atom:        utf8Atom,
		selectionAtom:   selectionAtom,
		AuthorityFile:   authorityFile.Name(),
		authorityCookie: cookie,
		width:           types.MaxRDPScreenWidth,
		height:          types.MaxRDPScreenHeight,
		resizeTimestamp: math.MaxInt32,
	}

	go x.processEvents()

	success = true
	return x, nil
}

func (x *Backend) processEvents() {
	for {
		event, err := x.conn.WaitForEvent()
		if event == nil && err == nil {
			return
		}
		if err != nil {
			x.config.Logger.ErrorContext(x.ctx, "X11 error", "error", err.Error())
			continue
		}
		switch event := event.(type) {
		case damage.NotifyEvent:
		// do nothing, we handle changes through GetChanges
		case randr.NotifyEvent:
			if event.SubCode != randr.NotifyCrtcChange {
				continue
			}
			cc := event.U.Cc
			x.mu.Lock()
			width := x.width
			height := x.height
			timestamp := x.resizeTimestamp
			if cc.Timestamp >= timestamp && (cc.Width != width || cc.Height != height) {
				x.config.Logger.DebugContext(x.ctx, "Received external resolution change event", "width", cc.Width, "height", cc.Height)
				x.config.Logger.DebugContext(x.ctx, "Restoring desired resolution", "width", width, "height", height)
				if err := x.setScreenSizeLocked(width, height); err != nil {
					x.config.Logger.ErrorContext(x.ctx, "Couldn't restore resolution", "error", err)
				}
			}
			x.mu.Unlock()
		case xproto.SelectionRequestEvent:
			if event.Property == xproto.AtomNone {
				event.Property = x.selectionAtom
			}
			x.mu.Lock()
			data := x.clipboardData
			x.mu.Unlock()
			switch event.Target {
			case x.utf8Atom:
				xproto.ChangeProperty(x.conn, xproto.PropModeReplace, event.Requestor, event.Property, xproto.AtomString, 8, uint32(len(data)), data)
			case xproto.AtomString:
				data := asciiRE.ReplaceAllLiteralString(string(data), "")
				xproto.ChangeProperty(x.conn, xproto.PropModeReplace, event.Requestor, event.Property, xproto.AtomString, 8, uint32(len(data)), []byte(data))
			case x.targetsAtom:
				atoms := make([]byte, 0, 8)
				atoms = binary.LittleEndian.AppendUint32(atoms, uint32(x.utf8Atom))
				atoms = binary.LittleEndian.AppendUint32(atoms, xproto.AtomString)
				xproto.ChangeProperty(x.conn, xproto.PropModeReplace, event.Requestor, event.Property, xproto.AtomAtom, 32, 2, atoms)
			default:
				event.Property = xproto.AtomNone
			}
			xproto.SendEvent(x.conn, true, event.Requestor, 0, string(xproto.SelectionNotifyEvent{
				Time:      event.Time,
				Requestor: event.Requestor,
				Selection: event.Selection,
				Target:    event.Target,
				Property:  event.Property,
			}.Bytes()))
		case xproto.SelectionNotifyEvent:
			if x.config.ClipboardDataReceiver == nil {
				continue
			}
			prop, err := xproto.GetProperty(x.conn, true, x.clipboardWindow, event.Property, event.Target, 0, 1024*1024).Reply()
			if err != nil {
				x.config.Logger.ErrorContext(x.ctx, "Couldn't get X11 property value", "error", err)
				continue
			}
			if len(prop.Value) == 0 {
				continue
			}
			x.config.ClipboardDataReceiver(prop.Value)
		case xfixes.SelectionNotifyEvent:
			xproto.ConvertSelection(x.conn, x.clipboardWindow, x.clipboardAtom, x.utf8Atom, x.selectionAtom, 0)
		case xfixes.CursorNotifyEvent:
			x.SendCurrentCursor()
		default:
			x.config.Logger.WarnContext(x.ctx, "Unrecognized event", "event", event)
		}
	}
}

// SendCurrentCursor fetches the X server's current cursor and passes the sprite
// (BGRA top-down) to CursorReceiver; no-op if none is configured. Call only
// after ServerHello: the sprite ships as a FastPath PDU, which the web client
// can't process until its FastPathProcessor is initialized.
func (x *Backend) SendCurrentCursor() {
	if x.config.CursorReceiver == nil {
		return
	}
	reply, err := xfixes.GetCursorImage(x.conn).Reply()
	if err != nil {
		x.config.Logger.ErrorContext(x.ctx, "failed to fetch X cursor image", "error", err)
		return
	}
	if reply == nil || reply.Width == 0 || reply.Height == 0 {
		return
	}
	// XFixes returns the sprite as a stream of 32-bit ARGB pixels. xgb reads each
	// u32 little-endian, so the in-memory byte order of each pixel is B, G, R, A,
	// which is the byte order the RDP NewPointer PDU expects.
	bgra := make([]byte, int(reply.Width)*int(reply.Height)*4)
	for i, pix := range reply.CursorImage {
		bgra[i*4+0] = byte(pix)
		bgra[i*4+1] = byte(pix >> 8)
		bgra[i*4+2] = byte(pix >> 16)
		bgra[i*4+3] = byte(pix >> 24)
	}
	x.config.CursorReceiver(reply.Width, reply.Height, reply.Xhot, reply.Yhot, bgra)
}

func (x *Backend) root() xproto.Window {
	return x.setup.Roots[0].Root
}

// Close stops the Backend process and waits for it to exit.
func (x *Backend) Close() error {
	x.conn.Close()
	x.cancel()

	os.Remove(x.AuthorityFile)

	var e *exec.ExitError

	err := x.cmd.Wait()
	x.config.Logger.DebugContext(x.ctx, "Xvfb process exited",
		"error", err, "exit_code", x.cmd.ProcessState.ExitCode())
	if errors.As(err, &e) {
		return nil
	}
	return trace.Wrap(err)
}

func connectToDisplay(display string, cookie []byte) (*xgb.Conn, *xproto.SetupInfo, error) {
	success := false

	dial, err := net.Dial("unix", fmt.Sprintf("/tmp/.X11-unix/X%s", display[1:]))
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	defer func() {
		if !success {
			dial.Close()
		}
	}()

	conn, err := xgb.NewConnNetWithCookieHex(dial, fmt.Sprintf("%x", cookie))
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	setup := xproto.Setup(conn)

	defer func() {
		if !success {
			conn.Close()
		}
	}()

	if err := xtest.Init(conn); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	if err := xfixes.Init(conn); err != nil {
		return nil, nil, trace.Wrap(err)
	}
	_, err = xfixes.QueryVersion(conn, 5, 0).Reply()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	if err := damage.Init(conn); err != nil {
		return nil, nil, trace.Wrap(err)
	}
	_, err = damage.QueryVersion(conn, 1, 1).Reply()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	if err := randr.Init(conn); err != nil {
		return nil, nil, trace.Wrap(err)
	}
	_, err = randr.QueryVersion(conn, 5, 0).Reply()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	success = true
	return conn, setup, nil
}

// GetChanges returns regions changed since the last call of this method
func (x *Backend) GetChanges() (rectangles []xproto.Rectangle, err error) {
	id, err := x.conn.NewId()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	region := xfixes.Region(id)
	if err := xfixes.CreateRegionChecked(x.conn, region, nil).Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	defer xfixes.DestroyRegion(x.conn, region)
	if err := damage.SubtractChecked(x.conn, x.damage, xfixes.RegionNone, region).Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	fetched, err := xfixes.FetchRegion(x.conn, region).Reply()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return fetched.Rectangles, nil
}

// GetImage captures image data for the requested rectangle in RGBA format.
func (x *Backend) GetImage(rect xproto.Rectangle) ([]byte, error) {
	root := xproto.Drawable(x.root())
	reply, err := xproto.GetImage(x.conn, xproto.ImageFormatZPixmap, root, rect.X, rect.Y, rect.Width, rect.Height, math.MaxUint32).Reply()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	data := reply.Data
	if len(data)%4 != 0 {
		return nil, trace.BadParameter("image data is not a multiple of 4")
	}
	// Data returned from xproto.GetImage is BGRA and alpha is always 0, we change it to RGBA and alpha set to 0xFF
	for i := 0; i < len(data); i += 4 {
		data[i+0], data[i+2] = data[i+2], data[i+0]
		data[i+3] = 0xff
	}
	return data, nil
}

// SendKeyboardButton sends a key press or release event.
func (x *Backend) SendKeyboardButton(keycode byte, pressed bool) error {
	eventType := xproto.KeyRelease
	if pressed {
		eventType = xproto.KeyPress
	}
	err := xtest.FakeInputChecked(x.conn, byte(eventType), keycode+8, xproto.TimeCurrentTime, x.root(), 0, 0, 0).Check()
	return trace.Wrap(err)
}

// SendMouseButton sends a mouse button press or release event.
func (x *Backend) SendMouseButton(button byte, pressed bool) error {
	eventType := xproto.ButtonRelease
	if pressed {
		eventType = xproto.ButtonPress
	}
	err := xtest.FakeInputChecked(x.conn, byte(eventType), button+1, xproto.TimeCurrentTime, x.root(), 0, 0, 0).Check()
	return trace.Wrap(err)
}

const (
	wheelUp byte = iota + 3
	wheelDown
	horizontalWheelLeft
	horizontalWheelRight
)

// SendMouseWheel sends one wheel step event in the requested direction.
func (x *Backend) SendMouseWheel(delta int) error {
	if delta == 0 {
		return nil
	}
	button := wheelDown
	if delta > 0 {
		button = wheelUp
	}
	// In X11 wheel is simulated by "clicking" virtual button
	if err := x.SendMouseButton(button, true); err != nil {
		return err
	}
	return x.SendMouseButton(button, false)
}

// SendHorizontalMouseWheel sends one horizontal wheel step event in the requested direction.
func (x *Backend) SendHorizontalMouseWheel(delta int) error {
	if delta == 0 {
		return nil
	}
	button := horizontalWheelRight
	if delta > 0 {
		button = horizontalWheelLeft
	}
	// In X11 wheel is simulated by "clicking" virtual button
	if err := x.SendMouseButton(button, true); err != nil {
		return err
	}
	return x.SendMouseButton(button, false)
}

func (x *Backend) SendMouseMove(px, py int16) error {
	err := xtest.FakeInputChecked(x.conn, xproto.MotionNotify, 0, xproto.TimeCurrentTime, x.root(), px, py, 0).Check()
	return trace.Wrap(err)
}

func (x *Backend) SendNoOpEvent() error {
	err := xtest.FakeInputChecked(x.conn, xproto.MotionNotify, 0, xproto.TimeCurrentTime, xproto.WindowNone, 0, 0, 0).Check()
	return trace.Wrap(err)
}

// EnsureMode ensures the primary output has a RandR mode of the given size,
// creating one only if no same-dimension mode exists. The dedup matters because
// Resize runs once per ScreenChangeNotify during the gnome-shell startup stomp
// loop; without it the mode list stacks duplicates and setScreenSize's
// slices.IndexFunc picks a stale entry, leaving a black framebuffer.
//
// The mode is not activated; callers hand the mode_id to mutter via
// DisplayConfig.ApplyMonitorsConfig.
func (x *Backend) EnsureMode(width, height uint16) error {
	if width > types.MaxRDPScreenWidth || height > types.MaxRDPScreenHeight {
		return trace.BadParameter("invalid size %dx%d, maximum size is %dx%d", width, height, types.MaxRDPScreenWidth, types.MaxRDPScreenHeight)
	}

	conn := x.conn
	root := x.root()

	resources, err := randr.GetScreenResources(conn, root).Reply()
	if err != nil {
		return trace.Wrap(err)
	}
	if len(resources.Outputs) == 0 {
		return trace.BadParameter("no outputs available")
	}

	for _, m := range resources.Modes {
		if m.Width == width && m.Height == height {
			return nil
		}
	}

	modeName := fmt.Sprintf("m%d", modeCount.Add(1))
	id, err := conn.NewId()
	if err != nil {
		return trace.Wrap(err)
	}
	m, err := randr.CreateMode(conn, root, randr.ModeInfo{
		Id:      id,
		Width:   width,
		Height:  height,
		NameLen: uint16(len(modeName)),
	}, modeName).Reply()
	if err != nil {
		return trace.Wrap(err)
	}
	if err := randr.AddOutputModeChecked(conn, resources.Outputs[0], m.Mode).Check(); err != nil {
		return trace.Wrap(err)
	}
	x.config.Logger.DebugContext(x.ctx, "EnsureMode: created mode",
		"name", modeName, "mode_id", uint32(m.Mode), "width", width, "height", height)
	return nil
}

// SetDesired records the size the screen should settle at, used by the restore
// guard in processEvents to ignore stomps. Call after handing the resize to
// mutter via ApplyMonitorsConfig so the guard doesn't fight mutter's own switch.
func (x *Backend) SetDesired(width, height uint16) {
	x.mu.Lock()
	x.width = width
	x.height = height
	x.mu.Unlock()
}

// Resize changes the virtual screen size.
func (x *Backend) Resize(width, height uint16) error {
	if err := x.EnsureMode(width, height); err != nil {
		return trace.Wrap(err)
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	if err := x.setScreenSizeLocked(width, height); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// setScreenSizeLocked changes CRTC configuration to select correct mode for requested screen size.
// This method should be called under x.mu lock
func (x *Backend) setScreenSizeLocked(width, height uint16) error {
	conn := x.conn
	root := x.root()

	resources, err := randr.GetScreenResourcesCurrent(conn, root).Reply()
	if err != nil {
		return trace.Wrap(err)
	}

	modeIndex := slices.IndexFunc(resources.Modes, func(m randr.ModeInfo) bool {
		return m.Width == width && m.Height == height
	})
	if modeIndex == -1 {
		return trace.NotFound("could not find mode %dx%d", width, height)
	}

	modeID := resources.Modes[modeIndex].Id

	if len(resources.Crtcs) == 0 {
		return trace.NotFound("no CRTCs available")
	}
	if len(resources.Outputs) == 0 {
		return trace.NotFound("no outputs available")
	}

	output := resources.Outputs[0]

	crtc := resources.Crtcs[0]
	crtctIndex := slices.IndexFunc(resources.Crtcs, func(c randr.Crtc) bool {
		info, err := randr.GetCrtcInfo(conn, c, resources.ConfigTimestamp).Reply()
		return err == nil && slices.Contains(info.Outputs, output)
	})
	if crtctIndex != -1 {
		crtc = resources.Crtcs[crtctIndex]
	}

	// Use SetScreenSize with max dimensions so SetCrtcConfig doesn't reject
	// the new mode as exceeding the framebuffer
	maxMm := pixelsToMm(types.MaxRDPScreenWidth)
	if err := randr.SetScreenSizeChecked(conn, root,
		types.MaxRDPScreenWidth, types.MaxRDPScreenHeight, maxMm, maxMm).Check(); err != nil {
		return trace.Wrap(err)
	}

	x.config.Logger.Log(x.ctx, logutils.TraceLevel, "setting crtc config",
		"crtc", crtc,
		"configTimestamp", resources.ConfigTimestamp,
		"mode", modeID,
		"output", output,
		"width", width,
		"height", height,
	)

	reply, err := randr.SetCrtcConfig(conn, crtc,
		resources.ConfigTimestamp, resources.ConfigTimestamp,
		0, 0, randr.Mode(modeID), randr.RotationRotate0,
		[]randr.Output{output}).Reply()
	if err != nil {
		return trace.Wrap(err)
	}

	widthMm := pixelsToMm(width)
	heightMm := pixelsToMm(height)
	if err := randr.SetScreenSizeChecked(conn, root, width, height, widthMm, heightMm).Check(); err != nil {
		return trace.Wrap(err)
	}

	x.width = width
	x.height = height
	x.resizeTimestamp = reply.Timestamp

	return nil
}

// SetClipboardData stores clipboard data and claims clipboard ownership.
func (x *Backend) SetClipboardData(data []byte) error {
	x.mu.Lock()
	x.clipboardData = data
	x.mu.Unlock()
	err := xproto.SetSelectionOwnerChecked(x.conn, x.clipboardWindow, x.clipboardAtom, 0).Check()
	return trace.Wrap(err)
}

const MagicCookieString = "MIT-MAGIC-COOKIE-1"

func generateXauthorityEntry(display string, cookie []byte) ([]byte, error) {
	host, err := os.Hostname()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	data := make([]byte, 0, 2+2+len(host)+2+len(display)+2+len(MagicCookieString)+2+16)
	data = binary.BigEndian.AppendUint16(data, 256)                            // address family: local
	data = binary.BigEndian.AppendUint16(data, uint16(len(host)))              // host name length
	data = append(data, host...)                                               // host name
	data = binary.BigEndian.AppendUint16(data, uint16(len(display[1:])))       // display length
	data = append(data, display[1:]...)                                        //display
	data = binary.BigEndian.AppendUint16(data, uint16(len(MagicCookieString))) // magic cookie string length
	data = append(data, MagicCookieString...)
	data = binary.BigEndian.AppendUint16(data, 16)
	data = append(data, cookie...) // random secret data
	return data, nil
}
