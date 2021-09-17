package web

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetOriginatingIPAddress(t *testing.T) {
	r := &http.Request{
		Header: http.Header{},
	}

	// Test without x-forwarded-for set.
	r.RemoteAddr = "test:"
	addr, err := getIPAddress(r)
	require.NoError(t, err)
	require.Equal(t, "test", addr)

	// Test with x-forwarded-for set.
	r.Header.Set("X-FORWARDED-FOR", "first, second, third")
	addr, err = getIPAddress(r)
	require.NoError(t, err)
	require.Equal(t, "first", addr)
}
