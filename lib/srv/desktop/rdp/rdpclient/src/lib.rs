// Copyright 2021 Gravitational, Inc
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

mod cliprdr;
mod errors;
mod piv;
mod rdpdr;
mod util;
mod vchan;

#[macro_use]
extern crate log;
#[macro_use]
extern crate num_derive;

use libc::{fd_set, select, FD_SET};
use rand::Rng;
use rand::SeedableRng;
use rdp::core::event::*;
use rdp::core::gcc::KeyboardLayout;
use rdp::core::global;
use rdp::core::mcs;
use rdp::core::sec;
use rdp::core::tpkt;
use rdp::core::x224;
use rdp::model::error::{Error as RdpError, RdpError as RdpProtocolError, RdpErrorKind, RdpResult};
use rdp::model::link::{Link, Stream};
use rdpdr::ServerCreateDriveRequest;
use std::convert::TryFrom;
use std::ffi::{CStr, CString};
use std::io::Error as IoError;
use std::io::ErrorKind;
use std::io::{Cursor, Read, Write};
use std::net::{TcpStream, ToSocketAddrs};
use std::os::raw::c_char;
use std::os::unix::io::AsRawFd;
use std::sync::{Arc, Mutex};
use std::{mem, ptr, slice, time};

#[no_mangle]
pub extern "C" fn init() {
    env_logger::try_init().unwrap_or_else(|e| println!("failed to initialize Rust logger: {}", e));
}

/// Client has an unusual lifecycle:
/// - connect_rdp creates it on the heap, grabs a raw pointer and returns in to Go
/// - most other exported rdp functions take the raw pointer, convert it to a reference for use
///   without dropping the Client
/// - free_rdp takes the raw pointer and drops it
///
/// All of the exported rdp functions could run concurrently, so the rdp_client is synchronized.
/// tcp_fd is only set in connect_rdp and used as read-only afterwards, so it does not need
/// synchronization.
pub struct Client {
    rdp_client: Arc<Mutex<RdpClient<TcpStream>>>,
    tcp_fd: usize,
    go_ref: usize,
}

impl Client {
    fn into_raw(self: Box<Self>) -> *mut Self {
        Box::into_raw(self)
    }
    unsafe fn from_ptr<'a>(ptr: *const Self) -> Result<&'a Client, CGOErrCode> {
        match ptr.as_ref() {
            Some(c) => Ok(c),
            None => {
                error!("invalid Rust client pointer");
                Err(CGOErrCode::ErrCodeFailure)
            }
        }
    }
    unsafe fn from_raw(ptr: *mut Self) -> Box<Self> {
        Box::from_raw(ptr)
    }
}

#[repr(C)]
pub struct ClientOrError {
    client: *mut Client,
    err: CGOErrCode,
}

impl From<Result<Client, ConnectError>> for ClientOrError {
    fn from(r: Result<Client, ConnectError>) -> ClientOrError {
        match r {
            Ok(client) => ClientOrError {
                client: Box::new(client).into_raw(),
                err: CGOErrCode::ErrCodeSuccess,
            },
            Err(e) => {
                error!("{:?}", e);
                ClientOrError {
                    client: ptr::null_mut(),
                    err: CGOErrCode::ErrCodeFailure,
                }
            }
        }
    }
}

