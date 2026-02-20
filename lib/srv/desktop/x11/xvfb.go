package x11

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
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

var modeCount atomic.Uint64

func init() {
	xgb.Logger = log.New(io.Discard, "", 0)
}

type Xvfb struct {
	cmd     *exec.Cmd
	dial    *xgb.Conn
	setup   *xproto.SetupInfo
	cancel  context.CancelFunc
	Display string
}

func IsXvfbPresent() bool {
	_, err := exec.LookPath("Xvfb")
	return err == nil
}

func NewXvfb(ctx context.Context) (*Xvfb, error) {
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

	dial, setup, err := connectToDisplay(display)
	if err != nil {
		return nil, trace.Wrap(err, "connecting to display")
	}

	success = true
	return &Xvfb{
		cmd:     cmd,
		dial:    dial,
		setup:   setup,
		cancel:  cancel,
		Display: display,
	}, nil
}

func (x *Xvfb) root() xproto.Window {
	return x.setup.Roots[0].Root
}

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
	dial, err := xgb.NewConnDisplay(display)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	setup := xproto.Setup(dial)
	if err := randr.Init(dial); err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if err := xtest.Init(dial); err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if err := damage.Init(dial); err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if err := xfixes.Init(dial); err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if err := randr.Init(dial); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	_, err = xfixes.QueryVersion(dial, 5, 0).Reply()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	_, err = damage.QueryVersion(dial, 1, 1).Reply()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	_, err = xfixes.QueryVersion(dial, 5, 0).Reply()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	_, err = randr.QueryVersion(dial, 5, 0).Reply()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return dial, setup, nil
}

func (x *Xvfb) SendKeyboardButton(keycode byte, pressed bool) error {
	eventType := byte(3)
	if pressed {
		eventType = 2
	}
	err := xtest.FakeInputChecked(x.dial, eventType, keycode+8, xproto.TimeCurrentTime, x.root(), 0, 0, 0).Check()
	return trace.Wrap(err)
}

func (x *Xvfb) SendMouseButton(button byte, pressed bool) error {
	eventType := byte(5)
	if pressed {
		eventType = 4
	}
	err := xtest.FakeInputChecked(x.dial, eventType, button+1, xproto.TimeCurrentTime, x.root(), 0, 0, 0).Check()
	return trace.Wrap(err)
}

func (x *Xvfb) SendMouseWheel(delta int) error {
	detail := byte(5)
	if delta > 0 {
		detail = 4
	}
	if err := xtest.FakeInputChecked(x.dial, 4, detail, xproto.TimeCurrentTime, x.root(), 0, 0, 0).Check(); err != nil {
		return trace.Wrap(err)
	}
	err := xtest.FakeInputChecked(x.dial, 5, detail, xproto.TimeCurrentTime, x.root(), 0, 0, 0).Check()
	return trace.Wrap(err)
}

func (x *Xvfb) Resize(width, height uint16) error {
	if width > 8192 || height > 8192 {
		return trace.BadParameter("maximum size is 8192x8192")
	}
	dial := x.dial
	root := x.root()

	modeName := fmt.Sprintf("mode%d", modeCount.Inc())

	id, err := dial.NewId()
	if err != nil {
		return trace.Wrap(err)
	}
	mode, err := randr.CreateMode(dial, root, randr.ModeInfo{
		Id:      id,
		Width:   width,
		Height:  height,
		NameLen: uint16(len(modeName)),
	}, modeName).Reply()
	if err != nil {
		return trace.Wrap(err)
	}
	screenResources, err := randr.GetScreenResources(dial, root).Reply()
	if err != nil {
		return trace.Wrap(err)
	}
	if err := randr.AddOutputModeChecked(dial, screenResources.Outputs[0], mode.Mode).Check(); err != nil {
		return trace.Wrap(err)
	}
	screen, err := randr.GetScreenInfo(dial, root).Reply()
	if err != nil {
		return trace.Wrap(err)
	}
	for i := 0; i < len(screen.Sizes); i++ {
		if screen.Sizes[i].Width == width && screen.Sizes[i].Height == height {
			_, err := randr.SetScreenConfig(dial, root, 0, screen.ConfigTimestamp, uint16(i), screen.Rotation, screen.Rate).Reply()
			if err != nil {
				return trace.Wrap(err)
			}
			return nil
		}
	}
	return trace.Errorf("could not find a screen with width %d and height %d", width, height)
}
