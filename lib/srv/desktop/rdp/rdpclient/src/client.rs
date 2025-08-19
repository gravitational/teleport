// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

pub mod global;

use crate::rdpdr::tdp;
use crate::{
    cgo_handle_fastpath_pdu, cgo_handle_rdp_connection_activated, cgo_handle_remote_copy, ssl,
    CGOErrCode, CGOKeyboardEvent, CGOMousePointerEvent, CGOPointerButton, CGOPointerWheel,
    CGOSyncKeys, CgoHandle,
};
#[cfg(feature = "fips")]
use boring::error::ErrorStack;
use bytes::BytesMut;
use ironrdp_cliprdr::{Cliprdr, CliprdrClient, CliprdrSvcMessages};
use ironrdp_connector::connection_activation::ConnectionActivationState;
use ironrdp_connector::credssp::KerberosConfig;
use ironrdp_connector::{
    Config, ConnectorError, ConnectorErrorKind, Credentials, DesktopSize, SmartCardIdentity,
};
use ironrdp_core::{encode_vec, EncodeError};
use ironrdp_core::{function, WriteBuf};
use ironrdp_displaycontrol::client::DisplayControlClient;
use ironrdp_displaycontrol::pdu::{
    DisplayControlMonitorLayout, DisplayControlPdu, MonitorLayoutEntry,
};
use ironrdp_dvc::{DrdynvcClient, DvcMessage};
use ironrdp_dvc::{DvcProcessor, DynamicVirtualChannel};
use ironrdp_pdu::input::fast_path::{
    FastPathInput, FastPathInputEvent, KeyboardFlags, SynchronizeFlags,
};
use ironrdp_pdu::input::mouse::PointerFlags;
use ironrdp_pdu::input::{InputEventError, MousePdu};
use ironrdp_pdu::nego::NegoRequestData;
use ironrdp_pdu::rdp::capability_sets::MajorPlatformType;
use ironrdp_pdu::rdp::client_info::PerformanceFlags;
use ironrdp_pdu::rdp::RdpError;
use ironrdp_pdu::PduError;
use ironrdp_pdu::PduResult;
use ironrdp_pdu::{encode_err, pdu_other_err};
use ironrdp_rdpdr::pdu::efs::ClientDeviceListAnnounce;
use ironrdp_rdpdr::pdu::RdpdrPdu;
use ironrdp_rdpdr::Rdpdr;
use ironrdp_rdpsnd::client::{NoopRdpsndBackend, Rdpsnd};
use ironrdp_session::x224::{self, DisconnectDescription, ProcessorOutput};
use ironrdp_session::SessionErrorKind::Reason;
use ironrdp_session::{reason_err, SessionError, SessionResult};
use ironrdp_svc::{SvcMessage, SvcProcessor, SvcProcessorMessages};
use ironrdp_tokio::{single_sequence_step_read, Framed, FramedWrite, TokioStream};
use log::debug;
use rand::{Rng, TryRngCore};
use std::error::Error;
use std::fmt::{Debug, Display, Formatter};
use std::io::{Error as IoError, ErrorKind as IoErrorKind};
use std::net::ToSocketAddrs;
use std::sync::{Arc, LazyLock, Mutex, MutexGuard};
use std::time::Duration;
use tokio::io::{split, ReadHalf, WriteHalf};
use tokio::net::TcpStream as TokioTcpStream;
use tokio::sync::mpsc::{channel, error::SendError, Receiver, Sender};
use tokio::task::{self, JoinError};
// Export this for crate level use.
use crate::cliprdr::{ClipboardFn, TeleportCliprdrBackend};
use crate::license::GoLicenseCache;
use crate::rdpdr::scard::SCARD_DEVICE_ID;
use crate::rdpdr::TeleportRdpdrBackend;
use crate::ssl::TlsStream;
#[cfg(feature = "fips")]
use tokio_boring::HandshakeError;
use url::Url;

const RDP_CONNECT_TIMEOUT: Duration = Duration::from_secs(5);

/// The "Microsoft::Windows::RDS::DisplayControl" DVC is opened
/// by the server. Until it does so, we withhold the latest screen
/// resize, and only send it once we're notified that the DVC is open.
struct PendingResize {
    pending_resize: Option<(u32, u32)>,
}

/// The RDP client on the Rust side of things. Each `Client`
/// corresponds with a Go `Client` specified by `cgo_handle`.
pub struct Client {
    cgo_handle: CgoHandle,
    client_handle: ClientHandle,
    read_stream: Option<RdpReadStream>,
    write_stream: Option<RdpWriteStream>,
    function_receiver: Option<FunctionReceiver>,
    x224_processor: Arc<Mutex<x224::Processor>>,
    pending_resize: Arc<Mutex<PendingResize>>,
}

/// A global, static tokio runtime for use by `Client`.
static TOKIO_RT: LazyLock<tokio::runtime::Runtime> =
    LazyLock::new(|| tokio::runtime::Runtime::new().unwrap());

