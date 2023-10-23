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

mod flags;
pub(crate) mod path;
pub(crate) mod scard;
pub(crate) mod tdp;

use self::scard::{padded_atr, Contexts, TRANSMIT_DATA_LIMIT};
use crate::client::{ClientFunction, ClientHandle};
use crate::rdpdr::scard::TELEPORT_READER_NAME;
use ironrdp_pdu::utils::CharacterSet;
use ironrdp_pdu::{custom_err, other_err, PduResult};
use ironrdp_rdpdr::pdu::esc::{
    rpce, CardProtocol, CardState, CardStateFlags, ConnectCall, ConnectReturn, ContextCall,
    EstablishContextReturn, GetDeviceTypeIdCall, GetDeviceTypeIdReturn, GetReaderIconReturn,
    GetStatusChangeCall, GetStatusChangeReturn, HCardAndDispositionCall, ListReadersReturn,
    ReadCacheCall, ReadCacheReturn, ReaderStateCommonCall, ScardCall, StatusReturn, TransmitCall,
    TransmitReturn, WriteCacheCall,
};
use ironrdp_rdpdr::pdu::RdpdrPdu;
use ironrdp_rdpdr::{
    pdu::{
        efs::{
            DeviceControlRequest, DeviceControlResponse, NtStatus, ServerDeviceAnnounceResponse,
        },
        esc::{LongReturn, ReturnCode, ScardIoCtlCode},
    },
    RdpdrBackend,
};
use iso7816::command::Command as CardCommand;
use std::vec;
use uuid::Uuid;

#[derive(Debug)]
pub struct TeleportRdpdrBackend {
    /// Active device ids for this session.
    ///
    /// The smartcard device id is always active, and always the first element in this vector.
    active_device_ids: Vec<u32>,
    /// The client handle for this backend, used to send messages to the RDP server.
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

impl RdpdrBackend for TeleportRdpdrBackend {
    fn handle_server_device_announce_response(
        &mut self,
        pdu: ServerDeviceAnnounceResponse,
    ) -> PduResult<()> {
        if !self.active_device_ids.contains(&pdu.device_id) {
            return Err(other_err!(
                "TeleportRdpdrBackend::handle_server_device_announce_response",
                "got ServerDeviceAnnounceResponse for unknown device_id",
            ));
        }

        if pdu.result_code != NtStatus::Success {
            return Err(other_err!(
                "TeleportRdpdrBackend::handle_server_device_announce_response",
                "ServerDeviceAnnounceResponse for smartcard redirection failed"
            ));
        }

        // Nothing to send back to the server
        Ok(())
    }

    fn handle_scard_call(
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
            ScardIoCtlCode::ListReadersW => match call {
                ScardCall::ListReadersCall(_) => self.handle_list_readers(req),
                _ => Self::unsupported_combo_error(req.io_control_code, call),
            },
            ScardIoCtlCode::GetStatusChangeW => match call {
                ScardCall::GetStatusChangeCall(call) => self.handle_get_status_change(req, call),
                _ => Self::unsupported_combo_error(req.io_control_code, call),
            },
            ScardIoCtlCode::ConnectW => match call {
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
            ScardIoCtlCode::ReadCacheW => match call {
                ScardCall::ReadCacheCall(call) => self.handle_read_cache(req, call),
                _ => Self::unsupported_combo_error(req.io_control_code, call),
            },
            ScardIoCtlCode::WriteCacheW => match call {
                ScardCall::WriteCacheCall(call) => self.handle_write_cache(req, call),
                _ => Self::unsupported_combo_error(req.io_control_code, call),
            },
            ScardIoCtlCode::GetReaderIcon => match call {
                ScardCall::GetReaderIconCall(_) => self.handle_get_reader_icon(req),
                _ => Self::unsupported_combo_error(req.io_control_code, call),
            },
            _ => Err(custom_err!(
                "TeleportRdpdrBackend::handle_scard_call",
                TeleportRdpdrBackendError(format!(
                    "received unhandled ScardIoCtlCode: {:?}",
                    req.io_control_code
                ))
            )),
        }
    }
}

impl TeleportRdpdrBackend {
    pub fn new(
        smartcard_device_id: u32,
        client_handle: ClientHandle,
        cert_der: Vec<u8>,
        key_der: Vec<u8>,
        pin: String,
    ) -> Self {
        Self {
            active_device_ids: vec![smartcard_device_id],
            client_handle,
            contexts: Contexts::new(),
            uuid: Uuid::new_v4(),
            cert_der,
            key_der,
            pin,
        }
    }

