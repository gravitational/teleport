//go:build desktop_access_rdp

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
// This package drives a standalone Rust binary (rdp-client) that
// ultimately calls IronRDP (https://github.com/Devolutions/IronRDP).
//
// The Rust binary is launched as a child process by Client.Run. The Go code and the Rust process
// communicate over a gRPC on a private Unix domain socket created for each session.
//
// The flow is roughly this:
//    Go                                                       Rust
// ==============================================================================
//  Client.Run   -------------spawns subprocess------------->  main
//                               *connected*
//                                                         run_read_loop
//  desktopServiceServer.forwardRustToTDP <----------- Session.Send(FastPathPDU)
//  desktopServiceServer.forwardRustToTDP <-----------
//  desktopServiceServer.forwardRustToTDP <-----------
//  			 *fast path (screen) streaming continues...*
//
//              *user input messages*
//                                                         run_write_loop
//  desktopServiceServer.forwardTDPToRust ----------->  Session.Recv(Envelope)
//            *user input continues...*
//
//        *connection closed (client or server side)*
//
//  The wds <--> RDP connection is guaranteed to close when the Rust process
//  exits (on error, graceful disconnect, or SIGTERM or SIGINT is sent to the process).
//
//  The browser <--> wds connection is guaranteed to close when WindowsService.handleConnection
//  returns.

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	tdpbv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/legacy"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/tdpb"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

func init() {
	var rustLogLevel string

	// initialize the Rust logger by setting $RUST_LOG based
	// on the slog log level
	// (unless RUST_LOG is already explicitly set, then we
	// assume the user knows what they want)
	rl := os.Getenv("RUST_LOG")
	if rl == "" {
		ctx := context.Background()
		switch {
		case slog.Default().Enabled(ctx, logutils.TraceLevel):
			rustLogLevel = "trace"
		case slog.Default().Enabled(ctx, slog.LevelDebug):
			rustLogLevel = "debug"
		case slog.Default().Enabled(ctx, slog.LevelInfo):
			rustLogLevel = "info"
		case slog.Default().Enabled(ctx, slog.LevelWarn):
			rustLogLevel = "warn"
		default:
			rustLogLevel = "error"
		}

		// sspi-rs info-level logs are extremely verbose, so filter them out by default
		// TODO(zmb3): remove this after sspi-rs logging is cleaned up
		rustLogLevel += ",sspi=warn"

		// IronRDP instruments hot-path decode functions (e.g. RemoteFX process_frame) at INFO.
		// With no tracing subscriber installed, tracing's `log` feature bridges those span
		// records to env_logger as noisy non-JSON lines, so drop the span-lifecycle target.
		rustLogLevel += ",tracing::span=off"

		os.Setenv("RUST_LOG", rustLogLevel)
	}
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
	requestedScale                  uint16
	username                        string
	keyboardLayout                  uint32

	// conn handles TDP messages between Windows Desktop Service
	// and a Teleport Proxy.
	conn tdp.MessageReadWriteCloser

	certDER, keyDER []byte

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

// PrepareConnecton reads in handshake messages and optionally wraps the connection in a translation layer
// based on the client protocol.
func PrepareConnecton(clientProtocol string, conn *tdp.Conn, logger *slog.Logger) (tdp.MessageReadWriteCloser, *tdpb.ClientHello, error) {
	// Read Hello either from tdpb or tdp.
	if clientProtocol == tdpb.ProtocolName {
		hello, err := readClientHello(conn, logger)
		return conn, hello, trace.Wrap(err)
	}
	hello, err := readLegacyHandshake(conn, logger)
	// Translate to legacy tdp
	return tdp.NewReadWriteInterceptor(conn, tdpb.TranslateToModern, tdpb.TranslateToLegacy), hello, trace.Wrap(err)
}

// New creates and connects a new Client based on cfg.
func New(conn tdp.MessageReadWriteCloser, hello *tdpb.ClientHello, cfg Config) (*Client, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, err
	}
	c := &Client{
		cfg:           cfg,
		readyForInput: 0,
	}

	c.conn = conn
	c.requestedScale = uint16(hello.ScreenSpec.GetScale())
	if c.requestedScale == 0 {
		c.requestedScale = 100
	}

	c.username = hello.Username
	c.keyboardLayout = hello.KeyboardLayout

	if err := cfg.AuthorizeFn(hello.Username); err != nil {
		return nil, trace.Wrap(err)
	}

	return c, trace.Wrap(c.setClientSize(hello.ScreenSpec.GetWidth(), hello.ScreenSpec.GetHeight()))
}

