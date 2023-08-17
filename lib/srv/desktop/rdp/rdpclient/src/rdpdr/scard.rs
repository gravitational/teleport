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

use crate::errors::{invalid_data_error, not_implemented_error};
use crate::rdpdr::consts::NTSTATUS;
use crate::{piv, Message};
use crate::{Encode, Payload};
use bitflags::bitflags;
use byteorder::{LittleEndian, ReadBytesExt, WriteBytesExt};
use iso7816::command::Command as CardCommand;
use num_traits::{FromPrimitive, ToPrimitive};
use rdp::model::data::Message as MessageTrait;
use rdp::model::error::*;
use std::char::{decode_utf16, REPLACEMENT_CHARACTER};
use std::collections::HashMap;
use std::convert::TryInto;
use std::io::{Read, Write};
use std::vec;
use uuid::Uuid;

use super::{DeviceControlRequest, DeviceControlResponse};

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
    pin: String,
}

impl Client {
    pub fn new(cert_der: Vec<u8>, key_der: Vec<u8>, pin: String) -> Self {
        Self {
            contexts: Contexts::new(),
            uuid: Uuid::new_v4(),
            cert_der,
            key_der,
            pin,
        }
    }

    // ioctl handles messages coming from the RDP server over the RDPDR channel.
    pub(super) fn ioctl(
        &mut self,
        ioctl: &DeviceControlRequest,
        input: &mut Payload,
    ) -> RdpResult<Vec<DeviceControlResponse>> {
        debug!("got IoctlCode {:?}", &ioctl.io_control_code);
        // Note: this is an incomplete implementation of the scard API.
        // It's the bare minimum needed to make RDP authentication using a smartcard work.
        //
        // Particularly, we only implement the Unicode IOCTL variants. All Ascii variants will
        // fail, but most modern Windows hosts shouldn't call those. If you're reading this because
        // some SCARD_IOCTL_*A call is failing, I was wrong and you'll have to implement the Ascii
        // calls.
        match ioctl.io_control_code {
            IoctlCode::SCARD_IOCTL_ACCESSSTARTEDEVENT => {
                self.handle_access_started_event(ioctl, input)
            }
            IoctlCode::SCARD_IOCTL_ESTABLISHCONTEXT => self.handle_establish_context(ioctl, input),
            IoctlCode::SCARD_IOCTL_RELEASECONTEXT => self.handle_release_context(ioctl, input),
            IoctlCode::SCARD_IOCTL_CANCEL => self.handle_cancel(ioctl, input),
            IoctlCode::SCARD_IOCTL_ISVALIDCONTEXT => self.handle_is_valid_context(ioctl, input),
            IoctlCode::SCARD_IOCTL_LISTREADERSW => self.handle_list_readers(ioctl, input),
            IoctlCode::SCARD_IOCTL_GETSTATUSCHANGEW => self.handle_get_status_change(ioctl, input),
            IoctlCode::SCARD_IOCTL_CONNECTW => self.handle_connect(ioctl, input),
            IoctlCode::SCARD_IOCTL_DISCONNECT => self.handle_disconnect(ioctl, input),
            IoctlCode::SCARD_IOCTL_BEGINTRANSACTION => self.handle_begin_transaction(ioctl, input),
            IoctlCode::SCARD_IOCTL_ENDTRANSACTION => self.handle_end_transaction(ioctl, input),
            IoctlCode::SCARD_IOCTL_STATUSA => {
                self.handle_status(ioctl, input, StringEncoding::Ascii)
            }
            IoctlCode::SCARD_IOCTL_STATUSW => {
                self.handle_status(ioctl, input, StringEncoding::Unicode)
            }
            // Transmit is where communication with the actual smartcard (and the PIV application
            // on it) happens. All other messages are managing the smartcard reader and
            // establishing a connection to the smartcard.
            IoctlCode::SCARD_IOCTL_TRANSMIT => self.handle_transmit(ioctl, input),
            IoctlCode::SCARD_IOCTL_GETDEVICETYPEID => self.handle_get_device_type_id(ioctl, input),
            // Note: we keep an in-memory hashmap as a cache to implement these commands. Windows
            // doesn't seem to like a smartcard without a functioning cache.
            IoctlCode::SCARD_IOCTL_READCACHEW => self.handle_read_cache(ioctl, input),
            IoctlCode::SCARD_IOCTL_WRITECACHEW => self.handle_write_cache(ioctl, input),
            IoctlCode::SCARD_IOCTL_GETREADERICON => self.handle_get_reader_icon(ioctl, input),
            _ => self.handle_unimplemented_ioctl(ioctl, ioctl.io_control_code),
        }
    }

    fn handle_access_started_event(
        &self,
        ioctl: &DeviceControlRequest,
        input: &mut Payload,
    ) -> RdpResult<Vec<DeviceControlResponse>> {
        let req = ScardAccessStartedEvent_Call::decode(input)?;
        debug!("got {:?}", req);
        let resp = Long_Return::new(ReturnCode::SCARD_S_SUCCESS);
        debug!("sending {:?}", resp);
        Ok(vec![DeviceControlResponse::new(
            ioctl,
            NTSTATUS::STATUS_SUCCESS,
            Box::new(resp),
        )])
    }

    fn handle_establish_context(
        &mut self,
        ioctl: &DeviceControlRequest,
        input: &mut Payload,
    ) -> RdpResult<Vec<DeviceControlResponse>> {
        let req = EstablishContext_Call::decode(input)?;
        debug!("got {:?}", req);
        let ctx = self.contexts.establish();
        let resp = EstablishContext_Return::new(ReturnCode::SCARD_S_SUCCESS, ctx);
        debug!("sending {:?}", resp);
        Ok(vec![DeviceControlResponse::new(
            ioctl,
            NTSTATUS::STATUS_SUCCESS,
            Box::new(resp),
        )])
    }

    fn handle_release_context(
        &mut self,
        ioctl: &DeviceControlRequest,
        input: &mut Payload,
    ) -> RdpResult<Vec<DeviceControlResponse>> {
        let req = Context_Call::decode(input)?;
        debug!("got {:?}", req);
        self.contexts.release(req.context.value);
        let resp = Long_Return::new(ReturnCode::SCARD_S_SUCCESS);
        debug!("sending {:?}", resp);
        Ok(vec![DeviceControlResponse::new(
            ioctl,
            NTSTATUS::STATUS_SUCCESS,
            Box::new(resp),
        )])
    }

    fn handle_cancel(
        &mut self,
        ioctl: &DeviceControlRequest,
        input: &mut Payload,
    ) -> RdpResult<Vec<DeviceControlResponse>> {
        let mut responses = vec![];
        let req = Context_Call::decode(input)?;
        debug!("got {:?}", req);

        // Fetch the pending SCARD_IOCTL_GETSTATUSCHANGEW response and add it to the responses.
        if let Some(dcr) = self
            .contexts
            .get(req.context.value)?
            .scard_cancel_response
            .take()
        {
            responses.push(dcr);
        } else {
            warn!("Received SCARD_IOCTL_CANCEL for a context without a pending SCARD_IOCTL_GETSTATUSCHANGEW.")
        }

        // Also add the response to the SCARD_IOCTL_CANCEL request.
        responses.push(DeviceControlResponse::new(
            ioctl,
            NTSTATUS::STATUS_SUCCESS,
            Box::new(Long_Return::new(ReturnCode::SCARD_S_SUCCESS)),
        ));
        debug!("sending {:?}", responses);
        Ok(responses)
    }

    fn handle_is_valid_context(
        &self,
        ioctl: &DeviceControlRequest,
        input: &mut Payload,
    ) -> RdpResult<Vec<DeviceControlResponse>> {
        let req = Context_Call::decode(input)?;
        debug!("got {:?}", req);
        let resp = Long_Return::new(ReturnCode::SCARD_S_SUCCESS);
        debug!("sending {:?}", resp);
        Ok(vec![DeviceControlResponse::new(
            ioctl,
            NTSTATUS::STATUS_SUCCESS,
            Box::new(resp),
        )])
    }

    fn handle_list_readers(
        &self,
        ioctl: &DeviceControlRequest,
        input: &mut Payload,
    ) -> RdpResult<Vec<DeviceControlResponse>> {
        let req = ListReaders_Call::decode(input)?;
        debug!("got {:?}", req);
        let resp =
            ListReaders_Return::new(ReturnCode::SCARD_S_SUCCESS, vec!["Teleport".to_string()]);
        debug!("sending {:?}", resp);
        Ok(vec![DeviceControlResponse::new(
            ioctl,
            NTSTATUS::STATUS_SUCCESS,
            Box::new(resp),
        )])
    }

