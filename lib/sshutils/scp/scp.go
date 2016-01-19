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
package scp

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
)

const (
	OKByte   = 0x0
	WarnByte = 0x1
	ErrByte  = 0x2
)

type Server struct {
	cmd Command
}

func New(cmd Command) (*Server, error) {
	return &Server{cmd: cmd}, nil
}

func (s *Server) Serve(ch io.ReadWriter) error {
	if s.cmd.Source {
		return s.serveSource(ch)
	}
	return s.serveSink(ch)
}

func (s *Server) serveSource(ch io.ReadWriter) error {
	log.Infof("serving source")

	r := newReader(ch)

	if err := r.read(); err != nil {
		return trace.Wrap(err)
	}

	f, err := os.Stat(s.cmd.Target)
	if err != nil {
		log.Infof("failed to stat file: %v", err)
		return sendError(ch, err.Error())
	}

	if f.IsDir() && !s.cmd.Recursive {
		return sendError(
			ch, fmt.Sprintf(
				"%v is not a file, turn recursive mode to copy dirs",
				s.cmd.Target))
	}

	if f.IsDir() {
		if err := s.sendDir(r, ch, f, s.cmd.Target); err != nil {
			return sendError(ch, err.Error())
		}
	} else {
		if err := s.sendFile(r, ch, f, s.cmd.Target); err != nil {
			return sendError(ch, err.Error())
		}
	}

	log.Infof("send completed")
	return nil
}

func (s *Server) sendDir(r *reader, ch io.ReadWriter, fi os.FileInfo, path string) error {
	_, err := fmt.Fprintf(ch, "D%04o 0 %s\n", fi.Mode()&os.ModePerm, fi.Name())
	if err != nil {
		return trace.Wrap(err)
	}
	if err := r.read(); err != nil {
		return trace.Wrap(err)
	}
	log.Infof("sendDir got OK")
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
			err := s.sendDir(r, ch, sfi, filepath.Join(path, sfi.Name()))
			if err != nil {
				return trace.Wrap(err)
			}
		} else {
			err := s.sendFile(r, ch, sfi, filepath.Join(path, sfi.Name()))
			if err != nil {
				return trace.Wrap(err)
			}
		}
	}
	if _, err = fmt.Fprintf(ch, "E\n"); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *Server) sendFile(r *reader, ch io.ReadWriter, fi os.FileInfo, path string) error {
	_, err := fmt.Fprintf(ch, "C%04o %d %s\n", fi.Mode()&os.ModePerm, fi.Size(), fi.Name())
	if err != nil {
		return trace.Wrap(err)
	}
	if err := r.read(); err != nil {
		return trace.Wrap(err)
	}
	log.Infof("sendFile got OK")
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
	return r.read()
}

func (s *Server) serveSink(ch io.ReadWriter) error {
	log.Infof("serving sink")

	if err := sendOK(ch); err != nil {
		return trace.Wrap(err)
	}
	st := &state{}
	var b = make([]byte, 1)
	r := bufio.NewScanner(ch)
	for {
		n, err := ch.Read(b)
		if err != nil {
			if err == io.EOF {
				log.Infof("got EOF")
				return nil
			}
			return trace.Wrap(err)
		}
		if n < 1 {
			return trace.Errorf("unexpected error, read 0 bytes")
		}

		if b[0] == OKByte {
			log.Infof("got OK")
			continue
		}

		r.Scan()
		if err := r.Err(); err != nil {
			return trace.Wrap(err)
		}
		if err := s.processCommand(ch, st, b[0], r.Text()); err != nil {
			if e := sendError(ch, err.Error()); e != nil {
				log.Warningf("error sending error: %v", e)
			}
			return trace.Wrap(err)
		}
		if err := sendOK(ch); err != nil {
			return trace.Wrap(err)
		}
		log.Infof("sent OK")
	}
}

func (s *Server) processCommand(ch io.ReadWriter, st *state, b byte, line string) error {
	switch b {
	case WarnByte:
		log.Warningf("got warning: %v", line)
		return nil
	case ErrByte:
		return trace.Errorf(line)
	case 'C':
		f, err := ParseNewFile(line)
		if err != nil {
			return trace.Wrap(err)
		}
		log.Infof("got new file command: %#v", f)
		if err := sendOK(ch); err != nil {
			return trace.Wrap(err)
		}
		return s.receiveFile(st, *f, ch)
	case 'D':
		d, err := ParseNewFile(line)
		if err != nil {
			return trace.Wrap(err)
		}
		log.Infof("got new dir command: %#v", d)
		if err := s.receiveDir(st, *d, ch); err != nil {
			return trace.Wrap(err)
		}
		return nil
	case 'E':
		log.Infof("got end dir command")
		return st.pop()
	case 'T':
		log.Infof("got mtime command")
		m, err := ParseMtime(line)
		if err != nil {
			return trace.Wrap(err)
		}
		log.Infof("got mtime command: %#v", m)
	}
	return trace.Errorf("got unrecognized command: %v", string(b))
}

