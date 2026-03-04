package x11

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jezek/xgb"
	"github.com/jezek/xgb/damage"
	"github.com/jezek/xgb/randr"
	"github.com/jezek/xgb/xfixes"
	"github.com/jezek/xgb/xproto"
	"github.com/jezek/xgb/xtest"
	"go.uber.org/atomic"
)

var asciiRE = regexp.MustCompile("[[:^ascii:]]")

var modeCount atomic.Uint64

func init() {
	xgb.Logger = log.New(io.Discard, "", 0)
}

type Config struct {
	ClipboardDataReceiver func([]byte)
}

type Xvfb struct {
	cmd     *exec.Cmd
	conn    *xgb.Conn
	setup   *xproto.SetupInfo
	cancel  context.CancelFunc
	Display string
	damage  damage.Damage

	clipboardAtom   xproto.Atom
	clipboardWindow xproto.Window
	clipboardData   atomic.Pointer[[]byte]
}

// IsXvfbPresent reports whether the Xvfb binary is available in PATH.
func IsXvfbPresent() bool {
	_, err := exec.LookPath("Xvfb")
	return err == nil
}

// NewXvfb starts a Xvfb server and returns a connected client wrapper for interacting with the display.
func NewXvfb(ctx context.Context, config Config) (*Xvfb, error) {
	if !IsXvfbPresent() {
		return nil, trace.NotFound("Xvfb is not installed")
	}

	ctx, cancel := context.WithCancel(ctx)

	cmd := exec.CommandContext(ctx, "Xvfb",
		"-displayfd", "1",
		"-screen", "0", "8192x8192x24",
		"-dpi", "50",
		"-nolock",
		"-nolisten", "tcp",
		"-iglx")

	if os.Getenv("TELEPORT_UNSTABLE_USE_XEPHYR") != "" {
		cmd = exec.CommandContext(ctx, "Xephyr",
			"-displayfd", "1",
			"-screen", "8192x8192x24",
			"-dpi", "50",
			"-nolock",
			"-nolisten", "tcp",
			"-iglx")
	}

	cmd.WaitDelay = 5 * time.Second

	var success bool
	defer func() {
		if !success {
			cancel()
		}
	}()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	if err := cmd.Start(); err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	reader := bufio.NewReader(stdout)
	display, err := reader.ReadString('\n')
	if err != nil {
		return nil, trace.Wrap(err, "reading Xvfb display")
	}
	display = ":" + strings.TrimSuffix(strings.TrimPrefix(display, ":"), "\n")

	conn, setup, err := connectToDisplay(display)
	if err != nil {
		return nil, trace.Wrap(err, "connecting to display")
	}

	id, err := conn.NewId()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	damageID := damage.Damage(id)

	err = damage.CreateChecked(conn, damageID, xproto.Drawable(setup.Roots[0].Root), damage.ReportLevelNonEmpty).Check()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	atomReply, err := xproto.InternAtom(conn, false, uint16(len("CLIPBOARD")), "CLIPBOARD").Reply()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clipboard := atomReply.Atom

	window := setup.Roots[0].Root
	if err := xfixes.SelectSelectionInputChecked(conn, window, clipboard, xfixes.SelectionEventMaskSetSelectionOwner).Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	clipWindow, err := xproto.NewWindowId(conn)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := xproto.CreateWindowChecked(conn, 0, clipWindow, window, -10, -10, 1, 1, 0, xproto.WindowClassInputOnly, xproto.WindowClassCopyFromParent, 0, nil).Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	property, err := xproto.InternAtom(conn, false, uint16(len("TELEPORT_SELECTION")), "TELEPORT_SELECTION").Reply()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	targets, err := xproto.InternAtom(conn, false, uint16(len("TARGETS")), "TARGETS").Reply()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	utf8, err := xproto.InternAtom(conn, false, uint16(len("UTF8_STRING")), "UTF8_STRING").Reply()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	x := &Xvfb{
		cmd:             cmd,
		conn:            conn,
		setup:           setup,
		cancel:          cancel,
		Display:         display,
		damage:          damageID,
		clipboardAtom:   clipboard,
		clipboardWindow: clipWindow,
	}

	go func() {
		for {
			event, err := conn.WaitForEvent()
			if event == nil && err == nil {
				return
			}
			switch event := event.(type) {
			case xproto.SelectionRequestEvent:
				if event.Property == xproto.AtomNone {
					event.Property = property.Atom
				}
				data := *x.clipboardData.Load()
				switch event.Target {
				case utf8.Atom:
					xproto.ChangeProperty(conn, xproto.PropModeReplace, event.Requestor, event.Property, xproto.AtomString, 8, uint32(len(data)), data)
				case xproto.AtomString:
					data := asciiRE.ReplaceAllLiteralString(string(data), "")
					xproto.ChangeProperty(conn, xproto.PropModeReplace, event.Requestor, event.Property, xproto.AtomString, 8, uint32(len(data)), []byte(data))
				case targets.Atom:
					atoms := make([]byte, 8)
					binary.LittleEndian.PutUint32(atoms, uint32(utf8.Atom))
					binary.LittleEndian.PutUint32(atoms[4:], xproto.AtomString)
					xproto.ChangeProperty(conn, xproto.PropModeReplace, event.Requestor, event.Property, xproto.AtomString, 8, uint32(len(atoms)), atoms)
				default:
					event.Property = xproto.AtomNone
				}
				xproto.SendEvent(conn, true, event.Requestor, 0, string(xproto.SelectionNotifyEvent{
					Time:      event.Time,
					Requestor: event.Requestor,
					Selection: event.Selection,
					Target:    event.Target,
					Property:  event.Property,
				}.Bytes()))
			case xproto.SelectionNotifyEvent:
				if config.ClipboardDataReceiver == nil {
					continue
				}
				prop, err := xproto.GetProperty(conn, true, clipWindow, event.Property, event.Target, 0, 1024).Reply()
				if err != nil {
					continue
				}
				config.ClipboardDataReceiver(prop.Value)
			case xfixes.SelectionNotifyEvent:
				xproto.ConvertSelection(conn, clipWindow, clipboard, utf8.Atom, property.Atom, 0)
			}
		}
	}()

	success = true
	return x, nil
}

