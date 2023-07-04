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

//! This crate contains an RDP Client with the minimum functionality required
//! for Teleport's Desktop Access feature.
//!
//! Along with core RDP functionality, it contains code for:
//! - Calling functions defined in Go (these are declared in an `extern "C"` block)
//! - Functions to be called from Go (any function prefixed with the `#[no_mangle]`
//!   macro and a `pub unsafe extern "C"`).
//! - Structs for passing between the two (those prefixed with the `#[repr(C)]` macro
//!   and whose name begins with `CGO`)
//!
//! Memory management at this interface can be tricky, given the long list of rules
//! required by CGO (https://pkg.go.dev/cmd/cgo). We can simplify our job in this
//! regard by sticking to the following design principles:
//!
//! 1) Whichever side of the Rust-Go interface allocates some memory on the heap is
//!    responsible for freeing it.
//! 2) And therefore whenever one side of the Rust-Go interface is passed some memory
//!    it didn't allocate but needs to hold on to, is responsible for copying it to its
//!    own respective heap.
//!
//! In practice, this means that all the functions called from Go (those
//! prefixed with `pub unsafe extern "C"`) MUST NOT hang on to any of the
//! pointers passed in to them after they return. All pointer data that needs to
//! persist MUST be copied into Rust-owned memory.

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

use bytes::BytesMut;
use errors::try_error;
use ironrdp_pdu::input::fast_path::{FastPathInput, FastPathInputEvent};
use ironrdp_pdu::input::mouse::PointerFlags;
use ironrdp_pdu::input::MousePdu;
use ironrdp_pdu::PduParsing;
use ironrdp_session::image::DecodedImage;
use ironrdp_session::utils::swap_hashmap_kv;
use ironrdp_session::{x224, ActiveStageOutput, SessionError, SessionErrorKind, SessionResult};
use rdp::core::event::*;
use rdp::model::error::{Error as RdpError, RdpResult};
use rdpdr::path::UnixPath;
use rdpdr::ServerCreateDriveRequest;
use sspi::network_client::reqwest_network_client::RequestClientFactory;
use std::convert::TryFrom;
use std::ffi::{CStr, CString, NulError};
use std::fmt::Debug;
use std::io::Cursor;
use std::io::{self, Error as IoError};
use std::net::ToSocketAddrs;
use std::os::raw::c_char;
use std::{mem, ptr, slice, time};
use tokio::net::TcpStream as TokioTcpStream;

#[no_mangle]
pub extern "C" fn init() {
    env_logger::try_init().unwrap_or_else(|e| println!("failed to initialize Rust logger: {e}"));
}

pub struct IronRDPClient {
    framed: UpgradedFramed,
    x224_processor: x224::Processor,
}

impl IronRDPClient {
    fn new(upgraded_framed: UpgradedFramed, x224_processor: x224::Processor) -> Self {
        Self {
            framed: upgraded_framed,
            x224_processor,
        }
    }
}

/// Client has an unusual lifecycle:
/// - The function connect_rdp calls Client::new(), which creates it on the heap (Box::new), grabs a raw pointer(Box::into_raw),
///   and returns in to Go.
/// - Most other exported rdp functions (pub unsafe extern "C") take the raw pointer and convert it (Box::leak(Box::from_raw(ptr)))
///   to a reference (&'static mut Client), which can then be used without dropping the client.
/// - The function free_rdp takes the raw pointer and drops it.
///
/// The Client makes use of asynchronous rust via the tokio runtime. A single runtime is created in connect_rdp and held on to by the
/// Client. The exported rdp functions which need to call async rust functions start with `client.tokio_rt.handle().clone().block_on( ... )`,
/// which creates a new task on the tokio runtime and blocks the current thread until it completes. Since these functions are called from
/// Go, the "current thread" can be thought of as whichever goroutine the exported rdp function is called from. Because the client might
/// be being used by multiple goroutines concurrently, it is up to the programmer to consider any synchronization mechanisms that might
/// need to be implemented as features are added to the Client going forward.
pub struct Client {
    iron_rdp_client: IronRDPClient,
    tokio_rt: tokio::runtime::Runtime,
    go_ref: usize,
}

impl Client {
    fn new(
        iron_rdp_client: IronRDPClient,
        go_ref: usize,
        tokio_rt: tokio::runtime::Runtime,
    ) -> *mut Self {
        Box::into_raw(Box::new(Self {
            iron_rdp_client,
            tokio_rt,
            go_ref,
        }))
    }

