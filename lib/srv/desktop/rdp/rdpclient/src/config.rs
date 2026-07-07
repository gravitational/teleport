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

use crate::ipc::IpcClient;
use crate::license::GoLicenseCache;
use clap::Parser;
use ironrdp_connector::{Credentials, DesktopSize, SmartCardIdentity};
use ironrdp_displaycontrol::pdu::MonitorLayoutEntry;
use ironrdp_pdu::nego::NegoRequestData;
use ironrdp_pdu::rdp::capability_sets::{
    client_codecs_capabilities, BitmapCodecs, MajorPlatformType,
};
use ironrdp_pdu::rdp::client_info::{PerformanceFlags, TimezoneInfo};
use log::{debug, error};
use rand::{Rng, TryRngCore};
use std::path::PathBuf;
use std::sync::{Arc, Mutex};

/// Teleport RDP Client CLI arguments
#[derive(Parser, Debug)]
pub struct Args {
    /// The path of the IPC socket for communication
    #[clap(long)]
    ipc_socket: PathBuf,
    /// A target RDP server username
    #[clap(long)]
    username: String,
    /// An address of the target RDP server
    #[clap(long)]
    server_addr: String,
    /// An address of the target KDC server (optional)
    #[clap(long)]
    kdc_addr: Option<String>,
    /// The name of the target computer (optional)
    #[clap(long)]
    computer_name: Option<String>,
    /// The width of the RDP session screen in pixels
    #[clap(long)]
    screen_width: u16,
    /// The height of the RDP session screen in pixels
    #[clap(long)]
    screen_height: u16,
    /// The scale of the RDP session screen
    #[clap(long)]
    screen_scale: u16,
    /// Whether to allow clipboard sharing between the client and server
    #[clap(long)]
    allow_clipboard: bool,
    /// Whether to allow directory sharing between the client and server
    #[clap(long)]
    allow_directory_sharing: bool,
    /// Whether to show the desktop wallpaper
    #[clap(long)]
    show_desktop_wallpaper: bool,
    /// Whether the target RDP server is part of an Active Directory domain
    #[clap(long)]
    ad: bool,
    /// Whether to use Network Level Authentication
    #[clap(long)]
    nla: bool,
    /// The client ID to use for the RDP session
    #[clap(long, value_parser = parse_client_id)]
    client_id: [u32; 4],
    /// The keyboard layout to use for the RDP session
    #[clap(long)]
    keyboard_layout: u32,
}

fn parse_client_id(s: &str) -> Result<[u32; 4], String> {
    let parts: Vec<&str> = s.split(',').collect();

    if parts.len() != 4 {
        return Err("client id must have exactly 4 numbers".into());
    }

    let mut client_id = [0u32; 4];
    for (i, p) in parts.iter().enumerate() {
        client_id[i] = p
            .trim()
            .parse::<u32>()
            .map_err(|e| format!("failed to parse number '{}': {}", p, e))?;
    }

    Ok(client_id)
}

/// Teleport RDP Client configuration
pub struct Config {
    /// The path of the IPC socket for communication
    pub ipc_socket: PathBuf,
    /// A target RDP server username
    pub username: String,
    /// A PIN for the smartcard used for authentication
    pub scard_pin: String,
    /// An address of the target RDP server
    pub server_addr: String,
    /// An address of the target KDC server (optional)
    pub kdc_addr: Option<String>,
    /// The name of the target computer (optional)
    pub computer_name: Option<String>,
    /// The width of the RDP session screen in pixels
    pub screen_width: u16,
    /// The height of the RDP session screen in pixels
    pub screen_height: u16,
    /// The scale of the RDP session screen
    pub screen_scale: u16,
    /// Whether to allow directory sharing between the client and server
    pub allow_directory_sharing: bool,
    /// Whether to show the desktop wallpaper
    pub show_desktop_wallpaper: bool,
    /// Whether to allow clipboard sharing between the client and server
    pub allow_clipboard: bool,
    /// Whether the target RDP server is part of an Active Directory domain
    pub ad: bool,
    /// Whether to use Network Level Authentication
    pub nla: bool,
    /// The client ID to use for the RDP session
    pub client_id: [u32; 4],
    /// The keyboard layout to use for the RDP session
    pub keyboard_layout: u32,
}

