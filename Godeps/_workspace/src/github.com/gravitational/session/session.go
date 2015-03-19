package session

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/lemma/random"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/lemma/secret"
)

type SecureID string

// Secure encrypted session id
// Plain text unique session id
type PlainID string

// IDPair is a pair of unique and encrypted session id
type IDPair struct {
	SID SecureID
	PID PlainID
}

func NewID(s *secret.Service) (*IDPair, error) {
	p := &random.CSPRNG{}
	bytes, err := p.Bytes(32)
	if err != nil {
		return nil, err
	}
	pid := []byte(hex.EncodeToString(bytes))
	sealed, err := s.Seal(pid)
	if err != nil {
		return nil, err
	}
	id := fmt.Sprintf("%v.%v",
		base64.URLEncoding.EncodeToString(sealed.Ciphertext),
		base64.URLEncoding.EncodeToString(sealed.Nonce))
	return &IDPair{SID: SecureID(id), PID: PlainID(pid)}, nil
}

func DecodeSID(sid SecureID, s *secret.Service) (PlainID, error) {
	out := strings.Split(string(sid), ".")
	if len(out) != 2 {
		return "", &MalformedSessionError{S: sid, Msg: "invalid format, missing separator"}
	}
	ctext, err := base64.URLEncoding.DecodeString(out[0])
	if err != nil {
		return "", &MalformedSessionError{S: sid, Msg: err.Error()}
	}
	nonce, err := base64.URLEncoding.DecodeString(out[1])
	if err != nil {
		return "", &MalformedSessionError{S: sid, Msg: err.Error()}
	}
	id, err := s.Open(&secret.SealedBytes{Ciphertext: ctext, Nonce: nonce})
	if err != nil {
		return "", &MalformedSessionError{S: sid, Msg: err.Error()}
	}
	return PlainID(id), nil
}

type MalformedSessionError struct {
	S   SecureID
	Msg string
}

func (m *MalformedSessionError) Error() string {
	return fmt.Sprintf("malformed session: %v", m.Msg)
}
