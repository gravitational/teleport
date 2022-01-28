// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

use crate::errors::{invalid_data_error, NTSTATUS_OK, SPECIAL_NO_RESPONSE};
use crate::piv;
use crate::Payload;
use bitflags::bitflags;
use byteorder::{LittleEndian, ReadBytesExt, WriteBytesExt};
use iso7816::command::Command as CardCommand;
use num_traits::{FromPrimitive, ToPrimitive};
use rdp::model::data::Message;
use rdp::model::error::*;
use std::char::{decode_utf16, REPLACEMENT_CHARACTER};
use std::collections::HashMap;
use std::convert::TryInto;
use std::io::{Read, Write};
use uuid::Uuid;

// Client implements the smartcard emulator, forwarded over an RDP virtual channel.
// Spec: https://winprotocoldoc.blob.core.windows.net/productionwindowsarchives/MS-RDPESC/%5bMS-RDPESC%5d.pdf
//
// This emulator always reports a single card reader with a single active card called "Teleport".
pub struct Client {
    // contexts holds all the active contexts for the server, established using
    // SCARD_IOCTL_ESTABLISHCONTEXT. Some IOCTLs are context-specific and pass it as argument.
    //
    // contexts also holds a cache and connected smartcard handles for each context.
    contexts: Contexts,
    uuid: Uuid,
    cert_der: Vec<u8>,
    key_der: Vec<u8>,
}

impl Client {
    pub fn new(cert_der: Vec<u8>, key_der: Vec<u8>) -> Self {
        Self {
            contexts: Contexts::new(),
            uuid: Uuid::new_v4(),
            cert_der,
            key_der,
        }
    }

    // ioctl handles messages coming from the RDP server over the RDPDR channel.
    pub fn ioctl(&mut self, code: u32, input: &mut Payload) -> RdpResult<(u32, Vec<u8>)> {
        let code = IoctlCode::from_u32(code).ok_or_else(|| {
            invalid_data_error(&format!("invalid I/O control code value {:#010x}", code))
        })?;

        debug!("got IoctlCode {:?}", &code);
        // Note: this is an incomplete implementation of the scard API.
        // It's the bare minimum needed to make RDP authentication using a smartcard work.
        //
        // Particularly, we only implement the Unicode IOCTL variants. All Ascii variants will
        // fail, but most modern Windows hosts shouldn't call those. If you're reading this because
        // some SCARD_IOCTL_*A call is failing, I was wrong and you'll have to implement the Ascii
        // calls.
        let resp = match code {
            IoctlCode::SCARD_IOCTL_ACCESSSTARTEDEVENT => self.handle_access_started_event(input),
            IoctlCode::SCARD_IOCTL_ESTABLISHCONTEXT => self.handle_establish_context(input),
            IoctlCode::SCARD_IOCTL_RELEASECONTEXT => self.handle_release_context(input),
            IoctlCode::SCARD_IOCTL_CANCEL => self.handle_cancel(input),
            IoctlCode::SCARD_IOCTL_ISVALIDCONTEXT => self.handle_is_valid_context(input),
            IoctlCode::SCARD_IOCTL_LISTREADERSW => self.handle_list_readers(input),
            IoctlCode::SCARD_IOCTL_GETSTATUSCHANGEW => self.handle_get_status_change(input),
            IoctlCode::SCARD_IOCTL_CONNECTW => self.handle_connect(input),
            IoctlCode::SCARD_IOCTL_DISCONNECT => self.handle_disconnect(input),
            IoctlCode::SCARD_IOCTL_BEGINTRANSACTION => self.handle_begin_transaction(input),
            IoctlCode::SCARD_IOCTL_ENDTRANSACTION => self.handle_end_transaction(input),
            IoctlCode::SCARD_IOCTL_STATUSA => self.handle_status(input, StringEncoding::Ascii),
            IoctlCode::SCARD_IOCTL_STATUSW => self.handle_status(input, StringEncoding::Unicode),
            // Transmit is where communication with the actual smartcard (and the PIV application
            // on it) happens. All other messages are managing the smartcard reader and
            // establishing a connection to the smartcard.
            IoctlCode::SCARD_IOCTL_TRANSMIT => self.handle_transmit(input),
            IoctlCode::SCARD_IOCTL_GETDEVICETYPEID => self.handle_get_device_type_id(input),
            // Note: we keep an in-memory hashmap as a cache to implement these commands. Windows
            // doesn't seem to like a smartcard without a functioning cache.
            IoctlCode::SCARD_IOCTL_READCACHEW => self.handle_read_cache(input),
            IoctlCode::SCARD_IOCTL_WRITECACHEW => self.handle_write_cache(input),
            IoctlCode::SCARD_IOCTL_GETREADERICON => self.handle_get_reader_icon(input),
            _ => {
                warn!("unimplemented IOCTL: {:?}", code);
                let resp = Long_Return::new(ReturnCode::SCARD_F_INTERNAL_ERROR);
                debug!("sending {:?}", resp);
                Ok(Some(resp.encode()?))
            }
        }?;

        if let Some(resp) = resp {
            Ok((NTSTATUS_OK, encode_response(resp)?))
        } else {
            Ok((SPECIAL_NO_RESPONSE, vec![]))
        }
    }

    fn handle_access_started_event(&self, input: &mut Payload) -> RdpResult<Option<Vec<u8>>> {
        let req = ScardAccessStartedEvent_Call::decode(input)?;
        debug!("got {:?}", req);
        let resp = Long_Return::new(ReturnCode::SCARD_S_SUCCESS);
        debug!("sending {:?}", resp);
        Ok(Some(resp.encode()?))
    }

    fn handle_establish_context(&mut self, input: &mut Payload) -> RdpResult<Option<Vec<u8>>> {
        let req = EstablishContext_Call::decode(input)?;
        debug!("got {:?}", req);
        let ctx = self.contexts.establish();
        let resp = EstablishContext_Return::new(ReturnCode::SCARD_S_SUCCESS, ctx);
        debug!("sending {:?}", resp);
        Ok(Some(resp.encode()?))
    }

    fn handle_release_context(&mut self, input: &mut Payload) -> RdpResult<Option<Vec<u8>>> {
        let req = Context_Call::decode(input)?;
        debug!("got {:?}", req);
        self.contexts.release(req.context.value);
        let resp = Long_Return::new(ReturnCode::SCARD_S_SUCCESS);
        debug!("sending {:?}", resp);
        Ok(Some(resp.encode()?))
    }

    fn handle_cancel(&self, input: &mut Payload) -> RdpResult<Option<Vec<u8>>> {
        let req = Context_Call::decode(input)?;
        debug!("got {:?}", req);
        let resp = Long_Return::new(ReturnCode::SCARD_S_SUCCESS);
        debug!("sending {:?}", resp);
        Ok(Some(resp.encode()?))
    }

    fn handle_is_valid_context(&self, input: &mut Payload) -> RdpResult<Option<Vec<u8>>> {
        let req = Context_Call::decode(input)?;
        debug!("got {:?}", req);
        let resp = Long_Return::new(ReturnCode::SCARD_S_SUCCESS);
        debug!("sending {:?}", resp);
        Ok(Some(resp.encode()?))
    }

    fn handle_list_readers(&self, input: &mut Payload) -> RdpResult<Option<Vec<u8>>> {
        let req = ListReaders_Call::decode(input)?;
        debug!("got {:?}", req);
        let resp =
            ListReaders_Return::new(ReturnCode::SCARD_S_SUCCESS, vec!["Teleport".to_string()]);
        debug!("sending {:?}", resp);
        Ok(Some(resp.encode()?))
    }

    fn handle_get_status_change(&self, input: &mut Payload) -> RdpResult<Option<Vec<u8>>> {
        let req = GetStatusChange_Call::decode(input)?;
        debug!("got {:?}", req);
        let resp = GetStatusChange_Return::new(ReturnCode::SCARD_S_SUCCESS, req);
        if resp.no_change() {
            debug!("blocking GetStatusChange call indefinitely, no response since our status will never change");
            Ok(None)
        } else {
            debug!("sending {:?}", resp);
            Ok(Some(resp.encode()?))
        }
    }

    fn handle_connect(&mut self, input: &mut Payload) -> RdpResult<Option<Vec<u8>>> {
        let req = Connect_Call::decode(input)?;
        debug!("got {:?}", req);

        let ctx = self
            .contexts
            .get(req.common.context.value)
            .ok_or_else(|| invalid_data_error("unknown context ID"))?;
        let handle = ctx.connect(req.common.context, self.uuid, &self.cert_der, &self.key_der)?;

        let resp = Connect_Return::new(ReturnCode::SCARD_S_SUCCESS, handle);
        debug!("sending {:?}", resp);
        Ok(Some(resp.encode()?))
    }

