//go:build desktop_access_rdp
// +build desktop_access_rdp

/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package rdpclient

// Some implementation details that don't belong in the public godoc:
// This package wraps a Rust library that ultimately calls IronRDP
// (https://github.com/Devolutions/IronRDP).
//
// The Rust library is statically-compiled and called via CGO.
// The Go code sends and receives the CGO versions of Rust RDP/TDP
// events and passes them to and from the browser.
//
// The flow is roughly this:
//    Go                                Rust
// ==============================================
//  rdpclient.Run -----------------> client_run
//                    *connected*
//                                    run_read_loop
//  handleRDPFastPathPDU <----------- cgo_handle_fastpath_pdu
//  handleRDPFastPathPDU <-----------
//  handleRDPFastPathPDU <-----------
//  			 *fast path (screen) streaming continues...*
//
//              *user input messages*
//                                   run_write_loop
//  ReadMessage(MouseMove) --------> client_write_rdp_pointer
//  ReadMessage(MouseButton) ------> client_write_rdp_pointer
//  ReadMessage(KeyboardButton) ---> client_write_rdp_keyboard
//            *user input continues...*
//
//        *connection closed (client or server side)*
//
//  The wds <--> RDP connection is guaranteed to close when the rust Client is dropped,
//  which happens when client_run returns (typically either due to an error or because
//  client_stop was called).
//
//  The browser <--> wds connection is guaranteed to close when WindowsService.handleConnection
//  returns.

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

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

