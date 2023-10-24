/*
 *
 * Copyright 2021 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * /
 */

use boring::ssl::{SslConnector, SslMethod, SslVerifyMode};
use std::io;
use tokio::io::AsyncWriteExt;
use tokio::net::TcpStream;
use tokio_boring::SslStream;

use crate::client::ClientResult;

pub(crate) async fn upgrade(
    initial_stream: TcpStream,
    server_name: &str,
) -> ClientResult<(SslStream<TcpStream>, Vec<u8>)> {
    let mut builder = SslConnector::builder(SslMethod::tls_client())?;
    builder.set_verify(SslVerifyMode::NONE);
    let configuration = builder.build().configure()?;
    let mut tls_stream = tokio_boring::connect(configuration, server_name, initial_stream).await?;
    tls_stream.flush().await?;
    let cert = tls_stream
        .ssl()
        .peer_certificate()
        .ok_or_else(|| io::Error::new(io::ErrorKind::Other, "peer certificate is missing"))?;
    let public_key = cert.public_key()?;
    let bytes = public_key.public_key_to_der()?;
    Ok((tls_stream, bytes))
}
