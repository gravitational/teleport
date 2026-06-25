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

use anyhow::{bail, Context as _};
use env_logger::Target;
use ironrdp_session::x224::DisconnectDescription;
use log::info;
use rdp_client::client::Client;
use rdp_client::config::Config;
use rdp_client::ipc::connect_ipc;
use tokio::signal::unix::{signal, SignalKind};
use tokio_util::sync::CancellationToken;

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    let config = Config::parse_args();

    // Initialize logger.
    env_logger::Builder::new()
        // Set the default log level to `Info`.
        .filter_level(log::LevelFilter::Info)
        // Configure the logger using the environment variables.
        .parse_default_env()
        .target(Target::Stdout)
        .try_init()
        .context("failed to initialize logger")?;

    let cancellation_token = CancellationToken::new();

    // Watch for SIGTERM or SIGINT.
    let mut sigterm = signal(SignalKind::terminate())?;
    let mut sigint = signal(SignalKind::interrupt())?;
    let cancellation_token_clone = cancellation_token.clone();

    tokio::spawn(async move {
        tokio::select! {
            _ = sigterm.recv() => {}
            _ = sigint.recv() => {}
        }
        cancellation_token_clone.cancel();
    });

    info!("Rust RDP client started");

    let (ipc_tdpb_sender, ipc_tdpb_stream, ipc_client) = connect_ipc(&config.ipc_socket).await?;

    match Client::run(
        config,
        ipc_client,
        ipc_tdpb_stream,
        ipc_tdpb_sender,
        cancellation_token,
    )
    .await
    {
        // Use stderr to report the disconnect or failure reason to the Go process.
        Ok(Some(disconnect_description)) => {
            let disconnect_desc = match disconnect_description {
                DisconnectDescription::McsDisconnect(reason) => reason.description(),
                DisconnectDescription::ErrorInfo(info) => &info.description(),
            };
            eprintln!("RDP client disconnected gracefully: {disconnect_desc}");
        }
        Ok(None) => eprintln!("RDP client disconnected gracefully"),
        Err(err) => {
            // Error will be printed to `stderr`.
            bail!("{err}");
        }
    };

    Ok(())
}
