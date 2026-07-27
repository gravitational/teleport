//! Big-endian byte cursor for reading TDP wire-format messages. Owns the
//! source [`Bytes`] so payload slices can be returned by refcount instead of
//! by copy.

use crate::error::CodecError;
use bytes::Bytes;

pub struct Cursor {
    buf: Bytes,
    pos: usize,
}

impl Cursor {
    #[must_use]
    pub fn new(buf: Bytes) -> Self {
        Self { buf, pos: 0 }
    }

    #[must_use]
    pub fn remaining(&self) -> usize {
        self.buf.len().saturating_sub(self.pos)
    }

    fn take(&mut self, n: usize, field: &'static str) -> Result<(usize, usize), CodecError> {
        let end = self
            .pos
            .checked_add(n)
            .ok_or(CodecError::Truncated(field))?;
        if end > self.buf.len() {
            return Err(CodecError::Truncated(field));
        }
        let start = self.pos;
        self.pos = end;
        Ok((start, end))
    }

    fn slice(&mut self, n: usize, field: &'static str) -> Result<&[u8], CodecError> {
        let (s, e) = self.take(n, field)?;
        Ok(&self.buf[s..e])
    }

    pub fn u8(&mut self, field: &'static str) -> Result<u8, CodecError> {
        Ok(self.slice(1, field)?[0])
    }

    pub fn u16(&mut self, field: &'static str) -> Result<u16, CodecError> {
        Ok(u16::from_be_bytes(self.slice(2, field)?.try_into().unwrap()))
    }

    pub fn u32(&mut self, field: &'static str) -> Result<u32, CodecError> {
        Ok(u32::from_be_bytes(self.slice(4, field)?.try_into().unwrap()))
    }

    pub fn u64(&mut self, field: &'static str) -> Result<u64, CodecError> {
        Ok(u64::from_be_bytes(self.slice(8, field)?.try_into().unwrap()))
    }

    pub fn bytes(&mut self, n: usize, field: &'static str) -> Result<&[u8], CodecError> {
        self.slice(n, field)
    }

    /// Refcounted slice into the source buffer — no copy.
    pub fn bytes_owned(&mut self, n: usize, field: &'static str) -> Result<Bytes, CodecError> {
        let (s, e) = self.take(n, field)?;
        Ok(self.buf.slice(s..e))
    }

    /// Length-prefixed UTF-8 string: `u32 length || bytes`.
    pub fn string(&mut self, field: &'static str) -> Result<String, CodecError> {
        let len = self.u32(field)? as usize;
        let bytes = self.slice(len, field)?;
        std::str::from_utf8(bytes)
            .map(str::to_owned)
            .map_err(|_| CodecError::Invalid(crate::error::InvalidValue::Utf8))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn primitive_reads_advance_position_in_order() {
        let mut cur = Cursor::new(Bytes::from_static(&[
            0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A,
        ]));
        assert_eq!(cur.u8("a").unwrap(), 0x01);
        assert_eq!(cur.u16("b").unwrap(), 0x0203);
        assert_eq!(cur.u32("c").unwrap(), 0x0405_0607);
        assert_eq!(cur.remaining(), 3);
    }

    #[test]
    fn bytes_owned_shares_allocation_with_source() {
        // The whole point of `bytes_owned` is that decoded payloads borrow
        // the source allocation. If this regresses to a copy, playback hot
        // paths will silently start heap-allocating per frame.
        let source = Bytes::from_static(b"header-payload");
        let source_addr = source.as_ptr() as usize;
        let mut cur = Cursor::new(source);
        cur.bytes(6, "header").unwrap();
        let payload = cur.bytes_owned(8, "payload").unwrap();
        assert_eq!(&payload[..], b"-payload");
        let payload_addr = payload.as_ptr() as usize;
        // The returned `Bytes` sits at offset 6 inside the original
        // allocation; if it had been copied it would live anywhere else.
        assert_eq!(payload_addr - source_addr, 6);
    }

    #[test]
    fn take_past_buffer_returns_truncated() {
        let mut cur = Cursor::new(Bytes::from_static(&[0u8; 2]));
        assert!(matches!(
            cur.u32("field"),
            Err(CodecError::Truncated("field"))
        ));
    }

    #[test]
    fn take_with_overflowing_length_returns_truncated() {
        // A malicious wire payload could claim a length close to usize::MAX;
        // the cursor must reject it without panicking on integer overflow.
        let mut cur = Cursor::new(Bytes::from_static(&[0u8; 4]));
        cur.u16("preamble").unwrap();
        assert!(matches!(
            cur.bytes(usize::MAX, "huge"),
            Err(CodecError::Truncated("huge"))
        ));
    }

    #[test]
    fn invalid_utf8_string_surfaces_utf8_error() {
        let mut payload = vec![0, 0, 0, 2];
        payload.extend_from_slice(&[0xFF, 0xFE]);
        let mut cur = Cursor::new(Bytes::from(payload));
        assert!(matches!(
            cur.string("name"),
            Err(CodecError::Invalid(crate::error::InvalidValue::Utf8))
        ));
    }
}