/// connect_rdp establishes an RDP connection to go_addr with the provided credentials and screen
/// size. If succeeded, the client is internally registered under client_ref. When done with the
/// connection, the caller must call close_rdp.
///
/// # Safety
///
/// The caller mmust ensure that go_addr, go_username, cert_der, key_der point to valid buffers in respect
/// to their corresponding parameters.
#[no_mangle]
pub unsafe extern "C" fn connect_rdp(
    go_ref: usize,
    go_addr: *const c_char,
    go_username: *const c_char,
    cert_der_len: u32,
    cert_der: *mut u8,
    key_der_len: u32,
    key_der: *mut u8,
    screen_width: u16,
    screen_height: u16,
    allow_clipboard: bool,
    allow_directory_sharing: bool,
) -> ClientOrError {
    // Convert from C to Rust types.
    let addr = from_go_string(go_addr);
    let username = from_go_string(go_username);
    let cert_der = from_go_array(cert_der_len, cert_der);
    let key_der = from_go_array(key_der_len, key_der);

    connect_rdp_inner(
        go_ref,
        &addr,
        ConnectParams {
            username,
            cert_der,
            key_der,
            screen_width,
            screen_height,
            allow_clipboard,
            allow_directory_sharing,
        },
    )
    .into()
}

#[derive(Debug)]
enum ConnectError {
    Tcp(IoError),
    Rdp(RdpError),
    InvalidAddr(),
}

impl From<IoError> for ConnectError {
    fn from(e: IoError) -> ConnectError {
        ConnectError::Tcp(e)
    }
}

impl From<RdpError> for ConnectError {
    fn from(e: RdpError) -> ConnectError {
        ConnectError::Rdp(e)
    }
}

const RDP_CONNECT_TIMEOUT: time::Duration = time::Duration::from_secs(5);
const RDPSND_CHANNEL_NAME: &str = "rdpsnd";

struct ConnectParams {
    username: String,
    cert_der: Vec<u8>,
    key_der: Vec<u8>,
    screen_width: u16,
    screen_height: u16,
    allow_clipboard: bool,
    allow_directory_sharing: bool,
}

