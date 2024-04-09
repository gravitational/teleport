// Teleport
// Copyright (C) 2023  Gravitational, Inc.
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

//! This crate contains an RDP Client with the minimum functionality required
//! for Teleport's Desktop Access feature.
//!
//! Along with core RDP functionality, it contains code for:
//! - Calling functions defined in Go (these are declared in an `extern "C"` block)
//! - Functions to be called from Go (any function prefixed with the `#[no_mangle]`
//!   macro and a `pub unsafe extern "C"`).
//! - Structs for passing between the two (those prefixed with the `#[repr(C)]` macro
//!   and whose name begins with `CGO`)

use crate::client::global::get_client_handle;
use crate::client::Client;
use crate::rdpdr::tdp::SharedDirectoryAnnounce;
use client::{ClientHandle, ClientResult, ConnectParams};
use log::{error, trace, warn};
use rdpdr::path::UnixPath;
use rdpdr::tdp::{
    FileSystemObject, FileType, SharedDirectoryAcknowledge, SharedDirectoryCreateResponse,
    SharedDirectoryDeleteResponse, SharedDirectoryInfoResponse, SharedDirectoryListResponse,
    SharedDirectoryMoveResponse, SharedDirectoryReadResponse, SharedDirectoryTruncateResponse,
    SharedDirectoryWriteResponse, TdpErrCode,
};
use std::ffi::CString;
use std::fmt::Debug;
use std::os::raw::c_char;
use std::ptr;
use util::{from_c_string, from_go_array};
pub mod client;
mod cliprdr;
mod piv;
mod rdpdr;
mod ssl;
mod util;

#[no_mangle]
pub extern "C" fn init() {
    env_logger::try_init().unwrap_or_else(|e| println!("failed to initialize Rust logger: {e}"));
}

/// free_string is used to free memory for strings that were passed back to Go side.
///
/// # Safety
///
/// The caller must ensure that the provided pointer was created by Rust using CString::into_raw
/// method and that length of the string was not modified in the meantime.
#[no_mangle]
pub unsafe extern "C" fn free_string(ptr: *mut c_char) {
    if !ptr.is_null() {
        drop(CString::from_raw(ptr));
    }
}

/// client_run establishes an RDP connection with the provided `params`
/// and executes the RDP session, hanging until the session ends.
///
/// Sessions can end due to an error, or the caller can end the session
/// manually by calling [`client_stop`]. Failure to end a session can
/// result in a memory leak.
///
/// Caller must free memory allocated for message returned (CGOResult.message)
/// using free_string function.
///
/// Message returned by this function can be null.
///
/// # Safety
///
/// The caller must ensure that cgo_handle is a valid handle and that
/// go_addr, go_username, cert_der, key_der point to valid buffers.
#[no_mangle]
pub unsafe extern "C" fn client_run(cgo_handle: CgoHandle, params: CGOConnectParams) -> CGOResult {
    trace!("client_run");
    // Convert from C to Rust types.
    let addr = from_c_string(params.go_addr);
    let cert_der = from_go_array(params.cert_der, params.cert_der_len);
    let key_der = from_go_array(params.key_der, params.key_der_len);

    match Client::run(
        cgo_handle,
        ConnectParams {
            addr,
            cert_der,
            key_der,
            screen_width: params.screen_width,
            screen_height: params.screen_height,
            allow_clipboard: params.allow_clipboard,
            allow_directory_sharing: params.allow_directory_sharing,
            show_desktop_wallpaper: params.show_desktop_wallpaper,
        },
    ) {
        Ok(res) => CGOResult {
            err_code: CGOErrCode::ErrCodeSuccess,
            message: match res {
                Some(reason) => CString::new(reason.description().to_string())
                    .map(|c| c.into_raw())
                    .unwrap_or(ptr::null_mut()),
                None => ptr::null_mut(),
            },
        },

        Err(e) => {
            error!("client_run failed: {:?}", e);
            CGOResult {
                err_code: CGOErrCode::ErrCodeFailure,
                message: CString::new(format!("{}", e))
                    .map(|c| c.into_raw())
                    .unwrap_or(ptr::null_mut()),
            }
        }
    }
}

