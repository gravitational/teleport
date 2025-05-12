// Teleport
// Copyright (C) 2025  Gravitational, Inc.
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

use crate::client::{
    global, ClientError, ClientFunction, ClientHandle, ClientResult, FunctionReceiver, TOKIO_RT,
};
use crate::util::from_c_string;
use crate::{
    cgo_handle_rdp_connection_activated, cgo_handle_x11_update, CGOConnectParams, CGOErrCode,
    CGOPointerButton, CGOPointerWheel, CGOResult, CgoHandle,
};
use log::{debug, info, trace};
use rustix::io::Errno;
use std::fs::File;
use std::io::Write;
use std::os::fd::AsRawFd;
use std::process::{Command, Stdio};
use std::str::Utf8Error;
use std::sync::Arc;
use std::thread::sleep;
use std::time::{Duration, Instant};
use std::{ptr, thread};
use x11rb::connection::Connection;
use x11rb::errors::{ConnectError, ConnectionError, ReplyError, ReplyOrIdError};
use x11rb::image::Image;
use x11rb::protocol::damage::{ConnectionExt, DamageWrapper, ReportLevel};
use x11rb::protocol::randr::{ConnectionExt as _, ModeInfo, Rotation};
use x11rb::protocol::xfixes;
use x11rb::protocol::xfixes::{ConnectionExt as _, RegionWrapper, SelectionEventMask};
use x11rb::protocol::xproto::{ConnectionExt as _, CreateWindowAux, EventMask, WindowClass};
use x11rb::protocol::xtest::ConnectionExt as _;
use x11rb::protocol::Event::{DamageNotify, SelectionNotify, XfixesSelectionNotify};
use x11rb::{COPY_DEPTH_FROM_PARENT, CURRENT_TIME};

impl From<Errno> for ClientError {
    fn from(value: Errno) -> Self {
        ClientError::InternalError(value.to_string())
    }
}

impl From<ConnectError> for ClientError {
    fn from(value: ConnectError) -> Self {
        ClientError::InternalError(value.to_string())
    }
}

impl From<ConnectionError> for ClientError {
    fn from(value: ConnectionError) -> Self {
        ClientError::InternalError(value.to_string())
    }
}

impl From<Utf8Error> for ClientError {
    fn from(value: Utf8Error) -> Self {
        ClientError::InternalError(value.to_string())
    }
}

impl From<ReplyError> for ClientError {
    fn from(value: ReplyError) -> Self {
        ClientError::InternalError(value.to_string())
    }
}

impl From<ReplyOrIdError> for ClientError {
    fn from(value: ReplyOrIdError) -> Self {
        ClientError::InternalError(value.to_string())
    }
}

#[no_mangle]
pub unsafe extern "C" fn local_client_run(
    cgo_handle: CgoHandle,
    params: CGOConnectParams,
) -> CGOResult {
    trace!("local_client_run");
    // Convert from C to Rust types.
    let username = from_c_string(params.go_username);

    let (client_handle, mut function_receiver) = ClientHandle::new(100);
    global::CLIENT_HANDLES.insert(cgo_handle, client_handle);
    cgo_handle_rdp_connection_activated(
        cgo_handle,
        1,
        1,
        params.screen_width,
        params.screen_height,
    );
    trace!("handle {} inserted", cgo_handle);

    run(cgo_handle, params, username, &mut function_receiver);

    CGOResult {
        err_code: CGOErrCode::ErrCodeSuccess,
        message: ptr::null_mut(),
    }
}

