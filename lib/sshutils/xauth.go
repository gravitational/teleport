package sshutils

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gravitational/trace"
)

const (
	xauthPathEnv        = "XAUTHORITY"
	xauthHomePath       = ".Xauthority"
	xauthLockSuffix     = "-c"
	xauthLockTTL        = time.Second * 5
	xauthFamilyHostname = 256
)

func addXAuthEntry(host string, display int, authProto string, authCookie string) error {
	newEntry, err := newXAuthEntry(host, display, authProto, authCookie)
	if err != nil {
		return trace.Wrap(err)
	}

	// Open existing xauth file and get existing entries
	xauthPath, err := getXAuthPath()
	if err != nil {
		return trace.Wrap(err)
	}

	xauthFile, err := os.OpenFile(xauthPath, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return trace.Wrap(err)
	}
	defer xauthFile.Close()

	existingEntries, err := readXAuthEntries(xauthFile)
	if err != nil {
		return trace.Wrap(err)
	}

	// Open a new xauth lock file to update xauth and write new and nonoverlapping entries to lock file
	xauthLockFile, err := openXAuthLock(xauthPath)
	if err != nil {
		return trace.Wrap(err)
	}
	defer os.Remove(xauthLockFile.Name())

	if err := writeXAuthEntry(xauthLockFile, newEntry); err != nil {
		return trace.Wrap(err)
	}

	for _, e := range existingEntries {
		if e.family != newEntry.family || !bytes.Equal(e.host, newEntry.host) || !bytes.Equal(e.display, newEntry.display) {
			if err := writeXAuthEntry(xauthLockFile, e); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	// Copy contents of the lock file to the xauth file
	if _, err := io.Copy(xauthFile, xauthLockFile); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func getXAuthPath() (string, error) {
	// if xauthPath := os.Getenv(xauthPathEnv); xauthPath != "" {
	// 	return xauthPath, nil
	// }
	home, err := os.UserHomeDir()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return filepath.Join(home, xauthHomePath), nil
}

func openXAuthLock(path string) (*os.File, error) {
	lockPath := path + xauthLockSuffix

	info, err := os.Stat(lockPath)
	if !os.IsNotExist(err) {
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// Check if the xauth lock file is currently locked
		if time.Now().After(info.ModTime().Add(xauthLockTTL)) {
			return nil, trace.AlreadyExists("xauth lock file already exists")
		}
	}

	file, err := os.Create(lockPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return file, nil
}

// xauthEntries are written directly in binary with the following structure
//
// family			- 2 bytes
// hostSize			- 2 bytes
// host				- {hostSize} bytes
// displaySize		- 2 bytes
// display			- {displaySize} bytes
// authProtoSize	- 2 bytes
// authProto		- {authProtoSize} bytes
// authDataSize		- 2 bytes
// authData			- {authDataSize} bytes
type xauthEntry struct {
	family    uint16
	host      []byte
	display   []byte
	authProto []byte
	authData  []byte
}

func newXAuthEntry(host string, display int, authProto string, authCookie string) (xauthEntry, error) {
	// xauth uses the os hostname for local entries
	if host[0] == '/' || host == "" || host == "unix" || host == "localhost" {
		var err error
		if host, err = os.Hostname(); err != nil {
			return xauthEntry{}, trace.Wrap(err)
		}
	}

	authData, err := hex.DecodeString(authCookie)
	if err != nil {
		return xauthEntry{}, trace.Wrap(err, "authCookie must be a hex encoded string")
	}

	e := xauthEntry{
		family:    xauthFamilyHostname,
		host:      []byte(host),
		display:   []byte(strconv.Itoa(display)),
		authProto: []byte(authProto),
		authData:  authData,
	}

	return e, nil
}

func readXAuthEntry(r io.Reader) (e xauthEntry, err error) {
	if e.family, err = readUint16(r); err != nil {
		return xauthEntry{}, trace.Wrap(err)
	} else if e.host, err = readNext(r); err != nil {
		return xauthEntry{}, trace.Wrap(err)
	} else if e.display, err = readNext(r); err != nil {
		return xauthEntry{}, trace.Wrap(err)
	} else if e.authProto, err = readNext(r); err != nil {
		return xauthEntry{}, trace.Wrap(err)
	} else if e.authData, err = readNext(r); err != nil {
		return xauthEntry{}, trace.Wrap(err)
	}
	return e, nil
}

func readXAuthEntries(r io.Reader) (entries []xauthEntry, err error) {
	for {
		entry, err := readXAuthEntry(r)
		if trace.IsEOF(err) {
			break
		} else if err != nil {
			return nil, trace.Wrap(err)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func readUint16(r io.Reader) (uint16, error) {
	var u uint16
	if err := binary.Read(r, binary.BigEndian, &u); err != nil {
		return 0, trace.Wrap(err)
	}
	return u, nil
}

func readBytes(r io.Reader, n int) ([]byte, error) {
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, trace.Wrap(err)
	}
	return buf, nil
}

// readNext reads the next size+data pair
func readNext(r io.Reader) ([]byte, error) {
	size, err := readUint16(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	bytes, err := readBytes(r, int(size))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return bytes, nil
}

func writeXAuthEntry(w io.Writer, e xauthEntry) error {
	if err := writeUint16(w, e.family); err != nil {
		return trace.Wrap(err)
	} else if err := writeNext(w, e.host); err != nil {
		return trace.Wrap(err)
	} else if err := writeNext(w, e.display); err != nil {
		return trace.Wrap(err)
	} else if err := writeNext(w, e.authProto); err != nil {
		return trace.Wrap(err)
	} else if err := writeNext(w, e.authData); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func writeUint16(w io.Writer, u uint16) error {
	return trace.Wrap(binary.Write(w, binary.BigEndian, u))
}

func writeBytes(w io.Writer, bytes []byte) error {
	return trace.Wrap(binary.Write(w, binary.BigEndian, bytes))
}

// writeNext writes the given str as a size+data pair
func writeNext(w io.Writer, bytes []byte) error {
	size := uint16(len(bytes))
	if err := binary.Write(w, binary.BigEndian, size); err != nil {
		return trace.Wrap(err)
	}
	if err := writeBytes(w, bytes); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
