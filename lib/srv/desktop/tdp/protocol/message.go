package protocol

import "errors"

// ErrEmptyMessage is returned when an empty message is decoded.
var ErrEmptyMessage = errors.New("decoded empty TDP message")

// Message is a Go representation of a desktop protocol message.
type Message interface {
	Encode() ([]byte, error)
}

// These correspond to TdpErrCode enum in the rust RDP client.
const (
	ErrCodeNil           uint32 = 0
	ErrCodeFailed        uint32 = 1
	ErrCodeDoesNotExist  uint32 = 2
	ErrCodeAlreadyExists uint32 = 3
)
