package devicetrustpublicv1

import (
	"context"
	"errors"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	devicetrustpublicv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/public/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
)

// enrollDeviceStreamAdapter presents the public EnrollDevice stream as the
// private one, so the enrollment ceremony from devicetrustv1 runs unchanged.
// iOS payloads ride in the macOS fields. Their contents are identical, both
// platforms enroll a Secure Enclave key. See RFD 32e.
type enrollDeviceStreamAdapter struct {
	devicetrustpublicv1pb.DeviceTrustService_EnrollDeviceServer

	// init is the already-read init message, replayed on the first Recv. The
	// handler consumes it from the stream to resolve the token user before the
	// ceremony starts.
	//
	// Recv is not safe for concurrent use, like the stream it wraps, so init
	// needs no lock.
	init *devicetrustpublicv1pb.EnrollDeviceInit
}

func (a *enrollDeviceStreamAdapter) Recv() (*devicepb.EnrollDeviceRequest, error) {
	if init := a.init; init != nil {
		a.init = nil
		return devicepb.EnrollDeviceRequest_builder{
			Init: devicepb.EnrollDeviceInit_builder{
				Token:        init.GetToken(),
				CredentialId: init.GetCredentialId(),
				DeviceData:   init.GetDeviceData(),
				Macos: devicepb.MacOSEnrollPayload_builder{
					PublicKeyDer: init.GetIos().GetPublicKeyDer(),
				}.Build(),
			}.Build(),
		}.Build(), nil
	}

	req, err := a.DeviceTrustService_EnrollDeviceServer.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if resp := req.GetIosChallengeResponse(); resp != nil {
		return devicepb.EnrollDeviceRequest_builder{
			MacosChallengeResponse: devicepb.MacOSEnrollChallengeResponse_builder{
				Signature: resp.GetSignature(),
			}.Build(),
		}.Build(), nil
	}
	return nil, trace.BadParameter("bad payload, expected IOSEnrollChallengeResponse")
}

func (a *enrollDeviceStreamAdapter) Send(resp *devicepb.EnrollDeviceResponse) error {
	switch {
	case resp.GetMacosChallenge() != nil:
		return trace.Wrap(a.DeviceTrustService_EnrollDeviceServer.Send(
			devicetrustpublicv1pb.EnrollDeviceResponse_builder{
				IosChallenge: devicetrustpublicv1pb.IOSEnrollChallenge_builder{
					Challenge: resp.GetMacosChallenge().GetChallenge(),
				}.Build(),
			}.Build(),
		))
	case resp.GetSuccess() != nil:
		return trace.Wrap(a.DeviceTrustService_EnrollDeviceServer.Send(
			devicetrustpublicv1pb.EnrollDeviceResponse_builder{
				Success: devicetrustpublicv1pb.EnrollDeviceSuccess_builder{
					Device: resp.GetSuccess().GetDevice(),
				}.Build(),
			}.Build(),
		))
	default:
		return trace.Errorf("enrollment ceremony produced a %q response the public stream cannot carry, this is a bug",
			resp.WhichPayload())
	}
}

// runWithStreamTimeout wraps stream in a [streamWithTimeout], calls fn with the
// wrapper, and returns timeoutErr if the timeout fires before fn returns. It is
// meant to be the whole body of a bidi RPC handler, with fn doing the actual
// work.
//
// A blocked Recv or Send on a gRPC server stream is only released by returning
// from the RPC handler that called runWithStreamTimeout, so fn runs in its own
// goroutine and runWithStreamTimeout returns as soon as the timeout fires. gRPC
// then cancels stream.Context, which fails the pending Recv or Send and lets fn
// and the goroutine exit.
//
// fn is given a child of stream.Context with the timeout attached, since gRPC
// owns stream.Context and a timeout can only be added by deriving from it. The
// child ends any calls fn makes with it at the timeout, and its cause tells the
// timeout apart from a client that disconnected, in the select below as much as
// in the wrapper's Recv. Canceling the child does not release a blocked
// stream.Recv, only canceling stream.Context does, hence the goroutine above.
func runWithStreamTimeout[Req, Resp any](
	stream grpc.BidiStreamingServer[Req, Resp],
	timeout time.Duration,
	timeoutErr error,
	fn func(grpc.BidiStreamingServer[Req, Resp]) error,
) error {
	ctx, cancel := context.WithTimeoutCause(stream.Context(), timeout, timeoutErr)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- fn(&streamWithTimeout[Req, Resp]{
			wrappedStream: stream,
			ctx:           ctx,
			timeoutErr:    timeoutErr,
		})
	}()
	// This select decides what the client sees as the error.
	select {
	case <-ctx.Done():
		return trace.Wrap(context.Cause(ctx))
	case err := <-errCh:
		return trace.Wrap(err)
	}
}

// streamWithTimeout wraps a gRPC stream. fn run by [runWithStreamTimeout]
// receives the wrapper in place of the gRPC stream.
type streamWithTimeout[Req, Resp any] struct {
	wrappedStream[Req, Resp]
	ctx        context.Context
	timeoutErr error
}

// wrappedStream names the gRPC stream embedded in [streamWithTimeout].
type wrappedStream[Req, Resp any] = grpc.BidiStreamingServer[Req, Resp]

// Context is the context that streamWithTimeout hands to fn passed to
// [runWithStreamTimeout].
func (s *streamWithTimeout[Req, Resp]) Context() context.Context {
	return s.ctx
}

// Recv reports timeoutErr where the wrapped stream reports a cancellation. The
// timeout cancels only the child context that Context returns, not the wrapped
// stream, whose pending Recv keeps blocking. What ends the RPC is
// runWithStreamTimeout returning, upon which gRPC cancels the wrapped stream
// and fails that Recv with a cancellation error.
func (s *streamWithTimeout[Req, Resp]) Recv() (*Req, error) {
	req, err := s.wrappedStream.Recv()
	if err != nil {
		// The return from this branch decides what fn sees as the error.
		//
		// Without the translation, fn would see the cancellation as the reason the
		// call failed, instead of the timeout that caused it. Cancellation by the
		// client and the timeout need to stay distinct, hence returning the cause
		// here.
		if cause := context.Cause(s.ctx); errors.Is(cause, s.timeoutErr) {
			return nil, trace.Wrap(cause)
		}
		return nil, trace.Wrap(err)
	}
	return req, nil
}
