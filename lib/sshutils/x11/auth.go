package x11

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

const (
	XAuthPathEnv = "XAUTHORITY"

	xauthDefaultProto     = "MIT-MAGIC-COOKIE-1"
	xauthDefaultProtoSize = 16
)

func runXAuthCommand(ctx context.Context, xauthFile string, args ...string) ([]byte, error) {
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

func generateUntrustedXAuthEntry(ctx context.Context, display string, timeout uint) (*xAuthEntry, error) {
	xauthDir, err := os.MkdirTemp(os.TempDir(), "tsh-*")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer os.RemoveAll(xauthDir)

	xauthFile, err := os.CreateTemp(xauthDir, "xauthfile")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := generateXAuthEntry(ctx, xauthFile.Name(), display, xauthDefaultProto, false, timeout); err != nil {
		return nil, trace.Wrap(err)
	}

	xAuthEntry, err := readXAuthEntry(ctx, xauthFile.Name(), display)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return xAuthEntry, nil
}

func readXAuthEntry(ctx context.Context, xauthFile, display string) (*xAuthEntry, error) {
	out, err := runXAuthCommand(ctx, xauthFile, "list", display)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Ignore entries beyond the first listed
	entry := bytes.Split(out, []byte("\n"))[0]
	xAuthEntry, err := parseXAuthEntry(entry)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return xAuthEntry, nil
}

func generateXAuthEntry(ctx context.Context, xauthFile, display, authProto string, trusted bool, timeout uint) error {
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

	out, err := runXAuthCommand(ctx, xauthFile, args...)
	if err != nil {
		log.Debug("error with xauth generate: ", string(out))
	}
	return trace.Wrap(err)
}

func addXAuthEntry(ctx context.Context, xauthFile, display, proto, cookie string) error {
	log.Debug("cookie: ", cookie)
	log.Debug("cookie_len: ", len(cookie))
	_, err := runXAuthCommand(ctx, xauthFile, "add", display, proto, cookie)
	return trace.Wrap(err)
}

func removeXAuthEntry(ctx context.Context, xauthFile, display string) error {
	_, err := runXAuthCommand(ctx, xauthFile, "remove", display)
	return trace.Wrap(err)
}

type xAuthEntry struct {
	display string
	proto   string
	cookie  string
}

// parseXAuthEntry parses and validates an xAuthEntry returned from "xauth list"
// which prints entries in the format - "hostname/display proto cookie"
func parseXAuthEntry(entry []byte) (*xAuthEntry, error) {
	splitEntry := strings.Split(string(entry), "  ")
	if len(splitEntry) != 3 {
		return nil, trace.BadParameter("invalid xAuthEntry, expected a single three-part entry")
	}

	display, proto, cookie := splitEntry[0], splitEntry[1], splitEntry[2]
	xauthEntry, err := newXAuthEntry(display, proto, cookie)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return xauthEntry, nil
}

func newXAuthEntry(display, proto, cookie string) (*xAuthEntry, error) {
	if err := validateXAuthEntryPart(display); err != nil {
		return nil, trace.Wrap(err, "%q is not a valid xauth display", display)
	}
	if err := validateXAuthEntryPart(proto); err != nil {
		return nil, trace.Wrap(err, "%q is not a valid xauth protocol", proto)
	}
	if err := validateXAuthCookie(cookie); err != nil {
		return nil, trace.Wrap(err, "%q is not a valid xauth cookie", cookie)
	}
	return &xAuthEntry{
		display: display,
		proto:   proto,
		cookie:  cookie,
	}, nil
}

func newFakeXAuthEntry(display string) (*xAuthEntry, error) {
	fakeCookie, err := newFakeCookie(xauthDefaultProtoSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	xAuthEntry, err := newXAuthEntry(display, xauthDefaultProto, fakeCookie)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return xAuthEntry, nil
}

// spoof creates a copy of the given xAuthEntry with a new random cookie of the same size and proto
func (e *xAuthEntry) spoof() (*xAuthEntry, error) {
	originalAuthData, err := hex.DecodeString(e.cookie)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	fakeCookie, err := newFakeCookie(len(originalAuthData))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	xAuthEntry, err := newXAuthEntry(string(e.display), e.proto, fakeCookie)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return xAuthEntry, nil
}

func newFakeCookie(numBytes int) (string, error) {
	cookieBytes := make([]byte, numBytes)
	if _, err := rand.Read(cookieBytes); err != nil {
		return "", trace.Wrap(err)
	}
	return hex.EncodeToString(cookieBytes), nil
}

func validateXAuthCookie(cookie string) error {
	if err := validateXAuthEntryPart(cookie); err != nil {
		return trace.Wrap(err)
	}
	if _, err := hex.DecodeString(cookie); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func validateXAuthEntryPart(str string) error {
	if str == "" {
		return trace.BadParameter("xauth entry part cannot be empty")
	}
	isValidSpecialChar := func(r rune) bool {
		return r == ':' || r == '/' || r == '.' || r == '-' || r == '_'
	}
	for _, c := range str {
		if !unicode.IsLetter(c) && !unicode.IsNumber(c) && !isValidSpecialChar(c) {
			return trace.BadParameter("invalid character %q", c)
		}
	}
	return nil
}

func xauthHomePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return filepath.Join(home, ".Xauthority"), nil
}