    fn handle_disconnect(&mut self, input: &mut Payload) -> RdpResult<Option<Vec<u8>>> {
        let req = HCardAndDisposition_Call::decode(input)?;
        debug!("got {:?}", req);

        self.contexts
            .get(req.handle.context.value)
            .ok_or_else(|| invalid_data_error("unknown context ID"))?
            .disconnect(req.handle.value);

        let resp = Long_Return::new(ReturnCode::SCARD_S_SUCCESS);
        debug!("sending {:?}", resp);
        Ok(Some(resp.encode()?))
    }

    fn handle_begin_transaction(&self, input: &mut Payload) -> RdpResult<Option<Vec<u8>>> {
        let req = HCardAndDisposition_Call::decode(input)?;
        debug!("got {:?}", req);
        let resp = Long_Return::new(ReturnCode::SCARD_S_SUCCESS);
        debug!("sending {:?}", resp);
        Ok(Some(resp.encode()?))
    }

    fn handle_end_transaction(&self, input: &mut Payload) -> RdpResult<Option<Vec<u8>>> {
        let req = HCardAndDisposition_Call::decode(input)?;
        debug!("got {:?}", req);
        let resp = Long_Return::new(ReturnCode::SCARD_S_SUCCESS);
        debug!("sending {:?}", resp);
        Ok(Some(resp.encode()?))
    }

    fn handle_status(
        &self,
        input: &mut Payload,
        enc: StringEncoding,
    ) -> RdpResult<Option<Vec<u8>>> {
        let req = Status_Call::decode(input)?;
        debug!("got {:?}", req);
        let resp = Status_Return::new(
            ReturnCode::SCARD_S_SUCCESS,
            vec!["Teleport".to_string()],
            enc,
        );
        debug!("sending {:?}", resp);
        Ok(Some(resp.encode()?))
    }

    fn handle_transmit(&mut self, input: &mut Payload) -> RdpResult<Option<Vec<u8>>> {
        let req = Transmit_Call::decode(input)?;
        debug!("got {:?}", req);

        // Decode the card command before sending it to piv.rs.
        // In retrospect, piv.rs should probably handle this decoding.
        let cmd =
            CardCommand::<TRANSMIT_DATA_LIMIT>::try_from(&req.send_buffer).map_err(|err| {
                invalid_data_error(&format!(
                    "failed to parse smartcard command {:?}: {:?}",
                    &req.send_buffer, err
                ))
            })?;

        let card = self
            .contexts
            .get(req.handle.context.value)
            .ok_or_else(|| invalid_data_error("unknown context ID"))?
            .get(req.handle.value)
            .ok_or_else(|| invalid_data_error("unknown handle ID"))?;

        let resp = card.handle(cmd)?;

        let resp = Transmit_Return::new(ReturnCode::SCARD_S_SUCCESS, resp.encode());
        debug!("sending {:?}", resp);
        Ok(Some(resp.encode()?))
    }

    fn handle_get_device_type_id(&mut self, input: &mut Payload) -> RdpResult<Option<Vec<u8>>> {
        let req = GetDeviceTypeId_Call::decode(input)?;
        debug!("got {:?}", req);

        let _ctx = self
            .contexts
            .get(req.context.value)
            .ok_or_else(|| invalid_data_error("unknown context ID"))?;

        let resp = GetDeviceTypeId_Return::new(ReturnCode::SCARD_S_SUCCESS);
        debug!("sending {:?}", resp);
        Ok(Some(resp.encode()?))
    }

    fn handle_read_cache(&mut self, input: &mut Payload) -> RdpResult<Option<Vec<u8>>> {
        let req = ReadCache_Call::decode(input)?;
        debug!("got {:?}", req);

        let val = self
            .contexts
            .get(req.common.context.value)
            .ok_or_else(|| invalid_data_error("unknown context ID"))?
            .cache_read(&req.lookup_name);

        let resp = ReadCache_Return::new(val);
        debug!("sending {:?}", resp);
        Ok(Some(resp.encode()?))
    }

    fn handle_write_cache(&mut self, input: &mut Payload) -> RdpResult<Option<Vec<u8>>> {
        let req = WriteCache_Call::decode(input)?;
        debug!("got {:?}", req);

        self.contexts
            .get(req.common.context.value)
            .ok_or_else(|| invalid_data_error("unknown context ID"))?
            .cache_write(req.lookup_name, req.common.data);

        let resp = Long_Return::new(ReturnCode::SCARD_S_SUCCESS);
        debug!("sending {:?}", resp);
        Ok(Some(resp.encode()?))
    }

    fn handle_get_reader_icon(&mut self, input: &mut Payload) -> RdpResult<Option<Vec<u8>>> {
        let req = GetReaderIcon_Call::decode(input)?;
        debug!("got {:?}", req);

        let _ctx = self
            .contexts
            .get(req.context.value)
            .ok_or_else(|| invalid_data_error("unknown context ID"))?;

        let resp = GetReaderIcon_Return::new(ReturnCode::SCARD_E_UNSUPPORTED_FEATURE);
        debug!("sending {:?}", resp);
        Ok(Some(resp.encode()?))
    }
}

// TRANSMIT_DATA_LIMIT is the maximum size of transmit request/response short data, in bytes.
const TRANSMIT_DATA_LIMIT: usize = 1024;

#[derive(Debug, FromPrimitive, ToPrimitive)]
#[allow(non_camel_case_types)]
enum IoctlCode {
    SCARD_IOCTL_ESTABLISHCONTEXT = 0x00090014,
    SCARD_IOCTL_RELEASECONTEXT = 0x00090018,
    SCARD_IOCTL_ISVALIDCONTEXT = 0x0009001C,
    SCARD_IOCTL_LISTREADERGROUPSA = 0x00090020,
    SCARD_IOCTL_LISTREADERGROUPSW = 0x00090024,
    SCARD_IOCTL_LISTREADERSA = 0x00090028,
    SCARD_IOCTL_LISTREADERSW = 0x0009002C,
    SCARD_IOCTL_INTRODUCEREADERGROUPA = 0x00090050,
    SCARD_IOCTL_INTRODUCEREADERGROUPW = 0x00090054,
    SCARD_IOCTL_FORGETREADERGROUPA = 0x00090058,
    SCARD_IOCTL_FORGETREADERGROUPW = 0x0009005C,
    SCARD_IOCTL_INTRODUCEREADERA = 0x00090060,
    SCARD_IOCTL_INTRODUCEREADERW = 0x00090064,
    SCARD_IOCTL_FORGETREADERA = 0x00090068,
    SCARD_IOCTL_FORGETREADERW = 0x0009006C,
    SCARD_IOCTL_ADDREADERTOGROUPA = 0x00090070,
    SCARD_IOCTL_ADDREADERTOGROUPW = 0x00090074,
    SCARD_IOCTL_REMOVEREADERFROMGROUPA = 0x00090078,
    SCARD_IOCTL_REMOVEREADERFROMGROUPW = 0x0009007C,
    SCARD_IOCTL_LOCATECARDSA = 0x00090098,
    SCARD_IOCTL_LOCATECARDSW = 0x0009009C,
    SCARD_IOCTL_GETSTATUSCHANGEA = 0x000900A0,
    SCARD_IOCTL_GETSTATUSCHANGEW = 0x000900A4,
    SCARD_IOCTL_CANCEL = 0x000900A8,
    SCARD_IOCTL_CONNECTA = 0x000900AC,
    SCARD_IOCTL_CONNECTW = 0x000900B0,
    SCARD_IOCTL_RECONNECT = 0x000900B4,
    SCARD_IOCTL_DISCONNECT = 0x000900B8,
    SCARD_IOCTL_BEGINTRANSACTION = 0x000900BC,
    SCARD_IOCTL_ENDTRANSACTION = 0x000900C0,
    SCARD_IOCTL_STATE = 0x000900C4,
    SCARD_IOCTL_STATUSA = 0x000900C8,
    SCARD_IOCTL_STATUSW = 0x000900CC,
    SCARD_IOCTL_TRANSMIT = 0x000900D0,
    SCARD_IOCTL_CONTROL = 0x000900D4,
    SCARD_IOCTL_GETATTRIB = 0x000900D8,
    SCARD_IOCTL_SETATTRIB = 0x000900DC,
    SCARD_IOCTL_ACCESSSTARTEDEVENT = 0x000900E0,
    SCARD_IOCTL_RELEASETARTEDEVENT = 0x000900E4,
    SCARD_IOCTL_LOCATECARDSBYATRA = 0x000900E8,
    SCARD_IOCTL_LOCATECARDSBYATRW = 0x000900EC,
    SCARD_IOCTL_READCACHEA = 0x000900F0,
    SCARD_IOCTL_READCACHEW = 0x000900F4,
    SCARD_IOCTL_WRITECACHEA = 0x000900F8,
    SCARD_IOCTL_WRITECACHEW = 0x000900FC,
    SCARD_IOCTL_GETTRANSMITCOUNT = 0x00090100,
    SCARD_IOCTL_GETREADERICON = 0x00090104,
    SCARD_IOCTL_GETDEVICETYPEID = 0x00090108,
}