// Run starts the RDP client, using the provided user certificate and private key.
// It blocks until the client disconnects.
func (c *Client) Run(ctx context.Context, certDER, keyDER []byte) error {
	c.certDER = certDER
	c.keyDER = keyDER

	// Create a private directory for the Unix socket to avoid other users on the system from accessing it.
	socketDir, err := os.MkdirTemp("", "rdp-ipc-*")
	if err != nil {
		return trace.Wrap(err)
	}
	defer os.RemoveAll(socketDir)

	if err := os.Chmod(socketDir, 0700); err != nil {
		return trace.Wrap(err)
	}

	socketPath := filepath.Join(socketDir, "rdp.sock")

	lis, err := net.Listen("unix", socketPath)
	if err != nil {
		return trace.Wrap(err)
	}

	grpcServer := grpc.NewServer()
	tdpbv1.RegisterDesktopServiceServer(grpcServer, &desktopServiceServer{client: c})

	grpcErrCh := make(chan error, 1)
	go func() {
		grpcErrCh <- grpcServer.Serve(lis)
	}()
	defer grpcServer.GracefulStop()

	clientID := rdpClientIDToUint32Array[uint32](newRDPClientID(c.cfg.HostID))

	args := []string{
		"--ipc-socket", socketPath,
		"--username", c.username,
		"--server-addr", c.cfg.Addr,
		"--screen-width", strconv.FormatUint(uint64(c.requestedWidth), 10),
		"--screen-height", strconv.FormatUint(uint64(c.requestedHeight), 10),
		"--screen-scale", strconv.FormatUint(uint64(c.requestedScale), 10),
		"--client-id", fmt.Sprintf("%d,%d,%d,%d", clientID[0], clientID[1], clientID[2], clientID[3]),
		"--keyboard-layout", strconv.FormatUint(uint64(c.keyboardLayout), 10),
	}

	if c.cfg.KDCAddr != "" {
		args = append(args, "--kdc-addr", c.cfg.KDCAddr)
	}
	if c.cfg.ComputerName != "" {
		args = append(args, "--computer-name", c.cfg.ComputerName)
	}
	if c.cfg.AD {
		args = append(args, "--ad")
	}
	if c.cfg.NLA {
		args = append(args, "--nla")
	}
	if c.cfg.AllowClipboard {
		args = append(args, "--allow-clipboard")
	}
	if c.cfg.AllowDirectorySharing {
		args = append(args, "--allow-directory-sharing")
	}
	if c.cfg.ShowDesktopWallpaper {
		args = append(args, "--show-desktop-wallpaper")
	}

	var stderrBuf bytes.Buffer

	cmd := exec.CommandContext(ctx, "./rdp-client", args...)
	// Rust RDP client writes logs to stdout, which we redirect to the configured log writer.
	cmd.Stdout = c.cfg.LogWriter
	// Stderr is used to report the disconnect or failure reason from the Rust RDP client.
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return trace.Wrap(err)
	}

	rustProcErrCh := make(chan error, 1)
	go func() {
		err := cmd.Wait()
		msg := strings.TrimSpace(stderrBuf.String())

		if err != nil {
			if msg != "" {
				err = fmt.Errorf("%s", msg)
			} else {
				err = fmt.Errorf("%w: RDP client exited with an unknown error", err)
			}

			c.sendTDBPAlert(err.Error(), tdpbv1.AlertSeverity_ALERT_SEVERITY_ERROR)
		} else {
			if msg == "" {
				msg = "RDP client exited gracefully"
			}

			c.cfg.Logger.InfoContext(ctx, msg)
			c.sendTDBPAlert(msg, tdpbv1.AlertSeverity_ALERT_SEVERITY_INFO)
		}

		rustProcErrCh <- err
	}()

	// Helper function to stop the Rust RDP process.
	stopRustProc := func() {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		select {
		case <-time.After(5 * time.Second):
			// If the process doesn't exit after 5 seconds, force kill it
			if err := cmd.Process.Kill(); err != nil {
                c.cfg.Logger.WarnContext(ctx, "failed to kill the RDP client process", "error", err)
            }
            <-rustProcErrCh
		case <-rustProcErrCh:
			// Process exited gracefully
		}
	}

	select {
	case err := <-rustProcErrCh:
		return trace.Wrap(err)
	case err := <-grpcErrCh:
		stopRustProc()
		return trace.Wrap(err)
	case <-ctx.Done():
		stopRustProc()
		return trace.Wrap(ctx.Err())
	}
}

func (c *Client) GetClientUsername() string {
	return c.username
}

// ReadClientScreenSpec reads the next message from the connection, expecting
// it to be a ClientScreenSpec. If it is not, an error is returned.
func ReadClientScreenSpec(conn *tdp.Conn) (*tdpbv1.ClientScreenSpec, error) {
	m, err := conn.ReadMessage()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	spec, ok := m.(legacy.ClientScreenSpec)
	if !ok {
		return nil, trace.BadParameter("expected ClientScreenSpec, got %T", m)
	}

	return &tdpbv1.ClientScreenSpec{Width: spec.Width, Height: spec.Height}, nil
}

// SendNotification is a convenience function for sending a Notification message.
func (c *Client) SendNotification(message string, severity legacy.Severity) error {
	return c.conn.WriteMessage(legacy.Alert{Message: message, Severity: severity})
}