fn connect_rdp_inner(
    go_ref: usize,
    addr: &str,
    params: ConnectParams,
) -> Result<Client, ConnectError> {
    // Connect and authenticate.
    let addr = addr
        .to_socket_addrs()?
        .next()
        .ok_or(ConnectError::InvalidAddr())?;
    let tcp = TcpStream::connect_timeout(&addr, RDP_CONNECT_TIMEOUT)?;
    let tcp_fd = tcp.as_raw_fd() as usize;
    // Domain name "." means current domain.
    let domain = ".";

    // From rdp-rs/src/core/client.rs
    let tcp = Link::new(Stream::Raw(tcp));
    let protocols = x224::Protocols::ProtocolSSL as u32 | x224::Protocols::ProtocolRDP as u32;
    let x224 = x224::Client::connect(tpkt::Client::new(tcp), protocols, false, None, false, false)?;
    let mut mcs = mcs::Client::new(x224);

    // request the static channels we'll need:
    // rdpdr: derive redirection (smart cards)
    // rdpsnd: sound (for some reason we need to request this)
    // cliprdr: clipboard
    let mut static_channels = vec![
        rdpdr::CHANNEL_NAME.to_string(),
        RDPSND_CHANNEL_NAME.to_string(),
    ];
    if params.allow_clipboard {
        static_channels.push(cliprdr::CHANNEL_NAME.to_string())
    }
    mcs.connect(
        "rdp-rs".to_string(),
        params.screen_width,
        params.screen_height,
        KeyboardLayout::US,
        &static_channels,
    )?;
    // Generate a random 8-digit PIN for our smartcard.
    let mut rng = rand_chacha::ChaCha20Rng::from_entropy();
    let pin = format!("{:08}", rng.gen_range(0i32..=99999999i32));
    sec::connect(
        &mut mcs,
        &domain.to_string(),
        &params.username,
        &pin,
        true,
        // InfoPasswordIsScPin means that the user will not be prompted for the smartcard PIN code,
        // which is known only to Teleport and unique for each RDP session.
        Some(sec::InfoFlag::InfoPasswordIsScPin as u32 | sec::InfoFlag::InfoMouseHasWheel as u32),
        Some(
            sec::ExtendedInfoFlag::PerfDisableCursorBlink as u32
                | sec::ExtendedInfoFlag::PerfDisableFullWindowDrag as u32
                | sec::ExtendedInfoFlag::PerfDisableMenuAnimations as u32,
        ),
    )?;
    // Client for the "global" channel - video output and user input.
    let global = global::Client::new(
        mcs.get_user_id(),
        mcs.get_global_channel_id(),
        params.screen_width,
        params.screen_height,
        KeyboardLayout::US,
        "rdp-rs",
    );

    let tdp_sd_acknowledge = Box::new(move |ack: SharedDirectoryAcknowledge| -> RdpResult<()> {
        debug!("sending: {:?}", ack);
        unsafe {
            if tdp_sd_acknowledge(go_ref, &mut CGOSharedDirectoryAcknowledge::from(ack))
                != CGOErrCode::ErrCodeSuccess
            {
                return Err(RdpError::TryError(String::from(
                    "call to tdp_sd_acknowledge failed",
                )));
            }
        }
        Ok(())
    });

    let tdp_sd_info_request = Box::new(move |req: SharedDirectoryInfoRequest| -> RdpResult<()> {
        debug!("sending: {:?}", req);
        // Create C compatible string from req.path
        match CString::new(req.path.clone()) {
            Ok(c_string) => {
                unsafe {
                    let err = tdp_sd_info_request(
                        go_ref,
                        &mut CGOSharedDirectoryInfoRequest {
                            completion_id: req.completion_id,
                            directory_id: req.directory_id,
                            path: c_string.as_ptr(),
                        },
                    );
                    if err != CGOErrCode::ErrCodeSuccess {
                        return Err(RdpError::TryError(String::from(
                            "call to tdp_sd_info_request failed",
                        )));
                    };
                }
                Ok(())
            }
            Err(_) => {
                // TODO(isaiah): change TryError to TeleportError for a generic error caused by Teleport specific code.
                return Err(RdpError::TryError(format!(
                    "path contained characters that couldn't be converted to a C string: {}",
                    req.path
                )));
            }
        }
    });

    // Client for the "rdpdr" channel - smartcard emulation and drive redirection.
    let rdpdr = rdpdr::Client::new(
        params.cert_der,
        params.key_der,
        pin,
        params.allow_directory_sharing,
        tdp_sd_acknowledge,
        tdp_sd_info_request,
    );

    // Client for the "cliprdr" channel - clipboard sharing.
    let cliprdr = if params.allow_clipboard {
        Some(cliprdr::Client::new(Box::new(move |v| -> RdpResult<()> {
            unsafe {
                if handle_remote_copy(go_ref, v.as_ptr() as _, v.len() as u32)
                    != CGOErrCode::ErrCodeSuccess
                {
                    return Err(errors::try_error("failed to handle remote copy"));
                }
            }
            Ok(())
        })))
    } else {
        None
    };

    let rdp_client = RdpClient {
        mcs,
        global,
        rdpdr,
        cliprdr,
    };
    Ok(Client {
        rdp_client: Arc::new(Mutex::new(rdp_client)),
        tcp_fd,
        go_ref,
    })
}

/// From rdp-rs/src/core/client.rs
struct RdpClient<S> {
    mcs: mcs::Client<S>,
    global: global::Client,
    rdpdr: rdpdr::Client,

    cliprdr: Option<cliprdr::Client>,
}

impl<S: Read + Write> RdpClient<S> {
    pub fn read<T>(&mut self, callback: T) -> RdpResult<()>
    where
        T: FnMut(RdpEvent),
    {
        let (channel_name, message) = self.mcs.read()?;
        // De-multiplex static channels. Forward messages to the correct channel client based on
        // name.
        match channel_name.as_str() {
            "global" => self.global.read(message, &mut self.mcs, callback),
            rdpdr::CHANNEL_NAME => self.rdpdr.read_and_reply(message, &mut self.mcs),
            cliprdr::CHANNEL_NAME => match self.cliprdr {
                Some(ref mut clip) => clip.read_and_reply(message, &mut self.mcs),
                None => Ok(()),
            },
            RDPSND_CHANNEL_NAME => {
                debug!("skipping RDPSND message, audio output not supported");
                Ok(())
            }
            _ => Err(RdpError::RdpError(RdpProtocolError::new(
                RdpErrorKind::UnexpectedType,
                &format!("Invalid channel name {:?}", channel_name),
            ))),
        }
    }

