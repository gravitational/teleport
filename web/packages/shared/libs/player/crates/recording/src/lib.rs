//! Reader for Teleport ProtoStream V1 desktop session recordings.
//!
//! Recording layout (matches `lib/events/stream.go`):
//!
//! - One or more "parts" concatenated end-to-end. Each part:
//!   - 24-byte header (3× `u64` big-endian): `version`, `slice_size`, `padding_size`.
//!     `version == 1`. `slice_size` is the meaningful length of the gzipped body;
//!     `padding_size` is trailing padding to skip after the gzip blob.
//!   - `slice_size` bytes of gzip stream.
//!   - `padding_size` bytes of padding (ignored).
//! - Inside each decompressed part: a sequence of `(u32 BE record_len, record_len bytes)`
//!   where each record is an `events.OneOf` protobuf. The oneof field
//!   `desktop_recording` (tag #69) carries the TDP/TDPB frame we care about.
//!
//! We don't pull the full `events.proto` into Rust; we scan the wire format
//! for just the fields we need and yield a stream of [`Frame`]s.

use std::io::{self, Read};

use bytes::Bytes;
use flate2::read::GzDecoder;

pub mod wire;

/// A single TDP/TDPB frame extracted from a `DesktopRecording` audit event.
pub struct Frame {
    /// Milliseconds since the start of the session.
    pub delay_ms: i64,
    pub kind: FrameKind,
}

pub enum FrameKind {
    /// Legacy TDP wire-format bytes (`DesktopRecording.message`, field #2).
    Tdp(Bytes),
    /// TDPB wire-format bytes (`DesktopRecording.tdpb_message`, field #4):
    /// 4-byte big-endian length prefix followed by an `Envelope` protobuf —
    /// identical to the framing on a live TDPB connection.
    Tdpb(Bytes),
}

