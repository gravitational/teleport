package web

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
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
		assert.ErrorIs(t, err, &trace.NotFoundError{Message: "the enrollment request no longer exists"})
	})

	t.Run("returns the device once the pairing is claimed", func(t *testing.T) {
		t.Parallel()
		webPack := s.newAuthWebPack(t, "carol")
		claimEnrollPairing(t, s, webPack)

		getResp, err := webPack.clt.Get(s.ctx,
			webPack.clt.Endpoint("enterprise", "devices", "enroll_pairing"), nil)
		require.NoError(t, err)

		var got getEnrollPairingResponse
		require.NoError(t, json.Unmarshal(getResp.Bytes(), &got))

		assert.Equal(t, "awaiting_approval", got.State)
		assert.Equal(t, &enrollPairingDevice{
			OSType:       "iOS",
			SerialNumber: "CXXXXXXXXX01",
			OSVersion:    "26.3.1",
		}, got.Device)
	})
}

func TestApproveEnrollPairing(t *testing.T) {
	t.Parallel()
	s := newWebSuiteForDeviceTrust(t)

	t.Run("approves the pairing", func(t *testing.T) {
		t.Parallel()
		webPack := s.newAuthWebPack(t, "alice")
		token := claimEnrollPairing(t, s, webPack)

		endpoint := webPack.clt.Endpoint("enterprise", "devices", "enroll_pairing", "approve")
		_, err := webPack.clt.PostJSON(s.ctx, endpoint, enrollPairingActionRequest{Token: token})
		require.NoError(t, err)

		getResp, err := webPack.clt.Get(s.ctx,
			webPack.clt.Endpoint("enterprise", "devices", "enroll_pairing"), nil)
		require.NoError(t, err)
		var got getEnrollPairingResponse
		require.NoError(t, json.Unmarshal(getResp.Bytes(), &got))
		assert.Equal(t, "approved", got.State)
	})

	t.Run("rejects a missing token", func(t *testing.T) {
		t.Parallel()
		webPack := s.newAuthWebPack(t, "bob")
		endpoint := webPack.clt.Endpoint("enterprise", "devices", "enroll_pairing", "approve")

		_, err := webPack.clt.PostJSON(s.ctx, endpoint, enrollPairingActionRequest{})
		assert.ErrorIs(t, err, &trace.BadParameterError{Message: "missing token"})
	})

	t.Run("rewraps NotFound on a token mismatch", func(t *testing.T) {
		t.Parallel()
		webPack := s.newAuthWebPack(t, "carol")
		claimEnrollPairing(t, s, webPack)

		endpoint := webPack.clt.Endpoint("enterprise", "devices", "enroll_pairing", "approve")
		_, err := webPack.clt.PostJSON(s.ctx, endpoint, enrollPairingActionRequest{Token: "some-token"})
		// The exact message match makes sure that the RPC-level error is
		// rewrapped.
		assert.ErrorIs(t, err, &trace.NotFoundError{Message: "the enrollment request no longer exists"})
	})
}

func TestDenyEnrollPairing(t *testing.T) {
	t.Parallel()
	s := newWebSuiteForDeviceTrust(t)

	t.Run("deletes the pairing", func(t *testing.T) {
		t.Parallel()
		webPack := s.newAuthWebPack(t, "alice")
		token := claimEnrollPairing(t, s, webPack)

		endpoint := webPack.clt.Endpoint("enterprise", "devices", "enroll_pairing", "deny")
		_, err := webPack.clt.PostJSON(s.ctx, endpoint, enrollPairingActionRequest{Token: token})
		require.NoError(t, err)

		_, err = webPack.clt.Get(s.ctx,
			webPack.clt.Endpoint("enterprise", "devices", "enroll_pairing"), nil)
		assert.ErrorIs(t, err, &trace.NotFoundError{Message: "the enrollment request no longer exists"})
	})

	t.Run("rewraps NotFound when no pairing exists", func(t *testing.T) {
		t.Parallel()
		webPack := s.newAuthWebPack(t, "bob")

		endpoint := webPack.clt.Endpoint("enterprise", "devices", "enroll_pairing", "deny")
		_, err := webPack.clt.PostJSON(s.ctx, endpoint, enrollPairingActionRequest{Token: "some-token"})
		// The exact message match makes sure that the RPC-level error is
		// rewrapped.
		assert.ErrorIs(t, err, &trace.NotFoundError{Message: "the enrollment request no longer exists"})
	})
}

// claimEnrollPairing creates a pairing through the web endpoint and advances it
// to AWAITING_APPROVAL the way the mobile app would, returning its token. The
// wizard only offers approve and deny from that state.
func claimEnrollPairing(t *testing.T, s *webSuite, webPack *authWebPack) string {
	t.Helper()
	resp, err := webPack.clt.PostJSON(s.ctx,
		webPack.clt.Endpoint("enterprise", "devices", "enroll_pairing"), nil)
	require.NoError(t, err)

	var created createEnrollPairingResponse
	require.NoError(t, json.Unmarshal(resp.Bytes(), &created))

	authServer := s.testAuthServer.Auth()
	pairing, err := authServer.GetEnrollPairingByToken(s.ctx, created.Token)
	require.NoError(t, err)

	_, err = authServer.RequestEnrollPairingApproval(s.ctx, pairing,
		devicepb.EnrollPairingDevice_builder{
			OsType:       devicepb.OSType_OS_TYPE_IOS,
			SerialNumber: "CXXXXXXXXX01",
			OsVersion:    "26.3.1",
		}.Build())
	require.NoError(t, err)

	return created.Token
}
