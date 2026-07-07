// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

use crate::error::{ClientError, ClientResult};
use bytes::Bytes;
use rdp_client_proto::desktop::DesktopServiceClient;
use rdp_client_proto::tdpb;
use std::os::unix::ffi::OsStrExt;
use std::path::Path;
use std::time::Duration;
use tokio::sync::mpsc::channel;
use tokio_stream::wrappers::ReceiverStream;

const IPC_CONNECT_TIMEOUT: Duration = Duration::from_secs(5);
const IPC_TDPB_CHANNEL_BUFFER: usize = 100;

pub type IpcClient = DesktopServiceClient<tonic::transport::Channel>;
pub type IpcTdpbSender = tokio::sync::mpsc::Sender<tdpb::Envelope>;
pub type IpcTdpbStream = tonic::Streaming<tdpb::Envelope>;

/// Connects to the IPC gRPC server and starts the `Session` bidirectional streaming RPC
/// used to exchange TDPB messages.
///
/// # Returns
///
/// A tuple containing:
/// - the sender for outgoing TDPB messages
/// - the stream of incoming TDPB messages
/// - the `DesktopService` IPC client
pub async fn connect_ipc(
    ipc_socket: &Path,
) -> ClientResult<(IpcTdpbSender, IpcTdpbStream, IpcClient)> {
    // Unix socket URI follows [RFC-3986](https://datatracker.ietf.org/doc/html/rfc3986)
    // which is aligned with [the gRPC naming convention](https://github.com/grpc/grpc/blob/master/doc/naming.md).
    //
    // unix:///absolute_path
    let mut socket_uri = b"unix:///".to_vec();
    socket_uri.extend_from_slice(ipc_socket.as_os_str().as_bytes());

    let mut ipc_client = tokio::time::timeout(
        IPC_CONNECT_TIMEOUT,
        DesktopServiceClient::connect(Bytes::from(socket_uri)),
    )
    .await
    .map_err(|_| ClientError::IpcTimeout)??;

    let ipc_client_copy = ipc_client.clone();

    let (ipc_tdpb_sender, ipc_tdpb_receiver) = channel::<tdpb::Envelope>(IPC_TDPB_CHANNEL_BUFFER);
    let outbound_stream = ReceiverStream::new(ipc_tdpb_receiver);

    let ipc_tdpb_stream = ipc_client.session(outbound_stream).await?.into_inner();

    Ok((ipc_tdpb_sender, ipc_tdpb_stream, ipc_client_copy))
}
