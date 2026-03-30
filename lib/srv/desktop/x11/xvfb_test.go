package x11

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/trace"
	"github.com/jezek/xgb"
	"github.com/jezek/xgb/randr"
	"github.com/jezek/xgb/xproto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testConfig() Config {
	slog.SetLogLoggerLevel(logutils.TraceLevel)
	return Config{
		Logger: slog.Default(),
	}
}

func TestNewBackend(t *testing.T) {
	if !IsBackendPresent() {
		t.Skip("this test requires Backend to be installed")
	}
	xvfb, err := NewBackend(t.Context(), testConfig())
	require.NoError(t, err)
	require.NotNil(t, xvfb)

	matched, err := regexp.MatchString(":[0-9]+", xvfb.Display)
	assert.NoError(t, err)
	assert.True(t, matched)

	err = xvfb.Close()
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		err = xvfb.cmd.Process.Signal(syscall.Signal(0))
		return err != nil
	}, 5*time.Second, 100*time.Millisecond, "waiting for process to exit")
}

func TestResize(t *testing.T) {
	if !IsBackendPresent() {
		t.Skip("this test requires Backend to be installed")
	}

	xvfb, err := NewBackend(t.Context(), testConfig())
	require.NoError(t, err)
	defer xvfb.Close()

	conn, _, err := connectToDisplay(xvfb.Display, xvfb.authorityCookie)
	require.NoError(t, err)
	defer conn.Close()

	screenInfo, err := randr.GetScreenInfo(conn, xvfb.root()).Reply()
	require.NoError(t, err)
	require.Equal(t, uint16(0), screenInfo.SizeID)
	require.Equal(t, uint16(8192), screenInfo.Sizes[0].Width)
	require.Equal(t, uint16(8192), screenInfo.Sizes[0].Height)

	err = xvfb.Resize(9000, 200)
	require.True(t, trace.IsBadParameter(err))
	err = xvfb.Resize(2000, 9200)
	require.True(t, trace.IsBadParameter(err))
	err = xvfb.Resize(2000, 1000)
	require.NoError(t, err)

	screenInfo, err = randr.GetScreenInfo(conn, xvfb.root()).Reply()
	require.NoError(t, err)
	require.Equal(t, uint16(1), screenInfo.SizeID)
	require.Equal(t, uint16(2000), screenInfo.Sizes[1].Width)
	require.Equal(t, uint16(1000), screenInfo.Sizes[1].Height)
}

func TestGetChanges(t *testing.T) {
	if !IsBackendPresent() {
		t.Skip("this test requires Backend to be installed")
	}

	xvfb, err := NewBackend(t.Context(), testConfig())
	require.NoError(t, err)
	defer xvfb.Close()

	changes, err := xvfb.GetChanges()
	require.NoError(t, err)
	require.Len(t, changes, 1)
	require.Equal(t, uint16(8192), changes[0].Width)
	require.Equal(t, uint16(8192), changes[0].Height)

	changes, err = xvfb.GetChanges()
	require.NoError(t, err)
	require.Len(t, changes, 0)

	err = xvfb.Resize(1000, 1000)
	require.NoError(t, err)

	changes, err = xvfb.GetChanges()
	require.NoError(t, err)
	require.Len(t, changes, 1)
	require.Equal(t, uint16(1000), changes[0].Width)
	require.Equal(t, uint16(1000), changes[0].Height)

	conn, setup, err := connectToDisplay(xvfb.Display, xvfb.authorityCookie)
	require.NoError(t, err)
	defer conn.Close()

	drawRectangle(t, conn, setup, xproto.Rectangle{
		X:      5,
		Y:      10,
		Width:  10,
		Height: 20,
	})

	changes, err = xvfb.GetChanges()
	require.NoError(t, err)
	require.Len(t, changes, 1)
	require.Equal(t, int16(5), changes[0].X)
	require.Equal(t, int16(10), changes[0].Y)
	require.Equal(t, uint16(10), changes[0].Width)
	require.Equal(t, uint16(20), changes[0].Height)
}

func TestGetImage(t *testing.T) {
	if !IsBackendPresent() {
		t.Skip("this test requires Backend to be installed")
	}

	xvfb, err := NewBackend(t.Context(), testConfig())
	require.NoError(t, err)
	defer xvfb.Close()

	conn, setup, err := connectToDisplay(xvfb.Display, xvfb.authorityCookie)
	require.NoError(t, err)
	defer conn.Close()

	drawRectangle(t, conn, setup, xproto.Rectangle{
		X:      1,
		Y:      1,
		Width:  1,
		Height: 1,
	})

	image, err := xvfb.GetImage(xproto.Rectangle{
		Width:  2,
		Height: 2,
	})
	require.NoError(t, err)
	require.Equal(t, []byte{0, 0, 0, 0xFF, 0, 0, 0, 0xFF, 0, 0, 0, 0xFF, 0x12, 0x34, 0x56, 0xFF}, image)
}

