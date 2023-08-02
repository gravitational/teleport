use crate::util::{extract_tls_server_public_key, hang_until_read_ready};
use crate::{
    handle_remote_fx_frame, util::to_c_string, CGODisconnectCode, CGOErrCode, CGOMousePointerEvent,
    CGOPointerButton, CGOPointerWheel, CGOReadRdpOutputReturns,
};
use bytes::BytesMut;
use ironrdp_blocking::Framed;
use ironrdp_pdu::input::fast_path::{FastPathInput, FastPathInputEvent};
use ironrdp_pdu::input::mouse::PointerFlags;
use ironrdp_pdu::input::MousePdu;
use ironrdp_pdu::PduParsing;
use ironrdp_session::utils::swap_hashmap_kv;
use ironrdp_session::{x224, ActiveStageOutput, SessionError, SessionErrorKind, SessionResult};
use native_tls::TlsStream;
use rdp::model::error::*;
use sspi::network_client::reqwest_network_client::RequestClientFactory;
use std::io::{self, Error as IoError};
use std::net::{TcpStream, ToSocketAddrs};
use std::os::fd::AsRawFd;
use std::sync::Mutex;

/// Client has an unusual lifecycle:
/// - The function client_connect calls Client::connect(), which creates it on the heap (Box::new), grabs a raw pointer(Box::into_raw),
///   and returns in to Go.
/// - Most other exported rdp functions (pub unsafe extern "C") take the raw pointer and convert it Client::from_raw
///   to a reference (&Client), which can then be used without dropping the client.
/// - The function client_drop takes the raw pointer and drops it.
///
/// The Client is forced to be Sync. See "Go/Rust Interface" in ../README.md for more details.
pub struct Client {
    /// We need to use the ironrdp_client as an `&mut ironrdp_client`, but in practice we only
    /// ever have access to this Client itself as an `&Client`. Therefore we use a Mutex to allow
    /// us to get a mutable reference to the ironrdp_client when we need it.
    ironrdp_client: Mutex<IronRDPClient>,
    /// The raw file descriptor of the underlying connection with the RDP server. This is used to
    /// synchronize Mutex lock attempts on the ironrdp_client. By waiting for this file descriptor
    /// to tell us that the file descriptor is ready for a read/write, before locking the mutex, we
    /// can avoid deadlocks.
    raw_fd: i32,
    /// The raw pointer back to the owning Go struct. This is used to call back into Go.
    go_ref: usize,
}

/// Forces the compiler to check that the Client struct is Send + Sync.
/// See the README for more details.
const _: () = {
    const fn assert_send_sync<T: Send + Sync>() {}
    assert_send_sync::<Client>();
};

