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

use crate::piv;
use ironrdp_pdu::{other_err, PduResult};
use ironrdp_rdpdr::pdu::efs::DeviceControlResponse;
use ironrdp_rdpdr::pdu::esc::{ReadCacheCall, ScardContext, ScardHandle, WriteCacheCall};
use std::collections::HashMap;
use uuid::Uuid;

#[derive(Debug)]
pub struct Contexts {
    contexts: HashMap<u32, ContextInternal>,
    next_id: u32,
}

impl Contexts {
    pub fn new() -> Self {
        Self {
            next_id: 1,
            contexts: HashMap::new(),
        }
    }

    pub fn establish(&mut self) -> ScardContext {
        let ctx_internal = ContextInternal::new();
        let id = self.next_id;
        self.next_id += 1;
        let ctx = ScardContext::new(id);
        self.contexts.insert(id, ctx_internal);
        ctx
    }

    pub fn connect(
        &mut self,
        ctx: ScardContext,
        id: u32,
        uuid: Uuid,
        cert_der: &[u8],
        key_der: &[u8],
        pin: String,
    ) -> PduResult<ScardHandle> {
        let ctx_internal = self.get_internal_mut(id)?;
        let handle = ctx_internal.connect(ctx, uuid, cert_der, key_der, pin)?;
        Ok(handle)
    }

    pub fn disconnect(&mut self, handle: ScardHandle) -> PduResult<()> {
        self.get_internal_mut(handle.context.value)?
            .disconnect(handle.value);
        Ok(())
    }

    pub fn set_scard_cancel_response(
        &mut self,
        id: u32,
        resp: DeviceControlResponse,
    ) -> PduResult<()> {
        self.get_internal_mut(id)?.set_scard_cancel_response(resp)
    }

    pub fn take_scard_cancel_response(
        &mut self,
        id: u32,
    ) -> PduResult<Option<DeviceControlResponse>> {
        Ok(self.get_internal_mut(id)?.take_scard_cancel_response())
    }

    pub fn get_card(
        &mut self,
        handle: &ScardHandle,
    ) -> PduResult<&mut piv::Card<TRANSMIT_DATA_LIMIT>> {
        self.get_internal_mut(handle.context.value)?
            .get(handle.value)
            .ok_or_else(|| other_err!("Contexts::get_card", "unknown ScardHandle"))
    }

    pub fn exists(&self, id: u32) -> bool {
        self.contexts.contains_key(&id)
    }

    pub fn read_cache(&mut self, call: ReadCacheCall) -> PduResult<Option<Vec<u8>>> {
        Ok(self
            .get_internal_mut(call.common.context.value)?
            .cache_read(&call.lookup_name))
    }

    pub fn write_cache(&mut self, call: WriteCacheCall) -> PduResult<()> {
        self.get_internal_mut(call.common.context.value)?
            .cache_write(call.lookup_name, call.common.data);
        Ok(())
    }

    fn get_internal_mut(&mut self, id: u32) -> PduResult<&mut ContextInternal> {
        self.contexts
            .get_mut(&id)
            .ok_or_else(|| other_err!("Contexts::get_internal_mut", "unknown context id"))
    }

    pub fn release(&mut self, id: u32) {
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

    fn set_scard_cancel_response(&mut self, resp: DeviceControlResponse) -> PduResult<()> {
        if self.scard_cancel_response.is_some() {
            return Err(other_err!(
                "ContextInternal::set_scard_cancel_response",
                "SCARD_IOCTL_CANCEL already received",
            ));
        }
        self.scard_cancel_response = Some(resp);
        Ok(())
    }

    fn take_scard_cancel_response(&mut self) -> Option<DeviceControlResponse> {
        self.scard_cancel_response.take()
    }

    fn connect(
        &mut self,
        ctx: ScardContext,
        uuid: Uuid,
        cert_der: &[u8],
        key_der: &[u8],
        pin: String,
    ) -> PduResult<ScardHandle> {
        let card = piv::Card::new(uuid, cert_der, key_der, pin)?;
        let id = self.next_id;
        self.next_id += 1;
        let handle = ScardHandle::new(ctx, id);
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

pub fn padded_atr<const SIZE: usize>() -> (u32, [u8; SIZE]) {
    let mut atr = [0; SIZE];
    let len = STATIC_ATR.len().min(SIZE);
    atr[..len].copy_from_slice(&STATIC_ATR[..len]);
    (len as u32, atr)
}

pub const SCARD_DEVICE_ID: u32 = 1;
pub const TELEPORT_READER_NAME: &str = "Teleport";
// TRANSMIT_DATA_LIMIT is the maximum size of transmit request/response short data, in bytes.
pub const TRANSMIT_DATA_LIMIT: usize = 1024;
pub const TIMEOUT_INFINITE: u32 = 0xffffffff;
pub const TIMEOUT_IMMEDIATE: u32 = 0;
