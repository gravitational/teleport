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

// Package scp handles file uploads and downloads via SCP command.
// See https://web.archive.org/web/20170215184048/https://blogs.oracle.com/janp/entry/how_the_scp_protocol_works
// for the high-level protocol overview.
//
// Authoritative source for the protocol is the source code for OpenSSH scp:
// https://github.com/openssh/openssh-portable/blob/add926dd1bbe3c4db06e27cab8ab0f9a3d00a0c2/scp.c
package scp

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// OKByte is SCP OK message bytes
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
	// Verbose sets a logging mode
	Verbose bool
	// Target sets targeted files to be transferred
	Target []string
	// Recursive indicates recursive file transfer
	Recursive bool
	// RemoteAddr is a remote host address
	RemoteAddr string
	// LocalAddr is a local host address
	LocalAddr string
	// DirectoryMode indicates that a directory is being sent.
	DirectoryMode bool
	// PreserveAttrs preserves access and modification times
	// from the original file
	PreserveAttrs bool
}

// Config describes Command configuration settings
type Config struct {
	// Flags is a set of SCP command line flags
	Flags Flags
	// User is a user who runs SCP command
	User string
	// AuditLog is AuditLog log
	AuditLog events.AuditLogSessionStreamer
	// ProgressWriter is a writer for printing the progress
	// (used only on the client)
	ProgressWriter io.Writer
	// FileSystem is a source file system abstraction for the SCP command
	FileSystem FileSystem
	// RemoteLocation is a destination location of the file
	RemoteLocation string
	// RunOnServer is low level API flag that indicates that
	// this command will be run on the server
	RunOnServer bool
	// Log optionally specifies the logger
	Log *slog.Logger
}

// Command is an API that describes command operations
type Command interface {
	// Execute processes SCP traffic
	Execute(ch io.ReadWriter) error
	// GetRemoteShellCmd returns a remote shell command that
	// has to be executed on the remove server (handled by Teleport)
	GetRemoteShellCmd() (string, error)
}

// FileSystem is an interface that abstracts file system methods used in SCP command functions
type FileSystem interface {
	// IsDir returns true if a given file path is a directory
	IsDir(path string) bool
	// GetFileInfo returns FileInfo for a given file path
	GetFileInfo(filePath string) (FileInfo, error)
	// MkDir creates a directory
	MkDir(path string, mode int) error
	// OpenFile opens a file and returns its Reader
	OpenFile(filePath string) (io.ReadCloser, error)
	// CreateFile creates a new file
	CreateFile(filePath string, length uint64) (io.WriteCloser, error)
	// Chmod sets file permissions
	Chmod(path string, mode int) error
	// Chtimes sets file access and modification time
	Chtimes(path string, atime, mtime time.Time) error
}

// FileInfo provides access to file metadata
type FileInfo interface {
	// IsDir returns true if a file is a directory
	IsDir() bool
	// ReadDir returns information of directory files
	ReadDir() ([]FileInfo, error)
	// GetName returns a file name
	GetName() string
	// GetPath returns a file path
	GetPath() string
	// GetModePerm returns file permissions
	GetModePerm() os.FileMode
	// GetSize returns file size
	GetSize() int64
	// GetModTime returns file modification time
	GetModTime() time.Time
	// GetAccessTime returns file last access time
	GetAccessTime() time.Time
}

