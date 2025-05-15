package listener

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryListenerClient(t *testing.T) {
	var wg sync.WaitGroup
	expectedMessage := "hello from server"

	listener := InNewMemoryListener()
	t.Cleanup(func() { listener.Close() })

	wg.Add(1)
	go func() {
		defer wg.Done()

		conn, err := listener.Accept()
		if err != nil {
			return
		}
		t.Cleanup(func() { conn.Close() })

		_, _ = conn.Write([]byte(expectedMessage))
	}()

	// To avoid blocking in case the server is not working correctly, wrap
	// the client connection into a eventually loop.
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		conn, err := listener.DialContext(t.Context(), "", "")
		require.NoError(collect, err)

		buf := make([]byte, len(expectedMessage))
		n, err := conn.Read(buf[0:])
		require.Equal(collect, len(expectedMessage), n)
		require.Equal(collect, expectedMessage, string(buf[:n]))
	}, 50*time.Millisecond, 10*time.Millisecond)

	require.Eventually(t, func() bool {
		wg.Wait()
		return true
	}, 50*time.Millisecond, 10*time.Millisecond)
}

func TestMemoryListenerServer(t *testing.T) {
	var wg sync.WaitGroup
	expectedMessage := "hello from client"

	listener := InNewMemoryListener()
	t.Cleanup(func() { listener.Close() })

	wg.Add(1)
	go func() {
		defer wg.Done()

		conn, err := listener.DialContext(t.Context(), "", "")
		if err != nil {
			return
		}

		_, _ = conn.Write([]byte(expectedMessage))
	}()

	// To avoid blocking in case the client is not working correctly, wrap
	// the server accept connection into a eventually loop.
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		conn, err := listener.Accept()
		require.NoError(collect, err)

		buf := make([]byte, len(expectedMessage))
		n, err := conn.Read(buf[0:])
		require.Equal(collect, len(expectedMessage), n)
		require.Equal(collect, expectedMessage, string(buf[:n]))
	}, 50*time.Millisecond, 10*time.Millisecond)

	// Close the listener and expect subsequent accept calls to return error.
	listener.Close()
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		_, err := listener.Accept()
		require.Error(collect, err)
		require.ErrorIs(collect, err, io.EOF)
	}, 50*time.Millisecond, 10*time.Millisecond)

	require.Eventually(t, func() bool {
		wg.Wait()
		return true
	}, 50*time.Millisecond, 10*time.Millisecond)
}

func TestMemoryListenerDialTimeout(t *testing.T) {
	listener := InNewMemoryListener()
	t.Cleanup(func() { listener.Close() })

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		_, err := listener.DialContext(ctx, "", "")
		require.Error(collect, err)
		require.ErrorIs(collect, err, context.DeadlineExceeded)
	}, 100*time.Millisecond, 10*time.Millisecond)

	require.Never(t, func() bool {
		_, _ = listener.Accept()
		return true
	}, 50*time.Millisecond, 10*time.Millisecond, "expected server to not have received connections")
}
