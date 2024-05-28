/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package responsewriters

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"sync"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/streaming"
	"k8s.io/apimachinery/pkg/watch"
	restclientwatch "k8s.io/client-go/rest/watch"
)

const (
	// DefaultContentTypeHeader is the default content type header used by the Kubernetes API
	ContentTypeHeader = "Content-Type"
	// JSONContentType is the JSON content type used by the Kubernetes API
	JSONContentType = "application/json"
	// YAMLContentType is the YAML content type used by the Kubernetes API
	YAMLContentType = "application/yaml"
	// DefaultContentType is the default content type used by the Kubernetes API
	DefaultContentType = JSONContentType
)

// WatcherResponseWriter satisfies the http.ResponseWriter interface and
// once the server writes the headers and response code spins a goroutine
// that parses each event frame, decodes it and analyzes if the user
// is allowed to receive the events for that pod.
// If the user is not allowed, the event is ignored.
// If allowed, the event is encoded into the user's response.
type WatcherResponseWriter struct {
	// mtx protects target and ensures writes happen sequentially.
	mtx sync.Mutex
	// target is the user response writer.
	// everything written will be received by the user.
	target http.ResponseWriter
	// status holds the response code status for logging purposes.
	status int
	// group is the errorgroup used by the spinning goroutine.
	group errgroup.Group
	// pipeReader and pipeWriter are synchronous memory pipes used to forward
	// events written from the upstream server to the routine that decodes
	// them and validates if the event should be forward downstream.
	pipeReader *io.PipeReader
	pipeWriter *io.PipeWriter
	// negotiator is the client negotiator used to select the serializers based
	// on response content-type.
	negotiator runtime.ClientNegotiator
	// filter hold the filtering rules to filter events.
	filter FilterWrapper
	// evtsChan is used to send fake events to target connection.
	evtsChan chan *watch.Event
	// closeChanGuard guards the closeChan close call
	closeChanGuard sync.Once
	// closeChan indicates if the watcher as been closed.
	closeChan chan struct{}
}

// NewWatcherResponseWriter creates a new WatcherResponseWriter.
func NewWatcherResponseWriter(
	target http.ResponseWriter,
	negotiator runtime.ClientNegotiator,
	filter FilterWrapper,
) (*WatcherResponseWriter, error) {
	if err := checkWatcherRequiredFields(target, negotiator); err != nil {
		return nil, trace.Wrap(err)
	}
	reader, writer := io.Pipe()
	return &WatcherResponseWriter{
		target:     target,
		pipeReader: reader,
		pipeWriter: writer,
		negotiator: negotiator,
		filter:     filter,
		closeChan:  make(chan struct{}),
		evtsChan:   make(chan *watch.Event, 10),
	}, nil
}

// checkWatcherRequiredFields checks if the target response writer and negotiator are
// defined.
func checkWatcherRequiredFields(target http.ResponseWriter, negotiator runtime.ClientNegotiator) error {
	if target == nil {
		return trace.BadParameter("missing target ResponseWriter")
	}
	if negotiator == nil {
		return trace.BadParameter("missing negotiator")
	}
	return nil
}

// Write writes buf into the pipeWriter.
func (w *WatcherResponseWriter) Write(buf []byte) (int, error) {
	return w.pipeWriter.Write(buf)
}

// Header returns the target headers.
func (w *WatcherResponseWriter) Header() http.Header {
	return w.target.Header()
}

// PushVirtualEventToClient pushes a Teleport generated event to the target connection.
// It's consumed by a goroutine spawn by watchDecoder.
func (w *WatcherResponseWriter) PushVirtualEventToClient(ctx context.Context, evt *watch.Event) {
	select {
	case <-ctx.Done():
		return
	case <-w.closeChan:
		return
		// wait until we can push the evts
	case w.evtsChan <- evt:
	}
}

