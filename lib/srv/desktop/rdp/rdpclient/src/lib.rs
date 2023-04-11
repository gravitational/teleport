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

use bytes::{Bytes, BytesMut};
use errors::try_error;
use futures_util::io::{AsyncWrite, AsyncWriteExt};
use ironrdp::graphics::image_processing::PixelFormat;
use ironrdp::input::fast_path::{FastPathInput, FastPathInputEvent};
use ironrdp::input::mouse::PointerFlags;
use ironrdp::input::MousePdu;
use ironrdp::pdu::geometry::Rectangle;
use ironrdp::pdu::PduParsing as _;
use ironrdp::session::connection_sequence::{process_connection_sequence, UpgradedStream};
use ironrdp::session::image::DecodedImage;
use ironrdp::session::{
    ActiveStageOutput, ActiveStageProcessor, ErasedWriter, FramedReader, InputConfig,
    RdpError as IronRdpError,
};
use rdp::core::event::*;
use rdp::core::global;
use rdp::core::mcs;
use rdp::model::error::{Error as RdpError, RdpError as RdpProtocolError, RdpErrorKind, RdpResult};
use rdpdr::path::UnixPath;
use rdpdr::ServerCreateDriveRequest;
use sspi::network_client::reqwest_network_client::RequestClientFactory;
use sspi::{AuthIdentity, Secret};
use std::convert::TryFrom;
use std::convert::TryInto;
use std::ffi::{CStr, CString, NulError};
use std::fmt::Debug;
use std::io::Error as IoError;
use std::io::{Cursor, Read, Write};
use std::net::{TcpStream, ToSocketAddrs};
use std::os::raw::c_char;
use std::pin::Pin;
use std::sync::Arc;
use std::{mem, ptr, slice, time};
use tokio::io::AsyncWriteExt as _;
use tokio::net::TcpStream as TokioTcpStream;
use tokio_util::compat::TokioAsyncReadCompatExt as _;
use x509_parser::prelude::{FromDer as _, X509Certificate};

pub fn test() {}

#[no_mangle]
pub extern "C" fn init() {
    env_logger::try_init().unwrap_or_else(|e| println!("failed to initialize Rust logger: {e}"));
}

#[derive(Clone)]
struct SharedStream {
    tcp: Arc<TcpStream>,
}

impl SharedStream {
    fn new(tcp: TcpStream) -> Self {
        Self { tcp: Arc::new(tcp) }
    }
}

impl Read for SharedStream {
    fn read(&mut self, buf: &mut [u8]) -> Result<usize, IoError> {
        self.tcp.as_ref().read(buf)
    }
}

impl Write for SharedStream {
    fn write(&mut self, buf: &[u8]) -> Result<usize, IoError> {
        self.tcp.as_ref().write(buf)
    }

    fn flush(&mut self) -> Result<(), IoError> {
        self.tcp.as_ref().flush()
    }
}

pub struct IronRDPClient {
    reader: FramedReader,
    writer: ErasedWriter,
    processor: ActiveStageProcessor,
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
    iron_rdp_client: IronRDPClient,
    tokio_rt: Option<tokio::runtime::Runtime>,
    go_ref: usize,
}

impl Client {
    fn into_raw(self: Box<Self>) -> *mut Self {
        Box::into_raw(self)
    }

    unsafe fn from_ptr<'a>(ptr: *mut Self) -> Result<&'a mut Client, CGOErrCode> {
        match ptr.as_ref() {
            None => {
                error!("invalid Rust client pointer");
                Err(CGOErrCode::ErrCodeClientPtr)
            }
            Some(_) => Ok(Box::leak(Box::from_raw(ptr))),
        }
    }
    unsafe fn from_raw(ptr: *mut Self) -> Box<Self> {
        Box::from_raw(ptr)
    }

    fn read_frame(
        &mut self,
    ) -> impl std::future::Future<Output = Result<Option<BytesMut>, ironrdp::pdu::RdpError>> + '_
    {
        self.iron_rdp_client.reader.read_frame()
    }

    fn process_frame(
        &mut self,
        image: &mut DecodedImage,
        frame: Bytes,
    ) -> Result<Vec<ActiveStageOutput>, IronRdpError> {
        self.iron_rdp_client.processor.process(image, frame)
    }

