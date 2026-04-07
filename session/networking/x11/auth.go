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

package x11

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/gravitational/trace"
)

const (
	// mitMagicCookieProto is the default xauth protocol used for X11 forwarding.
	mitMagicCookieProto = "MIT-MAGIC-COOKIE-1"
	// mitMagicCookieSize is the number of bytes in an mit magic cookie.
	mitMagicCookieSize = 16
)

// XAuthEntry is an entry in an XAuthority database which can be used to authenticate
// and authorize requests from an XServer to the associated X display.
type XAuthEntry struct {
	// Display is an X display in the format - [hostname]:[display_number].[screen_number]
	Display Display `json:"display"`
	// Proto is an XAuthority protocol, generally "MIT-MAGIC-COOKIE-1"
	Proto string `json:"proto"`
	// Cookie is a hex encoded XAuthority cookie
	Cookie string `json:"cookie"`
}

// NewFakeXAuthEntry creates a fake xauth entry with a randomly generated MIT-MAGIC-COOKIE-1.
func NewFakeXAuthEntry(display Display) (*XAuthEntry, error) {
	cookie, err := newCookie(mitMagicCookieSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &XAuthEntry{
		Display: display,
		Proto:   mitMagicCookieProto,
		Cookie:  cookie,
	}, nil
}

// SpoofXAuthEntry creates a new xauth entry with a random cookie with the
// same length as the original entry's cookie. This is used to create a
// believable spoof of the client's xauth data to send to the server.
func (e *XAuthEntry) SpoofXAuthEntry() (*XAuthEntry, error) {
	spoofedCookie, err := newCookie(hex.DecodedLen(len(e.Cookie)))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &XAuthEntry{
		Display: e.Display,
		Proto:   e.Proto,
		Cookie:  spoofedCookie,
	}, nil
}

// newCookie makes a random hex-encoded cookie with the given byte length.
func newCookie(byteLength int) (string, error) {
	cookieBytes := make([]byte, byteLength)
	if _, err := rand.Read(cookieBytes); err != nil {
		return "", trace.Wrap(err)
	}
	return hex.EncodeToString(cookieBytes), nil
}

// XAuthCommand is a os/exec.Cmd wrapper for running xauth commands.
type XAuthCommand struct {
	*exec.Cmd
}

// NewXAuthCommand reate a new "xauth" command. xauthFile can be
// optionally provided to run the xauth command against a specific xauth file.
func NewXAuthCommand(ctx context.Context, xauthFile string) *XAuthCommand {
	var args []string
	if xauthFile != "" {
		args = []string{"-f", xauthFile}
	}
	return &XAuthCommand{exec.CommandContext(ctx, "xauth", args...)}
}

// ReadEntry runs "xauth list" to read the first xauth entry for the given display.
func (x *XAuthCommand) ReadEntry(display Display) (*XAuthEntry, error) {
	x.Cmd.Args = append(x.Cmd.Args, "list", display.String())
	out, err := x.output()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(out) == 0 {
		return nil, trace.NotFound("no xauth entry found")
	}

	// Ignore entries beyond the first listed.
	entry := strings.Split(string(out), "\n")[0]

	splitEntry := strings.Split(entry, "  ")
	if len(splitEntry) != 3 {
		return nil, trace.Errorf("invalid xAuthEntry, expected entry to have three parts")
	}
	proto, cookie := splitEntry[1], splitEntry[2]

	return &XAuthEntry{
		Display: display,
		Proto:   proto,
		Cookie:  cookie,
	}, nil
}

// RemoveEntries runs "xauth remove" to remove any xauth entries for the given display.
func (x *XAuthCommand) RemoveEntries(display Display) error {
	x.Cmd.Args = append(x.Cmd.Args, "remove", display.String())
	return trace.Wrap(x.run())
}

// AddEntry runs "xauth add" to add the given xauth entry.
func (x *XAuthCommand) AddEntry(entry XAuthEntry) error {
	x.Cmd.Args = append(x.Cmd.Args, "add", entry.Display.String(), entry.Proto, entry.Cookie)
	return trace.Wrap(x.run())
}

// GenerateUntrustedCookie runs "xauth generate untrusted" to create a new xauth entry with
// an untrusted MIT-MAGIC-COOKIE-1. A timeout can optionally be set for the xauth entry, after
// which the XServer will ignore this cookie.
func (x *XAuthCommand) GenerateUntrustedCookie(display Display, timeout time.Duration) error {
	x.Cmd.Args = append(x.Cmd.Args, "generate", display.String(), mitMagicCookieProto, "untrusted")
	x.Cmd.Args = append(x.Cmd.Args, "timeout", fmt.Sprint(timeout/time.Second))
	return trace.Wrap(x.run())
}

// run the command and return stderr if there is an error.
func (x *XAuthCommand) run() error {
	_, err := x.output()
	return trace.Wrap(err)
}

// run the command and return stdout or stderr if there is an error.
func (x *XAuthCommand) output() ([]byte, error) {
	stdout, err := x.Cmd.StdoutPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	stderr, err := x.Cmd.StderrPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := x.Cmd.Start(); err != nil {
		return nil, trace.Wrap(err)
	}

	// We add a conservative peak length of 10 KB to prevent potential
	// output spam from the client provided `xauth` binary
	var peakLength int64 = 10000
	out, err := io.ReadAll(io.LimitReader(stdout, peakLength))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	errOut, err := io.ReadAll(io.LimitReader(stderr, peakLength))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := x.Wait(); err != nil {
		return nil, trace.Wrap(err, "command \"%s\" failed with stderr: \"%s\"", strings.Join(x.Cmd.Args, " "), errOut)
	}

	return out, nil
}

// CheckXAuthPath checks if xauth is runnable in the current environment.
func CheckXAuthPath() error {
	_, err := exec.LookPath("xauth")
	return trace.Wrap(err)
}

// ReadAndRewriteXAuthPacket reads the initial xauth packet from an XServer request. The xauth packet has 2 parts:
//  1. fixed size buffer (12 bytes) - holds byteOrder bit, and the sizes of the protocol string and auth data
//  2. variable size xauth packet - holds xauth protocol and data used to connect to the remote XServer.
//
// Then it compares the received auth packet with the auth proto and fake cookie
// sent to the server with the original "x11-req". If the data matches, the auth
// packet is returned with the fake cookie replaced by the real cookie to provide
// access to the client's X display.
func ReadAndRewriteXAuthPacket(xreq io.Reader, spoofedXAuthEntry, realXAuthEntry *XAuthEntry) ([]byte, error) {
	if spoofedXAuthEntry.Proto != realXAuthEntry.Proto || len(spoofedXAuthEntry.Cookie) != len(realXAuthEntry.Cookie) {
		return nil, trace.BadParameter("spoofed and real xauth entries must use the same xauth protocol")
	}

	// xauth packet starts with a fixed sized buffer of 12 bytes
	// which is used to size and decode the remaining bytes
	initBuf := make([]byte, xauthPacketInitBufSize)
	if _, err := io.ReadFull(xreq, initBuf); err != nil {
		return nil, trace.Wrap(err, "X11 channel initial packet buffer missing or too short")
	}

	protoLen, dataLen, err := readXauthPacketInitBuf(initBuf)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// authPacket size is equal to protoLen (rounded up by 4) + dataLen.
	// In openssh, the rounding is performed with: (protoLen + 3) & ~3
	authPacketSize := protoLen + (4-protoLen%4)%4 + dataLen
	authPacket := make([]byte, authPacketSize)
	if _, err := io.ReadFull(xreq, authPacket); err != nil {
		return nil, trace.Wrap(err, "X11 channel auth packet missing or too short")
	}

	proto := authPacket[:protoLen]
	authData := authPacket[len(authPacket)-dataLen:]
	if string(proto) != spoofedXAuthEntry.Proto || hex.EncodeToString(authData) != spoofedXAuthEntry.Cookie {
		return nil, trace.AccessDenied("X11 channel has the wrong authentication data")
	}

	// Replace auth data with the real auth data
	realAuthData, err := hex.DecodeString(realXAuthEntry.Cookie)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	copy(authData, realAuthData)

	return append(initBuf, authPacket...), trace.Wrap(err)
}

const (
	// xauthPacketInitBufSize is the size of the initial
	// fixed portion of an xauth packet
	xauthPacketInitBufSize = 12
	// little endian byte order
	littleEndian = 'l'
	// big endian byte order
	bigEndian = 'B'
)

// readXauthPacketInitBuf reads the initial fixed size portion of
// an xauth packet to get the length of the auth proto and auth data
// portions of the xauth packet.
func readXauthPacketInitBuf(initBuf []byte) (protoLen int, dataLen int, err error) {
	// The first byte in the packet determines the
	// byte order of the initial buffer's bytes.
	var e binary.ByteOrder

	switch initBuf[0] {
	case bigEndian:
		e = binary.BigEndian
	case littleEndian:
		e = binary.LittleEndian
	default:
		return 0, 0, trace.BadParameter("X11 channel auth packet has invalid byte order: %v", initBuf[0])
	}

	// bytes 6-7 and 8-9 are used to determine the length of
	// the auth proto and auth data fields respectively.
	protoLen = int(e.Uint16(initBuf[6:8]))
	dataLen = int(e.Uint16(initBuf[8:10]))
	return protoLen, dataLen, nil
}
