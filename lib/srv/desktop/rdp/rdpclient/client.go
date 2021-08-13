//+build desktop_access_beta

// Package rdpclient implements an RDP client.
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
#cgo linux LDFLAGS: -L${SRCDIR}/target/debug -l:librdp_client.a -lpthread -lcrypto -ldl -lssl -lm
#cgo darwin LDFLAGS: -framework CoreFoundation -framework Security -L${SRCDIR}/target/debug -lrdp_client -lpthread -lcrypto -ldl -lssl -lm
#include <librdprs.h>
*/
import "C"
import (
	"errors"
	"fmt"
	"image"
	"sync"
	"unsafe"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/lib/srv/desktop/deskproto"
)

func init() {
	C.init()
}

// Options for creating a new Client.
type Options struct {
	// Addr is the network address of the RDP server, in the form host:port.
	Addr string
	// Username and Password are the RDP credentials to use for authentication.
	Username string
	Password string
	// ClientWidth and ClientHeight define the size of the outbound desktop
	// image.
	ClientWidth  uint16
	ClientHeight uint16
	// InputMessage is called to receive a message from the client for the RDP
	// server. This function should block until there is a message.
	InputMessage func() (deskproto.Message, error)
	// OutputMessage is called to send a message from RDP server to the client.
	OutputMessage func(deskproto.Message) error
}

func (o Options) validate() error {
	if o.Addr == "" {
		return trace.BadParameter("missing Addr in rdpclient.Options")
	}
	if o.ClientWidth == 0 {
		return trace.BadParameter("missing ClientWidth in rdpclient.Options")
	}
	if o.ClientHeight == 0 {
		return trace.BadParameter("missing ClientHeight in rdpclient.Options")
	}
	if o.InputMessage == nil {
		return trace.BadParameter("missing InputMessage in rdpclient.Options")
	}
	if o.OutputMessage == nil {
		return trace.BadParameter("missing OutputMessage in rdpclient.Options")
	}
	return nil
}

// Client is the RDP client.
type Client struct {
	opts          Options
	rustClientRef *C.Client
	wg            sync.WaitGroup
	closeOnce     sync.Once
}

// New creates and connects a new Client based on opts.
func New(opts Options) (*Client, error) {
	c := &Client{opts: opts}
	if err := c.connect(); err != nil {
		return nil, trace.Wrap(err)
	}
	c.start()
	return c, nil
}

func (c *Client) connect() error {
	addr := C.CString(c.opts.Addr)
	defer C.free(unsafe.Pointer(addr))
	username := C.CString(c.opts.Username)
	defer C.free(unsafe.Pointer(username))
	password := C.CString(c.opts.Password)
	defer C.free(unsafe.Pointer(password))

	res := C.connect_rdp(
		addr,
		username,
		password,
		C.uint16_t(c.opts.ClientWidth),
		C.uint16_t(c.opts.ClientHeight),
	)
	if err := cgoError(res.err); err != nil {
		return trace.Wrap(err)
	}
	c.rustClientRef = res.client
	return nil
}

