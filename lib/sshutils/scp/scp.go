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

	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

const (
	// OKByte is scp OK message bytes
	OKByte = 0x0
	// WarnByte tells that next goes a warning string
	WarnByte = 0x1
	// ErrByte tells that next goes an error string
	ErrByte = 0x2
)

// Flags describes SCP command line flags
type Flags struct {
	// Source indicates upload mode
	Source bool
	// Sink indicates receive mode
	Sink bool
	//  Verbose sets a logging mode
	Verbose bool
	// Target sets targeted files to be transfered
	Target []string
	// Recursive indicates recursive file transfer
	Recursive bool
	// RemoteAddr is remote host address
	RemoteAddr string
	// LocalAddr is local host address
	LocalAddr string
}

// Config describes Command configuration settings
type Config struct {
	// Flags is SCP command line flags
	Flags Flags
	// User is a user who runs SCP command
	User *user.User
	// AuditLog is AuditLog log
	AuditLog events.IAuditLog
	// ProgressWriter is a writer to for printing the progress
	// (used only on the client for printing the progress)
	ProgressWriter io.Writer
	// FileSystem is a file system on which SCP command is ran
	FileSystem FileSystem
	// RemoteLocation is the file remote destination
	RemoteLocation string
}

// Command is an API that describes command operations
type Command interface {
	// Execute processes SCP traffic
	Execute(ch io.ReadWriter) error
	// GetRemoteShellCmd returns a remove shell command that
	// has to be executed on the remove server (handled by Teleport)
	GetRemoteShellCmd() (string, error)
}

// FileSystem is an API that describes file methods
type FileSystem interface {
	// IsDir tells if given path is a directory
	IsDir(path string) bool
	// GetFileInfo returns FileInfo for a given file path
	GetFileInfo(filePath string) (FileInfo, error)
	// MkDir creates a directory
	MkDir(path string, mode int) error
	// OpenFile opens a file and returns its Reader
	OpenFile(filePath string) (io.ReadCloser, error)
	// CreateFile creates a new file
	CreateFile(filePath string, length uint64) (io.WriteCloser, error)
	// SetChmod sets file permissions
	SetChmod(path string, mode int) error
}

// FileInfo is an API that describes methods that provide file information
type FileInfo interface {
	// IsDir tells if this is a directory
	IsDir() bool
	// ReadDir returns information of directory files
	ReadDir() ([]FileInfo, error)
	// GetName returns file name
	GetName() string
	// GetPath returns file path
	GetPath() string
	// GetModePerm returns file permissions
	GetModePerm() os.FileMode
	// GetSize returns file size
	GetSize() int64
}

// CreateDownloadCommand configures and returns a command used
// to download a file
func CreateDownloadCommand(config Config) (Command, error) {
	config.Flags.Sink = true
	config.Flags.Source = false
	cmd := CreateCommand(config)
	return cmd, nil
}

// CreateUploadCommand configures and returns a command used
// to upload a file
func CreateUploadCommand(config Config) (Command, error) {
	config.Flags.Sink = false
	config.Flags.Source = true
	cmd := CreateCommand(config)
	return cmd, nil
}

// CreateCommand creates and returns a new Command
func CreateCommand(params Config) Command {
	cmd := command{
		Config: params,
	}

	if params.FileSystem == nil {
		cmd.FileSystem = &localFileSystem{}
	}

	return &cmd
}

// Command mimics behavior of SCP command line tool
// to teleport can pretend it launches real scp behind the scenes
type command struct {
	Config
}

// Execute() implements SSH file copy (SCP). It is called on both tsh (client)
// and teleport (server) side.
func (cmd *command) Execute(ch io.ReadWriter) (err error) {
	if cmd.FileSystem == nil {
		return trace.BadParameter("missing file system")
	}

	if cmd.Flags.Source {
		err = cmd.serveSource(ch)
	} else {
		err = cmd.serveSink(ch)
	}
	if err != nil {
		if cmd.runningOnClient() {
			return trace.Wrap(err)
		} else {
			// when 'teleport scp' encounters an error, it SHOULD NOT be logged
			// to stderr (i.e. we should not return an error here) and instead
			// it should be sent back to scp client using scp protocol
			sendError(ch, err)
		}
	}
	return nil
}

func (cmd *command) runningOnClient() bool {
	return cmd.ProgressWriter != nil
}