// CreateDownloadCommand configures and returns a command used
// to download a file
func CreateDownloadCommand(cfg Config) (Command, error) {
	cfg.Flags.Sink = true
	cfg.Flags.Source = false
	cmd, err := CreateCommand(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return cmd, nil
}

// CreateUploadCommand configures and returns a command used
// to upload a file
func CreateUploadCommand(cfg Config) (Command, error) {
	cfg.Flags.Sink = false
	cfg.Flags.Source = true
	cmd, err := CreateCommand(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return cmd, nil
}

// CheckAndSetDefaults checks and sets default values
func (c *Config) CheckAndSetDefaults() error {
	logger := c.Log
	if logger == nil {
		logger = slog.Default()
	}
	c.Log = logger.With(
		teleport.ComponentKey, "SCP",
		"local_addr", c.Flags.LocalAddr,
		"remote_addr", c.Flags.RemoteAddr,
		"target", c.Flags.Target,
		"preserve_attrs", c.Flags.PreserveAttrs,
		"user", c.User,
		"run_on_server", c.RunOnServer,
		"remote_location", c.RemoteLocation,
	)
	if c.FileSystem == nil {
		c.FileSystem = &localFileSystem{}
	}
	if c.User == "" {
		return trace.BadParameter("missing User parameter")
	}

	return nil
}

// CreateCommand creates and returns a new SCP command with
// specified configuration.
func CreateCommand(cfg Config) (Command, error) {
	err := cfg.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &command{
		Config: cfg,
		log:    cfg.Log,
	}, nil
}

// Command mimics behavior of SCP command line tool
// to teleport can pretend it launches real SCP behind the scenes
type command struct {
	Config
	log *slog.Logger
}

// Execute implements SSH file copy (SCP). It is called on both tsh (client)
// and teleport (server) side.
func (cmd *command) Execute(ch io.ReadWriter) (err error) {
	if cmd.Flags.Source {
		return trace.Wrap(cmd.serveSource(ch))
	}
	return trace.Wrap(cmd.serveSink(ch))
}

// GetRemoteShellCmd returns a command line to copy
// file(s) or a directory to a remote location
func (cmd *command) GetRemoteShellCmd() (shellCmd string, err error) {
	if cmd.RemoteLocation == "" {
		return "", trace.BadParameter("missing remote file location")
	}

	// "impersonate" SCP to a server
	// See https://docstore.mik.ua/orelly/networking_2ndEd/ssh/ch03_08.htm, section "scp1 Details"
	// about the hidden to/from switches
	shellCmd = "/usr/bin/scp -f"
	if cmd.Flags.Source {
		shellCmd = "/usr/bin/scp -t"
	}

	if cmd.Flags.Recursive {
		shellCmd += " -r"
	}
	if cmd.Flags.DirectoryMode {
		shellCmd += " -d"
	}
	if cmd.Flags.PreserveAttrs {
		shellCmd += " -p"
	}
	shellCmd += (" " + cmd.RemoteLocation)

	return shellCmd, nil
}

func (cmd *command) serveSource(ch io.ReadWriter) (retErr error) {
	defer func() {
		// If anything goes wrong, notify the remote side so it can terminate
		// with an error too.
		// This is necessary to emit correct audit events (if the remote end is
		// emitting them).
		if retErr != nil {
			cmd.sendErr(ch, retErr)
		}
	}()

	fileInfos := make([]FileInfo, len(cmd.Flags.Target))
	for i := range cmd.Flags.Target {
		fileInfo, err := cmd.FileSystem.GetFileInfo(cmd.Flags.Target[i])
		if err != nil {
			return trace.Errorf("could not access local path %q: %v", cmd.Flags.Target[i], err)
		}
		if fileInfo.IsDir() && !cmd.Flags.Recursive {
			// Note: using any other error constructor (e.g. BadParameter)
			// might lead to relogin attempt and a completely obscure
			// error message
			return trace.Errorf("%v is a directory, use -r flag to copy recursively", fileInfo.GetName())
		}
		fileInfos[i] = fileInfo
	}

	r := newReader(ch)
	if err := r.read(); err != nil {
		return trace.Wrap(err)
	}

	for i := range fileInfos {
		info := fileInfos[i]
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

	cmd.log.DebugContext(context.Background(), "Send completed")
	return nil
}

func (cmd *command) sendDir(r *reader, ch io.ReadWriter, fileInfo FileInfo) error {
	if cmd.Config.Flags.PreserveAttrs {
		if err := cmd.sendFileTimes(r, ch, fileInfo); err != nil {
			return trace.Wrap(err)
		}
	}
	if err := cmd.sendDirMode(r, ch, fileInfo); err != nil {
		return trace.Wrap(err)
	}

	cmd.log.DebugContext(context.Background(), "sendDir got OK")

	fileInfos, err := fileInfo.ReadDir()
	if err != nil {
		return trace.Wrap(err)
	}

	for i := range fileInfos {
		info := fileInfos[i]
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
	return trace.Wrap(r.read())
}

func (cmd *command) sendFile(r *reader, ch io.ReadWriter, fileInfo FileInfo) error {
	reader, err := cmd.FileSystem.OpenFile(fileInfo.GetPath())
	if err != nil {
		return trace.Wrap(err)
	}
	defer reader.Close()

	if cmd.Config.Flags.PreserveAttrs {
		if err := cmd.sendFileTimes(r, ch, fileInfo); err != nil {
			return trace.Wrap(err)
		}
	}

	if err := cmd.sendFileMode(r, ch, fileInfo); err != nil {
		return trace.Wrap(err)
	}

	n, err := io.Copy(ch, reader)
	if err != nil {
		return trace.Wrap(err)
	}
	if n != fileInfo.GetSize() {
		return trace.Errorf("short write: written %v, expected %v", n, fileInfo.GetSize())
	}

	// report progress:
	if cmd.ProgressWriter != nil {
		statusMessage := fmt.Sprintf("-> %s (%d)", fileInfo.GetPath(), fileInfo.GetSize())
		defer fmt.Fprint(cmd.ProgressWriter, utils.EscapeControl(statusMessage)+"\n")
	}
	if err := sendOK(ch); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(r.read())
}

func (cmd *command) sendErr(ch io.Writer, err error) {
	out := fmt.Sprintf("%c%s\n", byte(ErrByte), err)
	if _, err := ch.Write([]byte(out)); err != nil {
		cmd.log.DebugContext(context.Background(), "Failed sending SCP error message to the remote side", "error", err)
	}
}

// serveSink executes file uploading, when a remote server sends file(s)
// via SCP
func (cmd *command) serveSink(ch io.ReadWriter) error {
	// Validate that if directory mode flag was sent, the target is an actual
	// directory.
	if cmd.Flags.DirectoryMode {
		if len(cmd.Flags.Target) != 1 {
			return trace.BadParameter("in directory mode, only single upload target is allowed but %q provided",
				cmd.Flags.Target)
		}
		if !cmd.FileSystem.IsDir(cmd.Flags.Target[0]) {
			return trace.BadParameter("target path must be a directory")
		}
	}

	rootDir := localDir
	if cmd.targetDirExists() {
		rootDir = newPathFromDir(cmd.Flags.Target[0])
	} else if cmd.Flags.Target[0] != "" {
		// Extract potential base directory from the target
		rootDir = newPathFromDir(filepath.Dir(cmd.Flags.Target[0]))
	}

	if err := sendOK(ch); err != nil {
		return trace.Wrap(err)
	}

	var st state
	st.path = rootDir
	var b [1]byte
	scanner := bufio.NewScanner(ch)
	for {
		n, err := ch.Read(b[:])
		if err != nil {
			if errors.Is(err, io.EOF) {
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
	cmd.log.DebugContext(context.Background(), "processing command", "b", string(b), "line", line)
	switch b {
	case WarnByte, ErrByte:
		return trace.Errorf("error from sender: %q", line)
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
		if len(st.path) == 0 {
			return trace.Errorf("empty path")
		}
		return cmd.updateDirTimes(st.pop())
	case 'T':
		stat, err := parseFileTimes(line)
		if err != nil {
			return trace.Wrap(err)
		}
		st.stat = stat
		return nil
	}
	return trace.Errorf("got unrecognized command: %v", string(b))
}

func (cmd *command) receiveFile(st *state, fc newFileCmd, ch io.ReadWriter) error {
	ctx := context.Background()
	cmd.log.DebugContext(ctx, "processing file copy request", "targets", cmd.Flags.Target, "file_name", fc.Name)

	// Unless target specifies a file, use the file name from the command
	path := cmd.Flags.Target[0]
	if cmd.FileSystem.IsDir(cmd.Flags.Target[0]) {
		path = st.makePath(fc.Name)
	}

	writer, err := cmd.FileSystem.CreateFile(path, fc.Length)
	if err != nil {
		return trace.Wrap(err)
	}
	defer writer.Close()

	// report progress:
	if cmd.ProgressWriter != nil {
		statusMessage := fmt.Sprintf("<- %s (%d)", path, fc.Length)
		defer fmt.Fprint(cmd.ProgressWriter, utils.EscapeControl(statusMessage)+"\n")
	}

	if err = sendOK(ch); err != nil {
		return trace.Wrap(err)
	}

	n, err := io.CopyN(writer, ch, int64(fc.Length))
	if err != nil {
		return trace.Wrap(err)
	}

	if n != int64(fc.Length) {
		return trace.Errorf("unexpected file copy length: %v", n)
	}

	// Change the file permissions only when client requested it.
	if cmd.Flags.PreserveAttrs {
		if err := cmd.FileSystem.Chmod(path, int(fc.Mode)); err != nil {
			return trace.Wrap(err)
		}
	}

	if st.stat != nil {
		err = cmd.FileSystem.Chtimes(path, st.stat.Atime, st.stat.Mtime)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	cmd.log.DebugContext(ctx, "File successfully copied", "file", fc.Name, "size", fc.Length, "destination", path)
	return nil
}

func (cmd *command) receiveDir(st *state, fc newFileCmd, ch io.ReadWriter) error {
	cmd.log.DebugContext(context.Background(), "processing directory copy request", "targets", cmd.Flags.Target, "name", fc.Name)

	if cmd.FileSystem.IsDir(cmd.Flags.Target[0]) {
		// Copying into an existing directory? append to it:
		st.push(fc.Name, st.stat)
	} else {
		// If target specifies a new directory, we need to reset
		// state with it
		st.path = newPathFromDirAndTimes(cmd.Flags.Target[0], st.stat)
	}
	targetDir := st.path.join()

	err := cmd.FileSystem.MkDir(targetDir, int(fc.Mode))
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	return nil
}

func (cmd *command) sendDirMode(r *reader, ch io.Writer, fileInfo FileInfo) error {
	out := fmt.Sprintf("D%04o 0 %s\n", fileInfo.GetModePerm(), fileInfo.GetName())
	cmd.log.DebugContext(context.Background(), "Sending directory mode", "cmd", out)
	_, err := io.WriteString(ch, out)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(r.read())
}

func (cmd *command) sendFileTimes(r *reader, ch io.Writer, fileInfo FileInfo) error {
	// OpenSSH handles nanoseconds to a certain precision
	// which is not sufficient to keep the exact timestamps:
	// See these for details:
	// https://github.com/openssh/openssh-portable/blob/279261e1ea8150c7c64ab5fe7cb4a4ea17acbb29/scp.c#L619-L621
	// https://github.com/openssh/openssh-portable/blob/279261e1ea8150c7c64ab5fe7cb4a4ea17acbb29/scp.c#L1332
	// https://github.com/openssh/openssh-portable/blob/279261e1ea8150c7c64ab5fe7cb4a4ea17acbb29/scp.c#L1344
	//
	// Se we copy its behavior and drop nanoseconds entirely
	out := fmt.Sprintf("T%d 0 %d 0\n",
		fileInfo.GetModTime().Unix(),
		fileInfo.GetAccessTime().Unix(),
	)
	cmd.log.DebugContext(context.Background(), "Sending file times", "cmd", out)
	_, err := io.WriteString(ch, out)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(r.read())
}

func (cmd *command) sendFileMode(r *reader, ch io.Writer, fileInfo FileInfo) error {
	out := fmt.Sprintf("C%04o %d %s\n",
		fileInfo.GetModePerm(),
		fileInfo.GetSize(),
		fileInfo.GetName(),
	)
	cmd.log.DebugContext(context.Background(), "Sending file mode", "cmd", out)
	_, err := io.WriteString(ch, out)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(r.read())
}

func (cmd *command) updateDirTimes(path pathSegments) error {
	if stat := path[len(path)-1].stat; stat != nil {
		err := cmd.FileSystem.Chtimes(path.join(), stat.Atime, stat.Mtime)
		if err != nil {
			return trace.ConvertSystemError(err)
		}
	}
	return nil
}

func (cmd *command) targetDirExists() bool {
	return len(cmd.Flags.Target) != 0 && cmd.FileSystem.IsDir(cmd.Flags.Target[0])
}

func (r newFileCmd) String() string {
	return fmt.Sprintf("newFileCmd(mode=%o,len=%d,name=%v)", r.Mode, r.Length, r.Name)
}

type newFileCmd struct {
	Mode   int64
	Length uint64
	Name   string
}

func parseNewFile(line string) (*newFileCmd, error) {
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

	// Don't allow malicious servers to send bad directory names. For more
	// details, see:
	//   * https://sintonen.fi/advisories/scp-client-multiple-vulnerabilities.txt
	//   * https://github.com/openssh/openssh-portable/commit/6010c03
	c.Name = parts[2]
	if len(c.Name) == 0 || strings.HasPrefix(c.Name, string(filepath.Separator)) || c.Name == "." || c.Name == ".." {
		return nil, trace.BadParameter("invalid name")
	}

	return &c, nil
}

type mtimeCmd struct {
	Mtime time.Time
	Atime time.Time
}

// parseFileTimes parses the input with access/modification file times:
//
// T<mtime.sec> <mtime.usec> <atime.sec> <atime.usec>
//
// Note that the leading 'T' will not be part of the input as it has already
// been seen and removed
func parseFileTimes(line string) (*mtimeCmd, error) {
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
	}
	return nil
}

type state struct {
	path pathSegments
	// stat optionally specifies access/modification time for the current file/directory
	stat *mtimeCmd
}

func (r pathSegments) join(elems ...string) string {
	path := make([]string, 0, len(r))
	for _, s := range r {
		path = append(path, s.dir)
	}
	return filepath.Join(append(path, elems...)...)
}

var localDir = newPathFromDir(".")

func newPathFromDir(dir string) pathSegments {
	return pathSegments{{dir: dir}}
}

func newPathFromDirAndTimes(dir string, stat *mtimeCmd) pathSegments {
	return pathSegments{{dir: dir, stat: stat}}
}

type pathSegments []pathSegment

type pathSegment struct {
	dir string
	// stat optionally specifies access/modification time for the directory
	stat *mtimeCmd
}

func (st *state) push(dir string, stat *mtimeCmd) {
	st.path = append(st.path, pathSegment{dir: dir, stat: stat})
}

// pop removes the last segment from the current path.
// Returns the old path as a result
func (st *state) pop() pathSegments {
	if len(st.path) == 0 {
		return nil
	}
	path := st.path
	st.path = st.path[:len(st.path)-1]
	st.stat = nil
	return path
}

func (st *state) makePath(filename string) string {
	return st.path.join(filename)
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
		return trace.BadParameter("unexpected error, read 0 bytes")
	}

	switch r.b[0] {
	case OKByte:
		return nil
	case WarnByte, ErrByte:
		r.s.Scan()
		if err := r.s.Err(); err != nil {
			return trace.Wrap(err)
		}
		return trace.BadParameter("error from receiver: %q", r.s.Text())
	}
	return trace.BadParameter("unrecognized command: %v", r.b)
}
