// Package u2ftoken implements the FIDO U2F raw message protocol used to
// communicate with U2F tokens.
package u2ftoken

import (
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	cmdRegister     = 1
	cmdAuthenticate = 2
	cmdVersion      = 3

	tupRequired = 1 // Test of User Presence required
	tupConsume  = 2 // Consume a Test of User Presence
	tupTestOnly = 4 // Check valid key handle only, no test of user presence required

	authEnforce = tupRequired | tupConsume
	// This makes zero sense, but the check command is all three flags, not just tupTestOnly
	authCheckOnly = tupRequired | tupConsume | tupTestOnly

	statusNoError                = 0x9000
	statusWrongLength            = 0x6700
	statusInvalidData            = 0x6984
	statusConditionsNotSatisfied = 0x6985
	statusWrongData              = 0x6a80
	statusInsNotSupported        = 0x6d00
)

// ErrPresenceRequired is returned by Register and Authenticate if proof of user
// presence must be provide before the operation can be retried successfully.
var ErrPresenceRequired = errors.New("u2ftoken: user presence required")

// ErrUnknownKeyHandle is returned by Authenticate and CheckAuthenticate if the
// key handle is unknown to the token.
var ErrUnknownKeyHandle = errors.New("u2ftoken: unknown key handle")

// Device implements a message transport to a concrete U2F device. It is
// implemented in package u2fhid.
type Device interface {
	// Message sends a message to the device and returns the response.
	Message(data []byte) ([]byte, error)
}

// NewToken returns a token that will use Device to communicate with the device.
func NewToken(d Device) *Token {
	return &Token{d: d}
}

// A Token implements the FIDO U2F hardware token messages as defined in the Raw
// Message Formats specification.
type Token struct {
	d Device
}

// A RegisterRequest is a message used for token registration.
type RegisterRequest struct {
	// Challenge is the 32-byte SHA-256 hash of the Client Data JSON prepared by
	// the client.
	Challenge []byte

	// Application is the 32-byte SHA-256 hash of the application identity of
	// the relying party requesting registration.
	Application []byte
}

// Register registers an application with the token and returns the raw
// registration response message to be passed to the relying party. It returns
// ErrPresenceRequired if the call should be retried after proof of user
// presence is provided to the token.
func (t *Token) Register(req RegisterRequest) ([]byte, error) {
	if len(req.Challenge) != 32 {
		return nil, fmt.Errorf("u2ftoken: Challenge must be exactly 32 bytes")
	}
	if len(req.Application) != 32 {
		return nil, fmt.Errorf("u2ftoken: Application must be exactly 32 bytes")
	}

	res, err := t.Message(Request{
		Param1:  authEnforce,
		Command: cmdRegister,
		Data:    append(req.Challenge, req.Application...),
	})
	if err != nil {
		return nil, err
	}

	if res.Status != statusNoError {
		switch res.Status {
		case statusConditionsNotSatisfied:
			return nil, ErrPresenceRequired
		default:
			return nil, fmt.Errorf("u2ftoken: unexpected error %d during registration", res.Status)
		}
	}

	return res.Data, nil
}

// An AuthenticateRequires is a message used for authenticating to a relying party
type AuthenticateRequest struct {
	// Challenge is the 32-byte SHA-256 hash of the Client Data JSON prepared by
	// the client.
	Challenge []byte

	// Application is the 32-byte SHA-256 hash of the application identity of
	// the relying party requesting authentication.
	Application []byte

	// KeyHandle is the opaque key handle that was provided to the relying party
	// during registration.
	KeyHandle []byte
}

// An AuthenticateResponse is a message returned in response to a successful
// authentication request.
type AuthenticateResponse struct {
	// Counter is the value of the counter that is incremented by the token
	// every time it performs an authentication operation.
	Counter uint32

	// Signature is the P-256 ECDSA signature over the authentication data.
	Signature []byte

	// RawResponse is the raw response bytes from the U2F token.
	RawResponse []byte
}

