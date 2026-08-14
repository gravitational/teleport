package web

import (
	"bytes"
	"context"
	"image/png"
	"net/http"
	"net/url"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/web"
)

type createEnrollPairingResponse struct {
	// State mirrors devicepb.EnrollPairingState with the ENROLL_PAIRING_STATE_
	// prefix stripped and lowercased ("awaiting_device", "awaiting_approval",
	// "approved").
	State string `json:"state"`
	// Token is the pairing token. The wizard sends it back with Approve/Deny to
	// target this specific pairing.
	Token string `json:"token"`
	// QRCode is the PNG-encoded QR image. Populated only when state is
	// "awaiting_device" - past that point the mobile app already has the token
	// and the wizard isn't displaying a QR.
	QRCode []byte `json:"qrCode,omitempty"`
}

// createEnrollPairingHandle creates an enroll pairing or returns an existing
// pairing for the calling user. This lets the Web UI send a single POST
// regardless of whether a pairing already exists, while letting the gRPC layer
// keep the strict RFD 153 Create contract.
func (p *Plugin) createEnrollPairingHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (any, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pairing, err := getOrCreateEnrollPairing(r.Context(), clt.DevicesClient())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp := createEnrollPairingResponse{
		State: enrollPairingStateToString(pairing.GetStatus().GetState()),
		Token: pairing.GetStatus().GetToken(),
	}
	// The Web UI needs the QR code only when the pairing is in this state.
	if pairing.GetStatus().GetState() == devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_AWAITING_DEVICE {
		qrPNG, err := enrollMobileDeviceQR(p.h.PublicProxyAddr(), pairing.GetStatus().GetToken())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resp.QRCode = qrPNG
	}
	return resp, nil
}

// getOrCreateEnrollPairing tries to create a new EnrollPairing for the caller,
// falling back to GetCurrentEnrollPairing when one already exists.
func getOrCreateEnrollPairing(ctx context.Context, client devicepb.DeviceTrustServiceClient) (*devicepb.EnrollPairing, error) {
	createResp, err := client.CreateEnrollPairing(ctx, &devicepb.CreateEnrollPairingRequest{})
	if err == nil {
		return createResp.GetEnrollPairing(), nil
	}
	if !trace.IsAlreadyExists(err) {
		return nil, trace.Wrap(err, "creating enroll pairing")
	}
	getResp, err := client.GetCurrentEnrollPairing(ctx, &devicepb.GetCurrentEnrollPairingRequest{})
	if err != nil {
		return nil, trace.Wrap(err, "getting current enroll pairing")
	}
	return getResp.GetEnrollPairing(), nil
}

type getEnrollPairingResponse struct {
	// State mirrors devicepb.EnrollPairingState with the ENROLL_PAIRING_STATE_
	// prefix stripped and lowercased ("awaiting_device", "awaiting_approval",
	// "approved").
	State string `json:"state"`
	// Token is the pairing token. The wizard sends it back with Approve/Deny to
	// target this specific pairing.
	Token string `json:"token"`
}

// getEnrollPairingHandle returns the current enroll pairing for the calling
// user. It deliberately omits the QR code. The wizard gets the QR code already
// from the Create response. Re-generating it on every poll from the Web UI
// would require some sort of caching.
func (p *Plugin) getEnrollPairingHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (any, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := clt.DevicesClient().GetCurrentEnrollPairing(r.Context(), &devicepb.GetCurrentEnrollPairingRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pairing := resp.GetEnrollPairing()
	return getEnrollPairingResponse{
		State: enrollPairingStateToString(pairing.GetStatus().GetState()),
		Token: pairing.GetStatus().GetToken(),
	}, nil
}

func enrollPairingStateToString(s devicepb.EnrollPairingState) string {
	switch s {
	case devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_AWAITING_DEVICE:
		return "awaiting_device"
	case devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_AWAITING_APPROVAL:
		return "awaiting_approval"
	case devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_APPROVED:
		return "approved"
	default:
		return s.String()
	}
}

// enrollMobileDeviceQR encodes the teleport:// deep link the mobile app reads
// as a 456x456 PNG-encoded QR code.
func enrollMobileDeviceQR(proxyHost, token string) ([]byte, error) {
	deepLink := url.URL{
		Scheme:   "teleport",
		Host:     proxyHost,
		Path:     "/enroll_mobile_device",
		RawQuery: url.Values{"enroll_pairing_token": {token}}.Encode(),
	}
	code, err := qr.Encode(deepLink.String(), qr.M, qr.Auto)
	if err != nil {
		return nil, trace.Wrap(err, "encoding QR code")
	}
	code, err = barcode.Scale(code, 456, 456)
	if err != nil {
		return nil, trace.Wrap(err, "scaling QR code")
	}
	var out bytes.Buffer
	if err := png.Encode(&out, code); err != nil {
		return nil, trace.Wrap(err, "writing QR code as PNG")
	}
	return out.Bytes(), nil
}