#[derive(Debug, thiserror::Error)]
pub enum Error {
    #[error("io error: {0}")]
    Io(#[from] io::Error),
    #[error("unsupported ProtoStream version: {0} (only V1 supported)")]
    UnsupportedVersion(u64),
    #[error("malformed protobuf wire format: {0}")]
    Wire(&'static str),
    #[error("record length {0} exceeds remaining part bytes {1}")]
    RecordOverflow(usize, usize),
}

/// Streams desktop frames out of a ProtoStream V1 recording. Audit events
/// without a `desktop_recording` payload (`SessionStart`/`SessionEnd`/etc.)
/// are skipped silently.
pub struct Reader<R: Read> {
    source: R,
    /// Decompressed body of the current part; frame payloads are issued as
    /// refcounted slices into it. Empty before the first part and at EOF.
    part: Bytes,
    pos: usize,
    eof: bool,
}

impl<R: Read> Reader<R> {
    pub fn new(source: R) -> Self {
        Self {
            source,
            part: Bytes::new(),
            pos: 0,
            eof: false,
        }
    }

    /// Returns the next desktop frame, or `Ok(None)` at clean EOF. Records
    /// that aren't desktop events are consumed but skipped.
    pub fn next_frame(&mut self) -> Result<Option<Frame>, Error> {
        loop {
            let Some(record) = self.next_record()? else {
                return Ok(None);
            };
            if let Some(frame) = extract_desktop_recording(&record)? {
                return Ok(Some(frame));
            }
        }
    }

    /// Returns the next length-prefixed `events.OneOf` body, refilling from
    /// the next part on demand. Body length is the u32 prefix; this method
    /// returns the bytes after it.
    fn next_record(&mut self) -> Result<Option<Bytes>, Error> {
        if self.pos >= self.part.len() && !self.load_next_part()? {
            return Ok(None);
        }
        let remaining = self.part.len() - self.pos;
        if remaining < 4 {
            return Err(Error::Wire("truncated record size prefix"));
        }
        let size =
            u32::from_be_bytes(self.part[self.pos..self.pos + 4].try_into().unwrap()) as usize;
        self.pos += 4;
        let body_end = self.pos + size;
        if body_end > self.part.len() {
            return Err(Error::RecordOverflow(size, self.part.len() - self.pos));
        }
        let body = self.part.slice(self.pos..body_end);
        self.pos = body_end;
        Ok(Some(body))
    }

    /// Pulls the next part's gzip blob into `self.part`. Returns `false` at
    /// clean EOF; a short read mid-header bubbles up as `UnexpectedEof`.
    fn load_next_part(&mut self) -> Result<bool, Error> {
        if self.eof {
            return Ok(false);
        }
        self.pos = 0;
        loop {
            let mut hdr = [0u8; 24];
            if !read_exact_or_eof(&mut self.source, &mut hdr)? {
                self.part = Bytes::new();
                self.eof = true;
                return Ok(false);
            }
            let version = u64::from_be_bytes(hdr[0..8].try_into().unwrap());
            let slice_size = u64::from_be_bytes(hdr[8..16].try_into().unwrap());
            let padding_size = u64::from_be_bytes(hdr[16..24].try_into().unwrap());
            if version != 1 {
                return Err(Error::UnsupportedVersion(version));
            }
            if slice_size == 0 {
                // Pure-padding part: skip and look for a real one.
                skip_bytes(&mut self.source, padding_size)?;
                continue;
            }
            let mut decompressed = Vec::new();
            {
                let mut gz = GzDecoder::new((&mut self.source).take(slice_size));
                gz.read_to_end(&mut decompressed)?;
            }
            skip_bytes(&mut self.source, padding_size)?;
            self.part = Bytes::from(decompressed);
            return Ok(true);
        }
    }
}

/// Like [`Read::read_exact`], but distinguishes a clean EOF (returning
/// `Ok(false)`) from a partial read (which surfaces as `UnexpectedEof`).
fn read_exact_or_eof<R: Read>(r: &mut R, buf: &mut [u8]) -> io::Result<bool> {
    let mut total = 0;
    while total < buf.len() {
        match r.read(&mut buf[total..])? {
            0 => {
                if total == 0 {
                    return Ok(false);
                }
                return Err(io::Error::new(
                    io::ErrorKind::UnexpectedEof,
                    "short read inside ProtoStream",
                ));
            }
            n => total += n,
        }
    }
    Ok(true)
}

fn skip_bytes<R: Read>(r: &mut R, n: u64) -> io::Result<()> {
    let mut remaining = n;
    let mut scratch = [0u8; 8192];
    while remaining > 0 {
        let want = remaining.min(scratch.len() as u64) as usize;
        let got = r.read(&mut scratch[..want])?;
        if got == 0 {
            return Err(io::Error::new(
                io::ErrorKind::UnexpectedEof,
                "EOF while skipping bytes",
            ));
        }
        remaining -= got as u64;
    }
    Ok(())
}

fn extract_desktop_recording(oneof: &Bytes) -> Result<Option<Frame>, Error> {
    let mut cur = wire::Cursor::new(oneof);
    while !cur.is_empty() {
        let (field, wt) = cur.read_tag()?;
        // events.OneOf.desktop_recording = 69
        if field == 69 && wt == wire::WireType::LengthDelimited {
            let (start, end) = cur.read_len_delimited_bounds()?;
            return parse_desktop_recording(&oneof.slice(start..end)).map(Some);
        }
        cur.skip_field(wt)?;
    }
    Ok(None)
}

fn parse_desktop_recording(record: &Bytes) -> Result<Frame, Error> {
    let mut cur = wire::Cursor::new(record);
    let mut tdp: Option<Bytes> = None;
    let mut tdpb: Option<Bytes> = None;
    let mut delay_ms: i64 = 0;
    while !cur.is_empty() {
        let (field, wt) = cur.read_tag()?;
        match (field, wt) {
            // DesktopRecording.message = 2 (legacy TDP)
            (2, wire::WireType::LengthDelimited) => {
                let (s, e) = cur.read_len_delimited_bounds()?;
                tdp = Some(record.slice(s..e));
            }
            // DesktopRecording.delay_milliseconds = 3
            (3, wire::WireType::Varint) => {
                delay_ms = cur.read_varint()? as i64;
            }
            // DesktopRecording.tdpb_message = 4
            (4, wire::WireType::LengthDelimited) => {
                let (s, e) = cur.read_len_delimited_bounds()?;
                tdpb = Some(record.slice(s..e));
            }
            (_, w) => cur.skip_field(w)?,
        }
    }
    let kind = match (tdpb, tdp) {
        (Some(b), _) => FrameKind::Tdpb(b),
        (None, Some(t)) => FrameKind::Tdp(t),
        (None, None) => return Err(Error::Wire("DesktopRecording has no payload")),
    };
    Ok(Frame { delay_ms, kind })
}

#[cfg(test)]
mod tests {
    use super::*;
    use flate2::write::GzEncoder;
    use flate2::Compression;
    use std::io::{Cursor, Write};

    fn write_varint(buf: &mut Vec<u8>, mut v: u64) {
        while v >= 0x80 {
            buf.push((v as u8) | 0x80);
            v >>= 7;
        }
        buf.push(v as u8);
    }

    fn write_tag(buf: &mut Vec<u8>, field: u32, wire_type: u8) {
        write_varint(buf, u64::from((field << 3) | u32::from(wire_type)));
    }

    fn write_length_delimited(buf: &mut Vec<u8>, field: u32, body: &[u8]) {
        write_tag(buf, field, 2);
        write_varint(buf, body.len() as u64);
        buf.extend_from_slice(body);
    }

    /// Builds an `events.OneOf` carrying a `DesktopRecording` with the given
    /// TDP body and delay. Returns the protobuf bytes (no outer framing).
    fn desktop_recording_oneof(tdp: &[u8], delay_ms: u64) -> Vec<u8> {
        let mut inner = Vec::new();
        write_length_delimited(&mut inner, 2, tdp); // DesktopRecording.message
        write_tag(&mut inner, 3, 0); // DesktopRecording.delay_milliseconds
        write_varint(&mut inner, delay_ms);

        let mut oneof = Vec::new();
        write_length_delimited(&mut oneof, 69, &inner); // OneOf.desktop_recording
        oneof
    }

    /// Wraps records into a single ProtoStream V1 part: gzip blob preceded by
    /// the 24-byte BE header `[version=1, slice_size, padding_size=0]`.
    fn protostream_part(records: &[Vec<u8>]) -> Vec<u8> {
        let mut framed = Vec::new();
        for r in records {
            framed.extend_from_slice(&(r.len() as u32).to_be_bytes());
            framed.extend_from_slice(r);
        }
        let mut gz = GzEncoder::new(Vec::new(), Compression::fast());
        gz.write_all(&framed).unwrap();
        let compressed = gz.finish().unwrap();

        let mut out = Vec::new();
        out.extend_from_slice(&1u64.to_be_bytes()); // version
        out.extend_from_slice(&(compressed.len() as u64).to_be_bytes()); // slice_size
        out.extend_from_slice(&0u64.to_be_bytes()); // padding_size
        out.extend_from_slice(&compressed);
        out
    }

    #[test]
    fn reader_yields_desktop_frame_and_then_eof() {
        let oneof = desktop_recording_oneof(b"hello-tdp", 250);
        let part = protostream_part(&[oneof]);
        let mut reader = Reader::new(Cursor::new(part));

        let frame = reader.next_frame().unwrap().expect("frame");
        assert_eq!(frame.delay_ms, 250);
        match frame.kind {
            FrameKind::Tdp(b) => assert_eq!(&b[..], b"hello-tdp"),
            FrameKind::Tdpb(_) => panic!("expected legacy TDP frame"),
        }
        assert!(reader.next_frame().unwrap().is_none());
    }

    #[test]
    fn reader_skips_records_that_arent_desktop_events() {
        // First record has a single varint field (`OneOf.session_start`-shaped
        // placeholder, field 1); the second is the real desktop frame.
        let mut non_desktop = Vec::new();
        write_tag(&mut non_desktop, 1, 0);
        write_varint(&mut non_desktop, 42);

        let real = desktop_recording_oneof(b"X", 7);
        let part = protostream_part(&[non_desktop, real]);
        let mut reader = Reader::new(Cursor::new(part));

        let frame = reader.next_frame().unwrap().expect("frame");
        assert_eq!(frame.delay_ms, 7);
        match frame.kind {
            FrameKind::Tdp(b) => assert_eq!(&b[..], b"X"),
            FrameKind::Tdpb(_) => panic!("expected legacy TDP frame"),
        }
    }

    #[test]
    fn unsupported_version_in_header_errors() {
        let mut bytes = Vec::new();
        bytes.extend_from_slice(&99u64.to_be_bytes());
        bytes.extend_from_slice(&0u64.to_be_bytes());
        bytes.extend_from_slice(&0u64.to_be_bytes());
        let mut reader = Reader::new(Cursor::new(bytes));
        assert!(matches!(
            reader.next_frame(),
            Err(Error::UnsupportedVersion(99)),
        ));
    }

    #[test]
    fn record_overrun_errors_without_panicking() {
        // Corrupt the size prefix to claim more bytes than the part contains.
        let oneof = desktop_recording_oneof(b"x", 0);
        let mut framed = Vec::new();
        framed.extend_from_slice(&(oneof.len() as u32 + 1024).to_be_bytes());
        framed.extend_from_slice(&oneof);
        let mut gz = GzEncoder::new(Vec::new(), Compression::fast());
        gz.write_all(&framed).unwrap();
        let compressed = gz.finish().unwrap();
        let mut bytes = Vec::new();
        bytes.extend_from_slice(&1u64.to_be_bytes());
        bytes.extend_from_slice(&(compressed.len() as u64).to_be_bytes());
        bytes.extend_from_slice(&0u64.to_be_bytes());
        bytes.extend_from_slice(&compressed);

        let mut reader = Reader::new(Cursor::new(bytes));
        assert!(matches!(reader.next_frame(), Err(Error::RecordOverflow(..))));
    }

    #[test]
    fn empty_input_is_clean_eof() {
        let mut reader = Reader::new(Cursor::new(Vec::new()));
        assert!(reader.next_frame().unwrap().is_none());
    }
}
