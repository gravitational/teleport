/*
Copyright 2023 Gravitational, Inc.

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

package tcpip

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
)

const (
	// maxLengthPrefixedMessageSize is the upper limit of length prefixed messages that we will permit
	// encoding/decoding. Note that this value is actually much larger than required for the usecase that
	// these helpers were written to support.
	maxLengthPrefixedMessageSize = 1024 * 64

	// socketSuffix is the suffix applied to all sockets created/expected by this package.
	socketSuffix = ".sock"
)

// ListenerFromFD wraps net.FileListener and ensures that the old fd is
// closed as soon as it is no longer needed.
func ListenerFromFD(fd *os.File) (net.Listener, error) {
	listener, err := net.FileListener(fd)
	fd.Close()

	if l, ok := listener.(*net.UnixListener); ok {
		// child takes 'ownership' of the socket, meaning that we should
		// desroy it on close.
		l.SetUnlinkOnClose(true)
	}

	return listener, trace.Wrap(err)
}

// SetupListenerFD sets up a unix listener in the target directory. If the dir does not exist
// it is created with PrivateDirMode. Socket names are generated with the form '<uuid>.sock'.
// The returned FD is ready to be passed to a child for reconstruction with ListenerFromFD.
func SetupListenerFD(dir string) (name string, file *os.File, err error) {
	if err := os.MkdirAll(dir, teleport.PrivateDirMode); err != nil {
		return "", nil, trace.Errorf("failed to create socket dir %q: %v", dir, err)
	}

	sname := filepath.Join(dir, uuid.NewString()+socketSuffix)

	listener, err := net.ListenUnix("unix", &net.UnixAddr{
		Name: sname,
		Net:  "unix",
	})
	if err != nil {
		return "", nil, fmt.Errorf("failed to create socket listener %q: %v", sname, err)
	}

	// ensure that closing the listener does not destroy the socket.
	listener.SetUnlinkOnClose(false)
	defer listener.Close()

	fd, err := listener.File()

	return sname, fd, err
}

func WriteLengthPrefixedMessage(w io.Writer, msg []byte) error {
	if len(msg) > maxLengthPrefixedMessageSize {
		return fmt.Errorf("cannot write %d byte message (exceeds max message size %d)", len(msg), maxLengthPrefixedMessageSize)
	}

	var lb [4]byte
	binary.BigEndian.PutUint32(lb[:], uint32(len(msg)))

	_, err := w.Write(lb[:])
	if err != nil {
		return trace.Wrap(err)
	}

	if len(msg) == 0 {
		// nothing to write
		return nil
	}

	_, err = w.Write(msg[:])
	return trace.Wrap(err)
}

func ReadLengthPrefixedMessage(r io.Reader) ([]byte, error) {
	var lb [4]byte
	_, err := io.ReadFull(r, lb[:])
	if err != nil {
		if err == io.EOF {
			return nil, io.EOF
		}
		return nil, trace.Wrap(err)
	}

	msgLen := int(binary.BigEndian.Uint32(lb[:]))
	if msgLen > maxLengthPrefixedMessageSize {
		return nil, fmt.Errorf("cannot read %d byte message (exceeds max message size %d)", msgLen, maxLengthPrefixedMessageSize)
	}

	if msgLen == 0 {
		// nothing to read
		return nil, nil
	}

	buf := make([]byte, msgLen)

	_, err = io.ReadFull(r, buf[:])
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return buf, nil
}

// cleanSocks checks if the sockets in the target directory are dialable. Undialable sockets that
// are in 'prev' are deleted, otherwise they are added to 'next'. Each call to cleanSocks should
// be passed the output map from the previous run, which will have the effect of causing any
// socket that is undialable two runs in a row to be removed.
func cleanSocks(dir string, prev map[string]struct{}, iotimeout time.Duration) (next map[string]struct{}, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	next = make(map[string]struct{})

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), socketSuffix) {
			continue
		}

		spath := filepath.Join(dir, entry.Name())

		err = pingSocket(spath, iotimeout)
		if err == nil {
			continue
		}

		if _, ok := prev[entry.Name()]; !ok {
			// previous run did not mark this socket as unhealthy, mark
			// it for future closure and continue.
			next[entry.Name()] = struct{}{}
			continue
		}

		// socket appeared unhealthy twice in a row, remove it.
		err = os.Remove(filepath.Join(dir, entry.Name()))
		if err != nil {
			logrus.Warnf("Failed to clean up socket: %q", spath)
		}
	}

	return next, nil
}

var (
	controlMsgPrefix = []byte("@")
	pingMsg          = []byte("@ping")
	pongMsg          = []byte("@pong")
	keepaliveMsg     = []byte("@keepalive")
	keepaliveAck     = []byte("@ok")
	unknownMsg       = []byte("@unknown")
)

func pingSocket(spath string, iotimeout time.Duration) error {
	conn, err := net.DialTimeout("unix", spath, iotimeout)
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(iotimeout))

	if err := WriteLengthPrefixedMessage(conn, pingMsg); err != nil {
		return trace.Wrap(err)
	}

	rsp, err := ReadLengthPrefixedMessage(conn)
	if err != nil {
		return trace.Wrap(err)
	}

	if !bytes.Equal(rsp, pongMsg) {
		return trace.Errorf("unexpected ping response: %q", rsp)
	}

	return nil
}

func keepaliveSocket(spath string, iotimeout time.Duration) error {
	conn, err := net.DialTimeout("unix", spath, iotimeout)
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(iotimeout))

	if err := WriteLengthPrefixedMessage(conn, keepaliveMsg); err != nil {
		return trace.Wrap(err)
	}

	rsp, err := ReadLengthPrefixedMessage(conn)
	if err != nil {
		return trace.Wrap(err)
	}

	if !bytes.Equal(rsp, keepaliveAck) {
		return trace.Errorf("unexpected keepalive response: %q", rsp)
	}

	return nil
}
