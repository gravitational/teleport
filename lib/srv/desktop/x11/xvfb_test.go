package x11

import (
	"regexp"
	"syscall"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewXvfb(t *testing.T) {
	if !IsXvfbPresent() {
		t.Skip("this test requires Xvfb to be installed")
	}
	xvfb, err := NewXvfb(t.Context())
	require.NoError(t, err)
	require.NotNil(t, xvfb)

	matched, err := regexp.MatchString(":[0-9]+", xvfb.Display)
	assert.NoError(t, err)
	assert.True(t, matched)

	err = xvfb.Close()
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		err = xvfb.cmd.Process.Signal(syscall.Signal(0))
		return err != nil
	}, 5*time.Second, 100*time.Millisecond, "waiting for process to exit")
}

func TestResize(t *testing.T) {
	xvfb, err := NewXvfb(t.Context())
	require.NoError(t, err)
	err = xvfb.Resize(9000, 200)
	require.True(t, trace.IsBadParameter(err))
	err = xvfb.Resize(2000, 9200)
	require.True(t, trace.IsBadParameter(err))
	err = xvfb.Resize(1000, 1000)
	require.NoError(t, err)
}