    pub fn write(&mut self, event: RdpEvent) -> RdpResult<()> {
        match event {
            RdpEvent::Pointer(pointer) => {
                self.global.write_input_event(pointer.into(), &mut self.mcs)
            }
            RdpEvent::Key(key) => self.global.write_input_event(key.into(), &mut self.mcs),
            _ => Err(RdpError::RdpError(RdpProtocolError::new(
                RdpErrorKind::UnexpectedType,
                "RDPCLIENT: This event can't be sent",
            ))),
        }
    }

    pub fn write_client_device_list_announce(
        &mut self,
        req: rdpdr::ClientDeviceListAnnounce,
    ) -> RdpResult<()> {
        self.rdpdr
            .write_client_device_list_announce(req, &mut self.mcs)
    }

    pub fn handle_tdp_sd_info_response(
        &mut self,
        res: SharedDirectoryInfoResponse,
    ) -> RdpResult<()> {
        self.rdpdr.handle_tdp_sd_info_response(res, &mut self.mcs)
    }

    pub fn shutdown(&mut self) -> RdpResult<()> {
        self.mcs.shutdown()
    }
}

/// CGOBitmap is a CGO-compatible version of BitmapEvent that we pass back to Go.
/// BitmapEvent is a video output update from the server.
#[repr(C)]
pub struct CGOBitmap {
    pub dest_left: u16,
    pub dest_top: u16,
    pub dest_right: u16,
    pub dest_bottom: u16,
    /// The memory of this field is managed by the Rust side.
    pub data_ptr: *mut u8,
    pub data_len: usize,
    pub data_cap: usize,
}

impl TryFrom<BitmapEvent> for CGOBitmap {
    type Error = RdpError;

    fn try_from(e: BitmapEvent) -> Result<Self, Self::Error> {
        let mut res = CGOBitmap {
            dest_left: e.dest_left,
            dest_top: e.dest_top,
            dest_right: e.dest_right,
            dest_bottom: e.dest_bottom,
            data_ptr: ptr::null_mut(),
            data_len: 0,
            data_cap: 0,
        };

        // e.decompress consumes e, so we need to call it separately, after populating the fields
        // above.
        let mut data = if e.is_compress {
            e.decompress()?
        } else {
            e.data
        };
        res.data_ptr = data.as_mut_ptr();
        res.data_len = data.len();
        res.data_cap = data.capacity();

        // Prevent the data field from being freed while Go handles it.
        // It will be dropped once CGOBitmap is dropped (see below).
        mem::forget(data);

        Ok(res)
    }
}

impl Drop for CGOBitmap {
    fn drop(&mut self) {
        // Reconstruct into Vec to drop the allocated buffer.
        unsafe {
            Vec::from_raw_parts(self.data_ptr, self.data_len, self.data_cap);
        }
    }
}

#[cfg(unix)]
fn wait_for_fd(fd: usize) -> bool {
    unsafe {
        let mut raw_fds: fd_set = mem::zeroed();

        FD_SET(fd as i32, &mut raw_fds);

        let result = select(
            fd as i32 + 1,
            &mut raw_fds,
            ptr::null_mut(),
            ptr::null_mut(),
            ptr::null_mut(),
        );
        result == 1
    }
}