    fn get_scard_device_id(&self) -> PduResult<u32> {
        if self.active_device_ids.is_empty() {
            return Err(custom_err!(
                "TeleportRdpdrBackend::get_scard_device_id",
                TeleportRdpdrBackendError("no active devices".to_string())
            ));
        }
        Ok(self.active_device_ids[0])
    }

    fn handle_access_started_event(
        &mut self,
        req: DeviceControlRequest<ScardIoCtlCode>,
    ) -> PduResult<()> {
        let scard_device_id = self.get_scard_device_id()?;
        if req.header.device_id != scard_device_id {
            return Err(custom_err!(
                "TeleportRdpdrBackend::handle_scard_access_started_event_call",
                TeleportRdpdrBackendError(
                    format!(
                        "got ScardAccessStartedEventCall for unknown device_id [{}], expected [{}]",
                        req.header.device_id, scard_device_id
                    )
                    .to_string()
                ),
            ));
        }

        self.write_rdpdr_response(req, Box::new(LongReturn::new(ReturnCode::Success)))
    }

    fn handle_establish_context(
        &mut self,
        req: DeviceControlRequest<ScardIoCtlCode>,
    ) -> PduResult<()> {
        let ctx = self.contexts.establish();

        self.write_rdpdr_response(
            req,
            Box::new(EstablishContextReturn::new(ReturnCode::Success, ctx)),
        )
    }

    fn handle_list_readers(&mut self, req: DeviceControlRequest<ScardIoCtlCode>) -> PduResult<()> {
        self.write_rdpdr_response(
            req,
            Box::new(ListReadersReturn::new(
                ReturnCode::Success,
                vec![scard::TELEPORT_READER_NAME.to_string()],
            )),
        )
    }

    fn handle_get_status_change(
        &mut self,
        req: DeviceControlRequest<ScardIoCtlCode>,
        call: GetStatusChangeCall,
    ) -> PduResult<()> {
        let timeout = call.timeout;
        let context_id = call.context.value;

        if timeout != scard::TIMEOUT_INFINITE && timeout != scard::TIMEOUT_IMMEDIATE {
            // We've never seen one of these but we log a warning here in case we ever come
            // across one and need to debug a related issue.
            warn!(
                "logic for a non-infinite/non-immediate timeout [{}] is not implemented",
                timeout
            );
        }

        let get_status_change_ret = Self::create_get_status_change_return(call);

        // We have no status change to report, cache a response
        // for later in case we get an SCARD_IOCTL_CANCEL.
        if Self::has_no_change(&get_status_change_ret) {
            if timeout != scard::TIMEOUT_INFINITE {
                return Err(other_err!(
                    "TeleportRdpdrBackend::handle_list_readers",
                    "got no change for non-infinite timeout",
                ));
            }

            // Received a GetStatusChangeCall with an infinite timeout, so we're adding
            // a corresponding DeviceControlResponse holding a GetStatusChangeReturn
            // with its return code set to SCARD_E_CANCELLED to this Context. This value will
            // be returned when we get an SCARD_IOCTL_CANCEL call for this Context.
            self.contexts.set_scard_cancel_response(
                context_id,
                DeviceControlResponse::new(
                    req,
                    NtStatus::Success,
                    Box::new(GetStatusChangeReturn::new(
                        ReturnCode::Cancelled,
                        get_status_change_ret.into_inner().reader_states,
                    )),
                ),
            )?;

            debug!("blocking GetStatusChange call indefinitely (since our status never changes) until we receive an SCARD_IOCTL_CANCEL");

            return Ok(());
        }

        // We have some status change to report, send it to the server.
        self.write_rdpdr_response(req, Box::new(get_status_change_ret))
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

        self.write_rdpdr_response(
            req,
            Box::new(ConnectReturn::new(
                ReturnCode::Success,
                handle,
                CardProtocol::SCARD_PROTOCOL_T1,
            )),
        )
    }