// # Some notes on the encoding format.
//
// ## RPCE
//
// All messages are prepended with headers as described in section 2.2.6 of
// https://winprotocoldoc.blob.core.windows.net/productionwindowsarchives/MS-RPCE/%5bMS-RPCE%5d.pdf
// These headers specify version (always 1) and endianness for data. Windows hosts I tested against
// all send little-endian, so big-endian is not implemented in here at all.
//
// In addition, all messages are padded with zeroes to align to 8-byte boundary. Don't know why,
// but that's what the spec requires.
//
// ## NDR
//
// Request/response messages are nested structs with fields, encoded as NDR (network data
// representation), which is another Microsoft abomination.
//
// Fixed-sized fields are encoded in-line as they appear in the struct.
//
// Variable-sized fields (strings, byte arrays, sometimes structs) are encoded as pointers:
// - in place of the field in the struct, a "pointer" is written
// - the pointer value is 0x0002xxxx, where xxxx is an "index" in increments of 4
// - for example, first pointer is 0x00020000, second is 0x00020004, third is 0x00020008 etc.
// - the actual values are then appended at the end of the message, in the same order as their
//   pointers appeared
// - in the code below, "*_ptr" is the pointer value and "*_value" the actual data
// - note that some fields (like arrays) will have a length prefix before the pointer and also
//   before the actual data at the end of the message
//
// To deal with this, fixed-size structs only have encode/decode methods, while variable-size ones
// have encode_ptr/decode_ptr and encode_value/decode_value methods. Messages are parsed linearly,
// so decode_ptr/decode_value are called at different stages (same for encoding).
//
// Most of the above was reverse-engineered from FreeRDP:
// https://github.com/FreeRDP/FreeRDP/blob/master/channels/smartcard/client/smartcard_pack.c
//
// ## snake_case_types
//
// The structs below use naming copied from the spec. These names are CamelCase_And_Also_Snake,
// such as with '*_Call' (request) and '*_Return' (response) suffixes. It's not idiomatic Rust, but
// makes it much easier to switch between code and spec.

#[derive(Debug)]
struct RPCEStreamHeader {
    version: u8,
    endianness: RPCEEndianness,
    common_header_length: u16,
    filler: u32,
}

impl RPCEStreamHeader {
    fn new() -> Self {
        Self {
            version: 1,
            // We assume little endian for all messages, incoming and outgoing. If there's a weird
            // Windows machine out there that sends us big endian data, decoding will just fail.
            endianness: RPCEEndianness::LittleEndian,
            common_header_length: 8,
            filler: 0xcccccccc,
        }
    }
    fn encode(&self) -> RdpResult<Vec<u8>> {
        let mut w = vec![];
        w.write_u8(self.version)?;
        w.write_u8(self.endianness.to_u8().unwrap())?;
        w.write_u16::<LittleEndian>(self.common_header_length)?;
        w.write_u32::<LittleEndian>(self.filler)?;
        Ok(w)
    }
    fn decode(payload: &mut Payload) -> RdpResult<Self> {
        let header = Self {
            version: payload.read_u8()?,
            endianness: RPCEEndianness::from_u8(payload.read_u8()?)
                .ok_or_else(|| invalid_data_error("invalid endianness in RPCE stream header"))?,
            common_header_length: payload.read_u16::<LittleEndian>()?,
            filler: payload.read_u32::<LittleEndian>()?,
        };
        // TODO(zmb3): implement big endian parsing support
        if let RPCEEndianness::LittleEndian = header.endianness {
            Ok(header)
        } else {
            Err(invalid_data_error(
                "server returned big-endian data, parsing not implemented",
            ))
        }
    }
}

fn encode_response(resp: Vec<u8>) -> RdpResult<Vec<u8>> {
    let mut resp = resp;
    // Pad response to be 8-byte aligned.
    let tail = resp.length() % 8;
    if tail > 0 {
        resp.resize((resp.length() + (8 - tail)) as usize, 0);
    }

    let mut buf = RPCEStreamHeader::new().encode()?;
    RPCETypeHeader::new(resp.length() as u32).encode(&mut buf)?;
    buf.extend_from_slice(&resp);
    Ok(buf)
}

#[derive(Debug, FromPrimitive, ToPrimitive)]
#[allow(non_camel_case_types)]
enum RPCEEndianness {
    BigEndian = 0x00,
    LittleEndian = 0x10,
}

#[derive(Debug)]
struct RPCETypeHeader {
    object_buffer_length: u32,
    filler: u32,
}

impl RPCETypeHeader {
    fn new(len: u32) -> Self {
        Self {
            object_buffer_length: len,
            filler: 0,
        }
    }
    fn encode(&self, w: &mut dyn Write) -> RdpResult<()> {
        w.write_u32::<LittleEndian>(self.object_buffer_length)?;
        w.write_u32::<LittleEndian>(self.filler)?;
        Ok(())
    }
    fn decode(payload: &mut Payload) -> RdpResult<Self> {
        Ok(Self {
            object_buffer_length: payload.read_u32::<LittleEndian>()?,
            filler: payload.read_u32::<LittleEndian>()?,
        })
    }
}

#[derive(Debug)]
#[allow(non_camel_case_types, dead_code)]
struct ScardAccessStartedEvent_Call {
    unused: u32,
}

impl ScardAccessStartedEvent_Call {
    fn decode(payload: &mut Payload) -> RdpResult<Self> {
        Ok(Self {
            unused: payload.read_u32::<LittleEndian>()?,
        })
    }
}

