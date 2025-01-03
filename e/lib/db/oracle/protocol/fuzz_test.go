package protocol

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func FuzzReadPacket(f *testing.F) {
	f.Fuzz(func(t *testing.T, largeSDU bool, body []byte) {
		require.NotPanics(t, func() {
			proto := TNSVersionMinLargeSdu - 1
			if largeSDU {
				proto = TNSVersionMinLargeSdu
			}

			_, _ = ReadPacket(uint16(proto), bytes.NewReader(body))
		})
	})
}

func FuzzUtils(f *testing.F) {
	f.Fuzz(func(t *testing.T, body []byte) {
		require.NotPanics(t, func() {
			_, _ = readVarInt64(bytes.NewReader(body))
			_, _ = readByteArray(bytes.NewReader(body))
		})
	})
}
