pub mod global;
use crate::{
    handle_remote_fx_frame, CGOErrCode, CGOKeyboardEvent, CGOMousePointerEvent, CGOPointerButton,
    CGOPointerWheel, CgoHandle,
};
use ironrdp_connector::{Config, ConnectorError};
use ironrdp_pdu::input::fast_path::{FastPathInput, FastPathInputEvent, KeyboardFlags};
use ironrdp_pdu::input::mouse::PointerFlags;
use ironrdp_pdu::input::MousePdu;
use ironrdp_pdu::nego::SecurityProtocol;
use ironrdp_pdu::rdp::capability_sets::MajorPlatformType;
use ironrdp_pdu::rdp::RdpError;
use ironrdp_pdu::PduParsing;
use ironrdp_session::x224::Processor;
use ironrdp_session::{ActiveStageOutput, SessionError, SessionResult};
use ironrdp_tls::TlsStream;
use ironrdp_tokio::{Framed, TokioStream};
use sspi::network_client::reqwest_network_client::RequestClientFactory;
use std::io::Error as IoError;
use std::net::ToSocketAddrs;
use tokio::net::TcpStream as TokioTcpStream;
use tokio::sync::mpsc::{channel, Receiver, Sender};
use tokio::sync::oneshot;

// Export this for crate level use.
pub(crate) use global::call_function_on_handle;

/// The RDP client on the Rust side of things. Each `Client`
/// corresponds with a Go `Client` specified by `cgo_handle`.
pub struct Client {
    cgo_handle: CgoHandle,
    rdp_stream: RdpStream,
    x224_processor: Processor,
    function_receiver: Option<FunctionReceiver>,
}

impl Client {
    /// Connects a new client to the RDP server specified by `params` and starts the session.
    ///
    /// After creating the connection, this function registers the newly made Client with
    /// the [`global::ClientHandles`] map, and creates a task for reading frames from the  RDP
    /// server and sending them back to Go, and receiving function calls via [`global::call_function_on_handle`]
    /// and executing them.
    ///
    /// This function hangs until the RDP session ends or a [`ClientFunction::Stop`] is dispatched
    /// (see [`global::call_function_on_handle`]).
    pub fn run(cgo_handle: CgoHandle, params: ConnectParams) -> Result<(), ClientError> {
        global::TOKIO_RT.block_on(async {
            match Self::connect(cgo_handle, params)
                .await?
                .register()
                .run_rdp_loop()
                .await
            {
                Ok(res) => res,
                Err(e) => {
                    error!("failed to receive error: {}", e);
                    Err(ClientError::InternalError)
                }
            }
        })
    }

    /// Initializes the RDP connection with the given [`ConnectParams`].
    async fn connect(cgo_handle: CgoHandle, params: ConnectParams) -> Result<Self, ClientError> {
        let server_addr = params.addr.clone();
        let server_socket_addr = server_addr.to_socket_addrs().unwrap().next().unwrap();

        let stream = TokioTcpStream::connect(&server_socket_addr).await?;

        // Create a framed stream for use by connect_begin
        let mut framed = ironrdp_tokio::TokioFramed::new(stream);

        let connector_config = create_config(params);
        let mut connector = ironrdp_connector::ClientConnector::new(connector_config)
            .with_server_addr(server_socket_addr)
            .with_server_name(server_addr)
            .with_credssp_network_client(RequestClientFactory);

        let should_upgrade = ironrdp_tokio::connect_begin(&mut framed, &mut connector).await?;

        // Take the stream back out of the framed object for upgrading
        let initial_stream = framed.into_inner_no_leftover();
        let (upgraded_stream, server_public_key) =
            ironrdp_tls::upgrade(initial_stream, &server_socket_addr.ip().to_string()).await?;

        let upgraded =
            ironrdp_tokio::mark_as_upgraded(should_upgrade, &mut connector, server_public_key);

        let mut rdp_stream = ironrdp_tokio::TokioFramed::new(upgraded_stream);

        let connection_result =
            ironrdp_tokio::connect_finalize(upgraded, &mut rdp_stream, connector).await?;

        debug!("connection_result: {:?}", connection_result);

        let x224_processor = Processor::new(
            connection_result.static_channels,
            connection_result.user_channel_id,
            connection_result.io_channel_id,
            None,
            None,
        );

        Ok(Self {
            cgo_handle,
            rdp_stream,
            x224_processor,
            function_receiver: None,
        })
    }