#[derive(Debug, FromPrimitive, ToPrimitive)]
#[allow(non_camel_case_types)]
#[repr(u32)]
enum ReturnCode {
    SCARD_S_SUCCESS = 0x00000000,
    SCARD_F_INTERNAL_ERROR = 0x80100001,
    SCARD_E_CANCELLED = 0x80100002,
    SCARD_E_INVALID_HANDLE = 0x80100003,
    SCARD_E_INVALID_PARAMETER = 0x80100004,
    SCARD_E_INVALID_TARGET = 0x80100005,
    SCARD_E_NO_MEMORY = 0x80100006,
    SCARD_F_WAITED_TOO_LONG = 0x80100007,
    SCARD_E_INSUFFICIENT_BUFFER = 0x80100008,
    SCARD_E_UNKNOWN_READER = 0x80100009,
    SCARD_E_TIMEOUT = 0x8010000A,
    SCARD_E_SHARING_VIOLATION = 0x8010000B,
    SCARD_E_NO_SMARTCARD = 0x8010000C,
    SCARD_E_UNKNOWN_CARD = 0x8010000D,
    SCARD_E_CANT_DISPOSE = 0x8010000E,
    SCARD_E_PROTO_MISMATCH = 0x8010000F,
    SCARD_E_NOT_READY = 0x80100010,
    SCARD_E_INVALID_VALUE = 0x80100011,
    SCARD_E_SYSTEM_CANCELLED = 0x80100012,
    SCARD_F_COMM_ERROR = 0x80100013,
    SCARD_F_UNKNOWN_ERROR = 0x80100014,
    SCARD_E_INVALID_ATR = 0x80100015,
    SCARD_E_NOT_TRANSACTED = 0x80100016,
    SCARD_E_READER_UNAVAILABLE = 0x80100017,
    SCARD_P_SHUTDOWN = 0x80100018,
    SCARD_E_PCI_TOO_SMALL = 0x80100019,
    SCARD_E_ICC_INSTALLATION = 0x80100020,
    SCARD_E_ICC_CREATEORDER = 0x80100021,
    SCARD_E_UNSUPPORTED_FEATURE = 0x80100022,
    SCARD_E_DIR_NOT_FOUND = 0x80100023,
    SCARD_E_FILE_NOT_FOUND = 0x80100024,
    SCARD_E_NO_DIR = 0x80100025,
    SCARD_E_READER_UNSUPPORTED = 0x8010001A,
    SCARD_E_DUPLICATE_READER = 0x8010001B,
    SCARD_E_CARD_UNSUPPORTED = 0x8010001C,
    SCARD_E_NO_SERVICE = 0x8010001D,
    SCARD_E_SERVICE_STOPPED = 0x8010001E,
    SCARD_E_UNEXPECTED = 0x8010001F,
    SCARD_E_NO_FILE = 0x80100026,
    SCARD_E_NO_ACCESS = 0x80100027,
    SCARD_E_WRITE_TOO_MANY = 0x80100028,
    SCARD_E_BAD_SEEK = 0x80100029,
    SCARD_E_INVALID_CHV = 0x8010002A,
    SCARD_E_UNKNOWN_RES_MSG = 0x8010002B,
    SCARD_E_NO_SUCH_CERTIFICATE = 0x8010002C,
    SCARD_E_CERTIFICATE_UNAVAILABLE = 0x8010002D,
    SCARD_E_NO_READERS_AVAILABLE = 0x8010002E,
    SCARD_E_COMM_DATA_LOST = 0x8010002F,
    SCARD_E_NO_KEY_CONTAINER = 0x80100030,
    SCARD_E_SERVER_TOO_BUSY = 0x80100031,
    SCARD_E_PIN_CACHE_EXPIRED = 0x80100032,
    SCARD_E_NO_PIN_CACHE = 0x80100033,
    SCARD_E_READ_ONLY_CARD = 0x80100034,
    SCARD_W_UNSUPPORTED_CARD = 0x80100065,
    SCARD_W_UNRESPONSIVE_CARD = 0x80100066,
    SCARD_W_UNPOWERED_CARD = 0x80100067,
    SCARD_W_RESET_CARD = 0x80100068,
    SCARD_W_REMOVED_CARD = 0x80100069,
    SCARD_W_SECURITY_VIOLATION = 0x8010006A,
    SCARD_W_WRONG_CHV = 0x8010006B,
    SCARD_W_CHV_BLOCKED = 0x8010006C,
    SCARD_W_EOF = 0x8010006D,
    SCARD_W_CANCELLED_BY_USER = 0x8010006E,
    SCARD_W_CARD_NOT_AUTHENTICATED = 0x8010006F,
    SCARD_W_CACHE_ITEM_NOT_FOUND = 0x80100070,
    SCARD_W_CACHE_ITEM_STALE = 0x80100071,
    SCARD_W_CACHE_ITEM_TOO_BIG = 0x80100072,
}

#[derive(Debug)]
#[allow(non_camel_case_types)]
struct Long_Return {
    return_code: ReturnCode,
}

impl Long_Return {
    fn new(return_code: ReturnCode) -> Self {
        Self { return_code }
    }
    fn encode(&self) -> RdpResult<Vec<u8>> {
        let mut w = vec![];
        w.write_u32::<LittleEndian>(self.return_code.to_u32().unwrap())?;
        Ok(w)
    }
}

#[derive(Debug)]
#[allow(non_camel_case_types, dead_code)]
struct EstablishContext_Call {
    scope: Scope,
}

impl EstablishContext_Call {
    fn decode(payload: &mut Payload) -> RdpResult<Self> {
        let _header = RPCEStreamHeader::decode(payload)?;
        let _header = RPCETypeHeader::decode(payload)?;
        let scope = payload.read_u32::<LittleEndian>()?;
        Ok(Self {
            scope: Scope::from_u32(scope).ok_or_else(|| {
                invalid_data_error(&format!("invalid smart card scope {:?}", scope))
            })?,
        })
    }
}

#[derive(Debug, FromPrimitive, ToPrimitive)]
#[allow(non_camel_case_types)]
enum Scope {
    SCARD_SCOPE_USER = 0x00000000,
    SCARD_SCOPE_TERMINAL = 0x00000001,
    SCARD_SCOPE_SYSTEM = 0x00000002,
}

#[derive(Debug)]
#[allow(non_camel_case_types)]
struct EstablishContext_Return {
    return_code: ReturnCode,
    context: Context,
}

impl EstablishContext_Return {
    fn new(return_code: ReturnCode, context: Context) -> Self {
        Self {
            return_code,
            context,
        }
    }
    fn encode(&self) -> RdpResult<Vec<u8>> {
        let mut w = vec![];
        w.write_u32::<LittleEndian>(self.return_code.to_u32().unwrap())?;
        let mut index = 0;
        self.context.encode_ptr(&mut index, &mut w)?;
        self.context.encode_value(&mut w)?;
        Ok(w)
    }
}

#[derive(Debug)]
struct Context {
    length: u32,
    // Shortcut: we always create 4-byte context values.
    // The spec allows this field to have variable length.
    value: u32,
}

impl Context {
    fn new(val: u32) -> Self {
        Self {
            length: 4,
            value: val,
        }
    }
    fn encode_ptr(&self, index: &mut u32, w: &mut dyn Write) -> RdpResult<()> {
        encode_ptr(self.length, index, w)
    }
    fn encode_value(&self, w: &mut dyn Write) -> RdpResult<()> {
        w.write_u32::<LittleEndian>(self.length)?;
        w.write_u32::<LittleEndian>(self.value)?;
        Ok(())
    }
    fn decode_ptr(payload: &mut Payload, index: &mut u32) -> RdpResult<Self> {
        let length = payload.read_u32::<LittleEndian>()?;
        let _ptr = decode_ptr(payload, index)?;
        Ok(Self { length, value: 0 })
    }
    fn decode_value(&mut self, payload: &mut Payload) -> RdpResult<()> {
        let length = payload.read_u32::<LittleEndian>()?;
        if length != self.length {
            Err(invalid_data_error(
                "mismatched length in Context reference and value",
            ))
        } else {
            self.value = payload.read_u32::<LittleEndian>()?;
            Ok(())
        }
    }
}

// encode_ptr/decode_ptr and various encode_value/decode_value functions implement the strange NDR
// protocol. See the big comment above with encoding notes.
fn encode_ptr(length: u32, index: &mut u32, w: &mut dyn Write) -> RdpResult<()> {
    w.write_u32::<LittleEndian>(length)?;
    w.write_u32::<LittleEndian>(0x00020000 + *index * 4)?;
    *index += 1;
    Ok(())
}

fn decode_ptr(payload: &mut Payload, index: &mut u32) -> RdpResult<u32> {
    let ptr = payload.read_u32::<LittleEndian>()?;
    if ptr == 0 {
        // NULL pointer is OK. Don't update index.
        return Ok(ptr);
    }
    let expect_ptr = 0x00020000 + *index * 4;
    *index += 1;
    if ptr != expect_ptr {
        Err(invalid_data_error(&format!(
            "invalid NDR pointer value {:#010X}, expected {:#010X}",
            ptr, expect_ptr
        )))
    } else {
        Ok(ptr)
    }
}

#[derive(Debug)]
#[allow(non_camel_case_types, dead_code)]
struct ListReaders_Call {
    context: Context,
    groups_length: u32,
    groups: Vec<String>,
    readers_is_null: bool,
    readers_size: u32,
}

impl ListReaders_Call {
    fn decode(payload: &mut Payload) -> RdpResult<Self> {
        let _header = RPCEStreamHeader::decode(payload)?;
        let _header = RPCETypeHeader::decode(payload)?;

        let mut index = 0;
        let mut context = Context::decode_ptr(payload, &mut index)?;
        let groups_ptr_length = payload.read_u32::<LittleEndian>()?;
        let groups_ptr = decode_ptr(payload, &mut index)?;
        let readers_is_null = (payload.read_u32::<LittleEndian>()?) == 0x00000001;
        let readers_size = payload.read_u32::<LittleEndian>()?;

        context.decode_value(payload)?;

        if groups_ptr == 0 {
            return Ok(Self {
                context,
                groups_length: 0,
                groups: Vec::new(),
                readers_is_null,
                readers_size,
            });
        }
        let (groups_length, groups) = decode_multistring_unicode(payload)?;
        if groups_length != groups_ptr_length {
            Err(invalid_data_error(
                "mismatched reader groups length in NDR pointer and value",
            ))
        } else {
            Ok(Self {
                context,
                groups_length,
                groups,
                readers_is_null,
                readers_size,
            })
        }
    }
}

