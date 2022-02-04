//go:build desktop_access_rdp
// +build desktop_access_rdp

/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package rdpclient

// Some implementation details that don't belong in the public godoc:
// This package wraps a Rust library based on https://crates.io/crates/rdp-rs.
//
// The Rust library is statically-compiled and called via CGO.
// The Go code sends and receives the CGO versions of Rust RDP events
// https://docs.rs/rdp-rs/0.1.0/rdp/core/event/index.html and translates them
// to the desktop protocol versions.
//
// The flow is roughly this:
//    Go                                Rust
// ==============================================
//  rdpclient.New -----------------> connect_rdp
//                   *connected*
//
//            *register output callback*
//                -----------------> read_rdp_output
//  handleBitmap  <----------------
//  handleBitmap  <----------------
//  handleBitmap  <----------------
//           *output streaming continues...*
//
//              *user input messages*
//  InputMessage(MouseMove) ------> write_rdp_pointer
//  InputMessage(MouseButton) ----> write_rdp_pointer
//  InputMessage(KeyboardButton) -> write_rdp_keyboard
//            *user input continues...*
//
//        *connection closed (client or server side)*
//    Wait       -----------------> close_rdp
//

/*
// Flags to include the static Rust library.
#cgo linux,386 LDFLAGS: -L${SRCDIR}/../../../../../target/i686-unknown-linux-gnu/release
#cgo linux,amd64 LDFLAGS: -L${SRCDIR}/../../../../../target/x86_64-unknown-linux-gnu/release
#cgo linux,arm LDFLAGS: -L${SRCDIR}/../../../../../target/arm-unknown-linux-gnueabihf/release
#cgo linux,arm64 LDFLAGS: -L${SRCDIR}/../../../../../target/aarch64-unknown-linux-gnu/release
#cgo linux LDFLAGS: -l:librdp_client.a -lpthread -ldl -lm
#cgo darwin,amd64 LDFLAGS: -L${SRCDIR}/../../../../../target/x86_64-apple-darwin/release
#cgo darwin,arm64 LDFLAGS: -L${SRCDIR}/../../../../../target/aarch64-apple-darwin/release
#cgo darwin LDFLAGS: -framework CoreFoundation -framework Security -lrdp_client -lpthread -ldl -lm
#include <librdprs.h>
*/
import "C"
import (
	"context"
	"errors"
	"fmt"
	"image"
	"os"
	"runtime/cgo"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/trace"

	"github.com/sirupsen/logrus"
)

func init() {
	// initialize the Rust logger by setting $RUST_LOG based
	// on the logrus log level
	// (unless RUST_LOG is already explicitly set, then we
	// assume the user knows what they want)
	if rl := os.Getenv("RUST_LOG"); rl == "" {
		var rustLogLevel string
		switch l := logrus.GetLevel(); l {
		case logrus.TraceLevel:
			rustLogLevel = "trace"
		case logrus.DebugLevel:
			rustLogLevel = "debug"
		case logrus.InfoLevel:
			rustLogLevel = "info"
		case logrus.WarnLevel:
			rustLogLevel = "warn"
		default:
			rustLogLevel = "error"
		}

		os.Setenv("RUST_LOG", rustLogLevel)
	}

	C.init()
}

// Client is the RDP client.
type Client struct {
	cfg Config

	// Parameters read from the TDP stream.
	clientWidth, clientHeight uint16
	username                  string

	// RDP client on the Rust side.
	rustClient *C.Client

	// Synchronization point to prevent input messages from being forwarded
	// until the connection is established.
	// Used with sync/atomic, 0 means false, 1 means true.
	readyForInput uint32

	// wg is used to wait for the input/output streaming
	// goroutines to complete
	wg        sync.WaitGroup
	closeOnce sync.Once

	clientActivityMu sync.RWMutex
	clientLastActive time.Time
}

// New creates and connects a new Client based on cfg.
func New(ctx context.Context, cfg Config) (*Client, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, err
	}
	c := &Client{
		cfg:           cfg,
		readyForInput: 0,
	}

	if err := c.readClientUsername(); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := cfg.AuthorizeFn(c.username); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := c.readClientSize(); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := c.connect(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	c.start()
	return c, nil
}

func (c *Client) readClientUsername() error {
	for {
		msg, err := c.cfg.Conn.InputMessage()
		if err != nil {
			return trace.Wrap(err)
		}
		u, ok := msg.(tdp.ClientUsername)
		if !ok {
			c.cfg.Log.Debugf("Expected ClientUsername message, got %T", msg)
			continue
		}
		c.cfg.Log.Debugf("Got RDP username %q", u.Username)
		c.username = u.Username
		return nil
	}
}

