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

use crate::client::ClientHandle;
use crate::{piv, util};
use ironrdp_pdu::utils::CharacterSet;
use ironrdp_pdu::{custom_err, other_err, PduResult};
use ironrdp_rdpdr::pdu::efs::{DeviceControlRequest, DeviceControlResponse, NtStatus};
use ironrdp_rdpdr::pdu::esc::{
    rpce, CardProtocol, CardState, CardStateFlags, ConnectCall, ConnectReturn, ContextCall,
    EstablishContextReturn, GetDeviceTypeIdCall, GetDeviceTypeIdReturn, GetReaderIconReturn,
    GetStatusChangeCall, GetStatusChangeReturn, HCardAndDispositionCall, ListReadersReturn,
    LongReturn, ReadCacheCall, ReadCacheReturn, ReaderStateCommonCall, ReturnCode, ScardCall,
    ScardContext, ScardHandle, ScardIoCtlCode, StatusReturn, TransmitCall, TransmitReturn,
    WriteCacheCall,
};
use iso7816::Command as CardCommand;
use log::{debug, warn};
use std::collections::HashMap;
use uuid::Uuid;

/// `ScardBackend` implements the smartcard device redirection backend as described in [\[MS-RDPESC\]: Remote Desktop Protocol: Smart Card Virtual Channel Extension]
///
/// [\[MS-RDPESC\]: Remote Desktop Protocol: Smart Card Virtual Channel Extension]: https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpesc/0428ca28-b4dc-46a3-97c3-01887fa44a90
pub struct ScardBackend {
    client_handle: ClientHandle,
    /// contexts holds all the active contexts for the server, established using
    /// SCARD_IOCTL_ESTABLISHCONTEXT. Some IOCTLs are context-specific and pass it as argument.
    ///
    /// contexts also holds a cache and connected smartcard handles for each context.
    contexts: Contexts,
    uuid: Uuid,
    cert_der: Vec<u8>,
    key_der: Vec<u8>,
    pin: String,
}

impl std::fmt::Debug for ScardBackend {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("ScardBackend")
            .field("client_handle", &self.client_handle)
            .field("contexts", &self.contexts)
            .field("uuid", &self.uuid)
            .field("cert_der", &util::vec_u8_debug(&self.cert_der))
            // Important we don't leak key_der to the logs.
            .field("key_der", &util::vec_u8_debug(&self.key_der))
            // Out of an abundance of caution, don't leak the PIN to the logs.
            .field("pin", &util::str_debug(&self.pin))
            .finish()
    }
}

impl ScardBackend {
    pub fn new(
        client_handle: ClientHandle,
        cert_der: Vec<u8>,
        key_der: Vec<u8>,
        pin: String,
    ) -> Self {
        Self {
            client_handle,
            contexts: Contexts::new(),
            uuid: Uuid::new_v4(),
            cert_der,
            key_der,
            pin,
        }
    }