    fn handle_get_status_change(
        &mut self,
        ioctl: &DeviceControlRequest,
        input: &mut Payload,
    ) -> RdpResult<Vec<DeviceControlResponse>> {
        let req = GetStatusChange_Call::decode(input)?;
        let timeout = req.timeout;
        let context_value = req.context.value;
        debug!("got {:?}", req);

        if timeout != TIMEOUT_INFINITE && timeout != TIMEOUT_IMMEDIATE {
            // We've never seen one of these but we log a warning here in case we ever come
            // across one and need to debug a related issue.
            warn!(
                "logic for a non-infinite/non-immediate timeout [{}] is not implemented",
                timeout
            );
        }

        let mut resp = GetStatusChange_Return::new(ReturnCode::SCARD_S_SUCCESS, req);
        if resp.no_change() {
            if timeout != TIMEOUT_INFINITE {
                return Err(not_implemented_error(&format!(
                    "no change for non-infinite timeout [{}] is not implemented",
                    timeout
                )));
            }

            // Received a GetStatusChange_Call with an infinite timeout, so we're adding
            // a corresponding DeviceControlResponse request holding a GetStatusChange_Return
            // with its return code set to SCARD_E_CANCELLED to this Context. This value will
            // be returned when we get an SCARD_IOCTL_CANCEL call for this Context.
            resp.set_return_code(ReturnCode::SCARD_E_CANCELLED);
            self.contexts
                .get(context_value)?
                .set_scard_cancel_response(DeviceControlResponse::new(
                    ioctl,
                    NTSTATUS::STATUS_SUCCESS,
                    Box::new(resp),
                ))?;
            debug!("blocking GetStatusChange call indefinitely (since our status never changes) until we receive an SCARD_IOCTL_CANCEL");
            Ok(vec![])
        } else {
            debug!("sending {:?}", resp);
            Ok(vec![DeviceControlResponse::new(
                ioctl,
                NTSTATUS::STATUS_SUCCESS,
                Box::new(resp),
            )])
        }
    }

    fn handle_connect(
        &mut self,
        ioctl: &DeviceControlRequest,
        input: &mut Payload,
    ) -> RdpResult<Vec<DeviceControlResponse>> {
        let req = Connect_Call::decode(input)?;
        debug!("got {:?}", req);

        let ctx = self.contexts.get(req.common.context.value)?;
        let handle = ctx.connect(
            req.common.context,
            self.uuid,
            &self.cert_der,
            &self.key_der,
            self.pin.clone(),
        )?;

        let resp = Connect_Return::new(ReturnCode::SCARD_S_SUCCESS, handle);
        debug!("sending {:?}", resp);
        Ok(vec![DeviceControlResponse::new(
            ioctl,
            NTSTATUS::STATUS_SUCCESS,
            Box::new(resp),
        )])
    }

    fn handle_disconnect(
        &mut self,
        ioctl: &DeviceControlRequest,
        input: &mut Payload,
    ) -> RdpResult<Vec<DeviceControlResponse>> {
        let req = HCardAndDisposition_Call::decode(input)?;
        debug!("got {:?}", req);

        self.contexts
            .get(req.handle.context.value)?
            .disconnect(req.handle.value);

        let resp = Long_Return::new(ReturnCode::SCARD_S_SUCCESS);
        debug!("sending {:?}", resp);
        Ok(vec![DeviceControlResponse::new(
            ioctl,
            NTSTATUS::STATUS_SUCCESS,
            Box::new(resp),
        )])
    }

    fn handle_begin_transaction(
        &self,
        ioctl: &DeviceControlRequest,
        input: &mut Payload,
    ) -> RdpResult<Vec<DeviceControlResponse>> {
        let req = HCardAndDisposition_Call::decode(input)?;
        debug!("got {:?}", req);
        let resp = Long_Return::new(ReturnCode::SCARD_S_SUCCESS);
        debug!("sending {:?}", resp);
        Ok(vec![DeviceControlResponse::new(
            ioctl,
            NTSTATUS::STATUS_SUCCESS,
            Box::new(resp),
        )])
    }

    fn handle_end_transaction(
        &self,
        ioctl: &DeviceControlRequest,
        input: &mut Payload,
    ) -> RdpResult<Vec<DeviceControlResponse>> {
        let req = HCardAndDisposition_Call::decode(input)?;
        debug!("got {:?}", req);
        let resp = Long_Return::new(ReturnCode::SCARD_S_SUCCESS);
        debug!("sending {:?}", resp);
        Ok(vec![DeviceControlResponse::new(
            ioctl,
            NTSTATUS::STATUS_SUCCESS,
            Box::new(resp),
        )])
    }

    fn handle_status(
        &self,
        ioctl: &DeviceControlRequest,
        input: &mut Payload,
        enc: StringEncoding,
    ) -> RdpResult<Vec<DeviceControlResponse>> {
        let req = Status_Call::decode(input)?;
        debug!("got {:?}", req);
        let resp = Status_Return::new(
            ReturnCode::SCARD_S_SUCCESS,
            vec!["Teleport".to_string()],
            enc,
        );
        debug!("sending {:?}", resp);
        Ok(vec![DeviceControlResponse::new(
            ioctl,
            NTSTATUS::STATUS_SUCCESS,
            Box::new(resp),
        )])
    }

    fn handle_transmit(
        &mut self,
        ioctl: &DeviceControlRequest,
        input: &mut Payload,
    ) -> RdpResult<Vec<DeviceControlResponse>> {
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
            .get(req.handle.context.value)?
            .get(req.handle.value)
            .ok_or_else(|| invalid_data_error("unknown handle ID"))?;

        let resp = card.handle(cmd)?;

        let resp = Transmit_Return::new(ReturnCode::SCARD_S_SUCCESS, resp.encode());
        debug!("sending {:?}", resp);
        Ok(vec![DeviceControlResponse::new(
            ioctl,
            NTSTATUS::STATUS_SUCCESS,
            Box::new(resp),
        )])
    }

    fn handle_get_device_type_id(
        &mut self,
        ioctl: &DeviceControlRequest,
        input: &mut Payload,
    ) -> RdpResult<Vec<DeviceControlResponse>> {
        let req = GetDeviceTypeId_Call::decode(input)?;
        debug!("got {:?}", req);

        let _ctx = self.contexts.get(req.context.value)?;

        let resp = GetDeviceTypeId_Return::new(ReturnCode::SCARD_S_SUCCESS);
        debug!("sending {:?}", resp);
        Ok(vec![DeviceControlResponse::new(
            ioctl,
            NTSTATUS::STATUS_SUCCESS,
            Box::new(resp),
        )])
    }

    fn handle_read_cache(
        &mut self,
        ioctl: &DeviceControlRequest,
        input: &mut Payload,
    ) -> RdpResult<Vec<DeviceControlResponse>> {
        let req = ReadCache_Call::decode(input)?;
        debug!("got {:?}", req);

        let val = self
            .contexts
            .get(req.common.context.value)?
            .cache_read(&req.lookup_name);

        let resp = ReadCache_Return::new(val);
        debug!("sending {:?}", resp);
        Ok(vec![DeviceControlResponse::new(
            ioctl,
            NTSTATUS::STATUS_SUCCESS,
            Box::new(resp),
        )])
    }

    fn handle_write_cache(
        &mut self,
        ioctl: &DeviceControlRequest,
        input: &mut Payload,
    ) -> RdpResult<Vec<DeviceControlResponse>> {
        let req = WriteCache_Call::decode(input)?;
        debug!("got {:?}", req);

        self.contexts
            .get(req.common.context.value)?
            .cache_write(req.lookup_name, req.common.data);

        let resp = Long_Return::new(ReturnCode::SCARD_S_SUCCESS);
        debug!("sending {:?}", resp);
        Ok(vec![DeviceControlResponse::new(
            ioctl,
            NTSTATUS::STATUS_SUCCESS,
            Box::new(resp),
        )])
    }

    fn handle_get_reader_icon(
        &mut self,
        ioctl: &DeviceControlRequest,
        input: &mut Payload,
    ) -> RdpResult<Vec<DeviceControlResponse>> {
        let req = GetReaderIcon_Call::decode(input)?;
        debug!("got {:?}", req);

        let _ctx = self.contexts.get(req.context.value)?;

        let resp = GetReaderIcon_Return::new(ReturnCode::SCARD_E_UNSUPPORTED_FEATURE);
        debug!("sending {:?}", resp);
        Ok(vec![DeviceControlResponse::new(
            ioctl,
            NTSTATUS::STATUS_SUCCESS,
            Box::new(resp),
        )])
    }

    fn handle_unimplemented_ioctl(
        &self,
        ioctl: &DeviceControlRequest,
        code: IoctlCode,
    ) -> RdpResult<Vec<DeviceControlResponse>> {
        warn!("unimplemented IOCTL: {:?}", code);
        let resp = Long_Return::new(ReturnCode::SCARD_F_INTERNAL_ERROR);
        debug!("sending {:?}", resp);
        Ok(vec![DeviceControlResponse::new(
            ioctl,
            NTSTATUS::STATUS_SUCCESS,
            Box::new(resp),
        )])
    }
}

// TRANSMIT_DATA_LIMIT is the maximum size of transmit request/response short data, in bytes.
const TRANSMIT_DATA_LIMIT: usize = 1024;