// WriteHeader writes the status code and headers into the target http.ResponseWriter
// and spins a go-routine that will wait for events received in w.pipeReader
// and analyze if they must be forwarded to target.
func (w *WatcherResponseWriter) WriteHeader(code int) {
	w.status = code
	w.target.WriteHeader(code)
	contentType := GetContentTypeHeader(w.Header())
	w.group.Go(
		func() error {
			switch {
			case code == http.StatusSwitchingProtocols:
				// no-op, we've been upgraded
				return nil
			case code < http.StatusOK /* 200 */ || code > http.StatusPartialContent /* 206 */ :
				// If code is bellow 200 (OK) or higher than 206 (PartialContent), it means that
				// Kubernetes returned an error response which does not contain watch events.
				// In that case, it is safe to write it back to target and return early.
				// Some valid cases:
				// - user does not have the `watch` permission.
				// - API is unable to serve the request.
				// Logic from: https://github.com/kubernetes/client-go/blob/58ff029093df37cad9fa28778a37f11fa495d9cf/rest/request.go#L1040
				_, err := io.Copy(w.target, w.pipeReader)
				return trace.Wrap(err)
			default:
				err := w.watchDecoder(contentType, w.target, w.pipeReader)
				return trace.Wrap(err)
			}
		},
	)
}

// Status returns the http status response.
func (w *WatcherResponseWriter) Status() int {
	return w.getStatus()
}

func (w *WatcherResponseWriter) getStatus() int {
	// http.ResponseWriter implicitly sets StatusOK, if WriteHeader hasn't been
	// explicitly called.
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

// Close closes the reader part of the pipe with io.EOF and waits until
// the spinned goroutine terminates.
// After closes the writer pipe and flushes the response into target.
func (w *WatcherResponseWriter) Close() error {
	w.closeChanGuard.Do(func() {
		close(w.closeChan)
	})
	w.pipeReader.CloseWithError(io.EOF)
	err := w.group.Wait()
	w.pipeWriter.CloseWithError(io.EOF)
	w.Flush()
	return trace.Wrap(err)
}

// Flush flushes the response into the target and returns.
func (w *WatcherResponseWriter) Flush() {
	w.mtx.Lock()
	defer w.mtx.Unlock()

	w.flushLocked()
}

func (w *WatcherResponseWriter) flushLocked() {
	if flusher, ok := w.target.(http.Flusher); ok {
		flusher.Flush()
	}
}

// watchDecoder waits for events written into w.pipeWriter and decodes them.
// Once decoded, it checks if the user is allowed to watch the events for that pod
// and ignores or forwards them downstream depending on the result.
func (w *WatcherResponseWriter) watchDecoder(contentType string, writer io.Writer, reader io.ReadCloser) error {
	// parse mime type.
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return trace.Wrap(err)
	}
	// create a stream decoder based on mediaType.s
	objectDecoder, streamingSerializer, framer, err := w.negotiator.StreamDecoder(mediaType, params)
	if err != nil {
		return trace.Wrap(err)
	}
	// create a encoder to encode filtered requests to the user.
	encoder, err := w.negotiator.Encoder(mediaType, params)
	if err != nil {
		return trace.Wrap(err)
	}
	// create a frameReader that waits until the Kubernetes API sends the full
	// event frame.
	frameReader := framer.NewFrameReader(reader)
	defer frameReader.Close()
	// create a frameWriter that writes event frames into the user's connection.
	frameWriter := framer.NewFrameWriter(writer)
	// streamingDecoder is the decoder that parses metav1.WatchEvents from the
	// long-lived connection.
	streamingDecoder := streaming.NewDecoder(frameReader, streamingSerializer)
	defer streamingDecoder.Close()
	// create encoders
	watchEventEncoder := streaming.NewEncoder(frameWriter, streamingSerializer)
	watchEncoder := restclientwatch.NewEncoder(watchEventEncoder, encoder)
	// instantiate filterObj if available.
	var filter FilterObj
	if w.filter != nil {
		filter, err = w.filter(contentType, w.getStatus())
		if err != nil {
			return trace.Wrap(err)
		}
	}

	// watchEncoderGuard prevents multiple sources to push events at the same time.
	// only one source is able to push and flush events to ensure proper json formatting.
	writeEventAndFlush := func(evt *watch.Event) error {
		w.mtx.Lock()
		defer w.mtx.Unlock()
		// encode the event into the target connection.
		if err := watchEncoder.Encode(evt); err != nil {
			return trace.Wrap(err)
		}
		// Stream the response into the target connection, as we are dealing with
		// streaming events. However, the Kubernetes API does not include the
		// content-type as chunked. As a result, the forwarder is unaware that
		// the connection is chunked and delays the response writing by buffering
		// to minimize the number of writes.
		// In cases where the connection stream is busy with events, the user may
		// not receive individual events as chunks, leading to incomplete data.
		// This could result in the user receiving malformed JSON and triggering
		// an abort.
		// To avoid this, we flush the response after each event to ensure that
		// the user receives the event as a chunk.
		w.flushLocked()
		return nil
	}

	w.group.Go(func() error {
		for {
			select {
			case evt := <-w.evtsChan:
				if err := writeEventAndFlush(evt); err != nil {
					slog.WarnContext(context.Background(), "error pushing fake pod event", "err", err)
				}
			case <-w.closeChan:
				return nil
			}
		}
	})
	// wait for events received from upstream until the connection is terminated.
	for {
		eventType, obj, err := w.decodeStreamingMessage(streamingDecoder, objectDecoder)
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) {
			return nil
		} else if err != nil {
			return trace.Wrap(err)
		}

		switch obj.(type) {
		case *metav1.Status:
			// Status object is returned when the Kubernetes API returns an error and
			// should be forwarded to the user.
			// If eventType is empty, it means that status was returned without event.
			if eventType == "" {
				err = encoder.Encode(obj, writer)
				return trace.Wrap(err)
			}
			err := writeEventAndFlush(
				&watch.Event{
					Type:   eventType,
					Object: obj,
				},
			)
			return trace.Wrap(err)
		default:
			if filter != nil {
				// check if the event object matches the filtering criteria.
				// If it does not match, ignore the event.
				publish, _, err := filter.FilterObj(obj)
				if err != nil {
					return trace.Wrap(err)
				}
				if !publish {
					continue
				}
			}

			if err := writeEventAndFlush(
				&watch.Event{
					Type:   eventType,
					Object: obj,
				},
			); err != nil {
				return trace.Wrap(err)
			}
		}
	}
}

