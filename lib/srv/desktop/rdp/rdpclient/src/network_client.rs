/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

use std::future::Future;
use std::pin::Pin;

use ironrdp_connector::sspi::generator::NetworkRequest;
use ironrdp_connector::sspi::network_client::NetworkProtocol;
use ironrdp_connector::{custom_err, general_err, ConnectorResult};
use ironrdp_tokio::AsyncNetworkClient;
use sspi::{Error, ErrorKind};
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::TcpStream;
use url::Url;

pub(crate) struct NetworkClient;

impl AsyncNetworkClient for NetworkClient {
    fn send<'a>(
        &'a mut self,
        request: &'a NetworkRequest,
    ) -> Pin<Box<dyn Future<Output = ConnectorResult<Vec<u8>>> + 'a>> {
        Box::pin(async move {
            match &request.protocol {
                NetworkProtocol::Tcp => self.send_tcp(&request.url, &request.data).await,
                NetworkProtocol::Udp => Err(general_err!("UDP is not supported")),
                NetworkProtocol::Http => Err(general_err!("HTTP is not supported")),
                NetworkProtocol::Https => Err(general_err!("HTTPS is not supported")),
            }
        })
    }
}

impl NetworkClient {
    pub(crate) fn new() -> Self {
        Self
    }
}

const DEFAULT_KERBEROS_PORT: u16 = 88;

impl NetworkClient {
    async fn send_tcp(&self, url: &Url, data: &[u8]) -> ConnectorResult<Vec<u8>> {
        let addr = format!(
            "{}:{}",
            url.host_str().unwrap_or_default(),
            url.port().unwrap_or(DEFAULT_KERBEROS_PORT)
        );

        let mut stream = TcpStream::connect(addr)
            .await
            .map_err(|e| Error::new(ErrorKind::NoAuthenticatingAuthority, format!("{:?}", e)))
            .map_err(|e| custom_err!("failed to connect to KDC over TCP", e))?;

        stream
            .write(data)
            .await
            .map_err(|e| Error::new(ErrorKind::NoAuthenticatingAuthority, format!("{:?}", e)))
            .map_err(|e| custom_err!("failed to send KDC request over TCP", e))?;

        let len = stream
            .read_u32()
            .await
            .map_err(|e| Error::new(ErrorKind::NoAuthenticatingAuthority, format!("{:?}", e)))
            .map_err(|e| custom_err!("failed to read KDC response length over TCP", e))?;

        let mut buf = vec![0; len as usize + 4];
        buf[0..4].copy_from_slice(&(len.to_be_bytes()));

        stream
            .read_exact(&mut buf[4..])
            .await
            .map_err(|e| Error::new(ErrorKind::NoAuthenticatingAuthority, format!("{:?}", e)))
            .map_err(|e| custom_err!("failed to send KDC response over TCP", e))?;

        Ok(buf)
    }
}
