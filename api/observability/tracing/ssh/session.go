package ssh

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/observability/tracing"
)

const (
	env          = "env"
	ptyReq       = "pty-req"
	subsystem    = "subsystem"
	windowChange = "window-change"
	signal       = "signal"
	exec         = "exec"
	shell        = "shell"
)

type Session struct {
	*ssh.Session
	wrapper    *clientWrapper
	capability tracingCapability
	opts       []tracing.Option
}

func (s *Session) SendRequest(ctx context.Context, name string, wantReply bool, payload []byte) (bool, error) {
	config := tracing.NewConfig(s.wrapper.opts)
	tracer := config.TracerProvider.Tracer(instrumentationName)

	ctx, span := tracer.Start(
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

	return s.Session.SendRequest(name, wantReply, wrapPayload(ctx, s.capability, config.TextMapPropagator, payload))
}

func (s *Session) Setenv(ctx context.Context, name, value string) error {
	s.wrapper.addContext(env, ctx)

	config := tracing.NewConfig(s.wrapper.opts)
	tracer := config.TracerProvider.Tracer(instrumentationName)
	ctx, span := tracer.Start(
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

	return s.Session.Setenv(name, value)
}

func (s *Session) RequestPty(ctx context.Context, term string, h, w int, termmodes ssh.TerminalModes) error {
	s.wrapper.addContext(ptyReq, ctx)

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

	return s.Session.RequestPty(term, h, w, termmodes)
}

func (s *Session) RequestSubsystem(ctx context.Context, subsystem string) error {
	s.wrapper.addContext(subsystem, ctx)

	config := tracing.NewConfig(s.wrapper.opts)
	tracer := config.TracerProvider.Tracer(instrumentationName)
	ctx, span := tracer.Start(
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

	return s.Session.RequestSubsystem(subsystem)
}

func (s *Session) WindowChange(ctx context.Context, h, w int) error {
	s.wrapper.addContext(windowChange, ctx)

	config := tracing.NewConfig(s.wrapper.opts)
	tracer := config.TracerProvider.Tracer(instrumentationName)
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("ssh.WindowChange"),
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

	return s.Session.WindowChange(h, w)
}

func (s *Session) Signal(ctx context.Context, sig ssh.Signal) error {
	s.wrapper.addContext(signal, ctx)

	config := tracing.NewConfig(s.wrapper.opts)
	tracer := config.TracerProvider.Tracer(instrumentationName)
	ctx, span := tracer.Start(
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

	return s.Session.Signal(sig)
}

func (s *Session) Start(ctx context.Context, cmd string) error {
	s.wrapper.addContext(exec, ctx)

	config := tracing.NewConfig(s.wrapper.opts)
	tracer := config.TracerProvider.Tracer(instrumentationName)
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("ssh.Start/%s", cmd),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String("ssh.Session"),
			semconv.RPCMethodKey.String("Start"),
			semconv.RPCSystemKey.String("ssh"),
		),
	)
	defer span.End()

	return s.Session.Start(cmd)
}

func (s *Session) Shell(ctx context.Context) error {
	s.wrapper.addContext(shell, ctx)

	config := tracing.NewConfig(s.wrapper.opts)
	tracer := config.TracerProvider.Tracer(instrumentationName)
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("ssh.Shell"),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String("ssh.Session"),
			semconv.RPCMethodKey.String("Shell"),
			semconv.RPCSystemKey.String("ssh"),
		),
	)
	defer span.End()

	return s.Session.Shell()
}

func (s *Session) Run(ctx context.Context, cmd string) error {
	s.wrapper.addContext(exec, ctx)

	config := tracing.NewConfig(s.wrapper.opts)
	tracer := config.TracerProvider.Tracer(instrumentationName)
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("ssh.Run/%s", cmd),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String("ssh.Session"),
			semconv.RPCMethodKey.String("Run"),
			semconv.RPCSystemKey.String("ssh"),
		),
	)
	defer span.End()

	return s.Session.Run(cmd)
}

func (s *Session) Output(ctx context.Context, cmd string) ([]byte, error) {
	s.wrapper.addContext(exec, ctx)

	config := tracing.NewConfig(s.wrapper.opts)
	tracer := config.TracerProvider.Tracer(instrumentationName)
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("ssh.Output/%s", cmd),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String("ssh.Session"),
			semconv.RPCMethodKey.String("Output"),
			semconv.RPCSystemKey.String("ssh"),
		),
	)
	defer span.End()

	return s.Session.Output(cmd)
}

func (s *Session) CombinedOutput(ctx context.Context, cmd string) ([]byte, error) {
	s.wrapper.addContext(exec, ctx)

	config := tracing.NewConfig(s.wrapper.opts)
	tracer := config.TracerProvider.Tracer(instrumentationName)
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("ssh.CombinedOutput/%s", cmd),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String("ssh.Session"),
			semconv.RPCMethodKey.String("CombinedOutput"),
			semconv.RPCSystemKey.String("ssh"),
		),
	)
	defer span.End()

	return s.Session.CombinedOutput(cmd)
}

type sessionChannel struct {
	ssh.Channel
	wrapper *clientWrapper
}

func (s sessionChannel) SendRequest(name string, wantReply bool, payload []byte) (bool, error) {
	ctx := s.wrapper.nextContext(name)

	config := tracing.NewConfig(s.wrapper.opts)
	tracer := config.TracerProvider.Tracer(instrumentationName)

	ctx, span := tracer.Start(
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

	ok, err := s.Channel.SendRequest(name, wantReply, wrapPayload(ctx, s.wrapper.capability, config.TextMapPropagator, payload))
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}

	return ok, err
}
