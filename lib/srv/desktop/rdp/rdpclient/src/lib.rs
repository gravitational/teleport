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

// bring in rdp-decoder to export its unmangled symbols in the staticlib
extern crate rdp_decoder as _;

use crate::client::global::get_client_handle;
use crate::client::Client;
use crate::rdpdr::tdp::SharedDirectoryAnnounce;
use client::{ClientHandle, ClientResult, ConnectParams, MonitorSpec};
use ironrdp_session::x224::DisconnectDescription;
use log::{error, trace, warn};
use rdpdr::path::UnixPath;
use rdpdr::tdp::{
    FileSystemObject, FileType, SharedDirectoryAcknowledge, SharedDirectoryCreateResponse,
    SharedDirectoryDeleteResponse, SharedDirectoryInfoResponse, SharedDirectoryListResponse,
    SharedDirectoryMoveResponse, SharedDirectoryReadResponse, SharedDirectoryRemove,
    SharedDirectoryTruncateResponse, SharedDirectoryWriteResponse, TdpErrCode,
};
use std::ffi::CString;
use std::fmt::Debug;
use std::io::ErrorKind;
use std::os::raw::c_char;
use std::ptr;
use util::{from_c_string, from_go_array};
pub mod client;
mod cliprdr;
mod egfx;
mod license;
#[cfg(feature = "desktop-encoder")]
mod linux_desktop_encoder;
mod network_client;
mod piv;
mod rdpdr;
mod rfx_video_probe;
mod ssl;
mod util;