    pub fn handle(
        &mut self,
        req: DeviceControlRequest<ScardIoCtlCode>,
        call: ScardCall,
    ) -> PduResult<()> {
        match req.io_control_code {
            ScardIoCtlCode::AccessStartedEvent => match call {
                ScardCall::AccessStartedEventCall(_) => self.handle_access_started_event(req),
                _ => Self::unsupported_combo_error(req.io_control_code, call),
            },
            ScardIoCtlCode::EstablishContext => match call {
                ScardCall::EstablishContextCall(_) => self.handle_establish_context(req),
                _ => Self::unsupported_combo_error(req.io_control_code, call),
            },
            ScardIoCtlCode::ListReadersW | ScardIoCtlCode::ListReadersA => match call {
                ScardCall::ListReadersCall(_) => self.handle_list_readers(req),
                _ => Self::unsupported_combo_error(req.io_control_code, call),
            },
            ScardIoCtlCode::GetStatusChangeW | ScardIoCtlCode::GetStatusChangeA => match call {
                ScardCall::GetStatusChangeCall(call) => self.handle_get_status_change(req, call),
                _ => Self::unsupported_combo_error(req.io_control_code, call),
            },
            ScardIoCtlCode::ConnectW | ScardIoCtlCode::ConnectA => match call {
                ScardCall::ConnectCall(call) => self.handle_connect(req, call),
                _ => Self::unsupported_combo_error(req.io_control_code, call),
            },
            ScardIoCtlCode::BeginTransaction => match call {
                ScardCall::HCardAndDispositionCall(_) => self.handle_begin_transaction(req),
                _ => Self::unsupported_combo_error(req.io_control_code, call),
            },
            ScardIoCtlCode::Transmit => match call {
                ScardCall::TransmitCall(call) => self.handle_transmit(req, call),
                _ => Self::unsupported_combo_error(req.io_control_code, call),
            },
            ScardIoCtlCode::StatusW | ScardIoCtlCode::StatusA => match call {
                ScardCall::StatusCall(_) => self.handle_status(req),
                _ => Self::unsupported_combo_error(req.io_control_code, call),
            },
            ScardIoCtlCode::ReleaseContext => match call {
                ScardCall::ContextCall(call) => self.handle_release_context(req, call),
                _ => Self::unsupported_combo_error(req.io_control_code, call),
            },
            ScardIoCtlCode::EndTransaction => match call {
                ScardCall::HCardAndDispositionCall(_) => self.handle_end_transaction(req),
                _ => Self::unsupported_combo_error(req.io_control_code, call),
            },
            ScardIoCtlCode::Disconnect => match call {
                ScardCall::HCardAndDispositionCall(call) => self.handle_disconnect(req, call),
                _ => Self::unsupported_combo_error(req.io_control_code, call),
            },
            ScardIoCtlCode::Cancel => match call {
                ScardCall::ContextCall(call) => self.handle_cancel(req, call),
                _ => Self::unsupported_combo_error(req.io_control_code, call),
            },
            ScardIoCtlCode::IsValidContext => match call {
                ScardCall::ContextCall(_) => self.handle_is_valid_context(req),
                _ => Self::unsupported_combo_error(req.io_control_code, call),
            },
            ScardIoCtlCode::GetDeviceTypeId => match call {
                ScardCall::GetDeviceTypeIdCall(call) => self.handle_get_device_type_id(req, call),
                _ => Self::unsupported_combo_error(req.io_control_code, call),
            },
            ScardIoCtlCode::ReadCacheW | ScardIoCtlCode::ReadCacheA => match call {
                ScardCall::ReadCacheCall(call) => self.handle_read_cache(req, call),
                _ => Self::unsupported_combo_error(req.io_control_code, call),
            },
            ScardIoCtlCode::WriteCacheW | ScardIoCtlCode::WriteCacheA => match call {
                ScardCall::WriteCacheCall(call) => self.handle_write_cache(req, call),
                _ => Self::unsupported_combo_error(req.io_control_code, call),
            },
            ScardIoCtlCode::GetReaderIcon => match call {
                ScardCall::GetReaderIconCall(_) => self.handle_get_reader_icon(req),
                _ => Self::unsupported_combo_error(req.io_control_code, call),
            },
            _ => Err(custom_err!(SmartcardBackendError(format!(
                "received unhandled ScardIoCtlCode: {:?}",
                req.io_control_code
            )))),
        }?;

        Ok(())
    }

    fn handle_access_started_event(
        &mut self,
        req: DeviceControlRequest<ScardIoCtlCode>,
    ) -> PduResult<()> {
        self.send_device_control_response(req, LongReturn::new(ReturnCode::Success))
    }

    fn handle_establish_context(
        &mut self,
        req: DeviceControlRequest<ScardIoCtlCode>,
    ) -> PduResult<()> {
        let ctx = self.contexts.establish();

        self.send_device_control_response(
            req,
            EstablishContextReturn::new(ReturnCode::Success, ctx),
        )
    }

    fn handle_list_readers(&mut self, req: DeviceControlRequest<ScardIoCtlCode>) -> PduResult<()> {
        self.send_device_control_response(
            req,
            ListReadersReturn::new(ReturnCode::Success, vec![TELEPORT_READER_NAME.to_string()]),
        )
    }

