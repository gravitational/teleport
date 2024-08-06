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

use crate::client::{ClientError, ClientResult};
#[cfg(feature = "fips")]
use static_init::dynamic;
use tokio::net::TcpStream;

#[cfg(feature = "fips")]
pub type TlsStream<S> = tokio_boring::SslStream<S>;

#[cfg(feature = "fips")]
#[dynamic(0)]
static mut FIPS_CHECK: () = unsafe {
    // Make sure that we really have FIPS enabled.
    // This assert will run at the start of the program and panic if we
    // build for FIPS but it's somehow disabled
    use boring;
    assert!(boring::fips::enabled(), "FIPS mode not enabled");
};

#[cfg(not(feature = "fips"))]
pub type TlsStream<S> = ironrdp_tls::TlsStream<S>;

pub(crate) async fn upgrade(
    initial_stream: TcpStream,
    server_name: &str,
) -> ClientResult<(TlsStream<TcpStream>, Vec<u8>)> {
    #[cfg(feature = "fips")]
    {
        use boring::ssl::{SslConnector, SslMethod, SslVerifyMode};
        use std::io;
        use tokio::io::AsyncWriteExt;
        let mut builder = SslConnector::builder(SslMethod::tls_client())?;
        builder.set_verify(SslVerifyMode::NONE);
        builder.set_fips_compliance_policy()?;
        let configuration = builder.build().configure()?;
        let mut tls_stream =
            tokio_boring::connect(configuration, server_name, initial_stream).await?;
        tls_stream.flush().await?;
        let cert = tls_stream
            .ssl()
            .peer_certificate()
            .ok_or_else(|| io::Error::new(io::ErrorKind::Other, "peer certificate is missing"))?;
        let public_key = cert.public_key()?;
        let bytes = public_key.public_key_to_der()?;
        Ok((tls_stream, bytes))
    }
    #[cfg(not(feature = "fips"))]
    ironrdp_tls::upgrade(initial_stream, server_name)
        .await
        .map_err(ClientError::from)
}