    unsafe fn from_raw(ptr: *mut Self) -> Result<&'static mut Client, CGOErrCode> {
        match ptr.as_ref() {
            None => {
                error!("invalid Rust client pointer");
                Err(CGOErrCode::ErrCodeClientPtr)
            }
            Some(_) => Ok(Box::leak(Box::from_raw(ptr))),
        }
    }

    unsafe fn drop(ptr: *mut Self) {
        drop(Box::from_raw(ptr))
    }

    async fn read_pdu(&mut self) -> io::Result<(ironrdp_pdu::Action, BytesMut)> {
        self.iron_rdp_client.framed.read_pdu().await
    }

    async fn write_all(&mut self, buf: &[u8]) -> io::Result<()> {
        self.iron_rdp_client.framed.write_all(buf).await
    }

    fn process_x224_frame(&mut self, frame: &[u8]) -> SessionResult<Vec<ActiveStageOutput>> {
        let output = self.iron_rdp_client.x224_processor.process(frame)?;
        let mut stage_outputs = Vec::new();
        if !output.is_empty() {
            stage_outputs.push(ActiveStageOutput::ResponseFrame(output));
        }
        Ok(stage_outputs)
    }

    /// Iterates through any response frames in result, sending them to the RDP server.
    /// Typically returns None if everything goes as expected and the session should continue.
    // TODO(isaiah): this api is weird, should probably return a Result instead of an Option.
    async fn process_active_stage_result(
        &mut self,
        result: SessionResult<Vec<ActiveStageOutput>>,
    ) -> Option<ReadRdpOutputReturns> {
        match result {
            Ok(outputs) => {
                for output in outputs {
                    match output {
                        ActiveStageOutput::ResponseFrame(response) => {
                            match self.write_all(&response).await {
                                Ok(_) => {
                                    trace!("write_all succeeded, continuing");
                                    continue;
                                }
                                Err(e) => {
                                    return Some(ReadRdpOutputReturns {
                                        user_message: format!("Failed to write frame: {}", e),
                                        disconnect_code: CGODisconnectCode::DisconnectCodeUnknown,
                                        err_code: CGOErrCode::ErrCodeFailure,
                                    });
                                }
                            }
                        }
                        ActiveStageOutput::Terminate => {
                            return Some(ReadRdpOutputReturns {
                                user_message: "RDP session terminated".to_string(),
                                disconnect_code: CGODisconnectCode::DisconnectCodeUnknown,
                                err_code: CGOErrCode::ErrCodeSuccess,
                            });
                        }
                        ActiveStageOutput::GraphicsUpdate(_) => {
                            error!("unexpected GraphicsUpdate, this should be handled on the client side");
                            return Some(ReadRdpOutputReturns {
                                user_message: "Server error".to_string(),
                                disconnect_code: CGODisconnectCode::DisconnectCodeUnknown,
                                err_code: CGOErrCode::ErrCodeFailure,
                            });
                        }
                    }
                }
            }
            Err(err) => {
                error!("failed to process frame: {}", err);
                return Some(ReadRdpOutputReturns {
                    user_message: "Failed to process frame".to_string(),
                    disconnect_code: CGODisconnectCode::DisconnectCodeUnknown,
                    err_code: CGOErrCode::ErrCodeFailure,
                });
            }
        }

        // All outputs were response frames, return None to indicate that the client should continue
        trace!("process_active_stage_result succeeded, returning None");
        None
    }
}

#[repr(C)]
pub struct ClientOrError {
    client: *mut Client,
    err: CGOErrCode,
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
pub unsafe extern "C" fn connect_rdp(go_ref: usize, params: CGOConnectParams) -> ClientOrError {
    // Convert from C to Rust types.
    let addr = from_c_string(params.go_addr);
    let username = from_c_string(params.go_username);
    let cert_der = from_go_array(params.cert_der, params.cert_der_len);
    let key_der = from_go_array(params.key_der, params.key_der_len);

    let tokio_rt = tokio::runtime::Runtime::new().unwrap();

    match connect_rdp_inner(
        go_ref,
        tokio_rt,
        ConnectParams {
            addr,
            username,
            cert_der,
            key_der,
            screen_width: params.screen_width,
            screen_height: params.screen_height,
            allow_clipboard: params.allow_clipboard,
            allow_directory_sharing: params.allow_directory_sharing,
            show_desktop_wallpaper: params.show_desktop_wallpaper,
        },
    ) {
        Ok(client) => ClientOrError {
            client,
            err: CGOErrCode::ErrCodeSuccess,
        },
        Err(err) => {
            error!("{:?}", err);
            ClientOrError {
                client: ptr::null_mut(),
                err: CGOErrCode::ErrCodeFailure,
            }
        }
    }
}

#[derive(Debug)]
enum ConnectError {
    Tcp(IoError),
    Rdp(RdpError),
    InvalidAddr(),
    IronRdpError(SessionError), //todo(isaiah): reconsider error typing
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
const RDP_HANDSHAKE_TIMEOUT: time::Duration = time::Duration::from_secs(10);
const RDPSND_CHANNEL_NAME: &str = "rdpsnd";

#[repr(C)]
pub struct CGOConnectParams {
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
    show_desktop_wallpaper: bool,
}

#[derive(Debug)]
struct ConnectParams {
    addr: String,
    username: String,
    cert_der: Vec<u8>,
    key_der: Vec<u8>,
    screen_width: u16,
    screen_height: u16,
    allow_clipboard: bool,
    allow_directory_sharing: bool,
    show_desktop_wallpaper: bool,
}

type UpgradedFramed = ironrdp_tokio::TokioFramed<ironrdp_tls::TlsStream<TokioTcpStream>>;

fn connect_rdp_inner(
    go_ref: usize,
    tokio_rt: tokio::runtime::Runtime,
    params: ConnectParams,
) -> Result<*mut Client, ConnectError> {
    match tokio_rt.block_on(async {
        let server_addr = params.addr;
        let server_socket_addr = server_addr.to_socket_addrs()?.next().unwrap();

        let stream = match TokioTcpStream::connect(&server_socket_addr).await {
            Ok(it) => it,
            Err(err) => {
                error!("tcp connect error: {:?}", err);
                return Err(ConnectError::IronRdpError(SessionError::new(
                    "tcp connect error",
                    SessionErrorKind::General,
                )));
            }
        };

        let mut framed = ironrdp_tokio::TokioFramed::new(stream);

        let connector_config = ironrdp_connector::Config {
            desktop_size: ironrdp_connector::DesktopSize {
                width: params.screen_width,
                height: params.screen_height,
            },
            security_protocol: ironrdp_pdu::nego::SecurityProtocol::HYBRID_EX,
            username: params.username,
            password: std::env::var("RDP_PASSWORD").unwrap(), //todo(isaiah)
            domain: None,
            client_build: 0,
            client_name: "Teleport".to_string(),
            keyboard_type: ironrdp_pdu::gcc::KeyboardType::IbmEnhanced,
            keyboard_subtype: 0,
            keyboard_functional_keys_count: 12,
            ime_file_name: "".to_string(),
            graphics: None,
            bitmap: Some(ironrdp_connector::BitmapConfig {
                lossy_compression: true,
                color_depth: 32, // Changing this to 16 gets us uncompressed bitmaps on machines configured like https://github.com/Devolutions/IronRDP/blob/55d11a5000ebd474c2ddc294b8b3935554443112/README.md?plain=1#L17-L36
            }),
            dig_product_id: "".to_string(),
            client_dir: "C:\\Windows\\System32\\mstscax.dll".to_string(),
            platform: ironrdp_pdu::rdp::capability_sets::MajorPlatformType::Unspecified,
        };

        let mut connector = ironrdp_connector::ClientConnector::new(connector_config)
            .with_server_addr(server_socket_addr)
            .with_server_name(server_addr)
            .with_credssp_client_factory(Box::new(RequestClientFactory));

        let should_upgrade = match ironrdp_tokio::connect_begin(&mut framed, &mut connector).await {
            Ok(it) => it,
            Err(e) => {
                error!("connect_begin error: {:?}", e);
                return Err(ConnectError::IronRdpError(SessionError::new(
                    "connect_begin error",
                    SessionErrorKind::General,
                )));
            }
        };

        debug!("TLS upgrade");

        // Ensure there is no leftover
        let initial_stream = framed.into_inner_no_leftover();
        let (upgraded_stream, server_public_key) =
            ironrdp_tls::upgrade(initial_stream, &server_socket_addr.ip().to_string()).await?;

        let upgraded =
            ironrdp_tokio::mark_as_upgraded(should_upgrade, &mut connector, server_public_key);

        let mut upgraded_framed = ironrdp_tokio::TokioFramed::new(upgraded_stream);

        let connection_result = match ironrdp_tokio::connect_finalize(
            upgraded,
            &mut upgraded_framed,
            connector,
        )
        .await
        {
            Ok(it) => it,
            Err(e) => {
                error!("connect_finalize error: {:?}", e);
                return Err(ConnectError::IronRdpError(SessionError::new(
                    "connect_finalize error",
                    SessionErrorKind::General,
                )));
            }
        };

        debug!("connection_result: {:?}", connection_result);

        let x224_processor = x224::Processor::new(
            swap_hashmap_kv(connection_result.static_channels),
            connection_result.user_channel_id,
            connection_result.io_channel_id,
            None,
            None,
        );

        Ok((upgraded_framed, x224_processor))
    }) {
        Ok((upgraded_framed, x224_processor)) => Ok(Client::new(
            IronRDPClient::new(upgraded_framed, x224_processor),
            go_ref,
            tokio_rt,
        )),
        Err(err) => Err(err),
    }
}

/// CGOPNG is a CGO-compatible version of PNG that we pass back to Go.
#[repr(C)]
pub struct CGOPNG {
    pub dest_left: u16,
    pub dest_top: u16,
    pub dest_right: u16,
    pub dest_bottom: u16,
    /// The memory of this field is managed by the Rust side.
    pub data_ptr: *mut u8,
    pub data_len: usize,
    pub data_cap: usize,
}

impl TryFrom<BitmapEvent> for CGOPNG {
    type Error = RdpError;

