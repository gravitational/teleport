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
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"unicode"
)

// defaultClientRecvSize is the grpc client default for received message sizes
const defaultClientRecvSize = 4 * 1024 * 1024 // 4MB

// parseBytes takes human represtation of bytes such as '24mb' and returns number of bytes as an integer
//
// Only support a subset of SI prefixes up to gi/gb.
func parseBytes(s string) (int, error) {
	const (
		Byte = 1 << (iota * 10)
		KiByte
		MiByte
		GiByte
		IByte = 1
		KByte = IByte * 1000
		MByte = KByte * 1000
		GByte = MByte * 1000
	)

	var bytesSizeTable = map[string]int{
		"b":   Byte,
		"kib": KiByte,
		"kb":  KByte,
		"mib": MiByte,
		"mb":  MByte,
		"gib": GiByte,
		"gb":  GByte,
		"":    Byte,
		"ki":  KiByte,
		"k":   KByte,
		"mi":  MiByte,
		"m":   MByte,
		"gi":  GiByte,
		"g":   GByte,
	}

	lastDigit := 0
	for _, r := range s {
		if !unicode.IsDigit(r) && r != '.' {
			break
		}
		lastDigit++
	}

	num := s[:lastDigit]

	var value float32
	if f, err := strconv.ParseFloat(num, 32); err == nil {
		value = float32(f)
	} else {
		return 0, err

	}

	extra := strings.ToLower(strings.TrimSpace(s[lastDigit:]))
	if m, ok := bytesSizeTable[extra]; ok {
		value *= float32(m)
		if value >= math.MaxInt32 {
			return 0, fmt.Errorf("too large: %v", s)
		}
		return int(value), nil
	}

	return 0, fmt.Errorf("unhandled size name: %v", extra)
}

// MaxClientRecvMsgSize returns maximum message size in bytes the client can receive.
//
// By default 4MB is returned, to overwrite this, set `TELEPORT_UNSTABLE_GRPC_RECV_SIZE` envriroment
// variable. If the value cannot be parsed or exceeds int32 limits, the default value is returned.
//
// The result of this call can be passed directly into `grpc.MaxCallRecvMsgSize`, example:
//
//	conn, err := grpc.DialContext(ctx, target,
//		grpc.WithDefaultCallOptions(
//			grpc.MaxCallRecvMsgSize(grpcutils.MaxClientRecvMsgSize()),
//		),
//	)
func MaxClientRecvMsgSize() int {

	val := os.Getenv("TELEPORT_UNSTABLE_GRPC_RECV_SIZE")
	if val == "" {
		return defaultClientRecvSize
	}

	size, err := parseBytes(val)
	if err != nil {
		return defaultClientRecvSize
	}

	return size
}