    fn handle_get_status_change(
        &mut self,
        req: DeviceControlRequest<ScardIoCtlCode>,
        call: GetStatusChangeCall,
    ) -> PduResult<()> {
        let timeout = call.timeout;
        let context_id = call.context.value;

        if timeout != TIMEOUT_INFINITE && timeout != TIMEOUT_IMMEDIATE {
            // We've never seen one of these but we log a warning here in case we ever come
            // across one and need to debug a related issue.
            warn!(
                "logic for a non-infinite/non-immediate timeout [{}] is not implemented",
                timeout
            );
        }

        let reader_states = Self::create_get_status_change_reader_states(call);

        // We have no status change to report.
        if Self::has_no_change(&reader_states) {
            if timeout != TIMEOUT_INFINITE {
                // Since our status never changes, we just return immediately here
                // as if the call timed out.
                debug!("got no change for non-infinite timeout");
                return self.send_device_control_response(
                    req,
                    GetStatusChangeReturn::new(ReturnCode::Timeout, reader_states),
                );
            }

            // Received a GetStatusChangeCall with an infinite timeout, so we're adding
            // a corresponding DeviceControlResponse holding a GetStatusChangeReturn
            // with its return code set to SCARD_E_CANCELLED to this Context. This value will
            // be returned when we get an SCARD_IOCTL_CANCEL call for this Context.
            self.contexts.set_scard_cancel_response(
                context_id,
                DeviceControlResponse::new(
                    req,
                    NtStatus::SUCCESS,
                    Some(Box::new(GetStatusChangeReturn::new(
                        ReturnCode::Cancelled,
                        reader_states,
                    ))),
                ),
            )?;

            debug!("blocking GetStatusChange call indefinitely (since our status never changes) until we receive an SCARD_IOCTL_CANCEL");

            return Ok(());
        }

        // We have some status change to report, send it to the server.
        self.send_device_control_response(
            req,
            GetStatusChangeReturn::new(ReturnCode::Success, reader_states),
        )
    }

