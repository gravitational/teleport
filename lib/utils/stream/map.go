package stream

import "github.com/gravitational/teleport/api/internalutils/stream"

// MapStreams performs simple type transformations of a stream with a callback.
type MapStreams[T, V any] struct {
	inner   stream.Stream[T]
	f       func(T) V
	current V
}

// NewMapStreams returns a new instance of MapStreams.
func NewMapStreams[T, V any](inner stream.Stream[T], f func(T) V) *MapStreams[T, V] {
	return &MapStreams[T, V]{
		inner: inner,
		f:     f,
	}
}

// Next attempts to advance the stream to the next item. If false is returned,
// then no more items are available. Next() and Item() must not be called after the
// first time Next() returns false.
func (m *MapStreams[T, V]) Next() bool {
	if !m.inner.Next() {
		return false
	}
	m.current = m.f(m.inner.Item())
	return true
}

// Item gets the current item. Invoking Item() is only safe if Next was previously
// invoked *and* returned true. Invoking Item() before invoking Next(), or after Next()
// returned false may cause panics or other unpredictable behavior. Whether or not the
// item returned is safe for access after the stream is advanced again is dependent
// on the implementation and should be documented (e.g. an I/O based stream might
// re-use an underlying buffer).
func (m *MapStreams[T, V]) Item() V {
	return m.current
}

// Done checks for any errors that occurred during streaming and informs the stream
// that we've finished consuming items from it. Invoking Next() or Item() after Done()
// has been called is not permitted. Done may trigger cleanup operations, but unlike Close()
// the error reported is specifically related to failures that occurred *during* streaming,
// meaning that if Done() returns an error, there is a high likelihood that the complete
// set of values was not observed. For this reason, Done() should always be checked explicitly
// rather than deferred as Close() might be.
func (m *MapStreams[T, V]) Done() error {
	return m.inner.Done()
}
