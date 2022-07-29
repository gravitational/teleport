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

package pingconn

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPingConn(t *testing.T) {
	makePingConn := func() (*pingConn, *pingConn) {
		writer, reader := net.Pipe()
		return New(writer), New(reader)
	}

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
				r, w := makePingConn()

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

		r, w := makePingConn()
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

		// Create TCP connections to have asynchronous read/write.
		listener, err := net.Listen("tcp", "localhost:0")
		require.NoError(t, err)
		defer listener.Close()

		nWrites := 10
		dataWritten := []byte("message")
		writeChan := make(chan error)

		// Accept connection and make multiple writes.
		go func() {
			conn, err := listener.Accept()
			require.NoError(t, err)
			w := New(conn)

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
		}()

		// Dial connection.
		connChan := make(chan struct {
			net.Conn
			error
		})
		go func() {
			conn, err := net.Dial("tcp", listener.Addr().String())
			connChan <- struct {
				net.Conn
				error
			}{conn, err}
		}()

		var r net.Conn
		select {
		case <-ctx.Done():
			require.Fail(t, "failed to make connection")
		case res := <-connChan:
			require.NoError(t, res.error)
			r = New(res.Conn)
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
}
