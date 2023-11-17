/*
Copyright 2023 Gravitational, Inc.

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

package app

import (
	"compress/gzip"
	"io"

	"github.com/gravitational/trace"
)

// newGzipCompressor creates a new gzip compressor that compresses data from
// dataReader and returns the compressed data as an io.ReadCloser.
// This compressor is used to compress the body of HTTP requests without having
// to read the entire body into memory after compression.
// The caller must call Close on the returned io.ReadCloser when done reading.
func newGzipCompressor(dataReader io.Reader) (io.ReadCloser, error) {
	// bodyReader is the body of the HTTP request, as an io.Reader.
	// httpWriter is the body of the HTTP request, as an io.Writer.
	bodyReader, httpWriter := io.Pipe()

	// gzipWriter compresses data to httpWriter.
	gzipWriter := gzip.NewWriter(httpWriter)

	// errch collects any errors from the writing goroutine.
	errch := make(chan error, 1)

	go func() {
		defer close(errch)
		sentErr := false
		sendErr := func(err error) {
			if !sentErr {
				errch <- err
				sentErr = true
			}
		}

		// Copy our data to gzipWriter, which compresses it to
		// gzipWriter, which feeds it to bodyReader.
		if _, err := io.Copy(gzipWriter, dataReader); err != nil && err != io.ErrClosedPipe {
			sendErr(err)
		}
		if err := gzipWriter.Close(); err != nil && err != io.ErrClosedPipe {
			sendErr(err)
		}
		if err := httpWriter.Close(); err != nil && err != io.ErrClosedPipe {
			sendErr(err)
		}
	}()
	return &gzipCompressorCloser{
		ReadCloser: bodyReader,
		errC:       errch,
	}, nil
}

type gzipCompressorCloser struct {
	io.ReadCloser
	errC chan error
}

func (c *gzipCompressorCloser) Close() error {
	err := c.ReadCloser.Close()
	return trace.NewAggregate(err, <-c.errC)
}