fn handle_operation<T>(cgo_handle: CgoHandle, ctx: &'static str, f: T) -> CGOErrCode
where
    T: FnOnce(ClientHandle) -> ClientResult<()>,
{
    let client_handle = match get_client_handle(cgo_handle) {
        Some(it) => it,
        None => {
            warn!("call_function_on_handle failed: handle not found");
            return CGOErrCode::ErrCodeFailure;
        }
    };
    match f(client_handle) {
        Ok(_) => CGOErrCode::ErrCodeSuccess,
        Err(e) => {
            error!("{} failed: {:?}", ctx, e);
            CGOErrCode::ErrCodeFailure
        }
    }
}

/// client_stop ensures that a connection started by [`client_run`] is stopped
/// and that all related memory is cleaned up. Calling [`client_stop`] on a handle
/// that's already been dropped is safe and will result in a no-op.
///
/// # Safety
///
/// All values of `cgo_handle` are safe to use.
#[no_mangle]
pub unsafe extern "C" fn client_stop(cgo_handle: CgoHandle) -> CGOErrCode {
    trace!("client_stop");
    handle_operation(cgo_handle, "client_stop", move |client_handle| {
        client_handle.stop()
    })
}

/// `client_update_clipboard` is called from Go, and caches data that was copied
/// client-side while notifying the RDP server that new clipboard data is available.
///
/// # Safety
///
/// `cgo_handle` must be a valid handle.
///
/// data MUST be a valid pointer.
/// (validity defined by the validity of data in https://doc.rust-lang.org/std/slice/fn.from_raw_parts_mut.html)
#[no_mangle]
pub unsafe extern "C" fn client_update_clipboard(
    cgo_handle: CgoHandle,
    data: *mut u8,
    len: u32,
) -> CGOErrCode {
    let data = from_go_array(data, len);
    match String::from_utf8(data) {
        Ok(s) => handle_operation(
            cgo_handle,
            "client_update_clipboard",
            move |client_handle| client_handle.update_clipboard(s),
        ),
        Err(e) => {
            error!("can't convert clipboard data: {}", e);
            CGOErrCode::ErrCodeFailure
        }
    }
}

/// client_handle_tdp_sd_announce announces a new drive that's ready to be
/// redirected over RDP.
///
///
/// # Safety
///
/// `cgo_handle` must be a valid handle.
///
/// sd_announce.name MUST be a non-null pointer to a C-style null terminated string.
#[no_mangle]
pub unsafe extern "C" fn client_handle_tdp_sd_announce(
    cgo_handle: CgoHandle,
    sd_announce: CGOSharedDirectoryAnnounce,
) -> CGOErrCode {
    let sd_announce = SharedDirectoryAnnounce::from(sd_announce);
    handle_operation(
        cgo_handle,
        "client_handle_tdp_sd_announce",
        move |client_handle| client_handle.handle_tdp_sd_announce(sd_announce),
    )
}

/// client_handle_tdp_sd_info_response handles a TDP Shared Directory Info Response
/// message
///
/// # Safety
///
/// `cgo_handle` must be a valid handle.
///
/// res.fso.path MUST be a non-null pointer to a C-style null terminated string.
#[no_mangle]
pub unsafe extern "C" fn client_handle_tdp_sd_info_response(
    cgo_handle: CgoHandle,
    res: CGOSharedDirectoryInfoResponse,
) -> CGOErrCode {
    let res = SharedDirectoryInfoResponse::from(res);
    handle_operation(
        cgo_handle,
        "client_handle_tdp_sd_info_response",
        move |client_handle| client_handle.handle_tdp_sd_info_response(res),
    )
}

/// client_handle_tdp_sd_create_response handles a TDP Shared Directory Create Response
/// message
///
/// # Safety
///
/// `cgo_handle` must be a valid handle.
#[no_mangle]
pub unsafe extern "C" fn client_handle_tdp_sd_create_response(
    cgo_handle: CgoHandle,
    res: CGOSharedDirectoryCreateResponse,
) -> CGOErrCode {
    let res = SharedDirectoryCreateResponse::from(res);
    handle_operation(
        cgo_handle,
        "client_handle_tdp_sd_create_response",
        move |client_handle| client_handle.handle_tdp_sd_create_response(res),
    )
}

/// client_handle_tdp_sd_delete_response handles a TDP Shared Directory Delete Response
/// message
///
/// # Safety
///
/// `cgo_handle` must be a valid handle.
#[no_mangle]
pub unsafe extern "C" fn client_handle_tdp_sd_delete_response(
    cgo_handle: CgoHandle,
    res: CGOSharedDirectoryDeleteResponse,
) -> CGOErrCode {
    handle_operation(
        cgo_handle,
        "client_handle_tdp_sd_delete_response",
        move |client_handle| client_handle.handle_tdp_sd_delete_response(res),
    )
}

/// client_handle_tdp_sd_list_response handles a TDP Shared Directory List Response message.
///
/// # Safety
///
/// `cgo_handle` must be a valid handle.
///
/// res.fso_list MUST be a valid pointer
/// (validity defined by the validity of data in https://doc.rust-lang.org/std/slice/fn.from_raw_parts_mut.html)
///
/// each res.fso_list[i].path MUST be a non-null pointer to a C-style null terminated string.
#[no_mangle]
pub unsafe extern "C" fn client_handle_tdp_sd_list_response(
    cgo_handle: CgoHandle,
    res: CGOSharedDirectoryListResponse,
) -> CGOErrCode {
    let res = SharedDirectoryListResponse::from(res);
    handle_operation(
        cgo_handle,
        "client_client_handle_tdp_sd_list_response",
        move |client_handle| client_handle.handle_tdp_sd_list_response(res),
    )
}

/// client_handle_tdp_sd_read_response handles a TDP Shared Directory Read Response
/// message
///
/// # Safety
///
/// `cgo_handle` must be a valid handle.
#[no_mangle]
pub unsafe extern "C" fn client_handle_tdp_sd_read_response(
    cgo_handle: CgoHandle,
    res: CGOSharedDirectoryReadResponse,
) -> CGOErrCode {
    let res = SharedDirectoryReadResponse::from(res);
    handle_operation(
        cgo_handle,
        "client_handle_tdp_sd_read_response",
        move |client_handle| client_handle.handle_tdp_sd_read_response(res),
    )
}

/// client_handle_tdp_sd_write_response handles a TDP Shared Directory Write Response
/// message
///
/// # Safety
///
/// `cgo_handle` must be a valid handle.
#[no_mangle]
pub unsafe extern "C" fn client_handle_tdp_sd_write_response(
    cgo_handle: CgoHandle,
    res: CGOSharedDirectoryWriteResponse,
) -> CGOErrCode {
    handle_operation(
        cgo_handle,
        "client_handle_tdp_sd_write_response",
        move |client_handle| client_handle.handle_tdp_sd_write_response(res),
    )
}

/// client_handle_tdp_sd_move_response handles a TDP Shared Directory Move Response
/// message
///
/// # Safety
///
/// `cgo_handle` must be a valid handle.
#[no_mangle]
pub unsafe extern "C" fn client_handle_tdp_sd_move_response(
    cgo_handle: CgoHandle,
    res: CGOSharedDirectoryMoveResponse,
) -> CGOErrCode {
    handle_operation(
        cgo_handle,
        "client_handle_tdp_sd_move_response",
        move |client_handle| client_handle.handle_tdp_sd_move_response(res),
    )
}

/// client_handle_tdp_sd_truncate_response handles a TDP Shared Directory Truncate Response
/// message
///
/// # Safety
///
/// `cgo_handle` must be a valid handle.
#[no_mangle]
pub unsafe extern "C" fn client_handle_tdp_sd_truncate_response(
    cgo_handle: CgoHandle,
    res: CGOSharedDirectoryTruncateResponse,
) -> CGOErrCode {
    handle_operation(
        cgo_handle,
        "client_handle_tdp_sd_truncate_response",
        move |client_handle| client_handle.handle_tdp_sd_truncate_response(res),
    )
}

/// client_handle_tdp_rdp_response_pdu handles a TDP RDP Response PDU message. It takes a raw encoded RDP PDU
/// created by the ironrdp client on the frontend and sends it directly to the RDP server.
///
/// res is the raw RDP response message to be sent back to the RDP server, without the TDP message type or
/// array length header.
///
/// # Safety
///
/// `cgo_handle` must be a valid handle.
#[no_mangle]
pub unsafe extern "C" fn client_handle_tdp_rdp_response_pdu(
    cgo_handle: CgoHandle,
    res: *mut u8,
    res_len: u32,
) -> CGOErrCode {
    let res = from_go_array(res, res_len);
    handle_operation(
        cgo_handle,
        "client_handle_tdp_rdp_response_pdu",
        move |client_handle| client_handle.write_raw_pdu(res),
    )
}

/// # Safety
///
/// `cgo_handle` must be a valid handle.
#[no_mangle]
pub unsafe extern "C" fn client_write_rdp_pointer(
    cgo_handle: CgoHandle,
    pointer: CGOMousePointerEvent,
) -> CGOErrCode {
    handle_operation(
        cgo_handle,
        "client_write_rdp_pointer",
        move |client_handle| client_handle.write_rdp_pointer(pointer),
    )
}

/// # Safety
///
/// `cgo_handle` must be a valid handle.
#[no_mangle]
pub unsafe extern "C" fn client_write_rdp_keyboard(
    cgo_handle: CgoHandle,
    key: CGOKeyboardEvent,
) -> CGOErrCode {
    handle_operation(
        cgo_handle,
        "client_write_rdp_keyboard",
        move |client_handle| client_handle.write_rdp_key(key),
    )
}

/// # Safety
///
/// `cgo_handle` must be a valid handle.
#[no_mangle]
pub unsafe extern "C" fn client_write_rdp_sync_keys(
    cgo_handle: CgoHandle,
    keys: CGOSyncKeys,
) -> CGOErrCode {
    handle_operation(
        cgo_handle,
        "client_write_rdp_sync_keys",
        move |client_handle| client_handle.write_rdp_sync_keys(keys),
    )
}

#[repr(C)]
pub struct CGOConnectParams {
    go_addr: *const c_char,
    cert_der_len: u32,
    cert_der: *mut u8,
    key_der_len: u32,
    key_der: *mut u8,
    screen_width: u16,
    screen_height: u16,
    allow_clipboard: bool,
    allow_directory_sharing: bool,
    show_desktop_wallpaper: bool,
}

/// CGOKeyboardEvent is a CGO-compatible version of KeyboardEvent that we pass back to Go.
/// KeyboardEvent is a keyboard update from the user.
#[repr(C)]
#[derive(Copy, Clone, Debug)]
pub struct CGOKeyboardEvent {
    // Note: there's only one key code sent at a time. A key combo is sent as a sequence of
    // KeyboardEvent messages, one key at a time in the "down" state. The RDP server takes care of
    // interpreting those.
    pub code: u16,
    pub down: bool,
}

#[repr(C)]
pub enum CGODisconnectCode {
    /// DisconnectCodeUnknown is for when we can't determine whether
    /// a disconnect was caused by the RDP client or server.
    DisconnectCodeUnknown = 0,
    /// DisconnectCodeClient is for when the RDP client initiated a disconnect.
    DisconnectCodeClient = 1,
    /// DisconnectCodeServer is for when the RDP server initiated a disconnect.
    DisconnectCodeServer = 2,
}

#[repr(C)]
pub struct CGOReadRdpOutputReturns {
    user_message: *const c_char,
    disconnect_code: CGODisconnectCode,
    err_code: CGOErrCode,
}

#[repr(C)]
pub struct CGOClientOrError {
    client: u64,
    err: CGOErrCode,
}

/// CGOMousePointerEvent is a CGO-compatible version of PointerEvent that we pass back to Go.
/// PointerEvent is a mouse move or click update from the user.
#[repr(C)]
#[derive(Copy, Clone, Debug)]
pub struct CGOMousePointerEvent {
    pub x: u16,
    pub y: u16,
    pub button: CGOPointerButton,
    pub down: bool,
    pub wheel: CGOPointerWheel,
    pub wheel_delta: i16,
}

#[repr(C)]
#[derive(Copy, Clone, Debug)]
pub struct CGOSyncKeys {
    pub scroll_lock_down: bool,
    pub num_lock_down: bool,
    pub caps_lock_down: bool,
    pub kana_lock_down: bool,
}

#[repr(C)]
#[derive(Copy, Clone, PartialEq, Debug)]
pub enum CGOPointerButton {
    PointerButtonNone,
    PointerButtonLeft,
    PointerButtonRight,
    PointerButtonMiddle,
}

#[repr(C)]
#[derive(Copy, Clone, Debug, PartialEq)]
pub enum CGOPointerWheel {
    PointerWheelNone,
    PointerWheelVertical,
    PointerWheelHorizontal,
}

#[repr(C)]
#[derive(Copy, Clone, PartialEq, Eq, Debug)]
pub enum CGOErrCode {
    ErrCodeSuccess = 0,
    ErrCodeFailure = 1,
    ErrCodeClientPtr = 2,
}

#[repr(C)]
pub struct CGOResult {
    pub err_code: CGOErrCode,
    pub message: *mut c_char,
}

#[repr(C)]
pub struct CGOSharedDirectoryAnnounce {
    pub directory_id: u32,
    pub name: *const c_char,
}

pub type CGOSharedDirectoryAcknowledge = SharedDirectoryAcknowledge;

#[repr(C)]
pub struct CGOSharedDirectoryInfoRequest {
    pub completion_id: u32,
    pub directory_id: u32,
    pub path: *const c_char,
}

#[repr(C)]
pub struct CGOSharedDirectoryInfoResponse {
    pub completion_id: u32,
    pub err_code: TdpErrCode,
    pub fso: CGOFileSystemObject,
}

#[repr(C)]
#[derive(Clone)]
pub struct CGOFileSystemObject {
    pub last_modified: u64,
    pub size: u64,
    pub file_type: FileType,
    pub is_empty: u8,
    pub path: *const c_char,
}

impl From<CGOFileSystemObject> for FileSystemObject {
    fn from(cgo_fso: CGOFileSystemObject) -> FileSystemObject {
        // # Safety
        //
        // This function MUST NOT hang on to any of the pointers passed in to it after it returns.
        // In other words, all pointer data that needs to persist after this function returns MUST
        // be copied into Rust-owned memory.
        unsafe {
            FileSystemObject {
                last_modified: cgo_fso.last_modified,
                size: cgo_fso.size,
                file_type: cgo_fso.file_type,
                is_empty: cgo_fso.is_empty,
                path: UnixPath::from(from_c_string(cgo_fso.path)),
            }
        }
    }
}

#[derive(Debug)]
#[repr(C)]
pub struct CGOSharedDirectoryWriteRequest {
    pub completion_id: u32,
    pub directory_id: u32,
    pub offset: u64,
    pub path_length: u32,
    pub path: *const c_char,
    pub write_data_length: u32,
    pub write_data: *mut u8,
}

#[repr(C)]
pub struct CGOSharedDirectoryReadRequest {
    pub completion_id: u32,
    pub directory_id: u32,
    pub path_length: u32,
    pub path: *const c_char,
    pub offset: u64,
    pub length: u32,
}

#[derive(Debug)]
#[repr(C)]
pub struct CGOSharedDirectoryReadResponse {
    pub completion_id: u32,
    pub err_code: TdpErrCode,
    pub read_data_length: u32,
    pub read_data: *mut u8,
}

pub type CGOSharedDirectoryWriteResponse = SharedDirectoryWriteResponse;

#[repr(C)]
pub struct CGOSharedDirectoryCreateRequest {
    pub completion_id: u32,
    pub directory_id: u32,
    pub file_type: FileType,
    pub path: *const c_char,
}

#[repr(C)]
pub struct CGOSharedDirectoryListResponse {
    completion_id: u32,
    err_code: TdpErrCode,
    fso_list_length: u32,
    fso_list: *mut CGOFileSystemObject,
}

#[repr(C)]
pub struct CGOSharedDirectoryMoveRequest {
    pub completion_id: u32,
    pub directory_id: u32,
    pub original_path: *const c_char,
    pub new_path: *const c_char,
}

#[repr(C)]
pub struct CGOSharedDirectoryCreateResponse {
    pub completion_id: u32,
    pub err_code: TdpErrCode,
    pub fso: CGOFileSystemObject,
}

#[repr(C)]
pub struct CGOSharedDirectoryDeleteRequest {
    pub completion_id: u32,
    pub directory_id: u32,
    pub path: *const c_char,
}

pub type CGOSharedDirectoryDeleteResponse = SharedDirectoryDeleteResponse;

pub type CGOSharedDirectoryMoveResponse = SharedDirectoryMoveResponse;

#[repr(C)]
pub struct CGOSharedDirectoryListRequest {
    pub completion_id: u32,
    pub directory_id: u32,
    pub path: *const c_char,
}

#[repr(C)]
pub struct CGOSharedDirectoryTruncateRequest {
    pub completion_id: u32,
    pub directory_id: u32,
    pub path: *const c_char,
    pub end_of_file: u32,
}

pub type CGOSharedDirectoryTruncateResponse = SharedDirectoryTruncateResponse;

// These functions are defined on the Go side.
// Look for functions with '//export funcname' comments.
extern "C" {
    fn cgo_handle_remote_copy(cgo_handle: CgoHandle, data: *mut u8, len: u32) -> CGOErrCode;
    fn cgo_handle_fastpath_pdu(cgo_handle: CgoHandle, data: *mut u8, len: u32) -> CGOErrCode;
    fn cgo_handle_rdp_connection_initialized(
        cgo_handle: CgoHandle,
        io_channel_id: u16,
        user_channel_id: u16,
        screen_width: u16,
        screen_height: u16,
    ) -> CGOErrCode;
    fn cgo_tdp_sd_acknowledge(
        cgo_handle: CgoHandle,
        ack: *mut CGOSharedDirectoryAcknowledge,
    ) -> CGOErrCode;
    fn cgo_tdp_sd_info_request(
        cgo_handle: CgoHandle,
        req: *mut CGOSharedDirectoryInfoRequest,
    ) -> CGOErrCode;
    fn cgo_tdp_sd_create_request(
        cgo_handle: CgoHandle,
        req: *mut CGOSharedDirectoryCreateRequest,
    ) -> CGOErrCode;
    fn cgo_tdp_sd_delete_request(
        cgo_handle: CgoHandle,
        req: *mut CGOSharedDirectoryDeleteRequest,
    ) -> CGOErrCode;
    fn cgo_tdp_sd_list_request(
        cgo_handle: CgoHandle,
        req: *mut CGOSharedDirectoryListRequest,
    ) -> CGOErrCode;
    fn cgo_tdp_sd_read_request(
        cgo_handle: CgoHandle,
        req: *mut CGOSharedDirectoryReadRequest,
    ) -> CGOErrCode;
    fn cgo_tdp_sd_write_request(
        cgo_handle: CgoHandle,
        req: *mut CGOSharedDirectoryWriteRequest,
    ) -> CGOErrCode;
    fn cgo_tdp_sd_move_request(
        cgo_handle: CgoHandle,
        req: *mut CGOSharedDirectoryMoveRequest,
    ) -> CGOErrCode;
    fn cgo_tdp_sd_truncate_request(
        cgo_handle: CgoHandle,
        req: *mut CGOSharedDirectoryTruncateRequest,
    ) -> CGOErrCode;
}

/// A [cgo.Handle] passed to us by Go.
///
/// [cgo.Handle]: https://pkg.go.dev/runtime/cgo#Handle
type CgoHandle = usize;