func encodeAuthenticateRequest(req AuthenticateRequest) ([]byte, error) {
	if len(req.Challenge) != 32 {
		return nil, fmt.Errorf("u2ftoken: Challenge must be exactly 32 bytes")
	}
	if len(req.Application) != 32 {
		return nil, fmt.Errorf("u2ftoken: Application must be exactly 32 bytes")
	}
	if len(req.KeyHandle) > 256 {
		return nil, fmt.Errorf("u2ftoken: KeyHandle is too long")
	}

	buf := make([]byte, 0, len(req.Challenge)+len(req.Application)+1+len(req.KeyHandle))
	buf = append(buf, req.Challenge...)
	buf = append(buf, req.Application...)
	buf = append(buf, byte(len(req.KeyHandle)))
	buf = append(buf, req.KeyHandle...)

	return buf, nil
}

// Authenticate peforms an authentication operation and returns the response to
// provide to the relying party. It returns ErrPresenceRequired if the call
// should be retried after proof of user presence is provided to the token and
// ErrUnknownKeyHandle if the key handle is unknown to the token.
func (t *Token) Authenticate(req AuthenticateRequest) (*AuthenticateResponse, error) {
	buf, err := encodeAuthenticateRequest(req)
	if err != nil {
		return nil, err
	}

	res, err := t.Message(Request{
		Command: cmdAuthenticate,
		Param1:  authEnforce,
		Data:    buf,
	})
	if err != nil {
		return nil, err
	}

	if res.Status != statusNoError {
		if res.Status == statusConditionsNotSatisfied {
			return nil, ErrPresenceRequired
		}
		return nil, fmt.Errorf("u2ftoken: unexpected error %d during authentication", res.Status)
	}

	if len(res.Data) < 6 {
		return nil, fmt.Errorf("u2ftoken: authenticate response is too short, got %d bytes", len(res.Data))
	}

	return &AuthenticateResponse{
		Counter:     binary.BigEndian.Uint32(res.Data[1:]),
		Signature:   res.Data[5:],
		RawResponse: res.Data,
	}, nil
}

// CheckAuthenticate checks if a key handle is known to the token without
// requiring a test for user presence. It returns ErrUnknownKeyHandle if the key
// handle is unknown to the token.
func (t *Token) CheckAuthenticate(req AuthenticateRequest) error {
	buf, err := encodeAuthenticateRequest(req)
	if err != nil {
		return err
	}

	res, err := t.Message(Request{
		Command: cmdAuthenticate,
		Param1:  authCheckOnly,
		Data:    buf,
	})
	if err != nil {
		return err
	}

	if res.Status != statusConditionsNotSatisfied {
		if res.Status == statusWrongData {
			return ErrUnknownKeyHandle
		}
		return fmt.Errorf("u2ftoken: unexpected error %d during auth check", res.Status)
	}

	return nil
}

// Version returns the U2F protocol version implemented by the token.
func (t *Token) Version() (string, error) {
	res, err := t.Message(Request{Command: cmdVersion})
	if err != nil {
		return "", err
	}

	if res.Status != statusNoError {
		return "", fmt.Errorf("u2ftoken: unexpected error %d during version request", res.Status)
	}

	return string(res.Data), nil
}

// A Request is a low-level request to the token.
type Request struct {
	Command uint8
	Param1  uint8
	Param2  uint8
	Data    []byte
}

// A Response is a low-level response from the token.
type Response struct {
	Data   []byte
	Status uint16
}

// Message sends a low-level request to the token and returns the response.
func (t *Token) Message(req Request) (*Response, error) {
	buf := make([]byte, 7, 7+len(req.Data))
	buf[1] = req.Command
	buf[2] = req.Param1
	buf[3] = req.Param2
	buf[4] = uint8(len(req.Data) >> 16)
	buf[5] = uint8(len(req.Data) >> 8)
	buf[6] = uint8(len(req.Data))
	buf = append(buf, req.Data...)

	data, err := t.d.Message(buf)
	if err != nil {
		return nil, err
	}
	if len(data) < 2 {
		return nil, fmt.Errorf("u2ftoken: response is too short, got %d bytes", len(data))
	}
	return &Response{
		Data:   data[:len(data)-2],
		Status: binary.BigEndian.Uint16(data[len(data)-2:]),
	}, nil
}
