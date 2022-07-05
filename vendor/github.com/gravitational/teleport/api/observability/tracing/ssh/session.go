// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssh

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/gravitational/trace"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/ssh"
)

// A Session represents a connection to a remote command or shell.
//
// This is a forked version of golang.org/x/crypto/ssh.Session
// that wraps payloads sent across the underlying channel in an
// Envelope, which allows us to provide tracing context to
// the server processing the requests.
type Session struct {
	// Stdin specifies the remote process's standard input.
	// If Stdin is nil, the remote process reads from an empty
	// bytes.Buffer.
	Stdin io.Reader

	// Stdout and Stderr specify the remote process's standard
	// output and error.
	//
	// If either is nil, Run connects the corresponding file
	// descriptor to an instance of io.Discard. There is a
	// fixed amount of buffering that is shared for the two streams.
	// If either blocks it may eventually cause the remote
	// command to block.
	Stdout io.Writer
	Stderr io.Writer

	ch        *Channel // the channel backing this session
	started   bool     // true once Start, Run or Shell is invoked.
	copyFuncs []func() error
	errors    chan error // one send per copyFunc

	// true if pipe method is active
	stdinpipe, stdoutpipe, stderrpipe bool

	// stdinPipeWriter is non-nil if StdinPipe has not been called
	// and Stdin was specified by the user; it is the write end of
	// a pipe connecting Session.Stdin to the stdin channel.
	stdinPipeWriter io.WriteCloser

	exitStatus chan error

	// tracer specifies the Tracer to be used to start spans for any
	// operations performed on this Session.
	tracer oteltrace.Tracer
	// tracingSupported indicates whether the server is capable
	// of receiving a tracing Envelope.
	tracingSupported bool
}

// SendRequest sends an out-of-band channel request on the SSH channel
// underlying the session.
func (s *Session) SendRequest(ctx context.Context, name string, wantReply bool, payload []byte) (bool, error) {
	ctx, span := s.tracer.Start(
		ctx,
		fmt.Sprintf("ssh.SendRequest/%s", name),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String("ssh.Session"),
			semconv.RPCMethodKey.String("SendRequest"),
			semconv.RPCSystemKey.String("ssh"),
			attribute.Bool("want_reply", wantReply),
		),
	)
	defer span.End()

	ok, err := s.ch.SendRequest(ctx, name, wantReply, payload)
	return ok, trace.Wrap(err)
}

func (s *Session) Close() error {
	return trace.Wrap(s.ch.Close())
}

// RFC 4254 Section 6.4.
type setenvRequest struct {
	Name  string
	Value string
}

// Setenv sets an environment variable that will be applied to any
// command executed by Shell or Run.
func (s *Session) Setenv(ctx context.Context, name, value string) error {
	ctx, span := s.tracer.Start(
		ctx,
		fmt.Sprintf("ssh.Setenv/%s", name),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String("ssh.Session"),
			semconv.RPCMethodKey.String("SendRequest"),
			semconv.RPCSystemKey.String("ssh"),
		),
	)
	defer span.End()

	msg := setenvRequest{
		Name:  name,
		Value: value,
	}
	ok, err := s.ch.SendRequest(ctx, "env", true, ssh.Marshal(&msg))
	if err == nil && !ok {
		err = errors.New("ssh: setenv failed")
	}
	return trace.Wrap(err)
}

// RFC 4254 Section 6.2.
type ptyRequestMsg struct {
	Term     string
	Columns  uint32
	Rows     uint32
	Width    uint32
	Height   uint32
	Modelist string
}

// POSIX terminal mode flags as listed in RFC 4254 Section 8.
const (
	ttyOpEnd = 0
)