// Decode blocks until it can return the next object in the reader. Returns an error
// if the reader is closed or an object can't be decoded.
// decodeStreamingMessage blocks until it can return the next object in the reader.
// Returns an error if the reader is closed or an object can't be decoded.
func (w *WatcherResponseWriter) decodeStreamingMessage(
	streamDecoder streaming.Decoder,
	embeddedEncoder runtime.Decoder,
) (watch.EventType, runtime.Object, error) {
	var event metav1.WatchEvent
	res, gvk, err := streamDecoder.Decode(nil, &event)
	if err != nil {
		return "", nil, err
	}
	if gvk != nil {
		res.GetObjectKind().SetGroupVersionKind(*gvk)
	}
	switch res.(type) {
	case *metav1.Status:
		// Status object is returned when the Kubernetes API returns an error and
		// should be forwarded to the user.
		return "", res, nil
	default:
		switch watch.EventType(event.Type) {
		case watch.Added, watch.Modified, watch.Deleted, watch.Error, watch.Bookmark:
		default:
			return "", nil, trace.BadParameter("got invalid watch event type: %v", event.Type)
		}
		obj, gvk, err := embeddedEncoder.Decode(event.Object.Raw, nil /* defaults */, nil /* into */)
		if err != nil {
			return "", nil, trace.Wrap(err)
		}
		if gvk != nil {
			obj.GetObjectKind().SetGroupVersionKind(*gvk)
		}
		return watch.EventType(event.Type), obj, nil
	}
}