/// `update_clipboard` is called from Go, and caches data that was copied
/// client-side while notifying the RDP server that new clipboard data is available.
///
/// # Safety
///
/// `client_ptr` must be a valid pointer to a Client.
#[no_mangle]
pub unsafe extern "C" fn update_clipboard(
    client_ptr: *mut Client,
    data: *mut u8,
    len: u32,
) -> CGOErrCode {
    let client = match Client::from_ptr(client_ptr) {
        Ok(client) => client,
        Err(cgo_error) => {
            return cgo_error;
        }
    };
    let data = from_go_array(len, data);
    let mut lock = client.rdp_client.lock().unwrap();

    match lock.cliprdr {
        Some(ref mut clip) => match clip
            .update_clipboard(String::from_utf8_lossy(&data).into_owned())
        {
            Ok(messages) => {
                for message in messages {
                    if let Err(e) = lock.mcs.write(&cliprdr::CHANNEL_NAME.to_string(), message) {
                        error!("failed writing cliprdr format list: {:?}", e);
                        return CGOErrCode::ErrCodeFailure;
                    }
                }
                CGOErrCode::ErrCodeSuccess
            }
            Err(e) => {
                error!("failed updating clipboard: {:?}", e);
                CGOErrCode::ErrCodeFailure
            }
        },
        None => CGOErrCode::ErrCodeSuccess,
    }
}

/// handle_tdp_sd_announce announces a new drive that's ready to be
/// redirected over RDP.
///
/// # Safety
///
/// The caller must ensure that sd_announce.name points to a valid buffer.
#[no_mangle]
pub unsafe extern "C" fn handle_tdp_sd_announce(
    client_ptr: *mut Client,
    sd_announce: CGOSharedDirectoryAnnounce,
) -> CGOErrCode {
    let client = match Client::from_ptr(client_ptr) {
        Ok(client) => client,
        Err(cgo_error) => {
            return cgo_error;
        }
    };

    let drive_name = from_go_string(sd_announce.name);
    let new_drive =
        rdpdr::ClientDeviceListAnnounce::new_drive(sd_announce.directory_id, drive_name);

    let mut rdp_client = client.rdp_client.lock().unwrap();
    match rdp_client.write_client_device_list_announce(new_drive) {
        Ok(()) => CGOErrCode::ErrCodeSuccess,
        Err(e) => {
            error!("failed to announce new drive: {:?}", e);
            CGOErrCode::ErrCodeFailure
        }
    }
}

/// handle_tdp_sd_info_response handles a TDP Shared Directory Info Response
/// message
///
/// # Safety
///
/// The caller must ensure that res.fso.path points to a valid buffer.
#[no_mangle]
pub unsafe extern "C" fn handle_tdp_sd_info_response(
    client_ptr: *mut Client,
    res: CGOSharedDirectoryInfoResponse,
) -> CGOErrCode {
    let client = match Client::from_ptr(client_ptr) {
        Ok(client) => client,
        Err(cgo_error) => {
            return cgo_error;
        }
    };

    let mut rdp_client = client.rdp_client.lock().unwrap();
    match rdp_client.handle_tdp_sd_info_response(SharedDirectoryInfoResponse::from(res)) {
        Ok(()) => CGOErrCode::ErrCodeSuccess,
        Err(e) => {
            error!("failed to handle Shared Directory Info Response: {:?}", e);
            CGOErrCode::ErrCodeFailure
        }
    }
}

/// `read_rdp_output` reads incoming RDP bitmap frames from client at client_ref and forwards them to
/// handle_bitmap.
///
/// # Safety
///
/// `client_ptr` must be a valid pointer to a Client.
/// `handle_bitmap` *must not* free the memory of CGOBitmap.
#[no_mangle]
pub unsafe extern "C" fn read_rdp_output(client_ptr: *mut Client) -> CGOErrCode {
    let client = match Client::from_ptr(client_ptr) {
        Ok(client) => client,
        Err(cgo_error) => {
            return cgo_error;
        }
    };
    if let Some(err) = read_rdp_output_inner(client) {
        error!("{}", err);
        CGOErrCode::ErrCodeFailure
    } else {
        CGOErrCode::ErrCodeSuccess
    }
}

