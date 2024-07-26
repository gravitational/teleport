package utils

import (
	"errors"
	"io"

	"github.com/gravitational/trace"
)

// ClientReceiver is an interface for receiving messages from a gRPC stream.
type ClientReceiver[T any] interface {
	// Recv reads the next message from the stream.
	Recv() (T, error)
}

// ConsumeStreamToErrorIfEOF reads from the gRPC bi-directional stream until an error is encountered if
// the sendErr is io.EOF. If the sendErr is not io.EOF, it is returned immediately because the error is not
// from the server - it is a client error and server did not send any response yet which will cause Recv to block.
// This function should be used when the client encounters an error while sending a message to the stream
// and wants to surface the server's error.
// gRPC never returns the server's error when calling Send function, instead client has to call Recv to get the error.
// It might need to call Recv multiple times to get the error if client's buffer has other messages from server.
func ConsumeStreamToErrorIfEOF[T any](sendErr error, stream ClientReceiver[T]) error {
	// If the error is not EOF, return it immediately.
	if !errors.Is(sendErr, io.EOF) {
		return sendErr
	}
	for {
		_, err := stream.Recv()
		if err != nil {
			return trace.Wrap(err)
		}
	}
}