    /// Registers the Client with the [`global::CLIENT_HANDLES`] cache.
    ///
    /// This constitutes creating a new [`ClientHandle`]/[`FunctionReceiver`] pair,
    /// storing the [`ClientHandle`] (indexed by `self.cgo_handle`) in [`global::CLIENT_HANDLES`],
    /// and assigning the [`FunctionReceiver`] to `self.function_receiver`.
    fn register(mut self) -> Self {
        let (client_handle, function_receiver) = channel(100);
        global::CLIENT_HANDLES.insert(self.cgo_handle, client_handle);
        self.function_receiver = Some(function_receiver);
        self
    }

    /// Spawns a task for running the RDP loop which:
    /// 1. Reads new frames from the RDP server and sends them to Go.
    /// 2. Listens on the Client's function_receiver for function calls
    ///    which it then executes.
    ///
    /// Returns immediately with a receiver which callers are expected to listen
    /// on in case of any errors, or until a [`ClientFunction::Stop`] is received.
    ///
    /// The caller is responsible for ensuring that the future spawned by this function
    /// eventually returns. Failure to do so can result in a leak.
    fn run_rdp_loop(self) -> oneshot::Receiver<Result<(), ClientError>> {
        let (result_tx, result_rx) = oneshot::channel::<Result<(), ClientError>>();

        global::TOKIO_RT.spawn(async move {
            let res = self.run_rdp_loop_internal().await;
            match result_tx.send(res) {
                Ok(_) => {}
                Err(res) => {
                    error!("failed to send result: {:?}", res)
                }
            };
        });

        result_rx
    }

    async fn run_rdp_loop_internal(mut self) -> Result<(), ClientError> {
        if let Some(mut function_receiver) = self.function_receiver.take() {
            loop {
                tokio::select! {
                     res = self.rdp_stream.read_pdu() => {
                        let (action, mut frame) = res?;
                        match action {
                            ironrdp_pdu::Action::X224 => {
                                let result = self.process_x224_frame(&frame).await;
                                self.process_active_stage_result(result).await?;
                            },
                            ironrdp_pdu::Action::FastPath => {
                                unsafe {
                                    handle_remote_fx_frame(self.cgo_handle, frame.as_mut_ptr(), frame.len() as u32);
                                }
                            },
                        };
                     }
                     Some(data) = function_receiver.recv() => {
                        trace!("Client received {:?}", data);
                        match data {
                            ClientFunction::WriteRdpKey(args) => {
                                self.write_rdp_key(args).await?;
                            },
                            ClientFunction::WriteRdpPointer(args) => {
                                self.write_rdp_pointer(args).await?;
                            },
                            ClientFunction::HandleResponsePdu(args) => {
                                self.handle_response_pdu(args).await?;
                            },
                            ClientFunction::Stop => {
                                return Ok(());
                            }
                        }
                    }
                }
            }
        } else {
            error!("cannot run rdp loop before the client is registered");
            Err(ClientError::InternalError)
        }
    }

    async fn process_x224_frame(&mut self, frame: &[u8]) -> SessionResult<Vec<ActiveStageOutput>> {
        let output = self.x224_processor.process(frame)?;
        let mut stage_outputs = Vec::new();
        if !output.is_empty() {
            stage_outputs.push(ActiveStageOutput::ResponseFrame(output));
        }
        Ok(stage_outputs)
    }

    /// Iterates through any response frames in result, sending them to the RDP server.
    /// Typically returns Ok(()) if everything goes as expected and the session should continue.
    async fn process_active_stage_result(
        &mut self,
        result: SessionResult<Vec<ActiveStageOutput>>,
    ) -> Result<(), CGOErrCode> {
        let outputs = result.map_err(|_| CGOErrCode::ErrCodeFailure)?;
        for output in outputs {
            match output {
                ActiveStageOutput::ResponseFrame(response) => {
                    self.rdp_stream
                        .write_all(&response)
                        .await
                        .map_err(|_| CGOErrCode::ErrCodeFailure)?;
                }
                ActiveStageOutput::Terminate => return Err(CGOErrCode::ErrCodeSuccess),
                ActiveStageOutput::GraphicsUpdate(_) => {
                    error!("unexpected GraphicsUpdate, this should be handled on the client side");
                    return Err(CGOErrCode::ErrCodeFailure);
                }
                _ => {}
            }
        }
        Ok(())
    }