impl Client {
    pub fn connect(go_ref: usize, params: ConnectParams) -> Result<*const Self, ConnectError> {
        // todo(isaiah): is this match necessary? better way to handle errors within this section besides
        // all the match-it patterns?
        match {
            let server_addr = params.addr;
            let server_socket_addr = server_addr.to_socket_addrs()?.next().unwrap();

            debug!("connecting to server at {}", server_addr);
            let stream = match TcpStream::connect(server_addr.clone()) {
                Ok(it) => it,
                Err(err) => {
                    error!("tcp connect error: {:?}", err);
                    return Err(ConnectError::IronRdpError(SessionError::new(
                        "tcp connect error",
                        SessionErrorKind::General,
                    )));
                }
            };

            let raw_fd = stream.as_raw_fd();

            let mut framed = ironrdp_blocking::Framed::new(stream);

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

            let should_upgrade = match ironrdp_blocking::connect_begin(&mut framed, &mut connector)
            {
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

            // Upgrade to TLS
            // todo(isaiah): move this to ironrdp?
            let tls_connector = match native_tls::TlsConnector::builder()
                .danger_accept_invalid_certs(true) // We don't verify the server's cert
                .build()
            {
                Ok(it) => it,
                Err(err) => {
                    error!("TlsConnector::new() error: {:?}", err);
                    return Err(ConnectError::IronRdpError(SessionError::new(
                        "TlsConnector::new() error",
                        SessionErrorKind::General,
                    )));
                }
            };

            let upgraded_stream =
                match tls_connector.connect(&server_socket_addr.to_string(), initial_stream) {
                    Ok(it) => it,
                    Err(err) => {
                        error!("TLS handshake error: {:?}", err);
                        return Err(ConnectError::IronRdpError(SessionError::new(
                            "TLS handshake error",
                            SessionErrorKind::General,
                        )));
                    }
                };

            let server_public_key = {
                let cert = upgraded_stream
                    .peer_certificate()
                    .map_err(|e| io::Error::new(io::ErrorKind::Other, e))?
                    .ok_or_else(|| {
                        io::Error::new(io::ErrorKind::Other, "peer certificate is missing")
                    })?;
                let cert = cert
                    .to_der()
                    .map_err(|e| io::Error::new(io::ErrorKind::Other, e))?;
                extract_tls_server_public_key(&cert)?
            };

            let upgraded = ironrdp_blocking::mark_as_upgraded(
                should_upgrade,
                &mut connector,
                server_public_key,
            );

            let mut upgraded_framed = ironrdp_blocking::Framed::new(upgraded_stream);

            let connection_result =
                match ironrdp_blocking::connect_finalize(upgraded, &mut upgraded_framed, connector)
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

            Ok((upgraded_framed, x224_processor, raw_fd))
        } {
            Ok((upgraded_framed, x224_processor, raw_fd)) => Ok(Box::into_raw(Box::new(Self {
                ironrdp_client: Mutex::new(IronRDPClient::new(upgraded_framed, x224_processor)),
                raw_fd,
                go_ref,
            }))),
            Err(err) => Err(err),
        }
    }

    pub fn null() -> *const Self {
        std::ptr::null()
    }

    /// # Safety
    ///
    /// client_ptr MUST be a valid pointer.
    /// (validity defined by https://doc.rust-lang.org/nightly/core/primitive.pointer.html#method.as_ref-1)
    pub unsafe fn from_raw<'a>(client_ptr: *const Self) -> Result<&'a Client, CGOErrCode> {
        match client_ptr.as_ref() {
            Some(it) => Ok(it),
            None => {
                error!("Client pointer is null");
                Err(CGOErrCode::ErrCodeClientPtr)
            }
        }
    }

    /// # Safety
    ///
    /// Calling this twice on the same Client
    /// pointer will result in a double free.
    pub unsafe fn drop(ptr: *mut Self) {
        if !ptr.is_null() {
            drop(Box::from_raw(ptr))
        }
    }

    /// Reads and processes PDUs from the RDP server in a loop.
    pub fn read_rdp_output(&self) -> CGOReadRdpOutputReturns {
        loop {
            trace!("awaiting frame from rdp server");
            let (action, frame) = match self.read_pdu() {
                Ok(it) => it,
                Err(e) => {
                    error!("error reading PDU: {:?}", e);
                    return CGOReadRdpOutputReturns {
                        user_message: to_c_string("error reading PDU").unwrap(),
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
                    let result = self.process_x224_frame(&frame);
                    if let Some(return_value) = self.process_active_stage_result(result) {
                        return return_value;
                    }
                }
                ironrdp_pdu::Action::FastPath => match self.handle_remote_fx_frame(frame) {
                    CGOErrCode::ErrCodeSuccess => continue,
                    err => {
                        error!("failed to process fastpath frame: {:?}", err);
                        return CGOReadRdpOutputReturns {
                            user_message: to_c_string("Failed to process fastpath frame").unwrap(),
                            disconnect_code: CGODisconnectCode::DisconnectCodeUnknown,
                            err_code: err,
                        };
                    }
                },
            };
        }
    }

    fn handle_remote_fx_frame(&self, mut frame: BytesMut) -> CGOErrCode {
        unsafe { handle_remote_fx_frame(self.go_ref, frame.as_mut_ptr(), frame.len() as u32) }
    }

    pub fn write_rdp_pointer(&self, pointer: CGOMousePointerEvent) -> CGOErrCode {
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

        self.write_all(&data).unwrap(); // todo(isaiah): handle error

        CGOErrCode::ErrCodeSuccess
    }

    pub fn handle_tdp_rdp_response_pdu(&self, res: Vec<u8>) -> CGOErrCode {
        match self.process_active_stage_result(Ok(vec![ActiveStageOutput::ResponseFrame(res)])) {
            Some(ret) => ret.err_code,
            None => CGOErrCode::ErrCodeSuccess,
        }
    }

    /// Iterates through any response frames in result, sending them to the RDP server.
    /// Typically returns None if everything goes as expected and the session should continue.
    // TODO(isaiah): this api is weird, should probably return a Result instead of an Option.
    pub fn process_active_stage_result(
        &self,
        result: SessionResult<Vec<ActiveStageOutput>>,
    ) -> Option<CGOReadRdpOutputReturns> {
        match result {
            Ok(outputs) => {
                for output in outputs {
                    match output {
                        ActiveStageOutput::ResponseFrame(response) => {
                            match self.write_all(&response) {
                                Ok(_) => {
                                    trace!("write_all succeeded, continuing");
                                    continue;
                                }
                                Err(e) => {
                                    return Some(CGOReadRdpOutputReturns {
                                        user_message: to_c_string(&format!(
                                            "Failed to write frame: {}",
                                            e
                                        ))
                                        .unwrap(),
                                        disconnect_code: CGODisconnectCode::DisconnectCodeUnknown,
                                        err_code: CGOErrCode::ErrCodeFailure,
                                    });
                                }
                            }
                        }
                        ActiveStageOutput::Terminate => {
                            return Some(CGOReadRdpOutputReturns {
                                user_message: to_c_string("RDP session terminated").unwrap(),
                                disconnect_code: CGODisconnectCode::DisconnectCodeUnknown,
                                err_code: CGOErrCode::ErrCodeSuccess,
                            });
                        }
                        ActiveStageOutput::GraphicsUpdate(_) => {
                            error!("unexpected GraphicsUpdate, this should be handled on the client side");
                            return Some(CGOReadRdpOutputReturns {
                                user_message: to_c_string("Server error").unwrap(),
                                disconnect_code: CGODisconnectCode::DisconnectCodeUnknown,
                                err_code: CGOErrCode::ErrCodeFailure,
                            });
                        }
                    }
                }
            }
            Err(err) => {
                error!("failed to process frame: {}", err);
                return Some(CGOReadRdpOutputReturns {
                    user_message: to_c_string("Failed to process frame").unwrap(),
                    disconnect_code: CGODisconnectCode::DisconnectCodeUnknown,
                    err_code: CGOErrCode::ErrCodeFailure,
                });
            }
        }

        // All outputs were response frames, return None to indicate that the client should continue
        trace!("process_active_stage_result succeeded, returning None");
        None
    }

    /// read_pdu hangs until it reads the next PDU from the RDP server,
    /// then returns the PDU Action and the PDU data.
    fn read_pdu(&self) -> io::Result<(ironrdp_pdu::Action, BytesMut)> {
        // Hang until the socket is ready to be read from
        hang_until_read_ready(self.raw_fd)?;
        // Only lock the client and read once there's actually data to read,
        // otherwise we might block the client from writing (deadlock).
        self.ironrdp_client.lock().unwrap().framed.read_pdu()
    }

    fn write_all(&self, buf: &[u8]) -> io::Result<()> {
        self.ironrdp_client.lock().unwrap().framed.write_all(buf)
    }

    fn process_x224_frame(&self, frame: &[u8]) -> SessionResult<Vec<ActiveStageOutput>> {
        let output = self
            .ironrdp_client
            .lock()
            .unwrap()
            .x224_processor
            .process(frame)?;
        let mut stage_outputs = Vec::new();
        if !output.is_empty() {
            stage_outputs.push(ActiveStageOutput::ResponseFrame(output));
        }
        Ok(stage_outputs)
    }
}

#[derive(Debug)]
pub struct ConnectParams {
    pub addr: String,
    pub username: String,
    pub cert_der: Vec<u8>,
    pub key_der: Vec<u8>,
    pub screen_width: u16,
    pub screen_height: u16,
    pub allow_clipboard: bool,
    pub allow_directory_sharing: bool,
    pub show_desktop_wallpaper: bool,
}

#[derive(Debug)]
pub enum ConnectError {
    Tcp(IoError),
    Rdp(RdpError),
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

/// IronRDPClient is the rdp client itself
struct IronRDPClient {
    /// Holds the underlying tls connection to the rdp server,
    /// wrapped in a Framed object which handles reading (`self.framed.read_pdu()`)
    /// and writing (`self.framed.write_all(buf)`) from/to the stream.
    framed: FramedTlsStream,
    /// When pdus are read from the stream (`self.framed.read_pdu()`), they are passed to the
    /// x224_processor for processing.
    x224_processor: x224::Processor,
}

impl IronRDPClient {
    pub fn new(upgraded_framed: FramedTlsStream, x224_processor: x224::Processor) -> Self {
        Self {
            framed: upgraded_framed,
            x224_processor,
        }
    }
}

type FramedTlsStream = Framed<TlsStream<TcpStream>>;
