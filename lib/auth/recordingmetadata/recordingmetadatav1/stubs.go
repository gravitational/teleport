/*
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package recordingmetadatav1

/*
#include <stdlib.h>
#include <stdint.h>
#include "librdpclient.h"
*/
import "C"

//export cgo_handle_fastpath_pdu
func cgo_handle_fastpath_pdu(handle C.CgoHandle, data *C.uint8_t, length C.uint32_t) C.CGOErrCode {
	return C.ErrCodeSuccess
}

//export cgo_handle_rdp_connection_activated
func cgo_handle_rdp_connection_activated(handle C.CgoHandle, ioChannelID C.uint16_t, userChannelID C.uint16_t, screenWidth C.uint16_t, screenHeight C.uint16_t) C.CGOErrCode {
	return C.ErrCodeSuccess
}

//export cgo_handle_remote_copy
func cgo_handle_remote_copy(handle C.CgoHandle, data *C.uint8_t, length C.uint32_t) C.CGOErrCode {
	return C.ErrCodeSuccess
}

//export cgo_free_rdp_license
func cgo_free_rdp_license(data *C.uint8_t) {
}

//export cgo_read_rdp_license
func cgo_read_rdp_license(handle C.CgoHandle, req *C.CGOLicenseRequest, dataOut **C.uint8_t, lenOut *C.uintptr_t) C.CGOErrCode {
	return C.ErrCodeNotFound
}

//export cgo_write_rdp_license
func cgo_write_rdp_license(handle C.CgoHandle, req *C.CGOLicenseRequest, data *C.uint8_t, length C.uintptr_t) C.CGOErrCode {
	return C.ErrCodeSuccess
}

//export cgo_tdp_sd_acknowledge
func cgo_tdp_sd_acknowledge(handle C.CgoHandle, ack *C.CGOSharedDirectoryAcknowledge) C.CGOErrCode {
	return C.ErrCodeSuccess
}

//export cgo_tdp_sd_info_request
func cgo_tdp_sd_info_request(handle C.CgoHandle, req *C.CGOSharedDirectoryInfoRequest) C.CGOErrCode {
	return C.ErrCodeSuccess
}

//export cgo_tdp_sd_create_request
func cgo_tdp_sd_create_request(handle C.CgoHandle, req *C.CGOSharedDirectoryCreateRequest) C.CGOErrCode {
	return C.ErrCodeSuccess
}

//export cgo_tdp_sd_delete_request
func cgo_tdp_sd_delete_request(handle C.CgoHandle, req *C.CGOSharedDirectoryDeleteRequest) C.CGOErrCode {
	return C.ErrCodeSuccess
}

//export cgo_tdp_sd_list_request
func cgo_tdp_sd_list_request(handle C.CgoHandle, req *C.CGOSharedDirectoryListRequest) C.CGOErrCode {
	return C.ErrCodeSuccess
}

//export cgo_tdp_sd_read_request
func cgo_tdp_sd_read_request(handle C.CgoHandle, req *C.CGOSharedDirectoryReadRequest) C.CGOErrCode {
	return C.ErrCodeSuccess
}

//export cgo_tdp_sd_write_request
func cgo_tdp_sd_write_request(handle C.CgoHandle, req *C.CGOSharedDirectoryWriteRequest) C.CGOErrCode {
	return C.ErrCodeSuccess
}

//export cgo_tdp_sd_move_request
func cgo_tdp_sd_move_request(handle C.CgoHandle, req *C.CGOSharedDirectoryMoveRequest) C.CGOErrCode {
	return C.ErrCodeSuccess
}

//export cgo_tdp_sd_truncate_request
func cgo_tdp_sd_truncate_request(handle C.CgoHandle, req *C.CGOSharedDirectoryTruncateRequest) C.CGOErrCode {
	return C.ErrCodeSuccess
}