func (c *Client) start() {
	// Video output streaming worker goroutine.
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		defer c.closeConn()
		defer logrus.Info("RDP output streaming finished")

		clientRef := registerClient(c)
		defer unregisterClient(clientRef)

		if err := cgoError(C.read_rdp_output(c.rustClientRef, C.int64_t(clientRef))); err != nil {
			logrus.Warningf("Failed reading RDP output frame: %v", err)
		}
	}()

	// User input streaming worker goroutine.
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		defer c.closeConn()
		defer logrus.Info("RDP input streaming finished")
		var mouseX, mouseY uint32
		for {
			msg, err := c.opts.InputMessage()
			if err != nil {
				logrus.Warningf("Failed reading RDP input message: %v", err)
				return
			}
			switch m := msg.(type) {
			case deskproto.MouseMove:
				mouseX, mouseY = m.X, m.Y
				if err := cgoError(C.write_rdp_pointer(
					c.rustClientRef,
					C.CGOPointer{
						x:      C.uint16_t(m.X),
						y:      C.uint16_t(m.Y),
						button: C.PointerButtonNone,
					},
				)); err != nil {
					logrus.Warningf("Failed forwarding RDP input message: %v", err)
					return
				}
			case deskproto.MouseButton:
				var button C.CGOPointerButton
				switch m.Button {
				case deskproto.LeftMouseButton:
					button = C.PointerButtonLeft
				case deskproto.RightMouseButton:
					button = C.PointerButtonRight
				case deskproto.MiddleMouseButton:
					button = C.PointerButtonMiddle
				default:
					button = C.PointerButtonNone
				}
				if err := cgoError(C.write_rdp_pointer(
					c.rustClientRef,
					C.CGOPointer{
						x:      C.uint16_t(mouseX),
						y:      C.uint16_t(mouseY),
						button: uint32(button),
						down:   m.State == deskproto.ButtonPressed,
					},
				)); err != nil {
					logrus.Warningf("Failed forwarding RDP input message: %v", err)
					return
				}
			case deskproto.KeyboardButton:
				if err := cgoError(C.write_rdp_keyboard(
					c.rustClientRef,
					C.CGOKey{
						code: C.uint16_t(m.KeyCode),
						down: m.State == deskproto.ButtonPressed,
					},
				)); err != nil {
					logrus.Warningf("Failed forwarding RDP input message: %v", err)
					return
				}
			default:
				logrus.Warningf("Skipping unimplemented desktop protocol message type %T", msg)
			}
		}
	}()
}

//export handle_bitmap
func handle_bitmap(ci C.int64_t, cb C.CGOBitmap) C.CGOError {
	return findClient(int64(ci)).handleBitmap(cb)
}

func (c *Client) handleBitmap(cb C.CGOBitmap) C.CGOError {
	data := C.GoBytes(unsafe.Pointer(cb.data_ptr), C.int(cb.data_len))
	// Convert BGRA to RGBA. It's likely due to Windows using uint32 values for
	// pixels (ARGB) and encoding them as big endian. The image.RGBA type uses
	// a byte slice with 4-byte segments representing pixels (RGBA).
	for i := 0; i < len(data); i += 4 {
		data[i], data[i+2] = data[i+2], data[i]
	}
	img := image.NewRGBA(image.Rectangle{
		Min: image.Pt(int(cb.dest_left), int(cb.dest_top)),
		Max: image.Pt(int(cb.dest_right)+1, int(cb.dest_bottom)+1),
	})
	copy(img.Pix, data)

	if err := c.opts.OutputMessage(deskproto.PNGFrame{Img: img}); err != nil {
		return C.CString(fmt.Sprintf("failed to send PNG frame %v: %v", img.Rect, err))
	}
	return nil
}

// Wait blocks until the client disconnects and runs the cleanup.
func (c *Client) Wait() error {
	c.wg.Wait()
	C.free_rdp(c.rustClientRef)
	return nil
}

func (c *Client) closeConn() {
	c.closeOnce.Do(func() {
		if err := cgoError(C.close_rdp(c.rustClientRef)); err != nil {
			logrus.Warningf("Error closing RDP connection: %v", err)
		}
	})
}

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

// Global registry of active clients. This allows Rust to reference a specific
// client without sending actual objects around.
var (
	clientsMu    = &sync.RWMutex{}
	clients      = make(map[int64]*Client)
	clientsIndex = int64(-1)
)

func registerClient(c *Client) int64 {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	clientsIndex++
	clients[clientsIndex] = c
	return clientsIndex
}

func unregisterClient(i int64) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	delete(clients, i)
}

func findClient(i int64) *Client {
	clientsMu.RLock()
	defer clientsMu.RUnlock()
	return clients[i]
}