#[derive(Debug)]
#[allow(non_camel_case_types)]
struct ListReaders_Return {
    return_code: ReturnCode,
    readers: Vec<String>,
}

impl ListReaders_Return {
    fn new(return_code: ReturnCode, readers: Vec<String>) -> Self {
        Self {
            return_code,
            readers,
        }
    }
    fn encode(&self) -> RdpResult<Vec<u8>> {
        let mut w = vec![];
        w.write_u32::<LittleEndian>(self.return_code.to_u32().unwrap())?;
        let readers = encode_multistring_unicode(&self.readers)?;
        let mut index = 0;
        encode_ptr(readers.length() as u32, &mut index, &mut w)?;

        w.write_u32::<LittleEndian>(readers.length() as u32)?;
        w.extend_from_slice(&readers);

        Ok(w)
    }
}

// Unicode multistring is a list of null-terminated UTF-16 strings. At the end, the list is
// terminated with another null byte (so, two null bytes if you want to find the end in a binary
// dump).
fn decode_multistring_unicode(payload: &mut Payload) -> RdpResult<(u32, Vec<String>)> {
    let len = payload.read_u32::<LittleEndian>()?;
    let mut items = vec![];
    let mut buf = vec![];
    // Each utf-16 character is 2 bytes. So there are len/2 characters.
    for _i in 0..(len / 2) {
        let c = payload.read_u16::<LittleEndian>()?;
        if c == 0 {
            if !buf.is_empty() {
                items.push(
                    decode_utf16(buf.iter().copied())
                        .map(|r| r.unwrap_or(REPLACEMENT_CHARACTER))
                        .collect::<String>(),
                );
                buf.clear();
            }
        } else {
            buf.push(c);
        }
    }
    Ok((len, items))
}

fn decode_string_unicode(payload: &mut Payload) -> RdpResult<String> {
    // These length/offset fields seem to be unnecessary since the strings are null-terminated. But
    // they are present in the encoded form anyway.
    let _len = payload.read_u32::<LittleEndian>()?;
    let _offset = payload.read_u32::<LittleEndian>()?;
    let _len2 = payload.read_u32::<LittleEndian>()?;

    // Read until NULL character.
    let mut buf = vec![];
    loop {
        let c = payload.read_u16::<LittleEndian>()?;
        if c == 0 {
            // Consume the extra padding for a 4-byte aligned NULL-terminated string.
            if buf.len() % 2 == 0 {
                let _padding = payload.read_u16::<LittleEndian>()?;
            }
            break;
        } else {
            buf.push(c);
        }
    }
    let s = decode_utf16(buf.iter().copied())
        .map(|r| r.unwrap_or(REPLACEMENT_CHARACTER))
        .collect::<String>();
    Ok(s)
}

fn encode_multistring_unicode(items: &[String]) -> RdpResult<Vec<u8>> {
    let mut buf = vec![];
    for s in items.iter() {
        for c in s.encode_utf16() {
            buf.write_u16::<LittleEndian>(c)?;
        }
        buf.write_u16::<LittleEndian>(0)?;
    }
    buf.write_u16::<LittleEndian>(0)?;
    Ok(buf)
}

// ASCII multistring is the same as a unicode one, except all characters are 1 byte.
fn encode_multistring_ascii(items: &[String]) -> RdpResult<Vec<u8>> {
    let mut buf = vec![];
    for s in items.iter() {
        buf.extend_from_slice(s.as_bytes());
        buf.write_u8(0)?;
    }
    buf.write_u8(0)?;
    Ok(buf)
}

#[derive(Debug)]
#[allow(non_camel_case_types)]
struct Context_Call {
    context: Context,
}

impl Context_Call {
    fn decode(payload: &mut Payload) -> RdpResult<Self> {
        let _header = RPCEStreamHeader::decode(payload)?;
        let _header = RPCETypeHeader::decode(payload)?;

        let mut index = 0;
        let mut context = Context::decode_ptr(payload, &mut index)?;
        context.decode_value(payload)?;
        Ok(Self { context })
    }
}

#[derive(Debug)]
#[allow(non_camel_case_types, dead_code)]
struct GetStatusChange_Call {
    context: Context,
    timeout: u32,
    states: Vec<ReaderState>,
}

impl GetStatusChange_Call {
    fn decode(payload: &mut Payload) -> RdpResult<Self> {
        let _header = RPCEStreamHeader::decode(payload)?;
        let _header = RPCETypeHeader::decode(payload)?;

        let mut index = 0;
        let mut context = Context::decode_ptr(payload, &mut index)?;

        let timeout = payload.read_u32::<LittleEndian>()?;
        let _states_length = payload.read_u32::<LittleEndian>()?;
        let _states_ptr = decode_ptr(payload, &mut index)?;

        context.decode_value(payload)?;

        let states_length = payload.read_u32::<LittleEndian>()?;
        let mut states = vec![];
        for _i in 0..states_length {
            let state = ReaderState::decode_ptr(payload, &mut index)?;
            states.push(state);
        }
        for state in states.iter_mut() {
            state.decode_value(payload)?;
        }
        Ok(Self {
            context,
            timeout,
            states,
        })
    }
}

#[derive(Debug)]
struct ReaderState {
    reader: String,
    common: ReaderState_Common_Call,
}

impl ReaderState {
    fn decode_ptr(payload: &mut Payload, index: &mut u32) -> RdpResult<Self> {
        let _reader_ptr = decode_ptr(payload, index)?;
        let common = ReaderState_Common_Call::decode(payload)?;
        Ok(Self {
            reader: String::new(),
            common,
        })
    }

    fn decode_value(&mut self, payload: &mut Payload) -> RdpResult<()> {
        self.reader = decode_string_unicode(payload)?;
        Ok(())
    }
}

#[derive(Debug)]
#[allow(non_camel_case_types)]
struct ReaderState_Common_Call {
    current_state: CardStateFlags,
    event_state: CardStateFlags,
    atr_length: u32,
    atr: [u8; 36],
}

impl ReaderState_Common_Call {
    fn decode(payload: &mut Payload) -> RdpResult<Self> {
        let current_state = CardStateFlags::from_bits_truncate(payload.read_u32::<LittleEndian>()?);
        let event_state = CardStateFlags::from_bits_truncate(payload.read_u32::<LittleEndian>()?);
        let atr_length = payload.read_u32::<LittleEndian>()?;
        let mut atr = vec![];
        atr.resize(36, 0);
        payload.read_exact(&mut atr)?;
        Ok(Self {
            current_state,
            event_state,
            atr_length,
            atr: atr.try_into().unwrap(),
        })
    }

    fn encode(&self, w: &mut Vec<u8>) -> RdpResult<()> {
        w.write_u32::<LittleEndian>(self.current_state.bits())?;
        w.write_u32::<LittleEndian>(self.event_state.bits())?;
        w.write_u32::<LittleEndian>(self.atr_length)?;
        w.extend_from_slice(&self.atr);
        Ok(())
    }
}

bitflags! {
    struct CardStateFlags: u32 {
        const SCARD_STATE_UNAWARE = 0x0000;
        const SCARD_STATE_IGNORE = 0x0001;
        const SCARD_STATE_CHANGED = 0x0002;
        const SCARD_STATE_UNKNOWN = 0x0004;
        const SCARD_STATE_UNAVAILABLE = 0x0008;
        const SCARD_STATE_EMPTY = 0x0010;
        const SCARD_STATE_PRESENT = 0x0020;
        const SCARD_STATE_ATRMATCH = 0x0040;
        const SCARD_STATE_EXCLUSIVE = 0x0080;
        const SCARD_STATE_INUSE = 0x0100;
        const SCARD_STATE_MUTE = 0x0200;
        const SCARD_STATE_UNPOWERED = 0x0400;
    }
}

// ATR value taken from
// http://ludovic.rousseau.free.fr/softwares/pcsc-tools/smartcard_list.txt
// (from vsmartcard project).
//
// The data encoded in here seems mostly unimportant, but it's used to identify specific smartcard
// devices. Windows matches cards to specific minidriver DLLs based on the ATR value, which changes
// how Windows interacts with the card entirely.
//
// This ATR will match us against the default smartcard minidriver:
// https://docs.microsoft.com/en-us/windows-hardware/drivers/smartcard/windows-inbox-smart-card-minidriver
const STATIC_ATR: [u8; 11] = [
    0x3B, 0x95, 0x13, 0x81, 0x01, 0x80, 0x73, 0xFF, 0x01, 0x00, 0x0B,
];

