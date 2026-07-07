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

#[cfg(feature = "fips")]
use boring::error::ErrorStack;
use bytes::BytesMut;
use ironrdp_cliprdr::{Cliprdr, CliprdrClient, CliprdrSvcMessages};
use ironrdp_connector::connection_activation::ConnectionActivationState;
use ironrdp_connector::credssp::KerberosConfig;
use ironrdp_connector::DesktopSize;
use ironrdp_core::encode_vec;
use ironrdp_core::{function, WriteBuf};
use ironrdp_displaycontrol::client::DisplayControlClient;
use ironrdp_displaycontrol::pdu::{
    DisplayControlMonitorLayout, DisplayControlPdu, MonitorLayoutEntry,
};
use ironrdp_dvc::{DrdynvcClient, DvcMessage};
use ironrdp_dvc::{DvcProcessor, DynamicVirtualChannel};
use ironrdp_pdu::encode_err;
use ironrdp_pdu::input::fast_path::{
    FastPathInput, FastPathInputEvent, KeyboardFlags, SynchronizeFlags,
};
use ironrdp_pdu::input::mouse::PointerFlags;
use ironrdp_pdu::input::MousePdu;
use ironrdp_pdu::PduResult;
use ironrdp_rdpdr::pdu::efs::ClientDeviceListAnnounce;
use ironrdp_rdpdr::pdu::RdpdrPdu;
use ironrdp_rdpdr::Rdpdr;
use ironrdp_rdpsnd::client::{NoopRdpsndBackend, Rdpsnd};
use ironrdp_session::x224::{self, DisconnectDescription, ProcessorOutput};
use ironrdp_session::{reason_err, SessionError, SessionResult};
use ironrdp_svc::{SvcMessage, SvcProcessor, SvcProcessorMessages};
use ironrdp_tokio::{single_sequence_step_read, Framed, FramedWrite, TokioStream};
use log::{debug, error, trace, warn};
use std::fmt::Debug;
use std::net::ToSocketAddrs;
use std::sync::{Arc, Mutex, MutexGuard};
use std::time::Duration;
use tokio::io::{split, ReadHalf, WriteHalf};
use tokio::net::TcpStream as TokioTcpStream;
use tokio::sync::mpsc::{channel, Receiver, Sender};
use tokio::task::{self};
// Export this for crate level use.
use crate::cliprdr::{ClipboardFn, TeleportCliprdrBackend};
use crate::config::{create_connector_config, Config};
use crate::error::{ClientError, ClientResult};
use crate::ipc::{IpcClient, IpcTdpbSender, IpcTdpbStream};
use crate::rdpdr::scard::SCARD_DEVICE_ID;
use crate::rdpdr::TeleportRdpdrBackend;
use crate::ssl;
use crate::ssl::TlsStream;
use rdp_client_proto::{desktop, tdpb};
#[cfg(feature = "fips")]
use tokio_boring::HandshakeError;
use tokio_util::sync::CancellationToken;
use url::Url;

const RDP_CONNECT_TIMEOUT: Duration = Duration::from_secs(5);

/// The "Microsoft::Windows::RDS::DisplayControl" DVC is opened
/// by the server. Until it does so, we withhold the latest screen
/// resize, and only send it once we're notified that the DVC is open.
struct PendingResize {
    pending_resize: Option<(u32, u32, u32)>,
}

#[derive(Default)]
struct Position {
    x: u16,
    y: u16,
}

/// The RDP client.
pub struct Client {
    client_handle: ClientHandle,
    read_stream: Option<RdpReadStream>,
    write_stream: Option<RdpWriteStream>,
    ipc_tdpb_sender: IpcTdpbSender,
    ipc_tdpb_stream: IpcTdpbStream,
    function_receiver: Option<FunctionReceiver>,
    x224_processor: Arc<Mutex<x224::Processor>>,
    pending_resize: Arc<Mutex<PendingResize>>,
}

impl Client {
    /// Establishes a new RDP connection and drives the RDP session.
    ///
    /// After establishing the connection, this function creates tasks to:
    /// - Read and process PDUs from the RDP server.
    /// - Read and process TDPB messages from the Go client and execute
    ///   client function calls received from `function_receiver`.
    ///
    /// This function hangs until the RDP session ends, one of the tasks exits, or
    /// cancellation is requested.
    pub async fn run(
        config: Config,
        ipc_client: IpcClient,
        ipc_tdpb_stream: IpcTdpbStream,
        ipc_tdpb_sender: IpcTdpbSender,
        cancellation_token: CancellationToken,
    ) -> ClientResult<Option<DisconnectDescription>> {
        trace!("Client::run");

        Self::connect(config, ipc_client, ipc_tdpb_stream, ipc_tdpb_sender)
            .await?
            .run_loops(cancellation_token)
            .await
    }