#[derive(Debug, FromPrimitive, ToPrimitive, Copy, Clone)]
#[allow(non_camel_case_types)]
pub enum IoctlCode {
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
    fn encode(&self) -> RdpResult<Message> {
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

fn pad_and_add_headers(resp: Vec<u8>) -> RdpResult<Vec<u8>> {
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
    _unused: u32,
}

impl ScardAccessStartedEvent_Call {
    fn decode(payload: &mut Payload) -> RdpResult<Self> {
        Ok(Self {
            _unused: payload.read_u32::<LittleEndian>()?,
        })
    }
}

impl Encode for ScardAccessStartedEvent_Call {
    fn encode(&self) -> RdpResult<Message> {
        let mut w = vec![];
        w.write_u32::<LittleEndian>(self._unused)?;
        Ok(w)
    }
}

const TIMEOUT_INFINITE: u32 = 0xffffffff;
const TIMEOUT_IMMEDIATE: u32 = 0;

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
}

impl Encode for Long_Return {
    fn encode(&self) -> RdpResult<Message> {
        let mut w = vec![];
        w.write_u32::<LittleEndian>(self.return_code.to_u32().unwrap())?;
        pad_and_add_headers(w)
    }
}

#[derive(Debug)]
#[allow(dead_code, non_camel_case_types)]
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
                invalid_data_error(&format!("invalid smart card scope {scope:?}"))
            })?,
        })
    }
}

impl Encode for EstablishContext_Call {
    fn encode(&self) -> RdpResult<Message> {
        let mut w = vec![];
        w.extend(RPCEStreamHeader::new().encode()?);
        RPCETypeHeader::new(0).encode(&mut w)?;
        w.write_u32::<LittleEndian>(self.scope as u32)?;
        Ok(w)
    }
}

#[derive(Debug, FromPrimitive, ToPrimitive, Copy, Clone)]
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
}

impl Encode for EstablishContext_Return {
    fn encode(&self) -> RdpResult<Message> {
        let mut w = vec![];
        w.write_u32::<LittleEndian>(self.return_code.to_u32().unwrap())?;
        let mut index = 0;
        self.context.encode_ptr(&mut index, &mut w)?;
        self.context.encode_value(&mut w)?;
        pad_and_add_headers(w)
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
        encode_ptr(Some(self.length), index, w)
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
fn encode_ptr(length: Option<u32>, index: &mut u32, w: &mut dyn Write) -> RdpResult<()> {
    if let Some(length) = length {
        w.write_u32::<LittleEndian>(length)?;
    }
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
            "invalid NDR pointer value {ptr:#010X}, expected {expect_ptr:#010X}"
        )))
    } else {
        Ok(ptr)
    }
}

#[derive(Debug)]
#[allow(dead_code, non_camel_case_types)]
struct ListReaders_Call {
    context: Context,
    groups_ptr_length: u32,
    groups_length: u32,
    groups_ptr: u32,
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
                groups_ptr_length,
                groups_ptr,
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
                groups_ptr_length,
                groups_ptr,
                groups_length,
                groups,
                readers_is_null,
                readers_size,
            })
        }
    }
}