fn padded_atr(size: usize) -> (u32, Vec<u8>) {
    let mut atr = STATIC_ATR.to_vec();
    atr.resize(size, 0);
    (STATIC_ATR.len() as u32, atr)
}

#[derive(Debug)]
#[allow(non_camel_case_types)]
struct GetStatusChange_Return {
    return_code: ReturnCode,
    reader_states: Vec<ReaderState_Common_Call>,
}
impl GetStatusChange_Return {
    fn new(return_code: ReturnCode, req: GetStatusChange_Call) -> Self {
        let mut reader_states = vec![];
        for state in req.states {
            match state.reader.as_str() {
                // I think PnP is Plug-and-Play. This special reader "name" is used to monitor for
                // new readers being plugged in.
                "\\\\?PnP?\\Notification" => {
                    reader_states.push(ReaderState_Common_Call {
                        current_state: state.common.current_state,
                        event_state: state.common.current_state,
                        atr_length: state.common.atr_length,
                        atr: state.common.atr,
                    });
                }
                // This is our actual emulated smartcard reader. We always advertise its state as
                // "present".
                "Teleport" => {
                    let (atr_length, atr) = padded_atr(36);
                    reader_states.push(ReaderState_Common_Call {
                        current_state: state.common.current_state,
                        event_state: CardStateFlags::SCARD_STATE_CHANGED
                            | CardStateFlags::SCARD_STATE_PRESENT,
                        atr_length,
                        atr: atr.try_into().unwrap(),
                    });
                }
                // All other reader names are unknown and unexpected.
                _ => {
                    reader_states.push(ReaderState_Common_Call {
                        current_state: state.common.current_state,
                        event_state: CardStateFlags::SCARD_STATE_CHANGED
                            | CardStateFlags::SCARD_STATE_UNKNOWN
                            | CardStateFlags::SCARD_STATE_IGNORE,
                        atr_length: state.common.atr_length,
                        atr: state.common.atr,
                    });
                }
            }
        }
        Self {
            return_code,
            reader_states,
        }
    }
    fn encode(&self) -> RdpResult<Vec<u8>> {
        let mut w = vec![];
        w.write_u32::<LittleEndian>(self.return_code.to_u32().unwrap())?;
        let mut index = 0;
        encode_ptr(self.reader_states.len() as u32, &mut index, &mut w)?;

        w.write_u32::<LittleEndian>(self.reader_states.len() as u32)?;
        for state in &self.reader_states {
            state.encode(&mut w)?;
        }

        Ok(w)
    }

    fn no_change(&self) -> bool {
        for state in &self.reader_states {
            if state.current_state != state.event_state {
                return false;
            }
        }
        true
    }
}

#[derive(Debug)]
#[allow(non_camel_case_types, dead_code)]
struct Connect_Call {
    reader: String,
    common: Connect_Common,
}

impl Connect_Call {
    fn decode(payload: &mut Payload) -> RdpResult<Self> {
        let _header = RPCEStreamHeader::decode(payload)?;
        let _header = RPCETypeHeader::decode(payload)?;

        let mut index = 0;
        let _reader_ptr = decode_ptr(payload, &mut index)?;
        let mut common = Connect_Common::decode_ptr(payload, &mut index)?;
        let reader = decode_string_unicode(payload)?;
        common.decode_value(payload)?;
        Ok(Self { reader, common })
    }
}

bitflags! {
    struct CardProtocol: u32 {
        const SCARD_PROTOCOL_UNDEFINED = 0x00000000;
        const SCARD_PROTOCOL_T0 = 0x00000001;
        const SCARD_PROTOCOL_T1 = 0x00000002;
        const SCARD_PROTOCOL_TX = 0x00000003;
        const SCARD_PROTOCOL_RAW = 0x00010000;
        const SCARD_PROTOCOL_DEFAULT = 0x80000000;
        const SCARD_PROTOCOL_OPTIMAL = 0x00000000;
    }
}

#[derive(Debug)]
#[allow(non_camel_case_types, dead_code)]
struct Connect_Common {
    context: Context,
    share_mode: u32,
    preferred_protocols: CardProtocol,
}

impl Connect_Common {
    fn decode_ptr(payload: &mut Payload, index: &mut u32) -> RdpResult<Self> {
        let context = Context::decode_ptr(payload, index)?;
        let share_mode = payload.read_u32::<LittleEndian>()?;
        let preferred_protocols = CardProtocol::from_bits(payload.read_u32::<LittleEndian>()?)
            .ok_or_else(|| {
                invalid_data_error("invalid preferred_protocols bits in Connect_Common")
            })?;
        Ok(Self {
            context,
            share_mode,
            preferred_protocols,
        })
    }
    fn decode_value(&mut self, payload: &mut Payload) -> RdpResult<()> {
        self.context.decode_value(payload)?;
        Ok(())
    }
}

#[derive(Debug)]
#[allow(non_camel_case_types)]
struct Connect_Return {
    return_code: ReturnCode,
    handle: Handle,
    active_protocol: CardProtocol,
}

impl Connect_Return {
    fn new(return_code: ReturnCode, handle: Handle) -> Self {
        Self {
            return_code,
            handle,
            active_protocol: CardProtocol::SCARD_PROTOCOL_T1,
        }
    }
    fn encode(&self) -> RdpResult<Vec<u8>> {
        let mut w = vec![];
        w.write_u32::<LittleEndian>(self.return_code.to_u32().unwrap())?;
        let mut index = 0;
        self.handle.encode_ptr(&mut index, &mut w)?;
        w.write_u32::<LittleEndian>(self.active_protocol.bits())?;
        self.handle.encode_value(&mut w)?;
        Ok(w)
    }
}

#[derive(Debug)]
struct Handle {
    context: Context,
    length: u32,
    // Shortcut: we always create 4-byte handle values.
    // The spec allows this field to have variable length.
    value: u32,
}

impl Handle {
    fn new(context: Context, value: u32) -> Self {
        Self {
            context,
            length: 4,
            value,
        }
    }
    fn encode_ptr(&self, index: &mut u32, w: &mut dyn Write) -> RdpResult<()> {
        self.context.encode_ptr(index, w)?;
        encode_ptr(self.length, index, w)?;
        Ok(())
    }
    fn encode_value(&self, w: &mut dyn Write) -> RdpResult<()> {
        self.context.encode_value(w)?;
        w.write_u32::<LittleEndian>(self.length)?;
        w.write_u32::<LittleEndian>(self.value)?;
        Ok(())
    }

    fn decode_ptr(payload: &mut Payload, index: &mut u32) -> RdpResult<Self> {
        let context = Context::decode_ptr(payload, index)?;
        let length = payload.read_u32::<LittleEndian>()?;
        let _ptr = decode_ptr(payload, index)?;
        Ok(Self {
            context,
            length,
            value: 0,
        })
    }
    fn decode_value(&mut self, payload: &mut Payload) -> RdpResult<()> {
        self.context.decode_value(payload)?;
        let length = payload.read_u32::<LittleEndian>()?;
        if length != self.length {
            Err(invalid_data_error(
                "mismatched length in Handle reference and value",
            ))
        } else {
            self.value = payload.read_u32::<LittleEndian>()?;
            Ok(())
        }
    }
}

#[derive(Debug)]
#[allow(non_camel_case_types, dead_code)]
struct HCardAndDisposition_Call {
    handle: Handle,
    disposition: u32,
}

impl HCardAndDisposition_Call {
    fn decode(payload: &mut Payload) -> RdpResult<Self> {
        let _header = RPCEStreamHeader::decode(payload)?;
        let _header = RPCETypeHeader::decode(payload)?;

        let mut index = 0;
        let mut handle = Handle::decode_ptr(payload, &mut index)?;
        let disposition = payload.read_u32::<LittleEndian>()?;
        handle.decode_value(payload)?;
        Ok(Self {
            handle,
            disposition,
        })
    }
}

#[derive(Debug)]
#[allow(non_camel_case_types, dead_code)]
struct Status_Call {
    handle: Handle,
    reader_names_is_null: bool,
    reader_length: u32,
    atr_length: u32,
}

impl Status_Call {
    fn decode(payload: &mut Payload) -> RdpResult<Self> {
        let _header = RPCEStreamHeader::decode(payload)?;
        let _header = RPCETypeHeader::decode(payload)?;

        let mut index = 0;
        let mut handle = Handle::decode_ptr(payload, &mut index)?;
        let reader_names_is_null = payload.read_u32::<LittleEndian>()? == 1;
        let reader_length = payload.read_u32::<LittleEndian>()?;
        let atr_length = payload.read_u32::<LittleEndian>()?;
        handle.decode_value(payload)?;
        Ok(Self {
            handle,
            reader_names_is_null,
            reader_length,
            atr_length,
        })
    }
}

