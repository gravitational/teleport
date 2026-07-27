//go:build desktop_access_rdp && desktop_encoder

// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package rdpclient

import (
	"unsafe"

	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/tdpb"
	"github.com/gravitational/trace"
)

/*
#include <librdpclient.h>
*/
import "C"

// EncodeQOIZ encodes changed frame to series of FastPath SetSurface PDUs using QOIZ codec.
// Resulting frames can be consumed directly by the FastPath processor from IronRDP if qoiz
// feature is enabled in ironrdp-session crate
func EncodeQOIZ(frame []byte, x, y, width, height uint16) ([]*tdpb.FastPathPDU, error) {
	if len(frame) == 0 {
		return nil, nil
	}
	if len(frame) != int(width)*int(height)*4 {
		return nil, trace.BadParameter("incorrect frame size")
	}
	data := unsafe.SliceData(frame)
	encodingResult := C.encode_qoiz((*C.uint8_t)(data), C.uint16_t(x), C.uint16_t(y), C.uint16_t(width), C.uint16_t(height))
	defer C.free_encoding_result(encodingResult)
	if encodingResult.error_code != C.ErrCodeSuccess {
		msg := C.GoBytes(unsafe.Pointer(encodingResult.error_msg), C.int(encodingResult.length))
		return nil, trace.Errorf("Couldn't encode frame: %s", string(msg))
	}
	pdus := unsafe.Slice((*C.Pdu)(encodingResult.pdus), encodingResult.length)
	messages := make([]*tdpb.FastPathPDU, 0, encodingResult.length)
	for _, frame := range pdus {
		messages = append(messages, &tdpb.FastPathPDU{
			Pdu: C.GoBytes(unsafe.Pointer(frame.data), C.int(frame.length)),
		})
	}
	return messages, nil
}

func EncodeQOIZAvailable() bool {
	return true
}
