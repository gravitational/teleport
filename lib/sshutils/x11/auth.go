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

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

const (
	// mitMagicCookieProto is the default xauth protocol used for x11 forwarding.
	mitMagicCookieProto = "MIT-MAGIC-COOKIE-1"
	// mitMagicCookieSize is the number of bytes in an mit magic cookie.
	mitMagicCookieSize = 16
)

// XAuthEntry is an entry in an XAuthority database which can be used to authenticate
// and authorize requests from an XServer to the associated X display.
type XAuthEntry struct {
	// Display is an X display in the format - [hostname]:[display_number].[screen_number]
	Display string `json:"display"`
	// Proto is an XAuthority protocol, generally "MIT-MAGIC-COOKIE-1"
	Proto string `json:"proto"`
	// Cookie is a hex encoded XAuthority cookie
	Cookie string `json:"cookie"`
}

// NewTrustedXauthEntry creates a new xauth entry with a trusted xauth cookie.
func NewTrustedXauthEntry(display string) (*XAuthEntry, error) {
	// For trusted x11 forwarding, we can use a fake cookie as it is only
	// used to validate the server-client connection. Locally, the client's
	// XServer will ignore the trusted cookie regardless of its origin and
	// use whatever authentication mechanisms it was going to use.
	cookie, err := newFakeCookie(mitMagicCookieSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &XAuthEntry{
		Display: display,
		Proto:   mitMagicCookieProto,
		Cookie:  cookie,
	}, nil
}

func newFakeCookie(byteLength int) (string, error) {
	cookieBytes := make([]byte, byteLength)
	if _, err := rand.Read(cookieBytes); err != nil {
		return "", trace.Wrap(err)
	}
	return hex.EncodeToString(cookieBytes), nil
}

// SpoofCookie creates a new random cookie with the same length as the entry's cookie.
// This is used to create a believable spoof of the client's xauth data to send to the server.
func (e *XAuthEntry) SpoofCookie() (string, error) {
	spoof, err := newFakeCookie(hex.DecodedLen(len(e.Cookie)))
	if err != nil {
		return "", trace.Wrap(err)
	}
	return spoof, nil
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
func (x *XAuthCommand) ReadEntry(display string) (*XAuthEntry, error) {
	x.Cmd.Args = append(x.Cmd.Args, "list", display)
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
	_, proto, cookie := splitEntry[0], splitEntry[1], splitEntry[2]

	return &XAuthEntry{
		Display: display,
		Proto:   proto,
		Cookie:  cookie,
	}, nil
}

// RemoveEntries runs "xauth remove" to remove any xauth entries for the given display.
func (x *XAuthCommand) RemoveEntries(display string) error {
	x.Cmd.Args = append(x.Cmd.Args, "remove", display)
	return trace.Wrap(x.run())
}

// AddEntry runs "xauth add" to add the given xauth entry.
func (x *XAuthCommand) AddEntry(entry *XAuthEntry) error {
	x.Cmd.Args = append(x.Cmd.Args, "add", entry.Display, entry.Proto, entry.Cookie)
	return trace.Wrap(x.run())
}

// GenerateUntrustedCookie runs "xauth generate untrusted" to create a new untrusted xauth
// entry for the given display. A timeout can optionally be set for the xauth entry.
//
// An untrusted cookie will signal to the XServer that fewer X privileges should be provided
// when opening local connections with this cookie. This prevents attackers from using the
// cookie to perform actions like keystroke monitoring.
func (x *XAuthCommand) GenerateUntrustedCookie(display, proto string, timeout uint) error {
	x.Cmd.Args = append(x.Cmd.Args, "generate", "untrusted", display, proto)
	if timeout != 0 {
		// Add some slack to the ttl to avoid XServer from denying
		// access to the ssh session during its lifetime.
		var timeoutSlack uint = 60
		x.Cmd.Args = append(x.Cmd.Args, "timeout", fmt.Sprint(timeout+timeoutSlack))
	}
	return trace.Wrap(x.run())
}

// run Run and wrap error with stderr.
func (x *XAuthCommand) run() error {
	err := x.Cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return trace.Wrap(err, "stderr: %q", exitErr.Stderr)
		}
	}
	return trace.Wrap(err)
}

// run Output and wrap error with stderr.
func (x *XAuthCommand) output() ([]byte, error) {
	out, err := x.Cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, trace.Wrap(err, "stderr: %q", exitErr.Stderr)
		}
	}
	return out, trace.Wrap(err)
}

// ReadAndRewriteXAuthPacket reads the initial xauth packet from the xserver request. The xauth packet has 2 parts:
//  1. fixed size buffer (12 bytes) - holds byteOrder bit, and the sizes of the protocol string and auth data
//  2. variable size xauth packet - holds xauth protocol and data used to connect to the remote XServer.
//
// Then it compares the received auth packet with the auth proto and fake cookie
// sent to the server with the original "x11-req". If the data matches, the auth
// packet is returned with the fake cookie replaced by the real cookie to provide
// access to the client's X display.
func ReadAndRewriteXAuthPacket(xchan ssh.Channel, authProto, fakeCookie, realCookie string) ([]byte, error) {
	// xauth packet starts with a fixed sized buffer of 12 bytes
	// which is used to size and decode the remaining bytes
	initBufSize := 12
	initBuf := make([]byte, initBufSize)
	if _, err := io.ReadFull(xchan, initBuf); err != nil {
		return nil, trace.Wrap(err, "x11 channel initial packet buffer missing or too short")
	}

	var protoLen, dataLen int
	switch byteOrder := initBuf[0]; byteOrder {
	///* Byte order MSB first. */
	case 0x42:
		protoLen = int(binary.BigEndian.Uint16(initBuf[6:8]))
		dataLen = int(binary.BigEndian.Uint16(initBuf[8:10]))
	///* Byte order LSB first. */
	case 0x6c:
		protoLen = int(binary.LittleEndian.Uint16(initBuf[6:8]))
		dataLen = int(binary.LittleEndian.Uint16(initBuf[8:10]))
	default:
		return nil, trace.Errorf("x11 channel auth packet has invalid byte order: ", byteOrder)
	}

	// authPacket size is equal to protoLen (rounded up by 4) + dataLen.
	// In openssh, the rounding is performed with: (protoLen + 3) & ~3
	authPacketSize := protoLen + (4-protoLen%4)%4 + dataLen
	authPacket := make([]byte, authPacketSize)
	if _, err := io.ReadFull(xchan, authPacket); err != nil {
		return nil, trace.Wrap(err, "x11 channel auth packet missing or too short")
	}

	proto := authPacket[:protoLen]
	data := authPacket[len(authPacket)-dataLen:]
	if string(proto) != authProto || hex.EncodeToString(data) != fakeCookie {
		return nil, trace.AccessDenied("x11 channel has the wrong authentication data")
	}

	realAuthData, err := hex.DecodeString(realCookie)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Replace auth data with the real auth data and write to conn
	for i := 0; i < len(data); i++ {
		data[i] = realAuthData[i]
	}

	return append(initBuf, authPacket...), trace.Wrap(err)
}
