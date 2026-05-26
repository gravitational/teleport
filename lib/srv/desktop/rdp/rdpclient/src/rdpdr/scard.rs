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
use ironrdp_core::{EncodeResult, WriteCursor};
use ironrdp_pdu::utils::CharacterSet;
use ironrdp_pdu::PduResult;
use ironrdp_pdu::{pdu_other_err, PduError};
use ironrdp_rdpdr::pdu::efs::{DeviceControlRequest, DeviceControlResponse, NtStatus};
use ironrdp_rdpdr::pdu::esc::rpce::Pdu;
use ironrdp_rdpdr::pdu::esc::{
    rpce, CardProtocol, CardState, CardStateFlags, ConnectCall, ConnectReturn, ContextCall,
    EstablishContextReturn, GetDeviceTypeIdCall, GetDeviceTypeIdReturn, GetReaderIconReturn,
    GetStatusChangeCall, GetStatusChangeReturn, HCardAndDispositionCall, ListReadersReturn,
    LongReturn, ReadCacheCall, ReadCacheReturn, ReaderState, ReaderStateCommonCall, ReturnCode,
    ScardCall, ScardContext, ScardHandle, ScardIoCtlCode, StatusReturn, TransmitCall,
    TransmitReturn, WriteCacheCall,
};
use iso7816::Command as CardCommand;
use log::{debug, warn};
use std::collections::HashMap;
use uuid::Uuid;

/// Carries the deferred state for an in-flight `SCARD_IOCTL_GETSTATUSCHANGE` request that was
/// received with an infinite timeout but had no state change to report at the time.
///
/// The request (`req`) and the cancellation response (`response`) are stored here
/// until a matching `SCARD_IOCTL_CANCEL` arrives for the same context, at which point both
/// are flushed through the I/O channel by [`ScardBackend::handle`].
#[derive(Debug, PartialEq)]
struct PendingGetStatusChange {
    req: DeviceControlRequest<ScardIoCtlCode>,
    response: ScardResponsePayload,
}

/// Describes the outcome of dispatching a single smartcard IOCTL in [`ScardBackend::handle_impl`].
#[derive(Debug, PartialEq)]
enum ScardHandleResponse {
    /// No response should be sent to the server for this request yet.
    ///
    /// Used when a `SCARD_IOCTL_GETSTATUSCHANGE` with an infinite timeout has no change to report.
    Deferred,
    /// A single response PDU for the current IOCTL should be sent immediately.
    Common(ScardResponsePayload),
    /// A `SCARD_IOCTL_CANCEL` was dispatched.
    ///
    /// If `get_status_change_response` is `Some`, the previously deferred
    /// `SCARD_IOCTL_GETSTATUSCHANGE` response must be sent first, followed by
    /// `cancel_response` for the cancel request itself.
    CancelDeferred {
        get_status_change_response: Option<PendingGetStatusChange>,
        cancel_response: ScardResponsePayload,
    },
}

impl ScardHandleResponse {
    fn success_long_return() -> Self {
        Self::Common(ScardResponsePayload::Long(
            LongReturn::new(ReturnCode::Success).into_inner(),
        ))
    }
}

/// A unified container for all PDUs returned by the smartcard backend.
///
/// Each variant wraps one concrete `*Return` type from `ironrdp_rdpdr::pdu::esc`. The enum
/// implements [`rpce::HeaderlessEncode`] by delegating to the inner type, so it can be passed
/// directly to [`ScardBackend::send_device_control_response`] as a `Pdu<ScardResponsePayload>`.
#[derive(Debug, PartialEq)]
enum ScardResponsePayload {
    Long(LongReturn),
    EstablishContext(EstablishContextReturn),
    ListReaders(ListReadersReturn),
    GetStatusChange(GetStatusChangeReturn),
    Connect(ConnectReturn),
    Transmit(TransmitReturn),
    Status(StatusReturn),
    GetDeviceTypeId(GetDeviceTypeIdReturn),
    ReadCache(ReadCacheReturn),
    GetReaderIcon(GetReaderIconReturn),
}

impl rpce::HeaderlessEncode for ScardResponsePayload {
    fn encode(&self, dst: &mut WriteCursor<'_>) -> EncodeResult<()> {
        match self {
            Self::Long(long_return) => long_return.encode(dst),
            Self::EstablishContext(establish_context) => establish_context.encode(dst),
            Self::ListReaders(list_readers) => list_readers.encode(dst),
            Self::GetStatusChange(get_status_change) => get_status_change.encode(dst),
            Self::Connect(connect) => connect.encode(dst),
            Self::Transmit(transmit) => transmit.encode(dst),
            Self::Status(status) => status.encode(dst),
            Self::GetDeviceTypeId(get_device_type_id) => get_device_type_id.encode(dst),
            Self::ReadCache(read_cache) => read_cache.encode(dst),
            Self::GetReaderIcon(get_reader_icon) => get_reader_icon.encode(dst),
        }
    }

