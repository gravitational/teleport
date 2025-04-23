/*
Copyright 2021 Gravitational, Inc.

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

package prompt

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	urlpkg "net/url"
	"os"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/term"
)

func TestContextReader(t *testing.T) {
	pr, pw := io.Pipe()
	t.Cleanup(func() { pr.Close() })
	t.Cleanup(func() { pw.Close() })

	write := func(t *testing.T, s string) {
		t.Helper()
		_, err := pw.Write([]byte(s))
		assert.NoError(t, err, "Write failed")
	}

	ctx := context.Background()
	cr := NewContextReader(pr)

	t.Run("simple read", func(t *testing.T) {
		go write(t, "hello")
		buf, err := cr.ReadContext(ctx)
		require.NoError(t, err)
		require.Equal(t, "hello", string(buf))
	})

	t.Run("reclaim abandoned read", func(t *testing.T) {
		done := make(chan struct{})
		cancelCtx, cancel := context.WithCancel(ctx)
		go func() {
			time.Sleep(1 * time.Millisecond) // give ReadContext time to block
			cancel()
			write(t, "after cancel")
			close(done)
		}()
		buf, err := cr.ReadContext(cancelCtx)
		require.ErrorIs(t, err, context.Canceled)
		require.Empty(t, buf)

		<-done // wait for write
		buf, err = cr.ReadContext(ctx)
		require.NoError(t, err)
		require.Equal(t, "after cancel", string(buf))
	})

	t.Run("close ContextReader", func(t *testing.T) {
		go func() {
			time.Sleep(1 * time.Millisecond) // give ReadContext time to block
			assert.NoError(t, cr.Close(), "Close errored")
		}()
		_, err := cr.ReadContext(ctx)
		require.ErrorIs(t, err, ErrReaderClosed)

		// Subsequent reads fail.
		_, err = cr.ReadContext(ctx)
		require.ErrorIs(t, err, ErrReaderClosed)

		// Ongoing read after Close is dropped.
		write(t, "unblock goroutine")
		buf, err := cr.ReadContext(ctx)
		assert.ErrorIs(t, err, ErrReaderClosed)
		assert.Empty(t, buf, "buf not empty")

		// Multiple closes are fine.
		assert.NoError(t, cr.Close(), "2nd Close failed")
	})

	// Re-creating is safe because the tests above leave no "pending" reads.
	cr = NewContextReader(pr)

	t.Run("close underlying reader", func(t *testing.T) {
		go func() {
			write(t, "before close")
			pw.CloseWithError(io.EOF)
		}()

		// Read the last chunk of data successfully.
		buf, err := cr.ReadContext(ctx)
		require.NoError(t, err)
		require.Equal(t, "before close", string(buf))

		// Next read fails because underlying reader is closed.
		buf, err = cr.ReadContext(ctx)
		require.ErrorIs(t, err, io.EOF)
		require.Empty(t, buf)
	})
}

func TestContextReader_ReadPassword(t *testing.T) {
	pr, pw := io.Pipe()
	write := func(t *testing.T, s string) {
		t.Helper()
		_, err := pw.Write([]byte(s))
		assert.NoError(t, err, "Write failed")
	}

	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0666)
	require.NoError(t, err, "Failed to open %v", os.DevNull)
	defer devNull.Close()

	term := &fakeTerm{reader: pr}
	cr := NewContextReader(pr)
	cr.term = term
	cr.fd = int(devNull.Fd()) // arbitrary, doesn't matter because term functions are mocked.

	ctx := context.Background()
	t.Run("read password", func(t *testing.T) {
		const want = "llama45"
		go write(t, want)

		got, err := cr.ReadPassword(ctx)
		require.NoError(t, err, "ReadPassword failed")
		assert.Equal(t, want, string(got), "ReadPassword mismatch")
	})

	t.Run("intertwine reads", func(t *testing.T) {
		const want1 = "hello, world"
		go write(t, want1)
		got, err := cr.ReadPassword(ctx)
		require.NoError(t, err, "ReadPassword failed")
		assert.Equal(t, want1, string(got), "ReadPassword mismatch")

		const want2 = "goodbye, world"
		go write(t, want2)
		got, err = cr.ReadContext(ctx)
		require.NoError(t, err, "ReadContext failed")
		assert.Equal(t, want2, string(got), "ReadContext mismatch")
	})

	t.Run("password read turned clean", func(t *testing.T) {
		require.False(t, term.restoreCalled, "restoreCalled sanity check failed")

		// Give ReadPassword time to block.
		cancelCtx, cancel := context.WithTimeout(ctx, 1*time.Millisecond)
		defer cancel()
		got, err := cr.ReadPassword(cancelCtx)
		require.ErrorIs(t, err, context.DeadlineExceeded, "ReadPassword returned unexpected error")
		require.Empty(t, got, "ReadPassword mismatch")

		// Reclaim as clean read.
		const want = "abandoned pwd read"
		go func() {
			// Once again, give ReadContext time to block.
			// This way we force a restore.
			time.Sleep(1 * time.Millisecond)
			write(t, want)
		}()
		got, err = cr.ReadContext(ctx)
		require.NoError(t, err, "ReadContext failed")
		assert.Equal(t, want, string(got), "ReadContext mismatch")
		assert.True(t, term.restoreCalled, "term.Restore not called")
	})

	t.Run("Close", func(t *testing.T) {
		require.NoError(t, cr.Close(), "Close errored")

		_, err := cr.ReadPassword(ctx)
		require.ErrorIs(t, err, ErrReaderClosed, "ReadPassword returned unexpected error")
	})
}

func TestNotifyExit_restoresTerminal(t *testing.T) {
	oldStdin := Stdin()
	t.Cleanup(func() { SetStdin(oldStdin) })

	pr, _ := io.Pipe()

	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0666)
	require.NoError(t, err, "Failed to open %v", os.DevNull)
	defer devNull.Close()

	term := &fakeTerm{reader: pr}
	ctx := context.Background()

	tests := []struct {
		name        string
		doRead      func(ctx context.Context, cr *ContextReader) error
		wantRestore bool
	}{
		{
			name: "no pending read",
			doRead: func(ctx context.Context, cr *ContextReader) error {
				<-ctx.Done()
				return ctx.Err()
			},
		},
		{
			name: "pending clean read",
			doRead: func(ctx context.Context, cr *ContextReader) error {
				_, err := cr.ReadContext(ctx)
				return err
			},
		},
		{
			name: "pending password read",
			doRead: func(ctx context.Context, cr *ContextReader) error {
				_, err := cr.ReadPassword(ctx)
				return err
			},
			wantRestore: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			term.restoreCalled = false // reset state between tests

			cr := NewContextReader(pr)
			cr.term = term
			cr.fd = int(devNull.Fd()) // arbitrary
			SetStdin(cr)

			// Give the read time to block.
			ctx, cancel := context.WithTimeout(ctx, 1*time.Millisecond)
			defer cancel()
			err := test.doRead(ctx, cr)
			require.ErrorIs(t, err, context.DeadlineExceeded, "unexpected read error")

			NotifyExit() // closes Stdin
			assert.Equal(t, test.wantRestore, term.restoreCalled, "term.Restore mismatch")
		})
	}
}

type fakeTerm struct {
	reader        io.Reader
	restoreCalled bool
}

func (t *fakeTerm) GetState(fd int) (*term.State, error) {
	return &term.State{}, nil
}

func (t *fakeTerm) IsTerminal(fd int) bool {
	return true
}

func (t *fakeTerm) ReadPassword(fd int) ([]byte, error) {
	const bufLen = 1024 // arbitrary, big enough for test data
	data := make([]byte, bufLen)
	n, err := t.reader.Read(data)
	data = data[:n]
	return data, err
}

func (t *fakeTerm) Restore(fd int, oldState *term.State) error {
	t.restoreCalled = true
	return nil
}

func TestURL(t *testing.T) {
	pr, pw := io.Pipe()
	t.Cleanup(func() { pr.Close() })
	t.Cleanup(func() { pw.Close() })
	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	}))
	t.Cleanup(httpSrv.Close)

	write := func(t *testing.T, s string) {
		t.Helper()
		_, err := pw.Write([]byte(s))
		assert.NoError(t, err, "Write failed")
	}

	ctx := context.Background()
	cr := NewContextReader(pr)

	t.Run("simple read", func(t *testing.T) {
		go write(t, httpSrv.URL)
		out := &bytes.Buffer{}
		gotURL, err := URL(ctx, out, cr, "Enter URL")
		require.NoError(t, err)
		require.Equal(t, httpSrv.URL, gotURL)
		require.Equal(t, "Enter URL: ", out.String())
	})

	t.Run("read with validator", func(t *testing.T) {
		go write(t, httpSrv.URL)
		out := &bytes.Buffer{}

		gotURL, err := URL(ctx, out, cr, "Enter URL", WithURLValidator(func(u *urlpkg.URL) error {
			rspBody, err := doHTTPCall(ctx, u.String())
			if err != nil {
				return trace.Wrap(err)
			}
			if rspBody != "hello" {
				return trace.BadParameter("unexpected response body: %q", rspBody)
			}
			return nil
		}))
		require.NoError(t, err)
		require.Equal(t, httpSrv.URL, gotURL)
		require.Equal(t, "Enter URL: ", out.String())
	})

	t.Run("read with failed validator", func(t *testing.T) {
		go write(t, httpSrv.URL)
		out := &bytes.Buffer{}

		_, err := URL(ctx, out, cr, "Enter URL", WithURLValidator(func(u *urlpkg.URL) error {
			rspBody, err := doHTTPCall(ctx, u.String())
			if err != nil {
				return trace.Wrap(err)
			}
			if rspBody != "notHello" {
				return trace.BadParameter("unexpected response body: %q", rspBody)
			}
			return nil
		}))
		require.Error(t, err)
	})
}

func doHTTPCall(ctx context.Context, u string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", trace.Wrap(err)
	}
	rsp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer rsp.Body.Close()
	defer io.Copy(io.Discard, rsp.Body)
	b, err := io.ReadAll(rsp.Body)
	return string(b), trace.Wrap(err)
}