func readClientUsername(conn *tdp.Conn, logger *slog.Logger) (string, error) {
	for {
		msg, err := conn.ReadMessage()
		if err != nil {
			return "", trace.Wrap(err)
		}
		u, ok := msg.(legacy.ClientUsername)
		if !ok {
			logger.DebugContext(context.Background(), "Received unexpected ClientUsername message", "message_type", logutils.TypeAttr(msg))
			continue
		}
		logger.DebugContext(context.Background(), "Got RDP username", "username", u.Username)
		return u.Username, nil
	}
}

func readClientSize(conn *tdp.Conn, logger *slog.Logger) (*tdpbv1.ClientScreenSpec, error) {
	for {
		s, err := ReadClientScreenSpec(conn)
		if err != nil {
			if trace.IsBadParameter(err) {
				logger.DebugContext(context.Background(), "Failed to read client screen spec", "error", err)
				continue
			} else {
				return nil, err
			}
		}
		return s, nil
	}
}

func readClientKeyboardLayout(conn *tdp.Conn, logger *slog.Logger) (uint32, error) {
	msgType, err := conn.PeekNextByte()
	if err != nil {
		return 0, trace.Wrap(err)
	}
	if legacy.MessageType(msgType) != legacy.TypeClientKeyboardLayout {
		logger.DebugContext(context.Background(), "Client did not send keyboard layout")
		return 0, nil
	}
	msg, err := conn.ReadMessage()
	if err != nil {
		return 0, trace.Wrap(err)
	}
	k, ok := msg.(legacy.ClientKeyboardLayout)
	if !ok {
		return 0, trace.BadParameter("Unexpected message %T", msg)
	}
	logger.DebugContext(context.Background(), "Got RDP keyboard layout", "keyboard_layout", k.KeyboardLayout)
	return k.KeyboardLayout, nil
}

func (c *Client) sendTDBPAlert(message string, severity tdpbv1.AlertSeverity) error {
	return c.conn.WriteMessage(&tdpb.Alert{Message: message, Severity: severity})
}

func readClientHello(conn *tdp.Conn, logger *slog.Logger) (*tdpb.ClientHello, error) {
	for {
		msg, err := conn.ReadMessage()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		m, ok := msg.(*tdpb.ClientHello)
		if !ok {
			// Most likely we received an early ping message
			if _, isPing := msg.(*tdpb.Ping); !isPing {
				logger.DebugContext(context.Background(), "Received unexpected message while waiting for client hello", "message_type", logutils.TypeAttr(msg))
			}
			continue
		}
		logger.DebugContext(context.Background(), "Got ClientHello", "message", m)
		return m, nil
	}
}

func readLegacyHandshake(conn *tdp.Conn, logger *slog.Logger) (*tdpb.ClientHello, error) {
	hello := &tdpb.ClientHello{}
	var err error

	if hello.Username, err = readClientUsername(conn, logger); err != nil {
		return nil, trace.Wrap(err)
	}

	if hello.ScreenSpec, err = readClientSize(conn, logger); err != nil {
		return nil, trace.Wrap(err)
	}

	if hello.KeyboardLayout, err = readClientKeyboardLayout(conn, logger); err != nil {
		return nil, trace.Wrap(err)
	}
	return hello, nil
}

func (c *Client) setClientSize(width uint32, height uint32) error {
	if c.cfg.hasSizeOverride() {
		// Some desktops have a screen size in their resource definition.
		// If non-zero then we always request this screen size.
		c.cfg.Logger.DebugContext(context.Background(), "Forcing a fixed screen size", "width", c.cfg.Width, "height", c.cfg.Height)
		c.requestedWidth = uint16(c.cfg.Width)
		c.requestedHeight = uint16(c.cfg.Height)
	} else {
		// The browser sends CSS pixel dimensions. Scale them by the display
		// scale factor (e.g. 200 for a 2x Retina display) to get the physical
		// pixel resolution that the RDP server should render at.
		w, h := applyScale(width, height, c.requestedScale)
		c.cfg.Logger.DebugContext(context.Background(), "Got RDP screen size", "css_width", width, "css_height", height, "scale", c.requestedScale, "width", w, "height", h)
		c.requestedWidth = uint16(w)
		c.requestedHeight = uint16(h)
	}

	if uint32(c.requestedWidth) > types.MaxRDPScreenWidth || uint32(c.requestedHeight) > types.MaxRDPScreenHeight {
		err := trace.BadParameter(
			"screen size of %d x %d is greater than the maximum allowed by RDP (%d x %d)",
			c.requestedWidth, c.requestedHeight, types.MaxRDPScreenWidth, types.MaxRDPScreenHeight,
		)
		return trace.Wrap(err)
	}

	return nil
}

// applyScale multiplies CSS pixel dimensions by a display scale factor percentage.
// For example, applyScale(1200, 800, 200) returns (2400, 1600).
// A scale of 100 or less returns the dimensions unchanged.
func applyScale(width, height uint32, scale uint16) (uint32, uint32) {
	if scale > 100 {
		width = width * uint32(scale) / 100
		height = height * uint32(scale) / 100
	}
	return width, height
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

// DisableNLA disables NLA in the client configuration.
func (c *Client) DisableNLA() {
	c.cfg.NLA = false
}