#[derive(Debug, FromPrimitive, ToPrimitive)]
#[allow(non_camel_case_types)]
enum State {
    SCARD_UNKNOWN = 0x00000000,
    SCARD_ABSENT = 0x00000001,
    SCARD_PRESENT = 0x00000002,
    SCARD_SWALLOWED = 0x00000003,
    SCARD_POWERED = 0x00000004,
    SCARD_NEGOTIABLE = 0x00000005,
    SCARD_SPECIFICMODE = 0x00000006,
}

#[derive(Debug)]
enum StringEncoding {
    Ascii,
    Unicode,
}

#[derive(Debug)]
#[allow(non_camel_case_types)]
struct Status_Return {
    return_code: ReturnCode,
    reader_names: Vec<String>,
    state: State,
    protocol: CardProtocol,
    atr: [u8; 32],
    atr_length: u32,

    encoding: StringEncoding,
}

impl Status_Return {
    fn new(return_code: ReturnCode, reader_names: Vec<String>, encoding: StringEncoding) -> Self {
        let (atr_length, atr) = padded_atr(32);
        Self {
            return_code,
            reader_names,
            // SPECIFICMODE state means that the card is ready to handle commands in a specific
            // mode, no other negotiation is necessary. Real smartcards would probably negotiate
            // some mode first.
            state: State::SCARD_SPECIFICMODE,
            protocol: CardProtocol::SCARD_PROTOCOL_T1,
            atr: atr.try_into().unwrap(),
            atr_length,
            encoding,
        }
    }
    fn encode(&self) -> RdpResult<Vec<u8>> {
        let mut w = vec![];
        w.write_u32::<LittleEndian>(self.return_code.to_u32().unwrap())?;

        let reader_names = match &self.encoding {
            StringEncoding::Unicode => encode_multistring_unicode(&self.reader_names)?,
            StringEncoding::Ascii => encode_multistring_ascii(&self.reader_names)?,
        };
        let mut index = 0;
        encode_ptr(reader_names.length() as u32, &mut index, &mut w)?;

        w.write_u32::<LittleEndian>(self.state.to_u32().unwrap())?;
        w.write_u32::<LittleEndian>(self.protocol.bits())?;
        w.extend_from_slice(&self.atr);
        w.write_u32::<LittleEndian>(self.atr_length)?;

        w.write_u32::<LittleEndian>(reader_names.length() as u32)?;
        w.extend_from_slice(&reader_names);

        Ok(w)
    }
}

#[derive(Debug)]
#[allow(non_camel_case_types, dead_code)]
struct Transmit_Call {
    handle: Handle,
    send_pci: SCardIO_Request,
    send_length: u32,
    send_buffer: Vec<u8>,
    recv_pci: Option<SCardIO_Request>,
    recv_buffer_is_null: bool,
    recv_length: u32,
}

impl Transmit_Call {
    fn decode(payload: &mut Payload) -> RdpResult<Self> {
        let _header = RPCEStreamHeader::decode(payload)?;
        let _header = RPCETypeHeader::decode(payload)?;

        let mut index = 0;
        let mut handle = Handle::decode_ptr(payload, &mut index)?;
        let mut send_pci = SCardIO_Request::decode_ptr(payload, &mut index)?;
        let _send_length = payload.read_u32::<LittleEndian>()?;
        let _send_buffer_ptr = decode_ptr(payload, &mut index)?;
        let recv_pci_ptr = decode_ptr(payload, &mut index)?;
        let recv_buffer_is_null = payload.read_u32::<LittleEndian>()? == 1;
        let recv_length = payload.read_u32::<LittleEndian>()?;

        handle.decode_value(payload)?;
        send_pci.decode_value(payload)?;

        let send_length = payload.read_u32::<LittleEndian>()?;
        let mut send_buffer = vec![];
        send_buffer.resize(send_length as usize, 0);
        payload.read_exact(&mut send_buffer)?;

        let recv_pci = if recv_pci_ptr != 0 {
            let mut recv_pci = SCardIO_Request::decode_ptr(payload, &mut index)?;
            recv_pci.decode_value(payload)?;
            Some(recv_pci)
        } else {
            None
        };

        Ok(Self {
            handle,
            send_pci,
            send_length,
            send_buffer,
            recv_pci,
            recv_buffer_is_null,
            recv_length,
        })
    }
}

#[derive(Debug)]
#[allow(non_camel_case_types, dead_code)]
struct SCardIO_Request {
    protocol: CardProtocol,
    extra_bytes_length: u32,
    extra_bytes: Vec<u8>,
}

impl SCardIO_Request {
    fn decode_ptr(payload: &mut Payload, index: &mut u32) -> RdpResult<Self> {
        let protocol = CardProtocol::from_bits(payload.read_u32::<LittleEndian>()?)
            .ok_or_else(|| invalid_data_error("invalid protocol bits in SCardIO_Request"))?;
        let extra_bytes_length = payload.read_u32::<LittleEndian>()?;
        let _extra_bytes_ptr = decode_ptr(payload, index)?;
        let mut extra_bytes = vec![];
        extra_bytes.resize(extra_bytes_length as usize, 0);
        Ok(Self {
            protocol,
            extra_bytes_length,
            extra_bytes,
        })
    }
    fn decode_value(&mut self, payload: &mut Payload) -> RdpResult<()> {
        payload.read_exact(&mut self.extra_bytes)?;
        Ok(())
    }
}

#[derive(Debug)]
#[allow(non_camel_case_types)]
struct Transmit_Return {
    return_code: ReturnCode,
    recv_buffer: Vec<u8>,
}

impl Transmit_Return {
    fn new(return_code: ReturnCode, recv_buffer: Vec<u8>) -> Self {
        Self {
            return_code,
            recv_buffer,
        }
    }
    fn encode(&self) -> RdpResult<Vec<u8>> {
        let mut w = vec![];
        w.write_u32::<LittleEndian>(self.return_code.to_u32().unwrap())?;

        // There is a recv_pci (SCardIO_Request) field before recv_buffer, but it's always null in
        // our case.
        w.write_u32::<LittleEndian>(0)?;

        let mut index = 0;
        encode_ptr(self.recv_buffer.len() as u32, &mut index, &mut w)?;
        w.write_u32::<LittleEndian>(self.recv_buffer.len() as u32)?;
        w.extend_from_slice(&self.recv_buffer);

        Ok(w)
    }
}

#[derive(Debug)]
#[allow(non_camel_case_types, dead_code)]
struct GetDeviceTypeId_Call {
    context: Context,
    reader_name: String,
}

impl GetDeviceTypeId_Call {
    fn decode(payload: &mut Payload) -> RdpResult<Self> {
        let _header = RPCEStreamHeader::decode(payload)?;
        let _header = RPCETypeHeader::decode(payload)?;

        let mut index = 0;
        let mut context = Context::decode_ptr(payload, &mut index)?;

        let _reader_ptr = decode_ptr(payload, &mut index)?;

        context.decode_value(payload)?;
        let reader_name = decode_string_unicode(payload)?;
        Ok(Self {
            context,
            reader_name,
        })
    }
}

#[derive(Debug)]
#[allow(non_camel_case_types)]
struct GetDeviceTypeId_Return {
    return_code: ReturnCode,
    device_type_id: u32,
}

impl GetDeviceTypeId_Return {
    fn new(return_code: ReturnCode) -> Self {
        // Reader type describes the type of the physical connection to the smartcard reader (e.g.
        // USB/serial/TPM). Type "vendor" means a proprietary vendor bus.
        //
        // See "ReaderType" in
        // https://docs.microsoft.com/en-us/windows-hardware/drivers/ddi/smclib/ns-smclib-_scard_reader_capabilities
        const SCARD_READER_TYPE_VENDOR: u32 = 0xF0;
        Self {
            return_code,
            device_type_id: SCARD_READER_TYPE_VENDOR,
        }
    }
    fn encode(&self) -> RdpResult<Vec<u8>> {
        let mut w = vec![];
        w.write_u32::<LittleEndian>(self.return_code.to_u32().unwrap())?;
        w.write_u32::<LittleEndian>(self.device_type_id)?;
        Ok(w)
    }
}

