package x11

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
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
	Display string
	// Proto is an XAuthority protocol, generally "MIT-MAGIC-COOKIE-1"
	Proto string
	// Cookie is a hex encoded XAuthority cookie
	Cookie string
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

// GetXauthEntry retrieves an existing xauth entry for the given display.
func GetXauthEntry(ctx context.Context, display string) (*XAuthEntry, error) {
	xauthEntry, err := xauthRead(ctx, "", display)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return xauthEntry, nil
}

// CreateUntrustedXauthEntry creates a new xauth entry for the given display.
// This xauth entry will not be injected into the user's actual XAuthority.
func CreateXauthEntry(ctx context.Context, display string, trusted bool, timeout uint) (*XAuthEntry, error) {
	if trusted {
		// the client's local XAuthority will treat this random
		// cookie the the same as one generated with xauth.
		fakeCookie, err := newFakeCookie(mitMagicCookieSize)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &XAuthEntry{
			Display: display,
			Proto:   mitMagicCookieProto,
			Cookie:  fakeCookie,
		}, nil
	}

	// If an untrusted cookie was requested, we must use xauth to generate an untrusted cookie.
	// This cookie will be provide fewer X privileges to prevent attackers from using the cookie to
	// perform actions like keystroke monitoring. This entry will be generated in a temp file to
	// prevent it from living outside of the context of this request.
	xauthDir, err := os.MkdirTemp(os.TempDir(), "tsh-*")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer os.RemoveAll(xauthDir)

	xauthFile, err := os.CreateTemp(xauthDir, "xauthfile")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := xauthGenerate(ctx, xauthFile.Name(), display, mitMagicCookieProto, false, timeout); err != nil {
		return nil, trace.Wrap(err)
	}

	xauthEntry, err := xauthRead(ctx, xauthFile.Name(), display)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return xauthEntry, nil
}

// Add the given auth proto and cookie as a new xauth entry for the given display.
func UpdateXAuthEntry(ctx context.Context, display, proto, cookie string) error {
	// Add the given auth cookie to the users home Xauthority - ~/.Xauthority.
	xauthFile, err := xauthHomePath()
	if err != nil {
		return trace.Wrap(err)
	}
	if err := xauthRemove(ctx, xauthFile, display); err != nil {
		return trace.Wrap(err)
	}
	if err := xauthAdd(ctx, xauthFile, display, proto, cookie); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// newFakeCookie returns a random hex cookie with the given number of bytes.
func newFakeCookie(numBytes int) (string, error) {
	cookieBytes := make([]byte, numBytes)
	if _, err := rand.Read(cookieBytes); err != nil {
		return "", trace.Wrap(err)
	}
	return hex.EncodeToString(cookieBytes), nil
}

// xauthRun runs xauth with the given arguments. If an xauthFile
// is provided, the xauth command will be run against that xauthFile
// rather than the defaults ($XAUTHORITY or ~/.Xauthority).
func xauthRun(ctx context.Context, xauthFile string, args ...string) ([]byte, error) {
	if len(args) == 0 {
		return nil, trace.BadParameter("xauth command expects at least one argument")
	}

	var xauthFileArgs []string
	if xauthFile != "" {
		xauthFileArgs = []string{"-f", xauthFile}
	}

	cmdArgs := append(xauthFileArgs, args...)
	cmd := exec.CommandContext(ctx, "xauth", cmdArgs...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, trace.Wrap(err, "xauth command %q failed with stderr: %q", args[0], exitErr.Stderr)
		}
		return nil, trace.Wrap(err, "xauth command %q failed", args[0])
	}
	return out, nil
}

// xauthRead runs "xauth list <display>" to retrieve existing
// xauth entries for the given display. Only the first listed
// xauth entry is returned, or a not found error if the result is empty.
func xauthRead(ctx context.Context, xauthFile, display string) (*XAuthEntry, error) {
	out, err := xauthRun(ctx, xauthFile, "list", display)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(out) == 0 {
		return nil, trace.NotFound("no xauth entry for the given display")
	}

	// Ignore entries beyond the first listed.
	entry := strings.Split(string(out), "\n")[0]

	splitEntry := strings.Split(entry, "  ")
	if len(splitEntry) != 3 {
		return nil, trace.BadParameter("invalid xAuthEntry, expected entry to have three parts")
	}
	_, proto, cookie := splitEntry[0], splitEntry[1], splitEntry[2]

	return &XAuthEntry{
		Display: display,
		Proto:   proto,
		Cookie:  cookie,
	}, nil
}

// xauthGenerate runs "xauth generate" to create a new xauth
// entry for the given display. trusted and timeout are optional
// arguments which will be applied to the generated entry.
func xauthGenerate(ctx context.Context, xauthFile, display, authProto string, trusted bool, timeout uint) error {
	args := []string{"generate", display, authProto}
	if !trusted {
		args = append(args, "untrusted")
	}

	if timeout != 0 {
		// Add some slack to the ttl to avoid XServer from denying
		// access to the ssh session during its lifetime.
		var timeoutSlack uint = 60
		args = append(args, "timeout", fmt.Sprint(timeout+timeoutSlack))
	}

	out, err := xauthRun(ctx, xauthFile, args...)
	if err != nil {
		log.Debug("error with xauth generate: ", string(out))
	}
	return trace.Wrap(err)
}

// xauthAdd runs "xauth add" to add a new xauth entry
// for the given protocol, cookie, and display.
func xauthAdd(ctx context.Context, xauthFile, display, proto, cookie string) error {
	_, err := xauthRun(ctx, xauthFile, "add", display, proto, cookie)
	return trace.Wrap(err)
}

// addXAuthEntry runs "xauth remove" to remove any
// xauth entries for the given display.
func xauthRemove(ctx context.Context, xauthFile, display string) error {
	_, err := xauthRun(ctx, xauthFile, "remove", display)
	return trace.Wrap(err)
}

// xauthHomePath returns the user's default Xauthority - ~/.Xauthority.
func xauthHomePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return filepath.Join(home, ".Xauthority"), nil
}