    fn handle_begin_transaction(
        &mut self,
        req: DeviceControlRequest<ScardIoCtlCode>,
    ) -> PduResult<()> {
        self.write_rdpdr_response(req, Box::new(LongReturn::new(ReturnCode::Success)))
    }

    fn handle_transmit(
        &mut self,
        req: DeviceControlRequest<ScardIoCtlCode>,
        call: TransmitCall,
    ) -> PduResult<()> {
        let cmd =
            CardCommand::<TRANSMIT_DATA_LIMIT>::try_from(&call.send_buffer).map_err(|err| {
                custom_err!(
                    "TeleportRdpdrBackend::handle_transmit",
                    TeleportRdpdrBackendError(format!(
                        "failed to parse smartcard command {:?}: {:?}",
                        &call.send_buffer, err
                    ))
                )
            })?;

        let card = self.contexts.get_card(&call.handle)?;
        let resp = card.handle(cmd)?;

        self.write_rdpdr_response(
            req,
            Box::new(TransmitReturn::new(
                ReturnCode::Success,
                None,
                resp.encode(),
            )),
        )
    }

    fn handle_status(&mut self, req: DeviceControlRequest<ScardIoCtlCode>) -> PduResult<()> {
        let enc = match req.io_control_code {
            ScardIoCtlCode::StatusW => CharacterSet::Unicode,
            ScardIoCtlCode::StatusA => CharacterSet::Ansi,
            _ => {
                return Err(custom_err!(
                    "TeleportRdpdrBackend::handle_status",
                    TeleportRdpdrBackendError(format!(
                        "got unexpected ScardIoCtlCode with a StatusCall: {:?}",
                        req.io_control_code
                    ))
                ));
            }
        };

        let (atr_length, atr) = padded_atr::<32>();

        self.write_rdpdr_response(
            req,
            Box::new(StatusReturn::new(
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
            )),
        )
    }

    fn handle_release_context(
        &mut self,
        req: DeviceControlRequest<ScardIoCtlCode>,
        call: ContextCall,
    ) -> PduResult<()> {
        self.contexts.release(call.context.value);
        self.write_rdpdr_response(req, Box::new(LongReturn::new(ReturnCode::Success)))
    }

    fn handle_end_transaction(
        &mut self,
        req: DeviceControlRequest<ScardIoCtlCode>,
    ) -> PduResult<()> {
        self.write_rdpdr_response(req, Box::new(LongReturn::new(ReturnCode::Success)))
    }

    fn handle_disconnect(
        &mut self,
        req: DeviceControlRequest<ScardIoCtlCode>,
        call: HCardAndDispositionCall,
    ) -> PduResult<()> {
        self.contexts.disconnect(call.handle)?;
        self.write_rdpdr_response(req, Box::new(LongReturn::new(ReturnCode::Success)))
    }

    fn handle_cancel(
        &mut self,
        req: DeviceControlRequest<ScardIoCtlCode>,
        call: ContextCall,
    ) -> PduResult<()> {
        let resp = self
            .contexts
            .take_scard_cancel_response(call.context.value)?;
        if let Some(resp) = resp {
            self.send_device_ctl_response(resp)?;
        } else {
            warn!("Received SCARD_IOCTL_CANCEL for a context without a pending SCARD_IOCTL_GETSTATUSCHANGEW");
        }
        self.write_rdpdr_response(req, Box::new(LongReturn::new(ReturnCode::Success)))
    }

