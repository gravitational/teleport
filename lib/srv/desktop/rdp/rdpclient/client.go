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
//  Send(MouseMove) ------> write_rdp_pointer
//  Send(MouseButton) ----> write_rdp_pointer
//  Send(KeyboardButton) -> write_rdp_keyboard
//            *user input continues...*
//
//        *connection closed (client or server side)*
//    Wait       -----------------> close_rdp
//

/*
// Flags to include the static Rust library.
#cgo linux,386 LDFLAGS: -L${SRCDIR}/target/i686-unknown-linux-gnu/release
#cgo linux,amd64 LDFLAGS: -L${SRCDIR}/target/x86_64-unknown-linux-gnu/release
#cgo linux,arm LDFLAGS: -L${SRCDIR}/target/arm-unknown-linux-gnueabihf/release
#cgo linux,arm64 LDFLAGS: -L${SRCDIR}/target/aarch64-unknown-linux-gnu/release
#cgo linux LDFLAGS: -l:librdp_client.a -lpthread -ldl -lm -latomic
#cgo darwin,amd64 LDFLAGS: -L${SRCDIR}/target/x86_64-apple-darwin/release
#cgo darwin,arm64 LDFLAGS: -L${SRCDIR}/target/aarch64-apple-darwin/release
#cgo darwin LDFLAGS: -framework CoreFoundation -framework Security -lrdp_client -lpthread -ldl -lm
#include <librdprs.h>
*/
import "C"
import (
	"context"
	"errors"
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
	if rl := os.Getenv("RUST_LOG"); rl != "" {
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
// It's caller is responsible for calling Cleanup() when the Client is no longer needed.
type Client struct {
	Cfg Config

	// RDP client on the Rust side.
	rustClient *C.Client

	// Synchronization point to prevent input messages from being forwarded
	// until the connection is established.
	// Used with sync/atomic, 0 means false, 1 means true.
	readyForInput uint32

	closeOnce sync.Once

	// Fields to remember mouse coordinates
	// to send them with all CGOPointer events
	// in ForwardInput.
	mouseX, mouseY uint32

	// outputCh is a channel that gets set to the channel passed into
	// passed into StartReceiving. RDP that are received should be translated
	// into TDP and then sent to this channel.
	sendTo chan<- tdp.Message

	clientActivityMu sync.RWMutex
	clientLastActive time.Time
}

// New creates and connects a new Client based on Cfg.
func New(ctx context.Context, Cfg Config) (*Client, error) {
	if err := Cfg.checkAndSetDefaults(); err != nil {
		return nil, err
	}
	c := &Client{
		Cfg:           Cfg,
		readyForInput: 0,
	}
	return c, nil
}

func (c *Client) Connect(ctx context.Context, u *tdp.ClientUsername, sc *tdp.ClientScreenSpec) error {
	userCertDER, userKeyDER, err := c.Cfg.GenerateUserCert(ctx, u.Username)
	if err != nil {
		return trace.Wrap(err)
	}

	// Addr and username strings only need to be valid for the duration of
	// C.connect_rdp. They are copied on the Rust side and can be freed here.
	addr := C.CString(c.Cfg.Addr)
	defer C.free(unsafe.Pointer(addr))
	username := C.CString(u.Username)
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
		C.uint16_t(uint16(sc.Width)),
		C.uint16_t(uint16(sc.Height)),
	)
	if err := cgoError(res.err); err != nil {
		return trace.Wrap(err)
	}
	c.rustClient = res.client
	return nil
}

// StartStreamingRDPtoTDP tells the RDP client to start receiving RDP messages,
// translating them to TDP messages, and sending those TDP messages to
// the passed channel. It hangs for the duration of the RDP connection,
// either until an RDP read returns an error for an unknown reason or
// because c.Close() was called. It closes the passed channel before returning.
// TODO: change this comment once the TODO in read_rdp_output_inner is addressed.
func (c *Client) StartStreamingRDPtoTDP(sendTo chan<- tdp.Message) error {
	c.sendTo = sendTo

	h := cgo.NewHandle(c)
	defer h.Delete()

	// C.read_rdp_output blocks for the duration of the RDP connection and
	// calls handle_bitmap repeatedly with the incoming bitmaps.
	err := cgoError(C.read_rdp_output(c.rustClient, C.uintptr_t(h)))
	close(c.sendTo)
	return err
}

// StartStreamingTDPtoRDP tells the RDP client to start receiving TDP messages
// on the passed channel, translating them to RDP messages, and sending those
// over the RDP connection. This function will hang until it's caller is closes
// the passed channel, or if an attempt at sending an RDP message returns an error.
func (c *Client) StartStreamingTDPtoRDP(recvFrom <-chan tdp.Message) error {
	for msg := range recvFrom {
		if err := c.sendRDP(msg); err != nil {
			c.Cfg.Log.Warning(err)
			return err
		}
	}

	return nil
}

// sendRDP translates a TDP message to RDP and writes it via the C client.
func (c *Client) sendRDP(msg tdp.Message) error {
	if atomic.LoadUint32(&c.readyForInput) == 0 {
		// Input not allowed yet, drop the message.
		return nil
	}

	c.UpdateClientActivity()

	switch m := msg.(type) {
	case tdp.MouseMove:
		c.mouseX, c.mouseY = m.X, m.Y
		if err := cgoError(C.write_rdp_pointer(
			c.rustClient,
			C.CGOMousePointerEvent{
				x:      C.uint16_t(m.X),
				y:      C.uint16_t(m.Y),
				button: C.PointerButtonNone,
				wheel:  C.PointerWheelNone,
			},
		)); err != nil {
			return trace.Errorf("Failed forwarding RDP input message: %v", err)
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
				x:      C.uint16_t(c.mouseX),
				y:      C.uint16_t(c.mouseY),
				button: uint32(button),
				down:   m.State == tdp.ButtonPressed,
				wheel:  C.PointerWheelNone,
			},
		)); err != nil {
			return trace.Errorf("Failed forwarding RDP input message: %v", err)
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
				x:           C.uint16_t(c.mouseX),
				y:           C.uint16_t(c.mouseY),
				button:      C.PointerButtonNone,
				wheel:       uint32(wheel),
				wheel_delta: C.int16_t(m.Delta),
			},
		)); err != nil {
			return trace.Errorf("Failed forwarding RDP input message: %v", err)
		}
	case tdp.KeyboardButton:
		if err := cgoError(C.write_rdp_keyboard(
			c.rustClient,
			C.CGOKeyboardEvent{
				code: C.uint16_t(m.KeyCode),
				down: m.State == tdp.ButtonPressed,
			},
		)); err != nil {
			return trace.Errorf("Failed forwarding RDP input message: %v", err)
		}
	default:
		c.Cfg.Log.Warningf("Skipping unimplemented desktop protocol message type %T", msg)
	}
	return nil
}

//export handle_bitmap
func handle_bitmap(handle C.uintptr_t, cb C.CGOBitmap) C.CGOError {
	return cgo.Handle(handle).Value().(*Client).handleBitmap(cb)
}

// TODO: this never returns a non-nil error, the exported handle_bitmap and it's
// caller in the rust library can be updated to account for this.
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

	c.sendTo <- tdp.PNGFrame{Img: img}

	return nil
}

// Cleanup cleans up C memory.
func (c *Client) Cleanup() {
	C.free_rdp(c.rustClient)
}

// Close shuts down the client and closes any existing connections.
// It is safe to call multiple times, from multiple goroutines.
// Calls other than the first one are no-ops.
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		if err := cgoError(C.close_rdp(c.rustClient)); err != nil {
			c.Cfg.Log.Warningf("Error closing RDP connection: %v", err)
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
