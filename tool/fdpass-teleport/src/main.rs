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

use nix::{
    errno::Errno,
    fcntl::{self, OFlag},
    libc,
    sys::socket::{self, ControlMessage, MsgFlags},
};
use simple_eyre::eyre::{self, OptionExt, Result, WrapErr};
use std::{
    env,
    io::{IoSlice, Write},
    os::{
        fd::AsRawFd,
        unix::{ffi::OsStrExt, net::UnixStream},
    },
    process,
};

fn main() -> Result<()> {
    simple_eyre::install()?;

    let mut args = env::args_os();

    if args.len() != 3 {
        let pkg_name = env!("CARGO_PKG_NAME");
        let version = option_env!("VERSION").unwrap_or("unknown");

        let argv0 = args.next();
        let argv0 = match &argv0 {
            Some(o) => o.to_string_lossy(),
            None => pkg_name.into(),
        };

        eprint!(
            "{pkg_name} (version {version})\n\nusage: {argv0} <path to mux socket> <connection target>\n\n Find out more at https://goteleport.com/docs/reference/cli/fdpass-teleport\n",
        );
        process::exit(libc::EXIT_FAILURE);
    }

    let mux_path = args.nth(1).ok_or_eyre("missing mux path")?;
    let target = args.next().ok_or_eyre("missing connection target")?;

    // in OpenSSH ProxyCommand+ProxyUseFdPass the program is executed with a
    // unix domain socket as stdout, which we can check with a getsockname();
    // we'll get a ENOTSOCK if stdout is not a socket, or EINVAL if the address
    // returned by getsockname() is not AF_UNIX (i.e. stdout is a socket but not
    // a unix domain socket)
    socket::getsockname::<socket::UnixAddr>(libc::STDOUT_FILENO)
        .wrap_err("stdout is not a unix socket")?;

    // to not have to bother with poll() for the later sendmsg() we have to
    // confirm that the socket is set to blocking mode, or we might end up
    // busylooping with EAGAIN; OpenSSH at the time of writing (9.7) will give
    // the ProxyCommand a blocking socketpair() half, so we should be good
    let fl = fcntl::fcntl(libc::STDOUT_FILENO, fcntl::F_GETFL)
        .wrap_err("could not check stdout for blocking mode")?;
    let fl = OFlag::from_bits_retain(fl);
    if fl.contains(OFlag::O_NONBLOCK) {
        let mut fl = fl;
        fl.set(OFlag::O_NONBLOCK, false);
        fcntl::fcntl(libc::STDOUT_FILENO, fcntl::F_SETFL(fl))
            .wrap_err("could not set stdout to blocking mode")?;
    }

    let mut mux_conn = UnixStream::connect(mux_path).wrap_err("could not connect to mux")?;

    let mut target = target;
    target.push("\0");
    let mut buf = target.as_bytes();

    // pass the current stderr to tbot, so it can be used to output dial errors
    loop {
        match socket::sendmsg::<()>(
            mux_conn.as_raw_fd(),
            &[IoSlice::new(buf)],
            &[ControlMessage::ScmRights(&[libc::STDERR_FILENO])],
            MsgFlags::empty(),
            None,
        ) {
            Err(Errno::EINTR) => continue,
            Ok(0) => eyre::bail!("unexpected sendmsg return value 0"),
            Ok(n) => {
                buf = &buf[n..];
                break;
            }
            Err(e) => {
                return Err(e).wrap_err("could not send connection target to mux");
            }
        };
    }
    // in all likelyhood the target fit in the socket buffer already, but we
    // can't assume that, so here we send the rest of the unsent target (if any)
    mux_conn
        .write_all(buf)
        .wrap_err("could not send connection target to mux")?;

    // we can now pass the connection to OpenSSH (or whoever launched us) over
    // stdout, sending a byte of actual data together with the connection's file
    // descriptor; logic lifted from OpenBSD's netcat fdpass code
    // (https://github.com/openbsd/src/blob/master/usr.bin/nc/netcat.c)
    loop {
        match socket::sendmsg::<()>(
            libc::STDOUT_FILENO,
            &[IoSlice::new(&[0])],
            &[ControlMessage::ScmRights(&[mux_conn.as_raw_fd()])],
            MsgFlags::empty(),
            None,
        ) {
            Err(Errno::EINTR) => continue,
            Ok(1) => break,
            Ok(s) => eyre::bail!("unexpected sendmsg return value {s}"),
            Err(e) => {
                return Err(e).wrap_err("could not pass connection to stdout");
            }
        };
    }

    // returning would close and deallocate things, and there's just no need for
    // that
    process::exit(libc::EXIT_SUCCESS);
}
