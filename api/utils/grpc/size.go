// Copyright 2025 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package grpc

import (
	"math"
	"os"

	"github.com/dustin/go-humanize"
)

// defaultClientRecvSize is the grpc client default for received message sizes
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