    fn create_get_status_change_reader_states(
        call: GetStatusChangeCall,
    ) -> Vec<ReaderStateCommonCall> {
        let mut reader_states = vec![];
        for state in call.states {
            match state.reader.as_str() {
                // PnP is Plug-and-Play. This special reader "name" is used to monitor for
                // new readers being plugged in.
                "\\\\?PnP?\\Notification" => {
                    reader_states.push(ReaderStateCommonCall {
                        current_state: state.common.current_state,
                        event_state: state.common.current_state,
                        atr_length: state.common.atr_length,
                        atr: state.common.atr,
                    });
                }
                // This is our actual emulated smartcard reader. We always advertise its state as
                // "present".
                TELEPORT_READER_NAME => {
                    let (atr_length, atr) = padded_atr::<36>();
                    reader_states.push(ReaderStateCommonCall {
                        current_state: state.common.current_state,
                        event_state: CardStateFlags::SCARD_STATE_CHANGED
                            | CardStateFlags::SCARD_STATE_PRESENT,
                        atr_length,
                        atr,
                    });
                }
                // All other reader names are unknown and unexpected.
                _ => {
                    warn!(
                        "got unexpected reader name [{}], ignoring",
                        state.reader.as_str()
                    );
                    reader_states.push(ReaderStateCommonCall {
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
        reader_states
    }

    fn has_no_change(reader_states: &[ReaderStateCommonCall]) -> bool {
        reader_states
            .iter()
            .all(|state| state.current_state == state.event_state)
    }

    fn handle_connect(
        &mut self,
        req: DeviceControlRequest<ScardIoCtlCode>,
        call: ConnectCall,
    ) -> PduResult<()> {
        let handle = self.contexts.connect(
            call.common.context,
            call.common.context.value,
            self.uuid,
            &self.cert_der,
            &self.key_der,
            self.pin.clone(),
        )?;

        self.send_device_control_response(
            req,
            ConnectReturn::new(ReturnCode::Success, handle, CardProtocol::SCARD_PROTOCOL_T1),
        )
    }

    fn handle_begin_transaction(
        &mut self,
        req: DeviceControlRequest<ScardIoCtlCode>,
    ) -> PduResult<()> {
        self.send_device_control_response(req, LongReturn::new(ReturnCode::Success))
    }

    fn handle_transmit(
        &mut self,
        req: DeviceControlRequest<ScardIoCtlCode>,
        call: TransmitCall,
    ) -> PduResult<()> {
        let cmd =
            CardCommand::<TRANSMIT_DATA_LIMIT>::try_from(&call.send_buffer).map_err(|err| {
                custom_err!(SmartcardBackendError(format!(
                    "failed to parse smartcard command {:?}: {:?}",
                    &call.send_buffer, err
                )))
            })?;

        let card = self.contexts.get_card(&call.handle)?;
        let resp = card.handle(cmd)?;

        self.send_device_control_response(
            req,
            TransmitReturn::new(ReturnCode::Success, None, resp.encode()),
        )
    }

    fn handle_status(&mut self, req: DeviceControlRequest<ScardIoCtlCode>) -> PduResult<()> {
        let enc = match req.io_control_code {
            ScardIoCtlCode::StatusW => CharacterSet::Unicode,
            ScardIoCtlCode::StatusA => CharacterSet::Ansi,
            _ => {
                return Err(custom_err!(SmartcardBackendError(format!(
                    "got unexpected ScardIoCtlCode with a StatusCall: {:?}",
                    req.io_control_code
                ))));
            }
        };

        let (atr_length, atr) = padded_atr::<32>();

        self.send_device_control_response(
            req,
            StatusReturn::new(
                ReturnCode::Success,
                vec![TELEPORT_READER_NAME.to_string()],
                // SPECIFICMODE state means that the card is ready to handle commands in a specific
                // mode, no other negotiation is necessary. Real smartcards would probably negotiate
                // some mode first.
                CardState::SpecificMode,
                CardProtocol::SCARD_PROTOCOL_T1,
                atr,
                atr_length,
                enc,
            ),
        )
    }

    fn handle_release_context(
        &mut self,
        req: DeviceControlRequest<ScardIoCtlCode>,
        call: ContextCall,
    ) -> PduResult<()> {
        self.contexts.release(call.context.value);
        self.send_device_control_response(req, LongReturn::new(ReturnCode::Success))
    }

    fn handle_end_transaction(
        &mut self,
        req: DeviceControlRequest<ScardIoCtlCode>,
    ) -> PduResult<()> {
        self.send_device_control_response(req, LongReturn::new(ReturnCode::Success))
    }

    fn handle_disconnect(
        &mut self,
        req: DeviceControlRequest<ScardIoCtlCode>,
        call: HCardAndDispositionCall,
    ) -> PduResult<()> {
        self.contexts.disconnect(call.handle)?;
        self.send_device_control_response(req, LongReturn::new(ReturnCode::Success))
    }

    fn handle_cancel(
        &mut self,
        req: DeviceControlRequest<ScardIoCtlCode>,
        call: ContextCall,
    ) -> PduResult<()> {
        debug!(
            "received SCARD_IOCTL_CANCEL for context [{}]",
            call.context.value
        );
        if let Some(resp) = self
            .contexts
            .take_scard_cancel_response(call.context.value)?
        {
            // Take the pending SCARD_IOCTL_GETSTATUSCHANGE response and send it back to the server.
            self.client_handle.write_rdpdr(resp.into())?;
        } else {
            warn!("Received SCARD_IOCTL_CANCEL for a context without a pending SCARD_IOCTL_GETSTATUSCHANGE");
        };

        // Also return a response for the SCARD_IOCTL_CANCEL request itself.
        self.send_device_control_response(req, LongReturn::new(ReturnCode::Success))
    }

    fn handle_is_valid_context(
        &mut self,
        req: DeviceControlRequest<ScardIoCtlCode>,
    ) -> PduResult<()> {
        // TODO: Currently we're always just sending ReturnCode::Success (based on awly's pre-
        // IronRDP code). Should we instead be checking if we have such a context in our cache
        // and returning an SCARD_E_INVALID_HANDLE (or something else)?
        self.send_device_control_response(req, LongReturn::new(ReturnCode::Success))
    }

    fn handle_get_device_type_id(
        &mut self,
        req: DeviceControlRequest<ScardIoCtlCode>,
        call: GetDeviceTypeIdCall,
    ) -> PduResult<()> {
        if self.contexts.exists(call.context.value) {
            // Reader type describes the type of the physical connection to the smartcard reader (e.g.
            // USB/serial/TPM). Type "vendor" means a proprietary vendor bus.
            //
            // See "ReaderType" in
            // https://docs.microsoft.com/en-us/windows-hardware/drivers/ddi/smclib/ns-smclib-_scard_reader_capabilitiesconst SCARD_READER_TYPE_VENDOR: u32 = 0xF0;
            const SCARD_READER_TYPE_VENDOR: u32 = 0xF0;
            self.send_device_control_response(
                req,
                GetDeviceTypeIdReturn::new(ReturnCode::Success, SCARD_READER_TYPE_VENDOR),
            )
        } else {
            Err(custom_err!(SmartcardBackendError(format!(
                "got GetDeviceTypeIdCall for unknown context [{}]",
                call.context.value
            ))))
        }
    }

    fn handle_read_cache(
        &mut self,
        req: DeviceControlRequest<ScardIoCtlCode>,
        call: ReadCacheCall,
    ) -> PduResult<()> {
        let (data, return_code) = match self.contexts.read_cache(call)? {
            Some(data) => (data, ReturnCode::Success),
            None => (vec![], ReturnCode::CacheItemNotFound),
        };
        self.send_device_control_response(req, ReadCacheReturn::new(return_code, data))
    }

    fn handle_write_cache(
        &mut self,
        req: DeviceControlRequest<ScardIoCtlCode>,
        call: WriteCacheCall,
    ) -> PduResult<()> {
        self.contexts.write_cache(call)?;
        self.send_device_control_response(req, LongReturn::new(ReturnCode::Success))
    }

    fn handle_get_reader_icon(
        &mut self,
        req: DeviceControlRequest<ScardIoCtlCode>,
    ) -> PduResult<()> {
        self.send_device_control_response(
            req,
            GetReaderIconReturn::new(ReturnCode::UnsupportedFeature, vec![]),
        )
    }

    /// This function returns the error for unsupported combinations of [`ScardIoCtlCode`] and [`ScardCall`].
    fn unsupported_combo_error(ioctl: ScardIoCtlCode, call: ScardCall) -> PduResult<()> {
        Err(custom_err!(SmartcardBackendError(format!(
            "received unsupported combination of ScardIoCtlCode [{:?}] with ScardCall [{:?}]",
            ioctl, call
        ))))
    }

    fn send_device_control_response(
        &self,
        req: DeviceControlRequest<ScardIoCtlCode>,
        output_buffer: impl rpce::Encode + 'static,
    ) -> PduResult<()> {
        let resp =
            DeviceControlResponse::new(req, NtStatus::SUCCESS, Some(Box::new(output_buffer)));
        self.client_handle.write_rdpdr(resp.into())?;
        Ok(())
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

    fn establish(&mut self) -> ScardContext {
        let ctx_internal = ContextInternal::new();
        let id = self.next_id;
        self.next_id += 1;
        let ctx = ScardContext::new(id);
        self.contexts.insert(id, ctx_internal);
        ctx
    }

    fn connect(
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

    fn disconnect(&mut self, handle: ScardHandle) -> PduResult<()> {
        self.get_internal_mut(handle.context.value)?
            .disconnect(handle.value);
        Ok(())
    }

    fn set_scard_cancel_response(&mut self, id: u32, resp: DeviceControlResponse) -> PduResult<()> {
        debug!("setting SCARD_IOCTL_CANCEL response for context [{}]", id);
        self.get_internal_mut(id)?.set_scard_cancel_response(resp)
    }

    fn take_scard_cancel_response(&mut self, id: u32) -> PduResult<Option<DeviceControlResponse>> {
        Ok(self.get_internal_mut(id)?.take_scard_cancel_response())
    }

    fn get_card(&mut self, handle: &ScardHandle) -> PduResult<&mut piv::Card<TRANSMIT_DATA_LIMIT>> {
        self.get_internal_mut(handle.context.value)?
            .get(handle.value)
            .ok_or_else(|| other_err!("unknown ScardHandle"))
    }

    fn exists(&self, id: u32) -> bool {
        self.contexts.contains_key(&id)
    }

    fn read_cache(&mut self, call: ReadCacheCall) -> PduResult<Option<Vec<u8>>> {
        Ok(self
            .get_internal_mut(call.common.context.value)?
            .cache_read(&call.lookup_name))
    }

    fn write_cache(&mut self, call: WriteCacheCall) -> PduResult<()> {
        self.get_internal_mut(call.common.context.value)?
            .cache_write(call.lookup_name, call.common.data);
        Ok(())
    }

    fn get_internal_mut(&mut self, id: u32) -> PduResult<&mut ContextInternal> {
        self.contexts
            .get_mut(&id)
            .ok_or_else(|| other_err!("unknown context id"))
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
            return Err(other_err!("SCARD_IOCTL_CANCEL already received",));
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

fn padded_atr<const SIZE: usize>() -> (u32, [u8; SIZE]) {
    let mut atr = [0; SIZE];
    let len = STATIC_ATR.len().min(SIZE);
    atr[..len].copy_from_slice(&STATIC_ATR[..len]);
    (len as u32, atr)
}

pub const SCARD_DEVICE_ID: u32 = 1;
const TELEPORT_READER_NAME: &str = "Teleport";
// TRANSMIT_DATA_LIMIT is the maximum size of transmit request/response short data, in bytes.
const TRANSMIT_DATA_LIMIT: usize = 1024;
const TIMEOUT_INFINITE: u32 = 0xffffffff;
const TIMEOUT_IMMEDIATE: u32 = 0;

/// A generic error type for the SmartcardBackend that can contain any arbitrary error message.
#[derive(Debug)]
#[allow(dead_code)] // The internal `String` is "dead code" according to the compiler, but we want it for debugging purposes.
struct SmartcardBackendError(pub String);

impl std::fmt::Display for SmartcardBackendError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{:#?}", self)
    }
}

impl std::error::Error for SmartcardBackendError {}
