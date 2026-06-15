/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package proxy

import (
	"context"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"strings"
	"sync"

	"github.com/gravitational/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/gravitational/teleport/lib/kube/proxy/responsewriters"
	"github.com/gravitational/teleport/lib/kube/proxy/streamfilter"
)

// filteringResponseWriter is an http.ResponseWriter that intercepts the upstream response,
// inspects headers to decide between streaming and buffered filtering, and routes the body accordingly.
type filteringResponseWriter struct {
	target  http.ResponseWriter
	headers http.Header
	status  int
	body    io.Writer
	once    sync.Once

	matcher         resourceMatcher
	filterWrapper   responsewriters.FilterWrapper
	log             *slog.Logger
	ctx             context.Context
	tracer          oteltrace.Tracer
	kubeServiceType string

	pipeWriter *io.PipeWriter
	memBuffer  *responsewriters.MemoryResponseWriter
	filterErr  error
	filterDone chan struct{} // closed when the streaming goroutine finishes
	streaming  bool
}

func newFilteringResponseWriter(
	target http.ResponseWriter,
	matcher resourceMatcher,
	filterWrapper responsewriters.FilterWrapper,
	log *slog.Logger,
	ctx context.Context,
	tracer oteltrace.Tracer,
	kubeServiceType string,
) *filteringResponseWriter {
	return &filteringResponseWriter{
		target:          target,
		headers:         make(http.Header),
		matcher:         matcher,
		filterWrapper:   filterWrapper,
		log:             log,
		ctx:             ctx,
		tracer:          tracer,
		kubeServiceType: kubeServiceType,
	}
}

func (fw *filteringResponseWriter) Header() http.Header {
	return fw.headers
}

func (fw *filteringResponseWriter) WriteHeader(statusCode int) {
	fw.once.Do(func() {
		fw.status = statusCode

		if statusCode == http.StatusOK {
			if fw.tryStreaming() {
				return
			}
		}

		fw.memBuffer = responsewriters.NewMemoryResponseWriter()
		maps.Copy(fw.memBuffer.Header(), fw.headers)
		fw.memBuffer.WriteHeader(statusCode)
		fw.body = fw.memBuffer.Buffer()
	})
}

// tryStreaming attempts to set up the streaming filter path.
// Returns true if streaming was activated, false to fall back to buffered.
func (fw *filteringResponseWriter) tryStreaming() bool {
	contentType := responsewriters.GetContentTypeHeader(fw.headers)
	if !strings.Contains(contentType, "application/json") {
		return false
	}
	sf := streamfilter.NewJSONFilter(fw.matcher, fw.log)

	contentEncoding := fw.headers.Get("Content-Encoding")
	if contentEncoding != "" && contentEncoding != "identity" && contentEncoding != "gzip" {
		fw.log.WarnContext(fw.ctx, "Unexpected Content-Encoding, falling back to buffered filter", "content_encoding", contentEncoding)
		return false
	}

	maps.Copy(fw.target.Header(), fw.headers)
	fw.target.Header().Del("Content-Length")
	fw.target.WriteHeader(fw.status)

	pr, pw := io.Pipe()
	fw.pipeWriter = pw
	fw.streaming = true
	fw.filterDone = make(chan struct{})
	fw.body = pw

	// wrapContentEncoding is called inside the goroutine because gzip.NewReader
	// reads the gzip header from the pipe, which blocks until Write provides data.
	// Calling it here in WriteHeader would deadlock.
	go func() {
		defer close(fw.filterDone)
		// Close the read end of the pipe when done to unblock any pending
		// pipeWriter.Write in ServeHTTP (e.g. if the filter exits early
		// due to a client disconnect).
		defer pr.Close()
		src, dst, err := wrapContentEncoding(pr, fw.target, contentEncoding)
		if err != nil {
			fw.filterErr = trace.ConnectionProblem(err, "failed to initialize content encoding wrapper for %q", contentEncoding)
			fw.log.ErrorContext(fw.ctx, "Streaming filter content encoding setup failed", "error", fw.filterErr)
			return
		}
		filterErr := sf.Filter(src, dst)
		fw.filterErr = trace.NewAggregate(filterErr, dst.Close(), src.Close())
		if fw.filterErr != nil {
			fw.log.ErrorContext(fw.ctx, "Streaming filter failed mid-write, client received truncated response", "error", fw.filterErr)
		}
	}()
	return true
}

func (fw *filteringResponseWriter) Write(b []byte) (int, error) {
	fw.WriteHeader(http.StatusOK)
	return fw.body.Write(b)
}

// Finish completes filtering and returns the status code and any error.
func (fw *filteringResponseWriter) Finish() (int, error) {
	if fw.status == 0 {
		return http.StatusBadGateway, trace.ConnectionProblem(nil, "upstream closed without response")
	}

	if fw.streaming {
		fw.pipeWriter.Close()
		<-fw.filterDone
		return fw.status, fw.filterErr
	}

	_, filterSpan := fw.tracer.Start(fw.ctx, "kube.Forwarder/listResourcesList/filterBuffer",
		oteltrace.WithSpanKind(oteltrace.SpanKindServer),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String(fw.kubeServiceType),
			semconv.RPCSystemKey.String("kube"),
		),
	)
	err := filterBuffer(fw.filterWrapper, fw.memBuffer)
	filterSpan.End()
	if err != nil {
		return fw.memBuffer.Status(), trace.Wrap(err)
	}

	err = fw.memBuffer.CopyInto(fw.target)
	return fw.memBuffer.Status(), trace.Wrap(err)
}
