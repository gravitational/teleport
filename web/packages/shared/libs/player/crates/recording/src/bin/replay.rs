//! Reads a Teleport desktop session recording (ProtoStream V1) and feeds each
//! TDPB/TDP frame through the codec crate.
//!
//! Usage: `cargo run -p recording --bin replay -- path/to/recording.tar`

use std::env;
use std::fs::File;
use std::io::BufReader;
use std::process::ExitCode;

use recording::{FrameKind, Reader};
use tracing_subscriber::EnvFilter;

const TDPB_HEADER_LEN: usize = 4;

fn main() -> ExitCode {
    tracing_subscriber::fmt()
        .with_env_filter(EnvFilter::try_from_default_env().unwrap_or_else(|_| "info".into()))
        .init();

    let Some(path) = env::args().nth(1) else {
        eprintln!("usage: replay <recording-file>");
        return ExitCode::from(2);
    };

    let file = match File::open(&path) {
        Ok(f) => f,
        Err(e) => {
            eprintln!("failed to open {path}: {e}");
            return ExitCode::from(1);
        }
    };
    let mut reader = Reader::new(BufReader::new(file));

    let mut tdpb_ok = 0usize;
    let mut tdpb_err = 0usize;
    let mut tdp_ok = 0usize;
    let mut tdp_err = 0usize;
    let mut index = 0usize;

    loop {
        let frame = match reader.next_frame() {
            Ok(Some(f)) => f,
            Ok(None) => break,
            Err(e) => {
                eprintln!("frame #{index}: reader error: {e}");
                return ExitCode::from(1);
            }
        };
        index += 1;
        match frame.kind {
            FrameKind::Tdpb(bytes) => {
                if bytes.len() < TDPB_HEADER_LEN {
                    eprintln!(
                        "frame #{index} (t+{}ms): TDPB blob too short ({} bytes)",
                        frame.delay_ms,
                        bytes.len()
                    );
                    tdpb_err += 1;
                    continue;
                }
                let claimed =
                    u32::from_be_bytes(bytes[..TDPB_HEADER_LEN].try_into().unwrap()) as usize;
                let body = bytes.slice(TDPB_HEADER_LEN..);
                if claimed != body.len() {
                    eprintln!(
                        "frame #{index} (t+{}ms): TDPB length mismatch: header says {claimed}, body is {} bytes",
                        frame.delay_ms,
                        body.len()
                    );
                    tdpb_err += 1;
                    continue;
                }
                let body_len = body.len();
                match codec::tdpb::decode::decode(body) {
                    Ok(_) => tdpb_ok += 1,
                    Err(e) => {
                        eprintln!(
                            "frame #{index} (t+{}ms, {body_len} bytes): TDPB decode error: {e}",
                            frame.delay_ms,
                        );
                        tdpb_err += 1;
                    }
                }
            }
            FrameKind::Tdp(bytes) => {
                let bytes_len = bytes.len();
                match codec::tdp::decode::decode(bytes) {
                    Ok(_) => tdp_ok += 1,
                    Err(e) => {
                        eprintln!(
                            "frame #{index} (t+{}ms, {bytes_len} bytes): TDP decode error: {e}",
                            frame.delay_ms,
                        );
                        tdp_err += 1;
                    }
                }
            }
        }
    }

    println!("frames seen: {index}");
    println!("  TDPB decoded ok:  {tdpb_ok}");
    println!("  TDPB decode err:  {tdpb_err}");
    println!("  TDP decoded ok:   {tdp_ok}");
    println!("  TDP decode err:   {tdp_err}");
    if tdpb_err > 0 || tdp_err > 0 {
        ExitCode::from(1)
    } else {
        ExitCode::SUCCESS
    }
}