    /// Initializes the RDP connection with the given [`Config`].
    async fn connect(
        config: Config,
        mut ipc_client: IpcClient,
        ipc_tdpb_stream: IpcTdpbStream,
        ipc_tdpb_sender: IpcTdpbSender,
    ) -> ClientResult<Self> {
        let server_socket_addr = config
            .server_addr
            .to_socket_addrs()?
            .next()
            .ok_or(ClientError::UnknownAddress)?;

        let stream = tokio::time::timeout(
            RDP_CONNECT_TIMEOUT,
            TokioTcpStream::connect(&server_socket_addr),
        )
        .await
        .map_err(|_| ClientError::TcpTimeout)??;

        // Create a framed stream for use by connect_begin
        let mut framed = ironrdp_tokio::TokioFramed::new(stream);

        let desktop::CertificateAndKey { cert, key } =
            ipc_client.get_certificate_and_key(()).await?.into_inner();

        let connector_config =
            create_connector_config(&config, cert.clone(), key.clone(), ipc_client.clone());

        // Create a channel for sending/receiving function calls to/from the Client.
        let (client_handle, function_receiver) = ClientHandle::new(100);
        let function_receiver = Some(function_receiver);

        let mut rdpdr = Rdpdr::new(
            Box::new(TeleportRdpdrBackend::new(
                client_handle.clone(),
                ipc_tdpb_sender.clone(),
                cert,
                key,
                config.scard_pin,
                config.allow_directory_sharing,
            )),
            "Teleport".to_string(), // directories will show up as "<directory> on Teleport"
        )
        .with_smartcard(SCARD_DEVICE_ID);

        if config.allow_directory_sharing {
            debug!("creating rdpdr client with directory sharing enabled");
            rdpdr = rdpdr.with_drives(None);
        } else {
            debug!("creating rdpdr client with directory sharing disabled")
        }

        let pending_resize = Arc::new(Mutex::new(PendingResize {
            pending_resize: Some((
                config.screen_width as u32,
                config.screen_height as u32,
                config.screen_scale as u32,
            )),
        }));

        let pending_resize_clone = pending_resize.clone();
        let display_control = DisplayControlClient::new(move |_| {
            Self::on_display_ctl_capabilities_received(&pending_resize_clone)
        });
        let drdynvc_client = DrdynvcClient::new().with_dynamic_channel(display_control);

        let mut connector =
            ironrdp_connector::ClientConnector::new(connector_config, server_socket_addr)
                .with_static_channel(drdynvc_client) // require for resizing
                .with_static_channel(Rdpsnd::new(Box::new(NoopRdpsndBackend {}))) // required for rdpdr to work
                .with_static_channel(rdpdr); // required for smart card + directory sharing

        if config.allow_clipboard {
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
        let kerberos_config = config
            .kdc_addr
            .map(|kdc_addr| Url::parse(&format!("tcp://{}", kdc_addr)))
            .transpose()
            .map_err(ClientError::Url)?
            .map(|kdc_url| KerberosConfig {
                kdc_proxy_url: Some(kdc_url),
                hostname: config
                    .computer_name
                    .as_deref()
                    .unwrap_or("missing.computer.name")
                    .to_string(),
            });
        let connection_result = ironrdp_tokio::connect_finalize(
            upgraded,
            connector,
            &mut rdp_stream,
            &mut network_client,
            config.computer_name.unwrap_or(config.server_addr).into(),
            server_public_key,
            kerberos_config,
        )
        .await?;

        // Register the RDP channels with the browser client.
        Self::send_connection_activated(
            ipc_tdpb_sender.clone(),
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
            connection_result.share_id,
            connection_result.connection_activation,
        )));