func init() {
	var rustLogLevel string

	// initialize the Rust logger by setting $RUST_LOG based
	// on the logrus log level
	// (unless RUST_LOG is already explicitly set, then we
	// assume the user knows what they want)
	rl := os.Getenv("RUST_LOG")
	if rl == "" {
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

		// sspi-rs info-level logs are extremely verbose, so filter them out by default
		// TODO(zmb3): remove this after sspi-rs logging is cleaned up
		rustLogLevel += ",sspi=warn"

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

	// Parameters read from the TDP stream
	requestedWidth, requestedHeight uint16
	username                        string

	// handle allows the rust code to call back into the client.
	handle cgo.Handle

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

	// mouseX and mouseY are the last mouse coordinates sent to the client.
	mouseX, mouseY uint32
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
	// Create a handle to the client to pass to Rust.
	// The handle is used to call back into this Client from Rust.
	// Since the handle is created and deleted here, methods which
	// rely on a valid c.handle can only be called between here and
	// when this function returns.
	c.handle = cgo.NewHandle(c)
	defer c.handle.Delete()

	// Create a channel to signal the startInputStreaming goroutine to stop
	stopCh := make(chan struct{})

	inputStreamingReturnCh := make(chan error, 1)
	// Kick off input streaming goroutine
	go func() {
		inputStreamingReturnCh <- c.startInputStreaming(stopCh)
	}()

	rustRDPReturnCh := make(chan error, 1)
	// Kick off rust RDP loop goroutine
	go func() {
		rustRDPReturnCh <- c.startRustRDP(ctx)
	}()

	select {
	case err := <-rustRDPReturnCh:
		// Ensure the startInputStreaming goroutine returns.
		close(stopCh)
		return trace.Wrap(err)
	case err := <-inputStreamingReturnCh:
		// Ensure the startRustRDP goroutine returns.
		stopErr := c.stopRustRDP()
		return trace.NewAggregate(err, stopErr)
	}
}

func (c *Client) GetClientUsername() string {
	return c.username
}

func (c *Client) readClientUsername() error {
	for {
		msg, err := c.cfg.Conn.ReadMessage()
		if err != nil {
			return trace.Wrap(err)
		}
		u, ok := msg.(tdp.ClientUsername)
		if !ok {
			c.cfg.Logger.DebugContext(context.Background(), fmt.Sprintf("Expected ClientUsername message, got %T", msg))
			continue
		}
		c.cfg.Logger.DebugContext(context.Background(), "Got RDP username", "username", u.Username)
		c.username = u.Username
		return nil
	}
}

func (c *Client) readClientSize() error {
	for {
		s, err := c.cfg.Conn.ReadClientScreenSpec()
		if err != nil {
			c.cfg.Logger.DebugContext(context.Background(), "Failed to read client screen spec", "error", err)
			continue
		}

		if c.cfg.hasSizeOverride() {
			// Some desktops have a screen size in their resource definition.
			// If non-zero then we always request this screen size.
			c.cfg.Logger.DebugContext(context.Background(), "Forcing a fixed screen size", "width", c.cfg.Width, "height", c.cfg.Height)
			c.requestedWidth = uint16(c.cfg.Width)
			c.requestedHeight = uint16(c.cfg.Height)
		} else {
			// If not otherwise specified, we request the screen size based
			// on what the client (browser) reports.
			c.cfg.Logger.DebugContext(context.Background(), "Got RDP screen size", "width", s.Width, "height", s.Height)
			c.requestedWidth = uint16(s.Width)
			c.requestedHeight = uint16(s.Height)
		}

		if c.requestedWidth > types.MaxRDPScreenWidth || c.requestedHeight > types.MaxRDPScreenHeight {
			err = trace.BadParameter(
				"screen size of %d x %d is greater than the maximum allowed by RDP (%d x %d)",
				s.Width, s.Height, types.MaxRDPScreenWidth, types.MaxRDPScreenHeight,
			)
			if err := c.sendTDPNotification(err.Error(), tdp.SeverityError); err != nil {
				return trace.Wrap(err)
			}
			return trace.Wrap(err)
		}

		return nil
	}
}

func (c *Client) sendTDPNotification(message string, severity tdp.Severity) error {
	return c.cfg.Conn.WriteMessage(tdp.Notification{Message: message, Severity: severity})
}

func (c *Client) startRustRDP(ctx context.Context) error {
	c.cfg.Logger.InfoContext(ctx, "Rust RDP loop starting")
	defer c.cfg.Logger.InfoContext(ctx, "Rust RDP loop finished")

	userCertDER, userKeyDER, err := c.cfg.GenerateUserCert(ctx, c.username, c.cfg.CertTTL)
	if err != nil {
		return trace.Wrap(err)
	}

	// [username] need only be valid for the duration of
	// C.client_run. It is copied on the Rust side and
	// thus can be freed here.
	username := C.CString(c.username)
	defer C.free(unsafe.Pointer(username))

	// [addr] need only be valid for the duration of
	// C.client_run. It is copied on the Rust side and
	// thus can be freed here.
	addr := C.CString(c.cfg.Addr)
	defer C.free(unsafe.Pointer(addr))

	// [kdcAddr] need only be valid for the duration of
	// C.client_run. It is copied on the Rust side and
	// thus can be freed here.
	kdcAddr := C.CString(c.cfg.KDCAddr)
	defer C.free(unsafe.Pointer(kdcAddr))

	// [computerName] need only be valid for the duration of
	// C.client_run. It is copied on the Rust side and
	// thus can be freed here.
	computerName := C.CString(c.cfg.ComputerName)
	defer C.free(unsafe.Pointer(computerName))

	cert_der, err := utils.UnsafeSliceData(userCertDER)
	if err != nil {
		return trace.Wrap(err)
	} else if cert_der == nil {
		return trace.BadParameter("user cert was nil")
	}

	key_der, err := utils.UnsafeSliceData(userKeyDER)
	if err != nil {
		return trace.Wrap(err)
	} else if key_der == nil {
		return trace.BadParameter("user key was nil")
	}

	hostID, err := uuid.Parse(c.cfg.HostID)
	if err != nil {
		return trace.Wrap(err)
	}

	nextHostID := hostID[:]
	cHostID := [4]C.uint32_t{}
	for i := 0; i < len(cHostID); i++ {
		const uint32Len = 4
		cHostID[i] = (C.uint32_t)(binary.LittleEndian.Uint32(nextHostID[:uint32Len]))
		nextHostID = nextHostID[uint32Len:]
	}

	res := C.client_run(
		C.uintptr_t(c.handle),
		C.CGOConnectParams{
			ad:               C.bool(c.cfg.AD),
			nla:              C.bool(c.cfg.NLA),
			go_username:      username,
			go_addr:          addr,
			go_computer_name: computerName,
			go_kdc_addr:      kdcAddr,
			// cert length and bytes.
			cert_der_len: C.uint32_t(len(userCertDER)),
			cert_der:     (*C.uint8_t)(cert_der),
			// key length and bytes.
			key_der_len:             C.uint32_t(len(userKeyDER)),
			key_der:                 (*C.uint8_t)(key_der),
			screen_width:            C.uint16_t(c.requestedWidth),
			screen_height:           C.uint16_t(c.requestedHeight),
			allow_clipboard:         C.bool(c.cfg.AllowClipboard),
			allow_directory_sharing: C.bool(c.cfg.AllowDirectorySharing),
			show_desktop_wallpaper:  C.bool(c.cfg.ShowDesktopWallpaper),
			client_id:               cHostID,
		},
	)

	var message string
	if res.message != nil {
		message = C.GoString(res.message)
		defer C.free_string(res.message)
	}

	// If the client exited with an error, send a tdp error notification and return it.
	if res.err_code != C.ErrCodeSuccess {
		var err error

		if message != "" {
			err = trace.Errorf("RDP client exited with an error: %v", message)
		} else {
			err = trace.Errorf("RDP client exited with an unknown error")
		}

		c.sendTDPNotification(err.Error(), tdp.SeverityError)
		return err
	}

	if message != "" {
		message = fmt.Sprintf("RDP client exited gracefully with message: %v", message)
	} else {
		message = "RDP client exited gracefully"
	}

	c.cfg.Logger.InfoContext(ctx, message)
	c.sendTDPNotification(message, tdp.SeverityError)

	return nil
}

func (c *Client) stopRustRDP() error {
	if errCode := C.client_stop(C.uintptr_t(c.handle)); errCode != C.ErrCodeSuccess {
		return trace.Errorf("client_stop failed: %v", errCode)
	}
	return nil
}

// start_input_streaming kicks off goroutines for input/output streaming and returns right
// away. Use Wait to wait for them to finish.
func (c *Client) startInputStreaming(stopCh chan struct{}) error {
	c.cfg.Logger.InfoContext(context.Background(), "TDP input streaming starting")
	defer c.cfg.Logger.InfoContext(context.Background(), "TDP input streaming finished")

	var withheldResize *tdp.ClientScreenSpec
	for {
		select {
		case <-stopCh:
			return nil
		default:
		}

		msg, err := c.cfg.Conn.ReadMessage()
		if utils.IsOKNetworkError(err) {
			return nil
		} else if tdp.IsNonFatalErr(err) {
			c.cfg.Conn.SendNotification(err.Error(), tdp.SeverityWarning)
			continue
		} else if err != nil {
			c.cfg.Logger.WarnContext(context.Background(), "Failed reading TDP input message", "error", err)
			return err
		}

		if atomic.LoadUint32(&c.readyForInput) == 0 {
			switch m := msg.(type) {
			case tdp.ClientScreenSpec:
				// Withhold the latest screen size until the client is ready for input. This ensures
				// that the client receives the correct screen size when it is ready.
				withheldResize = &m
				c.cfg.Logger.DebugContext(context.Background(), "Withholding screen size until client is ready for input", "width", m.Width, "height", m.Height)
			default:
				// Ignore all messages except ClientScreenSpec until the client is ready for input.
				c.cfg.Logger.DebugContext(context.Background(), "Dropping TDP input message, not ready for input")
			}

			continue
		}

		c.UpdateClientActivity()

		if withheldResize != nil {
			c.cfg.Logger.DebugContext(context.Background(), "Sending withheld screen size to client")
			if err := c.handleTDPInput(*withheldResize); err != nil {
				return trace.Wrap(err)
			}
			withheldResize = nil
		}

		if err := c.handleTDPInput(msg); err != nil {
			return trace.Wrap(err)
		}
	}
}

// handleTDPInput handles a single TDP message sent to us from the browser.
func (c *Client) handleTDPInput(msg tdp.Message) error {
	switch m := msg.(type) {
	case tdp.ClientScreenSpec:
		// If the client has specified a fixed screen size, we don't
		// need to send a screen resize event.
		if c.cfg.hasSizeOverride() {
			return nil
		}

		c.cfg.Logger.DebugContext(context.Background(), "Client changed screen size", "width", m.Width, "height", m.Height)
		if errCode := C.client_write_screen_resize(
			C.uintptr_t(c.handle),
			C.uint32_t(m.Width),
			C.uint32_t(m.Height),
		); errCode != C.ErrCodeSuccess {
			return trace.Errorf("ClientScreenSpec: client_write_screen_resize: %v", errCode)
		}
	case tdp.MouseMove:
		c.mouseX, c.mouseY = m.X, m.Y
		if errCode := C.client_write_rdp_pointer(
			C.uintptr_t(c.handle),
			C.CGOMousePointerEvent{
				x:      C.uint16_t(m.X),
				y:      C.uint16_t(m.Y),
				button: C.PointerButtonNone,
				wheel:  C.PointerWheelNone,
			},
		); errCode != C.ErrCodeSuccess {
			return trace.Errorf("MouseMove: client_write_rdp_pointer: %v", errCode)
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
		if errCode := C.client_write_rdp_pointer(
			C.uintptr_t(c.handle),
			C.CGOMousePointerEvent{
				x:      C.uint16_t(c.mouseX),
				y:      C.uint16_t(c.mouseY),
				button: uint32(button),
				down:   m.State == tdp.ButtonPressed,
				wheel:  C.PointerWheelNone,
			},
		); errCode != C.ErrCodeSuccess {
			return trace.Errorf("MouseButton: client_write_rdp_pointer: %v", errCode)
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
		if errCode := C.client_write_rdp_pointer(
			C.uintptr_t(c.handle),
			C.CGOMousePointerEvent{
				x:           C.uint16_t(c.mouseX),
				y:           C.uint16_t(c.mouseY),
				button:      C.PointerButtonNone,
				wheel:       uint32(wheel),
				wheel_delta: C.int16_t(m.Delta),
			},
		); errCode != C.ErrCodeSuccess {
			return trace.Errorf("MouseWheel: client_write_rdp_pointer: %v", errCode)
		}
	case tdp.KeyboardButton:
		if errCode := C.client_write_rdp_keyboard(
			C.uintptr_t(c.handle),
			C.CGOKeyboardEvent{
				code: C.uint16_t(m.KeyCode),
				down: m.State == tdp.ButtonPressed,
			},
		); errCode != C.ErrCodeSuccess {
			return trace.Errorf("KeyboardButton: client_write_rdp_keyboard: %v", errCode)
		}
	case tdp.SyncKeys:
		if errCode := C.client_write_rdp_sync_keys(C.uintptr_t(c.handle),
			C.CGOSyncKeys{
				scroll_lock_down: m.ScrollLockState == tdp.ButtonPressed,
				num_lock_down:    m.NumLockState == tdp.ButtonPressed,
				caps_lock_down:   m.CapsLockState == tdp.ButtonPressed,
				kana_lock_down:   m.KanaLockState == tdp.ButtonPressed,
			}); errCode != C.ErrCodeSuccess {
			return trace.Errorf("SyncKeys: client_write_rdp_sync_keys: %v", errCode)
		}
	case tdp.ClipboardData:
		if !c.cfg.AllowClipboard {
			c.cfg.Logger.DebugContext(context.Background(), "Received clipboard data, but clipboard is disabled")
			return nil
		}
		if len(m) > 0 {
			if errCode := C.client_update_clipboard(
				C.uintptr_t(c.handle),
				(*C.uint8_t)(unsafe.Pointer(&m[0])),
				C.uint32_t(len(m)),
			); errCode != C.ErrCodeSuccess {
				return trace.Errorf("ClipboardData: client_update_clipboard (len=%v): %v", len(m), errCode)
			}
		} else {
			c.cfg.Logger.WarnContext(context.Background(), "Received an empty clipboard message")
		}
	case tdp.SharedDirectoryAnnounce:
		if c.cfg.AllowDirectorySharing {
			driveName := C.CString(m.Name)
			defer C.free(unsafe.Pointer(driveName))
			if errCode := C.client_handle_tdp_sd_announce(C.uintptr_t(c.handle), C.CGOSharedDirectoryAnnounce{
				directory_id: C.uint32_t(m.DirectoryID),
				name:         driveName,
			}); errCode != C.ErrCodeSuccess {
				return trace.Errorf("SharedDirectoryAnnounce: failed with %v", errCode)
			}
		}
	case tdp.SharedDirectoryInfoResponse:
		if c.cfg.AllowDirectorySharing {
			path := C.CString(m.Fso.Path)
			defer C.free(unsafe.Pointer(path))
			if errCode := C.client_handle_tdp_sd_info_response(C.uintptr_t(c.handle), C.CGOSharedDirectoryInfoResponse{
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
				return trace.Errorf("SharedDirectoryInfoResponse failed: %v", errCode)
			}
		}
	case tdp.SharedDirectoryCreateResponse:
		if c.cfg.AllowDirectorySharing {
			path := C.CString(m.Fso.Path)
			defer C.free(unsafe.Pointer(path))
			if errCode := C.client_handle_tdp_sd_create_response(C.uintptr_t(c.handle), C.CGOSharedDirectoryCreateResponse{
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
				return trace.Errorf("SharedDirectoryCreateResponse failed: %v", errCode)
			}
		}
	case tdp.SharedDirectoryDeleteResponse:
		if c.cfg.AllowDirectorySharing {
			if errCode := C.client_handle_tdp_sd_delete_response(C.uintptr_t(c.handle), C.CGOSharedDirectoryDeleteResponse{
				completion_id: C.uint32_t(m.CompletionID),
				err_code:      m.ErrCode,
			}); errCode != C.ErrCodeSuccess {
				return trace.Errorf("SharedDirectoryDeleteResponse failed: %v", errCode)
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

			if errCode := C.client_handle_tdp_sd_list_response(C.uintptr_t(c.handle), C.CGOSharedDirectoryListResponse{
				completion_id:   C.uint32_t(m.CompletionID),
				err_code:        m.ErrCode,
				fso_list_length: C.uint32_t(fsoListLen),
				fso_list:        cgoFsoList,
			}); errCode != C.ErrCodeSuccess {
				return trace.Errorf("SharedDirectoryListResponse failed: %v", errCode)
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

			if errCode := C.client_handle_tdp_sd_read_response(C.uintptr_t(c.handle), C.CGOSharedDirectoryReadResponse{
				completion_id:    C.uint32_t(m.CompletionID),
				err_code:         m.ErrCode,
				read_data_length: C.uint32_t(m.ReadDataLength),
				read_data:        readData,
			}); errCode != C.ErrCodeSuccess {
				return trace.Errorf("SharedDirectoryReadResponse failed: %v", errCode)
			}
		}
	case tdp.SharedDirectoryWriteResponse:
		if c.cfg.AllowDirectorySharing {
			if errCode := C.client_handle_tdp_sd_write_response(C.uintptr_t(c.handle), C.CGOSharedDirectoryWriteResponse{
				completion_id: C.uint32_t(m.CompletionID),
				err_code:      m.ErrCode,
				bytes_written: C.uint32_t(m.BytesWritten),
			}); errCode != C.ErrCodeSuccess {
				return trace.Errorf("SharedDirectoryWriteResponse failed: %v", errCode)
			}
		}
	case tdp.SharedDirectoryMoveResponse:
		if c.cfg.AllowDirectorySharing {
			if errCode := C.client_handle_tdp_sd_move_response(C.uintptr_t(c.handle), C.CGOSharedDirectoryMoveResponse{
				completion_id: C.uint32_t(m.CompletionID),
				err_code:      m.ErrCode,
			}); errCode != C.ErrCodeSuccess {
				return trace.Errorf("SharedDirectoryMoveResponse failed: %v", errCode)
			}
		}
	case tdp.SharedDirectoryTruncateResponse:
		if c.cfg.AllowDirectorySharing {
			if errCode := C.client_handle_tdp_sd_truncate_response(C.uintptr_t(c.handle), C.CGOSharedDirectoryTruncateResponse{
				completion_id: C.uint32_t(m.CompletionID),
				err_code:      m.ErrCode,
			}); errCode != C.ErrCodeSuccess {
				return trace.Errorf("SharedDirectoryTruncateResponse failed: %v", errCode)
			}
		}
	case tdp.RDPResponsePDU:
		pduLen := uint32(len(m))
		if pduLen == 0 {
			c.cfg.Logger.ErrorContext(context.Background(), "response PDU empty")
		}
		rdpResponsePDU := (*C.uint8_t)(unsafe.SliceData(m))

		if errCode := C.client_handle_tdp_rdp_response_pdu(
			C.uintptr_t(c.handle), rdpResponsePDU, C.uint32_t(pduLen),
		); errCode != C.ErrCodeSuccess {
			return trace.Errorf("RDPResponsePDU failed: %v", errCode)
		}
	default:
		c.cfg.Logger.WarnContext(
			context.Background(),
			"Skipping unimplemented TDP message",
			"type", logutils.TypeAttr(msg),
		)
	}

	return nil
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

func toClient(handle C.uintptr_t) (value *Client, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = trace.Errorf("panic: %v", r)
		}
	}()
	return cgo.Handle(handle).Value().(*Client), nil
}

//export cgo_read_rdp_license
func cgo_read_rdp_license(handle C.uintptr_t, req *C.CGOLicenseRequest, data_out **C.uint8_t, len_out *C.size_t) C.CGOErrCode {
	*data_out = nil
	*len_out = 0

	client, err := toClient(handle)
	if err != nil {
		return C.ErrCodeFailure
	}

	issuer := C.GoString(req.issuer)
	company := C.GoString(req.company)
	productID := C.GoString(req.product_id)

	license, err := client.readRDPLicense(context.Background(), types.RDPLicenseKey{
		Version:   uint32(req.version),
		Issuer:    issuer,
		Company:   company,
		ProductID: productID,
	})
	if trace.IsNotFound(err) {
		return C.ErrCodeNotFound
	} else if err != nil {
		return C.ErrCodeFailure
	}

	// in this case, we expect the caller to use cgo_free_rdp_license
	// when the data is no longer needed
	*data_out = (*C.uint8_t)(C.CBytes(license))
	*len_out = C.size_t(len(license))
	return C.ErrCodeSuccess
}

//export cgo_free_rdp_license
func cgo_free_rdp_license(p *C.uint8_t) {
	C.free(unsafe.Pointer(p))
}

//export cgo_write_rdp_license
func cgo_write_rdp_license(handle C.uintptr_t, req *C.CGOLicenseRequest, data *C.uint8_t, length C.size_t) C.CGOErrCode {
	client, err := toClient(handle)
	if err != nil {
		return C.ErrCodeFailure
	}

	issuer := C.GoString(req.issuer)
	company := C.GoString(req.company)
	productID := C.GoString(req.product_id)

	licenseData := C.GoBytes(unsafe.Pointer(data), C.int(length))

	err = client.writeRDPLicense(context.Background(), types.RDPLicenseKey{
		Version:   uint32(req.version),
		Issuer:    issuer,
		Company:   company,
		ProductID: productID,
	}, licenseData)
	if err != nil {
		return C.ErrCodeFailure
	}

	return C.ErrCodeSuccess
}

func (c *Client) readRDPLicense(ctx context.Context, key types.RDPLicenseKey) ([]byte, error) {
	log := c.cfg.Logger.With(
		"issuer", key.Issuer,
		"company", key.Company,
		"version", key.Version,
		"product", key.ProductID,
	)

	license, err := c.cfg.LicenseStore.ReadRDPLicense(ctx, &key)
	switch {
	case trace.IsNotFound(err):
		log.InfoContext(ctx, "existing RDP license not found")
	case err != nil:
		log.ErrorContext(ctx, "could not look up existing RDP license", "error", err)
	case len(license) > 0:
		log.InfoContext(ctx, "found existing RDP license")
	}

	return license, trace.Wrap(err)
}

func (c *Client) writeRDPLicense(ctx context.Context, key types.RDPLicenseKey, license []byte) error {
	log := c.cfg.Logger.With(
		"issuer", key.Issuer,
		"company", key.Company,
		"version", key.Version,
		"product", key.ProductID,
	)
	log.InfoContext(ctx, "writing RDP license to storage")
	err := c.cfg.LicenseStore.WriteRDPLicense(ctx, &key, license)
	if err != nil {
		log.ErrorContext(ctx, "could not write RDP license", "error", err)
	}
	return trace.Wrap(err)
}

//export cgo_handle_fastpath_pdu
func cgo_handle_fastpath_pdu(handle C.uintptr_t, data *C.uint8_t, length C.uint32_t) C.CGOErrCode {
	goData := asRustBackedSlice(data, int(length))
	client, err := toClient(handle)
	if err != nil {
		return C.ErrCodeFailure
	}
	return client.handleRDPFastPathPDU(goData)
}

func (c *Client) handleRDPFastPathPDU(data []byte) C.CGOErrCode {
	// Notify the input forwarding goroutine that we're ready for input.
	// Input can only be sent after connection was established, which we infer
	// from the fact that a fast path pdu was sent.
	atomic.StoreUint32(&c.readyForInput, 1)

	if err := c.cfg.Conn.WriteMessage(tdp.RDPFastPathPDU(data)); err != nil {
		c.cfg.Logger.ErrorContext(context.Background(), "failed handling RDPFastPathPDU", "error", err)
		return C.ErrCodeFailure
	}
	return C.ErrCodeSuccess
}

//export cgo_handle_rdp_connection_activated
func cgo_handle_rdp_connection_activated(
	handle C.uintptr_t,
	io_channel_id C.uint16_t,
	user_channel_id C.uint16_t,
	screen_width C.uint16_t,
	screen_height C.uint16_t,
) C.CGOErrCode {
	client, err := toClient(handle)
	if err != nil {
		return C.ErrCodeFailure
	}
	return client.handleRDPConnectionActivated(io_channel_id, user_channel_id, screen_width, screen_height)
}

func (c *Client) handleRDPConnectionActivated(ioChannelID, userChannelID, screenWidth, screenHeight C.uint16_t) C.CGOErrCode {
	c.cfg.Logger.DebugContext(context.Background(), "Received RDP channel IDs", "io_channel_id", ioChannelID, "user_channel_id", userChannelID)

	// Note: RDP doesn't always use the resolution we asked for.
	// This is especially true when we request dimensions that are not a multiple of 4.
	c.cfg.Logger.DebugContext(context.Background(), "RDP server provided resolution", "width", screenWidth, "height", screenHeight)

	if err := c.cfg.Conn.WriteMessage(tdp.ConnectionActivated{
		IOChannelID:   uint16(ioChannelID),
		UserChannelID: uint16(userChannelID),
		ScreenWidth:   uint16(screenWidth),
		ScreenHeight:  uint16(screenHeight),
	}); err != nil {
		c.cfg.Logger.ErrorContext(context.Background(), "failed handling connection initialization", "error", err)
		return C.ErrCodeFailure
	}
	return C.ErrCodeSuccess
}

//export cgo_handle_remote_copy
func cgo_handle_remote_copy(handle C.uintptr_t, data *C.uint8_t, length C.uint32_t) C.CGOErrCode {
	goData := C.GoBytes(unsafe.Pointer(data), C.int(length))
	client, err := toClient(handle)
	if err != nil {
		return C.ErrCodeFailure
	}
	return client.handleRemoteCopy(goData)
}

// handleRemoteCopy is called from Rust when data is copied
// on the remote desktop
func (c *Client) handleRemoteCopy(data []byte) C.CGOErrCode {
	c.cfg.Logger.DebugContext(context.Background(), "Received clipboard data from Windows desktop", "len", len(data))

	if err := c.cfg.Conn.WriteMessage(tdp.ClipboardData(data)); err != nil {
		c.cfg.Logger.ErrorContext(context.Background(), "failed handling remote copy", "error", err)
		return C.ErrCodeFailure
	}
	return C.ErrCodeSuccess
}

//export cgo_tdp_sd_acknowledge
func cgo_tdp_sd_acknowledge(handle C.uintptr_t, ack *C.CGOSharedDirectoryAcknowledge) C.CGOErrCode {
	client, err := toClient(handle)
	if err != nil {
		return C.ErrCodeFailure
	}
	return client.sharedDirectoryAcknowledge(tdp.SharedDirectoryAcknowledge{
		//nolint:unconvert // Avoid hard dependencies on C types
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
		c.cfg.Logger.ErrorContext(context.Background(), "failed to send SharedDirectoryAcknowledge", "error", err)
		return C.ErrCodeFailure
	}
	return C.ErrCodeSuccess
}

//export cgo_tdp_sd_info_request
func cgo_tdp_sd_info_request(handle C.uintptr_t, req *C.CGOSharedDirectoryInfoRequest) C.CGOErrCode {
	client, err := toClient(handle)
	if err != nil {
		return C.ErrCodeFailure
	}
	return client.sharedDirectoryInfoRequest(tdp.SharedDirectoryInfoRequest{
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
		c.cfg.Logger.ErrorContext(context.Background(), "failed to send SharedDirectoryAcknowledge", "error", err)
		return C.ErrCodeFailure
	}
	return C.ErrCodeSuccess
}

//export cgo_tdp_sd_create_request
func cgo_tdp_sd_create_request(handle C.uintptr_t, req *C.CGOSharedDirectoryCreateRequest) C.CGOErrCode {
	client, err := toClient(handle)
	if err != nil {
		return C.ErrCodeFailure
	}
	return client.sharedDirectoryCreateRequest(tdp.SharedDirectoryCreateRequest{
		CompletionID: uint32(req.completion_id),
		DirectoryID:  uint32(req.directory_id),
		//nolint:unconvert // Avoid hard dependencies on C types.
		FileType: uint32(req.file_type),
		Path:     C.GoString(req.path),
	})
}

// sharedDirectoryCreateRequest is sent by the TDP server to
// the client to request the creation of a new file or directory.
func (c *Client) sharedDirectoryCreateRequest(req tdp.SharedDirectoryCreateRequest) C.CGOErrCode {
	if !c.cfg.AllowDirectorySharing {
		return C.ErrCodeFailure
	}

	if err := c.cfg.Conn.WriteMessage(req); err != nil {
		c.cfg.Logger.ErrorContext(context.Background(), "failed to send SharedDirectoryCreateRequest", "error", err)
		return C.ErrCodeFailure
	}
	return C.ErrCodeSuccess
}

//export cgo_tdp_sd_delete_request
func cgo_tdp_sd_delete_request(handle C.uintptr_t, req *C.CGOSharedDirectoryDeleteRequest) C.CGOErrCode {
	client, err := toClient(handle)
	if err != nil {
		return C.ErrCodeFailure
	}
	return client.sharedDirectoryDeleteRequest(tdp.SharedDirectoryDeleteRequest{
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
		c.cfg.Logger.ErrorContext(context.Background(), "failed to send SharedDirectoryDeleteRequest", "error", err)
		return C.ErrCodeFailure
	}
	return C.ErrCodeSuccess
}

//export cgo_tdp_sd_list_request
func cgo_tdp_sd_list_request(handle C.uintptr_t, req *C.CGOSharedDirectoryListRequest) C.CGOErrCode {
	client, err := toClient(handle)
	if err != nil {
		return C.ErrCodeFailure
	}
	return client.sharedDirectoryListRequest(tdp.SharedDirectoryListRequest{
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
		c.cfg.Logger.ErrorContext(context.Background(), "failed to send SharedDirectoryListRequest", "error", err)
		return C.ErrCodeFailure
	}
	return C.ErrCodeSuccess
}

//export cgo_tdp_sd_read_request
func cgo_tdp_sd_read_request(handle C.uintptr_t, req *C.CGOSharedDirectoryReadRequest) C.CGOErrCode {
	client, err := toClient(handle)
	if err != nil {
		return C.ErrCodeFailure
	}
	return client.sharedDirectoryReadRequest(tdp.SharedDirectoryReadRequest{
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
		c.cfg.Logger.ErrorContext(context.Background(), "failed to send SharedDirectoryReadRequest", "error", err)
		return C.ErrCodeFailure
	}
	return C.ErrCodeSuccess
}

//export cgo_tdp_sd_write_request
func cgo_tdp_sd_write_request(handle C.uintptr_t, req *C.CGOSharedDirectoryWriteRequest) C.CGOErrCode {
	client, err := toClient(handle)
	if err != nil {
		return C.ErrCodeFailure
	}
	return client.sharedDirectoryWriteRequest(tdp.SharedDirectoryWriteRequest{
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
		c.cfg.Logger.ErrorContext(context.Background(), "failed to send SharedDirectoryWriteRequest", "error", err)
		return C.ErrCodeFailure
	}
	return C.ErrCodeSuccess
}

//export cgo_tdp_sd_move_request
func cgo_tdp_sd_move_request(handle C.uintptr_t, req *C.CGOSharedDirectoryMoveRequest) C.CGOErrCode {
	client, err := toClient(handle)
	if err != nil {
		return C.ErrCodeFailure
	}
	return client.sharedDirectoryMoveRequest(tdp.SharedDirectoryMoveRequest{
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
		c.cfg.Logger.ErrorContext(context.Background(), "failed to send SharedDirectoryMoveRequest", "error", err)
		return C.ErrCodeFailure
	}
	return C.ErrCodeSuccess

}

//export cgo_tdp_sd_truncate_request
func cgo_tdp_sd_truncate_request(handle C.uintptr_t, req *C.CGOSharedDirectoryTruncateRequest) C.CGOErrCode {
	client, err := toClient(handle)
	if err != nil {
		return C.ErrCodeFailure
	}
	return client.sharedDirectoryTruncateRequest(tdp.SharedDirectoryTruncateRequest{
		CompletionID: uint32(req.completion_id),
		DirectoryID:  uint32(req.directory_id),
		Path:         C.GoString(req.path),
		EndOfFile:    uint32(req.end_of_file),
	})
}

func (c *Client) sharedDirectoryTruncateRequest(req tdp.SharedDirectoryTruncateRequest) C.CGOErrCode {
	if !c.cfg.AllowDirectorySharing {
		return C.ErrCodeFailure
	}

	if err := c.cfg.Conn.WriteMessage(req); err != nil {
		c.cfg.Logger.ErrorContext(context.Background(), "failed to send SharedDirectoryTruncateRequest", "error", err)
		return C.ErrCodeFailure
	}
	return C.ErrCodeSuccess

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