func (c *Client) readClientSize() error {
	for {
		msg, err := c.cfg.Conn.InputMessage()
		if err != nil {
			return trace.Wrap(err)
		}
		s, ok := msg.(tdp.ClientScreenSpec)
		if !ok {
			c.cfg.Log.Debugf("Expected ClientScreenSpec message, got %T", msg)
			continue
		}
		c.cfg.Log.Debugf("Got RDP screen size %dx%d", s.Width, s.Height)
		c.clientWidth = uint16(s.Width)
		c.clientHeight = uint16(s.Height)
		return nil
	}
}

func (c *Client) connect(ctx context.Context) error {
	userCertDER, userKeyDER, err := c.cfg.GenerateUserCert(ctx, c.username, c.cfg.CertTTL)
	if err != nil {
		return trace.Wrap(err)
	}

	// Addr and username strings only need to be valid for the duration of
	// C.connect_rdp. They are copied on the Rust side and can be freed here.
	addr := C.CString(c.cfg.Addr)
	defer C.free(unsafe.Pointer(addr))
	username := C.CString(c.username)
	defer C.free(unsafe.Pointer(username))

	res := C.connect_rdp(
		addr,
		username,
		// cert length and bytes.
		C.uint32_t(len(userCertDER)),
		(*C.uint8_t)(unsafe.Pointer(&userCertDER[0])),
		// key length and bytes.
		C.uint32_t(len(userKeyDER)),
		(*C.uint8_t)(unsafe.Pointer(&userKeyDER[0])),
		// screen size.
		C.uint16_t(c.clientWidth),
		C.uint16_t(c.clientHeight),
	)
	if err := cgoError(res.err); err != nil {
		return trace.Wrap(err)
	}
	c.rustClient = res.client
	return nil
}

// start kicks off goroutines for input/output streaming and returns right
// away. Use Wait to wait for them to finish.
func (c *Client) start() {
	// Video output streaming worker goroutine.
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		defer c.Close()
		defer c.cfg.Log.Info("RDP output streaming finished")

		h := cgo.NewHandle(c)
		defer h.Delete()

		// C.read_rdp_output blocks for the duration of the RDP connection and
		// calls handle_bitmap repeatedly with the incoming bitmaps.
		if err := cgoError(C.read_rdp_output(c.rustClient, C.uintptr_t(h))); err != nil {
			c.cfg.Log.Warningf("Failed reading RDP output frame: %v", err)
		}
	}()

	// User input streaming worker goroutine.
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		defer c.Close()
		defer c.cfg.Log.Info("RDP input streaming finished")
		// Remember mouse coordinates to send them with all CGOPointer events.
		var mouseX, mouseY uint32
		for {
			msg, err := c.cfg.Conn.InputMessage()
			if err != nil {
				c.cfg.Log.Warningf("Failed reading RDP input message: %v", err)
				return
			}

			if atomic.LoadUint32(&c.readyForInput) == 0 {
				// Input not allowed yet, drop the message.
				continue
			}

			c.UpdateClientActivity()

			switch m := msg.(type) {
			case tdp.MouseMove:
				mouseX, mouseY = m.X, m.Y
				if err := cgoError(C.write_rdp_pointer(
					c.rustClient,
					C.CGOMousePointerEvent{
						x:      C.uint16_t(m.X),
						y:      C.uint16_t(m.Y),
						button: C.PointerButtonNone,
						wheel:  C.PointerWheelNone,
					},
				)); err != nil {
					c.cfg.Log.Warningf("Failed forwarding RDP input message: %v", err)
					return
				}
			case tdp.MouseButton:
				// Map the button to a C enum value.
				var button C.CGOPointerButton
				switch m.Button {
				case tdp.LeftMouseButton:
					button = C.PointerButtonLeft
				case tdp.RightMouseButton:
					button = C.PointerButtonRight
				case tdp.MiddleMouseButton:
					button = C.PointerButtonMiddle
				default:
					button = C.PointerButtonNone
				}
				if err := cgoError(C.write_rdp_pointer(
					c.rustClient,
					C.CGOMousePointerEvent{
						x:      C.uint16_t(mouseX),
						y:      C.uint16_t(mouseY),
						button: uint32(button),
						down:   m.State == tdp.ButtonPressed,
						wheel:  C.PointerWheelNone,
					},
				)); err != nil {
					c.cfg.Log.Warningf("Failed forwarding RDP input message: %v", err)
					return
				}
			case tdp.MouseWheel:
				var wheel C.CGOPointerWheel
				switch m.Axis {
				case tdp.VerticalWheelAxis:
					wheel = C.PointerWheelVertical
				case tdp.HorizontalWheelAxis:
					wheel = C.PointerWheelHorizontal
					// TDP positive scroll deltas move towards top-left.
					// RDP positive scroll deltas move towards top-right.
					//
					// Fix the scroll direction to match TDP, it's inverted for
					// horizontal scroll in RDP.
					m.Delta = -m.Delta
				default:
					wheel = C.PointerWheelNone
				}
				if err := cgoError(C.write_rdp_pointer(
					c.rustClient,
					C.CGOMousePointerEvent{
						x:           C.uint16_t(mouseX),
						y:           C.uint16_t(mouseY),
						button:      C.PointerButtonNone,
						wheel:       uint32(wheel),
						wheel_delta: C.int16_t(m.Delta),
					},
				)); err != nil {
					c.cfg.Log.Warningf("Failed forwarding RDP input message: %v", err)
					return
				}
			case tdp.KeyboardButton:
				if err := cgoError(C.write_rdp_keyboard(
					c.rustClient,
					C.CGOKeyboardEvent{
						code: C.uint16_t(m.KeyCode),
						down: m.State == tdp.ButtonPressed,
					},
				)); err != nil {
					c.cfg.Log.Warningf("Failed forwarding RDP input message: %v", err)
					return
				}
			default:
				c.cfg.Log.Warningf("Skipping unimplemented desktop protocol message type %T", msg)
			}
		}
	}()
}