func (cmd *command) GetRemoteShellCmd() (string, error) {
	if cmd.RemoteLocation == "" {
		return "", trace.BadParameter("missing remote file location")
	}

	// "impersonate" scp to a server
	shellCmd := "/usr/bin/scp -t"
	if cmd.Flags.Source == true {
		shellCmd = "/usr/bin/scp -t"
	} else {
		shellCmd = "/usr/bin/scp -f"
	}

	if cmd.Flags.Recursive {
		shellCmd += " -r"
	}
	shellCmd += (" " + cmd.RemoteLocation)

	return shellCmd, nil
}

func (cmd *command) serveSource(ch io.ReadWriter) error {
	fileInfoSlice := make([]FileInfo, len(cmd.Flags.Target))
	for i := range cmd.Flags.Target {
		fileInfo, err := cmd.FileSystem.GetFileInfo(cmd.Flags.Target[i])
		if err != nil {
			return trace.Wrap(err)
		}
		if fileInfo.IsDir() && !cmd.Flags.Recursive {
			err := trace.Errorf("%v is a directory, perhaps try -r flag?", fileInfo.GetName())
			return trace.Wrap(err)
		}
		fileInfoSlice[i] = fileInfo
	}

	r := newReader(ch)
	if err := r.read(); err != nil {
		return trace.Wrap(err)
	}

	for i := range fileInfoSlice {
		info := fileInfoSlice[i]
		if info.IsDir() {
			if err := cmd.sendDir(r, ch, info); err != nil {
				return trace.Wrap(err)
			}
		} else {
			if err := cmd.sendFile(r, ch, info); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	log.Debugf("send completed")
	return nil
}

func (cmd *command) sendDir(r *reader, ch io.ReadWriter, fileInfo FileInfo) error {
	out := fmt.Sprintf("D%04o 0 %s\n", fileInfo.GetModePerm(), fileInfo.GetName())
	log.Debugf("sendDir: %v", out)
	_, err := io.WriteString(ch, out)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := r.read(); err != nil {
		return trace.Wrap(err)
	}
	log.Debug("sendDir got OK")

	fileInfoSlice, err := fileInfo.ReadDir()
	if err != nil {
		return trace.Wrap(err)
	}

	for i := range fileInfoSlice {
		info := fileInfoSlice[i]
		if info.IsDir() {
			err := cmd.sendDir(r, ch, info)
			if err != nil {
				return trace.Wrap(err)
			}
		} else {
			err := cmd.sendFile(r, ch, info)
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

func (cmd *command) sendFile(r *reader, ch io.ReadWriter, fileInfo FileInfo) error {
	// log audit event:
	if cmd.AuditLog != nil {
		cmd.AuditLog.EmitAuditEvent(events.SCPEvent, events.EventFields{
			events.SCPPath:    fileInfo.GetPath(),
			events.SCPLengh:   fileInfo.GetSize(),
			events.LocalAddr:  cmd.Flags.LocalAddr,
			events.RemoteAddr: cmd.Flags.RemoteAddr,
			events.EventLogin: cmd.User.Username,
			events.SCPAction:  "read",
		})
	}

	reader, err := cmd.FileSystem.OpenFile(fileInfo.GetPath())
	if err != nil {
		return trace.Wrap(err)
	}

	defer reader.Close()

	out := fmt.Sprintf("C%04o %d %s\n", fileInfo.GetModePerm(), fileInfo.GetSize(), fileInfo.GetName())

	// report progress:
	if cmd.ProgressWriter != nil {
		defer fmt.Fprintf(cmd.ProgressWriter, "-> %s (%d)\n", fileInfo.GetPath(), fileInfo.GetSize())
	}

	_, err = io.WriteString(ch, out)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := r.read(); err != nil {
		return trace.Wrap(err)
	}

	n, err := io.Copy(ch, reader)
	if err != nil {
		return trace.Wrap(err)
	}
	if n != fileInfo.GetSize() {
		err := fmt.Errorf("short write: %v %v", n, fileInfo.GetSize())
		return trace.Wrap(err)
	}
	if err := sendOK(ch); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(r.read())
}

// serveSink executes file uploading, when a remote server sends file(s)
// via scp
func (cmd *command) serveSink(ch io.ReadWriter) error {
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
				return nil
			}
			return trace.Wrap(err)
		}

		if n < 1 {
			return trace.Errorf("unexpected error, read 0 bytes")
		}

		if b[0] == OKByte {
			continue
		}

		scanner.Scan()
		if err := scanner.Err(); err != nil {
			return trace.Wrap(err)
		}
		if err := cmd.processCommand(ch, &st, b[0], scanner.Text()); err != nil {
			return trace.Wrap(err)
		}
		if err := sendOK(ch); err != nil {
			return trace.Wrap(err)
		}
	}
}

func (cmd *command) processCommand(ch io.ReadWriter, st *state, b byte, line string) error {
	log.Debugf("[SCP] <- %v %v", string(b), line)
	switch b {
	case WarnByte:
		return trace.Errorf(line)
	case ErrByte:
		return trace.Errorf(line)
	case 'C':
		f, err := parseNewFile(line)
		if err != nil {
			return trace.Wrap(err)
		}
		err = cmd.receiveFile(st, *f, ch)
		if err != nil {
			return trace.Wrap(err)
		}
		return nil
	case 'D':
		d, err := parseNewFile(line)
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
		_, err := parseMtime(line)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return trace.Errorf("got unrecognized command: %v", string(b))
}

func (cmd *command) receiveFile(st *state, fc newFileCmd, ch io.ReadWriter) error {
	log.Debugf("scp.receiveFile(%v)", cmd.Flags.Target)

	// if the dest path is a folder, we should save the file to that folder, but
	// only if is 'recursive' is set

	path := cmd.Flags.Target[0]
	if cmd.Flags.Recursive || cmd.FileSystem.IsDir(path) {
		path = st.makePath(path, fc.Name)
	}

	writer, err := cmd.FileSystem.CreateFile(path, fc.Length)
	if err != nil {
		return trace.Wrap(err)
	}

	// report progress:
	if cmd.ProgressWriter != nil {
		defer fmt.Fprintf(cmd.ProgressWriter, "<- %s (%d)\n", path, fc.Length)
	}

	// log audit event:
	if cmd.AuditLog != nil {
		cmd.AuditLog.EmitAuditEvent(events.SCPEvent, events.EventFields{
			events.LocalAddr:  cmd.Flags.LocalAddr,
			events.RemoteAddr: cmd.Flags.RemoteAddr,
			events.EventLogin: cmd.User.Username,
			events.SCPPath:    path,
			events.SCPLengh:   fc.Length,
			events.SCPAction:  "write",
		})
	}

	defer writer.Close()

	if err = sendOK(ch); err != nil {
		return trace.Wrap(err)
	}

	n, err := io.CopyN(writer, ch, int64(fc.Length))
	if err != nil {
		log.Error(err)
		return trace.Wrap(err)
	}

	if n != int64(fc.Length) {
		return trace.Errorf("unexpected file copy length: %v", n)
	}

	if err := cmd.FileSystem.SetChmod(path, int(fc.Mode)); err != nil {
		return trace.Wrap(err)
	}

	log.Debugf("file %v(%v) copied to %v", fc.Name, fc.Length, path)
	return nil
}

func (cmd *command) receiveDir(st *state, fc newFileCmd, ch io.ReadWriter) error {
	targetDir := cmd.Flags.Target[0]

	// copying into an existing directory? append to it:
	if cmd.FileSystem.IsDir(targetDir) {
		targetDir = st.makePath(targetDir, fc.Name)
		st.push(fc.Name)
	}

	err := cmd.FileSystem.MkDir(targetDir, int(fc.Mode))
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

type newFileCmd struct {
	Mode   int64
	Length uint64
	Name   string
}

func parseNewFile(line string) (*newFileCmd, error) {
	log.Debugf("[SCP] parseNewFile(%v)", line)

	parts := strings.SplitN(line, " ", 3)
	if len(parts) != 3 {
		return nil, trace.Errorf("broken command")
	}
	c := newFileCmd{}

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

type mtimeCmd struct {
	Mtime time.Time
	Atime time.Time
}

func parseMtime(line string) (*mtimeCmd, error) {
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
	return &mtimeCmd{
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

// sendError gets called during all errors during SCP transmission.
// It writes it back to the SCP client
func sendError(ch io.ReadWriter, err error) error {
	if err == nil {
		return nil
	}
	log.Error(err)
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

// ParseSCPDestination takes a string representing a remote resource for SCP
// to download/upload, like "user@host:/path/to/resource.txt" and returns
// 3 components of it
func ParseSCPDestination(s string) (login, host, dest string) {
	parts := strings.SplitN(s, "@", 2)
	if len(parts) > 1 {
		login = parts[0]
		host = parts[1]
	} else {
		host = parts[0]
	}
	parts = strings.SplitN(host, ":", 2)
	if len(parts) > 1 {
		host = parts[0]
		dest = parts[1]
	}
	if len(dest) == 0 {
		dest = "."
	}
	return login, host, dest
}
