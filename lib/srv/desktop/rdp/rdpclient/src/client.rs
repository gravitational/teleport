pub mod global;

use crate::{
    handle_fastpath_pdu, handle_rdp_channel_ids, CGOErrCode, CGOKeyboardEvent,
    CGOMousePointerEvent, CGOPointerButton, CGOPointerWheel, CgoHandle,
};
use bitflags::Flags;
use ironrdp_connector::{Config, ConnectorError};
use ironrdp_pdu::input::fast_path::{FastPathInput, FastPathInputEvent, KeyboardFlags};
use ironrdp_pdu::input::mouse::PointerFlags;
use ironrdp_pdu::input::{InputEventError, MousePdu};
use ironrdp_pdu::nego::SecurityProtocol;
use ironrdp_pdu::rdp::capability_sets::MajorPlatformType;
use ironrdp_pdu::rdp::RdpError;
use ironrdp_pdu::PduParsing;
use ironrdp_session::x224::Processor as X224Processor;
use ironrdp_session::SessionError;
use ironrdp_tls::TlsStream;
use ironrdp_tokio::{Framed, TokioStream};
use sspi::network_client::reqwest_network_client::RequestClientFactory;
use std::cell::RefCell;
use std::io::Error as IoError;
use std::net::ToSocketAddrs;
use tokio::io::{split, ReadHalf, WriteHalf};
use tokio::net::TcpStream as TokioTcpStream;
use tokio::sync::mpsc;
use tokio::sync::mpsc::{error::SendError, Receiver, Sender};
use tokio::task;
use tokio::task::JoinError;

// Export this for crate level use.
pub(crate) use global::call_function_on_handle;

/// The RDP client on the Rust side of things. Each `Client`
/// corresponds with a Go `Client` specified by `cgo_handle`.
pub struct Client {
    cgo_handle: CgoHandle,
    read_stream: Option<RdpReadStream>,
    write_stream: Option<RdpWriteStream>,
    x224_processor: RefCell<X224Processor>,
    write_requester: Option<ClientHandle>,
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
    pub fn run(cgo_handle: CgoHandle, params: ConnectParams) -> ClientResult<()> {
        global::TOKIO_RT.block_on(async {
            Self::connect(cgo_handle, params)
                .await?
                .register()
                .run_loops()
                .await
        })
    }

    /// Initializes the RDP connection with the given [`ConnectParams`].
    async fn connect(cgo_handle: CgoHandle, params: ConnectParams) -> ClientResult<Self> {
        let server_addr = params.addr.clone();
        let server_socket_addr = server_addr
            .to_socket_addrs()?
            .next()
            .ok_or(ClientError::UnknownServer)?;

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

        // Upgrade the stream
        let upgraded =
            ironrdp_tokio::mark_as_upgraded(should_upgrade, &mut connector, server_public_key);

        // Frame the stream again for use by connect_finalize
        let mut rdp_stream = ironrdp_tokio::TokioFramed::new(upgraded_stream);

        let connection_result =
            ironrdp_tokio::connect_finalize(upgraded, &mut rdp_stream, connector).await?;

        debug!("connection_result: {:?}", connection_result);

        // Register the RDP channels with the browser client.
        unsafe {
            ClientResult::from(handle_rdp_channel_ids(
                cgo_handle,
                connection_result.io_channel_id,
                connection_result.user_channel_id,
            ))
        }?;

        // Take the stream back out of the framed object for splitting.
        let rdp_stream = rdp_stream.into_inner_no_leftover();
        let (read_stream, write_stream) = split(rdp_stream);
        let read_stream = ironrdp_tokio::TokioFramed::new(read_stream);
        let write_stream = ironrdp_tokio::TokioFramed::new(write_stream);

        let x224_processor = X224Processor::new(
            connection_result.static_channels,
            connection_result.user_channel_id,
            connection_result.io_channel_id,
            None,
            None,
        );

        Ok(Self {
            cgo_handle,
            read_stream: Some(read_stream),
            write_stream: Some(write_stream),
            x224_processor: RefCell::new(x224_processor),
            write_requester: None,
            function_receiver: None,
        })
    }

    /// Registers the Client with the [`global::CLIENT_HANDLES`] cache.
    ///
    /// This constitutes creating a new [`ClientHandle`]/[`FunctionReceiver`] pair,
    /// storing the [`ClientHandle`] (indexed by `self.cgo_handle`) in [`global::CLIENT_HANDLES`],
    /// and assigning the [`FunctionReceiver`] to `self.function_receiver`.
    fn register(mut self) -> Self {
        let (client_handle, function_receiver) = mpsc::channel(100);
        self.write_requester = Some(client_handle.clone());
        self.function_receiver = Some(function_receiver);
        global::CLIENT_HANDLES.insert(self.cgo_handle, client_handle);
        self
    }

    /// Spawns separate tasks for the input and output loops:
    ///
    /// 1. Read Loop: reads new messages from the RDP server and processes them.
    ///
    /// 2. Write Loop: listens on the Client's function_receiver for function calls
    ///    which it then executes.
    ///
    /// When either loop returns, the other is aborted and the result is returned.
    async fn run_loops(mut self) -> ClientResult<()> {
        let read_stream = self
            .read_stream
            .take()
            .ok_or_else(|| ClientError::InternalError)?;

        let write_stream = self
            .write_stream
            .take()
            .ok_or_else(|| ClientError::InternalError)?;

        let write_requester = self
            .write_requester
            .take()
            .ok_or_else(|| ClientError::InternalError)?;

        let write_receiver = self
            .function_receiver
            .take()
            .ok_or_else(|| ClientError::InternalError)?;

        tokio::select! {
            res = Client::run_write_loop(write_stream, write_receiver, &self.x224_processor) => res,
            res = Client::run_read_loop(self.cgo_handle, read_stream, &self.x224_processor, write_requester) => res,
        }
    }

