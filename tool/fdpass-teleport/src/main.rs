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
            "{pkg_name} (version {version})\n\nusage: {argv0} <path to mux socket> <connection target>\n",
        );
        process::exit(libc::EXIT_FAILURE);
    }

    let mux_path = args.nth(1).ok_or_eyre("missing mux path")?;
    let mut target = args.next().ok_or_eyre("missing connection target")?;
    target.push("\0");

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
    mux_conn
        .write_all(target.as_bytes())
        .wrap_err("could not send connection target to mux")?;

    // logic lifted from OpenBSD's netcat fdpass code
    // (https://github.com/openbsd/src/blob/master/usr.bin/nc/netcat.c)
    loop {
        match socket::sendmsg::<()>(
            libc::STDOUT_FILENO,
            &[IoSlice::new(&[0])],
            &[ControlMessage::ScmRights(&[mux_conn.as_raw_fd()])],
            MsgFlags::empty(),
            None,
        ) {
            Ok(1) => break,
            Ok(s) => eyre::bail!("unexpected sendmsg return value {s}"),
            Err(Errno::EAGAIN | Errno::EINTR) => continue,
            Err(e) => Err(e).wrap_err("could not pass connection to stdout")?,
        };
    }

    // returning would close and deallocate things, and there's just no need for
    // that
    process::exit(libc::EXIT_SUCCESS);
}
