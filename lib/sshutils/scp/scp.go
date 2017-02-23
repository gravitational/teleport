/*
Copyright 2015 Gravitational, Inc.

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

// Package scp handles file uploads and downloads via scp command
package scp

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

const (
	// OKByte is scp OK message bytes
	OKByte = 0x0
	// WarnByte tells that next goes a warning string
	WarnByte = 0x1
	// ErrByte tells that next goes an error string
	ErrByte = 0x2
)

// Command mimics behavior of SCP command line tool
// to teleport can pretend it launches real scp behind the scenes
type Command struct {
	Source     bool // data producer
	Sink       bool // data consumer
	Verbose    bool // verbose
	Target     string
	Recursive  bool
	User       *user.User
	AuditLog   events.IAuditLog
	RemoteAddr string
	LocalAddr  string

	// terminal is only initialized on the client, for printing the progress
	Terminal io.Writer
}

// Execute implements SSH file copy (SCP)
func (cmd *Command) Execute(ch io.ReadWriter) (err error) {
	if cmd.Source {
		// download
		err = cmd.serveSource(ch)
	} else {
		// upload
		err = cmd.serveSink(ch)
	}
	return trace.Wrap(err)
}

func (cmd *Command) serveSource(ch io.ReadWriter) error {
	log.Debug("SCP: serving source")

	paths, err := filepath.Glob(cmd.Target)
	if err != nil {
		return trace.Wrap(err)
	}
	files := make([]os.FileInfo, len(paths))
	for i := range paths {
		f, err := os.Stat(paths[i])
		if err != nil {
			return trace.Wrap(sendError(ch, err))
		}
		if f.IsDir() && !cmd.Recursive {
			err := trace.Errorf("%v is a directory, perhaps try -r flag?", f.Name())
			return trace.Wrap(sendError(ch, err))
		}
		files[i] = f
	}

	r := newReader(ch)
	if err := r.read(); err != nil {
		return trace.Wrap(err)
	}

	for i, f := range files {
		if f.IsDir() {
			if err := cmd.sendDir(r, ch, f, paths[i]); err != nil {
				return trace.Wrap(sendError(ch, err))
			}
		} else {
			if err := cmd.sendFile(r, ch, f, paths[i]); err != nil {
				return trace.Wrap(sendError(ch, err))
			}
		}
	}

	log.Debugf("send completed")
	return nil
}

func (cmd *Command) sendDir(r *reader, ch io.ReadWriter, fi os.FileInfo, path string) error {
	out := fmt.Sprintf("D%04o 0 %s\n", fi.Mode()&os.ModePerm, fi.Name())
	log.Debugf("sendDir: %v", out)
	_, err := io.WriteString(ch, out)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := r.read(); err != nil {
		return trace.Wrap(err)
	}
	log.Debug("sendDir got OK")
	f, err := os.Open(path)
	if err != nil {
		return trace.Wrap(err)
	}
	fis, err := f.Readdir(0)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, sfi := range fis {
		if sfi.IsDir() {
			err := cmd.sendDir(r, ch, sfi, filepath.Join(path, sfi.Name()))
			if err != nil {
				return trace.Wrap(err)
			}
		} else {
			err := cmd.sendFile(r, ch, sfi, filepath.Join(path, sfi.Name()))
			if err != nil {
				return trace.Wrap(err)
			}
		}
	}
	if _, err = fmt.Fprintf(ch, "E\n"); err != nil {
		return trace.Wrap(err)
	}
	return r.read()
}

func (cmd *Command) sendFile(r *reader, ch io.ReadWriter, fi os.FileInfo, path string) error {
	// log audit event:
	if cmd.AuditLog != nil {
		cmd.AuditLog.EmitAuditEvent(events.SCPEvent, events.EventFields{
			events.SCPPath:    path,
			events.SCPLengh:   fi.Size(),
			events.LocalAddr:  cmd.LocalAddr,
			events.RemoteAddr: cmd.RemoteAddr,
			events.EventLogin: cmd.User.Username,
			events.SCPAction:  "read",
		})
	}
	out := fmt.Sprintf("C%04o %d %s\n", fi.Mode()&os.ModePerm, fi.Size(), fi.Name())
	log.Debugf("sendFile: %v", out)

	// report progress:
	if cmd.Terminal != nil {
		defer fmt.Fprintf(cmd.Terminal, "-> %s (%d)\n", path, fi.Size())
	}

	_, err := io.WriteString(ch, out)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := r.read(); err != nil {
		return trace.Wrap(err)
	}

	f, err := os.Open(path)
	if err != nil {
		return trace.Wrap(err)
	}
	defer f.Close()
	n, err := io.Copy(ch, f)
	if err != nil {
		return trace.Wrap(err)
	}
	if n != fi.Size() {
		err := fmt.Errorf("short write: %v %v", n, fi.Size())
		return trace.Wrap(err)
	}
	if err := sendOK(ch); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(r.read())
}

// serveSink executes file uploading, when a remote server sends file(s)
// via scp
func (cmd *Command) serveSink(ch io.ReadWriter) error {
	log.Debug("SCP: serving sink")

	if err := sendOK(ch); err != nil {
		return trace.Wrap(err)
	}
	var st state
	st.path = []string{"."}
	var b = make([]byte, 1)
	scanner := bufio.NewScanner(ch)
	for {
		n, err := ch.Read(b)
		if err != nil {
			if err == io.EOF {
				//log.Debug("<- EOF")
				return nil
			}
			return trace.Wrap(err)
		}
		if n < 1 {
			return trace.Errorf("unexpected error, read 0 bytes")
		}

		if b[0] == OKByte {
			//log.Debug("<- OK")
			continue
		}

		scanner.Scan()
		if err := scanner.Err(); err != nil {
			return trace.Wrap(err)
		}
		if err := cmd.processCommand(ch, &st, b[0], scanner.Text()); err != nil {
			return sendError(ch, err)
		}
		if err := sendOK(ch); err != nil {
			return trace.Wrap(err)
		}
		//log.Debug("-> OK")
	}
}

func (cmd *Command) processCommand(ch io.ReadWriter, st *state, b byte, line string) error {
	//log.Debugf("<- %v %v", string(b), line)
	switch b {
	case WarnByte:
		return trace.Errorf(line)
	case ErrByte:
		return trace.Errorf(line)
	case 'C':
		f, err := ParseNewFile(line)
		if err != nil {
			return trace.Wrap(err)
		}
		err = cmd.receiveFile(st, *f, ch)
		if err != nil {
			return trace.Wrap(err)
		}
		return nil
	case 'D':
		d, err := ParseNewFile(line)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := cmd.receiveDir(st, *d, ch); err != nil {
			return trace.Wrap(err)
		}
		return nil
	case 'E':
		return st.pop()
	case 'T':
		_, err := ParseMtime(line)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return trace.Errorf("got unrecognized command: %v", string(b))
}

func (cmd *Command) receiveFile(st *state, fc NewFileCmd, ch io.ReadWriter) error {
	//log.Debugf("scp.receiveFile(%v)", cmd.Target)

	// if the dest path is a folder, we should save the file to that folder, but
	// only if is 'recursive' is set
	path := cmd.Target
	if cmd.Recursive || utils.IsDir(path) {
		path = st.makePath(path, fc.Name)
	}
	f, err := os.Create(path)
	if err != nil {
		return trace.Wrap(err)
	}

	// report progress:
	if cmd.Terminal != nil {
		defer fmt.Fprintf(cmd.Terminal, "<- %s (%d)\n", path, fc.Length)
	}

	// log audit event:
	if cmd.AuditLog != nil {
		cmd.AuditLog.EmitAuditEvent(events.SCPEvent, events.EventFields{
			events.LocalAddr:  cmd.LocalAddr,
			events.RemoteAddr: cmd.RemoteAddr,
			events.EventLogin: cmd.User.Username,
			events.SCPPath:    path,
			events.SCPLengh:   fc.Length,
			events.SCPAction:  "write",
		})
	}

	defer f.Close()

	if err = sendOK(ch); err != nil {
		return trace.Wrap(err)
	}

	n, err := io.CopyN(f, ch, int64(fc.Length))
	if err != nil {
		log.Error(err)
		return trace.Wrap(err)
	}

	if n != int64(fc.Length) {
		return trace.Errorf("unexpected file copy length: %v", n)
	}
	mode := os.FileMode(int(fc.Mode) & int(os.ModePerm))
	if err := os.Chmod(path, mode); err != nil {
		return trace.Wrap(err)
	}
	log.Debugf("file %v(%v) copied to %v", fc.Name, fc.Length, path)
	return nil
}

func (cmd *Command) receiveDir(st *state, fc NewFileCmd, ch io.ReadWriter) error {
	isRoot := len(st.path) == 1 && st.path[0] == "."

	log.Debugf("----> receiveDir(cmd.Target=%v, st.path=%v, fc.Name=%v). isRoot=%v",
		cmd.Target, st.path, fc.Name, isRoot)

	targetDir := cmd.Target

	// copying into an exising directory? append to it:
	if utils.IsDir(targetDir) {
		targetDir = st.makePath(targetDir, fc.Name)
		st.push(fc.Name)
	}

	mode := os.FileMode(int(fc.Mode) & int(os.ModePerm))
	err := os.MkdirAll(targetDir, mode)
	if err != nil && !os.IsExist(err) {
		return trace.Wrap(err)
	}
	return nil
}

type NewFileCmd struct {
	Mode   int64
	Length uint64
	Name   string
}

func ParseNewFile(line string) (*NewFileCmd, error) {
	log.Debugf("ParseNewFile(%v)", line)
	parts := strings.SplitN(line, " ", 3)
	if len(parts) != 3 {
		return nil, trace.Errorf("broken command")
	}
	c := NewFileCmd{}

	var err error
	if c.Mode, err = strconv.ParseInt(parts[0], 8, 32); err != nil {
		return nil, trace.Wrap(err)
	}
	if c.Length, err = strconv.ParseUint(parts[1], 10, 64); err != nil {
		return nil, trace.Wrap(err)
	}
	c.Name = parts[2]
	return &c, nil
}

type MtimeCmd struct {
	Mtime time.Time
	Atime time.Time
}

func ParseMtime(line string) (*MtimeCmd, error) {
	parts := strings.SplitN(line, " ", 4)
	if len(parts) != 4 {
		return nil, trace.Errorf("broken mtime command")
	}
	var err error
	vars := make([]int64, 4)
	for i := range vars {
		if vars[i], err = strconv.ParseInt(parts[i], 10, 64); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return &MtimeCmd{
		Mtime: time.Unix(vars[0], vars[1]),
		Atime: time.Unix(vars[2], vars[3]),
	}, nil
}

func sendOK(ch io.ReadWriter) error {
	_, err := ch.Write([]byte{OKByte})
	if err != nil {
		return trace.Wrap(err)
	} else {
		return nil
	}
}

// sendError gets called during all errors during SCP transmission. It does
// logs the error into Teleport log and also writes it back to the SCP client
func sendError(ch io.ReadWriter, err error) error {
	log.Error(err)
	if err == nil {
		return nil
	}
	message := err.Error()
	bytes := make([]byte, 0, len(message)+2)
	bytes = append(bytes, ErrByte)
	bytes = append(bytes, message...)
	bytes = append(bytes, []byte{'\n'}...)
	_, writeErr := ch.Write(bytes)
	if writeErr != nil {
		log.Error(writeErr)
	}
	return trace.Wrap(err)
}

type state struct {
	path     []string
	finished bool
}

func (st *state) push(dir string) {
	st.path = append(st.path, dir)
}

func (st *state) pop() error {
	if st.finished {
		return trace.Errorf("empty path")
	}
	if len(st.path) == 0 {
		st.finished = true // allow extra 'E' command in the end
		return nil
	}
	st.path = st.path[:len(st.path)-1]
	return nil
}

func (st *state) makePath(target, filename string) string {
	return filepath.Join(target, filepath.Join(st.path...), filename)
}

func newReader(r io.Reader) *reader {
	return &reader{
		b: make([]byte, 1),
		s: bufio.NewScanner(r),
		r: r,
	}
}

type reader struct {
	b []byte
	s *bufio.Scanner
	r io.Reader
}

// read is used to "ask" for response messages after each SCP transmission
// it only reads text data until a newline and returns 'nil' for "OK" responses
// and errors for everything else
func (r *reader) read() error {
	n, err := r.r.Read(r.b)
	if err != nil {
		return trace.Wrap(err)
	}
	if n < 1 {
		return trace.Errorf("unexpected error, read 0 bytes")
	}

	switch r.b[0] {
	case OKByte:
		log.Debug("<- OK")
		return nil
	case WarnByte, ErrByte:
		r.s.Scan()
		if err := r.s.Err(); err != nil {
			return trace.Wrap(err)
		}
		return trace.Errorf(r.s.Text())
	}
	return trace.Errorf("unrecognized command: %#v", r.b)
}