func TestInputs(t *testing.T) {
	if !IsBackendPresent() {
		t.Skip("this test requires Backend to be installed")
	}

	xvfb, err := NewBackend(t.Context(), testConfig())
	require.NoError(t, err)
	defer xvfb.Close()

	conn, _, err := connectToDisplay(xvfb.Display, xvfb.authorityCookie)
	require.NoError(t, err)
	defer conn.Close()

	eventMask := uint16(xproto.EventMaskPointerMotion | xproto.EventMaskButtonPress | xproto.EventMaskButtonRelease)
	_, err = xproto.GrabPointer(conn, true, xvfb.root(), eventMask, xproto.GrabModeAsync, xproto.GrabModeAsync, xproto.AtomNone, xproto.AtomNone, xproto.TimeCurrentTime).Reply()
	require.NoError(t, err)
	_, err = xproto.GrabKeyboard(conn, true, xvfb.root(), xproto.TimeCurrentTime, xproto.GrabModeAsync, xproto.GrabModeAsync).Reply()
	require.NoError(t, err)

	err = xvfb.SendMouseButton(0, true)
	require.NoError(t, err)

	bp := pollEvent[xproto.ButtonPressEvent](t, conn)
	require.Equal(t, xproto.Button(1), bp.Detail)

	err = xvfb.SendMouseButton(0, false)
	require.NoError(t, err)

	br := pollEvent[xproto.ButtonReleaseEvent](t, conn)
	require.Equal(t, xproto.Button(1), br.Detail)

	err = xvfb.SendMouseWheel(1)
	require.NoError(t, err)

	bp = pollEvent[xproto.ButtonPressEvent](t, conn)
	require.Equal(t, xproto.Button(4), bp.Detail)

	br = pollEvent[xproto.ButtonReleaseEvent](t, conn)
	require.Equal(t, xproto.Button(4), br.Detail)

	err = xvfb.SendMouseWheel(-1)
	require.NoError(t, err)

	bp = pollEvent[xproto.ButtonPressEvent](t, conn)
	require.Equal(t, xproto.Button(5), bp.Detail)

	br = pollEvent[xproto.ButtonReleaseEvent](t, conn)
	require.Equal(t, xproto.Button(5), br.Detail)

	err = xvfb.SendKeyboardButton(13, true)
	require.NoError(t, err)

	pollEvent[xproto.MappingNotifyEvent](t, conn)
	pollEvent[xproto.MappingNotifyEvent](t, conn)
	kp := pollEvent[xproto.KeyPressEvent](t, conn)
	require.Equal(t, xproto.Keycode(21), kp.Detail)

	err = xvfb.SendKeyboardButton(13, false)
	require.NoError(t, err)
	kr := pollEvent[xproto.KeyReleaseEvent](t, conn)
	require.Equal(t, xproto.Keycode(21), kr.Detail)
}

func TestClipboard(t *testing.T) {
	if !IsBackendPresent() {
		t.Skip("this test requires Backend to be installed")
	}

	// clipboard interactions are really complex in X11
	// to verify it fully we have use external utility
	_, err := exec.LookPath("xclip")
	if err != nil {
		t.Skip("this test requires xclip to be installed")
	}

	var data atomic.Pointer[[]byte]

	config := testConfig()
	config.ClipboardDataReceiver = func(bytes []byte) {
		data.Store(&bytes)
	}
	xvfb, err := NewBackend(t.Context(), config)
	require.NoError(t, err)
	defer xvfb.Close()

	err = xvfb.SetClipboardData([]byte("sent_to_clipboard"))
	require.NoError(t, err)

	env := []string{
		fmt.Sprintf("DISPLAY=%s", xvfb.Display),
		fmt.Sprintf("XAUTHORITY=%s", xvfb.AuthorityFile.Name()),
	}

	cmd := exec.CommandContext(t.Context(), "xclip", "-selection", "clipboard", "-o")
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	require.Equal(t, "sent_to_clipboard", string(out))

	cmd = exec.CommandContext(t.Context(), "xclip", "-selection", "clipboard")
	cmd.Env = env
	w, err := cmd.StdinPipe()
	require.NoError(t, err)

	n, err := io.WriteString(w, "abcd")
	require.NoError(t, err)
	require.Equal(t, len("abcd"), n)
	require.NoError(t, w.Close())

	err = cmd.Run()
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return data.Load() != nil && len(*data.Load()) > 0
	}, 1*time.Second, 50*time.Millisecond, "waiting on clipboard data")

	require.Equal(t, "abcd", string(*data.Load()))
}

func pollEvent[T xgb.Event](t *testing.T, conn *xgb.Conn) T {
	var event xgb.Event
	var err error
	assert.Eventually(t, func() bool {
		event, err = conn.PollForEvent()
		return event != nil || err != nil
	}, 1*time.Second, 10*time.Millisecond, "polling for event")
	require.NoError(t, err)
	require.NotNil(t, event)
	tt, ok := event.(T)
	require.True(t, ok)
	return tt
}

func drawRectangle(t *testing.T, conn *xgb.Conn, setup *xproto.SetupInfo, rect xproto.Rectangle) {
	screen := setup.Roots[0]
	root := screen.Root
	// Create a GC (Graphics Context) on the root window
	gc, err := xproto.NewGcontextId(conn)
	require.NoError(t, err)

	// GC flags and values: set foreground color
	mask := uint32(xproto.GcForeground)
	values := []uint32{0x123456}

	err = xproto.CreateGCChecked(conn, gc, xproto.Drawable(root), mask, values).Check()
	require.NoError(t, err)

	// Draw a filled rectangle on the root window
	err = xproto.PolyFillRectangleChecked(conn, xproto.Drawable(root), gc, []xproto.Rectangle{rect}).Check()
	require.NoError(t, err)
}

func TestGenerateMagicCookie(t *testing.T) {
	host, err := os.Hostname()
	require.NoError(t, err)
	data := make([]byte, 16)
	cookie, err := generateXauthorityEntry(":0", data)
	require.NoError(t, err)
	require.Len(t, cookie, 45+len(host))
	require.Equal(t, []byte{1, 0}, cookie[0:2])
	clen := len(cookie)
	require.Equal(t, []byte(MagicCookieString), cookie[clen-18-18:clen-18])
	require.Equal(t, []byte{0, 18}, cookie[clen-18-18-2:clen-18-18])
	require.Equal(t, []byte{0, 16}, cookie[clen-16-2:clen-16])
}