impl Client {
    /// Connects a new client to the RDP server specified by `params` and starts the session.
    ///
    /// After creating the connection, this function registers the newly made Client with
    /// the [`global::ClientHandles`] map, and creates a task for reading frames from the  RDP
    /// server and sending them back to Go, and receiving function calls via [`ClientHandle`]
    /// and executing them.
    ///
    /// This function hangs until the RDP session ends or a [`ClientFunction::Stop`] is dispatched
    /// (see [`ClientHandle::stop`]).
    pub fn run(
        cgo_handle: CgoHandle,
        params: ConnectParams,
    ) -> ClientResult<Option<DisconnectDescription>> {
        TOKIO_RT.block_on(async {
            Self::connect(cgo_handle, params)
                .await?
                .register()?
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
            .ok_or(ClientError::UnknownAddress)?;

        let stream = match tokio::time::timeout(
            RDP_CONNECT_TIMEOUT,
            TokioTcpStream::connect(&server_socket_addr),
        )
        .await
        {
            Ok(stream) => stream?,
            Err(_) => return Err(ClientError::Tcp(IoError::from(IoErrorKind::TimedOut))),
        };

        // Create a framed stream for use by connect_begin
        let mut framed = ironrdp_tokio::TokioFramed::new(stream);

        // Generate a random 8-digit PIN for our smartcard.
        let pin = format!(
            "{:08}",
            rand::rngs::OsRng
                .unwrap_err()
                .random_range(0i32..=99999999i32)
        );

        let connector_config = create_config(&params, pin.clone(), cgo_handle);

        // Create a channel for sending/receiving function calls to/from the Client.
        let (client_handle, function_receiver) = ClientHandle::new(100);
        let function_receiver = Some(function_receiver);

        let mut rdpdr = Rdpdr::new(
            Box::new(TeleportRdpdrBackend::new(
                client_handle.clone(),
                params.cert_der,
                params.key_der,
                pin,
                cgo_handle,
                params.allow_directory_sharing,
            )),
            "Teleport".to_string(), // directories will show up as "<directory> on Teleport"
        )
        .with_smartcard(SCARD_DEVICE_ID);

        if params.allow_directory_sharing {
            debug!("creating rdpdr client with directory sharing enabled");
            rdpdr = rdpdr.with_drives(None);
        } else {
            debug!("creating rdpdr client with directory sharing disabled")
        }

        let pending_resize = Arc::new(Mutex::new(PendingResize {
            pending_resize: None,
        }));

        let pending_resize_clone = pending_resize.clone();
        let display_control = DisplayControlClient::new(move |_| {
            Self::on_display_ctl_capabilities_received(&pending_resize_clone)
        });
        let drdynvc_client = DrdynvcClient::new().with_dynamic_channel(display_control);

        let mut connector = ironrdp_connector::ClientConnector::new(connector_config.clone())
            .with_server_addr(server_socket_addr)
            .with_static_channel(drdynvc_client) // require for resizing
            .with_static_channel(Rdpsnd::new(Box::new(NoopRdpsndBackend {}))) // required for rdpdr to work
            .with_static_channel(rdpdr); // required for smart card + directory sharing

        if params.allow_clipboard {
            connector = connector.with_static_channel(Cliprdr::new(Box::new(
                TeleportCliprdrBackend::new(client_handle.clone()),
            )));
        }

        let should_upgrade = ironrdp_tokio::connect_begin(&mut framed, &mut connector).await?;

        // Take the stream back out of the framed object for upgrading
        let initial_stream = framed.into_inner_no_leftover();
        let (upgraded_stream, server_public_key) =
            ssl::upgrade(initial_stream, &server_socket_addr.ip().to_string()).await?;

        // Upgrade the stream
        let upgraded = ironrdp_tokio::mark_as_upgraded(should_upgrade, &mut connector);

        // Frame the stream again for use by connect_finalize
        let mut rdp_stream = ironrdp_tokio::TokioFramed::new(upgraded_stream);

        let mut network_client = crate::network_client::NetworkClient::new();
        let kerberos_config = params
            .kdc_addr
            .map(|kdc_addr| Url::parse(&format!("tcp://{}", kdc_addr)))
            .transpose()
            .map_err(ClientError::UrlError)?
            .map(|kdc_url| KerberosConfig {
                kdc_proxy_url: Some(kdc_url),
                hostname: params.computer_name.clone(),
            });
        let connection_result = ironrdp_tokio::connect_finalize(
            upgraded,
            &mut rdp_stream,
            connector,
            params.computer_name.unwrap_or(server_addr).into(),
            server_public_key,
            Some(&mut network_client),
            kerberos_config,
        )
        .await?;

        // Register the RDP channels with the browser client.
        Self::send_connection_activated(
            cgo_handle,
            connection_result.io_channel_id,
            connection_result.user_channel_id,
            connection_result.desktop_size,
        )
        .await?;

        // Take the stream back out of the framed object for splitting.
        let rdp_stream = rdp_stream.into_inner_no_leftover();
        let (read_stream, write_stream) = split(rdp_stream);
        let read_stream = Some(ironrdp_tokio::TokioFramed::new(read_stream));
        let write_stream = Some(ironrdp_tokio::TokioFramed::new(write_stream));

        let x224_processor = Arc::new(Mutex::new(x224::Processor::new(
            connection_result.static_channels,
            connection_result.user_channel_id,
            connection_result.io_channel_id,
            connection_result.connection_activation,
        )));

        Ok(Self {
            cgo_handle,
            client_handle,
            read_stream,
            write_stream,
            function_receiver,
            x224_processor,
            pending_resize,
        })
    }

    /// Registers the Client with the [`global::CLIENT_HANDLES`] cache.
    ///
    /// This constitutes storing the [`ClientHandle`] (indexed by `self.cgo_handle`)
    /// in [`global::CLIENT_HANDLES`].
    fn register(self) -> ClientResult<Self> {
        global::CLIENT_HANDLES.insert(self.cgo_handle, self.client_handle.clone());

        Ok(self)
    }

    /// Spawns separate tasks for the input and output loops:
    ///
    /// 1. Read Loop: reads new messages from the RDP server and processes them.
    ///
    /// 2. Write Loop: listens on the Client's function_receiver for function calls
    ///    which it then executes.
    ///
    /// When either loop returns, the other is aborted and the result is returned.
    async fn run_loops(mut self) -> ClientResult<Option<DisconnectDescription>> {
        let read_stream = self
            .read_stream
            .take()
            .ok_or_else(|| ClientError::InternalError("read_stream failed".to_string()))?;

        let write_stream = self
            .write_stream
            .take()
            .ok_or_else(|| ClientError::InternalError("write_stream failed".to_string()))?;

        let function_receiver = self
            .function_receiver
            .take()
            .ok_or_else(|| ClientError::InternalError("function_receiver failed".to_string()))?;

        let read_loop_handle = Client::run_read_loop(
            self.cgo_handle,
            read_stream,
            self.x224_processor.clone(),
            self.client_handle.clone(),
        );

        let write_loop_handle = Client::run_write_loop(
            self.cgo_handle,
            write_stream,
            function_receiver,
            self.x224_processor.clone(),
            self.pending_resize.clone(),
        );

        // Wait for either loop to finish. When one does, cancel the other and return the result.
        tokio::select! {
            res = read_loop_handle => {
                res
            },
            res = write_loop_handle => {
                res.map(|_|None)
            }
        }
    }

    async fn run_read_loop(
        cgo_handle: CgoHandle,
        mut read_stream: RdpReadStream,
        x224_processor: Arc<Mutex<x224::Processor>>,
        write_requester: ClientHandle,
    ) -> ClientResult<Option<DisconnectDescription>> {
        loop {
            let (action, mut frame) = read_stream.read_pdu().await?;
            match action {
                // Fast-path PDU, send to the browser for processing / rendering.
                ironrdp_pdu::Action::FastPath => {
                    task::spawn_blocking(move || unsafe {
                        let err_code = cgo_handle_fastpath_pdu(
                            cgo_handle,
                            frame.as_mut_ptr(),
                            frame.len() as u32,
                        );
                        ClientResult::from(err_code)
                    })
                    .await??
                }
                ironrdp_pdu::Action::X224 => {
                    // X224 PDU, process it and send any immediate response frames to the write loop
                    // for writing to the RDP server.
                    for output in Client::x224_process(x224_processor.clone(), frame).await? {
                        match output {
                            ProcessorOutput::ResponseFrame(frame) => {
                                // Send response frames to write loop for writing to RDP server.
                                write_requester.write_raw_pdu_async(frame).await?;
                            }
                            ProcessorOutput::Disconnect(reason) => {
                                return Ok(Some(reason));
                            }
                            ProcessorOutput::DeactivateAll(mut sequence) => {
                                // Execute the Deactivation-Reactivation Sequence:
                                // https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpbcgr/dfc234ce-481a-4674-9a5d-2a7bafb14432
                                debug!("Received Server Deactivate All PDU, executing Deactivation-Reactivation Sequence");
                                let mut buf = WriteBuf::new();
                                loop {
                                    let written = single_sequence_step_read(
                                        &mut read_stream,
                                        sequence.as_mut(),
                                        &mut buf,
                                        None,
                                    )
                                    .await?;

                                    if written.size().is_some() {
                                        write_requester
                                            .write_raw_pdu_async(buf.filled().to_vec())
                                            .await?;
                                    }

                                    if let ConnectionActivationState::Finalized {
                                        io_channel_id,
                                        user_channel_id,
                                        desktop_size,
                                        ..
                                    } = sequence.state
                                    {
                                        // Upon completing the activation sequence, register the io/user channels
                                        // and desktop size with the client, just like we do upon receiving the
                                        // connection result in [`Self::connect`].
                                        Self::send_connection_activated(
                                            cgo_handle,
                                            io_channel_id,
                                            user_channel_id,
                                            desktop_size,
                                        )
                                        .await?;
                                        break;
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }
    }

    async fn run_write_loop(
        cgo_handle: CgoHandle,
        mut write_stream: RdpWriteStream,
        mut write_receiver: FunctionReceiver,
        x224_processor: Arc<Mutex<x224::Processor>>,
        pending_resize: Arc<Mutex<PendingResize>>,
    ) -> ClientResult<()> {
        while let Some(write_request) = write_receiver.recv().await {
            match write_request {
                ClientFunction::WriteRdpKey(args) => {
                    Client::write_rdp_key(&mut write_stream, args).await?;
                }
                ClientFunction::WriteRdpPointer(args) => {
                    Client::write_rdp_pointer(&mut write_stream, args).await?;
                }
                ClientFunction::WriteRdpSyncKeys(args) => {
                    Client::write_rdp_sync_keys(&mut write_stream, args).await?;
                }
                ClientFunction::WriteRawPdu(args) => {
                    Client::write_raw_pdu(&mut write_stream, args).await?;
                }
                ClientFunction::WriteRdpdr(args) => {
                    Client::write_rdpdr(&mut write_stream, x224_processor.clone(), args).await?;
                }
                ClientFunction::WriteScreenResize(width, height) => {
                    Client::handle_screen_resize(
                        width,
                        height,
                        x224_processor.clone(),
                        &mut write_stream,
                        pending_resize.clone(),
                    )
                    .await?;
                }
                ClientFunction::HandleTdpSdAnnounce(sda) => {
                    Client::handle_tdp_sd_announce(&mut write_stream, x224_processor.clone(), sda)
                        .await?;
                }
                ClientFunction::HandleTdpSdInfoResponse(res) => {
                    Client::handle_tdp_sd_info_response(x224_processor.clone(), res).await?;
                }
                ClientFunction::HandleTdpSdCreateResponse(res) => {
                    Client::handle_tdp_sd_create_response(x224_processor.clone(), res).await?;
                }
                ClientFunction::HandleTdpSdDeleteResponse(res) => {
                    Client::handle_tdp_sd_delete_response(x224_processor.clone(), res).await?;
                }
                ClientFunction::HandleTdpSdListResponse(res) => {
                    Client::handle_tdp_sd_list_response(x224_processor.clone(), res).await?;
                }
                ClientFunction::HandleTdpSdReadResponse(res) => {
                    Client::handle_tdp_sd_read_response(x224_processor.clone(), res).await?;
                }
                ClientFunction::HandleTdpSdWriteResponse(res) => {
                    Client::handle_tdp_sd_write_response(x224_processor.clone(), res).await?;
                }
                ClientFunction::HandleTdpSdMoveResponse(res) => {
                    Client::handle_tdp_sd_move_response(x224_processor.clone(), res).await?;
                }
                ClientFunction::HandleTdpSdTruncateResponse(res) => {
                    Client::handle_tdp_sd_truncate_response(x224_processor.clone(), res).await?;
                }
                ClientFunction::WriteCliprdr(f) => {
                    Client::write_cliprdr(x224_processor.clone(), &mut write_stream, f).await?;
                }
                ClientFunction::HandleRemoteCopy(data) => {
                    Client::handle_remote_copy(cgo_handle, data).await?;
                }
                ClientFunction::UpdateClipboard(data) => {
                    Client::update_clipboard(x224_processor.clone(), data).await?;
                }
                ClientFunction::Stop => {
                    // Stop this write loop. The read loop will then be stopped by the caller.
                    return Ok(());
                }
            }
        }
        Ok(())
    }

    fn on_display_ctl_capabilities_received(
        pending_resize: &Arc<Mutex<PendingResize>>,
    ) -> PduResult<Vec<DvcMessage>> {
        debug!("DisplayControlClient channel opened");
        // We've been notified that the DisplayControl dvc channel has been opened:
        let mut pending_resize =
            Self::resize_manager_lock(pending_resize).map_err(ClientError::from)?;
        let pending_resize = pending_resize.pending_resize.take();
        if let Some((width, height)) = pending_resize {
            // If there was a resize pending, perform it now.
            debug!(
                "Pending resize for size [{:?}x{:?}] found, sending now",
                width, height
            );
            let pdu: DisplayControlPdu = DisplayControlMonitorLayout::new_single_primary_monitor(
                width,
                height,
                None,
                Some((width, height)),
            )
            .map_err(|e| encode_err!(e))?
            .into();
            return Ok(vec![Box::new(pdu)]);
        }

        // No resize was pending, nothing to do.
        Ok(vec![])
    }

    async fn send_connection_activated(
        cgo_handle: CgoHandle,
        io_channel_id: u16,
        user_channel_id: u16,
        desktop_size: DesktopSize,
    ) -> ClientResult<()> {
        task::spawn_blocking(move || unsafe {
            ClientResult::from(cgo_handle_rdp_connection_activated(
                cgo_handle,
                io_channel_id,
                user_channel_id,
                desktop_size.width,
                desktop_size.height,
            ))
        })
        .await?
    }

    async fn update_clipboard(
        x224_processor: Arc<Mutex<x224::Processor>>,
        data: String,
    ) -> ClientResult<()> {
        task::spawn_blocking(move || {
            let mut x224_processor = Self::x224_lock(&x224_processor)?;
            let cliprdr = Self::cliprdr_backend(&mut x224_processor)?;
            cliprdr.set_clipboard_data(data.clone());
            Ok(())
        })
        .await?
    }

    async fn handle_remote_copy(cgo_handle: CgoHandle, mut data: Vec<u8>) -> ClientResult<()> {
        let code = task::spawn_blocking(move || unsafe {
            cgo_handle_remote_copy(cgo_handle, data.as_mut_ptr(), data.len() as u32)
        })
        .await?;
        ClientResult::from(code)
    }

    async fn write_cliprdr(
        x224_processor: Arc<Mutex<x224::Processor>>,
        write_stream: &mut RdpWriteStream,
        fun: Box<dyn ClipboardFn>,
    ) -> ClientResult<()> {
        let processor = x224_processor.clone();
        let messages: ClientResult<CliprdrSvcMessages<ironrdp_cliprdr::Client>> =
            task::spawn_blocking(move || {
                let mut x224_processor = Self::x224_lock(&processor)?;
                let cliprdr = Self::get_svc_processor::<CliprdrClient>(&mut x224_processor)?;
                Ok(fun.call(cliprdr)?)
            })
            .await?;
        let encoded = Client::x224_process_svc_messages(x224_processor, messages?).await?;
        write_stream.write_all(&encoded).await?;
        Ok(())
    }

    async fn write_rdp_key(
        write_stream: &mut RdpWriteStream,
        key: CGOKeyboardEvent,
    ) -> ClientResult<()> {
        let mut flags: KeyboardFlags = KeyboardFlags::empty();
        if !key.down {
            flags = KeyboardFlags::RELEASE;
        }
        let extended = key.code & 0xE000 == 0xE000;
        if extended {
            flags |= KeyboardFlags::EXTENDED;
        }

        let event = FastPathInputEvent::KeyboardEvent(flags, key.code as u8);

        Self::write_fast_path_input_event(write_stream, event).await
    }

    async fn write_rdp_pointer(
        write_stream: &mut RdpWriteStream,
        pointer: CGOMousePointerEvent,
    ) -> ClientResult<()> {
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

        Self::write_fast_path_input_event(write_stream, event).await
    }

    async fn write_rdp_sync_keys(
        write_stream: &mut RdpWriteStream,
        keys: CGOSyncKeys,
    ) -> ClientResult<()> {
        let mut flags = SynchronizeFlags::empty();
        if keys.scroll_lock_down {
            flags |= SynchronizeFlags::SCROLL_LOCK;
        }
        if keys.num_lock_down {
            flags |= SynchronizeFlags::NUM_LOCK;
        }
        if keys.caps_lock_down {
            flags |= SynchronizeFlags::CAPS_LOCK;
        }
        if keys.kana_lock_down {
            flags |= SynchronizeFlags::KANA_LOCK;
        }

        let event = FastPathInputEvent::SyncEvent(flags);

        Self::write_fast_path_input_event(write_stream, event).await
    }

    /// Helper function for writing a single [`FastPathInputEvent`] to the RDP server.
    async fn write_fast_path_input_event(
        write_stream: &mut RdpWriteStream,
        event: FastPathInputEvent,
    ) -> ClientResult<()> {
        write_stream
            .write_all(&encode_vec(&FastPathInput(vec![event]))?)
            .await?;
        Ok(())
    }

    /// Writes a fully encoded PDU to the RDP server.
    async fn write_raw_pdu(write_stream: &mut RdpWriteStream, resp: Vec<u8>) -> ClientResult<()> {
        write_stream.write_all(&resp).await?;
        Ok(())
    }

    async fn write_rdpdr(
        write_stream: &mut RdpWriteStream,
        x224_processor: Arc<Mutex<x224::Processor>>,
        pdu: RdpdrPdu,
    ) -> ClientResult<()> {
        debug!("sending rdp: {:?}", pdu);
        // Process the RDPDR PDU.
        let encoded = Client::x224_process_svc_messages(
            x224_processor,
            SvcProcessorMessages::<Rdpdr>::new(vec![SvcMessage::from(pdu)]),
        )
        .await?;

        // Write the RDPDR PDU to the RDP server.
        write_stream.write_all(&encoded).await?;
        Ok(())
    }

    async fn handle_screen_resize(
        width: u32,
        height: u32,
        x224_processor: Arc<Mutex<x224::Processor>>,
        write_stream: &mut RdpWriteStream,
        pending_resize: Arc<Mutex<PendingResize>>,
    ) -> ClientResult<()> {
        // Adjust the screen size to the nearest supported resolution (per the RDP spec).
        let init_width = width;
        let init_height = height;
        debug!(
            "Received screen resize [{:?}x{:?}]",
            init_width, init_height
        );
        let (width, height) = MonitorLayoutEntry::adjust_display_size(init_width, init_height);
        if width != init_width || height != init_height {
            debug!("Adjusted screen resize to [{:?}x{:?}]", width, height);
        }

        // Determine whether to withhold the resize or perform it immediately.
        let action = {
            let x224_processor = Self::x224_lock(&x224_processor)?;
            let dvc = x224_processor.get_dvc::<DisplayControlClient>().ok_or(
                ClientError::InternalError("DisplayControlClient not found".to_string()),
            )?;

            if dvc.is_open() {
                // Resize channel is open, perform the resize immediately.
                Some((width, height))
            } else {
                // The client requested a resize but the DisplayControl channel has not been opened yet.
                // Sending the resize now would cause an RDP error and end the session; instead we withhold
                // it until the DisplayControl channel is ready.
                debug!("DisplayControl channel not ready, withholding resize");
                let mut pending_resize = Self::resize_manager_lock(&pending_resize)?;
                pending_resize.pending_resize = Some((width, height));
                None // No immediate action required.
            }
        }; // Drop the x224 lock here to avoid holding it over the await below.

        if let Some((width, height)) = action {
            return Client::write_screen_resize(
                write_stream,
                x224_processor.clone(),
                width,
                height,
            )
            .await;
        }

        Ok(())
    }

    /// Sends a screen resize to the RDP server.
    async fn write_screen_resize(
        write_stream: &mut RdpWriteStream,
        x224_processor: Arc<Mutex<x224::Processor>>,
        width: u32,
        height: u32,
    ) -> ClientResult<()> {
        let cloned = x224_processor.clone();
        let messages = task::spawn_blocking(move || {
            let x224_processor = Self::x224_lock(&cloned)?;
            let dvc = Self::get_dvc::<DisplayControlClient>(&x224_processor)?;
            let channel_id = dvc.channel_id().ok_or(ClientError::InternalError(
                "DisplayControlClient channel_id not found".to_string(),
            ))?;
            let disp_ctl_cli = dvc
                .channel_processor_downcast_ref::<DisplayControlClient>()
                .ok_or(ClientError::InternalError(
                    "DisplayControlClient not found".to_string(),
                ))?;

            Ok::<_, ClientError>(disp_ctl_cli.encode_single_primary_monitor(
                channel_id,
                width,
                height,
                None,
                Some((width, height)),
            ))
        })
        .await???;

        let encoded = Client::x224_process_svc_messages(
            x224_processor,
            SvcProcessorMessages::<DrdynvcClient>::new(messages),
        )
        .await?;
        debug!("Writing resize to [{:?}x{:?}]", width, height);
        write_stream.write_all(&encoded).await?;

        Ok(())
    }

    async fn handle_tdp_sd_announce(
        write_stream: &mut RdpWriteStream,
        x224_processor: Arc<Mutex<x224::Processor>>,
        sda: tdp::SharedDirectoryAnnounce,
    ) -> ClientResult<()> {
        debug!("received tdp: {:?}", sda);
        let pdu = Self::add_drive(x224_processor.clone(), sda).await?;
        Self::write_rdpdr(
            write_stream,
            x224_processor,
            RdpdrPdu::ClientDeviceListAnnounce(pdu),
        )
        .await?;
        Ok(())
    }

    async fn handle_tdp_sd_info_response(
        x224_processor: Arc<Mutex<x224::Processor>>,
        res: tdp::SharedDirectoryInfoResponse,
    ) -> ClientResult<()> {
        task::spawn_blocking(move || {
            debug!("received tdp: {:?}", res);
            let mut x224_processor = Self::x224_lock(&x224_processor)?;
            let rdpdr: &mut TeleportRdpdrBackend = Self::rdpdr_backend(&mut x224_processor)?;
            rdpdr.handle_tdp_sd_info_response(res)?;
            Ok(())
        })
        .await?
    }

    async fn handle_tdp_sd_create_response(
        x224_processor: Arc<Mutex<x224::Processor>>,
        res: tdp::SharedDirectoryCreateResponse,
    ) -> ClientResult<()> {
        task::spawn_blocking(move || {
            debug!("received tdp: {:?}", res);
            let mut x224_processor = Self::x224_lock(&x224_processor)?;
            let rdpdr = Self::rdpdr_backend(&mut x224_processor)?;
            rdpdr.handle_tdp_sd_create_response(res)?;
            Ok(())
        })
        .await?
    }

    async fn handle_tdp_sd_delete_response(
        x224_processor: Arc<Mutex<x224::Processor>>,
        res: tdp::SharedDirectoryDeleteResponse,
    ) -> ClientResult<()> {
        task::spawn_blocking(move || {
            debug!("received tdp: {:?}", res);
            let mut x224_processor = Self::x224_lock(&x224_processor)?;
            let rdpdr = Self::rdpdr_backend(&mut x224_processor)?;
            rdpdr.handle_tdp_sd_delete_response(res)?;
            Ok(())
        })
        .await?
    }

    async fn handle_tdp_sd_list_response(
        x224_processor: Arc<Mutex<x224::Processor>>,
        res: tdp::SharedDirectoryListResponse,
    ) -> ClientResult<()> {
        task::spawn_blocking(move || {
            debug!("received tdp: {:?}", res);
            let mut x224_processor = Self::x224_lock(&x224_processor)?;
            let rdpdr: &mut TeleportRdpdrBackend = Self::rdpdr_backend(&mut x224_processor)?;
            rdpdr.handle_tdp_sd_list_response(res)?;
            Ok(())
        })
        .await?
    }

    async fn handle_tdp_sd_read_response(
        x224_processor: Arc<Mutex<x224::Processor>>,
        res: tdp::SharedDirectoryReadResponse,
    ) -> ClientResult<()> {
        task::spawn_blocking(move || {
            debug!("received tdp: {:?}", res);
            let mut x224_processor = Self::x224_lock(&x224_processor)?;
            let rdpdr = Self::rdpdr_backend(&mut x224_processor)?;
            rdpdr.handle_tdp_sd_read_response(res)?;
            Ok(())
        })
        .await?
    }

    async fn handle_tdp_sd_write_response(
        x224_processor: Arc<Mutex<x224::Processor>>,
        res: tdp::SharedDirectoryWriteResponse,
    ) -> ClientResult<()> {
        task::spawn_blocking(move || {
            debug!("received tdp: {:?}", res);
            let mut x224_processor = Self::x224_lock(&x224_processor)?;
            let rdpdr = Self::rdpdr_backend(&mut x224_processor)?;
            rdpdr.handle_tdp_sd_write_response(res)?;
            Ok(())
        })
        .await?
    }

    async fn handle_tdp_sd_move_response(
        x224_processor: Arc<Mutex<x224::Processor>>,
        res: tdp::SharedDirectoryMoveResponse,
    ) -> ClientResult<()> {
        task::spawn_blocking(move || {
            debug!("received tdp: {:?}", res);
            let mut x224_processor = Self::x224_lock(&x224_processor)?;
            let rdpdr: &mut TeleportRdpdrBackend = Self::rdpdr_backend(&mut x224_processor)?;
            rdpdr.handle_tdp_sd_move_response(res)?;
            Ok(())
        })
        .await?
    }

    async fn handle_tdp_sd_truncate_response(
        x224_processor: Arc<Mutex<x224::Processor>>,
        res: tdp::SharedDirectoryTruncateResponse,
    ) -> ClientResult<()> {
        task::spawn_blocking(move || {
            debug!("received tdp: {:?}", res);
            let mut x224_processor = Self::x224_lock(&x224_processor)?;
            let rdpdr = Self::rdpdr_backend(&mut x224_processor)?;
            rdpdr.handle_tdp_sd_truncate_response(res)?;
            Ok(())
        })
        .await?
    }

    async fn add_drive(
        x224_processor: Arc<Mutex<x224::Processor>>,
        sda: tdp::SharedDirectoryAnnounce,
    ) -> ClientResult<ClientDeviceListAnnounce> {
        task::spawn_blocking(move || {
            let mut x224_processor = Self::x224_lock(&x224_processor)?;
            let rdpdr = Self::get_svc_processor_mut::<Rdpdr>(&mut x224_processor)?;
            let pdu = rdpdr.add_drive(sda.directory_id, sda.name);
            Ok(pdu)
        })
        .await?
    }

    /// Processes an x224 frame on a blocking thread.
    ///
    /// We use a blocking task here so we don't block the tokio runtime
    /// while waiting for the `x224_processor` lock, or while processing the frame.
    /// This function ensures `x224_processor` is locked only for the necessary duration
    /// of the function call.
    async fn x224_process(
        x224_processor: Arc<Mutex<x224::Processor>>,
        frame: BytesMut,
    ) -> SessionResult<Vec<ProcessorOutput>> {
        task::spawn_blocking(move || Self::x224_lock(&x224_processor)?.process(&frame))
            .await
            .map_err(|err| reason_err!(function!(), "JoinError: {:?}", err))?
    }

    /// Processes some [`SvcProcessorMessages`] on a blocking thread.
    ///
    /// We use a blocking task here so we don't block the tokio runtime
    /// while waiting for the `x224_processor` lock, or while processing the frame.
    /// This function ensures `x224_processor` is locked only for the necessary duration
    /// of the function call.
    async fn x224_process_svc_messages<C: SvcProcessor + 'static>(
        x224_processor: Arc<Mutex<x224::Processor>>,
        messages: SvcProcessorMessages<C>,
    ) -> SessionResult<Vec<u8>> {
        task::spawn_blocking(move || {
            Self::x224_lock(&x224_processor)?.process_svc_processor_messages(messages)
        })
        .await
        .map_err(|err| reason_err!(function!(), "JoinError: {:?}", err))?
    }

    fn x224_lock(
        x224_processor: &Arc<Mutex<x224::Processor>>,
    ) -> Result<MutexGuard<x224::Processor>, SessionError> {
        x224_processor
            .lock()
            .map_err(|err| reason_err!(function!(), "PoisonError: {:?}", err))
    }

    fn resize_manager_lock(
        pending_resize: &Arc<Mutex<PendingResize>>,
    ) -> Result<MutexGuard<PendingResize>, SessionError> {
        pending_resize
            .lock()
            .map_err(|err| reason_err!(function!(), "PoisonError: {:?}", err))
    }

    /// Returns an immutable reference to the [`SvcProcessor`] of type `S`.
    ///
    /// # Example
    ///
    /// ```
    /// let mut x224_processor = Self::x224_lock(&x224_processor)?;
    /// let cliprdr = Self::get_svc_processor::<Cliprdr>(&mut x224_processor)?;
    /// // Now we can call methods on the Cliprdr processor.
    /// ```
    fn get_svc_processor<'a, S>(
        x224_processor: &'a mut MutexGuard<'_, x224::Processor>,
    ) -> Result<&'a S, ClientError>
    where
        S: SvcProcessor + 'static,
    {
        x224_processor
            .get_svc_processor::<S>()
            .ok_or(ClientError::InternalError(format!(
                "get_svc_processor::<{}>() returned None",
                std::any::type_name::<S>(),
            )))
    }

    /// Returns a mutable reference to the [`SvcProcessor`] of type `S`.
    ///
    /// # Example
    ///
    /// ```
    /// let mut x224_processor = Self::x224_lock(&x224_processor)?;
    /// let cliprdr = Self::get_svc_processor_mut::<Cliprdr>(&mut x224_processor)?;
    /// // Now we can call mutating methods on the Cliprdr processor.
    /// ```
    fn get_svc_processor_mut<'a, S>(
        x224_processor: &'a mut MutexGuard<'_, x224::Processor>,
    ) -> Result<&'a mut S, ClientError>
    where
        S: SvcProcessor + 'static,
    {
        x224_processor
            .get_svc_processor_mut::<S>()
            .ok_or(ClientError::InternalError(format!(
                "get_svc_processor_mut::<{}>() returned None",
                std::any::type_name::<S>(),
            )))
    }

    fn get_dvc<'a, S>(
        x224_processor: &'a MutexGuard<'_, x224::Processor>,
    ) -> Result<&'a DynamicVirtualChannel, ClientError>
    where
        S: DvcProcessor + 'static,
    {
        x224_processor
            .get_dvc::<S>()
            .ok_or(ClientError::InternalError(format!(
                "get_dvc::<{}>() returned None",
                std::any::type_name::<S>(),
            )))
    }

    /// Returns a mutable reference to the [`TeleportCliprdrBackend`] of the [`Cliprdr`] processor.
    fn cliprdr_backend(
        x224_processor: &mut x224::Processor,
    ) -> ClientResult<&mut TeleportCliprdrBackend> {
        x224_processor
            .get_svc_processor_mut::<CliprdrClient>()
            .and_then(|c| c.downcast_backend_mut::<TeleportCliprdrBackend>())
            .ok_or(ClientError::InternalError(
                "cliprdr_backend returned None".to_string(),
            ))
    }

    /// Returns a mutable reference to the [`TeleportRdpdrBackend`] of the [`Rdpdr`] processor.
    fn rdpdr_backend(
        x224_processor: &mut x224::Processor,
    ) -> ClientResult<&mut TeleportRdpdrBackend> {
        x224_processor
            .get_svc_processor_mut::<Rdpdr>()
            .and_then(|c| c.downcast_backend_mut::<TeleportRdpdrBackend>())
            .ok_or(ClientError::InternalError(
                "rdpdr_backend returned None".to_string(),
            ))
    }
}

impl Drop for Client {
    fn drop(&mut self) {
        global::CLIENT_HANDLES.remove(self.cgo_handle)
    }
}

/// [`ClientFunction`] is an enum representing the different functions that can be called on a client.
/// Each variant corresponds to a different function, and carries the necessary arguments for that function.
///
/// This enum is used by [`ClientHandle`]'s methods to dispatch function calls to the corresponding [`Client`] instance.
#[derive(Debug)]
enum ClientFunction {
    /// Corresponds to [`Client::write_rdp_pointer`]
    WriteRdpPointer(CGOMousePointerEvent),
    /// Corresponds to [`Client::write_rdp_key`]
    WriteRdpKey(CGOKeyboardEvent),
    /// Corresponds to [`Client::write_rdp_sync_keys`]
    WriteRdpSyncKeys(CGOSyncKeys),
    /// Corresponds to [`Client::write_raw_pdu`]
    WriteRawPdu(Vec<u8>),
    /// Corresponds to [`Client::write_rdpdr`]
    WriteRdpdr(RdpdrPdu),
    /// Corresponds to [`Client::write_screen_resize`]
    WriteScreenResize(u32, u32),
    /// Corresponds to [`Client::handle_tdp_sd_announce`]
    HandleTdpSdAnnounce(tdp::SharedDirectoryAnnounce),
    /// Corresponds to [`Client::handle_tdp_sd_info_response`]
    HandleTdpSdInfoResponse(tdp::SharedDirectoryInfoResponse),
    /// Corresponds to [`Client::handle_tdp_sd_create_response`]
    HandleTdpSdCreateResponse(tdp::SharedDirectoryCreateResponse),
    /// Corresponds to [`Client::handle_tdp_sd_delete_response`]
    HandleTdpSdDeleteResponse(tdp::SharedDirectoryDeleteResponse),
    /// Corresponds to [`Client::handle_tdp_sd_list_response`]
    HandleTdpSdListResponse(tdp::SharedDirectoryListResponse),
    /// Corresponds to [`Client::handle_tdp_sd_read_response`]
    HandleTdpSdReadResponse(tdp::SharedDirectoryReadResponse),
    /// Corresponds to [`Client::handle_tdp_sd_write_response`]
    HandleTdpSdWriteResponse(tdp::SharedDirectoryWriteResponse),
    /// Corresponds to [`Client::handle_tdp_sd_move_response`]
    HandleTdpSdMoveResponse(tdp::SharedDirectoryMoveResponse),
    /// Corresponds to [`Client::handle_tdp_sd_truncate_response`]
    HandleTdpSdTruncateResponse(tdp::SharedDirectoryTruncateResponse),
    /// Corresponds to [`Client::write_cliprdr`]
    WriteCliprdr(Box<dyn ClipboardFn>),
    /// Corresponds to [`Client::update_clipboard`]
    UpdateClipboard(String),
    /// Corresponds to [`Client::handle_remote_copy`]
    HandleRemoteCopy(Vec<u8>),
    /// Aborts the client by stopping both the read and write loops.
    Stop,
}

/// `ClientHandle` is used to dispatch [`ClientFunction`]s calls
/// to a corresponding [`FunctionReceiver`] on a `Client`.
#[derive(Clone, Debug)]
pub struct ClientHandle(Sender<ClientFunction>);

impl ClientHandle {
    /// Creates a new `ClientHandle` and corresponding [`FunctionReceiver`] with a buffer of size `buffer`.
    fn new(buffer: usize) -> (Self, FunctionReceiver) {
        let (sender, receiver) = channel(buffer);
        (Self(sender), FunctionReceiver(receiver))
    }

    pub fn write_rdp_pointer(&self, pointer: CGOMousePointerEvent) -> ClientResult<()> {
        self.blocking_send(ClientFunction::WriteRdpPointer(pointer))
    }

    pub async fn write_rdp_pointer_async(&self, pointer: CGOMousePointerEvent) -> ClientResult<()> {
        self.send(ClientFunction::WriteRdpPointer(pointer)).await
    }

    pub fn write_rdp_key(&self, key: CGOKeyboardEvent) -> ClientResult<()> {
        self.blocking_send(ClientFunction::WriteRdpKey(key))
    }

    pub async fn write_rdp_key_async(&self, key: CGOKeyboardEvent) -> ClientResult<()> {
        self.send(ClientFunction::WriteRdpKey(key)).await
    }

    pub fn write_rdp_sync_keys(&self, keys: CGOSyncKeys) -> ClientResult<()> {
        self.blocking_send(ClientFunction::WriteRdpSyncKeys(keys))
    }

    pub async fn write_rdp_sync_keys_async(&self, keys: CGOSyncKeys) -> ClientResult<()> {
        self.send(ClientFunction::WriteRdpSyncKeys(keys)).await
    }

    pub fn write_raw_pdu(&self, resp: Vec<u8>) -> ClientResult<()> {
        self.blocking_send(ClientFunction::WriteRawPdu(resp))
    }

    pub async fn write_raw_pdu_async(&self, resp: Vec<u8>) -> ClientResult<()> {
        self.send(ClientFunction::WriteRawPdu(resp)).await
    }

    pub fn write_rdpdr(&self, pdu: RdpdrPdu) -> ClientResult<()> {
        self.blocking_send(ClientFunction::WriteRdpdr(pdu))
    }

    pub async fn write_rdpdr_async(&self, pdu: RdpdrPdu) -> ClientResult<()> {
        self.send(ClientFunction::WriteRdpdr(pdu)).await
    }

    pub fn write_screen_resize(&self, width: u32, height: u32) -> ClientResult<()> {
        self.blocking_send(ClientFunction::WriteScreenResize(width, height))
    }

    pub async fn write_screen_resize_async(&self, width: u32, height: u32) -> ClientResult<()> {
        self.send(ClientFunction::WriteScreenResize(width, height))
            .await
    }

    pub fn handle_tdp_sd_announce(&self, sda: tdp::SharedDirectoryAnnounce) -> ClientResult<()> {
        self.blocking_send(ClientFunction::HandleTdpSdAnnounce(sda))
    }

    pub async fn handle_tdp_sd_announce_async(
        &self,
        sda: tdp::SharedDirectoryAnnounce,
    ) -> ClientResult<()> {
        self.send(ClientFunction::HandleTdpSdAnnounce(sda)).await
    }

    pub fn handle_tdp_sd_info_response(
        &self,
        res: tdp::SharedDirectoryInfoResponse,
    ) -> ClientResult<()> {
        self.blocking_send(ClientFunction::HandleTdpSdInfoResponse(res))
    }

    pub async fn handle_tdp_sd_info_response_async(
        &self,
        res: tdp::SharedDirectoryInfoResponse,
    ) -> ClientResult<()> {
        self.send(ClientFunction::HandleTdpSdInfoResponse(res))
            .await
    }

    pub fn handle_tdp_sd_create_response(
        &self,
        res: tdp::SharedDirectoryCreateResponse,
    ) -> ClientResult<()> {
        self.blocking_send(ClientFunction::HandleTdpSdCreateResponse(res))
    }

    pub async fn handle_tdp_sd_create_response_async(
        &self,
        res: tdp::SharedDirectoryCreateResponse,
    ) -> ClientResult<()> {
        self.send(ClientFunction::HandleTdpSdCreateResponse(res))
            .await
    }

    pub fn handle_tdp_sd_delete_response(
        &self,
        res: tdp::SharedDirectoryDeleteResponse,
    ) -> ClientResult<()> {
        self.blocking_send(ClientFunction::HandleTdpSdDeleteResponse(res))
    }

    pub async fn handle_tdp_sd_delete_response_async(
        &self,
        res: tdp::SharedDirectoryDeleteResponse,
    ) -> ClientResult<()> {
        self.send(ClientFunction::HandleTdpSdDeleteResponse(res))
            .await
    }

    pub fn handle_tdp_sd_list_response(
        &self,
        res: tdp::SharedDirectoryListResponse,
    ) -> ClientResult<()> {
        self.blocking_send(ClientFunction::HandleTdpSdListResponse(res))
    }

    pub async fn handle_tdp_sd_list_response_async(
        &self,
        res: tdp::SharedDirectoryListResponse,
    ) -> ClientResult<()> {
        self.send(ClientFunction::HandleTdpSdListResponse(res))
            .await
    }

    pub fn handle_tdp_sd_read_response(
        &self,
        res: tdp::SharedDirectoryReadResponse,
    ) -> ClientResult<()> {
        self.blocking_send(ClientFunction::HandleTdpSdReadResponse(res))
    }

    pub async fn handle_tdp_sd_read_response_async(
        &self,
        res: tdp::SharedDirectoryReadResponse,
    ) -> ClientResult<()> {
        self.send(ClientFunction::HandleTdpSdReadResponse(res))
            .await
    }

    pub fn handle_tdp_sd_write_response(
        &self,
        res: tdp::SharedDirectoryWriteResponse,
    ) -> ClientResult<()> {
        self.blocking_send(ClientFunction::HandleTdpSdWriteResponse(res))
    }

    pub async fn handle_tdp_sd_write_response_async(
        &self,
        res: tdp::SharedDirectoryWriteResponse,
    ) -> ClientResult<()> {
        self.send(ClientFunction::HandleTdpSdWriteResponse(res))
            .await
    }

    pub fn handle_tdp_sd_move_response(
        &self,
        res: tdp::SharedDirectoryMoveResponse,
    ) -> ClientResult<()> {
        self.blocking_send(ClientFunction::HandleTdpSdMoveResponse(res))
    }

    pub async fn handle_tdp_sd_move_response_async(
        &self,
        res: tdp::SharedDirectoryMoveResponse,
    ) -> ClientResult<()> {
        self.send(ClientFunction::HandleTdpSdMoveResponse(res))
            .await
    }

    pub fn handle_tdp_sd_truncate_response(
        &self,
        res: tdp::SharedDirectoryTruncateResponse,
    ) -> ClientResult<()> {
        self.blocking_send(ClientFunction::HandleTdpSdTruncateResponse(res))
    }

    pub async fn handle_tdp_sd_truncate_response_async(
        &self,
        res: tdp::SharedDirectoryTruncateResponse,
    ) -> ClientResult<()> {
        self.send(ClientFunction::HandleTdpSdTruncateResponse(res))
            .await
    }

    pub fn write_cliprdr(&self, f: Box<dyn ClipboardFn>) -> ClientResult<()> {
        self.blocking_send(ClientFunction::WriteCliprdr(f))
    }

    pub async fn write_cliprdr_async(&self, f: Box<dyn ClipboardFn>) -> ClientResult<()> {
        self.send(ClientFunction::WriteCliprdr(f)).await
    }

    pub fn update_clipboard(&self, data: String) -> ClientResult<()> {
        self.blocking_send(ClientFunction::UpdateClipboard(data))
    }

    pub async fn update_clipboard_async(&self, data: String) -> ClientResult<()> {
        self.send(ClientFunction::UpdateClipboard(data)).await
    }

    pub fn handle_remote_copy(&self, data: Vec<u8>) -> ClientResult<()> {
        self.blocking_send(ClientFunction::HandleRemoteCopy(data))
    }

    pub async fn handle_remote_copy_async(&self, data: Vec<u8>) -> ClientResult<()> {
        self.send(ClientFunction::HandleRemoteCopy(data)).await
    }

    pub fn stop(&self) -> ClientResult<()> {
        self.blocking_send(ClientFunction::Stop)
    }

    pub async fn stop_async(&self) -> ClientResult<()> {
        self.send(ClientFunction::Stop).await
    }

    fn blocking_send(&self, fun: ClientFunction) -> ClientResult<()> {
        self.0
            .blocking_send(fun)
            .map_err(|e| ClientError::SendError(format!("{:?}", e)))
    }

    async fn send(&self, fun: ClientFunction) -> ClientResult<()> {
        self.0
            .send(fun)
            .await
            .map_err(|e| ClientError::SendError(format!("{:?}", e)))
    }
}

/// Each `Client` has a `FunctionReceiver` that it listens to for
/// incoming [`ClientFunction`] calls sent via its corresponding
/// [`ClientHandle`].
pub struct FunctionReceiver(Receiver<ClientFunction>);

impl FunctionReceiver {
    /// Receives a [`ClientFunction`] call from the `FunctionReceiver`.
    async fn recv(&mut self) -> Option<ClientFunction> {
        self.0.recv().await
    }
}

type RdpReadStream = Framed<TokioStream<ReadHalf<TlsStream<TokioTcpStream>>>>;
type RdpWriteStream = Framed<TokioStream<WriteHalf<TlsStream<TokioTcpStream>>>>;

fn create_config(params: &ConnectParams, pin: String, cgo_handle: CgoHandle) -> Config {
    Config {
        desktop_size: DesktopSize {
            width: params.screen_width,
            height: params.screen_height,
        },
        enable_tls: true,
        enable_credssp: params.ad && params.nla,
        credentials: Credentials::SmartCard {
            config: params.ad.then(|| SmartCardIdentity {
                csp_name: "Microsoft Base Smart Card Crypto Provider".to_string(),
                reader_name: "Teleport".to_string(),
                container_name: "".to_string(),
                certificate: params.cert_der.clone(),
                private_key: params.key_der.clone(),
            }),
            pin,
        },
        domain: None,
        // Windows 10, Version 1909, same as FreeRDP as of October 5th, 2021.
        // This determines which Smart Card Redirection dialect we use per
        // https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpesc/568e22ee-c9ee-4e87-80c5-54795f667062.
        client_build: 18363,
        client_name: "Teleport".to_string(),
        keyboard_type: ironrdp_pdu::gcc::KeyboardType::IbmEnhanced,
        keyboard_subtype: 0,
        keyboard_functional_keys_count: 12,
        keyboard_layout: params.keyboard_layout,
        ime_file_name: "".to_string(),
        bitmap: Some(ironrdp_connector::BitmapConfig {
            lossy_compression: true,
            // Changing this to 16 gets us uncompressed bitmaps on machines configured like
            // https://github.com/Devolutions/IronRDP/blob/55d11a5000ebd474c2ddc294b8b3935554443112/README.md?plain=1#L17-L36
            color_depth: 32,
        }),
        dig_product_id: "".to_string(),
        // `client_dir` is apparently unimportant, however most RDP clients hardcode this value (including FreeRDP):
        // https://github.com/FreeRDP/FreeRDP/blob/4e24b966c86fdf494a782f0dfcfc43a057a2ea60/libfreerdp/core/settings.c#LL49C34-L49C70
        client_dir: "C:\\Windows\\System32\\mstscax.dll".to_string(),
        platform: MajorPlatformType::UNSPECIFIED,
        no_server_pointer: false,
        autologon: true,
        pointer_software_rendering: false,
        // Send the username in the request cookie, which is sent in the initial connection request.
        // The RDP server ignores this value, but load balancers sitting in front of the server
        // can use it to implement persistence.
        request_data: Some(NegoRequestData::cookie(params.username.clone())),
        performance_flags: PerformanceFlags::default()
            | PerformanceFlags::DISABLE_CURSOR_SHADOW // this is required for pointer to work correctly in Windows 2019
            | if !params.show_desktop_wallpaper {
            PerformanceFlags::DISABLE_WALLPAPER
        } else {
            PerformanceFlags::empty()
        },
        desktop_scale_factor: 0,
        license_cache: Some(Arc::new(GoLicenseCache { cgo_handle })),
        hardware_id: Some(params.client_id),
    }
}

#[derive(Debug)]
pub struct ConnectParams {
    pub username: String,
    pub addr: String,
    pub kdc_addr: Option<String>,
    pub computer_name: Option<String>,
    pub cert_der: Vec<u8>,
    pub key_der: Vec<u8>,
    pub screen_width: u16,
    pub screen_height: u16,
    pub allow_clipboard: bool,
    pub allow_directory_sharing: bool,
    pub show_desktop_wallpaper: bool,
    pub ad: bool,
    pub nla: bool,
    pub client_id: [u32; 4],
    pub keyboard_layout: u32,
}

#[derive(Debug)]
pub enum ClientError {
    Tcp(IoError),
    Rdp(RdpError),
    EncodeError(EncodeError),
    PduError(PduError),
    SessionError(SessionError),
    ConnectorError(ConnectorError),
    CGOErrCode(CGOErrCode),
    SendError(String),
    JoinError(JoinError),
    InternalError(String),
    UnknownAddress,
    InputEventError(InputEventError),
    UrlError(url::ParseError),
    #[cfg(feature = "fips")]
    ErrorStack(ErrorStack),
    #[cfg(feature = "fips")]
    HandshakeError(HandshakeError<TokioTcpStream>),
}

impl std::error::Error for ClientError {}

impl Display for ClientError {
    fn fmt(&self, f: &mut Formatter<'_>) -> std::fmt::Result {
        match self {
            ClientError::Tcp(e) => Display::fmt(e, f),
            ClientError::Rdp(e) => Display::fmt(e, f),
            ClientError::SessionError(e) => match &e.kind {
                Reason(reason) => Display::fmt(reason, f),
                _ => Display::fmt(e, f),
            },
            // TODO(zmb3, probakowski): improve the formatting on the IronRDP side
            // https://github.com/Devolutions/IronRDP/blob/master/crates/ironrdp-connector/src/lib.rs#L263
            ClientError::ConnectorError(e) => match &e.kind {
                ConnectorErrorKind::Credssp(e) => {
                    write!(f, "CredSSP {:?}: {}", e.error_type, e.description)
                }
                ConnectorErrorKind::Custom => {
                    write!(f, "Error: {}", e.context)?;
                    if let Some(src) = e.source() {
                        write!(f, " ({})", src)
                    } else {
                        Ok(())
                    }
                }
                _ => Display::fmt(e, f),
            },
            ClientError::InputEventError(e) => Display::fmt(e, f),
            ClientError::JoinError(e) => Display::fmt(e, f),
            ClientError::CGOErrCode(e) => Debug::fmt(e, f),
            ClientError::SendError(msg) => Display::fmt(&msg.to_string(), f),
            ClientError::InternalError(msg) => Display::fmt(&msg.to_string(), f),
            ClientError::UnknownAddress => Display::fmt("Unknown address", f),
            ClientError::EncodeError(e) => Display::fmt(e, f),
            ClientError::PduError(e) => Display::fmt(e, f),
            ClientError::UrlError(e) => Display::fmt(e, f),
            #[cfg(feature = "fips")]
            ClientError::ErrorStack(e) => Display::fmt(e, f),
            #[cfg(feature = "fips")]
            ClientError::HandshakeError(e) => Display::fmt(e, f),
        }
    }
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
    fn from(value: SendError<T>) -> Self {
        ClientError::SendError(format!("{:?}", value))
    }
}

impl From<JoinError> for ClientError {
    fn from(e: JoinError) -> Self {
        ClientError::JoinError(e)
    }
}

impl From<EncodeError> for ClientError {
    fn from(e: EncodeError) -> Self {
        ClientError::EncodeError(e)
    }
}

impl From<PduError> for ClientError {
    fn from(e: PduError) -> Self {
        ClientError::PduError(e)
    }
}

impl From<ClientError> for PduError {
    fn from(e: ClientError) -> Self {
        pdu_other_err!("", source:e)
    }
}

#[cfg(feature = "fips")]
impl From<ErrorStack> for ClientError {
    fn from(e: ErrorStack) -> Self {
        ClientError::ErrorStack(e)
    }
}

#[cfg(feature = "fips")]
impl From<HandshakeError<TokioTcpStream>> for ClientError {
    fn from(e: HandshakeError<TokioTcpStream>) -> Self {
        ClientError::HandshakeError(e)
    }
}

pub type ClientResult<T> = Result<T, ClientError>;

impl From<CGOErrCode> for ClientResult<()> {
    fn from(value: CGOErrCode) -> Self {
        match value {
            CGOErrCode::ErrCodeSuccess => Ok(()),
            _ => Err(ClientError::from(value)),
        }
    }
}

impl From<InputEventError> for ClientError {
    fn from(e: InputEventError) -> Self {
        ClientError::InputEventError(e)
    }
}