// RequestPty requests the association of a pty with the session on the remote host.
func (s *Session) RequestPty(ctx context.Context, term string, h, w int, termmodes ssh.TerminalModes) error {
	ctx, span := s.tracer.Start(
		ctx,
		fmt.Sprintf("ssh.RequestPty/%s", term),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String("ssh.Session"),
			semconv.RPCMethodKey.String("SendRequest"),
			semconv.RPCSystemKey.String("ssh"),
			attribute.Int("width", w),
			attribute.Int("height", h),
		),
	)
	defer span.End()

	var tm []byte
	for k, v := range termmodes {
		kv := struct {
			Key byte
			Val uint32
		}{k, v}

		tm = append(tm, ssh.Marshal(&kv)...)
	}
	tm = append(tm, ttyOpEnd)
	req := ptyRequestMsg{
		Term:     term,
		Columns:  uint32(w),
		Rows:     uint32(h),
		Width:    uint32(w * 8),
		Height:   uint32(h * 8),
		Modelist: string(tm),
	}
	ok, err := s.ch.SendRequest(ctx, "pty-req", true, ssh.Marshal(&req))
	if err == nil && !ok {
		err = errors.New("ssh: pty-req failed")
	}
	return trace.Wrap(err)
}

// RFC 4254 Section 6.5.
type subsystemRequestMsg struct {
	Subsystem string
}

// RequestSubsystem requests the association of a subsystem with the session on the remote host.
// A subsystem is a predefined command that runs in the background when the ssh session is initiated
func (s *Session) RequestSubsystem(ctx context.Context, subsystem string) error {
	ctx, span := s.tracer.Start(
		ctx,
		fmt.Sprintf("ssh.RequestSubsystem/%s", subsystem),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String("ssh.Session"),
			semconv.RPCMethodKey.String("RequestSubsystem"),
			semconv.RPCSystemKey.String("ssh"),
		),
	)
	defer span.End()

	msg := subsystemRequestMsg{
		Subsystem: subsystem,
	}

	ok, err := s.ch.SendRequest(ctx, "subsystem", true, ssh.Marshal(&msg))
	if err == nil && !ok {
		err = errors.New("ssh: subsystem request failed")
	}
	return trace.Wrap(err)
}

// RFC 4254 Section 6.7.
type ptyWindowChangeMsg struct {
	Columns uint32
	Rows    uint32
	Width   uint32
	Height  uint32
}

// WindowChange informs the remote host about a terminal window dimension change to h rows and w columns.
func (s *Session) WindowChange(ctx context.Context, h, w int) error {
	ctx, span := s.tracer.Start(
		ctx,
		"ssh.WindowChange",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String("ssh.Session"),
			semconv.RPCMethodKey.String("WindowChange"),
			semconv.RPCSystemKey.String("ssh"),
			attribute.Int("height", h),
			attribute.Int("width", w),
		),
	)
	defer span.End()

	req := ptyWindowChangeMsg{
		Columns: uint32(w),
		Rows:    uint32(h),
		Width:   uint32(w * 8),
		Height:  uint32(h * 8),
	}
	_, err := s.ch.SendRequest(ctx, "window-change", false, ssh.Marshal(&req))
	return trace.Wrap(err)
}

// RFC 4254 Section 6.9.
type signalMsg struct {
	Signal string
}

// Signal sends the given signal to the remote process.
// sig is one of the SIG* constants.
func (s *Session) Signal(ctx context.Context, sig ssh.Signal) error {
	ctx, span := s.tracer.Start(
		ctx,
		fmt.Sprintf("ssh.Signal/%s", sig),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String("ssh.Session"),
			semconv.RPCMethodKey.String("Signal"),
			semconv.RPCSystemKey.String("ssh"),
		),
	)
	defer span.End()

	msg := signalMsg{
		Signal: string(sig),
	}

	_, err := s.ch.SendRequest(ctx, "signal", false, ssh.Marshal(&msg))
	return trace.Wrap(err)
}

// RFC 4254 Section 6.5.
type execMsg struct {
	Command string
}

// Start runs cmd on the remote host. Typically, the remote
// server passes cmd to the shell for interpretation.
// A Session only accepts one call to Run, Start or Shell.
func (s *Session) Start(ctx context.Context, cmd string) error {
	ctx, span := s.tracer.Start(
		ctx,
		fmt.Sprintf("ssh.Start/%s", stripCommandName(cmd)),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String("ssh.Session"),
			semconv.RPCMethodKey.String("Start"),
			semconv.RPCSystemKey.String("ssh"),
		),
	)
	defer span.End()

	if s.started {
		return trace.Wrap(errors.New("ssh: session already started"))
	}
	msg := execMsg{
		Command: cmd,
	}

	ok, err := s.ch.SendRequest(ctx, "exec", true, ssh.Marshal(&msg))
	if err == nil && !ok {
		err = fmt.Errorf("ssh: command %v failed", cmd)
	}
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(s.start())
}

