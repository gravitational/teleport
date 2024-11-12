/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
use ironrdp_connector::{general_err, reason_err, ConnectorResult};
use ironrdp_tokio::AsyncNetworkClient;
use log::error;
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

// Maximum response size from KDC we accept, Windows uses maximum token size of 48kB and recommends
// not to exceed 64kB
// https://learn.microsoft.com/en-us/troubleshoot/windows-server/windows-security/kerberos-authentication-problems-if-user-belongs-to-groups#calculating-the-maximum-token-size
const MAX_RESPONSE_LENGTH: u32 = 65535;

impl NetworkClient {
    async fn send_tcp(&self, url: &Url, data: &[u8]) -> ConnectorResult<Vec<u8>> {
        let addr = format!(
            "{}:{}",
            url.host_str().unwrap_or_default(),
            url.port().unwrap_or(DEFAULT_KERBEROS_PORT)
        );

        let mut stream = TcpStream::connect(addr).await.map_err(|e| {
            error!("KDC connection failed: {:?}", e);
            reason_err!("NLA", "connection to Key Distribution Center failed")
        })?;

        stream.write(data).await.map_err(|e| {
            error!("KDC send failed: {:?}", e);
            reason_err!("NLA", "sending data to Key Distribution Center failed")
        })?;

        let len = stream.read_u32().await.map_err(|e| {
            error!("KDC length read failed: {:?}", e);
            reason_err!("NLA", "reading data from Key Distribution Center failed")
        })?;

        if len > MAX_RESPONSE_LENGTH {
            error!("KDC response too large: {} > {}", len, MAX_RESPONSE_LENGTH);
            return Err(reason_err!(
                "NLA",
                "response from Key Distribution Center was too large"
            ));
        }

        let mut buf = vec![0; len as usize + 4];
        buf[0..4].copy_from_slice(&(len.to_be_bytes()));

        stream.read_exact(&mut buf[4..]).await.map_err(|e| {
            error!("KDC read failed: {:?}", e);
            reason_err!("NLA", "reading data from Key Distribution Center failed")
        })?;

        Ok(buf)
    }
}
