use std::collections::HashMap;
use std::io::Error as IoError;
use std::net::ToSocketAddrs;

use bitflags::Flags;
use bytes::BytesMut;
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

/// Creates a single, static tokio runtime for use by all clients.
#[dynamic]
pub static TOKIO_RT: tokio::runtime::Runtime = tokio::runtime::Runtime::new().unwrap();

#[dynamic]
static CHANNELS: RwLock<HashMap<usize, Sender<TdpMessage>>> = RwLock::new(HashMap::new());

#[derive(Debug)]
pub enum TdpMessage {
    Pointer(CGOMousePointerEvent),
    Key(CGOKeyboardEvent),
    PDU(Vec<u8>),
}

pub fn connect(cgo_ref: usize, params: ConnectParams) {
    let (tdp_sender, tdp_reader) = mpsc::channel(100);
    let (frame_sender, mut frame_reader) = mpsc::channel(100);
    add_channels(cgo_ref, tdp_sender);
    TOKIO_RT.spawn(async move {
        if let Err(e) = inner_connect(params, tdp_reader, frame_sender).await {
            error!("inner_connect error: {:?}", e);
        }
        CHANNELS.write().remove(&cgo_ref);
    });
    while let Some(mut frame) = frame_reader.blocking_recv() {
        unsafe { handle_remote_fx_frame(cgo_ref, frame.as_mut_ptr(), frame.len() as u32) }
    }
}

async fn inner_connect(
    params: ConnectParams,
    mut tdp_reader: Receiver<TdpMessage>,
    frame_sender: Sender<BytesMut>,
) -> Result<(), ConnectError> {
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

    let mut upgraded_framed = ironrdp_tokio::TokioFramed::new(upgraded_stream);

    let connection_result =
        ironrdp_tokio::connect_finalize(upgraded, &mut upgraded_framed, connector).await?;

    info!("connection_result: {:?}", connection_result);

    let mut x224_processor = Processor::new(
        connection_result.static_channels,
        connection_result.user_channel_id,
        connection_result.io_channel_id,
        None,
        None,
    );

    loop {
        tokio::select! {
             res = upgraded_framed.read_pdu() => {
                let (action, mut frame) = res?;
                match action {
                    ironrdp_pdu::Action::X224 => {
                        let result =
                            process_x224_frame(&mut x224_processor, &frame).await;
                        process_active_stage_result(&mut upgraded_framed, result).await.map_err(|e|{
                                error!("process_stage_result {:?}", e);
                                ConnectError::Rdp(RdpError::InvalidSecurityHeader)
                            })?;
                    },
                    ironrdp_pdu::Action::FastPath => {
                            frame_sender.send(frame).await;
                    },
                };
             }
             Some(data) = tdp_reader.recv() => {
                match data {
                    TdpMessage::Key(p) => {
                        write_rdp_key(&mut upgraded_framed, p).await;
                    }
                    TdpMessage::Pointer(p) => {
                        write_rdp_pointer(&mut upgraded_framed, p).await;
                    },
                    TdpMessage::PDU(res) => {
                        let output = Ok(vec![ActiveStageOutput::ResponseFrame(res)]);
                        process_active_stage_result(&mut upgraded_framed, output).await;
                    },
                    _ => {}
                }
            }
        }
    }
}

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

fn add_channels(cgo_ref: usize, tdp_sender: Sender<TdpMessage>) {
    CHANNELS.write().insert(cgo_ref, tdp_sender);
}

pub fn get_channels(cgo_ref: usize) -> Option<Sender<TdpMessage>> {
    CHANNELS.read().get(&cgo_ref).map(|c| (*c).clone())
}

pub async fn write_rdp_key(
    framed: &mut Framed<TokioStream<tokio_rustls::client::TlsStream<tokio::net::TcpStream>>>,
    key: CGOKeyboardEvent,
) -> Result<(), ConnectError> {
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

    framed.write_all(&data).await?;
    Ok(())
}

pub async fn write_rdp_pointer(
    framed: &mut Framed<TokioStream<tokio_rustls::client::TlsStream<tokio::net::TcpStream>>>,
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

    framed.write_all(&data).await?;
    Ok(())
}

async fn process_x224_frame(
    this: &mut Processor,
    frame: &[u8],
) -> SessionResult<Vec<ActiveStageOutput>> {
    let output = this.process(frame)?;
    let mut stage_outputs = Vec::new();
    if !output.is_empty() {
        stage_outputs.push(ActiveStageOutput::ResponseFrame(output));
    }
    Ok(stage_outputs)
}

/// Iterates through any response frames in result, sending them to the RDP server.
/// Typically returns Ok(()) if everything goes as expected and the session should continue.
async fn process_active_stage_result(
    writer: &mut Framed<TokioStream<tokio_rustls::client::TlsStream<tokio::net::TcpStream>>>,
    result: SessionResult<Vec<ActiveStageOutput>>,
) -> Result<(), CGOErrCode> {
    let outputs = result.map_err(|_| CGOErrCode::ErrCodeFailure)?;
    for output in outputs {
        match output {
            ActiveStageOutput::ResponseFrame(response) => {
                writer
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