// stripCommandName attempts to extract only the command
// name from the provided cmd which may contain a command
// and arguments. This prevents any potentially sensitive
// information being included in a trace.
func stripCommandName(cmd string) string {
	return strings.SplitN(cmd, " ", 2)[0]
}

// Run runs cmd on the remote host. Typically, the remote
// server passes cmd to the shell for interpretation.
// A Session only accepts one call to Run, Start, Shell, Output,
// or CombinedOutput.
//
// The returned error is nil if the command runs, has no problems
// copying stdin, stdout, and stderr, and exits with a zero exit
// status.
//
// If the remote server does not send an exit status, an error of type
// *ExitMissingError is returned. If the command completes
// unsuccessfully or is interrupted by a signal, the error is of type
// *ExitError. Other error types may be returned for I/O problems.
func (s *Session) Run(ctx context.Context, cmd string) error {
	ctx, span := s.tracer.Start(
		ctx,
		fmt.Sprintf("ssh.Run/%s", stripCommandName(cmd)),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String("ssh.Session"),
			semconv.RPCMethodKey.String("Run"),
			semconv.RPCSystemKey.String("ssh"),
		),
	)
	defer span.End()

	err := s.Start(ctx, cmd)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(s.Wait())
}

// Output runs cmd on the remote host and returns its standard output.
func (s *Session) Output(ctx context.Context, cmd string) ([]byte, error) {
	ctx, span := s.tracer.Start(
		ctx,
		fmt.Sprintf("ssh.Output/%s", stripCommandName(cmd)),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String("ssh.Session"),
			semconv.RPCMethodKey.String("Output"),
			semconv.RPCSystemKey.String("ssh"),
		),
	)
	defer span.End()

	if s.Stdout != nil {
		return nil, trace.Wrap(errors.New("ssh: Stdout already set"))
	}
	var b bytes.Buffer
	s.Stdout = &b
	err := s.Run(ctx, cmd)
	return b.Bytes(), trace.Wrap(err)
}

type singleWriter struct {
	b  bytes.Buffer
	mu sync.Mutex
}

func (w *singleWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.Write(p)
}

// CombinedOutput runs cmd on the remote host and returns its combined
// standard output and standard error.
func (s *Session) CombinedOutput(ctx context.Context, cmd string) ([]byte, error) {
	ctx, span := s.tracer.Start(
		ctx,
		fmt.Sprintf("ssh.CombinedOutput/%s", stripCommandName(cmd)),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String("ssh.Session"),
			semconv.RPCMethodKey.String("CombinedOutput"),
			semconv.RPCSystemKey.String("ssh"),
		),
	)
	defer span.End()

	if s.Stdout != nil {
		return nil, trace.Wrap(errors.New("ssh: Stdout already set"))
	}
	if s.Stderr != nil {
		return nil, trace.Wrap(errors.New("ssh: Stderr already set"))
	}
	var b singleWriter
	s.Stdout = &b
	s.Stderr = &b
	err := s.Run(ctx, cmd)
	return b.b.Bytes(), trace.Wrap(err)
}

// Shell starts a login shell on the remote host. A Session only
// accepts one call to Run, Start, Shell, Output, or CombinedOutput.
func (s *Session) Shell(ctx context.Context) error {
	ctx, span := s.tracer.Start(
		ctx,
		"ssh.Shell",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String("ssh.Session"),
			semconv.RPCMethodKey.String("Shell"),
			semconv.RPCSystemKey.String("ssh"),
		),
	)
	defer span.End()

	if s.started {
		return trace.Wrap(errors.New("ssh: session already started"))
	}

	ok, err := s.ch.SendRequest(ctx, "shell", true, nil)
	if err == nil && !ok {
		return trace.Wrap(errors.New("ssh: could not start shell"))
	}
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(s.start())
}