impl Encode for ListReaders_Call {
    fn encode(&self) -> RdpResult<Message> {
        let mut w = vec![];

        w.extend(RPCEStreamHeader::new().encode()?);
        RPCETypeHeader::new(0).encode(&mut w)?;

        let mut index = 0;
        self.context.encode_ptr(&mut index, &mut w)?;
        encode_ptr(Some(self.groups_ptr_length), &mut index, &mut w)?; // takes care of encoding groups_ptr
        let readers_is_null = u32::from(self.readers_is_null);
        w.write_u32::<LittleEndian>(readers_is_null)?;
        w.write_u32::<LittleEndian>(self.readers_size)?;

        self.context.encode_value(&mut w)?;

        w.write_u32::<LittleEndian>(self.groups_length)?;
        w.extend(encode_multistring_unicode(&self.groups)?);

        Ok(w)
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
}

impl Encode for ListReaders_Return {
    fn encode(&self) -> RdpResult<Message> {
        let mut w = vec![];
        w.write_u32::<LittleEndian>(self.return_code.to_u32().unwrap())?;
        let readers = encode_multistring_unicode(&self.readers)?;
        let mut index = 0;
        encode_ptr(Some(readers.length() as u32), &mut index, &mut w)?;

        w.write_u32::<LittleEndian>(readers.length() as u32)?;
        w.extend_from_slice(&readers);

        pad_and_add_headers(w)
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

fn encode_str_unicode(s: &str) -> RdpResult<Vec<u8>> {
    let mut buf = vec![];

    // It's not exactly clear what the purpose of these length/offset fields are,
    // but they're expected for single unicode strings.
    let len = s.len() as u32 + 1; // +1 for the null terminator.
    buf.write_u32::<LittleEndian>(len)?;
    buf.write_u32::<LittleEndian>(0)?;
    buf.write_u32::<LittleEndian>(len)?;

    for c in s.encode_utf16() {
        buf.write_u16::<LittleEndian>(c)?;
    }
    buf.write_u16::<LittleEndian>(0)?;

    if (len - 1) % 2 == 0 {
        // Add extra padding for a 4-byte aligned NULL-terminated string.
        buf.write_u16::<LittleEndian>(0)?;
    }

    Ok(buf)
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

impl Encode for Context_Call {
    fn encode(&self) -> RdpResult<Message> {
        let mut w = vec![];

        w.extend(RPCEStreamHeader::new().encode()?);
        RPCETypeHeader::new(0).encode(&mut w)?;

        let mut index = 0;
        self.context.encode_ptr(&mut index, &mut w)?;
        self.context.encode_value(&mut w)?;

        Ok(w)
    }
}

#[derive(Debug)]
#[allow(dead_code, non_camel_case_types)]
struct GetStatusChange_Call {
    context: Context,
    timeout: u32,
    states_ptr_length: u32,
    states_ptr: u32,
    states_length: u32,
    states: Vec<ReaderState>,
}

impl GetStatusChange_Call {
    fn decode(payload: &mut Payload) -> RdpResult<Self> {
        let _header = RPCEStreamHeader::decode(payload)?;
        let _header = RPCETypeHeader::decode(payload)?;

        let mut index = 0;
        let mut context = Context::decode_ptr(payload, &mut index)?;

        let timeout = payload.read_u32::<LittleEndian>()?;
        let states_ptr_length = payload.read_u32::<LittleEndian>()?;
        let states_ptr = decode_ptr(payload, &mut index)?;

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
            states_ptr_length,
            states_ptr,
            states_length,
            states,
        })
    }
}

impl Encode for GetStatusChange_Call {
    fn encode(&self) -> RdpResult<Message> {
        let mut w = vec![];

        w.extend(RPCEStreamHeader::new().encode()?);
        RPCETypeHeader::new(0).encode(&mut w)?;

        let mut index = 0;
        self.context.encode_ptr(&mut index, &mut w)?;

        w.write_u32::<LittleEndian>(self.timeout)?;
        encode_ptr(Some(self.states_ptr_length), &mut index, &mut w)?; // takes care of encoding states_ptr

        self.context.encode_value(&mut w)?;

        w.write_u32::<LittleEndian>(self.states_length)?;
        for state in &self.states {
            state.encode_ptr(&mut index, &mut w)?;
        }
        for state in &self.states {
            state.encode_value(&mut w)?;
        }

        Ok(w)
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

    fn encode_ptr(&self, index: &mut u32, w: &mut Vec<u8>) -> RdpResult<()> {
        encode_ptr(None, index, w)?;
        self.common.encode(w)?;
        Ok(())
    }

    fn encode_value(&self, w: &mut Vec<u8>) -> RdpResult<()> {
        w.extend(encode_str_unicode(&self.reader)?);
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
    #[derive(Debug, PartialEq, Clone, Copy)]
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

    fn set_return_code(&mut self, return_code: ReturnCode) {
        self.return_code = return_code;
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

impl Encode for GetStatusChange_Return {
    fn encode(&self) -> RdpResult<Message> {
        let mut w = vec![];
        w.write_u32::<LittleEndian>(self.return_code.to_u32().unwrap())?;
        let mut index = 0;
        encode_ptr(Some(self.reader_states.len() as u32), &mut index, &mut w)?;

        w.write_u32::<LittleEndian>(self.reader_states.len() as u32)?;
        for state in &self.reader_states {
            state.encode(&mut w)?;
        }

        pad_and_add_headers(w)
    }
}

#[derive(Debug)]
#[allow(dead_code, non_camel_case_types)]
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

impl Encode for Connect_Call {
    fn encode(&self) -> RdpResult<Message> {
        let mut w = vec![];

        w.extend(RPCEStreamHeader::new().encode()?);
        RPCETypeHeader::new(0).encode(&mut w)?;

        let mut index = 0;
        encode_ptr(None, &mut index, &mut w)?;
        self.common.encode_ptr(&mut index, &mut w)?;
        w.extend(encode_str_unicode(&self.reader)?);
        self.common.encode_value(&mut w)?;

        Ok(w)
    }
}

bitflags! {
    #[derive(Debug, Clone)]
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
#[allow(dead_code, non_camel_case_types)]
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

    fn encode_ptr(&self, index: &mut u32, w: &mut dyn Write) -> RdpResult<()> {
        self.context.encode_ptr(index, w)?;
        w.write_u32::<LittleEndian>(self.share_mode)?;
        w.write_u32::<LittleEndian>(self.preferred_protocols.bits())?;
        Ok(())
    }

    fn encode_value(&self, w: &mut dyn Write) -> RdpResult<()> {
        self.context.encode_value(w)
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
}

impl Encode for Connect_Return {
    fn encode(&self) -> RdpResult<Message> {
        let mut w = vec![];
        w.write_u32::<LittleEndian>(self.return_code.to_u32().unwrap())?;
        let mut index = 0;
        self.handle.encode_ptr(&mut index, &mut w)?;
        w.write_u32::<LittleEndian>(self.active_protocol.bits())?;
        self.handle.encode_value(&mut w)?;
        pad_and_add_headers(w)
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
        encode_ptr(Some(self.length), index, w)?;
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
#[allow(dead_code, non_camel_case_types)]
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

impl Encode for HCardAndDisposition_Call {
    fn encode(&self) -> RdpResult<Message> {
        let mut w = vec![];

        w.extend(RPCEStreamHeader::new().encode()?);
        RPCETypeHeader::new(0).encode(&mut w)?;

        let mut index = 0;
        self.handle.encode_ptr(&mut index, &mut w)?;
        w.write_u32::<LittleEndian>(self.disposition)?;
        self.handle.encode_value(&mut w)?;

        Ok(w)
    }
}

#[derive(Debug)]
#[allow(dead_code, non_camel_case_types)]
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

impl Encode for Status_Call {
    fn encode(&self) -> RdpResult<Message> {
        let mut w = vec![];

        w.extend(RPCEStreamHeader::new().encode()?);
        RPCETypeHeader::new(0).encode(&mut w)?;

        let mut index = 0;
        self.handle.encode_ptr(&mut index, &mut w)?;
        let reader_names_is_null = u32::from(self.reader_names_is_null);
        w.write_u32::<LittleEndian>(reader_names_is_null)?;
        w.write_u32::<LittleEndian>(self.reader_length)?;
        w.write_u32::<LittleEndian>(self.atr_length)?;
        self.handle.encode_value(&mut w)?;

        Ok(w)
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
}

impl Encode for Status_Return {
    fn encode(&self) -> RdpResult<Message> {
        let mut w = vec![];
        w.write_u32::<LittleEndian>(self.return_code.to_u32().unwrap())?;

        let reader_names = match &self.encoding {
            StringEncoding::Unicode => encode_multistring_unicode(&self.reader_names)?,
            StringEncoding::Ascii => encode_multistring_ascii(&self.reader_names)?,
        };
        let mut index = 0;
        encode_ptr(Some(reader_names.length() as u32), &mut index, &mut w)?;

        w.write_u32::<LittleEndian>(self.state.to_u32().unwrap())?;
        w.write_u32::<LittleEndian>(self.protocol.bits())?;
        w.extend_from_slice(&self.atr);
        w.write_u32::<LittleEndian>(self.atr_length)?;

        w.write_u32::<LittleEndian>(reader_names.length() as u32)?;
        w.extend_from_slice(&reader_names);

        pad_and_add_headers(w)
    }
}

#[derive(Debug)]
#[allow(dead_code, non_camel_case_types)]
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

impl Encode for Transmit_Call {
    fn encode(&self) -> RdpResult<Message> {
        let mut w = vec![];

        w.extend(RPCEStreamHeader::new().encode()?);
        RPCETypeHeader::new(0).encode(&mut w)?;

        let mut index = 0;
        self.handle.encode_ptr(&mut index, &mut w)?;
        self.send_pci.encode_ptr(&mut index, &mut w)?;
        w.write_u32::<LittleEndian>(0)?; // _send_length
        encode_ptr(None, &mut index, &mut w)?; // _send_buffer_ptr
                                               // recv_pci_ptr
        if let Some(_recv_pci) = self.recv_pci.clone() {
            encode_ptr(None, &mut index, &mut w)?;
        } else {
            w.write_u32::<LittleEndian>(0)?;
        }
        let recv_buffer_is_null = u32::from(self.recv_buffer_is_null);
        w.write_u32::<LittleEndian>(recv_buffer_is_null)?;
        w.write_u32::<LittleEndian>(self.recv_length)?;

        self.handle.encode_value(&mut w)?;
        self.send_pci.encode_value(&mut w)?;

        w.write_u32::<LittleEndian>(self.send_length)?;
        w.extend(self.send_buffer.clone());

        if let Some(recv_pci) = self.recv_pci.clone() {
            recv_pci.encode_ptr(&mut index, &mut w)?;
            recv_pci.encode_value(&mut w)?;
        }

        Ok(w)
    }
}

#[derive(Debug, Clone)]
#[allow(dead_code, non_camel_case_types)]
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

    fn encode_ptr(&self, index: &mut u32, w: &mut dyn Write) -> RdpResult<()> {
        w.write_u32::<LittleEndian>(self.protocol.bits())?;
        encode_ptr(Some(self.extra_bytes_length), index, w)
    }

    fn encode_value(&self, w: &mut Vec<u8>) -> RdpResult<()> {
        w.extend_from_slice(&self.extra_bytes);
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
}

impl Encode for Transmit_Return {
    fn encode(&self) -> RdpResult<Message> {
        let mut w = vec![];
        w.write_u32::<LittleEndian>(self.return_code.to_u32().unwrap())?;

        // There is a recv_pci (SCardIO_Request) field before recv_buffer, but it's always null in
        // our case.
        w.write_u32::<LittleEndian>(0)?;

        let mut index = 0;
        encode_ptr(Some(self.recv_buffer.len() as u32), &mut index, &mut w)?;
        w.write_u32::<LittleEndian>(self.recv_buffer.len() as u32)?;
        w.extend_from_slice(&self.recv_buffer);

        pad_and_add_headers(w)
    }
}

#[derive(Debug)]
#[allow(dead_code, non_camel_case_types)]
struct GetDeviceTypeId_Call {
    context: Context,
    reader_ptr: u32,
    reader_name: String,
}

impl GetDeviceTypeId_Call {
    fn decode(payload: &mut Payload) -> RdpResult<Self> {
        let _header = RPCEStreamHeader::decode(payload)?;
        let _header = RPCETypeHeader::decode(payload)?;

        let mut index = 0;
        let mut context = Context::decode_ptr(payload, &mut index)?;

        let reader_ptr = decode_ptr(payload, &mut index)?;

        context.decode_value(payload)?;
        let reader_name = decode_string_unicode(payload)?;
        Ok(Self {
            context,
            reader_ptr,
            reader_name,
        })
    }
}

impl Encode for GetDeviceTypeId_Call {
    fn encode(&self) -> RdpResult<Message> {
        let mut w = vec![];

        w.extend(RPCEStreamHeader::new().encode()?);
        RPCETypeHeader::new(0).encode(&mut w)?;

        let mut index = 0;
        self.context.encode_ptr(&mut index, &mut w)?;

        encode_ptr(None, &mut index, &mut w)?;

        self.context.encode_value(&mut w)?;

        let reader_name = encode_str_unicode(&self.reader_name)?;
        w.extend(reader_name);
        Ok(w)
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
}

impl Encode for GetDeviceTypeId_Return {
    fn encode(&self) -> RdpResult<Message> {
        let mut w = vec![];
        w.write_u32::<LittleEndian>(self.return_code.to_u32().unwrap())?;
        w.write_u32::<LittleEndian>(self.device_type_id)?;
        pad_and_add_headers(w)
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

impl Encode for ReadCache_Call {
    fn encode(&self) -> RdpResult<Message> {
        let mut w = vec![];

        w.extend(RPCEStreamHeader::new().encode()?);
        RPCETypeHeader::new(0).encode(&mut w)?;

        let mut index = 0;
        encode_ptr(None, &mut index, &mut w)?; // _send_buffer_ptr
        self.common.encode_ptr(&mut index, &mut w)?;

        w.extend(encode_str_unicode(&self.lookup_name)?);
        self.common.encode_value(&mut w)?;

        Ok(w)
    }
}

#[derive(Debug)]
#[allow(dead_code, non_camel_case_types)]
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

    fn encode_ptr(&self, index: &mut u32, w: &mut dyn Write) -> RdpResult<()> {
        self.context.encode_ptr(index, w)?;
        encode_ptr(None, index, w)?; // _card_uuid_ptr

        w.write_u32::<LittleEndian>(self.freshness_counter)?;
        let data_is_null = u32::from(self.data_is_null);
        w.write_u32::<LittleEndian>(data_is_null)?;
        w.write_u32::<LittleEndian>(self.data_len)?;

        Ok(())
    }

    fn encode_value(&self, w: &mut Vec<u8>) -> RdpResult<()> {
        self.context.encode_value(w)?;
        w.extend_from_slice(&self.card_uuid);
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
}

impl Encode for ReadCache_Return {
    fn encode(&self) -> RdpResult<Message> {
        let mut w = vec![];
        w.write_u32::<LittleEndian>(self.return_code.to_u32().unwrap())?;

        let mut index = 0;
        encode_ptr(Some(self.data.length() as u32), &mut index, &mut w)?;
        w.write_u32::<LittleEndian>(self.data.length() as u32)?;
        w.extend_from_slice(&self.data);
        pad_and_add_headers(w)
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

impl Encode for WriteCache_Call {
    fn encode(&self) -> RdpResult<Message> {
        let mut w = vec![];

        w.extend(RPCEStreamHeader::new().encode()?);
        RPCETypeHeader::new(0).encode(&mut w)?;

        let mut index = 0;
        encode_ptr(None, &mut index, &mut w)?; // _lookup_name_ptr
        self.common.encode_ptr(&mut index, &mut w)?;

        w.extend(encode_str_unicode(&self.lookup_name)?);
        self.common.encode_value(&mut w)?;

        Ok(w)
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

    fn encode_ptr(&self, index: &mut u32, w: &mut dyn Write) -> RdpResult<()> {
        self.context.encode_ptr(index, w)?;
        encode_ptr(None, index, w)?; // _card_uuid_ptr
        w.write_u32::<LittleEndian>(self.freshness_counter)?;
        encode_ptr(Some(0), index, w)?; // _data_len and _data_ptr

        Ok(())
    }

    fn encode_value(&self, w: &mut Vec<u8>) -> RdpResult<()> {
        self.context.encode_value(w)?;
        w.extend_from_slice(&self.card_uuid);

        w.write_u32::<LittleEndian>(self.data.len() as u32)?;
        w.extend_from_slice(&self.data);
        Ok(())
    }
}

#[derive(Debug)]
#[allow(dead_code, non_camel_case_types)]
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

impl Encode for GetReaderIcon_Call {
    fn encode(&self) -> RdpResult<Message> {
        let mut w = vec![];

        w.extend(RPCEStreamHeader::new().encode()?);
        RPCETypeHeader::new(0).encode(&mut w)?;

        let mut index = 0;
        self.context.encode_ptr(&mut index, &mut w)?;

        encode_ptr(None, &mut index, &mut w)?; // _reader_ptr

        self.context.encode_value(&mut w)?;
        w.extend(encode_str_unicode(&self.reader_name)?);

        Ok(w)
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
}

impl Encode for GetReaderIcon_Return {
    fn encode(&self) -> RdpResult<Message> {
        let mut w = vec![];
        w.write_u32::<LittleEndian>(self.return_code.to_u32().unwrap())?;

        // Encode empty data field, reader icon not implemented.
        // TODO: send Teleport/Pam logo.
        let mut index = 0;
        encode_ptr(Some(0), &mut index, &mut w)?;
        w.write_u32::<LittleEndian>(0)?;
        pad_and_add_headers(w)
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

    fn get(&mut self, id: u32) -> RdpResult<&mut ContextInternal> {
        self.contexts
            .get_mut(&id)
            .ok_or_else(|| invalid_data_error(&format!("unknown context id: {}", id)))
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
    // If we receive a SCARD_IOCTL_GETSTATUSCHANGEW with an infinite timeout, we need to
    // return a GetStatusChange_Return (embedded in a DeviceControlResponse) with
    // its return code set to SCARD_E_CANCELLED in the case that we receive a
    // SCARD_IOCTL_CANCEL.
    //
    // This value will be set during the handling of the SCARD_IOCTL_GETSTATUSCHANGEW, so that
    // it can be fetched and returned in response to a SCARD_IOCTL_CANCEL.
    scard_cancel_response: Option<DeviceControlResponse>,
}

impl ContextInternal {
    fn new() -> Self {
        Self {
            next_id: 1,
            handles: HashMap::new(),
            cache: HashMap::new(),
            scard_cancel_response: None,
        }
    }

    fn set_scard_cancel_response(&mut self, response: DeviceControlResponse) -> RdpResult<()> {
        if self.scard_cancel_response.is_some() {
            return Err(invalid_data_error("SCARD_IOCTL_CANCEL already received"));
        }
        self.scard_cancel_response = Some(response);
        Ok(())
    }

    fn connect(
        &mut self,
        ctx: Context,
        uuid: Uuid,
        cert_der: &[u8],
        key_der: &[u8],
        pin: String,
    ) -> RdpResult<Handle> {
        let card = piv::Card::new(uuid, cert_der, key_der, pin)?;
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

#[cfg(test)]
mod tests {
    use crate::{
        rdpdr::{
            consts::{MajorFunction, MinorFunction, SCARD_DEVICE_ID},
            DeviceIoRequest,
        },
        Encode,
    };

    use super::*;
    fn client() -> Client {
        Client::new(
            vec![
                48, 130, 4, 145, 48, 130, 3, 121, 160, 3, 2, 1, 2, 2, 16, 101, 91, 145, 220, 167,
                255, 174, 125, 129, 42, 229, 37, 240, 54, 206, 209, 48, 13, 6, 9, 42, 134, 72, 134,
                247, 13, 1, 1, 11, 5, 0, 48, 122, 49, 34, 48, 32, 6, 3, 85, 4, 10, 19, 25, 73, 115,
                97, 105, 97, 104, 115, 45, 77, 97, 99, 66, 111, 111, 107, 45, 80, 114, 111, 46,
                108, 111, 99, 97, 108, 49, 34, 48, 32, 6, 3, 85, 4, 3, 19, 25, 73, 115, 97, 105,
                97, 104, 115, 45, 77, 97, 99, 66, 111, 111, 107, 45, 80, 114, 111, 46, 108, 111,
                99, 97, 108, 49, 48, 48, 46, 6, 3, 85, 4, 5, 19, 39, 49, 56, 57, 50, 51, 56, 51,
                48, 50, 52, 50, 50, 52, 48, 52, 56, 56, 56, 48, 49, 48, 51, 50, 56, 57, 49, 53, 53,
                55, 56, 55, 54, 55, 50, 52, 57, 50, 53, 51, 48, 30, 23, 13, 50, 50, 48, 57, 49, 54,
                50, 50, 52, 48, 50, 50, 90, 23, 13, 50, 50, 48, 57, 49, 54, 50, 50, 52, 54, 50, 50,
                90, 48, 24, 49, 22, 48, 20, 6, 3, 85, 4, 3, 19, 13, 65, 100, 109, 105, 110, 105,
                115, 116, 114, 97, 116, 111, 114, 48, 130, 1, 34, 48, 13, 6, 9, 42, 134, 72, 134,
                247, 13, 1, 1, 1, 5, 0, 3, 130, 1, 15, 0, 48, 130, 1, 10, 2, 130, 1, 1, 0, 199,
                156, 191, 93, 193, 211, 66, 72, 35, 172, 242, 26, 214, 215, 157, 116, 92, 1, 15,
                91, 90, 220, 8, 12, 222, 194, 144, 51, 150, 158, 80, 93, 180, 61, 44, 203, 4, 79,
                26, 241, 6, 39, 87, 146, 182, 216, 119, 78, 236, 182, 90, 87, 89, 91, 148, 192,
                248, 34, 71, 215, 209, 212, 223, 121, 117, 220, 88, 82, 208, 28, 4, 228, 98, 1,
                254, 210, 179, 41, 163, 85, 200, 242, 107, 250, 148, 170, 254, 65, 245, 52, 167,
                153, 176, 216, 157, 111, 133, 40, 74, 66, 214, 165, 219, 238, 119, 160, 44, 172,
                24, 244, 161, 186, 50, 80, 192, 98, 109, 242, 125, 246, 155, 191, 127, 126, 94,
                149, 122, 75, 143, 209, 24, 28, 170, 191, 207, 247, 245, 223, 7, 165, 126, 168, 33,
                24, 154, 79, 53, 63, 70, 153, 113, 212, 114, 95, 198, 238, 12, 101, 239, 217, 79,
                184, 129, 146, 25, 34, 172, 221, 29, 188, 120, 143, 128, 6, 55, 127, 156, 198, 193,
                216, 9, 177, 212, 117, 121, 70, 245, 64, 2, 118, 242, 9, 50, 211, 97, 107, 128, 24,
                95, 25, 143, 128, 144, 17, 183, 83, 111, 168, 30, 188, 34, 121, 150, 43, 58, 134,
                200, 95, 34, 43, 219, 64, 97, 114, 113, 47, 198, 117, 198, 51, 159, 145, 106, 183,
                240, 112, 61, 97, 13, 88, 142, 153, 168, 207, 148, 5, 109, 182, 37, 214, 11, 75,
                142, 99, 35, 123, 2, 3, 1, 0, 1, 163, 130, 1, 115, 48, 130, 1, 111, 48, 14, 6, 3,
                85, 29, 15, 1, 1, 255, 4, 4, 3, 2, 7, 128, 48, 12, 6, 3, 85, 29, 19, 1, 1, 255, 4,
                2, 48, 0, 48, 31, 6, 3, 85, 29, 35, 4, 24, 48, 22, 128, 20, 171, 253, 63, 101, 19,
                143, 160, 188, 223, 135, 95, 0, 250, 242, 75, 243, 201, 83, 122, 126, 48, 129, 213,
                6, 3, 85, 29, 31, 4, 129, 205, 48, 129, 202, 48, 129, 199, 160, 129, 196, 160, 129,
                193, 134, 129, 190, 108, 100, 97, 112, 58, 47, 47, 47, 67, 78, 61, 73, 115, 97,
                105, 97, 104, 115, 45, 77, 97, 99, 66, 111, 111, 107, 45, 80, 114, 111, 46, 108,
                111, 99, 97, 108, 44, 67, 78, 61, 84, 101, 108, 101, 112, 111, 114, 116, 44, 67,
                78, 61, 67, 68, 80, 44, 67, 78, 61, 80, 117, 98, 108, 105, 99, 32, 75, 101, 121,
                32, 83, 101, 114, 118, 105, 99, 101, 115, 44, 67, 78, 61, 83, 101, 114, 118, 105,
                99, 101, 115, 44, 67, 78, 61, 67, 111, 110, 102, 105, 103, 117, 114, 97, 116, 105,
                111, 110, 44, 68, 67, 61, 116, 101, 108, 101, 112, 111, 114, 116, 44, 68, 67, 61,
                100, 101, 118, 63, 99, 101, 114, 116, 105, 102, 105, 99, 97, 116, 101, 82, 101,
                118, 111, 99, 97, 116, 105, 111, 110, 76, 105, 115, 116, 63, 98, 97, 115, 101, 63,
                111, 98, 106, 101, 99, 116, 67, 108, 97, 115, 115, 61, 99, 82, 76, 68, 105, 115,
                116, 114, 105, 98, 117, 116, 105, 111, 110, 80, 111, 105, 110, 116, 48, 31, 6, 3,
                85, 29, 37, 4, 24, 48, 22, 6, 8, 43, 6, 1, 5, 5, 7, 3, 2, 6, 10, 43, 6, 1, 4, 1,
                130, 55, 20, 2, 2, 48, 53, 6, 3, 85, 29, 17, 4, 46, 48, 44, 160, 42, 6, 10, 43, 6,
                1, 4, 1, 130, 55, 20, 2, 3, 160, 28, 12, 26, 65, 100, 109, 105, 110, 105, 115, 116,
                114, 97, 116, 111, 114, 64, 116, 101, 108, 101, 112, 111, 114, 116, 46, 100, 101,
                118, 48, 13, 6, 9, 42, 134, 72, 134, 247, 13, 1, 1, 11, 5, 0, 3, 130, 1, 1, 0, 234,
                9, 25, 253, 27, 189, 163, 187, 130, 134, 206, 82, 174, 9, 2, 161, 27, 9, 168, 70,
                149, 101, 82, 114, 130, 214, 221, 36, 154, 248, 94, 46, 133, 193, 52, 223, 80, 99,
                111, 208, 95, 93, 86, 70, 215, 77, 176, 77, 176, 139, 109, 98, 118, 72, 147, 247,
                39, 170, 223, 195, 96, 149, 213, 252, 134, 78, 53, 105, 136, 135, 150, 118, 100,
                180, 51, 166, 202, 180, 104, 33, 244, 215, 60, 198, 255, 142, 20, 228, 86, 30, 229,
                181, 70, 19, 201, 97, 46, 139, 161, 90, 253, 178, 149, 173, 238, 44, 8, 119, 116,
                18, 106, 146, 82, 229, 234, 53, 24, 158, 13, 192, 196, 15, 136, 167, 154, 88, 109,
                103, 79, 49, 242, 231, 167, 248, 85, 80, 215, 236, 135, 135, 129, 4, 192, 88, 150,
                94, 60, 134, 224, 219, 176, 228, 200, 82, 101, 209, 195, 36, 181, 64, 35, 233, 34,
                93, 22, 221, 221, 202, 60, 69, 37, 129, 69, 17, 51, 125, 10, 175, 40, 73, 120, 99,
                246, 65, 133, 199, 61, 255, 72, 117, 121, 88, 227, 254, 219, 116, 240, 248, 220,
                146, 222, 241, 229, 53, 179, 146, 57, 149, 151, 113, 63, 122, 27, 14, 159, 36, 153,
                90, 7, 188, 13, 152, 106, 192, 191, 125, 153, 126, 84, 190, 48, 27, 29, 108, 69,
                195, 209, 202, 243, 113, 87, 244, 115, 95, 157, 188, 157, 255, 169, 30, 85, 52,
                175, 44, 118, 255,
            ],
            vec![
                48, 130, 4, 165, 2, 1, 0, 2, 130, 1, 1, 0, 199, 156, 191, 93, 193, 211, 66, 72, 35,
                172, 242, 26, 214, 215, 157, 116, 92, 1, 15, 91, 90, 220, 8, 12, 222, 194, 144, 51,
                150, 158, 80, 93, 180, 61, 44, 203, 4, 79, 26, 241, 6, 39, 87, 146, 182, 216, 119,
                78, 236, 182, 90, 87, 89, 91, 148, 192, 248, 34, 71, 215, 209, 212, 223, 121, 117,
                220, 88, 82, 208, 28, 4, 228, 98, 1, 254, 210, 179, 41, 163, 85, 200, 242, 107,
                250, 148, 170, 254, 65, 245, 52, 167, 153, 176, 216, 157, 111, 133, 40, 74, 66,
                214, 165, 219, 238, 119, 160, 44, 172, 24, 244, 161, 186, 50, 80, 192, 98, 109,
                242, 125, 246, 155, 191, 127, 126, 94, 149, 122, 75, 143, 209, 24, 28, 170, 191,
                207, 247, 245, 223, 7, 165, 126, 168, 33, 24, 154, 79, 53, 63, 70, 153, 113, 212,
                114, 95, 198, 238, 12, 101, 239, 217, 79, 184, 129, 146, 25, 34, 172, 221, 29, 188,
                120, 143, 128, 6, 55, 127, 156, 198, 193, 216, 9, 177, 212, 117, 121, 70, 245, 64,
                2, 118, 242, 9, 50, 211, 97, 107, 128, 24, 95, 25, 143, 128, 144, 17, 183, 83, 111,
                168, 30, 188, 34, 121, 150, 43, 58, 134, 200, 95, 34, 43, 219, 64, 97, 114, 113,
                47, 198, 117, 198, 51, 159, 145, 106, 183, 240, 112, 61, 97, 13, 88, 142, 153, 168,
                207, 148, 5, 109, 182, 37, 214, 11, 75, 142, 99, 35, 123, 2, 3, 1, 0, 1, 2, 130, 1,
                0, 110, 65, 254, 210, 99, 5, 182, 78, 242, 165, 204, 245, 86, 70, 179, 10, 90, 231,
                154, 251, 243, 44, 38, 166, 53, 69, 115, 49, 139, 184, 214, 219, 107, 123, 127, 10,
                132, 206, 205, 42, 229, 35, 70, 20, 28, 59, 101, 107, 139, 5, 14, 209, 192, 225,
                253, 64, 185, 206, 245, 176, 24, 143, 101, 1, 74, 64, 243, 232, 138, 91, 111, 184,
                87, 10, 147, 30, 255, 39, 184, 184, 225, 206, 70, 38, 155, 135, 247, 249, 166, 223,
                246, 211, 198, 3, 96, 179, 0, 242, 72, 82, 179, 13, 218, 117, 214, 77, 251, 94,
                244, 73, 236, 43, 85, 47, 149, 148, 200, 246, 112, 237, 143, 10, 47, 250, 53, 116,
                139, 159, 198, 103, 154, 135, 111, 92, 88, 115, 126, 154, 95, 237, 229, 96, 23, 57,
                137, 244, 122, 61, 178, 14, 243, 187, 157, 7, 103, 183, 26, 252, 46, 33, 214, 70,
                187, 103, 70, 175, 8, 34, 119, 177, 105, 58, 131, 172, 220, 147, 29, 222, 182, 15,
                9, 99, 4, 59, 114, 31, 133, 68, 214, 132, 93, 42, 84, 102, 224, 196, 105, 204, 133,
                142, 228, 170, 112, 177, 23, 144, 68, 127, 16, 33, 156, 6, 131, 53, 143, 48, 142,
                161, 218, 114, 47, 106, 111, 203, 225, 32, 74, 142, 151, 150, 42, 70, 254, 190,
                132, 198, 116, 153, 195, 244, 132, 27, 211, 26, 12, 97, 150, 185, 120, 162, 209,
                165, 129, 96, 5, 193, 2, 129, 129, 0, 248, 205, 132, 65, 11, 44, 236, 203, 74, 170,
                87, 190, 88, 35, 35, 66, 144, 149, 29, 77, 230, 112, 36, 89, 97, 78, 177, 44, 104,
                166, 98, 128, 151, 90, 192, 94, 20, 146, 165, 104, 191, 118, 213, 211, 62, 92, 43,
                230, 211, 68, 149, 3, 196, 58, 77, 232, 81, 255, 185, 21, 42, 6, 52, 180, 196, 152,
                138, 73, 200, 252, 187, 175, 87, 233, 76, 70, 242, 190, 74, 118, 221, 173, 106,
                140, 64, 38, 183, 242, 197, 181, 105, 251, 39, 36, 145, 70, 1, 77, 182, 63, 237, 5,
                8, 191, 110, 51, 98, 229, 246, 221, 240, 151, 116, 57, 162, 23, 41, 194, 194, 224,
                154, 134, 131, 191, 247, 150, 114, 143, 2, 129, 129, 0, 205, 98, 244, 169, 239,
                255, 0, 181, 34, 36, 2, 12, 28, 220, 6, 253, 89, 87, 101, 212, 169, 114, 250, 119,
                185, 47, 96, 27, 31, 29, 59, 83, 210, 214, 207, 251, 182, 27, 10, 142, 57, 91, 162,
                253, 144, 194, 138, 82, 244, 241, 225, 211, 196, 31, 116, 175, 71, 143, 182, 28,
                129, 93, 21, 7, 151, 89, 220, 253, 255, 161, 69, 165, 140, 84, 134, 134, 108, 138,
                253, 94, 94, 48, 75, 92, 14, 209, 104, 0, 93, 187, 35, 7, 244, 233, 18, 67, 189,
                63, 98, 165, 188, 220, 109, 254, 85, 105, 254, 152, 249, 160, 48, 14, 242, 255, 45,
                23, 205, 177, 92, 160, 94, 92, 47, 160, 70, 36, 70, 85, 2, 129, 129, 0, 241, 30,
                115, 18, 122, 27, 34, 172, 237, 130, 98, 32, 148, 216, 16, 190, 220, 209, 182, 33,
                157, 182, 134, 115, 156, 139, 31, 199, 50, 240, 52, 187, 252, 114, 181, 197, 55,
                88, 219, 54, 197, 127, 12, 64, 121, 201, 231, 189, 254, 119, 19, 151, 31, 223, 133,
                75, 37, 212, 151, 112, 252, 86, 33, 84, 34, 198, 214, 22, 37, 211, 80, 172, 224,
                156, 183, 16, 119, 5, 149, 178, 214, 168, 206, 126, 119, 89, 78, 161, 215, 155, 53,
                199, 113, 170, 205, 163, 51, 118, 53, 174, 132, 44, 129, 202, 203, 168, 191, 42,
                176, 113, 108, 77, 203, 20, 99, 146, 225, 36, 223, 169, 189, 247, 168, 205, 44,
                203, 191, 223, 2, 129, 129, 0, 146, 46, 77, 63, 42, 142, 207, 189, 28, 8, 142, 224,
                122, 37, 236, 95, 163, 151, 253, 229, 71, 153, 139, 69, 109, 43, 151, 246, 149,
                197, 163, 117, 60, 202, 33, 139, 225, 8, 12, 18, 64, 38, 197, 178, 61, 183, 8, 230,
                148, 106, 24, 54, 54, 15, 193, 104, 3, 193, 248, 118, 255, 103, 245, 208, 202, 91,
                110, 91, 229, 246, 173, 240, 111, 25, 182, 9, 180, 245, 147, 241, 247, 141, 222, 5,
                46, 146, 194, 184, 7, 254, 106, 167, 126, 27, 233, 33, 7, 112, 54, 209, 9, 195,
                198, 17, 208, 79, 57, 163, 61, 128, 82, 212, 65, 5, 119, 221, 202, 75, 227, 70, 77,
                2, 197, 239, 8, 29, 71, 101, 2, 129, 129, 0, 141, 75, 99, 52, 181, 42, 232, 22, 82,
                120, 119, 201, 255, 122, 220, 146, 225, 193, 162, 102, 82, 30, 94, 140, 197, 50,
                59, 122, 2, 92, 75, 64, 178, 230, 209, 216, 171, 40, 172, 143, 128, 77, 160, 241,
                130, 40, 205, 123, 241, 181, 38, 13, 24, 215, 218, 53, 217, 82, 125, 201, 153, 141,
                149, 236, 191, 149, 137, 125, 208, 56, 69, 217, 228, 65, 85, 148, 234, 30, 115, 31,
                81, 234, 98, 250, 222, 165, 236, 164, 56, 19, 34, 29, 150, 172, 118, 228, 179, 91,
                26, 208, 186, 161, 49, 218, 225, 211, 204, 48, 207, 193, 226, 158, 174, 105, 177,
                227, 28, 132, 109, 252, 218, 102, 20, 175, 152, 91, 201, 168,
            ],
            "68971585".to_string(),
        )
    }

    fn to_payload(e: &dyn Encode) -> Payload {
        Payload::new(e.encode().unwrap())
    }

    fn test_ioctl(
        established_ctxs: u32,
        connect_scard_to_ctx: Option<u32>,
        ctl_code: IoctlCode,
        payload: &dyn Encode,
        expected: &dyn Encode,
    ) {
        let mut c = client();

        for _ in 0..established_ctxs {
            c.contexts.establish();
        }
        if let Some(connect_scard_to_ctx) = connect_scard_to_ctx {
            connect_scard(&mut c, connect_scard_to_ctx);
        }

        let res = c
            .ioctl(
                &DeviceControlRequest::new(
                    DeviceIoRequest::new(
                        SCARD_DEVICE_ID,
                        0,
                        0,
                        MajorFunction::IRP_MJ_DEVICE_CONTROL,
                        MinorFunction::IRP_MN_NONE,
                    ),
                    0,
                    0,
                    ctl_code,
                ),
                &mut to_payload(payload),
            )
            .unwrap();
        assert_eq!(
            expected.encode().unwrap(),
            // Only SCARD_IOCTL_CANCEL every returns more than a single
            // result, and it's currently not tested, so res[0] works here
            // for now.
            res[0].output_buffer.encode().unwrap()
        );
    }

    /// Connects a piv::Card to the client's internal context cache
    /// (on the context corresponding to context_value). This is a manual
    /// way of doing what test_scard_ioctl_connectw does to the Client's
    /// internal state.
    fn connect_scard(c: &mut Client, context_value: u32) {
        let ctx = c.contexts.get(context_value).unwrap();
        ctx.connect(
            Context {
                length: 4,
                value: context_value,
            },
            c.uuid,
            &c.cert_der,
            &c.key_der,
            c.pin.clone(),
        )
        .unwrap();
    }

    #[test]
    fn test_accessstartedevent() {
        test_ioctl(
            0,
            None,
            IoctlCode::SCARD_IOCTL_ACCESSSTARTEDEVENT,
            &ScardAccessStartedEvent_Call {
                _unused: 3234823568,
            },
            &Long_Return {
                return_code: ReturnCode::SCARD_S_SUCCESS,
            },
        )
    }

    #[test]
    fn test_establishcontext() {
        test_ioctl(
            0,
            None,
            IoctlCode::SCARD_IOCTL_ESTABLISHCONTEXT,
            &EstablishContext_Call {
                scope: Scope::SCARD_SCOPE_SYSTEM,
            },
            &EstablishContext_Return {
                return_code: ReturnCode::SCARD_S_SUCCESS,
                context: Context {
                    length: 4,
                    value: 1,
                },
            },
        )
    }

    #[test]
    fn test_listreadersw() {
        test_ioctl(
            0,
            None,
            IoctlCode::SCARD_IOCTL_LISTREADERSW,
            &ListReaders_Call {
                context: Context {
                    length: 4,
                    value: 2,
                },
                groups_ptr_length: 36,
                groups_length: 36,
                groups_ptr: 131076,
                groups: vec!["SCard$AllReaders".to_string()],
                readers_is_null: false,
                readers_size: 4294967295,
            },
            &ListReaders_Return {
                return_code: ReturnCode::SCARD_S_SUCCESS,
                readers: vec!["Teleport".to_string()],
            },
        )
    }

    #[test]
    fn test_getdevicetypeid() {
        let context_value = 2;
        test_ioctl(
            context_value,
            None,
            IoctlCode::SCARD_IOCTL_GETDEVICETYPEID,
            &GetDeviceTypeId_Call {
                context: Context {
                    length: 4,
                    value: context_value,
                },
                reader_ptr: 131076,
                reader_name: "Teleport".to_string(),
            },
            &GetDeviceTypeId_Return {
                return_code: ReturnCode::SCARD_S_SUCCESS,
                device_type_id: 240,
            },
        )
    }

    #[test]
    fn test_releasecontext() {
        let context_value = 2;
        test_ioctl(
            context_value,
            None,
            IoctlCode::SCARD_IOCTL_RELEASECONTEXT,
            &Context_Call {
                context: Context {
                    length: 4,
                    value: context_value,
                },
            },
            &Long_Return {
                return_code: ReturnCode::SCARD_S_SUCCESS,
            },
        )
    }

    #[test]
    fn test_getstatuschangew() {
        let context_value = 1;
        test_ioctl(
            context_value,
            None,
            IoctlCode::SCARD_IOCTL_GETSTATUSCHANGEW,
            &GetStatusChange_Call {
                context: Context {
                    length: 4,
                    value: context_value,
                },
                timeout: 4294967295,
                states_ptr_length: 2,
                states_ptr: 131076,
                states_length: 2,
                states: vec![
                    ReaderState {
                        reader: "\\\\?PnP?\\Notification".to_string(),
                        common: ReaderState_Common_Call {
                            current_state: CardStateFlags::SCARD_STATE_UNAWARE,
                            event_state: CardStateFlags::SCARD_STATE_UNAWARE,
                            atr_length: 0,
                            atr: [0; 36],
                        },
                    },
                    ReaderState {
                        reader: "Teleport".to_string(),
                        common: ReaderState_Common_Call {
                            current_state: CardStateFlags::SCARD_STATE_EMPTY,
                            event_state: CardStateFlags::SCARD_STATE_UNAWARE,
                            atr_length: 0,
                            atr: [0; 36],
                        },
                    },
                ],
            },
            &GetStatusChange_Return {
                return_code: ReturnCode::SCARD_S_SUCCESS,
                reader_states: vec![
                    ReaderState_Common_Call {
                        current_state: CardStateFlags::SCARD_STATE_UNAWARE,
                        event_state: CardStateFlags::SCARD_STATE_UNAWARE,
                        atr_length: 0,
                        atr: [
                            0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
                            0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
                        ],
                    },
                    ReaderState_Common_Call {
                        current_state: CardStateFlags::SCARD_STATE_EMPTY,
                        event_state: CardStateFlags::SCARD_STATE_CHANGED
                            | CardStateFlags::SCARD_STATE_PRESENT,
                        atr_length: 11,
                        atr: [
                            59, 149, 19, 129, 1, 128, 115, 255, 1, 0, 11, 0, 0, 0, 0, 0, 0, 0, 0,
                            0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
                        ],
                    },
                ],
            },
        )
    }

    #[test]
    fn test_connectw() {
        let context_value = 5;
        test_ioctl(
            context_value,
            None,
            IoctlCode::SCARD_IOCTL_CONNECTW,
            &Connect_Call {
                reader: "Teleport".to_string(),
                common: Connect_Common {
                    context: Context {
                        length: 4,
                        value: context_value,
                    },
                    share_mode: 2,
                    preferred_protocols: CardProtocol::SCARD_PROTOCOL_T0
                        | CardProtocol::SCARD_PROTOCOL_T1
                        | CardProtocol::SCARD_PROTOCOL_TX,
                },
            },
            &Connect_Return {
                return_code: ReturnCode::SCARD_S_SUCCESS,
                handle: Handle {
                    context: Context {
                        length: 4,
                        value: 5,
                    },
                    length: 4,
                    value: 1,
                },
                active_protocol: CardProtocol::SCARD_PROTOCOL_T1,
            },
        )
    }

    #[test]
    fn test_begintransaction() {
        let context_value = 5;
        test_ioctl(
            context_value,
            None,
            IoctlCode::SCARD_IOCTL_BEGINTRANSACTION,
            &HCardAndDisposition_Call {
                handle: Handle {
                    context: Context {
                        length: 4,
                        value: 5,
                    },
                    length: 4,
                    value: 1,
                },
                disposition: 0,
            },
            &Long_Return {
                return_code: ReturnCode::SCARD_S_SUCCESS,
            },
        )
    }

    #[test]
    fn test_statusw() {
        let context_value = 5;
        test_ioctl(
            context_value,
            None,
            IoctlCode::SCARD_IOCTL_STATUSW,
            &Status_Call {
                handle: Handle {
                    context: Context {
                        length: 4,
                        value: 5,
                    },
                    length: 4,
                    value: 1,
                },
                reader_names_is_null: false,
                reader_length: 4294967295,
                atr_length: 32,
            },
            &Status_Return {
                return_code: ReturnCode::SCARD_S_SUCCESS,
                reader_names: vec!["Teleport".to_string()],
                state: State::SCARD_SPECIFICMODE,
                protocol: CardProtocol::SCARD_PROTOCOL_T1,
                atr: [
                    59, 149, 19, 129, 1, 128, 115, 255, 1, 0, 11, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
                    0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
                ],
                atr_length: 11,
                encoding: StringEncoding::Unicode,
            },
        )
    }

    #[test]
    fn test_transmit() {
        let context_value = 5;
        test_ioctl(
            context_value,
            Some(context_value),
            IoctlCode::SCARD_IOCTL_TRANSMIT,
            &Transmit_Call {
                handle: Handle {
                    context: Context {
                        length: 4,
                        value: 5,
                    },
                    length: 4,
                    value: 1,
                },
                send_pci: SCardIO_Request {
                    protocol: CardProtocol::SCARD_PROTOCOL_T1,
                    extra_bytes_length: 0,
                    extra_bytes: vec![],
                },
                send_length: 14,
                send_buffer: vec![0, 164, 4, 0, 9, 160, 0, 0, 3, 8, 0, 0, 16, 0],
                recv_pci: None,
                recv_buffer_is_null: false,
                recv_length: 258,
            },
            &Transmit_Return {
                return_code: ReturnCode::SCARD_S_SUCCESS,
                recv_buffer: vec![
                    97, 17, 79, 6, 0, 0, 16, 0, 1, 0, 121, 7, 79, 5, 160, 0, 0, 3, 8, 144, 0,
                ],
            },
        )
    }

    #[test]
    fn test_readcachew() {
        let context_value = 5;
        test_ioctl(
            context_value,
            None,
            IoctlCode::SCARD_IOCTL_READCACHEW,
            &ReadCache_Call {
                lookup_name: "Cached_CardmodFile\\Cached_Pin_Freshness".to_string(),
                common: ReadCache_Common {
                    context: Context {
                        length: 4,
                        value: 5,
                    },
                    card_uuid: vec![
                        138, 113, 14, 35, 145, 213, 78, 249, 174, 208, 142, 171, 174, 121, 3, 76,
                    ],
                    freshness_counter: 0,
                    data_is_null: false,
                    data_len: 4294967295,
                },
            },
            &ReadCache_Return {
                return_code: ReturnCode::SCARD_W_CACHE_ITEM_NOT_FOUND,
                data: vec![],
            },
        )
    }

    #[test]
    fn test_writecachew() {
        let context_value = 5;
        test_ioctl(
            context_value,
            None,
            IoctlCode::SCARD_IOCTL_WRITECACHEW,
            &WriteCache_Call {
                lookup_name: "Cached_CardProperty_Read Only Mode_0".to_string(),
                common: WriteCache_Common {
                    context: Context {
                        length: 4,
                        value: context_value,
                    },
                    card_uuid: vec![
                        138, 113, 14, 35, 145, 213, 78, 249, 174, 208, 142, 171, 174, 121, 3, 76,
                    ],
                    freshness_counter: 0,
                    data: vec![1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 4, 0, 0, 0, 1, 0, 0, 0],
                },
            },
            &Long_Return {
                return_code: ReturnCode::SCARD_S_SUCCESS,
            },
        )
    }

    #[test]
    fn test_endtransaction() {
        let context_value = 5;
        test_ioctl(
            context_value,
            None,
            IoctlCode::SCARD_IOCTL_ENDTRANSACTION,
            &HCardAndDisposition_Call {
                handle: Handle {
                    context: Context {
                        length: 4,
                        value: context_value,
                    },
                    length: 4,
                    value: 1,
                },
                disposition: 0,
            },
            &Long_Return {
                return_code: ReturnCode::SCARD_S_SUCCESS,
            },
        )
    }

    #[test]
    fn test_disconnect() {
        let context_value = 5;
        test_ioctl(
            context_value,
            None,
            IoctlCode::SCARD_IOCTL_DISCONNECT,
            &HCardAndDisposition_Call {
                handle: Handle {
                    context: Context {
                        length: 4,
                        value: context_value,
                    },
                    length: 4,
                    value: 1,
                },
                disposition: 0,
            },
            &Long_Return {
                return_code: ReturnCode::SCARD_S_SUCCESS,
            },
        )
    }

    #[test]
    fn test_getreadericon() {
        let context_value = 5;
        test_ioctl(
            context_value,
            None,
            IoctlCode::SCARD_IOCTL_GETREADERICON,
            &GetReaderIcon_Call {
                context: Context {
                    length: 4,
                    value: context_value,
                },
                reader_name: "Teleport".to_string(),
            },
            &GetReaderIcon_Return {
                return_code: ReturnCode::SCARD_E_UNSUPPORTED_FEATURE,
            },
        )
    }
}
