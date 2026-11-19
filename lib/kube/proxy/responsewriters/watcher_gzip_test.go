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

package responsewriters

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
)

// syncRecorder is a flushable http.ResponseWriter that is safe to read while the
// watcher writes into it from its goroutine.
type syncRecorder struct {
	mu     sync.Mutex
	header http.Header
	body   bytes.Buffer
	code   int
}

func (r *syncRecorder) Header() http.Header { return r.header }
func (r *syncRecorder) WriteHeader(code int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.code = code
}
func (r *syncRecorder) Write(b []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.body.Write(b)
}
func (r *syncRecorder) Flush() {}
func (r *syncRecorder) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.body.String()
}

// gzipMember compresses one flush of a watch stream as its own gzip member, as
// kube-apiserver's perFlushGzipWriter does with WatchListCompression.
func gzipMember(t *testing.T, s string) []byte {
	t.Helper()
	var b bytes.Buffer
	zw := gzip.NewWriter(&b)
	if _, err := zw.Write([]byte(s)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func waitFor(t *testing.T, rec *syncRecorder, want string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(rec.String(), want) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %q; client got so far:\n%s", want, rec.String())
}

func TestWatcherResponseWriterGzippedWatchList(t *testing.T) {
	negotiator := runtime.NewClientNegotiator(scheme.Codecs.WithoutConversion(), schema.GroupVersion{Version: "v1"})
	rec := &syncRecorder{header: http.Header{}}
	w, err := NewWatcherResponseWriter(rec, negotiator, nil)
	if err != nil {
		t.Fatal(err)
	}
	// What kube-apiserver 1.37 sends for a watch-list request from a client that accepts gzip.
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Encoding", "gzip")
	w.WriteHeader(http.StatusOK)

	added := `{"type":"ADDED","object":{"kind":"ConfigMap","apiVersion":"v1","metadata":{"name":"a","namespace":"ns","resourceVersion":"10"}}}` + "\n"
	bookmark := `{"type":"BOOKMARK","object":{"kind":"ConfigMap","apiVersion":"v1","metadata":{"resourceVersion":"11","annotations":{"k8s.io/initial-events-end":"true"}}}}` + "\n"
	modified := `{"type":"MODIFIED","object":{"kind":"ConfigMap","apiVersion":"v1","metadata":{"name":"a","namespace":"ns","resourceVersion":"12"}}}` + "\n"

	// Each flush is a separate gzip member. Every event must reach the client before the
	// next member is sent: a watch can stay idle for minutes after the initial-events bookmark.
	for _, step := range []struct{ event, want string }{
		{added, `"resourceVersion":"10"`},
		{bookmark, `"k8s.io/initial-events-end":"true"`},
		{modified, `"resourceVersion":"12"`},
	} {
		if _, err := w.Write(gzipMember(t, step.event)); err != nil {
			t.Fatal(err)
		}
		waitFor(t, rec, step.want)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Errorf("Content-Encoding = %q, want none: the events are re-encoded uncompressed", got)
	}
	var types []string
	for _, line := range strings.Split(strings.TrimSpace(rec.String()), "\n") {
		var evt struct{ Type string }
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			t.Fatalf("client got a line that isn't JSON: %q: %v", line, err)
		}
		types = append(types, evt.Type)
	}
	if want := []string{"ADDED", "BOOKMARK", "MODIFIED"}; strings.Join(types, ",") != strings.Join(want, ",") {
		t.Errorf("events = %v, want %v", types, want)
	}
}

func TestWatcherResponseWriterUncompressed(t *testing.T) {
	negotiator := runtime.NewClientNegotiator(scheme.Codecs.WithoutConversion(), schema.GroupVersion{Version: "v1"})
	rec := &syncRecorder{header: http.Header{}}
	w, err := NewWatcherResponseWriter(rec, negotiator, nil)
	if err != nil {
		t.Fatal(err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(`{"type":"ADDED","object":{"kind":"ConfigMap","apiVersion":"v1","metadata":{"name":"a","namespace":"ns","resourceVersion":"10"}}}` + "\n")); err != nil {
		t.Fatal(err)
	}
	waitFor(t, rec, `"resourceVersion":"10"`)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}
