package grpc

import (
	"math"
	"os"

	"github.com/dustin/go-humanize"
)

// defaultClientRecvSize is the grpc client default for recieved message sizes
const defaultClientRecvSize = 4 * 1024 * 1024 // 4MB

// MaxRecvSize returns maximum message size in bytes the client can receive.
//
// By default 4MB is returned, to overwrite this, set `TELEPORT_UNSTABLE_GRPC_RECV_SIZE` envriroment
// variable. If the value cannot be parsed or exceeds int32 limits, the default value is returned.
func MaxRecvSize() int {

	val := os.Getenv("TELEPORT_UNSTABLE_GRPC_RECV_SIZE")
	if val == "" {
		return defaultClientRecvSize
	}

	size, err := humanize.ParseBytes(val)
	if err != nil {
		return defaultClientRecvSize
	}

	if size > uint64(math.MaxInt32) {
		return defaultClientRecvSize
	}

	return int(size)
}
