// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ssh

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/observability/tracing"
)

// Session is a wrapper around ssh.Session that adds tracing support
type Session struct {
	*ssh.Session
	wrapper *clientWrapper
}

// SendRequest sends an out-of-band channel request on the SSH channel
// underlying the session.
func (s *Session) SendRequest(ctx context.Context, name string, wantReply bool, payload []byte) (bool, error) {
	config := tracing.NewConfig(s.wrapper.opts)
	ctx, span := config.TracerProvider.Tracer(instrumentationName).Start(
		ctx,
		fmt.Sprintf("ssh.SessionRequest/%s", name),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.Bool("want_reply", wantReply),
			semconv.RPCServiceKey.String("ssh.Session"),
			semconv.RPCMethodKey.String("SendRequest"),
			semconv.RPCSystemKey.String("ssh"),
		),
	)
	defer span.End()

	// no need to wrap payload here, the session's channel wrapper will do it for us
	s.wrapper.addContext(ctx, name)
	return s.Session.SendRequest(name, wantReply, payload)
}

// Setenv sets an environment variable that will be applied to any
// command executed by Shell or Run.
func (s *Session) Setenv(ctx context.Context, name, value string) error {
	const request = "env"
	config := tracing.NewConfig(s.wrapper.opts)
	ctx, span := config.TracerProvider.Tracer(instrumentationName).Start(
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

	s.wrapper.addContext(ctx, request)
	return s.Session.Setenv(name, value)
}

// RequestPty requests the association of a pty with the session on the remote host.
func (s *Session) RequestPty(ctx context.Context, term string, h, w int, termmodes ssh.TerminalModes) error {
	const request = "pty-req"
	config := tracing.NewConfig(s.wrapper.opts)
	tracer := config.TracerProvider.Tracer(instrumentationName)
	ctx, span := tracer.Start(
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

	s.wrapper.addContext(ctx, request)
	return s.Session.RequestPty(term, h, w, termmodes)
}

// RequestSubsystem requests the association of a subsystem with the session on the remote host.
// A subsystem is a predefined command that runs in the background when the ssh session is initiated.
func (s *Session) RequestSubsystem(ctx context.Context, subsystem string) error {
	const request = "subsystem"
	config := tracing.NewConfig(s.wrapper.opts)
	ctx, span := config.TracerProvider.Tracer(instrumentationName).Start(
		ctx,
		fmt.Sprintf("ssh.RequestSubsystem/%s", subsystem),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String("ssh.Session"),
			semconv.RPCMethodKey.String("SendRequest"),
			semconv.RPCSystemKey.String("ssh"),
		),
	)
	defer span.End()

	s.wrapper.addContext(ctx, request)
	return s.Session.RequestSubsystem(subsystem)
}

// WindowChange informs the remote host about a terminal window dimension change to h rows and w columns.
func (s *Session) WindowChange(ctx context.Context, h, w int) error {
	const request = "window-change"
	config := tracing.NewConfig(s.wrapper.opts)
	ctx, span := config.TracerProvider.Tracer(instrumentationName).Start(
		ctx,
		"ssh.WindowChange",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String("ssh.Session"),
			semconv.RPCMethodKey.String("SendRequest"),
			semconv.RPCSystemKey.String("ssh"),
			attribute.Int("height", h),
			attribute.Int("width", w),
		),
	)
	defer span.End()

	s.wrapper.addContext(ctx, request)
	return s.Session.WindowChange(h, w)
}

// Signal sends the given signal to the remote process.
// sig is one of the SIG* constants.
func (s *Session) Signal(ctx context.Context, sig ssh.Signal) error {
	const request = "signal"
	config := tracing.NewConfig(s.wrapper.opts)
	ctx, span := config.TracerProvider.Tracer(instrumentationName).Start(
		ctx,
		fmt.Sprintf("ssh.Signal/%s", sig),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String("ssh.Session"),
			semconv.RPCMethodKey.String("SendRequest"),
			semconv.RPCSystemKey.String("ssh"),
		),
	)
	defer span.End()

	s.wrapper.addContext(ctx, request)
	return s.Session.Signal(sig)
}

// Start runs cmd on the remote host. Typically, the remote
// server passes cmd to the shell for interpretation.
// A Session only accepts one call to Run, Start or Shell.
func (s *Session) Start(ctx context.Context, cmd string) error {
	const request = "exec"
	config := tracing.NewConfig(s.wrapper.opts)
	ctx, span := config.TracerProvider.Tracer(instrumentationName).Start(
		ctx,
		fmt.Sprintf("ssh.Start/%s", cmd),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String("ssh.Session"),
			semconv.RPCMethodKey.String("SendRequest"),
			semconv.RPCSystemKey.String("ssh"),
		),
	)
	defer span.End()

	s.wrapper.addContext(ctx, request)
	return s.Session.Start(cmd)
}

// Shell starts a login shell on the remote host. A Session only
// accepts one call to Run, Start, Shell, Output, or CombinedOutput.
func (s *Session) Shell(ctx context.Context) error {
	const request = "shell"
	config := tracing.NewConfig(s.wrapper.opts)
	ctx, span := config.TracerProvider.Tracer(instrumentationName).Start(
		ctx,
		"ssh.Shell",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String("ssh.Session"),
			semconv.RPCMethodKey.String("SendRequest"),
			semconv.RPCSystemKey.String("ssh"),
		),
	)
	defer span.End()

	s.wrapper.addContext(ctx, request)
	return s.Session.Shell()
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
	const request = "exec"
	config := tracing.NewConfig(s.wrapper.opts)
	ctx, span := config.TracerProvider.Tracer(instrumentationName).Start(
		ctx,
		fmt.Sprintf("ssh.Run/%s", cmd),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String("ssh.Session"),
			semconv.RPCMethodKey.String("SendRequest"),
			semconv.RPCSystemKey.String("ssh"),
		),
	)
	defer span.End()

	s.wrapper.addContext(ctx, request)
	return s.Session.Run(cmd)
}

// Output runs cmd on the remote host and returns its standard output.
func (s *Session) Output(ctx context.Context, cmd string) ([]byte, error) {
	const request = "exec"
	config := tracing.NewConfig(s.wrapper.opts)
	ctx, span := config.TracerProvider.Tracer(instrumentationName).Start(
		ctx,
		fmt.Sprintf("ssh.Output/%s", cmd),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String("ssh.Session"),
			semconv.RPCMethodKey.String("SendRequest"),
			semconv.RPCSystemKey.String("ssh"),
		),
	)
	defer span.End()

	s.wrapper.addContext(ctx, request)
	return s.Session.Output(cmd)
}

// CombinedOutput runs cmd on the remote host and returns its combined
// standard output and standard error.
func (s *Session) CombinedOutput(ctx context.Context, cmd string) ([]byte, error) {
	const request = "exec"
	config := tracing.NewConfig(s.wrapper.opts)
	ctx, span := config.TracerProvider.Tracer(instrumentationName).Start(
		ctx,
		fmt.Sprintf("ssh.CombinedOutput/%s", cmd),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String("ssh.Session"),
			semconv.RPCMethodKey.String("SendRequest"),
			semconv.RPCSystemKey.String("ssh"),
		),
	)
	defer span.End()

	s.wrapper.addContext(ctx, request)
	return s.Session.CombinedOutput(cmd)
}
