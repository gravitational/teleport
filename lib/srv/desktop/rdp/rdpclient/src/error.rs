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

#[cfg(feature = "fips")]
use boring::error::ErrorStack;
use ironrdp_connector::{ConnectorError, ConnectorErrorKind};
use ironrdp_core::EncodeError;
use ironrdp_pdu::{pdu_other_err, PduError};
use ironrdp_session::SessionError;
use std::error::Error;
use std::fmt::Debug;
use std::io::Error as IoError;
use thiserror::Error;
use tokio::sync::mpsc::error::SendError;
use tokio::task::JoinError;
#[cfg(feature = "fips")]
use tokio_boring::HandshakeError;

const TIMEOUT_ERROR_MESSAGE: &str = "Connection Timed Out\n\n\
Teleport could not connect to the host within the timeout period. \
This could be due to a firewall blocking connections, an overloaded system, \
or network congestion. To resolve this issue, ensure that the Teleport agent \
has connectivity to the Windows host.\n\n\
Use \"nc -vz HOST 3389\" to help debug this issue.";

pub type ClientResult<T> = Result<T, ClientError>;

#[derive(Debug, Error)]
pub enum ClientError {
    #[error(transparent)]
    Io(#[from] IoError),
    #[error("{TIMEOUT_ERROR_MESSAGE}")]
    TcpTimeout,
    #[error(transparent)]
    IpcTransport(#[from] tonic::transport::Error),
    #[error(transparent)]
    Ipc(#[from] tonic::Status),
    #[error("gRPC connection timed out")]
    IpcTimeout,
    #[error(transparent)]
    Encode(#[from] EncodeError),
    #[error("{}", fmt_pdu_error(.0))]
    Pdu(#[from] PduError),
    #[error(transparent)]
    Session(#[from] SessionError),
    #[error("{}", fmt_connector_error(.0))]
    Connector(#[from] ConnectorError),
    #[error("{0}")]
    Send(String),
    #[error(transparent)]
    Join(#[from] JoinError),
    #[error("{0}")]
    Internal(String),
    #[error("Unknown address")]
    UnknownAddress,
    #[error("Unknown device: {0}")]
    UnknownDevice(u32),
    #[error(transparent)]
    Url(#[from] url::ParseError),
    #[cfg(feature = "fips")]
    #[error(transparent)]
    ErrorStack(#[from] ErrorStack),
    #[cfg(feature = "fips")]
    #[error(transparent)]
    Handshake(#[from] HandshakeError<TokioTcpStream>),
}

impl<T> From<SendError<T>> for ClientError {
    fn from(value: SendError<T>) -> Self {
        ClientError::Send(format!("{:?}", value))
    }
}

impl From<ClientError> for PduError {
    fn from(e: ClientError) -> Self {
        pdu_other_err!("", source:e)
    }
}

fn fmt_connector_error(e: &ConnectorError) -> String {
    // TODO(zmb3, probakowski): improve the formatting on the IronRDP side
    // https://github.com/Devolutions/IronRDP/blob/master/crates/ironrdp-connector/src/lib.rs#L263
    match &e.kind() {
        ConnectorErrorKind::Credssp(e) => {
            format!("CredSSP {:?}: {}", e.error_type, e.description)
        }
        ConnectorErrorKind::Custom => {
            let source = if let Some(src) = e.source() {
                format!(" ({})", src)
            } else {
                String::new()
            };

            format!("Error: {}{}", e.report(), source)
        }
        _ => e.to_string(),
    }
}

fn fmt_pdu_error(e: &PduError) -> String {
    e.report().to_string()
}