func (s *Server) receiveFile(st *state, cmd NewFileCmd, ch io.ReadWriter) error {
	// if the dest path ends with "/", destination is a folder, we should
	// save the file to that folder
	// if the dest path doesn't end with "/", destination is a target filename
	// we should save the file to that filename
	// if the scp command has "-r" flag, desctination is always a folder
	path := s.cmd.Target
	if s.cmd.Recursive || strings.HasSuffix(s.cmd.Target, "/") {
		path = st.makePath(s.cmd.Target, cmd.Name)
	}
	f, err := os.Create(path)
	if err != nil {
		return trace.Wrap(err)
	}
	defer f.Close()
	n, err := io.CopyN(f, ch, int64(cmd.Length))
	if err != nil {
		return trace.Wrap(err)
	}
	if n != int64(cmd.Length) {
		return trace.Errorf("unexpected file copy length: %v", n)
	}
	mode := os.FileMode(int(cmd.Mode) & int(os.ModePerm))
	if err := os.Chmod(path, mode); err != nil {
		return trace.Wrap(err)
	}
	log.Infof("file %v(%v) copied to %v", cmd.Name, cmd.Length, path)
	return nil
}

func (s *Server) receiveDir(st *state, cmd NewFileCmd, ch io.ReadWriter) error {
	// if the dest path ends with "/", we should copy source folder
	// inside the dest folder
	// if the dest path doesn't end with "/", we should copy only the
	// content of the source folder to the dest folder
	// for all the copied subfolders we should copy source folder
	// inside dest folder
	path := s.cmd.Target
	if strings.HasSuffix(s.cmd.Target, "/") || st.notRoot {
		path = st.makePath(s.cmd.Target, cmd.Name)
		st.push(cmd.Name)

	}
	st.notRoot = true //next calls of receiveDir will be for subfolders
	mode := os.FileMode(int(cmd.Mode) & int(os.ModePerm))
	err := os.Mkdir(path, mode)
	if err != nil {
		return trace.Wrap(err)
	}
	log.Infof("dir %v(%v) created", cmd.Name, path)
	return nil
}

type Command struct {
	Source      bool // data producer
	Sink        bool // data consumer
	Verbose     bool // verbose
	TargetIsDir bool // target should be dir
	Target      string
	Recursive   bool
}

func IsSCP(cmd string) bool {
	args := strings.Split(cmd, " ")
	if len(args) < 1 {
		return false
	}
	_, f := filepath.Split(args[0])
	return f == "scp"
}

func ParseCommand(arg string) (*Command, error) {
	if !IsSCP(arg) {
		return nil, trace.Errorf("not scp command")
	}
	args := strings.Split(arg, " ")
	f := flag.NewFlagSet(args[0], flag.ContinueOnError)
	var cmd Command

	f.BoolVar(&cmd.Sink, "t", false, "sink mode (data consumer)")
	f.BoolVar(&cmd.Source, "f", false, "source mode (data producer)")
	f.BoolVar(&cmd.Verbose, "v", false, "verbose mode")
	f.BoolVar(&cmd.TargetIsDir, "d", false, "target is dir and must exist")
	f.BoolVar(&cmd.Recursive, "r", false, "is recursive")

	if err := f.Parse(args[1:]); err != nil {
		return nil, trace.Wrap(err)
	}

	cmd.Target = f.Arg(0)
	if cmd.Target == "" {
		return nil, trace.Errorf("missing target")
	}

	if !cmd.Source && !cmd.Sink {
		return nil, trace.Errorf("remote mode is not supported")
	}

	return &cmd, nil
}

type NewFileCmd struct {
	Mode   int64
	Length uint64
	Name   string
}

func ParseNewFile(line string) (*NewFileCmd, error) {
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

func sendError(ch io.ReadWriter, message string) error {
	bytes := make([]byte, 0, len(message)+2)
	bytes = append(bytes, ErrByte)
	bytes = append(bytes, message...)
	bytes = append(bytes, []byte{'\n'}...)
	_, err := ch.Write(bytes)
	if err != nil {
		return trace.Wrap(err)
	} else {
		return nil
	}
}

type state struct {
	notRoot  bool
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
		log.Infof("got OK")
		return nil
	case WarnByte, ErrByte:
		r.s.Scan()
		if err := r.s.Err(); err != nil {
			return trace.Wrap(err)
		}
		if r.b[0] == ErrByte {
			return trace.Wrap(err)
		}
		log.Warningf("warn: %v", r.s.Text())
		return nil
	}
	return trace.Errorf("unrecognized command: %#v", r.b)
}
