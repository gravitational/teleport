package s3io

import "io"

type ReaderWithDummyCloser struct{ io.Reader }

func (d ReaderWithDummyCloser) Close() error { return nil }

type NoOpReadCloser struct{}

func (d NoOpReadCloser) Read(b []byte) (n int, err error) { return 0, io.EOF }

func (d NoOpReadCloser) Close() error { return nil }