    fn name(&self) -> &'static str {
        match self {
            Self::Long(long_return) => long_return.name(),
            Self::EstablishContext(establish_context) => establish_context.name(),
            Self::ListReaders(list_readers) => list_readers.name(),
            Self::GetStatusChange(get_status_change) => get_status_change.name(),
            Self::Connect(connect) => connect.name(),
            Self::Transmit(transmit) => transmit.name(),
            Self::Status(status) => status.name(),
            Self::GetDeviceTypeId(get_device_type_id) => get_device_type_id.name(),
            Self::ReadCache(read_cache) => read_cache.name(),
            Self::GetReaderIcon(get_reader_icon) => get_reader_icon.name(),
        }
    }

    fn size(&self) -> usize {
        match self {
            Self::Long(long_return) => long_return.size(),
            Self::EstablishContext(establish_context) => establish_context.size(),
            Self::ListReaders(list_readers) => list_readers.size(),
            Self::GetStatusChange(get_status_change) => get_status_change.size(),
            Self::Connect(connect) => connect.size(),
            Self::Transmit(transmit) => transmit.size(),
            Self::Status(status) => status.size(),
            Self::GetDeviceTypeId(get_device_type_id) => get_device_type_id.size(),
            Self::ReadCache(read_cache) => read_cache.size(),
            Self::GetReaderIcon(get_reader_icon) => get_reader_icon.size(),
        }
    }
}

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
        match self.handle_impl(&req, call)? {
            ScardHandleResponse::Deferred => Ok(()),
            ScardHandleResponse::Common(response) => {
                self.send_device_control_response(req, Pdu(response))
            }
            ScardHandleResponse::CancelDeferred {
                get_status_change_response,
                cancel_response,
            } => {
                if let Some(get_status_change_response) = get_status_change_response {
                    self.send_device_control_response(
                        get_status_change_response.req,
                        Pdu(get_status_change_response.response),
                    )?;
                }
                self.send_device_control_response(req, Pdu(cancel_response))
            }
        }
    }

    fn handle_impl(
        &mut self,
        req: &DeviceControlRequest<ScardIoCtlCode>,
        call: ScardCall,
    ) -> PduResult<ScardHandleResponse> {
        Ok(match req.io_control_code {
            ScardIoCtlCode::AccessStartedEvent => match call {
                ScardCall::AccessStartedEventCall(_) => self.handle_access_started_event(),
                _ => return Err(Self::unsupported_combo_error(req.io_control_code, call)),
            },
            ScardIoCtlCode::EstablishContext => match call {
                ScardCall::EstablishContextCall(_) => self.handle_establish_context(),
                _ => return Err(Self::unsupported_combo_error(req.io_control_code, call)),
            },
            ScardIoCtlCode::ListReadersW | ScardIoCtlCode::ListReadersA => match call {
                ScardCall::ListReadersCall(_) => self.handle_list_readers(),
                _ => return Err(Self::unsupported_combo_error(req.io_control_code, call)),
            },
            ScardIoCtlCode::GetStatusChangeW | ScardIoCtlCode::GetStatusChangeA => match call {
                ScardCall::GetStatusChangeCall(call) => {
                    self.handle_get_status_change(req.clone(), call)?
                }
                _ => return Err(Self::unsupported_combo_error(req.io_control_code, call)),
            },
            ScardIoCtlCode::ConnectW | ScardIoCtlCode::ConnectA => match call {
                ScardCall::ConnectCall(call) => self.handle_connect(call)?,
                _ => return Err(Self::unsupported_combo_error(req.io_control_code, call)),
            },
            ScardIoCtlCode::BeginTransaction => match call {
                ScardCall::HCardAndDispositionCall(_) => self.handle_begin_transaction(),
                _ => return Err(Self::unsupported_combo_error(req.io_control_code, call)),
            },
            ScardIoCtlCode::Transmit => match call {
                ScardCall::TransmitCall(call) => self.handle_transmit(call)?,
                _ => return Err(Self::unsupported_combo_error(req.io_control_code, call)),
            },
            ScardIoCtlCode::StatusW | ScardIoCtlCode::StatusA => match call {
                ScardCall::StatusCall(_) => self.handle_status(req)?,
                _ => return Err(Self::unsupported_combo_error(req.io_control_code, call)),
            },
            ScardIoCtlCode::ReleaseContext => match call {
                ScardCall::ContextCall(call) => self.handle_release_context(call),
                _ => return Err(Self::unsupported_combo_error(req.io_control_code, call)),
            },
            ScardIoCtlCode::EndTransaction => match call {
                ScardCall::HCardAndDispositionCall(_) => self.handle_end_transaction(),
                _ => return Err(Self::unsupported_combo_error(req.io_control_code, call)),
            },
            ScardIoCtlCode::Disconnect => match call {
                ScardCall::HCardAndDispositionCall(call) => self.handle_disconnect(call)?,
                _ => return Err(Self::unsupported_combo_error(req.io_control_code, call)),
            },
            ScardIoCtlCode::Cancel => match call {
                ScardCall::ContextCall(call) => self.handle_cancel(call)?,
                _ => return Err(Self::unsupported_combo_error(req.io_control_code, call)),
            },
            ScardIoCtlCode::IsValidContext => match call {
                ScardCall::ContextCall(_) => self.handle_is_valid_context(),
                _ => return Err(Self::unsupported_combo_error(req.io_control_code, call)),
            },
            ScardIoCtlCode::GetDeviceTypeId => match call {
                ScardCall::GetDeviceTypeIdCall(call) => self.handle_get_device_type_id(call)?,
                _ => return Err(Self::unsupported_combo_error(req.io_control_code, call)),
            },
            ScardIoCtlCode::ReadCacheW | ScardIoCtlCode::ReadCacheA => match call {
                ScardCall::ReadCacheCall(call) => self.handle_read_cache(call)?,
                _ => return Err(Self::unsupported_combo_error(req.io_control_code, call)),
            },
            ScardIoCtlCode::WriteCacheW | ScardIoCtlCode::WriteCacheA => match call {
                ScardCall::WriteCacheCall(call) => self.handle_write_cache(call)?,
                _ => return Err(Self::unsupported_combo_error(req.io_control_code, call)),
            },
            ScardIoCtlCode::GetReaderIcon => match call {
                ScardCall::GetReaderIconCall(_) => self.handle_get_reader_icon(),
                _ => return Err(Self::unsupported_combo_error(req.io_control_code, call)),
            },
            _ => Err(pdu_other_err!(
                "",
                source:SmartcardBackendError(format!(
                    "received unhandled ScardIoCtlCode: {:?}",
                    req.io_control_code
                ))
            ))?,
        })
    }

    fn handle_access_started_event(&mut self) -> ScardHandleResponse {
        ScardHandleResponse::success_long_return()
    }

    fn handle_establish_context(&mut self) -> ScardHandleResponse {
        let ctx = self.contexts.establish();

        ScardHandleResponse::Common(ScardResponsePayload::EstablishContext(
            EstablishContextReturn::new(ReturnCode::Success, ctx).into_inner(),
        ))
    }

    fn handle_list_readers(&mut self) -> ScardHandleResponse {
        ScardHandleResponse::Common(ScardResponsePayload::ListReaders(
            ListReadersReturn::new(ReturnCode::Success, vec![TELEPORT_READER_NAME.to_string()])
                .into_inner(),
        ))
    }

    fn handle_get_status_change(
        &mut self,
        req: DeviceControlRequest<ScardIoCtlCode>,
        call: GetStatusChangeCall,
    ) -> PduResult<ScardHandleResponse> {
        let timeout = call.timeout;
        let context_id = call.context.value;

        if timeout != TIMEOUT_INFINITE && timeout != TIMEOUT_IMMEDIATE {
            // We've never seen one of these, but we log a warning here in case we ever come
            // across one and need to debug a related issue.
            warn!(
                "logic for a non-infinite/non-immediate timeout [{}] is not implemented",
                timeout
            );
        }

        let mut reader_states = call.states;

        // Check the integrity of the reader state structures:
        // https://github.com/LudovicRousseau/PCSC/blob/7eea3e3e3bb7b4b2d815a3f390394b4d690f5ea8/src/winscard_clnt.c#L1742
        if reader_states.iter().any(|state| state.reader.is_empty()) {
            return Ok(ScardHandleResponse::Common(
                ScardResponsePayload::GetStatusChange(
                    GetStatusChangeReturn::new(
                        ReturnCode::InvalidValue,
                        reader_states
                            .into_iter()
                            .map(|state| state.common)
                            .collect(),
                    )
                    .into_inner(),
                ),
            ));
        }

        // If there are no readers, or if all readers are in the SCARD_STATE_IGNORE state,
        // we should return immediately with `ReturnCode::Success`. This mirrors the behavior
        // of the reference pcsc-lite implementation:
        //
        // - All readers ignored: https://github.com/LudovicRousseau/PCSC/blob/7eea3e3e3bb7b4b2d815a3f390394b4d690f5ea8/src/winscard_clnt.c#L1749
        // - Reader list is empty: https://github.com/LudovicRousseau/PCSC/blob/7eea3e3e3bb7b4b2d815a3f390394b4d690f5ea8/src/winscard_clnt.c#L1766
        if reader_states.is_empty()
            || reader_states.iter().all(|state| {
                state
                    .common
                    .current_state
                    .contains(CardStateFlags::SCARD_STATE_IGNORE)
            })
        {
            return Ok(ScardHandleResponse::Common(
                ScardResponsePayload::GetStatusChange(
                    GetStatusChangeReturn::new(
                        ReturnCode::Success,
                        reader_states
                            .into_iter()
                            .map(|state| state.common)
                            .collect(),
                    )
                    .into_inner(),
                ),
            ));
        }

        // Compute the `event_state` field for each reader.
        Self::compute_reader_event_states(&mut reader_states);

        let reader_states_common: Vec<ReaderStateCommonCall> = reader_states
            .into_iter()
            .map(|state| state.common)
            .collect();

        // Check whether the state of any reader has been changed.
        let has_change = reader_states_common.iter().any(|state| {
            state
                .event_state
                .contains(CardStateFlags::SCARD_STATE_CHANGED)
        });

        if timeout == TIMEOUT_IMMEDIATE {
            let return_code = if has_change {
                ReturnCode::Success
            } else {
                ReturnCode::Timeout
            };
            return Ok(ScardHandleResponse::Common(
                ScardResponsePayload::GetStatusChange(
                    GetStatusChangeReturn::new(return_code, reader_states_common).into_inner(),
                ),
            ));
        }

        if has_change {
            return Ok(ScardHandleResponse::Common(
                ScardResponsePayload::GetStatusChange(
                    GetStatusChangeReturn::new(ReturnCode::Success, reader_states_common)
                        .into_inner(),
                ),
            ));
        }

        // Received a GetStatusChangeCall with an infinite timeout, and we don't have a change.
        // So we're adding a GetStatusChangeResponse holding a DeviceControlRequest and
        // GetStatusChangeReturn with its return code set to SCARD_E_CANCELLED to this Context.
        // This value will be returned when we get an SCARD_IOCTL_CANCEL call for this Context.
        self.contexts.set_scard_cancel_response(
            context_id,
            PendingGetStatusChange {
                req,
                response: ScardResponsePayload::GetStatusChange(
                    GetStatusChangeReturn::new(ReturnCode::Cancelled, reader_states_common)
                        .into_inner(),
                ),
            },
        )?;

        debug!("blocking GetStatusChange call indefinitely (since our status never changes) until we receive an SCARD_IOCTL_CANCEL");

        Ok(ScardHandleResponse::Deferred)
    }

    fn compute_reader_event_states(reader_states: &mut [ReaderState]) {
        for state in reader_states {
            // Clear event_state before processing.
            //
            // Reference: https://github.com/LudovicRousseau/PCSC/blob/7eea3e3e3bb7b4b2d815a3f390394b4d690f5ea8/src/winscard_clnt.c#L1822
            state.common.event_state = CardStateFlags::empty();

            // The reader must be ignored.
            // https://github.com/LudovicRousseau/PCSC/blob/7eea3e3e3bb7b4b2d815a3f390394b4d690f5ea8/src/winscard_clnt.c#L1874
            if state
                .common
                .current_state
                .contains(CardStateFlags::SCARD_STATE_IGNORE)
            {
                continue;
            }

            match state.reader.as_str() {
                // This is our actual emulated smartcard reader.
                TELEPORT_READER_NAME => {
                    let (atr_length, atr) = padded_atr::<36>();
                    state.common.event_state = CardStateFlags::SCARD_STATE_PRESENT;
                    state.common.atr_length = atr_length;
                    state.common.atr = atr;

                    // Strip SCARD_STATE_CHANGED from current_state before comparison.
                    // The caller typically feeds the previous event_state back as current_state,
                    // which may carry the SCARD_STATE_CHANGED bit. That bit is a transient output signal
                    // and must not influence the comparison - otherwise we would always detect a change.
                    //
                    // The reference pcsc-lite implementation avoids this issue implicitly,
                    // since it never compares the full state value directly.
                    // Instead, it checks individual flags (e.g. `dwCurrentState & SCARD_STATE_PRESENT`)
                    // and never tests against `SCARD_STATE_CHANGED` state.
                    // As a result, stale `SCARD_STATE_CHANGED` are naturally ignored.
                    //
                    // Because we compare full CardStateFlags values for equality here, we must mask out
                    // the `SCARD_STATE_CHANGED` bit explicitly to preserve the same behavior.
                    //
                    // Reference: https://github.com/LudovicRousseau/PCSC/blob/7eea3e3e3bb7b4b2d815a3f390394b4d690f5ea8/src/winscard_clnt.c#L1875-L2138
                    let current_without_changed =
                        state.common.current_state & !CardStateFlags::SCARD_STATE_CHANGED;
                    if current_without_changed != state.common.event_state {
                        state.common.event_state |= CardStateFlags::SCARD_STATE_CHANGED;
                    }
                }
                // This is the special reader name used to monitor for new readers being plugged in.
                // We leave its event_state empty because we never add new readers.
                //
                // The reference pcsc-lite implementation only sets dwEventState for this reader
                // when the reader count changes. If it does not change, dwEventState stays empty
                // (it was cleared before the loop). Since we never have reader count changes,
                // leaving event_state empty is the correct equivalent behavior.
                //
                // Reference: https://github.com/LudovicRousseau/PCSC/blob/7eea3e3e3bb7b4b2d815a3f390394b4d690f5ea8/src/winscard_clnt.c#L1903-L1918
                PLUG_AND_PLAY_READER_NAME => (),
                // All other reader names are unknown and unexpected.
                //
                // According to the specification, if the SCARD_STATE_UNKNOWN bit is set,
                // then SCARD_STATE_CHANGED and SCARD_STATE_IGNORE will also be set.
                //
                // Reference: https://learn.microsoft.com/en-us/windows/win32/api/winscard/ns-winscard-scard_readerstatea
                _ => {
                    warn!(
                        "got unexpected reader name [{}], ignoring",
                        state.reader.as_str()
                    );
                    state.common.event_state = CardStateFlags::SCARD_STATE_UNKNOWN
                        | CardStateFlags::SCARD_STATE_IGNORE
                        | CardStateFlags::SCARD_STATE_CHANGED;
                }
            }
        }
    }

    fn handle_connect(&mut self, call: ConnectCall) -> PduResult<ScardHandleResponse> {
        let handle = self.contexts.connect(
            call.common.context,
            call.common.context.value,
            self.uuid,
            &self.cert_der,
            &self.key_der,
            self.pin.clone(),
        )?;

        Ok(ScardHandleResponse::Common(ScardResponsePayload::Connect(
            ConnectReturn::new(ReturnCode::Success, handle, CardProtocol::SCARD_PROTOCOL_T1)
                .into_inner(),
        )))
    }

    fn handle_begin_transaction(&mut self) -> ScardHandleResponse {
        ScardHandleResponse::success_long_return()
    }

    fn handle_transmit(&mut self, call: TransmitCall) -> PduResult<ScardHandleResponse> {
        let cmd = CardCommand::<TRANSMIT_DATA_LIMIT>::try_from(&call.send_buffer);

        let transmit_return = match cmd {
            Ok(cmd) => {
                let card = self.contexts.get_card(&call.handle)?;
                let resp = card.handle(cmd)?;

                TransmitReturn::new(ReturnCode::Success, None, resp.encode()).into_inner()
            }
            Err(err) => {
                warn!("error parsing smart card command: {:?} - {:?}", call, err);
                TransmitReturn::new(ReturnCode::InvalidValue, None, Vec::new()).into_inner()
            }
        };

        Ok(ScardHandleResponse::Common(ScardResponsePayload::Transmit(
            transmit_return,
        )))
    }

    fn handle_status(
        &mut self,
        req: &DeviceControlRequest<ScardIoCtlCode>,
    ) -> PduResult<ScardHandleResponse> {
        let enc = match req.io_control_code {
            ScardIoCtlCode::StatusW => CharacterSet::Unicode,
            ScardIoCtlCode::StatusA => CharacterSet::Ansi,
            _ => {
                return Err(pdu_other_err!(
                    "",
                    source:SmartcardBackendError(format!(
                        "got unexpected ScardIoCtlCode with a StatusCall: {:?}",
                        req.io_control_code
                    ))
                ));
            }
        };

        let (atr_length, atr) = padded_atr::<32>();

        Ok(ScardHandleResponse::Common(ScardResponsePayload::Status(
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
            )
            .into_inner(),
        )))
    }

    fn handle_release_context(&mut self, call: ContextCall) -> ScardHandleResponse {
        self.contexts.release(call.context.value);
        ScardHandleResponse::success_long_return()
    }

    fn handle_end_transaction(&mut self) -> ScardHandleResponse {
        ScardHandleResponse::success_long_return()
    }

    fn handle_disconnect(
        &mut self,
        call: HCardAndDispositionCall,
    ) -> PduResult<ScardHandleResponse> {
        self.contexts.disconnect(call.handle)?;
        Ok(ScardHandleResponse::success_long_return())
    }

    fn handle_cancel(&mut self, call: ContextCall) -> PduResult<ScardHandleResponse> {
        debug!(
            "received SCARD_IOCTL_CANCEL for context [{}]",
            call.context.value
        );

        // Take the pending SCARD_IOCTL_GETSTATUSCHANGE response.
        let get_status_change_response = self
            .contexts
            .take_scard_cancel_response(call.context.value)?;
        if get_status_change_response.is_none() {
            warn!("Received SCARD_IOCTL_CANCEL for a context without a pending SCARD_IOCTL_GETSTATUSCHANGE");
        }

        Ok(ScardHandleResponse::CancelDeferred {
            get_status_change_response,
            // Set a response for the SCARD_IOCTL_CANCEL request itself.
            cancel_response: ScardResponsePayload::Long(
                LongReturn::new(ReturnCode::Success).into_inner(),
            ),
        })
    }

    fn handle_is_valid_context(&mut self) -> ScardHandleResponse {
        // TODO: Currently we're always just sending ReturnCode::Success (based on awly's pre-
        // IronRDP code). Should we instead be checking if we have such a context in our cache
        // and returning an SCARD_E_INVALID_HANDLE (or something else)?
        ScardHandleResponse::success_long_return()
    }

    fn handle_get_device_type_id(
        &mut self,
        call: GetDeviceTypeIdCall,
    ) -> PduResult<ScardHandleResponse> {
        if self.contexts.exists(call.context.value) {
            // Reader type describes the type of the physical connection to the smartcard reader (e.g.
            // USB/serial/TPM). Type "vendor" means a proprietary vendor bus.
            //
            // See "ReaderType" in
            // https://docs.microsoft.com/en-us/windows-hardware/drivers/ddi/smclib/ns-smclib-_scard_reader_capabilities
            const SCARD_READER_TYPE_VENDOR: u32 = 0xF0;
            Ok(ScardHandleResponse::Common(
                ScardResponsePayload::GetDeviceTypeId(
                    GetDeviceTypeIdReturn::new(ReturnCode::Success, SCARD_READER_TYPE_VENDOR)
                        .into_inner(),
                ),
            ))
        } else {
            Err(pdu_other_err!(
                "",
                source:SmartcardBackendError(format!(
                    "got GetDeviceTypeIdCall for unknown context [{}]",
                    call.context.value
                ))
            ))
        }
    }

    fn handle_read_cache(&mut self, call: ReadCacheCall) -> PduResult<ScardHandleResponse> {
        let (data, return_code) = match self.contexts.read_cache(call)? {
            Some(data) => (data, ReturnCode::Success),
            None => (vec![], ReturnCode::CacheItemNotFound),
        };
        Ok(ScardHandleResponse::Common(
            ScardResponsePayload::ReadCache(ReadCacheReturn::new(return_code, data).into_inner()),
        ))
    }

    fn handle_write_cache(&mut self, call: WriteCacheCall) -> PduResult<ScardHandleResponse> {
        self.contexts.write_cache(call)?;
        Ok(ScardHandleResponse::success_long_return())
    }

    fn handle_get_reader_icon(&mut self) -> ScardHandleResponse {
        ScardHandleResponse::Common(ScardResponsePayload::GetReaderIcon(
            GetReaderIconReturn::new(ReturnCode::UnsupportedFeature, vec![]).into_inner(),
        ))
    }

    /// This function returns the error for unsupported combinations of [`ScardIoCtlCode`] and [`ScardCall`].
    fn unsupported_combo_error(ioctl: ScardIoCtlCode, call: ScardCall) -> PduError {
        pdu_other_err!(
            "",
            source:SmartcardBackendError(format!(
                "received unsupported combination of ScardIoCtlCode [{:?}] with ScardCall [{:?}]",
                ioctl, call
            ))
        )
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

    fn set_scard_cancel_response(
        &mut self,
        id: u32,
        resp: PendingGetStatusChange,
    ) -> PduResult<()> {
        debug!("setting SCARD_IOCTL_CANCEL response for context [{}]", id);
        self.get_internal_mut(id)?.set_scard_cancel_response(resp)
    }

    fn take_scard_cancel_response(&mut self, id: u32) -> PduResult<Option<PendingGetStatusChange>> {
        Ok(self.get_internal_mut(id)?.take_scard_cancel_response())
    }

    fn get_card(&mut self, handle: &ScardHandle) -> PduResult<&mut piv::Card<TRANSMIT_DATA_LIMIT>> {
        self.get_internal_mut(handle.context.value)?
            .get(handle.value)
            .ok_or_else(|| pdu_other_err!("unknown ScardHandle"))
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
            .ok_or_else(|| pdu_other_err!("unknown context id"))
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
    // return a GetStatusChange_Return (embedded in a PendingGetStatusChange) with
    // its return code set to SCARD_E_CANCELLED in the case that we receive a
    // SCARD_IOCTL_CANCEL.
    //
    // This value will be set during the handling of the SCARD_IOCTL_GETSTATUSCHANGE, so that
    // it can be fetched and returned in response to a SCARD_IOCTL_CANCEL.
    scard_cancel_response: Option<PendingGetStatusChange>,
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

    fn set_scard_cancel_response(&mut self, resp: PendingGetStatusChange) -> PduResult<()> {
        if self.scard_cancel_response.is_some() {
            return Err(pdu_other_err!(
                "a pending SCARD_IOCTL_GETSTATUSCHANGE response already exists for this context",
            ));
        }
        self.scard_cancel_response = Some(resp);
        Ok(())
    }

    fn take_scard_cancel_response(&mut self) -> Option<PendingGetStatusChange> {
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
/// PnP is Plug-and-Play. This special reader "name" is used to monitor for
/// new readers being plugged in.
const PLUG_AND_PLAY_READER_NAME: &str = "\\\\?PnP?\\Notification";
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

#[cfg(test)]
mod tests {
    use super::*;
    use crate::client::FunctionReceiver;
    use ironrdp_rdpdr::pdu::efs::{DeviceIoRequest, MajorFunction, MinorFunction};
    use ironrdp_rdpdr::pdu::esc::{
        EstablishContextCall, ReaderState, ReaderStateCommonCall, Scope,
    };

    fn build_test_scard_backend() -> (ScardBackend, FunctionReceiver) {
        let (client_handle, function_receiver) = ClientHandle::new(100);
        let scard_backend = ScardBackend::new(client_handle, Vec::new(), Vec::new(), String::new());
        (scard_backend, function_receiver)
    }

    fn build_device_control_request(
        io_control_code: ScardIoCtlCode,
    ) -> DeviceControlRequest<ScardIoCtlCode> {
        DeviceControlRequest {
            header: DeviceIoRequest {
                device_id: 0,
                file_id: 0,
                completion_id: 0,
                major_function: MajorFunction::DeviceControl,
                minor_function: MinorFunction::from(0),
            },
            output_buffer_length: 256,
            input_buffer_length: 256,
            io_control_code,
        }
    }

    mod handle {
        use super::*;
        use crate::client::ClientFunction;
        use ironrdp_rdpdr::pdu::RdpdrPdu;
        use tokio::sync::mpsc::error::TryRecvError;

        #[test]
        fn all_scard_handle_response_variants_are_correctly_dispatched() {
            let (mut scard_backend, mut function_receiver) = build_test_scard_backend();
            let establish_context_req =
                build_device_control_request(ScardIoCtlCode::EstablishContext);
            let get_status_change_req =
                build_device_control_request(ScardIoCtlCode::GetStatusChangeW);
            let cancel_req = build_device_control_request(ScardIoCtlCode::Cancel);

            // Verify that ScardBackend::handle correctly handles ScardHandleResponse::Common.
            scard_backend
                .handle(
                    establish_context_req.clone(),
                    ScardCall::EstablishContextCall(EstablishContextCall { scope: Scope::User }),
                )
                .expect("EstablishContext failed");

            // Receive the EstablishContextReturn PDU emitted by the backend.
            let establish_context_pdu = match function_receiver.try_recv() {
                Ok(ClientFunction::WriteRdpdr(pdu)) => pdu,
                other => panic!("expected WriteRdpdr, got {:?}", other),
            };

            let expected_establish_context_pdu =
                RdpdrPdu::DeviceControlResponse(DeviceControlResponse::new(
                    establish_context_req,
                    NtStatus::SUCCESS,
                    Some(Box::new(EstablishContextReturn::new(
                        ReturnCode::Success,
                        ScardContext::new(1),
                    ))),
                ));

            pretty_assertions::assert_eq!(
                ironrdp_core::encode_vec(&establish_context_pdu).unwrap(),
                ironrdp_core::encode_vec(&expected_establish_context_pdu).unwrap()
            );

            // Verify that ScardBackend::handle correctly handles ScardHandleResponse::Deferred.
            scard_backend
                .handle(
                    get_status_change_req.clone(),
                    ScardCall::GetStatusChangeCall(GetStatusChangeCall {
                        context: ScardContext::new(1),
                        timeout: TIMEOUT_INFINITE,
                        states_ptr_length: 0,
                        states_ptr: 0,
                        states_length: 1,
                        states: vec![ReaderState {
                            reader: TELEPORT_READER_NAME.into(),
                            common: ReaderStateCommonCall {
                                current_state: CardStateFlags::SCARD_STATE_PRESENT,
                                event_state: CardStateFlags::empty(),
                                atr_length: 0,
                                atr: [0; 36],
                            },
                        }],
                    }),
                )
                .expect("ScardBackend::handle failed");

            match function_receiver.try_recv() {
                Err(TryRecvError::Empty) => (),
                other => panic!("expected empty channel, got {:?}", other),
            };

            // Verify that ScardBackend::handle correctly handles ScardHandleResponse::CancelDeferred.
            scard_backend
                .handle(
                    cancel_req.clone(),
                    ScardCall::ContextCall(ContextCall {
                        context: ScardContext::new(1),
                    }),
                )
                .expect("ScardBackend::handle failed");

            // Receive the GetStatusChangeReturn PDU emitted by the backend.
            let get_status_change_pdu = match function_receiver.try_recv() {
                Ok(ClientFunction::WriteRdpdr(pdu)) => pdu,
                other => panic!("expected WriteRdpdr, got {:?}", other),
            };

            let (atr_length, atr) = padded_atr::<36>();
            let expected_get_status_change_pdu =
                RdpdrPdu::DeviceControlResponse(DeviceControlResponse::new(
                    get_status_change_req,
                    NtStatus::SUCCESS,
                    Some(Box::new(GetStatusChangeReturn::new(
                        ReturnCode::Cancelled,
                        vec![ReaderStateCommonCall {
                            current_state: CardStateFlags::SCARD_STATE_PRESENT,
                            event_state: CardStateFlags::SCARD_STATE_PRESENT,
                            atr_length,
                            atr,
                        }],
                    ))),
                ));

            pretty_assertions::assert_eq!(
                ironrdp_core::encode_vec(&get_status_change_pdu).unwrap(),
                ironrdp_core::encode_vec(&expected_get_status_change_pdu).unwrap()
            );

            // Receive the cancel PDU emitted by the backend.
            let cancel_pdu = match function_receiver.try_recv() {
                Ok(ClientFunction::WriteRdpdr(pdu)) => pdu,
                other => panic!("expected WriteRdpdr, got {:?}", other),
            };

            let expected_cancel_pdu = RdpdrPdu::DeviceControlResponse(DeviceControlResponse::new(
                cancel_req,
                NtStatus::SUCCESS,
                Some(Box::new(LongReturn::new(ReturnCode::Success))),
            ));

            pretty_assertions::assert_eq!(
                ironrdp_core::encode_vec(&cancel_pdu).unwrap(),
                ironrdp_core::encode_vec(&expected_cancel_pdu).unwrap()
            );
        }
    }

    mod scard_get_status_change {
        use super::*;
        use rstest::rstest;

        #[rstest]
        #[case::infinite_timeout_unaware_state(
            TIMEOUT_INFINITE, CardStateFlags::SCARD_STATE_UNAWARE,
            CardStateFlags::SCARD_STATE_PRESENT | CardStateFlags::SCARD_STATE_CHANGED,
            ReturnCode::Success,
        )]
        #[case::infinite_timeout_unpowered_state(
            TIMEOUT_INFINITE, CardStateFlags::SCARD_STATE_UNPOWERED,
            CardStateFlags::SCARD_STATE_PRESENT | CardStateFlags::SCARD_STATE_CHANGED,
            ReturnCode::Success,
        )]
        #[case::immediate_timeout_changed_state(
            TIMEOUT_IMMEDIATE, CardStateFlags::SCARD_STATE_UNPOWERED,
            CardStateFlags::SCARD_STATE_PRESENT | CardStateFlags::SCARD_STATE_CHANGED,
            ReturnCode::Success,
        )]
        #[case::immediate_timeout_stable_state(
            TIMEOUT_IMMEDIATE,
            CardStateFlags::SCARD_STATE_PRESENT,
            CardStateFlags::SCARD_STATE_PRESENT,
            ReturnCode::Timeout
        )]
        #[case::stale_changed_bit_in_current_state(
            TIMEOUT_IMMEDIATE, CardStateFlags::SCARD_STATE_PRESENT | CardStateFlags::SCARD_STATE_CHANGED,
            CardStateFlags::SCARD_STATE_PRESENT,
            ReturnCode::Timeout,
        )]
        fn teleport_reader_event_state_computation(
            #[case] timeout: u32,
            #[case] current_state: CardStateFlags,
            #[case] expected_event_state: CardStateFlags,
            #[case] expected_return_code: ReturnCode,
        ) {
            let (mut scard_backend, _) = build_test_scard_backend();

            let response = scard_backend
                .handle_impl(
                    &build_device_control_request(ScardIoCtlCode::GetStatusChangeW),
                    ScardCall::GetStatusChangeCall(GetStatusChangeCall {
                        context: ScardContext::new(1),
                        timeout,
                        states_ptr_length: 0,
                        states_ptr: 0,
                        states_length: 1,
                        states: vec![ReaderState {
                            reader: TELEPORT_READER_NAME.into(),
                            common: ReaderStateCommonCall {
                                current_state,
                                event_state: CardStateFlags::empty(),
                                atr_length: 0,
                                atr: [0; 36],
                            },
                        }],
                    }),
                )
                .expect("ScardBackend::handle_impl failed");

            let (atr_length, atr) = padded_atr::<36>();
            let expected_response =
                ScardHandleResponse::Common(ScardResponsePayload::GetStatusChange(
                    GetStatusChangeReturn::new(
                        expected_return_code,
                        vec![ReaderStateCommonCall {
                            current_state,
                            event_state: expected_event_state,
                            atr_length,
                            atr,
                        }],
                    )
                    .into_inner(),
                ));

            pretty_assertions::assert_eq!(response, expected_response,);
        }

        #[test]
        fn infinite_timeout_stable_state_blocks_and_cancels_on_ioctl_cancel() {
            let (mut scard_backend, _) = build_test_scard_backend();
            let get_status_change_req =
                build_device_control_request(ScardIoCtlCode::GetStatusChangeW);

            let establish_context_response = scard_backend
                .handle_impl(
                    &build_device_control_request(ScardIoCtlCode::EstablishContext),
                    ScardCall::EstablishContextCall(EstablishContextCall { scope: Scope::User }),
                )
                .expect("EstablishContext failed");

            assert_eq!(
                establish_context_response,
                ScardHandleResponse::Common(ScardResponsePayload::EstablishContext(
                    EstablishContextReturn::new(ReturnCode::Success, ScardContext::new(1))
                        .into_inner()
                ))
            );

            let get_status_change_response = scard_backend
                .handle_impl(
                    &get_status_change_req,
                    ScardCall::GetStatusChangeCall(GetStatusChangeCall {
                        context: ScardContext::new(1),
                        timeout: TIMEOUT_INFINITE,
                        states_ptr_length: 0,
                        states_ptr: 0,
                        states_length: 1,
                        states: vec![ReaderState {
                            reader: TELEPORT_READER_NAME.into(),
                            common: ReaderStateCommonCall {
                                current_state: CardStateFlags::SCARD_STATE_PRESENT,
                                event_state: CardStateFlags::empty(),
                                atr_length: 0,
                                atr: [0; 36],
                            },
                        }],
                    }),
                )
                .expect("ScardBackend::handle_impl failed");

            // There are no changes. So, the function must defer sending a response
            // until the cancel request is received.
            pretty_assertions::assert_eq!(
                get_status_change_response,
                ScardHandleResponse::Deferred
            );

            let response = scard_backend
                .handle_impl(
                    &build_device_control_request(ScardIoCtlCode::Cancel),
                    ScardCall::ContextCall(ContextCall {
                        context: ScardContext::new(1),
                    }),
                )
                .expect("ScardBackend::handle_impl failed");

            let (atr_length, atr) = padded_atr::<36>();
            let expected_response = ScardHandleResponse::CancelDeferred {
                get_status_change_response: Some(PendingGetStatusChange {
                    req: get_status_change_req,
                    response: ScardResponsePayload::GetStatusChange(
                        GetStatusChangeReturn::new(
                            ReturnCode::Cancelled,
                            vec![ReaderStateCommonCall {
                                current_state: CardStateFlags::SCARD_STATE_PRESENT,
                                event_state: CardStateFlags::SCARD_STATE_PRESENT,
                                atr_length,
                                atr,
                            }],
                        )
                        .into_inner(),
                    ),
                }),
                cancel_response: ScardResponsePayload::Long(
                    LongReturn::new(ReturnCode::Success).into_inner(),
                ),
            };

            pretty_assertions::assert_eq!(response, expected_response);
        }

        #[test]
        fn all_readers_ignored_returns_immediately() {
            let (mut scard_backend, _) = build_test_scard_backend();

            let response = scard_backend
                .handle_impl(
                    &build_device_control_request(ScardIoCtlCode::GetStatusChangeW),
                    ScardCall::GetStatusChangeCall(GetStatusChangeCall {
                        context: ScardContext::new(1),
                        timeout: TIMEOUT_INFINITE,
                        states_ptr_length: 0,
                        states_ptr: 0,
                        states_length: 1,
                        states: vec![ReaderState {
                            reader: TELEPORT_READER_NAME.into(),
                            common: ReaderStateCommonCall {
                                current_state: CardStateFlags::SCARD_STATE_IGNORE,
                                event_state: CardStateFlags::empty(),
                                atr_length: 0,
                                atr: [0; 36],
                            },
                        }],
                    }),
                )
                .expect("ScardBackend::handle_impl failed");

            let expected_response =
                ScardHandleResponse::Common(ScardResponsePayload::GetStatusChange(
                    GetStatusChangeReturn::new(
                        ReturnCode::Success,
                        vec![ReaderStateCommonCall {
                            current_state: CardStateFlags::SCARD_STATE_IGNORE,
                            event_state: CardStateFlags::empty(),
                            atr_length: 0,
                            atr: [0; 36],
                        }],
                    )
                    .into_inner(),
                ));

            pretty_assertions::assert_eq!(response, expected_response,);
        }

        #[test]
        fn no_readers_returns_immediately() {
            let (mut scard_backend, _) = build_test_scard_backend();

            let response = scard_backend
                .handle_impl(
                    &build_device_control_request(ScardIoCtlCode::GetStatusChangeW),
                    ScardCall::GetStatusChangeCall(GetStatusChangeCall {
                        context: ScardContext::new(1),
                        timeout: TIMEOUT_INFINITE,
                        states_ptr_length: 0,
                        states_ptr: 0,
                        states_length: 0,
                        states: vec![],
                    }),
                )
                .expect("ScardBackend::handle_impl failed");

            let expected_response =
                ScardHandleResponse::Common(ScardResponsePayload::GetStatusChange(
                    GetStatusChangeReturn::new(ReturnCode::Success, vec![]).into_inner(),
                ));

            pretty_assertions::assert_eq!(response, expected_response,);
        }

        #[test]
        fn pnp_reader_nonempty_current_state_does_not_trigger_state_change() {
            let (mut scard_backend, _) = build_test_scard_backend();

            let response = scard_backend
                .handle_impl(
                    &build_device_control_request(ScardIoCtlCode::GetStatusChangeW),
                    ScardCall::GetStatusChangeCall(GetStatusChangeCall {
                        context: ScardContext::new(1),
                        timeout: TIMEOUT_IMMEDIATE,
                        states_ptr_length: 0,
                        states_ptr: 0,
                        states_length: 1,
                        states: vec![ReaderState {
                            reader: PLUG_AND_PLAY_READER_NAME.into(),
                            common: ReaderStateCommonCall {
                                current_state: CardStateFlags::SCARD_STATE_PRESENT
                                    | CardStateFlags::SCARD_STATE_UNPOWERED,
                                event_state: CardStateFlags::empty(),
                                atr_length: 0,
                                atr: [0; 36],
                            },
                        }],
                    }),
                )
                .expect("ScardBackend::handle_impl failed");

            let expected_response =
                ScardHandleResponse::Common(ScardResponsePayload::GetStatusChange(
                    GetStatusChangeReturn::new(
                        ReturnCode::Timeout,
                        vec![ReaderStateCommonCall {
                            current_state: CardStateFlags::SCARD_STATE_PRESENT
                                | CardStateFlags::SCARD_STATE_UNPOWERED,
                            event_state: CardStateFlags::empty(),
                            atr_length: 0,
                            atr: [0; 36],
                        }],
                    )
                    .into_inner(),
                ));

            pretty_assertions::assert_eq!(response, expected_response,);
        }

        #[test]
        fn unknown_reader_is_flagged_with_unknown_state_and_triggers_change() {
            let (mut scard_backend, _) = build_test_scard_backend();

            let response = scard_backend
                .handle_impl(
                    &build_device_control_request(ScardIoCtlCode::GetStatusChangeW),
                    ScardCall::GetStatusChangeCall(GetStatusChangeCall {
                        context: ScardContext::new(1),
                        timeout: TIMEOUT_IMMEDIATE,
                        states_ptr_length: 0,
                        states_ptr: 0,
                        states_length: 1,
                        states: vec![ReaderState {
                            reader: "Unknown Reader".into(),
                            common: ReaderStateCommonCall {
                                current_state: CardStateFlags::SCARD_STATE_UNAWARE,
                                event_state: CardStateFlags::empty(),
                                atr_length: 0,
                                atr: [0; 36],
                            },
                        }],
                    }),
                )
                .expect("ScardBackend::handle_impl failed");

            let expected_response =
                ScardHandleResponse::Common(ScardResponsePayload::GetStatusChange(
                    GetStatusChangeReturn::new(
                        ReturnCode::Success,
                        vec![ReaderStateCommonCall {
                            current_state: CardStateFlags::SCARD_STATE_UNAWARE,
                            event_state: CardStateFlags::SCARD_STATE_UNKNOWN
                                | CardStateFlags::SCARD_STATE_IGNORE
                                | CardStateFlags::SCARD_STATE_CHANGED,
                            atr_length: 0,
                            atr: [0; 36],
                        }],
                    )
                    .into_inner(),
                ));

            pretty_assertions::assert_eq!(response, expected_response,);
        }

        #[test]
        fn empty_reader_name_returns_invalid_value() {
            let (mut scard_backend, _) = build_test_scard_backend();

            let response = scard_backend
                .handle_impl(
                    &build_device_control_request(ScardIoCtlCode::GetStatusChangeW),
                    ScardCall::GetStatusChangeCall(GetStatusChangeCall {
                        context: ScardContext::new(1),
                        timeout: TIMEOUT_IMMEDIATE,
                        states_ptr_length: 0,
                        states_ptr: 0,
                        states_length: 1,
                        states: vec![ReaderState {
                            reader: "".into(),
                            common: ReaderStateCommonCall {
                                current_state: CardStateFlags::SCARD_STATE_UNAWARE,
                                event_state: CardStateFlags::empty(),
                                atr_length: 0,
                                atr: [0; 36],
                            },
                        }],
                    }),
                )
                .expect("ScardBackend::handle_impl failed");

            let expected_response =
                ScardHandleResponse::Common(ScardResponsePayload::GetStatusChange(
                    GetStatusChangeReturn::new(
                        ReturnCode::InvalidValue,
                        vec![ReaderStateCommonCall {
                            current_state: CardStateFlags::SCARD_STATE_UNAWARE,
                            event_state: CardStateFlags::empty(),
                            atr_length: 0,
                            atr: [0; 36],
                        }],
                    )
                    .into_inner(),
                ));

            pretty_assertions::assert_eq!(response, expected_response,);
        }
    }
}