func (s *Session) start() error {
	s.started = true

	type F func(*Session)
	for _, setupFd := range []F{(*Session).stdin, (*Session).stdout, (*Session).stderr} {
		setupFd(s)
	}

	s.errors = make(chan error, len(s.copyFuncs))
	for _, fn := range s.copyFuncs {
		go func(fn func() error) {
			s.errors <- fn()
		}(fn)
	}
	return nil
}

// Wait waits for the remote command to exit.
//
// The returned error is nil if the command runs, has no problems
// copying stdin, stdout, and stderr, and exits with a zero exit
// status.
//
// If the remote server does not send an exit status, an error of type
// *ExitMissingError is returned. If the command completes
// unsuccessfully or is interrupted by a signal, the error is of type
// *ExitError. Other error types may be returned for I/O problems.
func (s *Session) Wait() error {
	if !s.started {
		return trace.Wrap(errors.New("ssh: session not started"))
	}
	waitErr := <-s.exitStatus

	if s.stdinPipeWriter != nil {
		s.stdinPipeWriter.Close()
	}
	var copyError error
	for range s.copyFuncs {
		if err := <-s.errors; err != nil && copyError == nil {
			copyError = err
		}
	}
	if waitErr != nil {
		return trace.Wrap(waitErr)
	}
	return trace.Wrap(copyError)
}

var signals = map[ssh.Signal]int{
	ssh.SIGABRT: 6,
	ssh.SIGALRM: 14,
	ssh.SIGFPE:  8,
	ssh.SIGHUP:  1,
	ssh.SIGILL:  4,
	ssh.SIGINT:  2,
	ssh.SIGKILL: 9,
	ssh.SIGPIPE: 13,
	ssh.SIGQUIT: 3,
	ssh.SIGSEGV: 11,
	ssh.SIGTERM: 15,
}

func (s *Session) wait(reqs <-chan *ssh.Request) error {
	wm := Waitmsg{status: -1}
	// Wait for msg channel to be closed before returning.
	for msg := range reqs {
		switch msg.Type {
		case "exit-status":
			wm.status = int(binary.BigEndian.Uint32(msg.Payload))
		case "exit-signal":
			var sigval struct {
				Signal     string
				CoreDumped bool
				Error      string
				Lang       string
			}
			if err := ssh.Unmarshal(msg.Payload, &sigval); err != nil {
				return trace.Wrap(err)
			}

			// Must sanitize strings?
			wm.signal = sigval.Signal
			wm.msg = sigval.Error
			wm.lang = sigval.Lang
		default:
			// This handles keepalives and matches
			// OpenSSH's behaviour.
			if msg.WantReply {
				msg.Reply(false, nil)
			}
		}
	}
	if wm.status == 0 {
		return nil
	}
	if wm.status == -1 {
		// exit-status was never sent from server
		if wm.signal == "" {
			// signal was not sent either.  RFC 4254
			// section 6.10 recommends against this
			// behavior, but it is allowed, so we let
			// clients handle it.
			return &ExitMissingError{}
		}
		wm.status = 128
		if _, ok := signals[ssh.Signal(wm.signal)]; ok {
			wm.status += signals[ssh.Signal(wm.signal)]
		}
	}

	return &ExitError{wm}
}

// ExitMissingError is returned if a session is torn down cleanly, but
// the server sends no confirmation of the exit status.
type ExitMissingError struct{}

func (e *ExitMissingError) Error() string {
	return "wait: remote command exited without exit status or exit signal"
}

func (s *Session) stdin() {
	if s.stdinpipe {
		return
	}
	var stdin io.Reader
	if s.Stdin == nil {
		stdin = new(bytes.Buffer)
	} else {
		r, w := io.Pipe()
		go func() {
			_, err := io.Copy(w, s.Stdin)
			w.CloseWithError(err)
		}()
		stdin, s.stdinPipeWriter = r, w
	}
	s.copyFuncs = append(s.copyFuncs, func() error {
		_, err := io.Copy(s.ch, stdin)
		if err1 := s.ch.CloseWrite(); err == nil && err1 != io.EOF {
			err = err1
		}
		return err
	})
}