unsafe fn run(
    cgo_handle: CgoHandle,
    params: CGOConnectParams,
    username: String,
    function_receiver: &mut FunctionReceiver,
) -> ClientResult<()> {
    info!("Starting Xvfb");
    let (rd, wr) = rustix::pipe::pipe()?;
    let xvfb = Command::new("Xvfb")
        .args([
            "-displayfd",
            &format!("{}", wr.as_raw_fd()),
            "-screen",
            "0",
            // &format!("{}x{}x24", WIDTH, HEIGHT),
            "8192x8192x24",
            "-nolisten",
            "tcp",
            "-iglx",
        ])
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .spawn()?;
    drop(wr);
    let mut buf = [0u8; 64];
    let n = rustix::io::read(rd, &mut buf)?;
    let display = &format!(":{}", std::str::from_utf8(buf[..n].trim_ascii_end())?);
    info!("Starting xfce {}", display);
    #[cfg(target_os = "macos")]
    let xfce = Command::new("env")
        .args([&format!("DISPLAY={}", display), "xterm"])
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .spawn()?;
    #[cfg(target_os = "linux")]
    let xfce = Command::new("su")
        .args([
            "-c",
            &format!("env DISPLAY={} startxfce4", display),
            "-",
            &username,
        ])
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .spawn()?;
    sleep(Duration::from_millis(100));
    info!("Connecting to X11");
    let (x11, _) = x11rb::connect(Some(display))?;

    let root = &x11.setup().roots[0];
    let clipboard = x11.intern_atom(false, b"CLIPBOARD")?.reply()?.atom;
    let xsel_data = x11.intern_atom(false, b"XSEL_DATA")?.reply()?.atom;
    let utf8_string = x11.intern_atom(false, b"UTF8_STRING")?.reply()?.atom;
    let xfixes_version = x11.xfixes_query_version(5, 0)?.reply()?;
    let xdamage_version = x11.damage_query_version(1, 1)?.reply()?;
    let xtest_version = x11.xtest_get_version(10, 0)?.reply()?;
    let randr_version = x11.randr_query_version(10, 0)?.reply()?;
    info!("xfixes_version {:?}", xfixes_version);
    info!("xdamage_version {:?}", xdamage_version);
    info!("xtest_version {:?}", xtest_version);
    info!("randr_version {:?}", randr_version);
    x11.xfixes_select_selection_input(
        root.root,
        clipboard,
        SelectionEventMask::SET_SELECTION_OWNER,
    )?;
    let f = format!("{}x{}", params.screen_width, params.screen_height);
    info!("mode name {}", f);
    let mode_name = f.as_bytes();
    let mode = x11
        .randr_create_mode(
            root.root,
            ModeInfo {
                id: x11.generate_id()?,
                width: params.screen_width,
                height: params.screen_height,
                name_len: mode_name.len() as u16,
                ..Default::default()
            },
            mode_name,
        )?
        .reply()?;
    info!("mode {:?}", mode);
    let resources = x11.randr_get_screen_resources(root.root)?.reply()?;
    info!("outputs {:?}", resources.outputs);
    x11.randr_add_output_mode(resources.outputs[0], mode.mode)?;
    let screen_info = x11.randr_get_screen_info(root.root)?.reply()?;
    info!("screen_info {:?}", screen_info);
    let sscr = x11
        .randr_set_screen_config(
            root.root,
            CURRENT_TIME,
            screen_info.config_timestamp,
            1,
            Rotation::ROTATE0,
            0,
        )?
        .reply()?;
    info!("sscr {:?}", sscr);
    let damage = DamageWrapper::create(&x11, root.root, ReportLevel::DELTA_RECTANGLES)?;
    let win = x11.generate_id()?;
    x11.create_window(
        COPY_DEPTH_FROM_PARENT,
        win,
        root.root,
        0,
        0,
        1,
        1,
        0,
        WindowClass::INPUT_OUTPUT,
        root.root_visual,
        &CreateWindowAux::new().event_mask(EventMask::PROPERTY_CHANGE),
    )?;
    let mut screen =
        vec![0u8; (params.screen_width as usize) * (params.screen_height as usize) * 4];
    let region = RegionWrapper::create_region(&x11, &[])?;
    loop {
        match x11.poll_for_event()? {
            Some(SelectionNotify(event)) => {
                info!("got selection notify {:?}", event);
                let prop = x11
                    .get_property(false, win, xsel_data, utf8_string, 0, 50000)?
                    .reply()?;
                info!("prop: {:?}", prop);
            }
            Some(XfixesSelectionNotify(event)) => {
                info!("got xfixes selection notify {:?}", event);
                x11.convert_selection(win, event.selection, utf8_string, xsel_data, CURRENT_TIME)?;
            }
            Some(DamageNotify(event)) => {}
            None => {
                if let Some(cf) = TOKIO_RT.block_on(function_receiver.try_recv()) {
                    match cf {
                        ClientFunction::WriteRdpPointer(ev) => {
                            if ev.wheel != CGOPointerWheel::PointerWheelNone {
                                let detail = if ev.wheel_delta > 0 { 4 } else { 5 };
                                x11.xtest_fake_input(4, detail, 0, root.root, 0, 0, 0)?;
                                x11.xtest_fake_input(5, detail, 0, root.root, 0, 0, 0)?;
                            } else {
                                let detail = match ev.button {
                                    CGOPointerButton::PointerButtonNone => 0,
                                    CGOPointerButton::PointerButtonLeft => 1,
                                    CGOPointerButton::PointerButtonMiddle => 2,
                                    CGOPointerButton::PointerButtonRight => 3,
                                };
                                let event_type = match ev.button {
                                    CGOPointerButton::PointerButtonNone => 6,
                                    _ if ev.down => 4,
                                    _ => 5,
                                };
                                x11.xtest_fake_input(
                                    event_type,
                                    detail,
                                    0,
                                    root.root,
                                    ev.x as i16,
                                    ev.y as i16,
                                    0,
                                )?;
                            }
                        }
                        ClientFunction::WriteRdpKey(ev) => {
                            let event_type = if ev.down { 2 } else { 3 };
                            x11.xtest_fake_input(
                                event_type,
                                ev.code as u8 + 8,
                                0,
                                root.root,
                                0,
                                0,
                                0,
                            )?;
                        }
                        ClientFunction::WriteScreenResize(width, height) => {}
                        ClientFunction::Stop => return Ok(()),
                        cf => {
                            debug!("Client function {:?}", cf);
                        }
                    }
                }
                sleep(Duration::from_millis(40));
                x11.damage_subtract(damage.damage(), 0u32, region.region())?;
                let rects = xfixes::fetch_region(&x11, region.region())?.reply()?;
                for area in rects.rectangles {
                    let (image, _) =
                        Image::get(&x11, root.root, area.x, area.y, area.width, area.height)?;
                    let width = area.width as usize;
                    let height = area.height as usize;
                    let sw = params.screen_width as usize;
                    let x = area.x as usize;
                    let y = area.y as usize;
                    let mut diff = Vec::with_capacity(width * height * 4);
                    let mut rows = 0;
                    let mut start = None;
                    for (i, data) in image.data().chunks_exact(4 * width).enumerate() {
                        let mut row =
                            &mut screen[((i + y) * sw + x) * 4..((i + y) * sw + x + width) * 4];
                        if row != data {
                            diff.extend_from_slice(data);
                            row.copy_from_slice(data);
                            rows += 1u16;
                            if start.is_none() {
                                start = Some(y + i);
                            }
                        } else if let Some(start_row) = start {
                            let mut encoded = Vec::with_capacity(4 * size_of::<u16>() + diff.len());
                            encoded.extend_from_slice(&area.x.to_be_bytes());
                            encoded.extend_from_slice(&(start_row as u16).to_be_bytes());
                            encoded.extend_from_slice(&area.width.to_be_bytes());
                            encoded.extend_from_slice(&rows.to_be_bytes());
                            // encode(&mut encoded, &diff);
                            cgo_handle_x11_update(
                                cgo_handle,
                                encoded.as_mut_ptr(),
                                encoded.len() as u32,
                            );
                            diff = Vec::with_capacity(width * height * 4);
                            start = None;
                            rows = 0;
                        }
                    }
                    if let Some(start_row) = start {
                        let mut encoded = Vec::with_capacity(4 * size_of::<u16>() + diff.len());
                        encoded.extend_from_slice(&area.x.to_be_bytes());
                        encoded.extend_from_slice(&(start_row as u16).to_be_bytes());
                        encoded.extend_from_slice(&area.width.to_be_bytes());
                        encoded.extend_from_slice(&rows.to_be_bytes());
                        // encode(&mut encoded, &diff);
                        cgo_handle_x11_update(
                            cgo_handle,
                            encoded.as_mut_ptr(),
                            encoded.len() as u32,
                        );
                    }
                }
            }
            event => {
                info!("unknown event {:?}", event);
            }
        }
    }
}