        Ok(Self {
            client_handle,
            read_stream,
            write_stream,
            ipc_tdpb_sender,
            ipc_tdpb_stream,
            function_receiver,
            x224_processor,
            pending_resize,
        })
    }

    /// Spawns separate tasks for the read and write loops:
    ///
    /// 1. Read Loop: reads new messages from the RDP server and processes them.
    ///
    /// 2. Write Loop: processes TDPB messages from the Go client and
    ///    executes client function calls received from `function_receiver`.
    ///
    /// When either loop exits, or cancellation is requested, any remaining tasks
    /// are aborted and the result is returned.
    async fn run_loops(
        mut self,
        cancellation_token: CancellationToken,
    ) -> ClientResult<Option<DisconnectDescription>> {
        let read_stream = self
            .read_stream
            .take()
            .ok_or_else(|| ClientError::Internal("read_stream failed".to_string()))?;

        let write_stream = self
            .write_stream
            .take()
            .ok_or_else(|| ClientError::Internal("write_stream failed".to_string()))?;

        let function_receiver = self
            .function_receiver
            .take()
            .ok_or_else(|| ClientError::Internal("function_receiver failed".to_string()))?;

        let read_loop_handle = Client::run_read_loop(
            read_stream,
            self.ipc_tdpb_sender.clone(),
            self.x224_processor.clone(),
            self.client_handle.clone(),
        );

        let write_loop_handle = Client::run_write_loop(
            write_stream,
            self.ipc_tdpb_stream,
            self.ipc_tdpb_sender,
            function_receiver,
            self.x224_processor,
            self.pending_resize,
        );

        // Wait until the read loop, write loop, or cancellation completes.
        // Once any of them finishes, the remaining tasks are canceled and the
        // corresponding result is returned.
        tokio::select! {
            res = read_loop_handle => {
                res
            },
            res = write_loop_handle => {
                res.map(|_| None)
            },
            _ = cancellation_token.cancelled() => {
                Ok(None)
            },
        }
    }

    async fn run_read_loop(
        mut read_stream: RdpReadStream,
        ipc_tdpb_sender: IpcTdpbSender,
        x224_processor: Arc<Mutex<x224::Processor>>,
        write_requester: ClientHandle,
    ) -> ClientResult<Option<DisconnectDescription>> {
        loop {
            let (action, frame) = read_stream.read_pdu().await?;
            match action {
                // Fast-path PDU, send to the browser for processing / rendering.
                ironrdp_pdu::Action::FastPath => {
                    let fast_path_msg = tdpb::Envelope {
                        payload: Some(tdpb::envelope::Payload::FastPathPdu(tdpb::FastPathPdu {
                            pdu: frame.into(),
                        })),
                    };

                    ipc_tdpb_sender.send(fast_path_msg).await?;
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
                                    } = sequence.connection_activation_state()
                                    {
                                        // Upon completing the activation sequence, register the io/user channels
                                        // and desktop size with the client, just like we do upon receiving the
                                        // connection result in [`Self::connect`].
                                        Self::send_connection_activated(
                                            ipc_tdpb_sender.clone(),
                                            io_channel_id,
                                            user_channel_id,
                                            desktop_size,
                                        )
                                        .await?;
                                        break;
                                    }
                                }
                            }
                            ProcessorOutput::AutoDetect(req) => {
                                // These are allegedly handled automatically internally,
                                // so we'll just log them in case they're useful for debugging.
                                debug!("received autodetect request: {:?}", req);
                            }
                            ProcessorOutput::MultitransportRequest(_) => {
                                error!("Received unsupported multi-transport request")
                            }
                            ProcessorOutput::PointerUpdate(_) => {
                                error!("Received unsupported slow-path pointer update")
                            }
                            ProcessorOutput::GraphicsUpdate(_) => {
                                error!("Received unsupported slow-path graphics update")
                            }
                        }
                    }
                }
            }
        }
    }

    async fn run_write_loop(
        mut write_stream: RdpWriteStream,
        mut ipc_tdpb_stream: IpcTdpbStream,
        ipc_tdpb_sender: IpcTdpbSender,
        mut write_receiver: FunctionReceiver,
        x224_processor: Arc<Mutex<x224::Processor>>,
        pending_resize: Arc<Mutex<PendingResize>>,
    ) -> ClientResult<()> {
        let mut last_mouse_position = Position::default();

        loop {
            tokio::select! {
                result = ipc_tdpb_stream.message() => {
                    let msg = result?;
                    match msg {
                        Some(envelope) => Client::handle_tdpb_message(
                            &mut write_stream,
                            x224_processor.clone(),
                            pending_resize.clone(),
                            envelope,
                            &mut last_mouse_position,
                        ).await?,
                        None => {
                            warn!("IPC channel closed, shutting down write loop");
                            break;
                        }
                    }
                }
                client_fn = write_receiver.recv() => {
                    match client_fn {
                        Some(client_fn) => Client::handle_client_function(
                            &mut write_stream,
                            ipc_tdpb_sender.clone(),
                            x224_processor.clone(),
                            client_fn,
                        ).await?,
                        None => {
                            warn!("Client function channel closed, shutting down write loop");
                            break;
                        }
                    }
                }
            }
        }

        Ok(())
    }

    async fn handle_tdpb_message(
        write_stream: &mut RdpWriteStream,
        x224_processor: Arc<Mutex<x224::Processor>>,
        pending_resize: Arc<Mutex<PendingResize>>,
        msg: tdpb::Envelope,
        last_mouse_position: &mut Position,
    ) -> ClientResult<()> {
        use tdpb::envelope::Payload;

        let Some(payload) = msg.payload else {
            warn!("Received IPC Envelope message with no payload, skipping");
            return Ok(());
        };

        match payload {
            Payload::ClientScreenSpec(client_screen_spec) => {
                Client::handle_screen_resize(
                    client_screen_spec.width,
                    client_screen_spec.height,
                    client_screen_spec.scale,
                    x224_processor,
                    write_stream,
                    pending_resize,
                )
                .await
            }
            Payload::KeyboardButton(keyboard_button) => {
                Client::write_rdp_key(write_stream, keyboard_button).await
            }
            Payload::MouseMove(mouse_move) => {
                Client::write_rdp_mouse_move(write_stream, mouse_move, last_mouse_position).await
            }
            Payload::MouseButton(mouse_button) => {
                Client::write_rdp_mouse_button(write_stream, mouse_button, last_mouse_position)
                    .await
            }
            Payload::MouseWheel(mouse_wheel) => {
                Client::write_rdp_mouse_wheel(write_stream, mouse_wheel, last_mouse_position).await
            }
            Payload::SyncKeys(sync_keys) => {
                Client::write_rdp_sync_keys(write_stream, sync_keys).await
            }
            Payload::ClipboardData(clipboard_data) => {
                Client::update_clipboard(x224_processor, clipboard_data).await
            }
            Payload::SharedDirectoryAnnounce(sda) => {
                Client::handle_shared_dir_announce(write_stream, x224_processor, sda).await
            }
            Payload::SharedDirectoryRemove(sdr) => {
                Client::handle_shared_dir_remove(write_stream, x224_processor, sdr).await
            }
            Payload::SharedDirectoryResponse(res) => {
                Client::handle_shared_dir_response(x224_processor, res).await
            }
            Payload::RdpResponsePdu(rdp_response_pdu) => {
                Client::write_raw_pdu(write_stream, rdp_response_pdu.response).await
            }
            _ => {
                warn!("Skipping unimplemented TDPB message");
                Ok(())
            }
        }
    }

    async fn handle_client_function(
        write_stream: &mut RdpWriteStream,
        ipc_tdpb_sender: IpcTdpbSender,
        x224_processor: Arc<Mutex<x224::Processor>>,
        function: ClientFunction,
    ) -> ClientResult<()> {
        match function {
            ClientFunction::WriteRawPdu(pdu) => Client::write_raw_pdu(write_stream, pdu).await,
            ClientFunction::WriteRdpdr(args) => {
                Client::write_rdpdr(write_stream, x224_processor, args).await
            }
            ClientFunction::WriteCliprdr(f) => {
                Client::write_cliprdr(x224_processor, write_stream, f).await
            }
            ClientFunction::HandleRemoteCopy(data) => {
                Client::handle_remote_copy(ipc_tdpb_sender, data).await
            }
            ClientFunction::HandleSharedDirRemove(sdr) => {
                Client::handle_shared_dir_remove(write_stream, x224_processor, sdr).await
            }
        }
    }

    fn on_display_ctl_capabilities_received(
        pending_resize: &Arc<Mutex<PendingResize>>,
    ) -> PduResult<Vec<DvcMessage>> {
        debug!("DisplayControlClient channel opened");
        // We've been notified that the DisplayControl dvc channel has been opened:
        let mut pending_resize =
            Self::resize_manager_lock(pending_resize).map_err(ClientError::from)?;
        let pending_resize = pending_resize.pending_resize.take();
        if let Some((initial_width, initial_height, scale)) = pending_resize {
            // If there was a resize pending, perform it now.
            debug!(
                "Pending resize for size [{:?}x{:?}] scale [{:?}] found, sending now",
                initial_width, initial_height, scale
            );
            let (width, height) =
                MonitorLayoutEntry::adjust_display_size(initial_width, initial_height);
            if width != initial_width || height != initial_height {
                debug!("Adjusted screen resize to [{:?}x{:?}]", width, height);
            }
            let pdu: DisplayControlPdu = DisplayControlMonitorLayout::new_single_primary_monitor(
                width,
                height,
                rdp_scale_factor(scale),
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
        ipc_tdpb_sender: IpcTdpbSender,
        io_channel_id: u16,
        user_channel_id: u16,
        desktop_size: DesktopSize,
    ) -> ClientResult<()> {
        let server_hello = tdpb::Envelope {
            payload: Some(tdpb::envelope::Payload::ServerHello(tdpb::ServerHello {
                activation_spec: Some(tdpb::ConnectionActivated {
                    io_channel_id: u32::from(io_channel_id),
                    user_channel_id: u32::from(user_channel_id),
                    screen_width: u32::from(desktop_size.width),
                    screen_height: u32::from(desktop_size.height),
                }),
                clipboard_enabled: true,
                directory_remove_supported: true,
                sessions: Vec::new(),
                hidpi_supported: true,
                multidirectory_sharing_supported: true,
            })),
        };

        ipc_tdpb_sender.send(server_hello).await?;

        Ok(())
    }

    async fn update_clipboard(
        x224_processor: Arc<Mutex<x224::Processor>>,
        clipboard_data: tdpb::ClipboardData,
    ) -> ClientResult<()> {
        let data = match String::from_utf8(clipboard_data.data) {
            Ok(s) => s,
            Err(e) => {
                return Err(ClientError::Internal(format!(
                    "failed to convert clipboard data to UTF-8 string: {}",
                    e
                )));
            }
        };

        task::spawn_blocking(move || {
            let mut x224_processor = Self::x224_lock(&x224_processor)?;
            let cliprdr = Self::cliprdr_backend(&mut x224_processor)?;
            cliprdr.set_clipboard_data(data);
            Ok(())
        })
        .await?
    }

    async fn handle_remote_copy(ipc_tdpb_sender: IpcTdpbSender, data: Vec<u8>) -> ClientResult<()> {
        let clipboard_data_msg = tdpb::Envelope {
            payload: Some(tdpb::envelope::Payload::ClipboardData(
                tdpb::ClipboardData { data },
            )),
        };

        ipc_tdpb_sender.send(clipboard_data_msg).await?;

        Ok(())
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
                let cliprdr = Self::get_svc_processor_mut::<CliprdrClient>(&mut x224_processor)?;
                Ok(fun.call(cliprdr)?)
            })
            .await?;
        let encoded = Client::x224_process_svc_messages(x224_processor, messages?).await?;
        write_stream.write_all(&encoded).await?;
        Ok(())
    }

    async fn write_rdp_key(
        write_stream: &mut RdpWriteStream,
        keyboard_button: tdpb::KeyboardButton,
    ) -> ClientResult<()> {
        let mut flags: KeyboardFlags = KeyboardFlags::empty();
        if !keyboard_button.pressed {
            flags = KeyboardFlags::RELEASE;
        }
        let extended = keyboard_button.key_code & 0xE000 == 0xE000;
        if extended {
            flags |= KeyboardFlags::EXTENDED;
        }

        let event = FastPathInputEvent::KeyboardEvent(flags, keyboard_button.key_code as u8);

        Self::write_fast_path_input_event(write_stream, event).await
    }

    async fn write_rdp_mouse_move(
        write_stream: &mut RdpWriteStream,
        mouse_move: tdpb::MouseMove,
        last_mouse_pos: &mut Position,
    ) -> ClientResult<()> {
        *last_mouse_pos = Position {
            x: mouse_move.x as u16,
            y: mouse_move.y as u16,
        };

        let flags = PointerFlags::MOVE;

        let event = FastPathInputEvent::MouseEvent(MousePdu {
            flags,
            number_of_wheel_rotation_units: 0,
            x_position: last_mouse_pos.x,
            y_position: last_mouse_pos.y,
        });

        Self::write_fast_path_input_event(write_stream, event).await
    }

    async fn write_rdp_mouse_button(
        write_stream: &mut RdpWriteStream,
        mouse_button: tdpb::MouseButton,
        last_mouse_pos: &Position,
    ) -> ClientResult<()> {
        let button_type = tdpb::MouseButtonType::try_from(mouse_button.button)
            .inspect_err(|val| warn!("Received unknown mouse button type: {:?}", val))
            .unwrap_or(tdpb::MouseButtonType::Unspecified);

        let mut flags = match button_type {
            tdpb::MouseButtonType::Left => PointerFlags::LEFT_BUTTON,
            tdpb::MouseButtonType::Right => PointerFlags::RIGHT_BUTTON,
            tdpb::MouseButtonType::Middle => PointerFlags::MIDDLE_BUTTON_OR_WHEEL,
            _ => PointerFlags::MOVE,
        };

        if mouse_button.pressed {
            flags |= PointerFlags::DOWN;
        }

        let event = FastPathInputEvent::MouseEvent(MousePdu {
            flags,
            number_of_wheel_rotation_units: 0,
            x_position: last_mouse_pos.x,
            y_position: last_mouse_pos.y,
        });

        Self::write_fast_path_input_event(write_stream, event).await
    }

    async fn write_rdp_mouse_wheel(
        write_stream: &mut RdpWriteStream,
        mouse_wheel: tdpb::MouseWheel,
        last_mouse_pos: &Position,
    ) -> ClientResult<()> {
        let mouse_wheel_axis = tdpb::MouseWheelAxis::try_from(mouse_wheel.axis)
            .inspect_err(|val| warn!("Received unknown mouse wheel axis type: {:?}", val))
            .unwrap_or(tdpb::MouseWheelAxis::Unspecified);

        let mut delta = mouse_wheel.delta as i16;

        let flags = match mouse_wheel_axis {
            tdpb::MouseWheelAxis::Vertical => PointerFlags::VERTICAL_WHEEL,
            tdpb::MouseWheelAxis::Horizontal => {
                // TDP positive scroll deltas move towards top-left.
                // RDP positive scroll deltas move towards top-right.
                //
                // Fix the scroll direction to match TDP, it's inverted for
                // horizontal scroll in RDP.
                delta = -delta;

                PointerFlags::HORIZONTAL_WHEEL
            }
            _ => PointerFlags::MOVE,
        };

        let event = FastPathInputEvent::MouseEvent(MousePdu {
            flags,
            number_of_wheel_rotation_units: delta,
            x_position: last_mouse_pos.x,
            y_position: last_mouse_pos.y,
        });

        Self::write_fast_path_input_event(write_stream, event).await
    }

    async fn write_rdp_sync_keys(
        write_stream: &mut RdpWriteStream,
        keys: tdpb::SyncKeys,
    ) -> ClientResult<()> {
        let mut flags = SynchronizeFlags::empty();
        if keys.scroll_lock_pressed {
            flags |= SynchronizeFlags::SCROLL_LOCK;
        }
        if keys.num_lock_state {
            flags |= SynchronizeFlags::NUM_LOCK;
        }
        if keys.caps_lock_state {
            flags |= SynchronizeFlags::CAPS_LOCK;
        }
        if keys.kana_lock_state {
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
            .write_all(&encode_vec(&FastPathInput::single(event))?)
            .await?;
        Ok(())
    }

    /// Writes a fully encoded PDU to the RDP server.
    async fn write_raw_pdu(write_stream: &mut RdpWriteStream, resp: Vec<u8>) -> ClientResult<()> {
        write_stream.write_all(&resp).await?;
        Ok(())
    }

    async fn write_rdpdr_pdus(
        write_stream: &mut RdpWriteStream,
        x224_processor: Arc<Mutex<x224::Processor>>,
        pdus: Vec<RdpdrPdu>,
    ) -> ClientResult<()> {
        debug!("sending rdp: {:?}", pdus);

        let svc_messages: Vec<SvcMessage> = pdus.into_iter().map(SvcMessage::from).collect();

        // Process the RDPDR PDU.
        let encoded = Client::x224_process_svc_messages(
            x224_processor,
            SvcProcessorMessages::<Rdpdr>::new(svc_messages),
        )
        .await?;

        // Write the RDPDR PDU to the RDP server.
        write_stream.write_all(&encoded).await?;
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
        scale: u32,
        x224_processor: Arc<Mutex<x224::Processor>>,
        write_stream: &mut RdpWriteStream,
        pending_resize: Arc<Mutex<PendingResize>>,
    ) -> ClientResult<()> {
        // Adjust the screen size to the nearest supported resolution (per the RDP spec).
        let init_width = width;
        let init_height = height;
        debug!(
            "Received screen resize [{:?}x{:?}] scale [{:?}]",
            init_width, init_height, scale
        );
        let (width, height) = MonitorLayoutEntry::adjust_display_size(init_width, init_height);
        if width != init_width || height != init_height {
            debug!("Adjusted screen resize to [{:?}x{:?}]", width, height);
        }

        // Our DisplayControlClient is lazily initialized and added as a svc_processor
        // once the dynamic channel for display control is opened and server capabilities are
        // received. Failure to acquire the DVC is normal until this point in the connection setup.
        // Ensure that the DVC is both accessible and open.
        let dvc_is_ready = {
            Self::x224_lock(&x224_processor)?
                .get_dvc::<DisplayControlClient>()
                .is_some_and(|dvc| dvc.is_open())
        };

        if dvc_is_ready {
            return Client::write_screen_resize(write_stream, x224_processor, width, height, scale)
                .await;
        }

        // The client requested a resize but the DisplayControl channel has not been opened yet.
        // Sending the resize now would cause an RDP error and end the session; instead we withhold
        // it until the DisplayControl channel is ready.
        debug!("DisplayControl channel not ready, withholding resize");
        let mut pending_resize = Self::resize_manager_lock(&pending_resize)?;
        pending_resize.pending_resize = Some((width, height, scale));

        Ok(())
    }

    /// Sends a screen resize to the RDP server.
    async fn write_screen_resize(
        write_stream: &mut RdpWriteStream,
        x224_processor: Arc<Mutex<x224::Processor>>,
        width: u32,
        height: u32,
        scale: u32,
    ) -> ClientResult<()> {
        let cloned = x224_processor.clone();
        let scale_factor = rdp_scale_factor(scale);
        let messages = task::spawn_blocking(move || {
            let x224_processor = Self::x224_lock(&cloned)?;
            let dvc = Self::get_dvc::<DisplayControlClient>(&x224_processor)?;
            let channel_id = dvc.channel_id().ok_or(ClientError::Internal(
                "DisplayControlClient channel_id not found".to_string(),
            ))?;
            let disp_ctl_cli = dvc
                .channel_processor_downcast_ref::<DisplayControlClient>()
                .ok_or(ClientError::Internal(
                    "DisplayControlClient not found".to_string(),
                ))?;

            Ok::<_, ClientError>(disp_ctl_cli.encode_single_primary_monitor(
                channel_id,
                width,
                height,
                scale_factor,
                Some((width, height)),
            ))
        })
        .await???;

        let encoded = Client::x224_process_svc_messages(
            x224_processor,
            SvcProcessorMessages::<DrdynvcClient>::new(messages),
        )
        .await?;
        debug!(
            "Writing resize to [{:?}x{:?}] scale [{:?}]",
            width, height, scale
        );
        write_stream.write_all(&encoded).await?;

        Ok(())
    }

    async fn handle_shared_dir_announce(
        write_stream: &mut RdpWriteStream,
        x224_processor: Arc<Mutex<x224::Processor>>,
        sda: tdpb::SharedDirectoryAnnounce,
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

    async fn handle_shared_dir_remove(
        write_stream: &mut RdpWriteStream,
        x224_processor: Arc<Mutex<x224::Processor>>,
        sdr: tdpb::SharedDirectoryRemove,
    ) -> ClientResult<()> {
        debug!("received tdp: {:?}", sdr);

        let cancel_pdus = Self::remove_drive(x224_processor.clone(), sdr.directory_id)?;

        // Bulk send any cancellations for pending I/O requests.
        Self::write_rdpdr_pdus(write_stream, x224_processor.clone(), cancel_pdus).await?;
        Ok(())
    }

    async fn handle_shared_dir_response(
        x224_processor: Arc<Mutex<x224::Processor>>,
        res: tdpb::SharedDirectoryResponse,
    ) -> ClientResult<()> {
        task::spawn_blocking(move || {
            debug!("received tdp: {:?}", res);
            let mut x224_processor = Self::x224_lock(&x224_processor)?;
            let rdpdr = Self::rdpdr_backend(&mut x224_processor)?;
            rdpdr.handle_shared_dir_response(res)?;
            Ok(())
        })
        .await?
    }

    async fn add_drive(
        x224_processor: Arc<Mutex<x224::Processor>>,
        sda: tdpb::SharedDirectoryAnnounce,
    ) -> ClientResult<ClientDeviceListAnnounce> {
        task::spawn_blocking(move || {
            let mut x224_processor = Self::x224_lock(&x224_processor)?;
            // Make sure the teleport backend knows about this new drive.
            Self::rdpdr_backend(&mut x224_processor)?.add_device(sda.directory_id)?;
            // The Base Rdpdr instance must also know about the device.
            Ok(Self::get_svc_processor_mut::<Rdpdr>(&mut x224_processor)?
                .add_drive(sda.directory_id, sda.name))
        })
        .await?
    }

    fn remove_drive(
        x224_processor: Arc<Mutex<x224::Processor>>,
        device_id: u32,
    ) -> ClientResult<Vec<RdpdrPdu>> {
        // Lock the x224 processor before calling "remove_drive" so that the read loop
        // doesn't try to process an inbound message for a device that we're in the process
        // of removing.
        let mut processor = Self::x224_lock(&x224_processor)?;
        let backend = Self::rdpdr_backend(&mut processor)?;

        // Attempt to remove the device from the Teleport Rdpdr backend
        let (mut cancel_pdus, remove_complete) = backend
            .remove_device(device_id)
            .inspect_err(|e| warn!("could not remove device from teleport backend: {}", e))?;

        if remove_complete {
            // If the device was successfully removed from the backend, then remove it from
            // the top level rdpdr instance and send the device remove pdu. Otherwise,
            // do nothing. The the rdpdr backend will synthesize another TDP shared directory remove
            // message once the instance is ready for deletion (leading us right back here again to retry).
            let remove_pdu = processor
                .get_svc_processor_mut::<Rdpdr>()
                .ok_or(ClientError::UnknownDevice(device_id))?
                .remove_device(device_id)
                .ok_or(ClientError::UnknownDevice(device_id))?;

            // Make sure the remove PDU is pushed last.
            cancel_pdus.push(RdpdrPdu::ClientDeviceListRemove(remove_pdu))
        }

        Ok(cancel_pdus)
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
    ) -> Result<MutexGuard<'_, x224::Processor>, SessionError> {
        x224_processor
            .lock()
            .map_err(|err| reason_err!(function!(), "PoisonError: {:?}", err))
    }

    fn resize_manager_lock(
        pending_resize: &Arc<Mutex<PendingResize>>,
    ) -> Result<MutexGuard<'_, PendingResize>, SessionError> {
        pending_resize
            .lock()
            .map_err(|err| reason_err!(function!(), "PoisonError: {:?}", err))
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
            .ok_or(ClientError::Internal(format!(
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
            .ok_or(ClientError::Internal(format!(
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
            .ok_or(ClientError::Internal(
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
            .ok_or(ClientError::Internal(
                "rdpdr_backend returned None".to_string(),
            ))
    }
}

/// [`ClientFunction`] is an enum representing the different functions that can be called on a client.
/// Each variant corresponds to a different function, and carries the necessary arguments for that function.
///
/// This enum is used by [`ClientHandle`]'s methods to dispatch function calls to the corresponding [`Client`] instance.
#[derive(Debug)]
enum ClientFunction {
    /// Corresponds to [`Client::write_raw_pdu`]
    WriteRawPdu(Vec<u8>),
    /// Corresponds to [`Client::write_rdpdr`]
    WriteRdpdr(RdpdrPdu),
    /// Corresponds to [`Client::write_cliprdr`]
    WriteCliprdr(Box<dyn ClipboardFn>),
    /// Corresponds to [`Client::handle_remote_copy`]
    HandleRemoteCopy(Vec<u8>),
    /// Corresponds to [`Client::handle_shared_dir_remove`]
    HandleSharedDirRemove(tdpb::SharedDirectoryRemove),
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

    pub fn write_cliprdr(&self, f: Box<dyn ClipboardFn>) -> ClientResult<()> {
        self.blocking_send(ClientFunction::WriteCliprdr(f))
    }

    pub async fn write_cliprdr_async(&self, f: Box<dyn ClipboardFn>) -> ClientResult<()> {
        self.send(ClientFunction::WriteCliprdr(f)).await
    }

    pub fn handle_remote_copy(&self, data: Vec<u8>) -> ClientResult<()> {
        self.blocking_send(ClientFunction::HandleRemoteCopy(data))
    }

    pub async fn handle_remote_copy_async(&self, data: Vec<u8>) -> ClientResult<()> {
        self.send(ClientFunction::HandleRemoteCopy(data)).await
    }

    pub fn handle_shared_dir_remove(&self, sdr: tdpb::SharedDirectoryRemove) -> ClientResult<()> {
        self.blocking_send(ClientFunction::HandleSharedDirRemove(sdr))
    }

    pub async fn handle_shared_dir_remove_async(
        &self,
        sdr: tdpb::SharedDirectoryRemove,
    ) -> ClientResult<()> {
        self.send(ClientFunction::HandleSharedDirRemove(sdr)).await
    }

    fn blocking_send(&self, fun: ClientFunction) -> ClientResult<()> {
        self.0.blocking_send(fun).map_err(ClientError::from)
    }

    async fn send(&self, fun: ClientFunction) -> ClientResult<()> {
        self.0.send(fun).await.map_err(ClientError::from)
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

/// Converts a raw scale value to an RDP scale factor.
/// Per the RDP spec, valid values are in [100, 500]; anything outside
/// that range is treated as unset (None).
fn rdp_scale_factor(scale: u32) -> Option<u32> {
    if (100..=500).contains(&scale) {
        Some(scale)
    } else {
        None
    }
}
