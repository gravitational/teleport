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
	"github.com/gravitational/teleport/lib/devicetrust"
	"github.com/gravitational/teleport/lib/httplib"
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
	// Device describes the device asking to enroll. Populated once the mobile app
	// has claimed the pairing through CreatePairedDeviceEnrollToken, so that the
	// wizard can name the device the user is about to approve.
	Device *enrollPairingDevice `json:"device,omitempty"`
}

// enrollPairingDevice is the JSON shape of [devicepb.EnrollPairingDevice].
type enrollPairingDevice struct {
	// OSType is the friendly OS name, e.g. "iOS".
	OSType string `json:"osType"`
	// SerialNumber is the device serial number.
	SerialNumber string `json:"serialNumber"`
	// OSVersion is the OS version number, without the leading 'v', e.g. "26.3.1".
	OSVersion string `json:"osVersion"`
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
		return nil, p.rewrapEnrollPairingNotFound(r.Context(), err)
	}
	pairing := resp.GetEnrollPairing()
	return getEnrollPairingResponse{
		State:  enrollPairingStateToString(pairing.GetStatus().GetState()),
		Token:  pairing.GetStatus().GetToken(),
		Device: enrollPairingDeviceFromProto(pairing.GetStatus().GetDevice()),
	}, nil
}

func enrollPairingDeviceFromProto(device *devicepb.EnrollPairingDevice) *enrollPairingDevice {
	if device == nil {
		return nil
	}
	return &enrollPairingDevice{
		OSType:       devicetrust.FriendlyOSType(device.GetOsType()),
		SerialNumber: device.GetSerialNumber(),
		OSVersion:    device.GetOsVersion(),
	}
}

// approveEnrollPairingHandle approves the current enroll pairing for the
// calling user, which lets the mobile app retrieve its enrollment token.
func (p *Plugin) approveEnrollPairingHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (any, error) {
	req, err := readEnrollPairingActionRequest(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, err = clt.DevicesClient().ApproveEnrollPairing(r.Context(), devicepb.ApproveEnrollPairingRequest_builder{
		PairingToken: req.Token,
	}.Build())
	if err != nil {
		return nil, p.rewrapEnrollPairingNotFound(r.Context(), err)
	}
	return web.OK(), nil
}

// denyEnrollPairingHandle denies the current enroll pairing for the calling
// user by deleting it.
func (p *Plugin) denyEnrollPairingHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (any, error) {
	req, err := readEnrollPairingActionRequest(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, err = clt.DevicesClient().DenyEnrollPairing(r.Context(), devicepb.DenyEnrollPairingRequest_builder{
		PairingToken: req.Token,
	}.Build())
	if err != nil {
		return nil, p.rewrapEnrollPairingNotFound(r.Context(), err)
	}
	return web.OK(), nil
}

// enrollPairingActionRequest is the body of the approve and deny handlers.
type enrollPairingActionRequest struct {
	// Token is the pairing token the wizard was displaying, so that the user
	// cannot act on a pairing other than the one they saw. Otherwise if the
	// pairing expired while the wizard was open and a new one was created, the
	// user could approve the new one unknowingly.
	Token string `json:"token"`
}

func readEnrollPairingActionRequest(r *http.Request) (*enrollPairingActionRequest, error) {
	var req enrollPairingActionRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	if req.Token == "" {
		return nil, trace.BadParameter("missing token")
	}
	return &req, nil
}

// rewrapEnrollPairingNotFound replaces the NotFound message from the RPC which
// is written for API clients. The kind must stay NotFound as the wizard relies
// on the 404 status to show the denied-or-expired state.
func (p *Plugin) rewrapEnrollPairingNotFound(ctx context.Context, err error) error {
	if trace.IsNotFound(err) {
		p.Logger.DebugContext(ctx, "Rewrapping enroll pairing NotFound error", "error", err)
		return trace.NotFound("the enrollment request no longer exists")
	}
	return trace.Wrap(err)
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
