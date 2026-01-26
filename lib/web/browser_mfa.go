package web

import (
	"net/http"

	proto "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
)

func (h *Handler) putBrowserMFA(_ http.ResponseWriter, r *http.Request, params httprouter.Params, sctx *SessionContext) (any, error) {
	requestID := params.ByName("request_id")
	if requestID == "" {
		return "", trace.BadParameter("request is missing request ID")
	}

	var req client.MFAChallengeResponse
	if err := httplib.ReadResourceJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	mfaResp, err := req.GetOptionalMFAResponseProtoReq()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if mfaResp == nil {
		return nil, trace.Errorf("mfa response is nil")
	}

	proxyClient := h.GetProxyClient()

	redirectURL, err := proxyClient.ValidateBrowserMFAChallenge(r.Context(), &proto.BrowserMFAResponse{
		RequestId:        requestID,
		WebauthnResponse: mfaResp.GetWebauthn(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return redirectURL, nil
}