func (x *Xvfb) root() xproto.Window {
	return x.setup.Roots[0].Root
}

// Close stops the Xvfb process and waits for it to exit.
func (x *Xvfb) Close() error {
	x.cancel()
	var e *exec.ExitError

	err := x.cmd.Wait()
	if errors.As(err, &e) {
		return nil
	}
	return trace.Wrap(err)
}

func connectToDisplay(display string) (*xgb.Conn, *xproto.SetupInfo, error) {
	conn, err := xgb.NewConnDisplay(display)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	setup := xproto.Setup(conn)
	if err := randr.Init(conn); err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if err := xtest.Init(conn); err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if err := damage.Init(conn); err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if err := xfixes.Init(conn); err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if err := randr.Init(conn); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	_, err = xfixes.QueryVersion(conn, 5, 0).Reply()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	_, err = damage.QueryVersion(conn, 1, 1).Reply()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	_, err = xfixes.QueryVersion(conn, 5, 0).Reply()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	_, err = randr.QueryVersion(conn, 5, 0).Reply()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return conn, setup, nil
}

// GetChanges returns regions changed since the last call of this method
func (x *Xvfb) GetChanges() ([]xproto.Rectangle, error) {
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
func (x *Xvfb) GetImage(rect xproto.Rectangle) ([]byte, error) {
	root := xproto.Drawable(x.root())
	reply, err := xproto.GetImage(x.conn, xproto.ImageFormatZPixmap, root, rect.X, rect.Y, rect.Width, rect.Height, math.MaxUint32).Reply()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	data := reply.Data
	// Data returned from xproto.GetImage is BGRA and alpha is always 0, we change it to RGBA and alpha set to 0xFF
	for i := 0; i < len(data); i += 4 {
		data[i+0], data[i+2] = data[i+2], data[i+0]
		data[i+3] = 0xff
	}
	return data, nil
}

// SendKeyboardButton sends a key press or release event.
func (x *Xvfb) SendKeyboardButton(keycode byte, pressed bool) error {
	eventType := xproto.KeyRelease
	if pressed {
		eventType = xproto.KeyPress
	}
	err := xtest.FakeInputChecked(x.conn, byte(eventType), keycode+8, xproto.TimeCurrentTime, x.root(), 0, 0, 0).Check()
	return trace.Wrap(err)
}

// SendMouseButton sends a mouse button press or release event.
func (x *Xvfb) SendMouseButton(button byte, pressed bool) error {
	eventType := xproto.ButtonRelease
	if pressed {
		eventType = xproto.ButtonPress
	}
	err := xtest.FakeInputChecked(x.conn, byte(eventType), button+1, xproto.TimeCurrentTime, x.root(), 0, 0, 0).Check()
	return trace.Wrap(err)
}

// SendMouseWheel sends one wheel step event in the requested direction.
func (x *Xvfb) SendMouseWheel(delta int) error {
	detail := byte(5)
	if delta > 0 {
		detail = 4
	}
	if err := xtest.FakeInputChecked(x.conn, 4, detail, xproto.TimeCurrentTime, x.root(), 0, 0, 0).Check(); err != nil {
		return trace.Wrap(err)
	}
	err := xtest.FakeInputChecked(x.conn, 5, detail, xproto.TimeCurrentTime, x.root(), 0, 0, 0).Check()
	return trace.Wrap(err)
}

// Resize changes the virtual screen size.
func (x *Xvfb) Resize(width, height uint16) error {
	if width > 8192 || height > 8192 {
		return trace.BadParameter("maximum size is 8192x8192")
	}
	conn := x.conn
	root := x.root()

	modeName := fmt.Sprintf("mode%d", modeCount.Inc())

	id, err := conn.NewId()
	if err != nil {
		return trace.Wrap(err)
	}
	mode, err := randr.CreateMode(conn, root, randr.ModeInfo{
		Id:      id,
		Width:   width,
		Height:  height,
		NameLen: uint16(len(modeName)),
	}, modeName).Reply()
	if err != nil {
		return trace.Wrap(err)
	}
	screenResources, err := randr.GetScreenResources(conn, root).Reply()
	if err != nil {
		return trace.Wrap(err)
	}
	if err := randr.AddOutputModeChecked(conn, screenResources.Outputs[0], mode.Mode).Check(); err != nil {
		return trace.Wrap(err)
	}
	screen, err := randr.GetScreenInfo(conn, root).Reply()
	if err != nil {
		return trace.Wrap(err)
	}
	for i := 0; i < len(screen.Sizes); i++ {
		if screen.Sizes[i].Width == width && screen.Sizes[i].Height == height {
			_, err := randr.SetScreenConfig(conn, root, 0, screen.ConfigTimestamp, uint16(i), screen.Rotation, screen.Rate).Reply()
			if err != nil {
				return trace.Wrap(err)
			}
			return nil
		}
	}
	return trace.Errorf("could not find a screen with width %d and height %d", width, height)
}

// SetClipboardData stores clipboard data and claims clipboard ownership.
func (x *Xvfb) SetClipboardData(data []byte) error {
	x.clipboardData.Store(&data)
	err := xproto.SetSelectionOwnerChecked(x.conn, x.clipboardWindow, x.clipboardAtom, 0).Check()
	return trace.Wrap(err)
}