    async fn write_rdp_key(&mut self, key: CGOKeyboardEvent) -> Result<(), ClientError> {
        let mut fastpath_events = Vec::new();
        // TODO(isaiah): impl From for this
        let mut flags: KeyboardFlags = KeyboardFlags::empty();
        if !key.down {
            flags = KeyboardFlags::RELEASE;
        }
        let event = FastPathInputEvent::KeyboardEvent(flags, key.code as u8);
        fastpath_events.push(event);

        let mut data: Vec<u8> = Vec::new();
        let input_pdu = FastPathInput(fastpath_events);
        input_pdu.to_buffer(&mut data).unwrap();

        self.rdp_stream.write_all(&data).await?;
        Ok(())
    }

    async fn write_rdp_pointer(
        &mut self,
        pointer: CGOMousePointerEvent,
    ) -> Result<(), ClientError> {
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

        self.rdp_stream.write_all(&data).await?;
        Ok(())
    }

    async fn handle_response_pdu(&mut self, resp: Vec<u8>) -> Result<(), ClientError> {
        let output = Ok(vec![ActiveStageOutput::ResponseFrame(resp)]);
        self.process_active_stage_result(output)
            .await
            .map_err(|e| {
                error!("process_stage_result {:?}", e);
                ClientError::Rdp(RdpError::InvalidSecurityHeader) // TODO(isaiah, przemko)
            })?;
        Ok(())
    }
}

impl Drop for Client {
    fn drop(&mut self) {
        global::CLIENT_HANDLES.remove(self.cgo_handle)
    }
}

/// [`ClientFunction`] is an enum representing the different functions that can be called on a client.
/// Each variant corresponds to a different function, and carries the necessary arguments for that function.
/// This enum is used in conjunction with the [`call_function_on_handle`] function to call a specific function on a client.
#[derive(Debug)]
pub enum ClientFunction {
    /// Corresponds to [`Client::write_rdp_pointer`]
    WriteRdpPointer(CGOMousePointerEvent),
    /// Corresponds to [`Client::write_rdp_key`]
    WriteRdpKey(CGOKeyboardEvent),
    /// Corresponds to [`Client::handle_response_pdu`]
    HandleResponsePdu(Vec<u8>),
    /// Causes the looping future spawned by run_rdp_loop to return
    Stop,
}

/// `ClientHandle` is used to dispatch [`ClientFunction`]s calls
/// to a corresponding [`FunctionReceiver`] on a `Client`.
type ClientHandle = Sender<ClientFunction>;

/// Each `Client` has a `FunctionReceiver` that it listens to for
/// incoming [`ClientFunction`] calls sent via its corresponding
/// [`ClientHandle`].
pub type FunctionReceiver = Receiver<ClientFunction>;

type RdpStream = Framed<TokioStream<TlsStream<TokioTcpStream>>>;

fn create_config(params: ConnectParams) -> Config {
    Config {
        desktop_size: ironrdp_connector::DesktopSize {
            width: params.screen_width,
            height: params.screen_height,
        },
        security_protocol: SecurityProtocol::HYBRID_EX,
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
        platform: MajorPlatformType::UNSPECIFIED,
        no_server_pointer: false,
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
pub enum ClientError {
    Tcp(IoError),
    Rdp(RdpError),
    SessionError(SessionError),
    ConnectorError(ConnectorError),
    CGOErrCode(CGOErrCode),
    InternalError,
}

impl From<IoError> for ClientError {
    fn from(e: IoError) -> ClientError {
        ClientError::Tcp(e)
    }
}

impl From<RdpError> for ClientError {
    fn from(e: RdpError) -> ClientError {
        ClientError::Rdp(e)
    }
}

impl From<ConnectorError> for ClientError {
    fn from(value: ConnectorError) -> Self {
        ClientError::ConnectorError(value)
    }
}

impl From<CGOErrCode> for ClientError {
    fn from(value: CGOErrCode) -> Self {
        ClientError::CGOErrCode(value)
    }
}
