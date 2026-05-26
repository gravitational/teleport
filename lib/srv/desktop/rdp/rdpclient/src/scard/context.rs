// Teleport
// Copyright (C) 2023  Gravitational, Inc.
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

use crate::scard::piv::PivCard;
use crate::scard::TRANSMIT_DATA_LIMIT;
use ironrdp_pdu::{pdu_other_err, PduResult};
use ironrdp_rdpdr::pdu::efs::DeviceControlResponse;
use ironrdp_rdpdr::pdu::esc::{ReadCacheCall, ScardContext, ScardHandle, WriteCacheCall};
use log::debug;
use std::collections::HashMap;
use uuid::Uuid;

#[derive(Debug)]
pub(super) struct Contexts {
    contexts: HashMap<u32, ContextInternal>,
    next_id: u32,
}

impl Contexts {
    pub(super) fn new() -> Self {
        Self {
            next_id: 1,
            contexts: HashMap::new(),
        }
    }

    pub(super) fn establish(&mut self) -> ScardContext {
        let ctx_internal = ContextInternal::new();
        let id = self.next_id;
        self.next_id += 1;
        let ctx = ScardContext::new(id);
        self.contexts.insert(id, ctx_internal);
        ctx
    }

    pub(super) fn connect(
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

    pub(super) fn disconnect(&mut self, handle: ScardHandle) -> PduResult<()> {
        self.get_internal_mut(handle.context.value)?
            .disconnect(handle.value);
        Ok(())
    }

    pub(super) fn set_scard_cancel_response(
        &mut self,
        id: u32,
        resp: DeviceControlResponse,
    ) -> PduResult<()> {
        debug!("setting SCARD_IOCTL_CANCEL response for context [{}]", id);
        self.get_internal_mut(id)?.set_scard_cancel_response(resp)
    }

    pub(super) fn take_scard_cancel_response(
        &mut self,
        id: u32,
    ) -> PduResult<Option<DeviceControlResponse>> {
        Ok(self.get_internal_mut(id)?.take_scard_cancel_response())
    }

    pub(super) fn get_card(
        &mut self,
        handle: &ScardHandle,
    ) -> PduResult<&mut PivCard<TRANSMIT_DATA_LIMIT>> {
        self.get_internal_mut(handle.context.value)?
            .get(handle.value)
            .ok_or_else(|| pdu_other_err!("unknown ScardHandle"))
    }

    pub(super) fn exists(&self, id: u32) -> bool {
        self.contexts.contains_key(&id)
    }

    pub(super) fn read_cache(&mut self, call: ReadCacheCall) -> PduResult<Option<Vec<u8>>> {
        Ok(self
            .get_internal_mut(call.common.context.value)?
            .cache_read(&call.lookup_name))
    }

    pub(super) fn write_cache(&mut self, call: WriteCacheCall) -> PduResult<()> {
        self.get_internal_mut(call.common.context.value)?
            .cache_write(call.lookup_name, call.common.data);
        Ok(())
    }

    fn get_internal_mut(&mut self, id: u32) -> PduResult<&mut ContextInternal> {
        self.contexts
            .get_mut(&id)
            .ok_or_else(|| pdu_other_err!("unknown context id"))
    }

    pub(super) fn release(&mut self, id: u32) {
        self.contexts.remove(&id);
    }
}

#[derive(Debug)]
struct ContextInternal {
    handles: HashMap<u32, PivCard<TRANSMIT_DATA_LIMIT>>,
    next_id: u32,
    cache: HashMap<String, Vec<u8>>,
    // If we receive a SCARD_IOCTL_GETSTATUSCHANGE with an infinite timeout, we need to
    // return a GetStatusChange_Return (embedded in a DeviceControlResponse) with
    // its return code set to SCARD_E_CANCELLED in the case that we receive a
    // SCARD_IOCTL_CANCEL.
    //
    // This value will be set during the handling of the SCARD_IOCTL_GETSTATUSCHANGE, so that
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
            return Err(pdu_other_err!("SCARD_IOCTL_CANCEL already received",));
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
        let card = PivCard::new(uuid, cert_der, key_der, pin)?;
        let id = self.next_id;
        self.next_id += 1;
        let handle = ScardHandle::new(ctx, id);
        self.handles.insert(id, card);
        Ok(handle)
    }

    fn get(&mut self, id: u32) -> Option<&mut PivCard<TRANSMIT_DATA_LIMIT>> {
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