fn read_rdp_output_inner(client: &Client) -> Option<String> {
    let tcp_fd = client.tcp_fd;
    let client_ref = client.go_ref;

    // Read incoming events.
    //
    // Wait for some data to be available on the TCP socket FD before consuming it. This prevents
    // us from locking the mutex in Client permanently while no data is available.
    while wait_for_fd(tcp_fd as usize) {
        let mut err = CGOErrCode::ErrCodeSuccess;
        let res = client.rdp_client.lock().unwrap().read(|rdp_event| {
            // This callback can be called multiple times per rdp_client.read()
            // (if multiple messages were received since the last call). Therefore,
            // we check that the previous call to handle_bitmap succeeded, so we don't
            // have a situation where handle_bitmap fails repeatedly and creates a
            // bunch of repetitive error messages in the logs. If it fails once,
            // we assume the connection is broken and stop trying to send bitmaps.
            if err == CGOErrCode::ErrCodeSuccess {
                match rdp_event {
                    RdpEvent::Bitmap(bitmap) => {
                        let mut cbitmap = match CGOBitmap::try_from(bitmap) {
                            Ok(cb) => cb,
                            Err(e) => {
                                error!(
                                    "failed to convert RDP bitmap to CGO representation: {:?}",
                                    e
                                );
                                return;
                            }
                        };
                        unsafe {
                            err = handle_bitmap(client_ref, &mut cbitmap) as CGOErrCode;
                        };
                    }
                    // No other events should be sent by the server to us.
                    _ => {
                        debug!("got unexpected pointer event from RDP server, ignoring");
                    }
                }
            }
        });
        match res {
            Err(RdpError::Io(io_err)) if io_err.kind() == ErrorKind::UnexpectedEof => return None,
            Err(e) => {
                return Some(format!("RDP read failed: {:?}", e));
            }
            _ => {}
        }
        if err != CGOErrCode::ErrCodeSuccess {
            return Some("failed forwarding RDP bitmap frame".to_string());
        }
    }
    None
}

/// CGOMousePointerEvent is a CGO-compatible version of PointerEvent that we pass back to Go.
/// PointerEvent is a mouse move or click update from the user.
#[repr(C)]
#[derive(Copy, Clone)]
pub struct CGOMousePointerEvent {
    pub x: u16,
    pub y: u16,
    pub button: CGOPointerButton,
    pub down: bool,
    pub wheel: CGOPointerWheel,
    pub wheel_delta: i16,
}

#[repr(C)]
#[derive(Copy, Clone)]
pub enum CGOPointerButton {
    PointerButtonNone,
    PointerButtonLeft,
    PointerButtonRight,
    PointerButtonMiddle,
}

#[repr(C)]
#[derive(Copy, Clone, Debug)]
pub enum CGOPointerWheel {
    PointerWheelNone,
    PointerWheelVertical,
    PointerWheelHorizontal,
}

impl From<CGOMousePointerEvent> for PointerEvent {
    fn from(p: CGOMousePointerEvent) -> PointerEvent {
        PointerEvent {
            x: p.x,
            y: p.y,
            button: match p.button {
                CGOPointerButton::PointerButtonNone => PointerButton::None,
                CGOPointerButton::PointerButtonLeft => PointerButton::Left,
                CGOPointerButton::PointerButtonRight => PointerButton::Right,
                CGOPointerButton::PointerButtonMiddle => PointerButton::Middle,
            },
            down: p.down,
            wheel: match p.wheel {
                CGOPointerWheel::PointerWheelNone => PointerWheel::None,
                CGOPointerWheel::PointerWheelVertical => PointerWheel::Vertical,
                CGOPointerWheel::PointerWheelHorizontal => PointerWheel::Horizontal,
            },
            wheel_delta: p.wheel_delta,
        }
    }
}