/// rdpclient_init_log should be called at initialization time to set up
/// logging on the rdpclient side.
#[no_mangle]
pub extern "C" fn rdpclient_init_log() {
    if let Err(e) = env_logger::try_init() {
        eprintln!("failed to initialize Rust logger: {e}");
    }
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
/// go_addr, go_domain, go_kdc, cert_der, key_der point to valid buffers.
#[no_mangle]
pub unsafe extern "C" fn client_run(cgo_handle: CgoHandle, params: CGOConnectParams) -> CGOResult {
    trace!("client_run");
    // Convert from C to Rust types.
    let username = from_c_string(params.go_username);
    let addr = from_c_string(params.go_addr);
    let cert_der = from_go_array(params.cert_der, params.cert_der_len);
    let key_der = from_go_array(params.key_der, params.key_der_len);

    let kdc = from_c_string(params.go_kdc_addr);
    let kdc = if kdc.is_empty() { None } else { Some(kdc) };

    let computer_name = from_c_string(params.go_computer_name);
    let computer_name = if computer_name.is_empty() {
        None
    } else {
        Some(computer_name)
    };

    let monitors = monitors_from_c(params.monitors, params.monitors_len);

    match Client::run(
        cgo_handle,
        ConnectParams {
            ad: params.ad,
            nla: params.nla,
            username,
            addr,
            computer_name,
            cert_der,
            key_der,
            kdc_addr: kdc,
            screen_width: params.screen_width,
            screen_height: params.screen_height,
            screen_scale: params.screen_scale,
            monitors,
            allow_clipboard: params.allow_clipboard,
            allow_directory_sharing: params.allow_directory_sharing,
            show_desktop_wallpaper: params.show_desktop_wallpaper,
            client_id: params.client_id,
            keyboard_layout: params.keyboard_layout,
        },
    ) {
        Ok(res) => CGOResult {
            err_code: CGOErrCode::ErrCodeSuccess,
            message: match res {
                Some(DisconnectDescription::McsDisconnect(reason)) => {
                    CString::new(reason.description().to_string())
                        .map(|c| c.into_raw())
                        .unwrap_or(ptr::null_mut())
                }
                Some(DisconnectDescription::ErrorInfo(info)) => {
                    CString::new(info.description().to_string())
                        .map(|c| c.into_raw())
                        .unwrap_or(ptr::null_mut())
                }
                None => ptr::null_mut(),
            },
        },
        Err(e) => {
            error!("client_run failed: {:?}", e);
            let message = match e {
                client::ClientError::Tcp(io_err) if io_err.kind() == ErrorKind::TimedOut => {
                    String::from(TIMEOUT_ERROR_MESSAGE)
                }
                _ => format!("{}", e),
            };
            CGOResult {
                err_code: CGOErrCode::ErrCodeFailure,
                message: CString::new(message)
                    .map(|c| c.into_raw())
                    .unwrap_or(ptr::null_mut()),
            }
        }
    }
}

const TIMEOUT_ERROR_MESSAGE: &str = "Connection Timed Out\n\n\
Teleport could not connect to the host within the timeout period. \
This could be due to a firewall blocking connections, an overloaded system, \
or network congestion. To resolve this issue, ensure that the Teleport agent \
has connectivity to the Windows host.\n\n\
Use \"nc -vz HOST 3389\" to help debug this issue.";

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

/// client_handle_tdp_sd_remove removes a drive that has been redirected over RDP
///
///
/// # Safety
///
/// `cgo_handle` must be a valid handle.
///
#[no_mangle]
pub unsafe extern "C" fn client_handle_tdp_sd_remove(
    cgo_handle: CgoHandle,
    sd_remove: CGOSharedDirectoryRemove,
) -> CGOErrCode {
    let sd_remove = SharedDirectoryRemove::from(sd_remove);
    handle_operation(
        cgo_handle,
        "client_handle_tdp_sd_remove",
        move |client_handle| client_handle.handle_tdp_sd_remove(sd_remove),
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

/// client_handle_tdp_refresh_rect handles a TDP RefreshRect message by
/// constructing an RDP Refresh Rect PDU (MS-RDPBCGR § 2.2.11.2.1) for
/// the given inclusive pixel region and sending it to the Windows host.
/// Used by the browser to clear RFX decode trails after a window drag.
///
/// # Safety
///
/// `cgo_handle` must be a valid handle.
#[no_mangle]
pub unsafe extern "C" fn client_handle_tdp_refresh_rect(
    cgo_handle: CgoHandle,
    left: u16,
    top: u16,
    right: u16,
    bottom: u16,
) -> CGOErrCode {
    handle_operation(
        cgo_handle,
        "client_handle_tdp_refresh_rect",
        move |client_handle| client_handle.write_refresh_rect(left, top, right, bottom),
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

/// # Safety
///
/// `cgo_handle` must be a valid handle. `monitors` must point to `monitors_len`
/// valid `CGOMonitorLayout` entries, or be null if `monitors_len` is zero.
#[no_mangle]
pub unsafe extern "C" fn client_write_screen_resize(
    cgo_handle: CgoHandle,
    width: u32,
    height: u32,
    scale: u32,
    monitors: *const CGOMonitorLayout,
    monitors_len: u32,
) -> CGOErrCode {
    let monitors = monitors_from_c(monitors, monitors_len);
    // width and height are the bounding-box of the virtual desktop. Currently
    // unused on the Rust side because `monitors` is authoritative for the
    // DisplayControl PDU, but kept on the wire for parity with the proto and
    // for logging on the Go side.
    let _ = (width, height);
    handle_operation(
        cgo_handle,
        "client_write_screen_resize",
        move |client_handle| client_handle.write_screen_resize(monitors, scale),
    )
}

/// Copies a slice of `CGOMonitorLayout` from Go into an owned Rust `Vec`.
///
/// # Safety
///
/// `monitors` must point to `monitors_len` valid entries, or be null when
/// `monitors_len` is zero.
unsafe fn monitors_from_c(monitors: *const CGOMonitorLayout, monitors_len: u32) -> Vec<MonitorSpec> {
    if monitors.is_null() || monitors_len == 0 {
        return Vec::new();
    }
    std::slice::from_raw_parts(monitors, monitors_len as usize)
        .iter()
        .map(|m| MonitorSpec {
            x: m.x,
            y: m.y,
            width: m.width,
            height: m.height,
            is_primary: m.is_primary,
        })
        .collect()
}

#[repr(C)]
pub struct CGOConnectParams {
    ad: bool,
    nla: bool,
    go_username: *const c_char,
    go_addr: *const c_char,
    go_domain: *const c_char,
    go_kdc_addr: *const c_char,
    go_computer_name: *const c_char,
    cert_der_len: u32,
    cert_der: *mut u8,
    key_der_len: u32,
    key_der: *mut u8,
    screen_width: u16,
    screen_height: u16,
    screen_scale: u16,
    /// Pointer to a Go-owned slice of `CGOMonitorLayout` describing the client's
    /// monitors. Bounded at 4 entries server-side. Always has at least one
    /// entry (the primary). The pointer is only valid for the duration of the
    /// `client_run` call; Rust copies the entries during initialization.
    monitors: *const CGOMonitorLayout,
    monitors_len: u32,
    allow_clipboard: bool,
    allow_directory_sharing: bool,
    show_desktop_wallpaper: bool,
    client_id: [u32; 4],
    keyboard_layout: u32,
}

/// One monitor's position and size within the RDP virtual desktop, as sent
/// from Go. Mirrors the `MonitorLayout` proto. Coordinates may be negative for
/// monitors arranged to the left of or above the primary; Rust normalizes to
/// place the primary at the origin before encoding the DisplayControl PDU.
#[repr(C)]
#[derive(Copy, Clone, Debug)]
pub struct CGOMonitorLayout {
    pub x: i32,
    pub y: i32,
    pub width: u32,
    pub height: u32,
    pub is_primary: bool,
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
    ErrCodeNotFound = 3,
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
pub struct CGOSharedDirectoryRemove {
    pub directory_id: u32,
}

#[repr(C)]
pub struct CGOSharedDirectoryInfoRequest {
    pub completion_id: u32,
    pub directory_id: u32,
    pub path: *const c_char,
}

#[repr(C)]
pub struct CGOSharedDirectoryInfoResponse {
    pub completion_id: u32,
    pub directory_id: u32,
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
    pub directory_id: u32,
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
    directory_id: u32,
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
    pub directory_id: u32,
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
    pub end_of_file: u64,
}

pub type CGOSharedDirectoryTruncateResponse = SharedDirectoryTruncateResponse;

// These functions are defined on the Go side.
// Look for functions with '//export funcname' comments.
extern "C" {
    fn cgo_free_rdp_license(data: *mut u8);
    fn cgo_read_rdp_license(
        cgo_handle: CgoHandle,
        req: *mut CGOLicenseRequest,
        data_out: *mut *mut u8,
        len_out: *mut usize,
    ) -> CGOErrCode;
    fn cgo_write_rdp_license(
        cgo_handle: CgoHandle,
        req: *mut CGOLicenseRequest,
        data: *mut u8,
        length: usize,
    ) -> CGOErrCode;
    fn cgo_handle_remote_copy(cgo_handle: CgoHandle, data: *mut u8, len: u32) -> CGOErrCode;
    fn cgo_handle_fastpath_pdu(cgo_handle: CgoHandle, data: *mut u8, len: u32) -> CGOErrCode;
    /// Forwards a decoded EGFX (RDPGFX) bitmap update to Go for relay to the
    /// browser. `rgba` is `width * height * 4` bytes of row-major RGBA, and
    /// `(desktop_x, desktop_y)` is the top-left position in desktop coords
    /// (after applying the source surface's `MapSurfaceToOutput` origin).
    /// Pointer type and length type match the `cgo_handle_fastpath_pdu`
    /// pattern so Go's `//export`-generated header agrees (Go's exports
    /// can't use `const` pointers and use `C.uint32_t` for sizes).
    fn cgo_handle_egfx_bitmap(
        cgo_handle: CgoHandle,
        desktop_x: u32,
        desktop_y: u32,
        width: u32,
        height: u32,
        rgba: *mut u8,
        rgba_len: u32,
    ) -> CGOErrCode;
    /// Forwards a parsed AVC444/v2 frame to Go for relay to the browser,
    /// where the H.264 streams are decoded with the browser's WebCodecs
    /// VideoDecoder. The server only unpacks the
    /// RFX_AVC444V2_BITMAP_STREAM wrapper; the H.264 NAL units in
    /// `luma_h264` / `chroma_h264` are forwarded unchanged in AVC format
    /// (4-byte BE length per NAL). `codec_id` is the EGFX codec ID
    /// (0xe = Avc444, 0xf = Avc444v2) and `encoding` is the stream-presence
    /// flag (0 = both, 1 = luma only, 2 = chroma only).
    fn cgo_handle_egfx_avc_frame(
        cgo_handle: CgoHandle,
        surface_id: u32,
        desktop_x: u32,
        desktop_y: u32,
        dest_width: u32,
        dest_height: u32,
        codec_id: u32,
        encoding: u32,
        luma_h264: *mut u8,
        luma_h264_len: u32,
        chroma_h264: *mut u8,
        chroma_h264_len: u32,
    ) -> CGOErrCode;
    /// Forwards a raw ClearCodec ([MS-RDPEGFX] 2.2.4.2) PDU body to Go for
    /// relay to the wasm client. The wasm side runs the ClearCodec decoder
    /// in-place against the framebuffer image, preserving existing pixels
    /// for sub-regions the PDU doesn't paint. `dest_x` / `dest_y` are in
    /// desktop coordinates after applying the surface's MapSurfaceToOutput
    /// origin; the wasm decoder writes into the rect `(dest_x, dest_y) ..
    /// (dest_x + width, dest_y + height)` of the virtual desktop image.
    fn cgo_handle_egfx_clearcodec(
        cgo_handle: CgoHandle,
        surface_id: u32,
        dest_x: i32,
        dest_y: i32,
        width: u32,
        height: u32,
        pdu_data: *mut u8,
        pdu_data_len: u32,
    ) -> CGOErrCode;
    /// EGFX Uncompressed (`Codec1Type::Uncompressed`, codec_id 0x0). Raw
    /// pixel passthrough: bytes laid out in the surface's declared
    /// `pixel_format` (PIXEL_FORMAT_XRGB_8888 = 0x20 / PIXEL_FORMAT_ARGB_8888
    /// = 0x21 — both `[B, G, R, X/A]` in little-endian memory). The wasm
    /// side does the channel reorder and (for ARGB) source-over composite
    /// against the existing framebuffer.
    fn cgo_handle_egfx_uncompressed(
        cgo_handle: CgoHandle,
        surface_id: u32,
        dest_x: i32,
        dest_y: i32,
        width: u32,
        height: u32,
        pixel_format: u32,
        bitmap_data: *mut u8,
        bitmap_data_len: u32,
    ) -> CGOErrCode;
    /// EGFX Planar (`Codec1Type::Planar`, codec_id 0x0a). Raw passthrough;
    /// wasm decodes via `ironrdp_graphics::rdp6::bitmap_stream`.
    fn cgo_handle_egfx_planar(
        cgo_handle: CgoHandle,
        surface_id: u32,
        dest_x: i32,
        dest_y: i32,
        width: u32,
        height: u32,
        pdu_data: *mut u8,
        pdu_data_len: u32,
    ) -> CGOErrCode;
    /// EGFX Avc420 (`Codec1Type::Avc420`, codec_id 0x0b). Raw passthrough;
    /// wasm parses the `Avc420EncapsulatedBitmapStream` envelope and feeds
    /// H.264 NAL units to the browser's `VideoDecoder`.
    fn cgo_handle_egfx_avc420(
        cgo_handle: CgoHandle,
        surface_id: u32,
        dest_x: i32,
        dest_y: i32,
        width: u32,
        height: u32,
        pdu_data: *mut u8,
        pdu_data_len: u32,
    ) -> CGOErrCode;
    /// EGFX SolidFill ([MS-RDPEGFX] 2.2.2.4): paint `rect_count` rectangles
    /// on the given surface with a single (B, G, R) color. `rects` is a flat
    /// buffer of `rect_count * 4` u32s ordered (left, top, right, bottom).
    fn cgo_handle_egfx_solid_fill(
        cgo_handle: CgoHandle,
        surface_id: u32,
        color_b: u32,
        color_g: u32,
        color_r: u32,
        rect_count: u32,
        rects: *mut u32,
    ) -> CGOErrCode;
    /// EGFX SurfaceToCache ([MS-RDPEGFX] 2.2.2.6): snapshot the given
    /// surface region into bitmap cache slot `cache_slot`.
    fn cgo_handle_egfx_surface_to_cache(
        cgo_handle: CgoHandle,
        surface_id: u32,
        cache_key: u64,
        cache_slot: u32,
        src_left: u32,
        src_top: u32,
        src_right: u32,
        src_bottom: u32,
    ) -> CGOErrCode;
    /// EGFX CacheToSurface ([MS-RDPEGFX] 2.2.2.7): blit cache slot
    /// contents onto the surface at each (x, y) point. `points` is a flat
    /// buffer of `point_count * 2` u32s.
    fn cgo_handle_egfx_cache_to_surface(
        cgo_handle: CgoHandle,
        surface_id: u32,
        cache_slot: u32,
        point_count: u32,
        points: *mut u32,
    ) -> CGOErrCode;
    /// EGFX EvictCacheEntry ([MS-RDPEGFX] 2.2.2.8): drop the given slot.
    fn cgo_handle_egfx_evict_cache_entry(
        cgo_handle: CgoHandle,
        cache_slot: u32,
    ) -> CGOErrCode;
    /// EGFX EndFrame ([MS-RDPEGFX] 2.2.2.15): the server finished a logical
    /// frame; the client should present (flush) now.
    fn cgo_handle_egfx_end_frame(cgo_handle: CgoHandle, frame_id: u32) -> CGOErrCode;
    /// EGFX SurfaceToSurface ([MS-RDPEGFX] 2.2.2.5): copy a source-surface
    /// region to each (x, y) point on the destination surface. `points` is
    /// a flat buffer of `point_count * 2` u32s.
    fn cgo_handle_egfx_surface_to_surface(
        cgo_handle: CgoHandle,
        source_surface_id: u32,
        destination_surface_id: u32,
        src_left: u32,
        src_top: u32,
        src_right: u32,
        src_bottom: u32,
        point_count: u32,
        points: *mut u32,
    ) -> CGOErrCode;
    /// EGFX WireToSurface2 ([MS-RDPEGFX] 2.2.2.2): forward a raw RFX
    /// Progressive payload (`bitmap_data`) plus the metadata the wasm
    /// decoder needs to maintain per-(surface, codec_context_id) state and
    /// translate per-tile dest rects into desktop coordinates.
    fn cgo_handle_egfx_wire_to_surface2(
        cgo_handle: CgoHandle,
        surface_id: u32,
        codec_id: u32,
        codec_context_id: u32,
        pixel_format: u32,
        surface_origin_x: u32,
        surface_origin_y: u32,
        bitmap_data: *mut u8,
        bitmap_data_len: u32,
    ) -> CGOErrCode;
    /// EGFX DeleteEncodingContext ([MS-RDPEGFX] 2.2.2.3): drop the wasm-side
    /// progressive decoder state for the given (surface, codec_context_id).
    fn cgo_handle_egfx_delete_encoding_context(
        cgo_handle: CgoHandle,
        surface_id: u32,
        codec_context_id: u32,
    ) -> CGOErrCode;
    fn cgo_handle_rdp_connection_activated(
        cgo_handle: CgoHandle,
        io_channel_id: u16,
        user_channel_id: u16,
        screen_width: u16,
        screen_height: u16,
        share_id: u32,
    ) -> CGOErrCode;
    /// Notify Go that the EGFX virtual desktop was resized mid-session (a
    /// `ResetGraphics` PDU — e.g. a monitor was added/moved/resized via
    /// DisplayControl). Go re-announces the new desktop size to the browser so
    /// the wasm decoder grows its framebuffer to the new bounding box.
    fn cgo_handle_rdp_reset_graphics(
        cgo_handle: CgoHandle,
        width: u32,
        height: u32,
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
pub(crate) type CgoHandle = usize;

#[repr(C)]
pub struct CGOLicenseRequest {
    version: u32,
    issuer: *const c_char,
    company: *const c_char,
    product_id: *const c_char,
}
