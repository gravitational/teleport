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

package alpnproxy

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestPingConnection(t *testing.T) {
	t.Run("BufferSize", func(t *testing.T) {
		nWrites := 10
		dataWritten := []byte("message")

		for _, tt := range []struct {
			desc    string
			bufSize int
		}{
			{desc: "Same", bufSize: len(dataWritten)},
			{desc: "Large", bufSize: len(dataWritten) * 2},
			{desc: "Short", bufSize: len(dataWritten) / 2},
		} {
			t.Run(tt.desc, func(t *testing.T) {
				r, w := makePingConn(t)

				// Write routine
				errChan := make(chan error, 2)
				go func() {
					defer w.Close()

					for i := 0; i < nWrites; i++ {
						// Eventually write some pings.
						if i%2 == 0 {
							err := w.WritePing()
							if err != nil {
								errChan <- err
								return
							}
						}

						_, err := w.Write(dataWritten)
						if err != nil {
							errChan <- err
							return
						}
					}

					errChan <- nil
				}()

				// Read routine.
				go func() {
					defer r.Close()

					buf := make([]byte, tt.bufSize)

					for i := 0; i < nWrites; i++ {
						var (
							aggregator []byte
							n          int
							err        error
						)

						for n < len(dataWritten) {
							n, err = r.Read(buf)
							if err != nil {
								errChan <- err
								return
							}

							aggregator = append(aggregator, buf[:n]...)
						}

						if !bytes.Equal(dataWritten, aggregator) {
							errChan <- fmt.Errorf("wrong content read, expected '%s', got '%s'", string(dataWritten), string(buf[:n]))
							return
						}
					}

					errChan <- nil
				}()

				// Expect routines to finish.
				timer := time.NewTimer(10 * time.Second)
				defer timer.Stop()
				for i := 0; i < 1; i++ {
					select {
					case err := <-errChan:
						require.NoError(t, err)
					case <-timer.C:
						require.Fail(t, "routing didn't finished in time")
					}
				}
			})
		}
	})

	t.Run("ConcurrentReads", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		nWrites := 10
		dataWritten := []byte("message")

		r, w := makePingConn(t)
		defer r.Close()
		defer w.Close()

		readChan := make(chan []byte)

		// Write routine
		go func() {
			for i := 0; i < nWrites; i++ {
				_, err := w.Write(dataWritten)
				if err != nil {
					return
				}
			}
		}()

		// Read routines.
		for i := 0; i < nWrites/2; i++ {
			go func() {
				buf := make([]byte, len(dataWritten)/2)
				for {
					n, err := r.Read(buf)
					if err != nil {
						return
					}

					chanBytes := make([]byte, n)
					copy(chanBytes, buf[:n])
					readChan <- chanBytes
				}
			}()
		}

	readLoop:
		for i := 0; i < nWrites; i++ {
			var aggregator []byte
			for {
				select {
				case <-ctx.Done():
					require.Fail(t, "failed to read message, expected '%s' but received '%s'", dataWritten, aggregator)
				case data := <-readChan:
					aggregator = append(aggregator, data...)
					if bytes.Equal(dataWritten, aggregator) {
						continue readLoop
					}
				}
			}
		}
	})

	t.Run("ConcurrentWrites", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		w, r := makeBufferedPingConn(t)
		defer w.Close()
		defer r.Close()

		nWrites := 10
		dataWritten := []byte("message")
		writeChan := make(chan error)

		// Start write routines.
		for i := 0; i < nWrites/2; i++ {
			go func() {
				for writes := 0; writes < 2; writes++ {
					err := w.WritePing()
					if err != nil {
						writeChan <- err
						return
					}

					_, err = w.Write(dataWritten)
					if err != nil {
						writeChan <- err
						return
					}
				}

				writeChan <- nil
			}()
		}

		// Expect all writes to succeed.
		for i := 0; i < nWrites/2; i++ {
			select {
			case <-ctx.Done():
				require.Fail(t, "timout write")
			case err := <-writeChan:
				require.NoError(t, err)
			}
		}

		// Read all messages.
		buf := make([]byte, len(dataWritten))
		for i := 0; i < nWrites; i++ {
			n, err := r.Read(buf)
			require.NoError(t, err)
			require.Equal(t, dataWritten, buf[:n])
		}
	})

	t.Run("LargeContent", func(t *testing.T) {
		if os.Getenv("TELEPORT_TEST_PING_CONN_LARGE") == "" {
			t.Skip()
		}

		t.Run("SingleRead", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			t.Cleanup(cancel)

			r, w := makePingConn(t)
			t.Cleanup(func() {
				r.Close()
				w.Close()
			})

			largeSlice := make([]byte, math.MaxUint32)
			errChan := make(chan error, 2)

			// Produce some data to it to match after reading.
			written, err := rand.Read(largeSlice)
			require.NoError(t, err)
			require.Equal(t, math.MaxUint32, written)

			// Write goroutine.
			go func() {
				_, err = w.Write(largeSlice)
				errChan <- err
			}()

			// Read goroutine.
			go func() {
				buf := make([]byte, math.MaxUint32)
				read, err := r.Read(buf)
				if err != nil {
					errChan <- fmt.Errorf("expected no read error but got %s", err)
					return
				}

				if read != math.MaxUint32 {
					errChan <- fmt.Errorf("expected to read %d but got %d", math.MaxUint32, read)
					return
				}

				if !bytes.Equal(largeSlice, buf) {
					errChan <- fmt.Errorf("expected to content to be the same")
					return
				}
			}()

			for i := 0; i < 1; i++ {
				select {
				case <-ctx.Done():
					require.Fail(t, "exceeded time to read/write message")
				case err := <-errChan:
					require.NoError(t, err)
				}
			}
		})

		t.Run("LargerThanUint32", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			errChan := make(chan error)
			_, w := makePingConn(t)
			defer w.Close()

			// Place into a goroutine to avoid freezing tests in case where the
			// write succeeds.
			go func() {
				largeSlice := make([]byte, math.MaxUint32+1)
				_, err := w.Write(largeSlice)
				errChan <- err
			}()

			select {
			case err := <-errChan:
				require.True(t, trace.IsBadParameter(err), "expected error to be `BadParameter` but got %T", err)
			case <-ctx.Done():
				require.Fail(t, "expected to receive the write error but nothing was received")
			}
		})
	})
}

// makePingConn creates a piped ping connection.
func makePingConn(t *testing.T) (*pingConn, *pingConn) {
	t.Helper()

	writer, reader := net.Pipe()
	return &pingConn{Conn: writer}, &pingConn{Conn: reader}
}

// makeBufferedPingConn creates connections to have asynchronous writes.
func makeBufferedPingConn(t *testing.T) (*pingConn, *pingConn) {
	t.Helper()

	fakeAddr := &net.TCPAddr{}
	bufA := new(bytes.Buffer)
	bufB := new(bytes.Buffer)

	connA := utils.NewPipeNetConn(bufA, bufB, io.NopCloser(bufA), fakeAddr, fakeAddr)
	connB := utils.NewPipeNetConn(bufB, bufA, io.NopCloser(bufB), fakeAddr, fakeAddr)

	return newPingConn(connA), newPingConn(connB)
}