    fn handle_is_valid_context(
        &mut self,
        req: DeviceControlRequest<ScardIoCtlCode>,
    ) -> PduResult<()> {
        // TODO: Currently we're always just sending ReturnCode::Success (based on awly's pre-
        // IronRDP code). Should we instead be checking if we have such a context in our cache?
        self.write_rdpdr_response(req, Box::new(LongReturn::new(ReturnCode::Success)))
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
            self.write_rdpdr_response(
                req,
                Box::new(GetDeviceTypeIdReturn::new(
                    ReturnCode::Success,
                    SCARD_READER_TYPE_VENDOR,
                )),
            )
        } else {
            Err(custom_err!(
                "TeleportRdpdrBackend::handle_get_device_type_id",
                TeleportRdpdrBackendError(format!(
                    "got GetDeviceTypeIdCall for unknown context [{}]",
                    call.context.value
                ))
            ))
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
        self.write_rdpdr_response(req, Box::new(ReadCacheReturn::new(return_code, data)))
    }

    fn handle_write_cache(
        &mut self,
        req: DeviceControlRequest<ScardIoCtlCode>,
        call: WriteCacheCall,
    ) -> PduResult<()> {
        self.contexts.write_cache(call)?;
        self.write_rdpdr_response(req, Box::new(LongReturn::new(ReturnCode::Success)))
    }

    fn handle_get_reader_icon(
        &mut self,
        req: DeviceControlRequest<ScardIoCtlCode>,
    ) -> PduResult<()> {
        self.write_rdpdr_response(
            req,
            Box::new(GetReaderIconReturn::new(
                ReturnCode::UnsupportedFeature,
                vec![],
            )),
        )
    }

    fn create_get_status_change_return(
        call: GetStatusChangeCall,
    ) -> rpce::Pdu<GetStatusChangeReturn> {
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
                scard::TELEPORT_READER_NAME => {
                    let (atr_length, atr) = scard::padded_atr::<36>();
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

        GetStatusChangeReturn::new(ReturnCode::Success, reader_states)
    }

    fn has_no_change(pdu: &rpce::Pdu<GetStatusChangeReturn>) -> bool {
        pdu.into_inner_ref()
            .reader_states
            .iter()
            .all(|state| state.current_state == state.event_state)
    }

    fn write_rdpdr_response(
        &mut self,
        req: DeviceControlRequest<ScardIoCtlCode>,
        resp: Box<dyn rpce::Encode>,
    ) -> PduResult<()> {
        let resp = DeviceControlResponse::new(req, NtStatus::Success, resp);
        self.send_device_ctl_response(resp)
    }

    fn send_device_ctl_response(&mut self, resp: DeviceControlResponse) -> PduResult<()> {
        self.client_handle
            .blocking_send(ClientFunction::WriteRdpdr(RdpdrPdu::DeviceControlResponse(
                resp,
            )))
            .map_err(|e| {
                custom_err!(
                    "TeleportRdpdrBackend::write_rdpdr_dev_ctl_resp",
                    // Due to a long chain of trait dependencies in IronRDP that are impractical to unwind at this point,
                    // we can't put _e in the source field of the error because it isn't Sync (because ClientFunction itself
                    // isn't sync). We compromise here by just wrapping its Debug output in a TeleportRdpdrBackendError.
                    TeleportRdpdrBackendError(format!("{:?}", e))
                )
            })
    }

    /// This function returns the error for unsupported combinations of [`ScardIoCtlCode`] and [`ScardCall`].
    fn unsupported_combo_error(ioctl: ScardIoCtlCode, call: ScardCall) -> PduResult<()> {
        Err(custom_err!(
            "TeleportRdpdrBackend::unsupported_combo_error",
            TeleportRdpdrBackendError(format!(
                "received unsupported combination of ScardIoCtlCode [{:?}] with ScardCall [{:?}]",
                ioctl, call
            ))
        ))
    }
}

/// A generic error type for the TeleportRdpdrBackend that can contain any arbitrary error message.
#[derive(Debug)]
pub struct TeleportRdpdrBackendError(pub String);

impl std::fmt::Display for TeleportRdpdrBackendError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{:#?}", self)
    }
}

impl std::error::Error for TeleportRdpdrBackendError {}
