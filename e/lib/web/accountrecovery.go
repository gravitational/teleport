package web

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/api/cloud"
	v1 "github.com/gravitational/teleport/e/api/cloud/v1"
	enterpriseui "github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/auth"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/web"
	"github.com/gravitational/teleport/lib/web/ui"
)

// getAccountRecoveryTokenHandle retrieves a recovery token.
// If the recovery token type is approved, also returns a new qr code per retrieval.
func (p *Plugin) getAccountRecoveryTokenHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, client cloud.Client) (interface{}, error) {
	token, err := p.h.GetProxyClient().GetAccountRecoveryToken(r.Context(), &proto.GetAccountRecoveryTokenRequest{
		RecoveryTokenID: params.ByName("token"),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uiToken := enterpriseui.RecoveryToken{
		User:              token.GetUser(),
		IsRecoverPassword: token.GetUsage() == types.UserTokenUsage_USER_TOKEN_RECOVER_PASSWORD,
		IsApproved:        token.GetSubKind() == auth.UserTokenTypeRecoveryApproved,
		TokenID:           token.GetName(),
	}

	// Return qrcode to register a new TOTP device.
	if uiToken.IsApproved && !uiToken.IsRecoverPassword {
		res, err := p.h.GetProxyClient().CreateRegisterChallenge(r.Context(), &proto.CreateRegisterChallengeRequest{
			TokenID:    token.GetName(),
			DeviceType: proto.DeviceType_DEVICE_TYPE_TOTP,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		uiToken.QRCode = res.GetTOTP().GetQRCode()
	}

	return uiToken, nil
}

type startAccountRecoveryRequest struct {
	// Username is user's username.
	Username string `json:"username"`
	// RecoveryCode is one of the user's recovery code.
	RecoveryCode string `json:"recoveryCode"`
	// IsRecoverPassword is a flag that indicates if user requested
	// to recover password (true) or second factor (false).
	IsRecoverPassword bool `json:"isRecoverPassword"`
}

// startAccountRecoveryHandle is the first step of recovery process which obtains a recovery link and emails it to the requesting user.
// If a user gets locked from too many incorrect attempts, an email will be sent to notify user.
func (p *Plugin) startAccountRecoveryHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, client cloud.Client) (interface{}, error) {
	var req startAccountRecoveryRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	recoverType := types.UserTokenUsage_USER_TOKEN_RECOVER_MFA
	if req.IsRecoverPassword {
		recoverType = types.UserTokenUsage_USER_TOKEN_RECOVER_PASSWORD
	}

	token, err := p.h.GetProxyClient().StartAccountRecovery(r.Context(), &proto.StartAccountRecoveryRequest{
		Username:     req.Username,
		RecoveryCode: []byte(req.RecoveryCode),
		RecoverType:  recoverType,
	})
	if err != nil {
		p.Log.WithError(err).Warnf("Start account recovery denied for user %q.", req.Username)
		return nil, trace.AccessDenied("invalid username or recovery code")
	}

	if _, err := client.SendAccountRecoveryLink(r.Context(), &v1.SendAccountRecoveryLinkRequest{
		Email:     token.GetUser(),
		Url:       token.GetURL(),
		CreatedAt: time.Now().UTC().Unix(),
		IpAddr:    p.getIPAddress(r),
		UserAgent: r.UserAgent(),
	}); err != nil {
		p.Log.WithError(trail.FromGRPC(err)).Errorf("Failed to email user %v their recovery link(%v).", req.Username, token.GetURL())
		return nil, trace.BadParameter("unable to email account recovery link, please try again with a new recovery code or contact your system administrator")
	}

	return web.OK(), nil
}

type verifyAccountRecoveryRequest struct {
	// TokenID is recovery token id.
	TokenID string `json:"tokenId"`
	// Username is the name of user that the token belongs to.
	Username string `json:"username"`
	// Password is user's password.
	Password string `json:"password"`
	// SecondFactorToken is the otp value.
	SecondFactorToken string `json:"secondFactorToken"`
	// WebauthnAssertionResponse is a signed WebAuthn credential assertion.
	WebauthnAssertionResponse *wantypes.CredentialAssertionResponse `json:"webauthnAssertionResponse"`
}

// verifyAccountRecoveryHandle is the second step in recovery process which obtains a recovery approved token
// that will allow a user to make protected actions eg: set new authentication, delete device, and get new recovery codes.
// If a user gets locked from too many incorrect attempts, an email will be sent to notify user.
func (p *Plugin) verifyAccountRecoveryHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, client cloud.Client) (interface{}, error) {
	var req verifyAccountRecoveryRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	protoReq := &proto.VerifyAccountRecoveryRequest{
		RecoveryStartTokenID: req.TokenID,
		Username:             req.Username,
	}

	switch {
	case req.Password != "":
		protoReq.AuthnCred = &proto.VerifyAccountRecoveryRequest_Password{Password: []byte(req.Password)}
	case req.SecondFactorToken != "":
		protoReq.AuthnCred = &proto.VerifyAccountRecoveryRequest_MFAAuthenticateResponse{MFAAuthenticateResponse: &proto.MFAAuthenticateResponse{
			Response: &proto.MFAAuthenticateResponse_TOTP{TOTP: &proto.TOTPResponse{Code: req.SecondFactorToken}},
		}}
	case req.WebauthnAssertionResponse != nil:
		protoReq.AuthnCred = &proto.VerifyAccountRecoveryRequest_MFAAuthenticateResponse{MFAAuthenticateResponse: &proto.MFAAuthenticateResponse{
			Response: &proto.MFAAuthenticateResponse_Webauthn{Webauthn: wantypes.CredentialAssertionResponseToProto(req.WebauthnAssertionResponse)},
		}}
	default:
		return nil, trace.BadParameter("at least one auth credential is required")
	}

	token, err := p.h.GetProxyClient().VerifyAccountRecovery(r.Context(), protoReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return enterpriseui.RecoveryToken{
		TokenID:           token.GetName(),
		IsRecoverPassword: token.GetUsage() == types.UserTokenUsage_USER_TOKEN_RECOVER_PASSWORD,
	}, nil
}

type completeAccountRecoveryRequest struct {
	// TokenID is this token ID
	TokenID string `json:"tokenId"`
	// SecondFactorToken is 2nd factor token value
	SecondFactorToken string `json:"secondFactorToken"`
	// Password is user password
	Password string `json:"password"`
	// WebauthnCreationResponse is the signed credential creation response.
	WebauthnCreationResponse *wantypes.CredentialCreationResponse `json:"webauthnCreationResponse"`
	// DeviceName is the name of the second factor device.
	DeviceName string `json:"deviceName"`
}

// completeAccountRecoveryHandle sets a new password or mfa device for the user defined in token.
// On success, emails the user that their account was successfully recovered.
func (p *Plugin) completeAccountRecoveryHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, client cloud.Client) (interface{}, error) {
	var req completeAccountRecoveryRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	protoReq := &proto.CompleteAccountRecoveryRequest{
		RecoveryApprovedTokenID: req.TokenID,
		NewDeviceName:           req.DeviceName,
	}

	switch {
	case req.Password != "":
		protoReq.NewAuthnCred = &proto.CompleteAccountRecoveryRequest_NewPassword{NewPassword: []byte(req.Password)}
	case req.SecondFactorToken != "":
		protoReq.NewAuthnCred = &proto.CompleteAccountRecoveryRequest_NewMFAResponse{NewMFAResponse: &proto.MFARegisterResponse{
			Response: &proto.MFARegisterResponse_TOTP{TOTP: &proto.TOTPRegisterResponse{Code: req.SecondFactorToken}},
		}}
	case req.WebauthnCreationResponse != nil:
		protoReq.NewAuthnCred = &proto.CompleteAccountRecoveryRequest_NewMFAResponse{NewMFAResponse: &proto.MFARegisterResponse{
			Response: &proto.MFARegisterResponse_Webauthn{
				Webauthn: wantypes.CredentialCreationResponseToProto(req.WebauthnCreationResponse),
			},
		}}
	default:
		return nil, trace.BadParameter("at least one auth credential is required")
	}

	if err := p.h.GetProxyClient().CompleteAccountRecovery(r.Context(), protoReq); err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := p.h.GetProxyClient().GetAccountRecoveryToken(r.Context(), &proto.GetAccountRecoveryTokenRequest{
		RecoveryTokenID: req.TokenID,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if _, err := client.SendAccountRecovered(r.Context(), &v1.SendAccountRecoveredRequest{
		Email:       token.GetUser(),
		RecoveredAt: time.Now().UTC().Unix(),
		IpAddr:      p.getIPAddress(r),
		UserAgent:   r.UserAgent(),
	}); err != nil {
		p.Log.WithError(trail.FromGRPC(err)).Warnf("Failed to email user %q that their account was successfully recovered", token.GetUser())
	}

	return web.OK(), nil
}

type createAccountRecoveryCodes struct {
	TokenID string `json:"tokenId"`
}

// createAccountRecoveryCodesHandle creates, upserts, and returns new set of recovery codes for the user defined in the token.
func (p *Plugin) createAccountRecoveryCodesHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, client cloud.Client) (interface{}, error) {
	var req createAccountRecoveryCodes
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	res, err := p.h.GetProxyClient().CreateAccountRecoveryCodes(r.Context(), &proto.CreateAccountRecoveryCodesRequest{
		TokenID: req.TokenID,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ui.RecoveryCodes{
		Codes:   res.Codes,
		Created: &res.Created,
	}, nil
}

func (p *Plugin) getIPAddress(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		p.Log.WithError(err).Warnf("Failed to split host from port for remote address %s", r.RemoteAddr)
		return r.RemoteAddr
	}

	return ip
}

func (p *Plugin) getAccountRecoveryCodesMetadataHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return getAccountRecoveryCodesMetadata(r.Context(), clt)
}

func getAccountRecoveryCodesMetadata(ctx context.Context, clt accountRecoveryAPIGetter) (*ui.RecoveryCodes, error) {
	response, err := clt.GetAccountRecoveryCodes(ctx, &proto.GetAccountRecoveryCodesRequest{})
	switch {
	case trace.IsNotFound(err):
		return &ui.RecoveryCodes{}, nil
	case err != nil:
		return nil, trace.Wrap(err)
	}

	return &ui.RecoveryCodes{
		Created: &response.Created,
	}, nil
}

type accountRecoveryAPIGetter interface {
	// GetAccountRecoveryCodes returns the user in context their recovery codes resource without any secrets.
	GetAccountRecoveryCodes(ctx context.Context, req *proto.GetAccountRecoveryCodesRequest) (*proto.RecoveryCodes, error)
}
