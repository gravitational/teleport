//! Minimal protobuf wire-format reader. We use this to fish a single field
//! (`events.OneOf.desktop_recording`) out of an audit event without pulling
//! the entire `legacy/events.proto` into the Rust build.

use crate::Error;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum WireType {
    Varint,
    Fixed64,
    LengthDelimited,
    StartGroup,
    EndGroup,
    Fixed32,
}

impl WireType {
    fn from_u8(v: u8) -> Result<Self, Error> {
        match v {
            0 => Ok(Self::Varint),
            1 => Ok(Self::Fixed64),
            2 => Ok(Self::LengthDelimited),
            3 => Ok(Self::StartGroup),
            4 => Ok(Self::EndGroup),
            5 => Ok(Self::Fixed32),
            _ => Err(Error::Wire("unknown wire type")),
        }
    }
}

pub struct Cursor<'a> {
    buf: &'a [u8],
    pos: usize,
}

impl<'a> Cursor<'a> {
    pub fn new(buf: &'a [u8]) -> Self {
        Self { buf, pos: 0 }
    }

    pub fn is_empty(&self) -> bool {
        self.pos >= self.buf.len()
    }

    pub fn read_varint(&mut self) -> Result<u64, Error> {
        let mut result: u64 = 0;
        let mut shift = 0;
        loop {
            if self.pos >= self.buf.len() {
                return Err(Error::Wire("truncated varint"));
            }
            let b = self.buf[self.pos];
            self.pos += 1;
            result |= u64::from(b & 0x7f) << shift;
            if b & 0x80 == 0 {
                return Ok(result);
            }
            shift += 7;
            if shift >= 64 {
                return Err(Error::Wire("varint overflow"));
            }
        }
    }

    pub fn read_tag(&mut self) -> Result<(u32, WireType), Error> {
        let tag = self.read_varint()?;
        let field = (tag >> 3) as u32;
        let wt = WireType::from_u8((tag & 0x07) as u8)?;
        if field == 0 {
            return Err(Error::Wire("invalid field number 0"));
        }
        Ok((field, wt))
    }

    /// Returns the (start, end) byte offsets of the payload within the
    /// cursor's source buffer; the caller can then borrow `&buf[start..end]`
    /// or, when source is a `Bytes`, slice it refcount-cheaply.
    pub fn read_len_delimited_bounds(&mut self) -> Result<(usize, usize), Error> {
        let len = self.read_varint()? as usize;
        if self.pos + len > self.buf.len() {
            return Err(Error::Wire("truncated length-delimited field"));
        }
        let start = self.pos;
        let end = start + len;
        self.pos = end;
        Ok((start, end))
    }

    pub fn skip_field(&mut self, wt: WireType) -> Result<(), Error> {
        match wt {
            WireType::Varint => {
                self.read_varint()?;
            }
            WireType::Fixed64 => self.advance(8)?,
            WireType::LengthDelimited => {
                let _ = self.read_len_delimited_bounds()?;
            }
            WireType::Fixed32 => self.advance(4)?,
            WireType::StartGroup | WireType::EndGroup => {
                return Err(Error::Wire("proto2 groups not supported"));
            }
        }
        Ok(())
    }

    fn advance(&mut self, n: usize) -> Result<(), Error> {
        if self.pos + n > self.buf.len() {
            return Err(Error::Wire("truncated fixed-size field"));
        }
        self.pos += n;
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn varint_decode_two_byte_value() {
        // 150 = 0x96 → 0b10010110 0b00000001 in proto varint encoding.
        let mut c = Cursor::new(&[0x96, 0x01]);
        assert_eq!(c.read_varint().unwrap(), 150);
    }

    #[test]
    fn varint_overflow_rejected() {
        // Ten continuation bytes exceed u64; the reader must error rather
        // than truncate silently.
        let bytes = [0xFFu8; 11];
        let mut c = Cursor::new(&bytes);
        assert!(c.read_varint().is_err());
    }

    #[test]
    fn read_tag_decodes_desktop_recording_field() {
        // events.OneOf.desktop_recording = 69, wire type 2 (length-delimited).
        // (69 << 3) | 2 = 554 → varint bytes 0xAA, 0x04. If this assertion
        // ever fires the field constant elsewhere in the crate is wrong.
        let mut c = Cursor::new(&[0xAA, 0x04]);
        let (field, wt) = c.read_tag().unwrap();
        assert_eq!(field, 69);
        assert_eq!(wt, WireType::LengthDelimited);
    }

    #[test]
    fn read_tag_rejects_field_zero() {
        // Field number 0 is reserved; an audit-event source that emits one
        // is corrupt and should not be silently consumed.
        let mut c = Cursor::new(&[0x02]); // tag = (0 << 3) | 2
        assert!(c.read_tag().is_err());
    }

    #[test]
    fn skip_field_consumes_each_wire_type() {
        // varint (0x08, 0x01) then fixed64 (0x09 + 8 bytes) then
        // length-delimited (0x0A, 0x02, 'a', 'b') then fixed32 (0x0D + 4 bytes).
        let bytes = [
            0x08, 0x01, // field 1, varint = 1
            0x11, 0, 0, 0, 0, 0, 0, 0, 0, // field 2, fixed64
            0x1A, 0x02, b'a', b'b', // field 3, length-delimited
            0x25, 0, 0, 0, 0, // field 4, fixed32
        ];
        let mut c = Cursor::new(&bytes);
        for _ in 0..4 {
            let (_, wt) = c.read_tag().unwrap();
            c.skip_field(wt).unwrap();
        }
        assert!(c.is_empty());
    }

    #[test]
    fn skip_field_rejects_proto2_groups() {
        // Group wire types (3, 4) were retired in proto3; treat them as an
        // error rather than crash on the unsupported encoding.
        let mut c = Cursor::new(&[0x0B]); // field 1, StartGroup
        let (_, wt) = c.read_tag().unwrap();
        assert!(c.skip_field(wt).is_err());
    }

    #[test]
    fn len_delimited_bounds_rejects_overrun() {
        // Length claims 10 bytes when only 2 follow; must error not panic.
        let mut c = Cursor::new(&[0x0A]); // varint length = 10
        assert!(c.read_len_delimited_bounds().is_err());
    }
}
