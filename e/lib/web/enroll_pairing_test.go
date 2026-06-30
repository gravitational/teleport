package web

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateEnrollPairing(t *testing.T) {
	t.Parallel()
	s := newWebSuiteForDeviceTrust(t)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		webPack := s.newAuthWebPack(t, "alice")
		endpoint := webPack.clt.Endpoint("enterprise", "devices", "enroll_pairing")

		resp, err := webPack.clt.PostJSON(s.ctx, endpoint, nil)
		require.NoError(t, err)

		var got createEnrollPairingResponse
		require.NoError(t, json.Unmarshal(resp.Bytes(), &got))
		assert.Equal(t, "awaiting_device", got.State)
		assert.True(t,
			bytes.HasPrefix(got.QRCode, []byte("\x89PNG\r\n\x1a\n")),
			"qrCode does not start with a PNG signature")
	})

	t.Run("consecutive request returns existing pairing", func(t *testing.T) {
		t.Parallel()
		webPack := s.newAuthWebPack(t, "bob")
		endpoint := webPack.clt.Endpoint("enterprise", "devices", "enroll_pairing")

		resp1, err := webPack.clt.PostJSON(s.ctx, endpoint, nil)
		require.NoError(t, err)
		var first createEnrollPairingResponse
		require.NoError(t, json.Unmarshal(resp1.Bytes(), &first))

		resp2, err := webPack.clt.PostJSON(s.ctx, endpoint, nil)
		require.NoError(t, err)
		var second createEnrollPairingResponse
		require.NoError(t, json.Unmarshal(resp2.Bytes(), &second))

		assert.Equal(t, first.Token, second.Token)
		assert.Equal(t, first.QRCode, second.QRCode)
		assert.Equal(t, first.State, second.State)
	})
}

func TestGetCurrentEnrollPairing(t *testing.T) {
	t.Parallel()
	s := newWebSuiteForDeviceTrust(t)

	t.Run("returns existing pairing", func(t *testing.T) {
		t.Parallel()
		webPack := s.newAuthWebPack(t, "alice")
		endpoint := webPack.clt.Endpoint("enterprise", "devices", "enroll_pairing")

		_, err := webPack.clt.PostJSON(s.ctx, endpoint, nil)
		require.NoError(t, err)

		getResp, err := webPack.clt.Get(s.ctx, endpoint, nil)
		require.NoError(t, err)
		var got getEnrollPairingResponse
		require.NoError(t, json.Unmarshal(getResp.Bytes(), &got))
		assert.Equal(t, "awaiting_device", got.State)
		// The requests to approve or deny an enroll pairing target a specific
		// pairing, so the get endpoint must return the token.
		assert.NotEmpty(t, got.Token)
	})

	t.Run("returns NotFound when no pairing exists", func(t *testing.T) {
		t.Parallel()
		webPack := s.newAuthWebPack(t, "bob")
		endpoint := webPack.clt.Endpoint("enterprise", "devices", "enroll_pairing")

		_, err := webPack.clt.Get(s.ctx, endpoint, nil)
		require.ErrorAs(t, err, new(*trace.NotFoundError))
	})
}