/// # Safety
///
/// client_ptr must be a valid pointer to a Client.
#[no_mangle]
pub unsafe extern "C" fn write_rdp_pointer(
    client_ptr: *mut Client,
    pointer: CGOMousePointerEvent,
) -> CGOErrCode {
    let client = match Client::from_ptr(client_ptr) {
        Ok(client) => client,
        Err(cgo_error) => {
            return cgo_error;
        }
    };
    let res = client
        .rdp_client
        .lock()
        .unwrap()
        .write(RdpEvent::Pointer(pointer.into()));

    if let Err(e) = res {
        error!("failed writing RDP pointer event: {:?}", e);
        CGOErrCode::ErrCodeFailure
    } else {
        CGOErrCode::ErrCodeSuccess
    }
}

/// CGOKeyboardEvent is a CGO-compatible version of KeyboardEvent that we pass back to Go.
/// KeyboardEvent is a keyboard update from the user.
#[repr(C)]
#[derive(Copy, Clone)]
pub struct CGOKeyboardEvent {
    // Note: there's only one key code sent at a time. A key combo is sent as a sequence of
    // KeyboardEvent messages, one key at a time in the "down" state. The RDP server takes care of
    // interpreting those.
    pub code: u16,
    pub down: bool,
}

impl From<CGOKeyboardEvent> for KeyboardEvent {
    fn from(k: CGOKeyboardEvent) -> KeyboardEvent {
        KeyboardEvent {
            code: k.code,
            down: k.down,
        }
    }
}

/// # Safety
///
/// client_ptr must be a valid pointer to a Client.
#[no_mangle]
pub unsafe extern "C" fn write_rdp_keyboard(
    client_ptr: *mut Client,
    key: CGOKeyboardEvent,
) -> CGOErrCode {
    let client = match Client::from_ptr(client_ptr) {
        Ok(client) => client,
        Err(cgo_error) => {
            return cgo_error;
        }
    };
    let res = client
        .rdp_client
        .lock()
        .unwrap()
        .write(RdpEvent::Key(key.into()));
    if let Err(e) = res {
        error!("failed writing RDP keyboard event: {:?}", e);
        CGOErrCode::ErrCodeFailure
    } else {
        CGOErrCode::ErrCodeSuccess
    }
}

/// # Safety
///
/// client_ptr must be a valid pointer to a Client.
#[no_mangle]
pub unsafe extern "C" fn close_rdp(client_ptr: *mut Client) -> CGOErrCode {
    let client = match Client::from_ptr(client_ptr) {
        Ok(client) => client,
        Err(cgo_error) => {
            return cgo_error;
        }
    };
    match client.rdp_client.lock().unwrap().shutdown() {
        Err(_) => CGOErrCode::ErrCodeFailure,
        Ok(_) => CGOErrCode::ErrCodeSuccess,
    }
}

/// free_rdp lets the Go side inform us when it's done with Client and it can be dropped.
///
/// # Safety
///
/// client_ptr must be a valid pointer to a Client.
#[no_mangle]
pub unsafe extern "C" fn free_rdp(client_ptr: *mut Client) {
    drop(Client::from_raw(client_ptr))
}

/// # Safety
///
/// s must be a C-style null terminated string.
/// s is cloned here, and the caller is responsible for
/// ensuring its memory is freed.
unsafe fn from_go_string(s: *const c_char) -> String {
    CStr::from_ptr(s).to_string_lossy().into_owned()
}

/// # Safety
///
/// ptr must be a valid buffer of len bytes.
unsafe fn from_go_array(len: u32, ptr: *mut u8) -> Vec<u8> {
    slice::from_raw_parts(ptr, len as usize).to_vec()
}

#[repr(C)]
#[derive(Copy, Clone, PartialEq, Debug)]
pub enum CGOErrCode {
    ErrCodeSuccess = 0,
    ErrCodeFailure = 1,
}

#[repr(C)]
pub struct CGOSharedDirectoryAnnounce {
    pub directory_id: u32,
    pub name: *const c_char,
}

