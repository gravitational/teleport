use crate::{
    handle_remote_fx_frame, util::to_c_string, CGODisconnectCode, CGOErrCode, CGOMousePointerEvent,
    CGOPointerButton, CGOPointerWheel, CGOReadRdpOutputReturns,
};
use bytes::BytesMut;
use ironrdp_pdu::input::fast_path::{FastPathInput, FastPathInputEvent};
use ironrdp_pdu::input::mouse::PointerFlags;
use ironrdp_pdu::input::MousePdu;
use ironrdp_pdu::PduParsing;
use ironrdp_session::utils::swap_hashmap_kv;
use ironrdp_session::{x224, ActiveStageOutput, SessionError, SessionErrorKind, SessionResult};
use rdp::model::error::*;
use sspi::network_client::reqwest_network_client::RequestClientFactory;
use static_init::dynamic;
use std::io::{self, Error as IoError};
use std::net::ToSocketAddrs;
use tokio::io::{split, ReadHalf, WriteHalf};
use tokio::net::TcpStream as TokioTcpStream;
use tokio::sync::Mutex;

#[dynamic]
static TOKIO_RT: tokio::runtime::Runtime = tokio::runtime::Runtime::new().unwrap();

/// Client has an unusual lifecycle:
/// - The function client_connect calls Client::connect(), which creates it on the heap (Box::new), grabs a raw pointer(Box::into_raw),
///   and returns in to Go.
/// - Most other exported rdp functions (pub unsafe extern "C") take the raw pointer and convert it Client::from_raw
///   to a reference (&Client), which can then be used without dropping the client.
/// - The function client_drop takes the raw pointer and drops it.
///
/// The Client is forced to be Sync. See "Go/Rust Interface" in ../README.md for more details.
///
/// The Client makes use of asynchronous rust via the tokio runtime. A single runtime is created in Client::connect and held on to by the
/// Client. Functions which require async/await functionality Since these functions are called from
/// Go, the "current thread" can be thought of as whichever goroutine the exported rdp function is called from. Because the client might
/// be being used by multiple goroutines concurrently, it is up to the programmer to consider any synchronization mechanisms that might
/// need to be implemented as features are added to the Client going forward.
pub struct Client {
    iron_rdp_client: IronRDPClient,
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
        match TOKIO_RT.block_on(async {
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

            // Create a framed stream for use by connect_begin
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

            let should_upgrade =
                match ironrdp_tokio::connect_begin(&mut framed, &mut connector).await {
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

            // Take the stream back out of the framed object for upgrading
            let initial_stream = framed.into_inner_no_leftover();
            let (upgraded_stream, server_public_key) =
                ironrdp_tls::upgrade(initial_stream, &server_socket_addr.ip().to_string()).await?;

            let upgraded =
                ironrdp_tokio::mark_as_upgraded(should_upgrade, &mut connector, server_public_key);

            let mut upgraded_framed = ironrdp_tokio::TokioFramed::new(upgraded_stream);

            let connection_result =
                match ironrdp_tokio::connect_finalize(upgraded, &mut upgraded_framed, connector)
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

            // Take the stream back out of the framed object for splitting
            let upgraded_stream = upgraded_framed.into_inner_no_leftover();
            let (read_stream, write_stream) = split(upgraded_stream);
            let framed_reader = ironrdp_tokio::TokioFramed::new(read_stream);
            let framed_writer = ironrdp_tokio::TokioFramed::new(write_stream);

            let x224_processor = x224::Processor::new(
                swap_hashmap_kv(connection_result.static_channels),
                connection_result.user_channel_id,
                connection_result.io_channel_id,
                None,
                None,
            );

            Ok((framed_reader, framed_writer, x224_processor))
        }) {
            Ok((framed_reader, framed_writer, x224_processor)) => {
                Ok(Box::into_raw(Box::new(Self {
                    iron_rdp_client: IronRDPClient::new(
                        framed_reader,
                        framed_writer,
                        x224_processor,
                    ),
                    go_ref,
                })))
            }
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

    pub fn read_rdp_output(&self) -> CGOReadRdpOutputReturns {
        TOKIO_RT.block_on(async {
            loop {
                trace!("awaiting frame from rdp server");
                let (action, mut frame) = match self.read_pdu().await {
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
                        let result = self.process_x224_frame(&frame).await;
                        if let Some(return_value) = self.process_active_stage_result(result).await {
                            return return_value;
                        }
                    }
                    ironrdp_pdu::Action::FastPath => {
                        let go_ref = self.go_ref;
                        match unsafe {
                            handle_remote_fx_frame(go_ref, frame.as_mut_ptr(), frame.len() as u32)
                        } {
                            CGOErrCode::ErrCodeSuccess => continue,
                            err => {
                                error!("failed to process fastpath frame: {:?}", err);
                                return CGOReadRdpOutputReturns {
                                    user_message: to_c_string("Failed to process fastpath frame")
                                        .unwrap(),
                                    disconnect_code: CGODisconnectCode::DisconnectCodeUnknown,
                                    err_code: err,
                                };
                            }
                        }
                    }
                };
            }
        })
    }

    pub fn write_rdp_pointer(&self, pointer: CGOMousePointerEvent) -> CGOErrCode {
        TOKIO_RT.block_on(async {
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

            self.write_all(&data).await.unwrap(); // todo(isaiah): handle error

            CGOErrCode::ErrCodeSuccess
        })
    }

    pub fn handle_tdp_rdp_response_pdu(&self, res: Vec<u8>) -> CGOErrCode {
        TOKIO_RT.block_on(async {
            match self
                .process_active_stage_result(Ok(vec![ActiveStageOutput::ResponseFrame(res)]))
                .await
            {
                Some(ret) => ret.err_code,
                None => CGOErrCode::ErrCodeSuccess,
            }
        })
    }

    /// Iterates through any response frames in result, sending them to the RDP server.
    /// Typically returns None if everything goes as expected and the session should continue.
    // TODO(isaiah): this api is weird, should probably return a Result instead of an Option.
    pub async fn process_active_stage_result(
        &self,
        result: SessionResult<Vec<ActiveStageOutput>>,
    ) -> Option<CGOReadRdpOutputReturns> {
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

    async fn read_pdu(&self) -> io::Result<(ironrdp_pdu::Action, BytesMut)> {
        self.iron_rdp_client
            .framed_reader
            .lock()
            .await
            .read_pdu()
            .await
    }

    async fn write_all(&self, buf: &[u8]) -> io::Result<()> {
        self.iron_rdp_client
            .framed_writer
            .lock()
            .await
            .write_all(buf)
            .await
    }

    async fn process_x224_frame(&self, frame: &[u8]) -> SessionResult<Vec<ActiveStageOutput>> {
        let output = self
            .iron_rdp_client
            .x224_processor
            .lock()
            .await
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

pub struct IronRDPClient {
    framed_reader: Mutex<FramedReader>,
    framed_writer: Mutex<FramedWriter>,
    x224_processor: Mutex<x224::Processor>,
}

impl IronRDPClient {
    pub fn new(
        framed_reader: FramedReader,
        framed_writer: FramedWriter,
        x224_processor: x224::Processor,
    ) -> Self {
        Self {
            framed_reader: Mutex::new(framed_reader),
            framed_writer: Mutex::new(framed_writer),
            x224_processor: Mutex::new(x224_processor),
        }
    }
}

type FramedReader = ironrdp_tokio::TokioFramed<ReadHalf<ironrdp_tls::TlsStream<TokioTcpStream>>>;
type FramedWriter = ironrdp_tokio::TokioFramed<WriteHalf<ironrdp_tls::TlsStream<TokioTcpStream>>>;
