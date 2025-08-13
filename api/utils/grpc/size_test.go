package grpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaxRecvSize(t *testing.T) {

	testCases := []struct {
		desc  string
		size  string
		bytes int
	}{
		{
			desc:  "Decimal",
			size:  "1234",
			bytes: 1234,
		},
		{
			desc:  "Unset",
			size:  "",
			bytes: defaultClientRecvSize,
		},
		{
			desc:  "Massive",
			size:  "4TB",
			bytes: defaultClientRecvSize,
		},
		{
			desc:  "Rubbish",
			size:  "foobar",
			bytes: defaultClientRecvSize,
		},
		{
			desc:  "Human",
			size:  "8mib",
			bytes: 8 * 1024 * 1024,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.desc, func(t *testing.T) {
			t.Setenv("TELEPORT_UNSTABLE_GRPC_RECV_SIZE", tt.size)
			assert.Equal(t, tt.bytes, MaxRecvSize())
		})
	}
}