#[derive(Debug)]
#[allow(non_camel_case_types)]
struct ReadCache_Call {
    lookup_name: String,
    common: ReadCache_Common,
}

impl ReadCache_Call {
    fn decode(payload: &mut Payload) -> RdpResult<Self> {
        let _header = RPCEStreamHeader::decode(payload)?;
        let _header = RPCETypeHeader::decode(payload)?;

        let mut index = 0;
        let _lookup_name_ptr = decode_ptr(payload, &mut index)?;
        let mut common = ReadCache_Common::decode_ptr(payload, &mut index)?;

        let lookup_name = decode_string_unicode(payload)?;
        common.decode_value(payload)?;
        Ok(Self {
            lookup_name,
            common,
        })
    }
}

#[derive(Debug)]
#[allow(non_camel_case_types, dead_code)]
struct ReadCache_Common {
    context: Context,
    card_uuid: Vec<u8>,
    freshness_counter: u32,
    data_is_null: bool,
    data_len: u32,
}

impl ReadCache_Common {
    fn decode_ptr(payload: &mut Payload, index: &mut u32) -> RdpResult<Self> {
        let context = Context::decode_ptr(payload, index)?;
        let _card_uuid_ptr = decode_ptr(payload, index)?;

        let freshness_counter = payload.read_u32::<LittleEndian>()?;
        let data_is_null = payload.read_i32::<LittleEndian>()? == 1;
        let data_len = payload.read_u32::<LittleEndian>()?;

        Ok(Self {
            context,
            card_uuid: vec![],
            freshness_counter,
            data_is_null,
            data_len,
        })
    }

    fn decode_value(&mut self, payload: &mut Payload) -> RdpResult<()> {
        self.context.decode_value(payload)?;
        self.card_uuid.resize(16, 0); // 16 bytes for UUID.
        payload.read_exact(&mut self.card_uuid)?;
        Ok(())
    }
}

#[derive(Debug)]
#[allow(non_camel_case_types)]
struct ReadCache_Return {
    return_code: ReturnCode,
    data: Vec<u8>,
}

impl ReadCache_Return {
    fn new(val: Option<Vec<u8>>) -> Self {
        match val {
            None => Self {
                return_code: ReturnCode::SCARD_W_CACHE_ITEM_NOT_FOUND,
                data: vec![],
            },
            Some(data) => Self {
                return_code: ReturnCode::SCARD_S_SUCCESS,
                data,
            },
        }
    }
    fn encode(&self) -> RdpResult<Vec<u8>> {
        let mut w = vec![];
        w.write_u32::<LittleEndian>(self.return_code.to_u32().unwrap())?;

        let mut index = 0;
        encode_ptr(self.data.length() as u32, &mut index, &mut w)?;
        w.write_u32::<LittleEndian>(self.data.length() as u32)?;
        w.extend_from_slice(&self.data);
        Ok(w)
    }
}

#[derive(Debug)]
#[allow(non_camel_case_types)]
struct WriteCache_Call {
    lookup_name: String,
    common: WriteCache_Common,
}

impl WriteCache_Call {
    fn decode(payload: &mut Payload) -> RdpResult<Self> {
        let _header = RPCEStreamHeader::decode(payload)?;
        let _header = RPCETypeHeader::decode(payload)?;

        let mut index = 0;
        let _lookup_name_ptr = decode_ptr(payload, &mut index)?;
        let mut common = WriteCache_Common::decode_ptr(payload, &mut index)?;

        let lookup_name = decode_string_unicode(payload)?;
        common.decode_value(payload)?;
        Ok(Self {
            lookup_name,
            common,
        })
    }
}

#[derive(Debug)]
#[allow(non_camel_case_types, dead_code)]
struct WriteCache_Common {
    context: Context,
    card_uuid: Vec<u8>,
    freshness_counter: u32,
    data: Vec<u8>,
}

impl WriteCache_Common {
    fn decode_ptr(payload: &mut Payload, index: &mut u32) -> RdpResult<Self> {
        let context = Context::decode_ptr(payload, index)?;
        let _card_uuid_ptr = decode_ptr(payload, index)?;
        let freshness_counter = payload.read_u32::<LittleEndian>()?;
        let _data_len = payload.read_u32::<LittleEndian>()?;
        let _data_ptr = decode_ptr(payload, index)?;

        Ok(Self {
            context,
            card_uuid: vec![],
            freshness_counter,
            data: vec![],
        })
    }

    fn decode_value(&mut self, payload: &mut Payload) -> RdpResult<()> {
        self.context.decode_value(payload)?;
        self.card_uuid.resize(16, 0); // 16 bytes for UUID.
        payload.read_exact(&mut self.card_uuid)?;

        let data_len = payload.read_u32::<LittleEndian>()?;
        self.data.resize(data_len as usize, 0);
        payload.read_exact(&mut self.data)?;

        Ok(())
    }
}

#[derive(Debug)]
#[allow(non_camel_case_types, dead_code)]
struct GetReaderIcon_Call {
    context: Context,
    reader_name: String,
}

impl GetReaderIcon_Call {
    fn decode(payload: &mut Payload) -> RdpResult<Self> {
        let _header = RPCEStreamHeader::decode(payload)?;
        let _header = RPCETypeHeader::decode(payload)?;

        let mut index = 0;
        let mut context = Context::decode_ptr(payload, &mut index)?;

        let _reader_ptr = decode_ptr(payload, &mut index)?;

        context.decode_value(payload)?;
        let reader_name = decode_string_unicode(payload)?;
        Ok(Self {
            context,
            reader_name,
        })
    }
}

#[derive(Debug)]
#[allow(non_camel_case_types)]
struct GetReaderIcon_Return {
    return_code: ReturnCode,
}

impl GetReaderIcon_Return {
    fn new(return_code: ReturnCode) -> Self {
        Self { return_code }
    }
    fn encode(&self) -> RdpResult<Vec<u8>> {
        let mut w = vec![];
        w.write_u32::<LittleEndian>(self.return_code.to_u32().unwrap())?;

        // Encode empty data field, reader icon not implemented.
        // TODO: send Teleport/Pam logo.
        let mut index = 0;
        encode_ptr(0, &mut index, &mut w)?;
        w.write_u32::<LittleEndian>(0)?;
        Ok(w)
    }
}

#[derive(Debug)]
struct Contexts {
    contexts: HashMap<u32, ContextInternal>,
    next_id: u32,
}

impl Contexts {
    fn new() -> Self {
        Self {
            next_id: 1,
            contexts: HashMap::new(),
        }
    }

    fn establish(&mut self) -> Context {
        let ctx_internal = ContextInternal::new();
        let id = self.next_id;
        self.next_id += 1;
        let ctx = Context::new(id);
        self.contexts.insert(id, ctx_internal);
        ctx
    }

    fn get(&mut self, id: u32) -> Option<&mut ContextInternal> {
        self.contexts.get_mut(&id)
    }

    fn release(&mut self, id: u32) {
        self.contexts.remove(&id);
    }
}

#[derive(Debug)]
struct ContextInternal {
    handles: HashMap<u32, piv::Card<TRANSMIT_DATA_LIMIT>>,
    next_id: u32,
    cache: HashMap<String, Vec<u8>>,
}

impl ContextInternal {
    fn new() -> Self {
        Self {
            next_id: 1,
            handles: HashMap::new(),
            cache: HashMap::new(),
        }
    }

    fn connect(
        &mut self,
        ctx: Context,
        uuid: Uuid,
        cert_der: &[u8],
        key_der: &[u8],
    ) -> RdpResult<Handle> {
        let card = piv::Card::new(uuid, cert_der, key_der)?;
        let id = self.next_id;
        self.next_id += 1;
        let handle = Handle::new(ctx, id);
        self.handles.insert(id, card);
        Ok(handle)
    }

    fn get(&mut self, id: u32) -> Option<&mut piv::Card<TRANSMIT_DATA_LIMIT>> {
        self.handles.get_mut(&id)
    }

    fn disconnect(&mut self, id: u32) {
        self.handles.remove(&id);
    }

    fn cache_read(&self, key: &str) -> Option<Vec<u8>> {
        self.cache.get(key).cloned()
    }

    fn cache_write(&mut self, key: String, val: Vec<u8>) {
        self.cache.insert(key, val);
    }
}

#[allow(dead_code)]
// A little helper function for debugging unparsed payloads.
fn debug_print_payload(payload: &mut Payload) {
    let payload = payload.clone();
    let from = payload.position() as usize;
    let buf = &payload.into_inner()[from..];
    info!("========== payload {:?}", &buf);
}