    fn write_frame<'a>(
        &'a mut self,
        frame: &'a [u8],
    ) -> futures_util::io::WriteAll<'a, Pin<Box<(dyn AsyncWrite + Send + 'static)>>> {
        self.iron_rdp_client.writer.write_all(frame)
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
pub unsafe extern "C" fn connect_rdp(go_ref: usize, params: CGOConnectParams) -> ClientOrError {
    // Convert from C to Rust types.
    let addr = from_c_string(params.go_addr);
    let username = from_c_string(params.go_username);
    let cert_der = from_go_array(params.cert_der, params.cert_der_len);
    let key_der = from_go_array(params.key_der, params.key_der_len);

    let tokio_rt = tokio::runtime::Runtime::new().unwrap();

    let result = match tokio_rt.block_on(async {
        connect_rdp_inner(
            go_ref,
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
        )
        .await
    }) {
        Ok(mut client) => {
            client.tokio_rt = Some(tokio_rt);
            ClientOrError {
                client: Box::new(client).into_raw(),
                err: CGOErrCode::ErrCodeSuccess,
            }
        }
        Err(err) => {
            error!("{:?}", err);
            ClientOrError {
                client: ptr::null_mut(),
                err: CGOErrCode::ErrCodeFailure,
            }
        }
    };

    result
}

#[derive(Debug)]
enum ConnectError {
    Tcp(IoError),
    Rdp(RdpError),
    InvalidAddr(),
    IronRdpError(IronRdpError),
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

async fn connect_rdp_inner(go_ref: usize, params: ConnectParams) -> Result<Client, ConnectError> {
    // Connect and authenticate.
    let addr = params
        .addr
        .to_socket_addrs()?
        .next()
        .ok_or(ConnectError::InvalidAddr())?;

    let tdp_sd_acknowledge = Box::new(
        move |mut ack: SharedDirectoryAcknowledge| -> RdpResult<()> {
            debug!("sending TDP SharedDirectoryAcknowledge: {:?}", ack);
            unsafe {
                if tdp_sd_acknowledge(go_ref, &mut ack) != CGOErrCode::ErrCodeSuccess {
                    return Err(RdpError::TryError(String::from(
                        "call to tdp_sd_acknowledge failed",
                    )));
                }
                Ok(())
            }
        },
    );

    let tdp_sd_info_request = Box::new(move |req: SharedDirectoryInfoRequest| -> RdpResult<()> {
        debug!("sending TDP SharedDirectoryInfoRequest: {:?}", req);
        // Create C compatible string from req.path
        match req.path.to_cstring() {
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
                Err(RdpError::TryError(format!(
                    "path contained characters that couldn't be converted to a C string: {:?}",
                    req.path
                )))
            }
        }
    });

    let tdp_sd_create_request =
        Box::new(move |req: SharedDirectoryCreateRequest| -> RdpResult<()> {
            debug!("sending TDP SharedDirectoryCreateRequest: {:?}", req);
            // Create C compatible string from req.path
            match req.path.to_cstring() {
                Ok(c_string) => {
                    unsafe {
                        let err = tdp_sd_create_request(
                            go_ref,
                            &mut CGOSharedDirectoryCreateRequest {
                                completion_id: req.completion_id,
                                directory_id: req.directory_id,
                                file_type: req.file_type,
                                path: c_string.as_ptr(),
                            },
                        );
                        if err != CGOErrCode::ErrCodeSuccess {
                            return Err(RdpError::TryError(String::from(
                                "call to tdp_sd_create_request failed",
                            )));
                        };
                    }
                    Ok(())
                }
                Err(_) => {
                    // TODO(isaiah): change TryError to TeleportError for a generic error caused by Teleport specific code.
                    Err(RdpError::TryError(format!(
                        "path contained characters that couldn't be converted to a C string: {:?}",
                        req.path
                    )))
                }
            }
        });

    let tdp_sd_delete_request =
        Box::new(move |req: SharedDirectoryDeleteRequest| -> RdpResult<()> {
            debug!("sending TDP SharedDirectoryDeleteRequest: {:?}", req);
            // Create C compatible string from req.path
            match req.path.to_cstring() {
                Ok(c_string) => {
                    unsafe {
                        let err = tdp_sd_delete_request(
                            go_ref,
                            &mut CGOSharedDirectoryDeleteRequest {
                                completion_id: req.completion_id,
                                directory_id: req.directory_id,
                                path: c_string.as_ptr(),
                            },
                        );
                        if err != CGOErrCode::ErrCodeSuccess {
                            return Err(RdpError::TryError(String::from(
                                "call to tdp_sd_delete_request failed",
                            )));
                        };
                    }
                    Ok(())
                }
                Err(_) => {
                    // TODO(isaiah): change TryError to TeleportError for a generic error caused by Teleport specific code.
                    Err(RdpError::TryError(format!(
                        "path contained characters that couldn't be converted to a C string: {:?}",
                        req.path
                    )))
                }
            }
        });

    let tdp_sd_list_request = Box::new(move |req: SharedDirectoryListRequest| -> RdpResult<()> {
        debug!("sending TDP SharedDirectoryListRequest: {:?}", req);
        // Create C compatible string from req.path
        match req.path.to_cstring() {
            Ok(c_string) => {
                unsafe {
                    let err = tdp_sd_list_request(
                        go_ref,
                        &mut CGOSharedDirectoryListRequest {
                            completion_id: req.completion_id,
                            directory_id: req.directory_id,
                            path: c_string.as_ptr(),
                        },
                    );
                    if err != CGOErrCode::ErrCodeSuccess {
                        return Err(RdpError::TryError(String::from(
                            "call to tdp_sd_list_request failed",
                        )));
                    };
                }
                Ok(())
            }
            Err(_) => {
                // TODO(isaiah): change TryError to TeleportError for a generic error caused by Teleport specific code.
                Err(RdpError::TryError(format!(
                    "path contained characters that couldn't be converted to a C string: {:?}",
                    req.path
                )))
            }
        }
    });

    let tdp_sd_read_request = Box::new(move |req: SharedDirectoryReadRequest| -> RdpResult<()> {
        debug!("sending TDP SharedDirectoryReadRequest: {:?}", req);
        match req.path.to_cstring() {
            Ok(c_string) => {
                unsafe {
                    let err = tdp_sd_read_request(
                        go_ref,
                        &mut CGOSharedDirectoryReadRequest {
                            completion_id: req.completion_id,
                            directory_id: req.directory_id,
                            path: c_string.as_ptr(),
                            path_length: req.path.len(),
                            offset: req.offset,
                            length: req.length,
                        },
                    );

                    if err != CGOErrCode::ErrCodeSuccess {
                        return Err(RdpError::TryError(String::from(
                            "call to tdp_sd_read_request failed",
                        )));
                    }
                }
                Ok(())
            }
            Err(_) => Err(RdpError::TryError(format!(
                "path contained characters that couldn't be converted to a C string: {:?}",
                req.path
            ))),
        }
    });

    let tdp_sd_write_request = Box::new(move |req: SharedDirectoryWriteRequest| -> RdpResult<()> {
        debug!("sending TDP SharedDirectoryWriteRequest: {:?}", req);
        match req.path.to_cstring() {
            Ok(c_string) => {
                unsafe {
                    let err = tdp_sd_write_request(
                        go_ref,
                        &mut CGOSharedDirectoryWriteRequest {
                            completion_id: req.completion_id,
                            directory_id: req.directory_id,
                            offset: req.offset,
                            path: c_string.as_ptr(),
                            path_length: req.path.len(),
                            write_data_length: req.write_data.len() as u32,
                            write_data: req.write_data.as_ptr() as *mut u8,
                        },
                    );

                    if err != CGOErrCode::ErrCodeSuccess {
                        return Err(RdpError::TryError(String::from(
                            "call to tdp_sd_write_failed",
                        )));
                    }
                }
                Ok(())
            }
            Err(_) => Err(RdpError::TryError(format!(
                "path contained characters that couldn't be converted to a C string: {:?}",
                req.path
            ))),
        }
    });

    let tdp_sd_move_request = Box::new(move |req: SharedDirectoryMoveRequest| -> RdpResult<()> {
        debug!("sending TDP SharedDirectoryMoveRequest: {:?}", req);
        match req.original_path.to_cstring() {
            Ok(original_path) => match req.new_path.to_cstring() {
                Ok(new_path) => {
                    unsafe {
                        let err = tdp_sd_move_request(
                            go_ref,
                            &mut CGOSharedDirectoryMoveRequest {
                                completion_id: req.completion_id,
                                directory_id: req.directory_id,
                                original_path: original_path.as_ptr(),
                                new_path: new_path.as_ptr(),
                            },
                        );

                        if err != CGOErrCode::ErrCodeSuccess {
                            return Err(RdpError::TryError(String::from(
                                "call to tdp_sd_Move_failed",
                            )));
                        }
                    }
                    Ok(())
                }
                Err(_) => Err(RdpError::TryError(format!(
                    "new_path contained characters that couldn't be converted to a C string: {:?}",
                    req.new_path
                ))),
            },
            Err(_) => Err(RdpError::TryError(format!(
                "original_path contained characters that couldn't be converted to a C string: {:?}",
                req.original_path
            ))),
        }
    });

    let addr = ironrdp::session::connection_sequence::Address::lookup_addr("54.144.205.187:3389")?; //todo(isaiah): hardcoded

    let stream = match TokioTcpStream::connect(addr.sock)
        .await
        .map_err(IronRdpError::Connection)
    {
        Ok(it) => it,
        Err(err) => return Err(ConnectError::IronRdpError(err)),
    };

    let pass = std::env::var("RDP_PASSWORD").unwrap();

    let input = InputConfig {
        credentials: AuthIdentity {
            username: "Administrator".to_string(),
            password: Secret::new(pass), //todo(isaiah): hardcoded
            domain: None,
        },
        security_protocol: ironrdp::pdu::SecurityProtocol::HYBRID_EX,
        keyboard_type: ironrdp::pdu::gcc::KeyboardType::IbmEnhanced,
        keyboard_subtype: 0,
        keyboard_functional_keys_count: 12,
        ime_file_name: "".to_string(),
        dig_product_id: "".to_string(),
        width: 1728, //todo(isaiah): hardcoded
        height: 932, //todo(isaiah): hardcoded
        global_channel_name: "GLOBAL".to_string(),
        user_channel_name: "USER".to_string(),
        graphics_config: None,
    };

    let (connection_sequence_result, reader, writer) = match process_connection_sequence(
        stream.compat(),
        &addr,
        &input,
        establish_tls,
        Box::new(RequestClientFactory),
    )
    .await
    {
        Ok(it) => it,
        Err(err) => return Err(ConnectError::IronRdpError(err)),
    };

    let processor = ActiveStageProcessor::new(input, None, connection_sequence_result);

    let client = Client {
        iron_rdp_client: IronRDPClient {
            reader,
            writer,
            processor,
        },
        tokio_rt: None,
        go_ref,
    };

    Ok(client)
}

type TlsStream = tokio_util::compat::Compat<tokio_rustls::client::TlsStream<TokioTcpStream>>;
mod danger {
    use std::time::SystemTime;

    use tokio_rustls::rustls::client::ServerCertVerified;
    use tokio_rustls::rustls::{Certificate, Error, ServerName};

    pub struct NoCertificateVerification;

    impl tokio_rustls::rustls::client::ServerCertVerifier for NoCertificateVerification {
        fn verify_server_cert(
            &self,
            _end_entity: &Certificate,
            _intermediates: &[Certificate],
            _server_name: &ServerName,
            _scts: &mut dyn Iterator<Item = &[u8]>,
            _ocsp_response: &[u8],
            _now: SystemTime,
        ) -> Result<ServerCertVerified, Error> {
            Ok(tokio_rustls::rustls::client::ServerCertVerified::assertion())
        }
    }
}

// TODO: this can be refactored into a separate `ironrdp-tls` crate (all native clients will do the same TLS dance)
pub async fn establish_tls(
    stream: tokio_util::compat::Compat<TokioTcpStream>,
) -> Result<UpgradedStream<TlsStream>, IronRdpError> {
    let stream = stream.into_inner();

    let mut tls_stream = {
        let mut client_config = tokio_rustls::rustls::client::ClientConfig::builder()
            .with_safe_defaults()
            .with_custom_certificate_verifier(std::sync::Arc::new(
                danger::NoCertificateVerification,
            ))
            .with_no_client_auth();
        // This adds support for the SSLKEYLOGFILE env variable (https://wiki.wireshark.org/TLS#using-the-pre-master-secret)
        client_config.key_log = std::sync::Arc::new(tokio_rustls::rustls::KeyLogFile::new());
        let rc_config = std::sync::Arc::new(client_config);
        let example_com = "stub_string".try_into().unwrap();
        let connector = tokio_rustls::TlsConnector::from(rc_config);
        connector.connect(example_com, stream).await?
    };

    tls_stream.flush().await?;

    let server_public_key = {
        let cert = tls_stream
            .get_ref()
            .1
            .peer_certificates()
            .ok_or(IronRdpError::MissingPeerCertificate)?[0]
            .as_ref();
        get_tls_peer_pubkey(cert.to_vec())?
    };

    Ok(UpgradedStream {
        stream: tls_stream.compat(),
        server_public_key,
    })
}

fn get_tls_peer_pubkey(cert: Vec<u8>) -> std::io::Result<Vec<u8>> {
    let res = X509Certificate::from_der(&cert[..]).map_err(|_| {
        std::io::Error::new(std::io::ErrorKind::InvalidData, "Invalid der certificate.")
    })?;
    let public_key = res.1.tbs_certificate.subject_pki.subject_public_key;

    Ok(public_key.data.to_vec())
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
            rdpdr::CHANNEL_NAME => {
                let responses = self.rdpdr.read_and_create_reply(message)?;
                let chan = &rdpdr::CHANNEL_NAME.to_string();
                for resp in responses {
                    self.mcs.write(chan, resp)?;
                }
                Ok(())
            }
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
                &format!("Invalid channel name {channel_name:?}"),
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

    fn write_rdpdr(&mut self, messages: Messages) -> RdpResult<()> {
        let chan = &rdpdr::CHANNEL_NAME.to_string();
        for message in messages {
            self.mcs.write(chan, message)?;
        }
        Ok(())
    }

    pub fn handle_client_device_list_announce(
        &mut self,
        req: rdpdr::ClientDeviceListAnnounce,
    ) -> RdpResult<()> {
        let messages = self.rdpdr.handle_client_device_list_announce(req)?;
        self.write_rdpdr(messages)
    }

    pub fn handle_tdp_sd_info_response(
        &mut self,
        res: SharedDirectoryInfoResponse,
    ) -> RdpResult<()> {
        let messages = self.rdpdr.handle_tdp_sd_info_response(res)?;
        self.write_rdpdr(messages)
    }

    pub fn handle_tdp_sd_create_response(
        &mut self,
        res: SharedDirectoryCreateResponse,
    ) -> RdpResult<()> {
        let messages = self.rdpdr.handle_tdp_sd_create_response(res)?;
        self.write_rdpdr(messages)
    }

    pub fn handle_tdp_sd_delete_response(
        &mut self,
        res: SharedDirectoryDeleteResponse,
    ) -> RdpResult<()> {
        let messages = self.rdpdr.handle_tdp_sd_delete_response(res)?;
        self.write_rdpdr(messages)
    }

    pub fn handle_tdp_sd_list_response(
        &mut self,
        res: SharedDirectoryListResponse,
    ) -> RdpResult<()> {
        let messages = self.rdpdr.handle_tdp_sd_list_response(res)?;
        self.write_rdpdr(messages)
    }

    pub fn handle_tdp_sd_read_response(
        &mut self,
        res: SharedDirectoryReadResponse,
    ) -> RdpResult<()> {
        let messages = self.rdpdr.handle_tdp_sd_read_response(res)?;
        self.write_rdpdr(messages)
    }

    pub fn handle_tdp_sd_write_response(
        &mut self,
        res: SharedDirectoryWriteResponse,
    ) -> RdpResult<()> {
        let messages = self.rdpdr.handle_tdp_sd_write_response(res)?;
        self.write_rdpdr(messages)
    }

    pub fn handle_tdp_sd_move_response(
        &mut self,
        res: SharedDirectoryMoveResponse,
    ) -> RdpResult<()> {
        let messages = self.rdpdr.handle_tdp_sd_move_response(res)?;
        self.write_rdpdr(messages)
    }

    pub fn shutdown(&mut self) -> RdpResult<()> {
        self.mcs.shutdown()
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
        let w: u16 = image.width() as u16;
        let h: u16 = image.height() as u16;
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

impl CGOPNG {
    fn from_image_region(image: &DecodedImage, region: &Rectangle) -> Result<Self, RdpError> {
        let mut res = CGOPNG {
            dest_left: region.left,
            dest_top: region.top,
            dest_right: region.right,
            dest_bottom: region.bottom,
            data_ptr: ptr::null_mut(),
            data_len: 0,
            data_cap: 0,
        };

        let mut encoded = Vec::with_capacity(8192);
        encode_png(
            &mut encoded,
            region.width(),
            region.height(),
            extract_partial_image(image, region),
        )
        .map_err(|err| RdpError::TryError(format!("failed to encode bitmap to png: {err:?}")))?;

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

#[cfg(unix)]
fn wait_for_fd(fd: usize) -> RdpResult<()> {
    let fds = &mut libc::pollfd {
        fd: fd as i32,
        events: libc::POLLIN,
        revents: 0,
    };
    loop {
        let res = unsafe { libc::poll(fds, 1, -1) };

        // We only use a single fd and can't timeout, so
        // res will either be 1 for success or -1 for failure.
        if res != 1 {
            let os_err = std::io::Error::last_os_error();
            match os_err.raw_os_error() {
                Some(libc::EINTR) | Some(libc::EAGAIN) => continue,
                _ => return Err(RdpError::Io(os_err)),
            }
        }

        // res == 1
        // POLLIN means that the fd is ready to be read from,
        // POLLHUP means that the other side of the pipe was closed,
        // but we still may have data to read.
        if fds.revents & (libc::POLLIN | libc::POLLHUP) != 0 {
            return Ok(()); // ready for a read
        } else if fds.revents & libc::POLLNVAL != 0 {
            return Err(RdpError::Io(IoError::new(
                std::io::ErrorKind::InvalidInput,
                "invalid fd",
            )));
        } else {
            // fds.revents & libc::POLLERR != 0
            return Err(RdpError::Io(IoError::new(
                std::io::ErrorKind::Other,
                "error on fd",
            )));
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

/// `read_rdp_output` reads incoming RDP bitmap frames from client at client_ref, encodes bitmap
/// as a png and forwards them to handle_png.
///
/// # Safety
///
/// `client_ptr` must be a valid pointer to a Client.
/// `handle_png` *must not* free the memory of CGOPNG.
#[no_mangle]
pub unsafe extern "C" fn read_rdp_output(client_ptr: *mut Client) -> CGOReadRdpOutputReturns {
    let client = match Client::from_ptr(client_ptr) {
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

    if client.tokio_rt.is_none() {
        error!("tokio runtime not initialized");
        return ReadRdpOutputReturns {
            user_message: "unexpected error".to_string(),
            disconnect_code: CGODisconnectCode::DisconnectCodeUnknown,
            err_code: CGOErrCode::ErrCodeFailure,
        }
        .into();
    }

    // TODO(isaiah): make a client.block_on()
    client
        .tokio_rt
        .as_ref()
        .unwrap()
        .handle()
        .clone()
        .block_on(async { read_rdp_output_inner(client).await.into() })
}

async fn read_rdp_output_inner(client: &mut Client) -> ReadRdpOutputReturns {
    let client_ref = client.go_ref;
    let mut image = DecodedImage::new(
        PixelFormat::RgbA32,
        1728, //todo(isaiah): hardcoded
        932,  //todo(isaiah): hardcoded
    );

    loop {
        let frame = match client.read_frame().await {
            Ok(it) => match it {
                Some(frame) => frame.freeze(),
                None => {
                    // IronRDP returns RdpError::AccessDenied here.
                    error!("access denied");
                    return ReadRdpOutputReturns {
                        user_message: "Access denied".to_string(),
                        disconnect_code: CGODisconnectCode::DisconnectCodeUnknown,
                        err_code: CGOErrCode::ErrCodeFailure,
                    };
                }
            },
            Err(err) => {
                error!("failed to read frame: {}", err);
                return ReadRdpOutputReturns {
                    user_message: "Failed to read frame".to_string(),
                    disconnect_code: CGODisconnectCode::DisconnectCodeUnknown,
                    err_code: CGOErrCode::ErrCodeFailure,
                };
            }
        };
        let outputs = match client.process_frame(&mut image, frame) {
            Ok(o) => o,
            Err(err) => {
                error!("failed to process frame: {}", err);
                return ReadRdpOutputReturns {
                    user_message: "Failed to process frame".to_string(),
                    disconnect_code: CGODisconnectCode::DisconnectCodeUnknown,
                    err_code: CGOErrCode::ErrCodeFailure,
                };
            }
        };

        for out in outputs {
            match out {
                ActiveStageOutput::ResponseFrame(frame) => match client.write_frame(&frame).await {
                    Ok(_) => {}
                    Err(err) => {
                        error!("failed to write frame: {}", err);
                        return ReadRdpOutputReturns {
                            user_message: "Failed to write frame".to_string(),
                            disconnect_code: CGODisconnectCode::DisconnectCodeUnknown,
                            err_code: CGOErrCode::ErrCodeFailure,
                        };
                    }
                },
                ActiveStageOutput::GraphicsUpdate(region) => {
                    let mut cpng = match CGOPNG::from_image_region(&image, &region) {
                        Ok(cpng) => cpng,
                        Err(e) => {
                            if region.left == 0
                                && region.right == 0
                                && region.top == 0
                                && region.bottom == 0
                            {
                                debug!("got a blank frame, ignoring");
                                // This is a special case where the server is sending us a
                                // "blank" frame. We can safely ignore this.
                                continue;
                            }
                            error!("failed to convert image to png: {e:?}");
                            return ReadRdpOutputReturns {
                                user_message: "Failed to convert image to png".to_string(),
                                disconnect_code: CGODisconnectCode::DisconnectCodeUnknown,
                                err_code: CGOErrCode::ErrCodeFailure,
                            };
                        }
                    };

                    let err = unsafe { handle_png(client_ref, &mut cpng) as CGOErrCode };
                    if err != CGOErrCode::ErrCodeSuccess {
                        return ReadRdpOutputReturns {
                            user_message: "Failed to handle png".to_string(),
                            disconnect_code: CGODisconnectCode::DisconnectCodeUnknown,
                            err_code: err,
                        };
                    };
                }
                ActiveStageOutput::Terminate => {
                    // TODO(isaiah): This can also mean message on unknown channel received,
                    // see IronRDP.
                    warn!("Connection terminated by server");
                    return ReadRdpOutputReturns {
                        user_message: "Connection terminated by RDP server".to_string(),
                        disconnect_code: CGODisconnectCode::DisconnectCodeServer,
                        err_code: CGOErrCode::ErrCodeSuccess,
                    };
                }
            }
        }
    }
}

fn extract_partial_image(image: &DecodedImage, region: &Rectangle) -> Vec<u8> {
    let pixel_size = usize::from(image.pixel_format().bytes_per_pixel());

    let image_width = usize::try_from(image.width()).unwrap();

    let region_top = usize::from(region.top);
    let region_left = usize::from(region.left);
    let region_width = usize::from(region.width());
    let region_height = usize::from(region.height());

    let dst_buf_size = region_width * region_height * pixel_size;
    let mut dst = vec![0; dst_buf_size];

    let src = image.data();

    let image_stride = image_width * pixel_size;
    let region_stride = region_width * pixel_size;

    for row in 0..region_height {
        let src_begin = image_stride * (region_top + row) + region_left * pixel_size;
        let src_end = src_begin + region_stride;
        let src_slice = &src[src_begin..src_end];

        let target_begin = region_stride * row;
        let target_end = target_begin + region_stride;
        let target_slice = &mut dst[target_begin..target_end];

        target_slice.copy_from_slice(src_slice);
    }

    dst
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
    let client = match Client::from_ptr(client_ptr) {
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

    client
        .tokio_rt
        .as_ref()
        .unwrap()
        .handle()
        .clone()
        .block_on(async {
            // todo(isaiah): need a lock here? client.write_frame is also used in the main bitmap handling loop.
            client.write_frame(data.as_slice()).await.unwrap(); // todo(isaiah): handle error
                                                                // todo(isaiah): need to flush here?
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
    drop(Client::from_raw(client_ptr))
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
