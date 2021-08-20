package webauthn

import (
	"encoding/base64"

	"github.com/gravitational/trace"

	wan "github.com/duo-labs/webauthn/webauthn"
	wantypes "github.com/gravitational/teleport/api/types/webauthn"
)

func sessionToPB(sd *wan.SessionData) (*wantypes.SessionData, error) {
	rawChallenge, err := base64.RawURLEncoding.DecodeString(sd.Challenge)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &wantypes.SessionData{
		Challenge:        rawChallenge,
		UserId:           sd.UserID,
		AllowCredentials: sd.AllowedCredentialIDs,
	}, nil
}

func sessionFromPB(sd *wantypes.SessionData) *wan.SessionData {
	return &wan.SessionData{
		Challenge:            base64.RawURLEncoding.EncodeToString(sd.Challenge),
		UserID:               sd.UserId,
		AllowedCredentialIDs: sd.AllowCredentials,
	}
}