/// SharedDirectoryAcknowledge is a CGO-compatible version of
/// the TDP Shared Directory Knowledge message that we pass back to Go.
#[derive(Debug)]
pub struct SharedDirectoryAcknowledge {
    pub err_code: u32,
    pub directory_id: u32,
}

#[repr(C)]
pub struct CGOSharedDirectoryAcknowledge {
    pub err_code: u32,
    pub directory_id: u32,
}

impl From<SharedDirectoryAcknowledge> for CGOSharedDirectoryAcknowledge {
    fn from(ack: SharedDirectoryAcknowledge) -> CGOSharedDirectoryAcknowledge {
        CGOSharedDirectoryAcknowledge {
            err_code: ack.err_code,
            directory_id: ack.directory_id,
        }
    }
}

#[derive(Debug)]
pub struct SharedDirectoryInfoRequest {
    completion_id: u32,
    directory_id: u32,
    path: String,
}

#[repr(C)]
pub struct CGOSharedDirectoryInfoRequest {
    pub completion_id: u32,
    pub directory_id: u32,
    pub path: *const c_char,
}

impl From<ServerCreateDriveRequest> for SharedDirectoryInfoRequest {
    fn from(req: ServerCreateDriveRequest) -> SharedDirectoryInfoRequest {
        SharedDirectoryInfoRequest {
            completion_id: req.device_io_request.completion_id,
            directory_id: req.device_io_request.device_id,
            path: req.path,
        }
    }
}

#[derive(Debug)]
#[allow(dead_code)]
pub struct SharedDirectoryInfoResponse {
    completion_id: u32,
    err_code: u32,
    fso: FileSystemObject,
}

#[repr(C)]
pub struct CGOSharedDirectoryInfoResponse {
    pub completion_id: u32,
    pub err_code: u32,
    pub fso: CGOFileSystemObject,
}

impl From<CGOSharedDirectoryInfoResponse> for SharedDirectoryInfoResponse {
    fn from(cgo_res: CGOSharedDirectoryInfoResponse) -> SharedDirectoryInfoResponse {
        SharedDirectoryInfoResponse {
            completion_id: cgo_res.completion_id,
            err_code: cgo_res.err_code,
            fso: FileSystemObject::from(cgo_res.fso),
        }
    }
}

#[derive(Debug)]
#[allow(dead_code)]
pub struct FileSystemObject {
    last_modified: u64,
    size: u64,
    file_type: u32, // TODO(isaiah): make an enum
    path: String,
}

#[repr(C)]
pub struct CGOFileSystemObject {
    pub last_modified: u64,
    pub size: u64,
    pub file_type: u32, // TODO(isaiah): make an enum
    pub path: *const c_char,
}

impl From<CGOFileSystemObject> for FileSystemObject {
    fn from(cgo_fso: CGOFileSystemObject) -> FileSystemObject {
        unsafe {
            FileSystemObject {
                last_modified: cgo_fso.last_modified,
                size: cgo_fso.size,
                file_type: cgo_fso.file_type,
                path: from_go_string(cgo_fso.path),
            }
        }
    }
}

// These functions are defined on the Go side. Look for functions with '//export funcname'
// comments.
extern "C" {
    fn handle_bitmap(client_ref: usize, b: *mut CGOBitmap) -> CGOErrCode;
    fn handle_remote_copy(client_ref: usize, data: *mut u8, len: u32) -> CGOErrCode;

    fn tdp_sd_acknowledge(client_ref: usize, ack: *mut CGOSharedDirectoryAcknowledge)
        -> CGOErrCode;
    fn tdp_sd_info_request(
        client_ref: usize,
        req: *mut CGOSharedDirectoryInfoRequest,
    ) -> CGOErrCode;
}

/// Payload is a generic type used to represent raw incoming RDP messages for parsing.
pub(crate) type Payload = Cursor<Vec<u8>>;
