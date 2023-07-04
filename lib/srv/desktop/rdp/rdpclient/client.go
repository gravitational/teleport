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
//  ReadMessage(MouseMove) ------> write_rdp_pointer
//  ReadMessage(MouseButton) ----> write_rdp_pointer
//  ReadMessage(KeyboardButton) -> write_rdp_keyboard
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
	"encoding/binary"
	"fmt"
	"os"
	"runtime/cgo"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/utils"
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
// Its lifecycle is:
//
// ```
// rdpc := New()         // creates client
// rdpc.Run()   // starts rdp and waits for the duration of the connection
// ```
type Client struct {
	cfg Config

	// Parameters read from the TDP stream.
	clientWidth, clientHeight uint16
	username                  string

	// handle allows the rust code to call back into the client.
	handle cgo.Handle

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

	// png2FrameBuffer is used in the handlePNG function
	// to avoid allocation of the buffer on each png as
	// that part of the code is performance-sensitive.
	png2FrameBuffer []byte

	clientActivityMu sync.RWMutex
	clientLastActive time.Time
}

// New creates and connects a new Client based on cfg.
func New(cfg Config) (*Client, error) {
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
	return c, nil
}

// Run starts the rdp client and blocks until the client disconnects,
// then ensures the cleanup is run.
func (c *Client) Run(ctx context.Context) error {
	defer c.cleanup()

	c.handle = cgo.NewHandle(c)

	if err := c.connect(ctx); err != nil {
		return trace.Wrap(err)
	}
	c.start()

	// Hang until input and output streaming
	// goroutines both finish.
	c.wg.Wait()

	// Both goroutines have finished, it's now
	// safe for the deferred c.cleanup() call to
	// clean up the memory.

	return nil
}

