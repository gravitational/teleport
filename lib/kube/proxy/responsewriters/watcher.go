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

package responsewriters

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/streaming"
	"k8s.io/apimachinery/pkg/watch"
	restclientwatch "k8s.io/client-go/rest/watch"
)

const (
	ContentTypeHeader  = "Content-Type"
	DefaultContentType = "application/json"
)

type WatcherResponseWriter struct {
	target     http.ResponseWriter
	status     int
	group      errgroup.Group
	pipeReader *io.PipeReader
	pipeWriter *io.PipeWriter
	negotiator runtime.ClientNegotiator
	filter     FilterWrapper
}

func NewWatcherResponseWriter(target http.ResponseWriter, negotiator runtime.ClientNegotiator, filter FilterWrapper) *WatcherResponseWriter {
	reader, writer := io.Pipe()
	return &WatcherResponseWriter{
		target:     target,
		pipeReader: reader,
		pipeWriter: writer,
		negotiator: negotiator,
		filter:     filter,
	}
}

func (w *WatcherResponseWriter) Write(buf []byte) (int, error) {
	return w.pipeWriter.Write(buf)
}

func (w *WatcherResponseWriter) Header() http.Header {
	return w.target.Header()
}

func (w *WatcherResponseWriter) WriteHeader(code int) {
	w.status = code
	w.target.WriteHeader(code)
	contentType := GetContentHeader(w.Header())
	w.group.Go(
		func() error {
			switch {
			case code == http.StatusSwitchingProtocols:
				// no-op, we've been upgraded
				return nil
			case code < http.StatusOK || code > http.StatusPartialContent:
				_, err := io.Copy(w.target, w.pipeReader)
				return trace.Wrap(err)
			default:
				err := w.watchDecoder(contentType, w.pipeReader, w.target)
				return trace.Wrap(err)
			}
		},
	)
}

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

func (w *WatcherResponseWriter) Close() error {
	w.pipeReader.CloseWithError(io.EOF)
	err := w.group.Wait()
	w.pipeWriter.CloseWithError(io.EOF)
	w.Flush()
	return trace.Wrap(err)
}

func (w *WatcherResponseWriter) Flush() {
	if flusher, ok := w.target.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *WatcherResponseWriter) watchDecoder(contentType string, reader io.ReadCloser, writer io.Writer) error {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return trace.Wrap(err)
	}
	objectDecoder, streamingSerializer, framer, err := w.negotiator.StreamDecoder(mediaType, params)
	if err != nil {
		return trace.Wrap(err)
	}

	encoder, err := w.negotiator.Encoder(mediaType, params)
	if err != nil {
		return trace.Wrap(err)
	}

	frameReader := framer.NewFrameReader(reader)
	frameWriter := framer.NewFrameWriter(writer)
	streamingDecoder := streaming.NewDecoder(frameReader, streamingSerializer)
	defer streamingDecoder.Close()

	watchEventEncoder := streaming.NewEncoder(frameWriter, streamingSerializer)

	watchEncoder := restclientwatch.NewEncoder(watchEventEncoder, encoder)
	var filter FilterObj
	if w.filter != nil {
		filter, err = w.filter(contentType, w.getStatus())
		if err != nil {
			return trace.Wrap(err)
		}
	}
	for {
		eventType, obj, err := w.decodeStreamingMessage(streamingDecoder, objectDecoder)
		if errors.Is(err, io.EOF) {
			return nil
		} else if err != nil {
			return trace.Wrap(err)
		}

		switch obj.(type) {
		case *metav1.Status:
			err = encoder.Encode(obj, writer)
			return trace.Wrap(err)
		default:
			if filter != nil {
				publish, err := filter.FilterObj(obj)
				if err != nil {
					return trace.Wrap(err)
				}
				if !publish {
					continue
				}
			}
			err = watchEncoder.Encode(
				&watch.Event{
					Type:   eventType,
					Object: obj,
				},
			)
			if err != nil {
				return trace.Wrap(err)
			}
		}
	}
}

// Decode blocks until it can return the next object in the reader. Returns an error
// if the reader is closed or an object can't be decoded.
func (w *WatcherResponseWriter) decodeStreamingMessage(
	streamDecoder streaming.Decoder,
	embeededEncoder runtime.Decoder,
) (watch.EventType, runtime.Object, error) {
	var got metav1.WatchEvent
	res, gvk, err := streamDecoder.Decode(nil, &got)
	if err != nil {
		return "", nil, err
	}
	if gvk != nil {
		res.GetObjectKind().SetGroupVersionKind(*gvk)
	}
	switch res.(type) {
	case *metav1.Status:
		return "", res, nil
	default:
		switch got.Type {
		case string(watch.Added), string(watch.Modified), string(watch.Deleted), string(watch.Error), string(watch.Bookmark):
		default:
			return "", nil, fmt.Errorf("got invalid watch event type: %v", got.Type)
		}
		obj, gvk, err := embeededEncoder.Decode(got.Object.Raw, nil, nil)
		if err != nil {
			return "", nil, trace.Wrap(err)
		}
		if gvk != nil {
			obj.GetObjectKind().SetGroupVersionKind(*gvk)
		}
		return watch.EventType(got.Type), obj, nil
	}
}

func GetContentHeader(header http.Header) string {
	contentType := header.Get(ContentTypeHeader)
	if len(contentType) > 0 {
		return contentType
	}
	return DefaultContentType
}
