use std::collections::HashMap;
use std::io::Error as IoError;
use std::net::ToSocketAddrs;

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
use parking_lot::RwLock;
use sspi::network_client::reqwest_network_client::RequestClientFactory;
use static_init::dynamic;
use tokio::net::TcpStream as TokioTcpStream;
use tokio::sync::mpsc;
use tokio::sync::mpsc::{Receiver, Sender};

use crate::{
    handle_remote_fx_frame, CGOErrCode, CGOKeyboardEvent, CGOMousePointerEvent, CGOPointerButton,
    CGOPointerWheel,
};

/// A [cgo.Handle] passed to us by Go.
///
/// [cgo.Handle]: https://pkg.go.dev/runtime/cgo#Handle
type CgoHandle = usize;

/// Creates a single, static tokio runtime for use by all clients.
#[dynamic]
static TOKIO_RT: tokio::runtime::Runtime = tokio::runtime::Runtime::new().unwrap();

#[dynamic]
static CLIENT_HANDLES: RwLock<HashMap<CgoHandle, Sender<ClientFunction>>> =
    RwLock::new(HashMap::new());

fn insert_client_handle(cgo_handle: CgoHandle, tdp_sender: Sender<ClientFunction>) {
    CLIENT_HANDLES.write().insert(cgo_handle, tdp_sender);
}

pub fn get_client_handle(cgo_handle: CgoHandle) -> Option<Sender<ClientFunction>> {
    CLIENT_HANDLES.read().get(&cgo_handle).map(|c| (*c).clone())
}

fn remove_client_handle(cgo_handle: CgoHandle) {
    CLIENT_HANDLES.write().remove(&cgo_handle);
}

const _: () = {
    const fn assert_send_sync<T: Send + Sync>() {}
    assert_send_sync::<tokio::runtime::Runtime>();
    assert_send_sync::<RwLock<HashMap<usize, Sender<ClientFunction>>>>();
};

pub struct Client {
    cgo_handle: CgoHandle,
    rdp_stream: RdpStream,
    x224_processor: Processor,
    function_receiver: Receiver<ClientFunction>,
}

impl Client {
    pub fn run(cgo_handle: CgoHandle, params: ConnectParams) -> Result<(), ConnectError> {
        TOKIO_RT.block_on(async {
            let mut client = Self::connect(cgo_handle, params).await?;
            TOKIO_RT.spawn(async move {
                if let Err(e) = client.run_rdp_loop().await {
                    error!("rdp error: {:?}", e);
                }

                remove_client_handle(client.cgo_handle);
            });

            Ok(())
        })
    }

    async fn connect(cgo_handle: CgoHandle, params: ConnectParams) -> Result<Self, ConnectError> {
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

        info!("connection_result: {:?}", connection_result);

        let x224_processor = Processor::new(
            connection_result.static_channels,
            connection_result.user_channel_id,
            connection_result.io_channel_id,
            None,
            None,
        );

        // Create channel for sending and receiving TDP messages.
        let (function_sender, function_receiver) = mpsc::channel(100);
        insert_client_handle(cgo_handle, function_sender);

        Ok(Self {
            cgo_handle,
            rdp_stream,
            x224_processor,
            function_receiver,
        })
    }

    async fn run_rdp_loop(&mut self) -> Result<(), ConnectError> {
        loop {
            tokio::select! {
                 res = self.rdp_stream.read_pdu() => {
                    let (action, mut frame) = res?;
                    match action {
                        ironrdp_pdu::Action::X224 => {
                            let result = self.process_x224_frame(&frame).await;
                            self.process_active_stage_result(result)
                        .await
                        .map_err(|e| {
                            error!("process_stage_result {:?}", e);
                            ConnectError::Rdp(RdpError::InvalidSecurityHeader) // TODO(isaiah, przemko)
                        })?;
                        },
                        ironrdp_pdu::Action::FastPath => {
                            unsafe {
                                handle_remote_fx_frame(self.cgo_handle, frame.as_mut_ptr(), frame.len() as u32);
                            }
                        },
                    };
                 }
                 Some(data) = self.function_receiver.recv() => {
                    match data {
                        ClientFunction::WriteRdpKey(args) => {
                            self.write_rdp_key(args).await?;
                        }
                        ClientFunction::WriteRdpPointer(args) => {
                            self.write_rdp_pointer(args).await?;
                        },
                        ClientFunction::HandleResponsePdu(args) => {
                            self.handle_response_pdu(args).await?;
                        },
                    }
                }
            }
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

    async fn write_rdp_key(&mut self, key: CGOKeyboardEvent) -> Result<(), ConnectError> {
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
    ) -> Result<(), ConnectError> {
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

    async fn handle_response_pdu(&mut self, resp: Vec<u8>) -> Result<(), ConnectError> {
        let output = Ok(vec![ActiveStageOutput::ResponseFrame(resp)]);
        self.process_active_stage_result(output)
            .await
            .map_err(|e| {
                error!("process_stage_result {:?}", e);
                ConnectError::Rdp(RdpError::InvalidSecurityHeader) // TODO(isaiah, przemko)
            })?;
        Ok(())
    }
}

#[derive(Debug)]
pub enum ClientFunction {
    WriteRdpPointer(CGOMousePointerEvent),
    WriteRdpKey(CGOKeyboardEvent),
    HandleResponsePdu(Vec<u8>),
}

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
pub enum ConnectError {
    Tcp(IoError),
    Rdp(RdpError),
    SessionError(SessionError),
    //todo(isaiah): reconsider error typing
    ConnectorError(ConnectorError),
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

impl From<ConnectorError> for ConnectError {
    fn from(value: ConnectorError) -> Self {
        ConnectError::ConnectorError(value)
    }
}