//export handle_bitmap
func handle_bitmap(handle C.uintptr_t, cb C.CGOBitmap) C.CGOError {
	return cgo.Handle(handle).Value().(*Client).handleBitmap(cb)
}

func (c *Client) handleBitmap(cb C.CGOBitmap) C.CGOError {
	// Notify the input forwarding goroutine that we're ready for input.
	// Input can only be sent after connection was established, which we infer
	// from the fact that a bitmap was sent.
	atomic.StoreUint32(&c.readyForInput, 1)

	data := C.GoBytes(unsafe.Pointer(cb.data_ptr), C.int(cb.data_len))
	// Convert BGRA to RGBA. It's likely due to Windows using uint32 values for
	// pixels (ARGB) and encoding them as big endian. The image.RGBA type uses
	// a byte slice with 4-byte segments representing pixels (RGBA).
	//
	// Also, always force Alpha value to 100% (opaque). On some Windows
	// versions it's sent as 0% after decompression for some reason.
	for i := 0; i < len(data); i += 4 {
		data[i], data[i+2], data[i+3] = data[i+2], data[i], 255
	}
	img := image.NewNRGBA(image.Rectangle{
		Min: image.Pt(int(cb.dest_left), int(cb.dest_top)),
		Max: image.Pt(int(cb.dest_right)+1, int(cb.dest_bottom)+1),
	})
	copy(img.Pix, data)

	if err := c.cfg.Conn.OutputMessage(tdp.NewPNG(img, c.cfg.Encoder)); err != nil {
		return C.CString(fmt.Sprintf("failed to send PNG frame %v: %v", img.Rect, err))
	}
	return nil
}

// Wait blocks until the client disconnects and runs the cleanup.
func (c *Client) Wait() error {
	c.wg.Wait()
	// Let the Rust side free its data.
	C.free_rdp(c.rustClient)
	return nil
}

// Close shuts down the client and closes any existing connections.
// It is safe to call multiple times, from multiple goroutines.
// Calls other than the first one are no-ops.
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		if err := cgoError(C.close_rdp(c.rustClient)); err != nil {
			c.cfg.Log.Warningf("Error closing RDP connection: %v", err)
		}
	})
}

// GetClientLastActive returns the time of the last recorded activity.
// For RDP, "activity" is defined as user-input messages
// (mouse move, button press, etc.)
func (c *Client) GetClientLastActive() time.Time {
	c.clientActivityMu.RLock()
	defer c.clientActivityMu.RUnlock()
	return c.clientLastActive
}

// UpdateClientActivity updates the client activity timestamp.
func (c *Client) UpdateClientActivity() {
	c.clientActivityMu.Lock()
	c.clientLastActive = time.Now().UTC()
	c.clientActivityMu.Unlock()
}

// cgoError converts from a CGO-originated error to a Go error, copying the
// error string and releasing the CGO data.
func cgoError(s C.CGOError) error {
	if s == nil {
		return nil
	}
	gs := C.GoString(s)
	C.free_rust_string(s)
	return errors.New(gs)
}

//export free_go_string
func free_go_string(s *C.char) {
	C.free(unsafe.Pointer(s))
}