    fn try_from(e: BitmapEvent) -> Result<Self, Self::Error> {
        let mut res = CGOPNG {
            dest_left: e.dest_left,
            dest_top: e.dest_top,
            dest_right: e.dest_right,
            dest_bottom: e.dest_bottom,
            data_ptr: ptr::null_mut(),
            data_len: 0,
            data_cap: 0,
        };

        let w: u16 = e.width;
        let h: u16 = e.height;

        let mut encoded = Vec::with_capacity(8192);
        encode_png(&mut encoded, w, h, e.decompress()?).map_err(|err| {
            Self::Error::TryError(format!("failed to encode bitmap to png: {err:?}"))
        })?;

        res.data_ptr = encoded.as_mut_ptr();
        res.data_len = encoded.len();
        res.data_cap = encoded.capacity();

        // Prevent the data field from being freed while Go handles it.
        // It will be dropped once CGOPNG is dropped (see below).
        mem::forget(encoded);

        Ok(res)
    }
}

impl TryFrom<&DecodedImage> for CGOPNG {
    type Error = RdpError;

    fn try_from(image: &DecodedImage) -> Result<Self, Self::Error> {
        let w: u16 = image.width();
        let h: u16 = image.height();
        let mut res = CGOPNG {
            dest_left: 0,
            dest_top: 0,
            dest_right: w,
            dest_bottom: h,
            data_ptr: ptr::null_mut(),
            data_len: 0,
            data_cap: 0,
        };

        let mut encoded = Vec::with_capacity(8192);
        encode_png(&mut encoded, w, h, image.data().to_vec()).map_err(|err| {
            Self::Error::TryError(format!("failed to encode bitmap to png: {err:?}"))
        })?;

        res.data_ptr = encoded.as_mut_ptr();
        res.data_len = encoded.len();
        res.data_cap = encoded.capacity();

        // Prevent the data field from being freed while Go handles it.
        // It will be dropped once CGOPNG is dropped (see below).
        mem::forget(encoded);

        Ok(res)
    }
}

/// encodes png from the uncompressed bitmap data
///
/// # Arguments
///
/// * `dest` - buffer that will contain the png data
/// * `width` - width of the png
/// * `height` - height of the png
/// * `data` - buffer that contains uncompressed bitmap data
pub fn encode_png(
    dest: &mut Vec<u8>,
    width: u16,
    height: u16,
    mut data: Vec<u8>,
) -> Result<(), png::EncodingError> {
    let mut encoder = png::Encoder::new(dest, width as u32, height as u32);
    encoder.set_compression(png::Compression::Fast);
    encoder.set_color(png::ColorType::Rgba);

    let mut writer = encoder.write_header()?;
    writer.write_image_data(&data)?;
    writer.finish()?;
    Ok(())
}

impl Drop for CGOPNG {
    fn drop(&mut self) {
        // Reconstruct into Vec to drop the allocated buffer.
        unsafe {
            Vec::from_raw_parts(self.data_ptr, self.data_len, self.data_cap);
        }
    }
}

/// `update_clipboard` is called from Go, and caches data that was copied
/// client-side while notifying the RDP server that new clipboard data is available.
///
/// # Safety
///
/// client_ptr MUST be a valid pointer.
/// (validity defined by https://doc.rust-lang.org/nightly/core/primitive.pointer.html#method.as_ref-1)
///
/// data MUST be a valid pointer.
/// (validity defined by the validity of data in https://doc.rust-lang.org/std/slice/fn.from_raw_parts_mut.html)
#[no_mangle]
pub unsafe extern "C" fn update_clipboard(
    client_ptr: *mut Client,
    data: *mut u8,
    len: u32,
) -> CGOErrCode {
    warn!("unimplemented: update_clipboard");
    CGOErrCode::ErrCodeSuccess
}

/// handle_tdp_sd_announce announces a new drive that's ready to be
/// redirected over RDP.
///
///
/// # Safety
///
/// client_ptr MUST be a valid pointer.
/// (validity defined by https://doc.rust-lang.org/nightly/core/primitive.pointer.html#method.as_ref-1)
///
/// sd_announce.name MUST be a non-null pointer to a C-style null terminated string.
#[no_mangle]
pub unsafe extern "C" fn handle_tdp_sd_announce(
    client_ptr: *mut Client,
    sd_announce: CGOSharedDirectoryAnnounce,
) -> CGOErrCode {
    warn!("unimplemented: handle_tdp_sd_announce");
    CGOErrCode::ErrCodeSuccess
}

/// handle_tdp_sd_info_response handles a TDP Shared Directory Info Response
/// message
///
/// # Safety
///
/// client_ptr MUST be a valid pointer.
/// (validity defined by https://doc.rust-lang.org/nightly/core/primitive.pointer.html#method.as_ref-1)
///
/// res.fso.path MUST be a non-null pointer to a C-style null terminated string.
#[no_mangle]
pub unsafe extern "C" fn handle_tdp_sd_info_response(
    client_ptr: *mut Client,
    res: CGOSharedDirectoryInfoResponse,
) -> CGOErrCode {
    warn!("unimplemented: handle_tdp_sd_info_response");
    CGOErrCode::ErrCodeSuccess
}

/// handle_tdp_sd_create_response handles a TDP Shared Directory Create Response
/// message
///
/// # Safety
///
/// client_ptr MUST be a valid pointer.
/// (validity defined by https://doc.rust-lang.org/nightly/core/primitive.pointer.html#method.as_ref-1)
#[no_mangle]
pub unsafe extern "C" fn handle_tdp_sd_create_response(
    client_ptr: *mut Client,
    res: CGOSharedDirectoryCreateResponse,
) -> CGOErrCode {
    warn!("unimplemented: handle_tdp_sd_create_response");
    CGOErrCode::ErrCodeSuccess
}

/// handle_tdp_sd_delete_response handles a TDP Shared Directory Delete Response
/// message
///
/// # Safety
///
/// client_ptr MUST be a valid pointer.
/// (validity defined by https://doc.rust-lang.org/nightly/core/primitive.pointer.html#method.as_ref-1)
#[no_mangle]
pub unsafe extern "C" fn handle_tdp_sd_delete_response(
    client_ptr: *mut Client,
    res: CGOSharedDirectoryDeleteResponse,
) -> CGOErrCode {
    warn!("unimplemented: handle_tdp_sd_delete_response");
    CGOErrCode::ErrCodeSuccess
}

/// handle_tdp_sd_list_response handles a TDP Shared Directory List Response message.
///
/// # Safety
///
/// client_ptr MUST be a valid pointer.
/// (validity defined by https://doc.rust-lang.org/nightly/core/primitive.pointer.html#method.as_ref-1)
///
/// res.fso_list MUST be a valid pointer
/// (validity defined by the validity of data in https://doc.rust-lang.org/std/slice/fn.from_raw_parts_mut.html)
///
/// each res.fso_list[i].path MUST be a non-null pointer to a C-style null terminated string.
#[no_mangle]
pub unsafe extern "C" fn handle_tdp_sd_list_response(
    client_ptr: *mut Client,
    res: CGOSharedDirectoryListResponse,
) -> CGOErrCode {
    warn!("unimplemented: handle_tdp_sd_list_response");
    CGOErrCode::ErrCodeSuccess
}

/// handle_tdp_sd_read_response handles a TDP Shared Directory Read Response
/// message
///
/// # Safety
///
/// client_ptr must be a valid pointer
#[no_mangle]
pub unsafe extern "C" fn handle_tdp_sd_read_response(
    client_ptr: *mut Client,
    res: CGOSharedDirectoryReadResponse,
) -> CGOErrCode {
    warn!("unimplemented: handle_tdp_sd_read_response");
    CGOErrCode::ErrCodeSuccess
}

/// handle_tdp_sd_write_response handles a TDP Shared Directory Write Response
/// message
///
/// # Safety
///
/// client_ptr must be a valid pointer
#[no_mangle]
pub unsafe extern "C" fn handle_tdp_sd_write_response(
    client_ptr: *mut Client,
    res: CGOSharedDirectoryWriteResponse,
) -> CGOErrCode {
    warn!("unimplemented: handle_tdp_sd_write_response");
    CGOErrCode::ErrCodeSuccess
}

/// handle_tdp_sd_move_response handles a TDP Shared Directory Move Response
/// message
///
/// # Safety
///
/// client_ptr MUST be a valid pointer.
/// (validity defined by https://doc.rust-lang.org/nightly/core/primitive.pointer.html#method.as_ref-1)
#[no_mangle]
pub unsafe extern "C" fn handle_tdp_sd_move_response(
    client_ptr: *mut Client,
    res: CGOSharedDirectoryMoveResponse,
) -> CGOErrCode {
    warn!("unimplemented: handle_tdp_sd_move_response");
    CGOErrCode::ErrCodeSuccess
}

/// handle_tdp_rdp_response_pdu handles a TDP RDP Response PDU message. It takes a raw encoded RDP PDU
/// created by the ironrdp client on the frontend and sends it directly to the RDP server.
///
/// res is the raw RDP response message to be sent back to the RDP server, without the TDP message type or
/// array length header.
///
/// # Safety
///
/// client_ptr MUST be a valid pointer.
/// (validity defined by https://doc.rust-lang.org/nightly/core/primitive.pointer.html#method.as_ref-1)
#[no_mangle]
pub unsafe extern "C" fn handle_tdp_rdp_response_pdu(
    client_ptr: *mut Client,
    res: *mut u8,
    res_len: u32,
) -> CGOErrCode {
    let client = match Client::from_raw(client_ptr) {
        Ok(client) => client,
        Err(cgo_error) => {
            return cgo_error;
        }
    };

    client.tokio_rt.handle().clone().block_on(async {
        let res = from_go_array(res, res_len);
        match client
            .process_active_stage_result(Ok(vec![ActiveStageOutput::ResponseFrame(res)]))
            .await
        {
            Some(ret) => ret.err_code,
            None => CGOErrCode::ErrCodeSuccess,
        }
    })
}

/// `read_rdp_output` reads incoming RDP bitmap frames from client at client_ref, encodes bitmap
/// as a png and forwards them to handle_png.
///
/// # Safety
///
/// `client_ptr` must be a valid pointer to a Client.
/// `handle_png` *must not* free the memory of CGOPNG.
#[no_mangle]
pub unsafe extern "C" fn read_rdp_output(client_ptr: *mut Client) -> CGOReadRdpOutputReturns {
    let client = match Client::from_raw(client_ptr) {
        Ok(client) => client,
        Err(cgo_error) => {
            return ReadRdpOutputReturns {
                user_message: "invalid Rust client pointer".to_string(),
                disconnect_code: CGODisconnectCode::DisconnectCodeUnknown,
                err_code: cgo_error,
            }
            .into();
        }
    };

    client
        .tokio_rt
        .handle()
        .clone()
        .block_on(async { read_rdp_output_inner(client).await.into() })
}

async fn read_rdp_output_inner(client: &mut Client) -> ReadRdpOutputReturns {
    loop {
        trace!("awaiting frame from rdp server");
        let (action, mut frame) = match client.read_pdu().await {
            Ok(it) => it,
            Err(e) => {
                error!("error reading PDU: {:?}", e);
                return ReadRdpOutputReturns {
                    user_message: "error reading PDU".to_string(),
                    disconnect_code: CGODisconnectCode::DisconnectCodeUnknown,
                    err_code: CGOErrCode::ErrCodeFailure,
                };
            }
        };
        trace!(
            "Frame received, action = {:?}, frame_len = {:?}",
            action,
            frame.len()
        );

        match action {
            ironrdp_pdu::Action::X224 => {
                let result = client.process_x224_frame(&frame);
                if let Some(return_value) = client.process_active_stage_result(result).await {
                    return return_value;
                }
            }
            ironrdp_pdu::Action::FastPath => {
                let go_ref = client.go_ref;
                match unsafe {
                    handle_remote_fx_frame(go_ref, frame.as_mut_ptr(), frame.len() as u32)
                } {
                    CGOErrCode::ErrCodeSuccess => continue,
                    err => {
                        error!("failed to process fastpath frame: {:?}", err);
                        return ReadRdpOutputReturns {
                            user_message: "Failed to process fastpath frame".to_string(),
                            disconnect_code: CGODisconnectCode::DisconnectCodeUnknown,
                            err_code: err,
                        };
                    }
                }
            }
        };
    }
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
#[derive(Copy, Clone, PartialEq)]
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

impl From<CGOMousePointerEvent> for PointerEvent {
    fn from(p: CGOMousePointerEvent) -> PointerEvent {
        // # Safety
        //
        // This function MUST NOT hang on to any of the pointers passed in to it after it returns.
        // In other words, all pointer data that needs to persist after this function returns MUST
        // be copied into Rust-owned memory.
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
/// client_ptr MUST be a valid pointer.
/// (validity defined by https://doc.rust-lang.org/nightly/core/primitive.pointer.html#method.as_ref-1)
#[no_mangle]
pub unsafe extern "C" fn write_rdp_pointer(
    client_ptr: *mut Client,
    pointer: CGOMousePointerEvent,
) -> CGOErrCode {
    let client = match Client::from_raw(client_ptr) {
        Ok(client) => client,
        Err(cgo_error) => {
            return cgo_error;
        }
    };

    let mut fastpath_events = Vec::new();
    // TODO(isaiah): impl From for this
    let mut flags = match pointer.button {
        CGOPointerButton::PointerButtonLeft => PointerFlags::LEFT_BUTTON,
        CGOPointerButton::PointerButtonRight => PointerFlags::RIGHT_BUTTON,
        CGOPointerButton::PointerButtonMiddle => PointerFlags::MIDDLE_BUTTON_OR_WHEEL,
        _ => PointerFlags::empty(),
    };

    flags |= match pointer.wheel {
        CGOPointerWheel::PointerWheelVertical => PointerFlags::VERTICAL_WHEEL,
        CGOPointerWheel::PointerWheelHorizontal => PointerFlags::HORIZONTAL_WHEEL,
        _ => PointerFlags::empty(),
    };

    if pointer.button == CGOPointerButton::PointerButtonNone
        && pointer.wheel == CGOPointerWheel::PointerWheelNone
    {
        flags |= PointerFlags::MOVE;
    }

    if pointer.down {
        flags |= PointerFlags::DOWN;
    }

    // MousePdu.to_buffer takes care of the rest of the flags.
    let event = FastPathInputEvent::MouseEvent(MousePdu {
        flags,
        number_of_wheel_rotation_units: pointer.wheel_delta,
        x_position: pointer.x,
        y_position: pointer.y,
    });
    fastpath_events.push(event);

    let mut data: Vec<u8> = Vec::new();
    let input_pdu = FastPathInput(fastpath_events);
    input_pdu.to_buffer(&mut data).unwrap();

    client.tokio_rt.handle().clone().block_on(async {
        client.write_all(&data).await.unwrap(); // todo(isaiah): handle error
    });

    CGOErrCode::ErrCodeSuccess
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
        // # Safety
        //
        // This function MUST NOT hang on to any of the pointers passed in to it after it returns.
        // In other words, all pointer data that needs to persist after this function returns MUST
        // be copied into Rust-owned memory.
        KeyboardEvent {
            code: k.code,
            down: k.down,
        }
    }
}

/// # Safety
///
/// client_ptr MUST be a valid pointer.
/// (validity defined by https://doc.rust-lang.org/nightly/core/primitive.pointer.html#method.as_ref-1)
#[no_mangle]
pub unsafe extern "C" fn write_rdp_keyboard(
    client_ptr: *mut Client,
    key: CGOKeyboardEvent,
) -> CGOErrCode {
    warn!("unimplemented: write_rdp_keyboard");
    CGOErrCode::ErrCodeSuccess
}

/// # Safety
///
/// client_ptr must be a valid pointer to a Client.
#[no_mangle]
pub unsafe extern "C" fn close_rdp(client_ptr: *mut Client) -> CGOErrCode {
    warn!("unimplemented: close_rdp");
    CGOErrCode::ErrCodeSuccess
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

struct ReadRdpOutputReturns {
    user_message: String,
    disconnect_code: CGODisconnectCode,
    err_code: CGOErrCode,
}

#[repr(C)]
pub struct CGOReadRdpOutputReturns {
    user_message: *const c_char,
    disconnect_code: CGODisconnectCode,
    err_code: CGOErrCode,
}

impl From<ReadRdpOutputReturns> for CGOReadRdpOutputReturns {
    fn from(r: ReadRdpOutputReturns) -> CGOReadRdpOutputReturns {
        CGOReadRdpOutputReturns {
            user_message: to_c_string(&r.user_message).unwrap(),
            disconnect_code: r.disconnect_code,
            err_code: r.err_code,
        }
    }
}

/// free_rdp lets the Go side inform us when it's done with Client and it can be dropped.
///
/// # Safety
///
/// client_ptr MUST be a valid pointer.
/// (validity defined by https://doc.rust-lang.org/nightly/core/primitive.pointer.html#method.as_ref-1)
#[no_mangle]
pub unsafe extern "C" fn free_rdp(client_ptr: *mut Client) {
    Client::drop(client_ptr)
}

/// # Safety
///
/// s must be a C-style null terminated string.
/// s is cloned here, and the caller is responsible for
/// ensuring its memory is freed.
unsafe fn from_c_string(s: *const c_char) -> String {
    // # Safety
    //
    // This function MUST NOT hang on to any of the pointers passed in to it after it returns.
    // In other words, all pointer data that needs to persist after this function returns MUST
    // be copied into Rust-owned memory.
    CStr::from_ptr(s).to_string_lossy().into_owned()
}

/// Creates a Vec from a Go (C) array without a copy.
///
/// # Safety
///
/// See https://doc.rust-lang.org/std/slice/fn.from_raw_parts_mut.html
unsafe fn from_go_array<T: Clone>(data: *mut T, len: u32) -> Vec<T> {
    // # Safety
    //
    // This function MUST NOT hang on to any of the pointers passed in to it after it returns.
    // In other words, all pointer data that needs to persist after this function returns MUST
    // be copied into Rust-owned memory.
    slice::from_raw_parts(data, len as usize).to_vec()
}

/// to_c_string can be used to return string values over the Go boundary.
/// To avoid memory leaks, the Go function must call free_go_string once
/// it's done with the memory.
///
/// See https://doc.rust-lang.org/std/ffi/struct.CString.html#method.into_raw
fn to_c_string(s: &str) -> Result<*const c_char, NulError> {
    let c_string = CString::new(s)?;
    Ok(c_string.into_raw())
}

/// See the docstring for to_c_string.
///
/// # Safety
///
/// s must be a pointer originally created by to_c_string
#[no_mangle]
pub unsafe extern "C" fn free_c_string(s: *mut c_char) {
    // retake pointer to free memory
    let _ = CString::from_raw(s);
}

#[repr(C)]
#[derive(Copy, Clone, PartialEq, Eq, Debug)]
pub enum CGOErrCode {
    ErrCodeSuccess = 0,
    ErrCodeFailure = 1,
    ErrCodeClientPtr = 2,
}

#[repr(C)]
pub struct CGOSharedDirectoryAnnounce {
    pub directory_id: u32,
    pub name: *const c_char,
}

/// SharedDirectoryAnnounce is sent by the TDP client to the server
/// to announce a new directory to be shared over TDP.
pub struct SharedDirectoryAnnounce {
    directory_id: u32,
    name: String,
}

impl From<CGOSharedDirectoryAnnounce> for SharedDirectoryAnnounce {
    fn from(cgo: CGOSharedDirectoryAnnounce) -> SharedDirectoryAnnounce {
        // # Safety
        //
        // This function MUST NOT hang on to any of the pointers passed in to it after it returns.
        // In other words, all pointer data that needs to persist after this function returns MUST
        // be copied into Rust-owned memory.
        unsafe {
            SharedDirectoryAnnounce {
                directory_id: cgo.directory_id,
                name: from_c_string(cgo.name),
            }
        }
    }
}

/// SharedDirectoryAcknowledge is sent by the TDP server to the client
/// to acknowledge that a SharedDirectoryAnnounce was received.
#[derive(Debug)]
#[repr(C)]
pub struct SharedDirectoryAcknowledge {
    pub err_code: TdpErrCode,
    pub directory_id: u32,
}

pub type CGOSharedDirectoryAcknowledge = SharedDirectoryAcknowledge;

/// SharedDirectoryInfoRequest is sent from the TDP server to the client
/// to request information about a file or directory at a given path.
#[derive(Debug)]
pub struct SharedDirectoryInfoRequest {
    completion_id: u32,
    directory_id: u32,
    path: UnixPath,
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
            path: UnixPath::from(&req.path),
        }
    }
}

/// SharedDirectoryInfoResponse is sent by the TDP client to the server
/// in response to a `Shared Directory Info Request`.
#[derive(Debug)]
pub struct SharedDirectoryInfoResponse {
    completion_id: u32,
    err_code: TdpErrCode,

    fso: FileSystemObject,
}

#[repr(C)]
pub struct CGOSharedDirectoryInfoResponse {
    pub completion_id: u32,
    pub err_code: TdpErrCode,
    pub fso: CGOFileSystemObject,
}

impl From<CGOSharedDirectoryInfoResponse> for SharedDirectoryInfoResponse {
    fn from(cgo_res: CGOSharedDirectoryInfoResponse) -> SharedDirectoryInfoResponse {
        // # Safety
        //
        // This function MUST NOT hang on to any of the pointers passed in to it after it returns.
        // In other words, all pointer data that needs to persist after this function returns MUST
        // be copied into Rust-owned memory.
        SharedDirectoryInfoResponse {
            completion_id: cgo_res.completion_id,
            err_code: cgo_res.err_code,
            fso: FileSystemObject::from(cgo_res.fso),
        }
    }
}

#[derive(Debug, Clone)]
/// FileSystemObject is a TDP structure containing the metadata
/// of a file or directory.
pub struct FileSystemObject {
    last_modified: u64,
    size: u64,
    file_type: FileType,
    is_empty: u8,
    path: UnixPath,
}

impl FileSystemObject {
    fn name(&self) -> RdpResult<String> {
        if let Some(name) = self.path.last() {
            Ok(name.to_string())
        } else {
            Err(try_error(&format!(
                "failed to extract name from path: {:?}",
                self.path
            )))
        }
    }
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

#[repr(C)]
#[derive(Copy, Clone, PartialEq, Eq, Debug)]
pub enum FileType {
    File = 0,
    Directory = 1,
}

#[repr(C)]
#[derive(Copy, Clone, PartialEq, Eq, Debug)]
pub enum TdpErrCode {
    /// nil (no error, operation succeeded)
    Nil = 0,
    /// operation failed
    Failed = 1,
    /// resource does not exist
    DoesNotExist = 2,
    /// resource already exists
    AlreadyExists = 3,
}

/// SharedDirectoryWriteRequest is sent by the TDP server to the client
/// to write to a file.
#[derive(Clone)]
pub struct SharedDirectoryWriteRequest {
    completion_id: u32,
    directory_id: u32,
    offset: u64,
    path: UnixPath,
    write_data: Vec<u8>,
}

impl std::fmt::Debug for SharedDirectoryWriteRequest {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("SharedDirectoryWriteRequest")
            .field("completion_id", &self.completion_id)
            .field("directory_id", &self.directory_id)
            .field("offset", &self.offset)
            .field("path", &self.path)
            .field("write_data", &util::vec_u8_debug(&self.write_data))
            .finish()
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

/// SharedDirectoryReadRequest is sent by the TDP server to the client
/// to request the contents of a file.
#[derive(Debug)]
pub struct SharedDirectoryReadRequest {
    completion_id: u32,
    directory_id: u32,
    path: UnixPath,
    offset: u64,
    length: u32,
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

/// SharedDirectoryReadResponse is sent by the TDP client to the server
/// with the data as requested by a SharedDirectoryReadRequest.
#[repr(C)]
pub struct SharedDirectoryReadResponse {
    pub completion_id: u32,
    pub err_code: TdpErrCode,
    pub read_data: Vec<u8>,
}

impl std::fmt::Debug for SharedDirectoryReadResponse {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("SharedDirectoryReadResponse")
            .field("completion_id", &self.completion_id)
            .field("err_code", &self.err_code)
            .field("read_data", &util::vec_u8_debug(&self.read_data))
            .finish()
    }
}

impl From<CGOSharedDirectoryReadResponse> for SharedDirectoryReadResponse {
    fn from(cgo_response: CGOSharedDirectoryReadResponse) -> SharedDirectoryReadResponse {
        unsafe {
            SharedDirectoryReadResponse {
                completion_id: cgo_response.completion_id,
                err_code: cgo_response.err_code,
                read_data: from_go_array(cgo_response.read_data, cgo_response.read_data_length),
            }
        }
    }
}

#[derive(Debug)]
#[repr(C)]
pub struct CGOSharedDirectoryReadResponse {
    pub completion_id: u32,
    pub err_code: TdpErrCode,
    pub read_data_length: u32,
    pub read_data: *mut u8,
}

/// SharedDirectoryWriteResponse is sent by the TDP client to the server
/// to acknowledge the completion of a SharedDirectoryWriteRequest.
#[derive(Debug)]
#[repr(C)]
pub struct SharedDirectoryWriteResponse {
    pub completion_id: u32,
    pub err_code: TdpErrCode,
    pub bytes_written: u32,
}

pub type CGOSharedDirectoryWriteResponse = SharedDirectoryWriteResponse;

/// SharedDirectoryCreateRequest is sent by the TDP server to
/// the client to request the creation of a new file or directory.
#[derive(Debug)]
pub struct SharedDirectoryCreateRequest {
    completion_id: u32,
    directory_id: u32,
    file_type: FileType,
    path: UnixPath,
}

#[repr(C)]
pub struct CGOSharedDirectoryCreateRequest {
    pub completion_id: u32,
    pub directory_id: u32,
    pub file_type: FileType,
    pub path: *const c_char,
}

/// SharedDirectoryListResponse is sent by the TDP client to the server
/// in response to a SharedDirectoryInfoRequest.
#[derive(Debug)]
pub struct SharedDirectoryListResponse {
    completion_id: u32,
    err_code: TdpErrCode,
    fso_list: Vec<FileSystemObject>,
}

impl From<CGOSharedDirectoryListResponse> for SharedDirectoryListResponse {
    fn from(cgo: CGOSharedDirectoryListResponse) -> SharedDirectoryListResponse {
        // # Safety
        //
        // This function MUST NOT hang on to any of the pointers passed in to it after it returns.
        // In other words, all pointer data that needs to persist after this function returns MUST
        // be copied into Rust-owned memory.
        unsafe {
            let cgo_fso_list = from_go_array(cgo.fso_list, cgo.fso_list_length);
            let mut fso_list = vec![];
            for cgo_fso in cgo_fso_list.into_iter() {
                fso_list.push(FileSystemObject::from(cgo_fso));
            }

            SharedDirectoryListResponse {
                completion_id: cgo.completion_id,
                err_code: cgo.err_code,
                fso_list,
            }
        }
    }
}

#[repr(C)]
pub struct CGOSharedDirectoryListResponse {
    completion_id: u32,
    err_code: TdpErrCode,
    fso_list_length: u32,
    fso_list: *mut CGOFileSystemObject,
}

/// SharedDirectoryMoveRequest is sent from the TDP server to the client
/// to request a file at original_path be moved to new_path.
#[derive(Debug)]
pub struct SharedDirectoryMoveRequest {
    completion_id: u32,
    directory_id: u32,
    original_path: UnixPath,
    new_path: UnixPath,
}

#[repr(C)]
pub struct CGOSharedDirectoryMoveRequest {
    pub completion_id: u32,
    pub directory_id: u32,
    pub original_path: *const c_char,
    pub new_path: *const c_char,
}

/// SharedDirectoryCreateResponse is sent by the TDP client to the server
/// to acknowledge a SharedDirectoryCreateRequest was received and executed.
#[derive(Debug)]
pub struct SharedDirectoryCreateResponse {
    completion_id: u32,
    err_code: TdpErrCode,
    fso: FileSystemObject,
}

#[repr(C)]
pub struct CGOSharedDirectoryCreateResponse {
    pub completion_id: u32,
    pub err_code: TdpErrCode,
    pub fso: CGOFileSystemObject,
}

impl From<CGOSharedDirectoryCreateResponse> for SharedDirectoryCreateResponse {
    fn from(cgo_res: CGOSharedDirectoryCreateResponse) -> SharedDirectoryCreateResponse {
        // # Safety
        //
        // This function MUST NOT hang on to any of the pointers passed in to it after it returns.
        // In other words, all pointer data that needs to persist after this function returns MUST
        // be copied into Rust-owned memory.
        SharedDirectoryCreateResponse {
            completion_id: cgo_res.completion_id,
            err_code: cgo_res.err_code,
            fso: FileSystemObject::from(cgo_res.fso),
        }
    }
}

/// SharedDirectoryDeleteRequest is sent by the TDP server to the client
/// to request the deletion of a file or directory at path.
#[derive(Debug)]
pub struct SharedDirectoryDeleteRequest {
    completion_id: u32,
    directory_id: u32,
    path: UnixPath,
}

#[repr(C)]
pub struct CGOSharedDirectoryDeleteRequest {
    pub completion_id: u32,
    pub directory_id: u32,
    pub path: *const c_char,
}

/// SharedDirectoryDeleteResponse is sent by the TDP client to the server
/// to acknowledge a SharedDirectoryDeleteRequest was received and executed.
#[derive(Debug)]
#[repr(C)]
pub struct SharedDirectoryDeleteResponse {
    completion_id: u32,
    err_code: TdpErrCode,
}

pub type CGOSharedDirectoryDeleteResponse = SharedDirectoryDeleteResponse;

/// SharedDirectoryMoveResponse is sent by the TDP client to the server
/// to acknowledge a SharedDirectoryMoveRequest was received and expected.
#[derive(Debug)]
#[repr(C)]
pub struct SharedDirectoryMoveResponse {
    completion_id: u32,
    err_code: TdpErrCode,
}

pub type CGOSharedDirectoryMoveResponse = SharedDirectoryMoveResponse;

/// SharedDirectoryListRequest is sent by the TDP server to the client
/// to request the contents of a directory.
#[derive(Debug)]
pub struct SharedDirectoryListRequest {
    completion_id: u32,
    directory_id: u32,
    path: UnixPath,
}

#[repr(C)]
pub struct CGOSharedDirectoryListRequest {
    pub completion_id: u32,
    pub directory_id: u32,
    pub path: *const c_char,
}

// These functions are defined on the Go side. Look for functions with '//export funcname'
// comments.
extern "C" {
    fn handle_png(client_ref: usize, b: *mut CGOPNG) -> CGOErrCode;
    fn handle_remote_copy(client_ref: usize, data: *mut u8, len: u32) -> CGOErrCode;
    fn handle_remote_fx_frame(client_ref: usize, data: *mut u8, len: u32) -> CGOErrCode;
    fn tdp_sd_acknowledge(client_ref: usize, ack: *mut CGOSharedDirectoryAcknowledge)
        -> CGOErrCode;
    fn tdp_sd_info_request(
        client_ref: usize,
        req: *mut CGOSharedDirectoryInfoRequest,
    ) -> CGOErrCode;
    fn tdp_sd_create_request(
        client_ref: usize,
        req: *mut CGOSharedDirectoryCreateRequest,
    ) -> CGOErrCode;
    fn tdp_sd_delete_request(
        client_ref: usize,
        req: *mut CGOSharedDirectoryDeleteRequest,
    ) -> CGOErrCode;
    fn tdp_sd_list_request(
        client_ref: usize,
        req: *mut CGOSharedDirectoryListRequest,
    ) -> CGOErrCode;
    fn tdp_sd_read_request(
        client_ref: usize,
        req: *mut CGOSharedDirectoryReadRequest,
    ) -> CGOErrCode;
    fn tdp_sd_write_request(
        client_ref: usize,
        req: *mut CGOSharedDirectoryWriteRequest,
    ) -> CGOErrCode;
    fn tdp_sd_move_request(
        client_ref: usize,
        req: *mut CGOSharedDirectoryMoveRequest,
    ) -> CGOErrCode;
}

/// Payload represents raw incoming RDP messages for parsing.
pub(crate) type Payload = Cursor<Vec<u8>>;
/// Message represents a raw outgoing RDP message to send to the RDP server.
pub(crate) type Message = Vec<u8>;
pub(crate) type Messages = Vec<Message>;

/// Encode is an object that can be encoded for sending to the RDP server.
pub(crate) trait Encode: std::fmt::Debug {
    fn encode(&self) -> RdpResult<Message>;
}

/// This is the maximum size of an RDP message which we will accept
/// over a virtual channel.
///
/// Note that this is not an RDP defined value, but rather one we've chosen
/// in order to harden system security.
const MAX_ALLOWED_VCHAN_MSG_SIZE: usize = 2 * 1024 * 1024; // 2MB