func (s *Session) stdout() {
	if s.stdoutpipe {
		return
	}
	if s.Stdout == nil {
		s.Stdout = io.Discard
	}
	s.copyFuncs = append(s.copyFuncs, func() error {
		_, err := io.Copy(s.Stdout, s.ch)
		return err
	})
}

func (s *Session) stderr() {
	if s.stderrpipe {
		return
	}
	if s.Stderr == nil {
		s.Stderr = io.Discard
	}
	s.copyFuncs = append(s.copyFuncs, func() error {
		_, err := io.Copy(s.Stderr, s.ch.Stderr())
		return err
	})
}

// sessionStdin reroutes Close to CloseWrite.
type sessionStdin struct {
	io.Writer
	ch *Channel
}

func (s *sessionStdin) Close() error {
	return trace.Wrap(s.ch.CloseWrite())
}

// StdinPipe returns a pipe that will be connected to the
// remote command's standard input when the command starts.
func (s *Session) StdinPipe() (io.WriteCloser, error) {
	if s.Stdin != nil {
		return nil, trace.Wrap(errors.New("ssh: Stdin already set"))
	}
	if s.started {
		return nil, trace.Wrap(errors.New("ssh: StdinPipe after process started"))
	}
	s.stdinpipe = true
	return &sessionStdin{s.ch, s.ch}, nil
}

// StdoutPipe returns a pipe that will be connected to the
// remote command's standard output when the command starts.
// There is a fixed amount of buffering that is shared between
// stdout and stderr streams. If the StdoutPipe reader is
// not serviced fast enough it may eventually cause the
// remote command to block.
func (s *Session) StdoutPipe() (io.Reader, error) {
	if s.Stdout != nil {
		return nil, trace.Wrap(errors.New("ssh: Stdout already set"))
	}
	if s.started {
		return nil, trace.Wrap(errors.New("ssh: StdoutPipe after process started"))
	}
	s.stdoutpipe = true
	return s.ch, nil
}

// StderrPipe returns a pipe that will be connected to the
// remote command's standard error when the command starts.
// There is a fixed amount of buffering that is shared between
// stdout and stderr streams. If the StderrPipe reader is
// not serviced fast enough it may eventually cause the
// remote command to block.
func (s *Session) StderrPipe() (io.Reader, error) {
	if s.Stderr != nil {
		return nil, trace.Wrap(errors.New("ssh: Stderr already set"))
	}
	if s.started {
		return nil, trace.Wrap(errors.New("ssh: StderrPipe after process started"))
	}
	s.stderrpipe = true
	return s.ch.Stderr(), nil
}

// newSession returns a new interactive session on the remote host.
func newSession(ch *Channel, reqs <-chan *ssh.Request, tracingSupported bool, tracer oteltrace.Tracer) (*Session, error) {
	s := &Session{
		ch:               ch,
		tracingSupported: tracingSupported,
		tracer:           tracer,
	}
	s.exitStatus = make(chan error, 1)
	go func() {
		s.exitStatus <- s.wait(reqs)
	}()

	return s, nil
}

// An ExitError reports unsuccessful completion of a remote command.
type ExitError struct {
	Waitmsg
}

func (e *ExitError) Error() string {
	return e.Waitmsg.String()
}

// Waitmsg stores the information about an exited remote command
// as reported by Wait.
type Waitmsg struct {
	status int
	signal string
	msg    string
	lang   string
}

// ExitStatus returns the exit status of the remote command.
func (w Waitmsg) ExitStatus() int {
	return w.status
}

// Signal returns the exit signal of the remote command if
// it was terminated violently.
func (w Waitmsg) Signal() string {
	return w.signal
}

// Msg returns the exit message given by the remote command
func (w Waitmsg) Msg() string {
	return w.msg
}

// Lang returns the language tag. See RFC 3066
func (w Waitmsg) Lang() string {
	return w.lang
}

func (w Waitmsg) String() string {
	str := fmt.Sprintf("Process exited with status %v", w.status)
	if w.signal != "" {
		str += fmt.Sprintf(" from signal %v", w.signal)
	}
	if w.msg != "" {
		str += fmt.Sprintf(". Reason was: %v", w.msg)
	}
	return str
}