impl Config {
    /// Parses the command-line arguments and returns the [`Config`].
    pub fn parse_args() -> Self {
        let args = Args::parse();

        // Generate a random 8-digit PIN for our smartcard.
        let pin = format!(
            "{:08}",
            rand::rngs::OsRng
                .unwrap_err()
                .random_range(0i32..=99999999i32)
        );

        Self {
            ipc_socket: args.ipc_socket,
            username: args.username,
            scard_pin: pin,
            server_addr: args.server_addr,
            kdc_addr: args.kdc_addr,
            computer_name: args.computer_name,
            screen_width: args.screen_width,
            screen_height: args.screen_height,
            screen_scale: args.screen_scale,
            allow_directory_sharing: args.allow_directory_sharing,
            show_desktop_wallpaper: args.show_desktop_wallpaper,
            allow_clipboard: args.allow_clipboard,
            ad: args.ad,
            nla: args.nla,
            client_id: args.client_id,
            keyboard_layout: args.keyboard_layout,
        }
    }
}

/// Creates an IronRDP connector configuration.
pub fn create_connector_config(
    config: &Config,
    certificate: Vec<u8>,
    private_key: Vec<u8>,
    ipc_client: IpcClient,
) -> ironrdp_connector::Config {
    let initial_width = config.screen_width as u32;
    let initial_height = config.screen_height as u32;
    let (width, height) = MonitorLayoutEntry::adjust_display_size(initial_width, initial_height);
    if width != initial_width || height != initial_height {
        debug!("Adjusted screen size to [{:?}x{:?}]", width, height);
    }
    ironrdp_connector::Config {
        desktop_size: DesktopSize {
            width: config.screen_width,
            height: config.screen_height,
        },
        enable_tls: true,
        enable_credssp: config.ad && config.nla,
        enable_audio_playback: false,
        timezone_info: TimezoneInfo::default(),
        credentials: Credentials::SmartCard {
            config: config.ad.then(|| SmartCardIdentity {
                csp_name: "Microsoft Base Smart Card Crypto Provider".to_string(),
                reader_name: "Teleport".to_string(),
                container_name: "".to_string(),
                certificate,
                private_key,
            }),
            pin: config.scard_pin.clone(),
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
        keyboard_layout: config.keyboard_layout,
        ime_file_name: "".to_string(),
        bitmap: Some(ironrdp_connector::BitmapConfig {
            lossy_compression: true,
            // Changing this to 16 gets us uncompressed bitmaps on machines configured like
            // https://github.com/Devolutions/IronRDP/blob/55d11a5000ebd474c2ddc294b8b3935554443112/README.md?plain=1#L17-L36
            color_depth: 32,
            // Try to configure the client to use remotefx only. This should never fail in practice, but just in
            // case we'll log an error and fall back to defaults.
            codecs: client_codecs_capabilities(&["remotefx"]).unwrap_or_else(|err| {
                error!("Failed to configure client for remotefx: {}", err);
                BitmapCodecs::default()
            }),
        }),
        dig_product_id: "".to_string(),
        // `client_dir` is apparently unimportant, however most RDP clients hardcode this value (including FreeRDP):
        // https://github.com/FreeRDP/FreeRDP/blob/4e24b966c86fdf494a782f0dfcfc43a057a2ea60/libfreerdp/core/settings.c#LL49C34-L49C70
        client_dir: "C:\\Windows\\System32\\mstscax.dll".to_string(),
        platform: MajorPlatformType::UNSPECIFIED,
        enable_server_pointer: true,
        autologon: true,
        pointer_software_rendering: false,
        // Send the username in the request cookie, which is sent in the initial connection request.
        // The RDP server ignores this value, but load balancers sitting in front of the server
        // can use it to implement persistence.
        request_data: Some(NegoRequestData::cookie(config.username.clone())),
        performance_flags: PerformanceFlags::default()
            | PerformanceFlags::DISABLE_CURSOR_SHADOW // this is required for pointer to work correctly in Windows 2019
            | if !config.show_desktop_wallpaper {
            PerformanceFlags::DISABLE_WALLPAPER
        } else {
            PerformanceFlags::empty()
        },
        // Per the RDP spec, values must be in [100, 500]. Clamp the client's
        // reported scale factor (devicePixelRatio * 100) to this range, defaulting
        // to 100 if not provided.
        desktop_scale_factor: config.screen_scale.clamp(100, 500) as u32,
        license_cache: Some(Arc::new(GoLicenseCache {
            ipc_client: Mutex::new(ipc_client),
        })),
        hardware_id: Some(config.client_id),
        alternate_shell: "".to_string(),
        work_dir: "".to_string(),
        compression_type: None,
        multitransport_flags: None,
    }
}