    async fn run_read_loop(
        cgo_handle: CgoHandle,
        mut read_stream: RdpReadStream,
        x224_processor: &RefCell<X224Processor>,
        write_requester: ClientHandle,
    ) -> ClientResult<()> {
        let _marker = DropLog("read loop ends");
        loop {
            let (action, mut frame) = read_stream.read_pdu().await?;
            match action {
                // Fast-path PDU, send to the browser for processing / rendering.
                ironrdp_pdu::Action::FastPath => {
                    task::spawn_blocking(move || unsafe {
                        let err_code =
                            handle_fastpath_pdu(cgo_handle, frame.as_mut_ptr(), frame.len() as u32);
                        ClientResult::from(err_code)
                    })
                    .await??
                }
                ironrdp_pdu::Action::X224 => {
                    // X224 PDU, process it and send any immediate response frames to the write loop
                    // for writing to the RDP server.
                    // Process x224 frame.
                    let res = x224_processor.borrow_mut().process(&frame)?;
                    // Send response frames to write loop for writing to RDP server.
                    write_requester
                        .send(ClientFunction::WriteRawPdu(res))
                        .await?;
                }
            }
        }
    }

    async fn run_write_loop(
        mut write_stream: RdpWriteStream,
        mut write_receiver: FunctionReceiver,
        mut x224_processor: &RefCell<X224Processor>,
    ) -> ClientResult<()> {
        let _marker = DropLog("write loop ends");
        loop {
            x224_processor.borrow_mut();
            match write_receiver.recv().await {
                Some(write_request) => match write_request {
                    ClientFunction::WriteRdpKey(args) => {
                        Client::write_rdp_key(&mut write_stream, args).await?;
                    }
                    ClientFunction::WriteRdpPointer(args) => {
                        Client::write_rdp_pointer(&mut write_stream, args).await?;
                    }
                    ClientFunction::WriteRawPdu(args) => {
                        Client::write_raw_pdu(&mut write_stream, args).await?;
                    }
                    ClientFunction::Stop => {
                        // Stop this write loop. The read loop will then be stopped by the caller.
                        return Ok(());
                    }
                },
                None => {
                    return Ok(());
                }
            }
        }
    }

    async fn write_rdp_key(
        write_stream: &mut RdpWriteStream,
        key: CGOKeyboardEvent,
    ) -> ClientResult<()> {
        let mut fastpath_events = Vec::new();

        let mut flags: KeyboardFlags = KeyboardFlags::empty();
        if !key.down {
            flags = KeyboardFlags::RELEASE;
        }
        let event = FastPathInputEvent::KeyboardEvent(flags, key.code as u8);
        fastpath_events.push(event);

        let mut data: Vec<u8> = Vec::new();
        let input_pdu = FastPathInput(fastpath_events);
        input_pdu.to_buffer(&mut data)?;

        write_stream.write_all(&data).await?;
        Ok(())
    }

    async fn write_rdp_pointer(
        write_stream: &mut RdpWriteStream,
        pointer: CGOMousePointerEvent,
    ) -> ClientResult<()> {
        let mut fastpath_events = Vec::new();

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
        input_pdu.to_buffer(&mut data)?;

        write_stream.write_all(&data).await?;
        Ok(())
    }

    /// Writes a fully encoded PDU to the RDP server.
    async fn write_raw_pdu(write_stream: &mut RdpWriteStream, resp: Vec<u8>) -> ClientResult<()> {
        write_stream.write_all(&resp).await?;
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
    /// Corresponds to [`Client::write_raw_pdu`]
    WriteRawPdu(Vec<u8>),
    /// Aborts the client by stopping both the read and write loops.
    Stop,
}

/// `ClientHandle` is used to dispatch [`ClientFunction`]s calls
/// to a corresponding [`FunctionReceiver`] on a `Client`.
type ClientHandle = Sender<ClientFunction>;

/// Each `Client` has a `FunctionReceiver` that it listens to for
/// incoming [`ClientFunction`] calls sent via its corresponding
/// [`ClientHandle`].
pub type FunctionReceiver = Receiver<ClientFunction>;

type RdpReadStream = Framed<TokioStream<ReadHalf<TlsStream<TokioTcpStream>>>>;
type RdpWriteStream = Framed<TokioStream<WriteHalf<TlsStream<TokioTcpStream>>>>;

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
    SendError,
    JoinError(JoinError),
    InputEventError(InputEventError),
    InternalError,
    UnknownServer,
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

impl From<SessionError> for ClientError {
    fn from(value: SessionError) -> Self {
        ClientError::SessionError(value)
    }
}

impl<T> From<SendError<T>> for ClientError {
    fn from(_value: SendError<T>) -> Self {
        ClientError::SendError
    }
}

impl From<JoinError> for ClientError {
    fn from(e: JoinError) -> Self {
        ClientError::JoinError(e)
    }
}

impl From<InputEventError> for ClientError {
    fn from(e: InputEventError) -> Self {
        ClientError::InputEventError(e)
    }
}

type ClientResult<T> = Result<T, ClientError>;

impl From<CGOErrCode> for ClientResult<()> {
    fn from(value: CGOErrCode) -> Self {
        match value {
            CGOErrCode::ErrCodeSuccess => Ok(()),
            _ => Err(ClientError::from(value)),
        }
    }
}

struct DropLog(&'static str);

impl Drop for DropLog {
    fn drop(&mut self) {
        trace!("{}", self.0)
    }
}