func (c *Client) readClientUsername() error {
	for {
		msg, err := c.cfg.Conn.ReadMessage()
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
		msg, err := c.cfg.Conn.ReadMessage()
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
		C.uintptr_t(c.handle),
		C.CGOConnectParams{
			go_addr:     addr,
			go_username: username,
			// cert length and bytes.
			cert_der_len: C.uint32_t(len(userCertDER)),
			cert_der:     (*C.uint8_t)(unsafe.Pointer(&userCertDER[0])),
			// key length and bytes.
			key_der_len:             C.uint32_t(len(userKeyDER)),
			key_der:                 (*C.uint8_t)(unsafe.Pointer(&userKeyDER[0])),
			screen_width:            C.uint16_t(c.clientWidth),
			screen_height:           C.uint16_t(c.clientHeight),
			allow_clipboard:         C.bool(c.cfg.AllowClipboard),
			allow_directory_sharing: C.bool(c.cfg.AllowDirectorySharing),
			show_desktop_wallpaper:  C.bool(c.cfg.ShowDesktopWallpaper),
		},
	)
	if res.err != C.ErrCodeSuccess {
		return trace.ConnectionProblem(nil, "RDP connection failed")
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
		defer c.close()
		defer c.cfg.Log.Info("RDP output streaming finished")

		c.cfg.Log.Info("RDP output streaming starting")

		// C.read_rdp_output blocks for the duration of the RDP connection and
		// calls handle_png repeatedly with the incoming pngs.
		res := C.read_rdp_output(c.rustClient)

		// Copy the returned message and free the C memory.
		userMessage := C.GoString(res.user_message)
		C.free_c_string(res.user_message)

		// If the disconnect was initiated by the server or for
		// an unknown reason, try to alert the user as to why via
		// a TDP error message.
		if res.disconnect_code != C.DisconnectCodeClient {
			if err := c.cfg.Conn.WriteMessage(tdp.Error{
				Message: fmt.Sprintf("The Windows Desktop disconnected: %v", userMessage),
			}); err != nil {
				c.cfg.Log.WithError(err).Error("error sending server disconnect reason over TDP")
			}
		}

		// Select the logger to use based on the error code.
		logf := c.cfg.Log.Infof
		if res.err_code == C.ErrCodeFailure {
			logf = c.cfg.Log.Errorf
		}

		// Log a message to the user.
		var logPrefix string
		if res.disconnect_code == C.DisconnectCodeClient {
			logPrefix = "the RDP client ended the session with message: %v"
		} else if res.disconnect_code == C.DisconnectCodeServer {
			logPrefix = "the RDP server ended the session with message: %v"
		} else {
			logPrefix = "the RDP session ended unexpectedly with message: %v"
		}
		logf(logPrefix, userMessage)
	}()

	// User input streaming worker goroutine.
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		defer c.close()
		defer c.cfg.Log.Info("TDP input streaming finished")

		c.cfg.Log.Info("TDP input streaming starting")

		// Remember mouse coordinates to send them with all CGOPointer events.
		var mouseX, mouseY uint32
		for {
			msg, err := c.cfg.Conn.ReadMessage()
			if utils.IsOKNetworkError(err) {
				return
			} else if tdp.IsNonFatalErr(err) {
				c.cfg.Conn.SendNotification(err.Error(), tdp.SeverityWarning)
				continue
			} else if err != nil {
				c.cfg.Log.Warningf("Failed reading TDP input message: %v", err)
				return
			}

			if atomic.LoadUint32(&c.readyForInput) == 0 {
				// Input not allowed yet, drop the message.
				c.cfg.Log.Debugf("Dropping TDP input message: %T", msg)
				continue
			}

			c.UpdateClientActivity()

			switch m := msg.(type) {
			case tdp.MouseMove:
				mouseX, mouseY = m.X, m.Y
				if errCode := C.write_rdp_pointer(
					c.rustClient,
					C.CGOMousePointerEvent{
						x:      C.uint16_t(m.X),
						y:      C.uint16_t(m.Y),
						button: C.PointerButtonNone,
						wheel:  C.PointerWheelNone,
					},
				); errCode != C.ErrCodeSuccess {
					c.cfg.Log.Warningf("MouseMove: write_rdp_pointer @ (%v,%v): %v", m.X, m.Y, errCode)
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
				if errCode := C.write_rdp_pointer(
					c.rustClient,
					C.CGOMousePointerEvent{
						x:      C.uint16_t(mouseX),
						y:      C.uint16_t(mouseY),
						button: uint32(button),
						down:   m.State == tdp.ButtonPressed,
						wheel:  C.PointerWheelNone,
					},
				); errCode != C.ErrCodeSuccess {
					c.cfg.Log.Warningf("MouseButton: write_rdp_pointer @ (%v, %v) button=%v state=%v: %v",
						mouseX, mouseY, button, m.State, errCode)
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
				if errCode := C.write_rdp_pointer(
					c.rustClient,
					C.CGOMousePointerEvent{
						x:           C.uint16_t(mouseX),
						y:           C.uint16_t(mouseY),
						button:      C.PointerButtonNone,
						wheel:       uint32(wheel),
						wheel_delta: C.int16_t(m.Delta),
					},
				); errCode != C.ErrCodeSuccess {
					c.cfg.Log.Warningf("MouseWheel: write_rdp_pointer @ (%v, %v) wheel=%v delta=%v: %v",
						mouseX, mouseY, wheel, m.Delta, errCode)
					return
				}
			case tdp.KeyboardButton:
				if errCode := C.write_rdp_keyboard(
					c.rustClient,
					C.CGOKeyboardEvent{
						code: C.uint16_t(m.KeyCode),
						down: m.State == tdp.ButtonPressed,
					},
				); errCode != C.ErrCodeSuccess {
					c.cfg.Log.Warningf("KeyboardButton: write_rdp_keyboard code=%v state=%v: %v",
						m.KeyCode, m.State, errCode)
					return
				}
			case tdp.ClipboardData:
				if len(m) > 0 {
					if errCode := C.update_clipboard(
						c.rustClient,
						(*C.uint8_t)(unsafe.Pointer(&m[0])),
						C.uint32_t(len(m)),
					); errCode != C.ErrCodeSuccess {
						c.cfg.Log.Warningf("ClipboardData: update_clipboard (len=%v): %v", len(m), errCode)
						return
					}
				} else {
					c.cfg.Log.Warning("Received an empty clipboard message")
				}
			case tdp.SharedDirectoryAnnounce:
				if c.cfg.AllowDirectorySharing {
					driveName := C.CString(m.Name)
					defer C.free(unsafe.Pointer(driveName))
					if errCode := C.handle_tdp_sd_announce(c.rustClient, C.CGOSharedDirectoryAnnounce{
						directory_id: C.uint32_t(m.DirectoryID),
						name:         driveName,
					}); errCode != C.ErrCodeSuccess {
						c.cfg.Log.Errorf("SharedDirectoryAnnounce: failed with %v", errCode)
						return
					}
				}
			case tdp.SharedDirectoryInfoResponse:
				if c.cfg.AllowDirectorySharing {
					path := C.CString(m.Fso.Path)
					defer C.free(unsafe.Pointer(path))
					if errCode := C.handle_tdp_sd_info_response(c.rustClient, C.CGOSharedDirectoryInfoResponse{
						completion_id: C.uint32_t(m.CompletionID),
						err_code:      m.ErrCode,
						fso: C.CGOFileSystemObject{
							last_modified: C.uint64_t(m.Fso.LastModified),
							size:          C.uint64_t(m.Fso.Size),
							file_type:     m.Fso.FileType,
							is_empty:      C.uint8_t(m.Fso.IsEmpty),
							path:          path,
						},
					}); errCode != C.ErrCodeSuccess {
						c.cfg.Log.Errorf("SharedDirectoryInfoResponse failed: %v", errCode)
						return
					}
				}
			case tdp.SharedDirectoryCreateResponse:
				if c.cfg.AllowDirectorySharing {
					path := C.CString(m.Fso.Path)
					defer C.free(unsafe.Pointer(path))
					if errCode := C.handle_tdp_sd_create_response(c.rustClient, C.CGOSharedDirectoryCreateResponse{
						completion_id: C.uint32_t(m.CompletionID),
						err_code:      m.ErrCode,
						fso: C.CGOFileSystemObject{
							last_modified: C.uint64_t(m.Fso.LastModified),
							size:          C.uint64_t(m.Fso.Size),
							file_type:     m.Fso.FileType,
							is_empty:      C.uint8_t(m.Fso.IsEmpty),
							path:          path,
						},
					}); errCode != C.ErrCodeSuccess {
						c.cfg.Log.Errorf("SharedDirectoryCreateResponse failed: %v", errCode)
						return
					}
				}
			case tdp.SharedDirectoryDeleteResponse:
				if c.cfg.AllowDirectorySharing {
					if errCode := C.handle_tdp_sd_delete_response(c.rustClient, C.CGOSharedDirectoryDeleteResponse{
						completion_id: C.uint32_t(m.CompletionID),
						err_code:      m.ErrCode,
					}); errCode != C.ErrCodeSuccess {
						c.cfg.Log.Errorf("SharedDirectoryDeleteResponse failed: %v", errCode)
						return
					}
				}
			case tdp.SharedDirectoryListResponse:
				if c.cfg.AllowDirectorySharing {
					fsoList := make([]C.CGOFileSystemObject, 0, len(m.FsoList))

					for _, fso := range m.FsoList {
						path := C.CString(fso.Path)
						defer C.free(unsafe.Pointer(path))

						fsoList = append(fsoList, C.CGOFileSystemObject{
							last_modified: C.uint64_t(fso.LastModified),
							size:          C.uint64_t(fso.Size),
							file_type:     fso.FileType,
							is_empty:      C.uint8_t(fso.IsEmpty),
							path:          path,
						})
					}

					fsoListLen := len(fsoList)
					var cgoFsoList *C.CGOFileSystemObject

					if fsoListLen > 0 {
						cgoFsoList = (*C.CGOFileSystemObject)(unsafe.Pointer(&fsoList[0]))
					} else {
						cgoFsoList = (*C.CGOFileSystemObject)(unsafe.Pointer(&fsoList))
					}

					if errCode := C.handle_tdp_sd_list_response(c.rustClient, C.CGOSharedDirectoryListResponse{
						completion_id:   C.uint32_t(m.CompletionID),
						err_code:        m.ErrCode,
						fso_list_length: C.uint32_t(fsoListLen),
						fso_list:        cgoFsoList,
					}); errCode != C.ErrCodeSuccess {
						c.cfg.Log.Errorf("SharedDirectoryListResponse failed: %v", errCode)
						return
					}
				}
			case tdp.SharedDirectoryReadResponse:
				if c.cfg.AllowDirectorySharing {
					var readData *C.uint8_t
					if m.ReadDataLength > 0 {
						readData = (*C.uint8_t)(unsafe.Pointer(&m.ReadData[0]))
					} else {
						readData = (*C.uint8_t)(unsafe.Pointer(&m.ReadData))
					}

					if errCode := C.handle_tdp_sd_read_response(c.rustClient, C.CGOSharedDirectoryReadResponse{
						completion_id:    C.uint32_t(m.CompletionID),
						err_code:         m.ErrCode,
						read_data_length: C.uint32_t(m.ReadDataLength),
						read_data:        readData,
					}); errCode != C.ErrCodeSuccess {
						c.cfg.Log.Errorf("SharedDirectoryReadResponse failed: %v", errCode)
						return
					}
				}
			case tdp.SharedDirectoryWriteResponse:
				if c.cfg.AllowDirectorySharing {
					if errCode := C.handle_tdp_sd_write_response(c.rustClient, C.CGOSharedDirectoryWriteResponse{
						completion_id: C.uint32_t(m.CompletionID),
						err_code:      m.ErrCode,
						bytes_written: C.uint32_t(m.BytesWritten),
					}); errCode != C.ErrCodeSuccess {
						c.cfg.Log.Errorf("SharedDirectoryWriteResponse failed: %v", errCode)
						return
					}
				}
			case tdp.SharedDirectoryMoveResponse:
				if c.cfg.AllowDirectorySharing {
					if errCode := C.handle_tdp_sd_move_response(c.rustClient, C.CGOSharedDirectoryMoveResponse{
						completion_id: C.uint32_t(m.CompletionID),
						err_code:      m.ErrCode,
					}); errCode != C.ErrCodeSuccess {
						c.cfg.Log.Errorf("SharedDirectoryMoveResponse failed: %v", errCode)
						return
					}
				}
			case tdp.RDPResponsePDU:
				pduLen := uint32(len(m))
				if pduLen == 0 {
					c.cfg.Log.Error("response PDU empty")
					return
				}
				rdpResponsePDU := (*C.uint8_t)(unsafe.SliceData(m))

				if errCode := C.handle_tdp_rdp_response_pdu(
					c.rustClient, rdpResponsePDU, C.uint32_t(pduLen),
				); errCode != C.ErrCodeSuccess {
					c.cfg.Log.Errorf("RDPResponsePDU failed: %v", errCode)
					return
				}
			default:
				c.cfg.Log.Warningf("Skipping unimplemented TDP message type %T", msg)
			}
		}
	}()
}

// asRustBackedSlice creates a Go slice backed by data managed in Rust
// without copying it. The caller must ensure that the data is not freed
// by Rust while the slice is in use.
//
// This can be used in lieu of C.GoBytes (which copies the data) wherever
// performance is of greater concern.
func asRustBackedSlice(data *C.uint8_t, length int) []byte {
	ptr := unsafe.Pointer(data)
	uptr := (*uint8)(ptr)
	return unsafe.Slice(uptr, length)
}

//export handle_png
func handle_png(handle C.uintptr_t, cb *C.CGOPNG) C.CGOErrCode {
	return cgo.Handle(handle).Value().(*Client).handlePNG(cb)
}

func (c *Client) handlePNG(cb *C.CGOPNG) C.CGOErrCode {
	// Notify the input forwarding goroutine that we're ready for input.
	// Input can only be sent after connection was established, which we infer
	// from the fact that a png was sent.
	atomic.StoreUint32(&c.readyForInput, 1)

	data := asRustBackedSlice(cb.data_ptr, int(cb.data_len))

	c.png2FrameBuffer = c.png2FrameBuffer[:0]
	c.png2FrameBuffer = append(c.png2FrameBuffer, byte(tdp.TypePNG2Frame))
	c.png2FrameBuffer = binary.BigEndian.AppendUint32(c.png2FrameBuffer, uint32(len(data)))
	c.png2FrameBuffer = binary.BigEndian.AppendUint32(c.png2FrameBuffer, uint32(cb.dest_left))
	c.png2FrameBuffer = binary.BigEndian.AppendUint32(c.png2FrameBuffer, uint32(cb.dest_top))
	c.png2FrameBuffer = binary.BigEndian.AppendUint32(c.png2FrameBuffer, uint32(cb.dest_right))
	c.png2FrameBuffer = binary.BigEndian.AppendUint32(c.png2FrameBuffer, uint32(cb.dest_bottom))
	c.png2FrameBuffer = append(c.png2FrameBuffer, data...)

	if err := c.cfg.Conn.WriteMessage(tdp.PNG2Frame(c.png2FrameBuffer)); err != nil {
		c.cfg.Log.Errorf("failed to write PNG2Frame: %v", err)
		return C.ErrCodeFailure
	}
	return C.ErrCodeSuccess
}

//export handle_remote_fx_frame
func handle_remote_fx_frame(handle C.uintptr_t, data *C.uint8_t, length C.uint32_t) C.CGOErrCode {
	goData := asRustBackedSlice(data, int(length))
	return cgo.Handle(handle).Value().(*Client).handleRDPFastPathPDU(goData)
}

func (c *Client) handleRDPFastPathPDU(data []byte) C.CGOErrCode {
	// Notify the input forwarding goroutine that we're ready for input.
	// Input can only be sent after connection was established, which we infer
	// from the fact that a fast path pdu was sent.
	atomic.StoreUint32(&c.readyForInput, 1)

	if err := c.cfg.Conn.WriteMessage(tdp.RDPFastPathPDU(data)); err != nil {
		c.cfg.Log.Errorf("failed handling RDPFastPathPDU: %v", err)
		return C.ErrCodeFailure
	}
	return C.ErrCodeSuccess
}

//export handle_remote_copy
func handle_remote_copy(handle C.uintptr_t, data *C.uint8_t, length C.uint32_t) C.CGOErrCode {
	goData := C.GoBytes(unsafe.Pointer(data), C.int(length))
	return cgo.Handle(handle).Value().(*Client).handleRemoteCopy(goData)
}

// handleRemoteCopy is called from Rust when data is copied
// on the remote desktop
func (c *Client) handleRemoteCopy(data []byte) C.CGOErrCode {
	c.cfg.Log.Debugf("Received %d bytes of clipboard data from Windows desktop", len(data))

	if err := c.cfg.Conn.WriteMessage(tdp.ClipboardData(data)); err != nil {
		c.cfg.Log.Errorf("failed handling remote copy: %v", err)
		return C.ErrCodeFailure
	}
	return C.ErrCodeSuccess
}

//export tdp_sd_acknowledge
func tdp_sd_acknowledge(handle C.uintptr_t, ack *C.CGOSharedDirectoryAcknowledge) C.CGOErrCode {
	return cgo.Handle(handle).Value().(*Client).sharedDirectoryAcknowledge(tdp.SharedDirectoryAcknowledge{
		ErrCode:     uint32(ack.err_code),
		DirectoryID: uint32(ack.directory_id),
	})
}

// sharedDirectoryAcknowledge is sent by the TDP server to the client
// to acknowledge that a SharedDirectoryAnnounce was received.
func (c *Client) sharedDirectoryAcknowledge(ack tdp.SharedDirectoryAcknowledge) C.CGOErrCode {
	if !c.cfg.AllowDirectorySharing {
		return C.ErrCodeFailure
	}

	if err := c.cfg.Conn.WriteMessage(ack); err != nil {
		c.cfg.Log.Errorf("failed to send SharedDirectoryAcknowledge: %v", err)
		return C.ErrCodeFailure
	}
	return C.ErrCodeSuccess
}

//export tdp_sd_info_request
func tdp_sd_info_request(handle C.uintptr_t, req *C.CGOSharedDirectoryInfoRequest) C.CGOErrCode {
	return cgo.Handle(handle).Value().(*Client).sharedDirectoryInfoRequest(tdp.SharedDirectoryInfoRequest{
		CompletionID: uint32(req.completion_id),
		DirectoryID:  uint32(req.directory_id),
		Path:         C.GoString(req.path),
	})
}

// sharedDirectoryInfoRequest is sent from the TDP server to the client
// to request information about a file or directory at a given path.
func (c *Client) sharedDirectoryInfoRequest(req tdp.SharedDirectoryInfoRequest) C.CGOErrCode {
	if !c.cfg.AllowDirectorySharing {
		return C.ErrCodeFailure
	}

	if err := c.cfg.Conn.WriteMessage(req); err != nil {
		c.cfg.Log.Errorf("failed to send SharedDirectoryAcknowledge: %v", err)
		return C.ErrCodeFailure
	}
	return C.ErrCodeSuccess
}

//export tdp_sd_create_request
func tdp_sd_create_request(handle C.uintptr_t, req *C.CGOSharedDirectoryCreateRequest) C.CGOErrCode {
	return cgo.Handle(handle).Value().(*Client).sharedDirectoryCreateRequest(tdp.SharedDirectoryCreateRequest{
		CompletionID: uint32(req.completion_id),
		DirectoryID:  uint32(req.directory_id),
		FileType:     uint32(req.file_type),
		Path:         C.GoString(req.path),
	})
}

// sharedDirectoryCreateRequest is sent by the TDP server to
// the client to request the creation of a new file or directory.
func (c *Client) sharedDirectoryCreateRequest(req tdp.SharedDirectoryCreateRequest) C.CGOErrCode {
	if !c.cfg.AllowDirectorySharing {
		return C.ErrCodeFailure
	}

	if err := c.cfg.Conn.WriteMessage(req); err != nil {
		c.cfg.Log.Errorf("failed to send SharedDirectoryCreateRequest: %v", err)
		return C.ErrCodeFailure
	}
	return C.ErrCodeSuccess
}

//export tdp_sd_delete_request
func tdp_sd_delete_request(handle C.uintptr_t, req *C.CGOSharedDirectoryDeleteRequest) C.CGOErrCode {
	return cgo.Handle(handle).Value().(*Client).sharedDirectoryDeleteRequest(tdp.SharedDirectoryDeleteRequest{
		CompletionID: uint32(req.completion_id),
		DirectoryID:  uint32(req.directory_id),
		Path:         C.GoString(req.path),
	})
}

// sharedDirectoryDeleteRequest is sent by the TDP server to the client
// to request the deletion of a file or directory at path.
func (c *Client) sharedDirectoryDeleteRequest(req tdp.SharedDirectoryDeleteRequest) C.CGOErrCode {
	if !c.cfg.AllowDirectorySharing {
		return C.ErrCodeFailure
	}

	if err := c.cfg.Conn.WriteMessage(req); err != nil {
		c.cfg.Log.Errorf("failed to send SharedDirectoryDeleteRequest: %v", err)
		return C.ErrCodeFailure
	}
	return C.ErrCodeSuccess
}

//export tdp_sd_list_request
func tdp_sd_list_request(handle C.uintptr_t, req *C.CGOSharedDirectoryListRequest) C.CGOErrCode {
	return cgo.Handle(handle).Value().(*Client).sharedDirectoryListRequest(tdp.SharedDirectoryListRequest{
		CompletionID: uint32(req.completion_id),
		DirectoryID:  uint32(req.directory_id),
		Path:         C.GoString(req.path),
	})
}

// sharedDirectoryListRequest is sent by the TDP server to the client
// to request the contents of a directory.
func (c *Client) sharedDirectoryListRequest(req tdp.SharedDirectoryListRequest) C.CGOErrCode {
	if !c.cfg.AllowDirectorySharing {
		return C.ErrCodeFailure
	}

	if err := c.cfg.Conn.WriteMessage(req); err != nil {
		c.cfg.Log.Errorf("failed to send SharedDirectoryListRequest: %v", err)
		return C.ErrCodeFailure
	}
	return C.ErrCodeSuccess
}

//export tdp_sd_read_request
func tdp_sd_read_request(handle C.uintptr_t, req *C.CGOSharedDirectoryReadRequest) C.CGOErrCode {
	return cgo.Handle(handle).Value().(*Client).sharedDirectoryReadRequest(tdp.SharedDirectoryReadRequest{
		CompletionID: uint32(req.completion_id),
		DirectoryID:  uint32(req.directory_id),
		Path:         C.GoString(req.path),
		Offset:       uint64(req.offset),
		Length:       uint32(req.length),
	})
}

// SharedDirectoryReadRequest is sent by the TDP server to the client
// to request the contents of a file.
func (c *Client) sharedDirectoryReadRequest(req tdp.SharedDirectoryReadRequest) C.CGOErrCode {
	if !c.cfg.AllowDirectorySharing {
		return C.ErrCodeFailure
	}

	if err := c.cfg.Conn.WriteMessage(req); err != nil {
		c.cfg.Log.Errorf("failed to send SharedDirectoryReadRequest: %v", err)
		return C.ErrCodeFailure
	}
	return C.ErrCodeSuccess
}

//export tdp_sd_write_request
func tdp_sd_write_request(handle C.uintptr_t, req *C.CGOSharedDirectoryWriteRequest) C.CGOErrCode {
	return cgo.Handle(handle).Value().(*Client).sharedDirectoryWriteRequest(tdp.SharedDirectoryWriteRequest{
		CompletionID:    uint32(req.completion_id),
		DirectoryID:     uint32(req.directory_id),
		Offset:          uint64(req.offset),
		Path:            C.GoString(req.path),
		WriteDataLength: uint32(req.write_data_length),
		WriteData:       C.GoBytes(unsafe.Pointer(req.write_data), C.int(req.write_data_length)),
	})
}

// SharedDirectoryWriteRequest is sent by the TDP server to the client
// to write to a file.
func (c *Client) sharedDirectoryWriteRequest(req tdp.SharedDirectoryWriteRequest) C.CGOErrCode {
	if !c.cfg.AllowDirectorySharing {
		return C.ErrCodeFailure
	}

	if err := c.cfg.Conn.WriteMessage(req); err != nil {
		c.cfg.Log.Errorf("failed to send SharedDirectoryWriteRequest: %v", err)
		return C.ErrCodeFailure
	}
	return C.ErrCodeSuccess
}

//export tdp_sd_move_request
func tdp_sd_move_request(handle C.uintptr_t, req *C.CGOSharedDirectoryMoveRequest) C.CGOErrCode {
	return cgo.Handle(handle).Value().(*Client).sharedDirectoryMoveRequest(tdp.SharedDirectoryMoveRequest{
		CompletionID: uint32(req.completion_id),
		DirectoryID:  uint32(req.directory_id),
		OriginalPath: C.GoString(req.original_path),
		NewPath:      C.GoString(req.new_path),
	})
}

func (c *Client) sharedDirectoryMoveRequest(req tdp.SharedDirectoryMoveRequest) C.CGOErrCode {
	if !c.cfg.AllowDirectorySharing {
		return C.ErrCodeFailure
	}

	if err := c.cfg.Conn.WriteMessage(req); err != nil {
		c.cfg.Log.Errorf("failed to send SharedDirectoryMoveRequest: %v", err)
		return C.ErrCodeFailure
	}
	return C.ErrCodeSuccess

}

// close closes the RDP client connection and
// the TDP connection to the browser.
func (c *Client) close() {
	c.closeOnce.Do(func() {
		// Ensure the RDP connection is closed
		if errCode := C.close_rdp(c.rustClient); errCode != C.ErrCodeSuccess {
			c.cfg.Log.Warningf("error closing the RDP connection")
		} else {
			c.cfg.Log.Debug("RDP connection closed successfully")
		}

		// Ensure the TDP connection is closed
		if err := c.cfg.Conn.Close(); err != nil {
			c.cfg.Log.Warningf("error closing the TDP connection: %v", err)
		} else {
			c.cfg.Log.Debug("TDP connection closed successfully")
		}
	})
}

// cleanup frees the Rust client and
// frees the memory of the cgo.Handle.
// This function should only be called
// once per Client.
func (c *Client) cleanup() {
	// Let the Rust side free its data
	if c.rustClient != nil {
		C.free_rdp(c.rustClient)
	}

	// Release the memory of the cgo.Handle
	if c.handle != 0 {
		c.handle.Delete()
	}

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
