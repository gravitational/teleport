package web

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/u2f"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/web"
	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/e/api/cloud"
	v1 "github.com/gravitational/teleport/e/api/cloud/v1"
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

	uiToken := ui.RecoveryToken{
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
		if err.Error() == auth.MaxFailedAttemptsFromStartRecoveryErrMsg {
			if _, emailErr := client.SendAccountLocked(r.Context(), &v1.SendAccountLockedRequest{
				Email: req.Username,
			}); emailErr != nil {
				p.Log.WithError(trail.FromGRPC(emailErr)).Warnf("Failed to email user %v that their account got locked.", req.Username)
			}
		}
		return nil, trace.Wrap(err)
	}

	failedToEmailErrMsg := "unable to email account recovery link, please try again with a new recovery code or contact your system administrator"

	ipAddr, err := getIPAddress(r)
	if err != nil {
		p.Log.WithError(err).Errorf("Failed to email user %v their recovery link(%v).", req.Username, token.GetURL())
		return nil, trace.BadParameter(failedToEmailErrMsg)
	}

	if _, err := client.SendAccountRecoveryLink(r.Context(), &v1.SendAccountRecoveryLinkRequest{
		Email:     token.GetUser(),
		Url:       token.GetURL(),
		CreatedAt: time.Now().UTC().Unix(),
		IpAddr:    ipAddr,
		UserAgent: r.UserAgent(),
	}); err != nil {
		p.Log.WithError(trail.FromGRPC(err)).Errorf("Failed to email user %v their recovery link(%v).", req.Username, token.GetURL())
		return nil, trace.BadParameter(failedToEmailErrMsg)
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
	// U2FSignResponse is u2f sign response for a u2f challenge.
	U2FSignResponse *u2f.AuthenticateChallengeResponse `json:"u2fSignResponse"`
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
	case req.U2FSignResponse != nil:
		protoReq.AuthnCred = &proto.VerifyAccountRecoveryRequest_MFAAuthenticateResponse{MFAAuthenticateResponse: &proto.MFAAuthenticateResponse{
			Response: &proto.MFAAuthenticateResponse_U2F{U2F: &proto.U2FResponse{
				KeyHandle:  req.U2FSignResponse.KeyHandle,
				ClientData: req.U2FSignResponse.ClientData,
				Signature:  req.U2FSignResponse.SignatureData,
			}},
		}}
	default:
		return nil, trace.BadParameter("at least one auth credential is required")
	}

	token, err := p.h.GetProxyClient().VerifyAccountRecovery(r.Context(), protoReq)
	if err != nil {
		if err.Error() == auth.MaxFailedAttemptsFromVerifyRecoveryErrMsg {
			if _, emailErr := client.SendAccountLocked(r.Context(), &v1.SendAccountLockedRequest{
				Email: req.Username,
			}); emailErr != nil {
				p.Log.WithError(trail.FromGRPC(emailErr)).Warnf("Failed to email user %v that their account got locked.", req.Username)
			}
		}
		return nil, trace.Wrap(err)
	}

	return ui.RecoveryToken{
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
	// U2FRegisterResponse is U2F registration challenge response.
	U2FRegisterResponse *u2f.RegisterChallengeResponse `json:"u2fRegisterResponse"`
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
	case req.U2FRegisterResponse != nil:
		protoReq.NewAuthnCred = &proto.CompleteAccountRecoveryRequest_NewMFAResponse{NewMFAResponse: &proto.MFARegisterResponse{
			Response: &proto.MFARegisterResponse_U2F{U2F: &proto.U2FRegisterResponse{
				RegistrationData: req.U2FRegisterResponse.RegistrationData,
				ClientData:       req.U2FRegisterResponse.ClientData,
			}},
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

	ipAddr, err := getIPAddress(r)
	if err != nil {
		p.Log.WithError(trail.FromGRPC(err)).Warnf("Failed to email user %q that their account was successfully recovered", token.GetUser())
		return web.OK(), nil
	}

	if _, err := client.SendAccountRecovered(r.Context(), &v1.SendAccountRecoveredRequest{
		Email:       token.GetUser(),
		RecoveredAt: time.Now().UTC().Unix(),
		IpAddr:      ipAddr,
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

	return res.GetRecoveryCodes(), nil
}

func getIPAddress(r *http.Request) (string, error) {
	originatingIPAddr := ""

	// If load balancing is used.
	ips := strings.Split(r.Header.Get("x-forwarded-for"), ", ")
	if len(ips) > 0 {
		// First ip address in list is the ip address of the original request.
		// The rest are addresses of proxies.
		originatingIPAddr = ips[0]
	}

	// Fallback if load balancer wasn't used.
	if originatingIPAddr == "" {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			return "", trace.Wrap(err)
		}
		originatingIPAddr = ip
	}

	return originatingIPAddr, nil
}
