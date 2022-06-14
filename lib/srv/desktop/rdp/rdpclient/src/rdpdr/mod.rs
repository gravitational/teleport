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

mod consts;
mod flags;
mod scard;

use crate::errors::{
    invalid_data_error, not_implemented_error, rejected_by_server_error, try_error, NTSTATUS_OK,
    SPECIAL_NO_RESPONSE,
};
use crate::util;
use crate::vchan;
use crate::{
    Payload, SharedDirectoryAcknowledge, SharedDirectoryInfoRequest, SharedDirectoryInfoResponse,
};

use byteorder::{LittleEndian, ReadBytesExt, WriteBytesExt};
use consts::{
    CapabilityType, Component, DeviceType, FsInformationClassLevel, MajorFunction, MinorFunction,
    PacketId, DRIVE_CAPABILITY_VERSION_02, GENERAL_CAPABILITY_VERSION_02, NTSTATUS,
    SCARD_DEVICE_ID, SMARTCARD_CAPABILITY_VERSION_01, VERSION_MAJOR, VERSION_MINOR,
};
use num_traits::{FromPrimitive, ToPrimitive};
use rdp::core::mcs;
use rdp::core::tpkt;
use rdp::model::data::Message;
use rdp::model::error::Error as RdpError;
use rdp::model::error::*;
use std::collections::HashMap;
use std::convert::{TryFrom, TryInto};
use std::io::{Read, Write};

pub use consts::CHANNEL_NAME;

/// Client implements a device redirection (RDPDR) client, as defined in
/// https://winprotocoldoc.blob.core.windows.net/productionwindowsarchives/MS-RDPEFS/%5bMS-RDPEFS%5d.pdf
///
/// This client only supports a single smartcard device.
pub struct Client {
    vchan: vchan::Client,
    scard: scard::Client,

    allow_directory_sharing: bool,
    active_device_ids: Vec<u32>,

    // Functions for sending tdp messages to the browser client.
    tdp_sd_acknowledge: Box<dyn Fn(SharedDirectoryAcknowledge) -> RdpResult<()>>,
    tdp_sd_info_request: Box<dyn Fn(SharedDirectoryInfoRequest) -> RdpResult<()>>,

    // Completion-id-indexed maps of handlers for tdp messages coming from the browser client.
    pending_sd_info_resp_handlers: HashMap<u32, SharedDirectoryInfoResponseHandler>,
}

impl Client {
    pub fn new(
        cert_der: Vec<u8>,
        key_der: Vec<u8>,
        pin: String,
        allow_directory_sharing: bool,

        tdp_sd_acknowledge: Box<dyn Fn(SharedDirectoryAcknowledge) -> RdpResult<()>>,
        tdp_sd_info_request: Box<dyn Fn(SharedDirectoryInfoRequest) -> RdpResult<()>>,
    ) -> Self {
        if allow_directory_sharing {
            debug!("creating rdpdr client with directory sharing enabled")
        } else {
            debug!("creating rdpdr client with directory sharing disabled")
        }
        Client {
            vchan: vchan::Client::new(),
            scard: scard::Client::new(cert_der, key_der, pin),
            active_device_ids: vec![],
            allow_directory_sharing,

            tdp_sd_acknowledge,
            tdp_sd_info_request,

            pending_sd_info_resp_handlers: HashMap::new(),
        }
    }
    /// Reads raw RDP messages sent on the rdpdr virtual channel and replies as necessary.
    pub fn read_and_reply<S: Read + Write>(
        &mut self,
        payload: tpkt::Payload,
        mcs: &mut mcs::Client<S>,
    ) -> RdpResult<()> {
        if let Some(mut payload) = self.vchan.read(payload)? {
            let header = SharedHeader::decode(&mut payload)?;
            if let Component::RDPDR_CTYP_PRN = header.component {
                warn!("got {:?} RDPDR header from RDP server, ignoring because we're not redirecting any printers", header);
                return Ok(());
            }
            let responses = match header.packet_id {
                PacketId::PAKID_CORE_SERVER_ANNOUNCE => {
                    self.handle_server_announce(&mut payload)?
                }
                PacketId::PAKID_CORE_SERVER_CAPABILITY => {
                    self.handle_server_capability(&mut payload)?
                }
                PacketId::PAKID_CORE_CLIENTID_CONFIRM => {
                    self.handle_client_id_confirm(&mut payload)?
                }
                PacketId::PAKID_CORE_DEVICE_REPLY => self.handle_device_reply(&mut payload)?,
                // Device IO request is where communication with the smartcard and shared drive actually happens.
                // Everything up to this point was negotiation (and smartcard device registration).
                PacketId::PAKID_CORE_DEVICE_IOREQUEST => {
                    self.handle_device_io_request(&mut payload)?
                }
                _ => {
                    // We don't implement the full set of messages.
                    error!(
                        "RDPDR packets {:?} are not implemented yet, ignoring",
                        header.packet_id
                    );
                    vec![]
                }
            };

            let chan = &CHANNEL_NAME.to_string();
            for resp in responses {
                mcs.write(chan, resp)?;
            }
        }
        Ok(())
    }

    fn handle_server_announce(&self, payload: &mut Payload) -> RdpResult<Vec<Vec<u8>>> {
        let req = ServerAnnounceRequest::decode(payload)?;
        debug!("got ServerAnnounceRequest {:?}", req);

        let resp = self.add_headers_and_chunkify(
            PacketId::PAKID_CORE_CLIENTID_CONFIRM,
            ClientAnnounceReply::new(req).encode()?,
        )?;
        debug!("sending client announce reply");
        Ok(resp)
    }

    fn handle_server_capability(&self, payload: &mut Payload) -> RdpResult<Vec<Vec<u8>>> {
        let req = ServerCoreCapabilityRequest::decode(payload)?;
        debug!("got {:?}", req);

        let resp = self.add_headers_and_chunkify(
            PacketId::PAKID_CORE_CLIENT_CAPABILITY,
            ClientCoreCapabilityResponse::new_response(self.allow_directory_sharing).encode()?,
        )?;
        debug!("sending client core capability response");
        Ok(resp)
    }

    fn handle_client_id_confirm(&mut self, payload: &mut Payload) -> RdpResult<Vec<Vec<u8>>> {
        let req = ServerClientIdConfirm::decode(payload)?;
        debug!("got ServerClientIdConfirm {:?}", req);

        // The smartcard initialization sequence that contains this message happens once at session startup,
        // and once when login succeeds. We only need to announce the smartcard once.
        let resp = if !self.active_device_ids.contains(&SCARD_DEVICE_ID) {
            self.push_active_device_id(SCARD_DEVICE_ID)?;
            self.add_headers_and_chunkify(
                PacketId::PAKID_CORE_DEVICELIST_ANNOUNCE,
                ClientDeviceListAnnounceRequest::new_smartcard(SCARD_DEVICE_ID).encode()?,
            )?
        } else {
            self.add_headers_and_chunkify(
                PacketId::PAKID_CORE_DEVICELIST_ANNOUNCE,
                ClientDeviceListAnnounceRequest::new_empty().encode()?,
            )?
        };
        debug!("replying with: {:?}", resp);
        Ok(resp)
    }

    fn handle_device_reply(&self, payload: &mut Payload) -> RdpResult<Vec<Vec<u8>>> {
        let req = ServerDeviceAnnounceResponse::decode(payload)?;
        debug!("got ServerDeviceAnnounceResponse: {:?}", req);

        if self.active_device_ids.contains(&req.device_id) {
            if req.device_id != self.get_scard_device_id()? {
                // This was for a directory we're sharing over TDP
                let mut err_code: u32 = 0;
                if req.result_code != NTSTATUS_OK {
                    err_code = 1;
                    debug!("ServerDeviceAnnounceResponse for smartcard redirection failed with result code NTSTATUS({})", &req.result_code);
                } else {
                    debug!("ServerDeviceAnnounceResponse for shared directory succeeded")
                }

                (self.tdp_sd_acknowledge)(SharedDirectoryAcknowledge {
                    err_code,
                    directory_id: req.device_id,
                })?;
            } else {
                // This was for the smart card
                if req.result_code != NTSTATUS_OK {
                    // End the session, we cannot continue without
                    // the smart card being redirected.
                    return Err(rejected_by_server_error(&format!(
                        "ServerDeviceAnnounceResponse for smartcard redirection failed with result code NTSTATUS({})",
                        &req.result_code
                    )));
                }
                debug!("ServerDeviceAnnounceResponse for smartcard redirection succeeded");
            }
        } else {
            return Err(invalid_data_error(&format!(
                "got ServerDeviceAnnounceResponse for unknown device_id {}",
                &req.device_id
            )));
        }
        Ok(vec![])
    }

    fn handle_device_io_request(&mut self, payload: &mut Payload) -> RdpResult<Vec<Vec<u8>>> {
        let device_io_request = DeviceIoRequest::decode(payload)?;
        let major_function = device_io_request.major_function.clone();

        // Smartcard control only uses IRP_MJ_DEVICE_CONTROL; directory control uses IRP_MJ_DEVICE_CONTROL along with
        // all the other MajorFunctions supported by this Client. Therefore if we receive any major function when drive
        // redirection is not allowed, something has gone wrong. In such a case, we return an error as a security measure
        // to ensure directories are never shared when RBAC doesn't permit it.
        if major_function != MajorFunction::IRP_MJ_DEVICE_CONTROL && !self.allow_directory_sharing {
            return Err(Error::TryError(
                "received a drive redirection major function when drive redirection was not allowed"
                    .to_string(),
            ));
        }

        match major_function {
            MajorFunction::IRP_MJ_DEVICE_CONTROL => {
                let ioctl = DeviceControlRequest::decode(device_io_request, payload)?;
                let is_smart_card_op = ioctl.header.device_id == self.get_scard_device_id()?;
                debug!("got: {:?}", ioctl);

                // IRP_MJ_DEVICE_CONTROL is the one major function used by both the smartcard controller (always enabled)
                // and shared directory controller (potentially disabled by RBAC). Here we check that directory sharing
                // is enabled here before proceeding with any shared directory controls as an additional security measure.
                if !is_smart_card_op && !self.allow_directory_sharing {
                    return Err(Error::TryError("received a drive redirection major function when drive redirection was not allowed".to_string()));
                }
                let resp = if is_smart_card_op {
                    // Smart card control
                    let (code, res) = self.scard.ioctl(ioctl.io_control_code, payload)?;
                    if code == SPECIAL_NO_RESPONSE {
                        return Ok(vec![]);
                    }
                    DeviceControlResponse::new(&ioctl, code, res)
                } else {
                    // Drive redirection, mimic FreeRDP's "no-op"
                    // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_main.c#L677-L684
                    DeviceControlResponse::new(
                        &ioctl,
                        NTSTATUS::STATUS_SUCCESS.to_u32().unwrap(),
                        vec![],
                    )
                };
                debug!("replying with: {:?}", resp);
                let resp = self.add_headers_and_chunkify(
                    PacketId::PAKID_CORE_DEVICE_IOCOMPLETION,
                    resp.encode()?,
                )?;
                debug!("sending device IO response");
                Ok(resp)
            }
            MajorFunction::IRP_MJ_CREATE => {
                let rdp_req = ServerCreateDriveRequest::decode(device_io_request, payload)?;
                debug!("got: {:?}", rdp_req);

                // Send a TDP Shared Directory Info Request
                (self.tdp_sd_info_request)(SharedDirectoryInfoRequest::from(rdp_req.clone()))?;

                // Add a TDP Shared Directory Info Response handler to the handler cache.
                // When we receive a TDP Shared Directory Info Response with this completion_id,
                // this handler will be called.
                self.pending_sd_info_resp_handlers.insert(
                    rdp_req.device_io_request.completion_id,
                    Box::new(
                        |_cli: &mut Self,
                         res: SharedDirectoryInfoResponse|
                         -> RdpResult<Vec<Vec<u8>>> {
                            let _rdp_req = rdp_req;
                            debug!("got {:?}", res);
                            // TODO(isaiah): see https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_file.c#L207

                            Ok(vec![])
                        },
                    ),
                );
                Ok(vec![])
            }
            _ => Err(invalid_data_error(&format!(
                // TODO(isaiah): send back a not implemented response(?)
                "got unsupported major_function in DeviceIoRequest: {:?}",
                &major_function
            ))),
        }
    }

    /// This is called from Go (in effect) to announce a new directory
    /// for sharing.
    pub fn write_client_device_list_announce<S: Read + Write>(
        &mut self,
        req: ClientDeviceListAnnounce,
        mcs: &mut mcs::Client<S>,
    ) -> RdpResult<()> {
        self.push_active_device_id(req.device_list[0].device_id)?;
        debug!("sending new drive for redirection: {:?}", req);

        let responses =
            self.add_headers_and_chunkify(PacketId::PAKID_CORE_DEVICELIST_ANNOUNCE, req.encode()?)?;
        let chan = &CHANNEL_NAME.to_string();
        for resp in responses {
            mcs.write(chan, resp)?;
        }

        Ok(())
    }

    pub fn handle_tdp_sd_info_response<S: Read + Write>(
        &mut self,
        res: SharedDirectoryInfoResponse,
        mcs: &mut mcs::Client<S>,
    ) -> RdpResult<()> {
        if let Some(tdp_resp_handler) = self
            .pending_sd_info_resp_handlers
            .remove(&res.completion_id)
        {
            let rdp_responses = tdp_resp_handler(self, res)?;
            let chan = &CHANNEL_NAME.to_string();
            for resp in rdp_responses {
                mcs.write(chan, resp)?;
            }
            Ok(())
        } else {
            return Err(try_error(&format!(
                "received invalid completion id: {}",
                res.completion_id
            )));
        }
    }

    /// add_headers_and_chunkify takes an encoded PDU ready to be sent over a virtual channel (payload),
    /// adds on the Shared Header based the passed packet_id, adds the appropriate (virtual) Channel PDU Header,
    /// and splits the entire payload into chunks if the payload exceeds the maximum size.
    fn add_headers_and_chunkify(
        &self,
        packet_id: PacketId,
        payload: Vec<u8>,
    ) -> RdpResult<Vec<Vec<u8>>> {
        let mut inner = SharedHeader::new(Component::RDPDR_CTYP_CORE, packet_id).encode()?;
        inner.extend_from_slice(&payload);
        self.vchan.add_header_and_chunkify(None, inner)
    }

    fn push_active_device_id(&mut self, device_id: u32) -> RdpResult<()> {
        if self.active_device_ids.contains(&device_id) {
            return Err(RdpError::TryError(format!(
                "attempted to add a duplicate device_id {} to active_device_ids {:?}",
                device_id, self.active_device_ids
            )));
        }
        self.active_device_ids.push(device_id);
        Ok(())
    }

    fn get_scard_device_id(&self) -> RdpResult<u32> {
        // We always push it into the list first
        if !self.active_device_ids.is_empty() {
            return Ok(self.active_device_ids[0]);
        }
        Err(RdpError::TryError("no active device ids".to_string()))
    }
}

/// 2.2.1.1 Shared Header (RDPDR_HEADER)
/// This header is present at the beginning of every message in sent over the rdpdr virtual channel.
/// The purpose of this header is to describe the type of the message.
/// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpefs/29d4108f-8163-4a67-8271-e48c4b9c2a7c
#[derive(Debug)]
struct SharedHeader {
    component: Component,
    packet_id: PacketId,
}

impl SharedHeader {
    fn new(component: Component, packet_id: PacketId) -> Self {
        Self {
            component,
            packet_id,
        }
    }
    fn decode(payload: &mut Payload) -> RdpResult<Self> {
        let component = payload.read_u16::<LittleEndian>()?;
        let packet_id = payload.read_u16::<LittleEndian>()?;
        Ok(Self {
            component: Component::from_u16(component).ok_or_else(|| {
                invalid_data_error(&format!("invalid component value {:#06x}", component))
            })?,
            packet_id: PacketId::from_u16(packet_id).ok_or_else(|| {
                invalid_data_error(&format!("invalid packet_id value {:#06x}", packet_id))
            })?,
        })
    }
    fn encode(&self) -> RdpResult<Vec<u8>> {
        let mut w = vec![];
        w.write_u16::<LittleEndian>(self.component.to_u16().unwrap())?;
        w.write_u16::<LittleEndian>(self.packet_id.to_u16().unwrap())?;
        Ok(w)
    }
}

type ServerAnnounceRequest = ClientIdMessage;
type ClientAnnounceReply = ClientIdMessage;
type ServerClientIdConfirm = ClientIdMessage;

#[derive(Debug)]
struct ClientIdMessage {
    version_major: u16,
    version_minor: u16,
    client_id: u32,
}

impl ClientIdMessage {
    fn new(req: ServerAnnounceRequest) -> Self {
        Self {
            version_major: VERSION_MAJOR,
            version_minor: VERSION_MINOR,
            client_id: req.client_id,
        }
    }

    fn encode(&self) -> RdpResult<Vec<u8>> {
        let mut w = vec![];
        w.write_u16::<LittleEndian>(self.version_major)?;
        w.write_u16::<LittleEndian>(self.version_minor)?;
        w.write_u32::<LittleEndian>(self.client_id)?;
        Ok(w)
    }

    fn decode(payload: &mut Payload) -> RdpResult<Self> {
        Ok(Self {
            version_major: payload.read_u16::<LittleEndian>()?,
            version_minor: payload.read_u16::<LittleEndian>()?,
            client_id: payload.read_u32::<LittleEndian>()?,
        })
    }
}

#[derive(Debug)]
struct ServerCoreCapabilityRequest {
    num_capabilities: u16,
    padding: u16,
    capabilities: Vec<CapabilitySet>,
}

impl ServerCoreCapabilityRequest {
    fn new_response(allow_directory_sharing: bool) -> Self {
        // Clients are always required to send the "general" capability set.
        // In addition, we also send the optional smartcard capability (CAP_SMARTCARD_TYPE)
        // and drive capability (CAP_DRIVE_TYPE).
        let mut capabilities = vec![
            CapabilitySet {
                header: CapabilityHeader {
                    cap_type: CapabilityType::CAP_GENERAL_TYPE,
                    length: 8 + 36, // 8 byte header + 36 byte capability descriptor
                    version: GENERAL_CAPABILITY_VERSION_02,
                },
                data: Capability::General(GeneralCapabilitySet {
                    os_type: 0,
                    os_version: 0,
                    protocol_major_version: VERSION_MAJOR,
                    protocol_minor_version: VERSION_MINOR,
                    io_code_1: 0x00007fff, // Combination of all the required bits.
                    io_code_2: 0,
                    extended_pdu: 0x00000001 | 0x00000002, // RDPDR_DEVICE_REMOVE_PDUS | RDPDR_CLIENT_DISPLAY_NAME_PDU
                    extra_flags_1: 0,
                    extra_flags_2: 0,
                    special_type_device_cap: 1, // Request redirection of 1 special device - smartcard.
                }),
            },
            CapabilitySet {
                header: CapabilityHeader {
                    cap_type: CapabilityType::CAP_SMARTCARD_TYPE,
                    length: 8, // 8 byte header + empty capability descriptor
                    version: SMARTCARD_CAPABILITY_VERSION_01,
                },
                data: Capability::Smartcard,
            },
        ];

        if allow_directory_sharing {
            capabilities.push(CapabilitySet {
                header: CapabilityHeader {
                    cap_type: CapabilityType::CAP_DRIVE_TYPE,
                    length: 8, // 8 byte header + empty capability descriptor
                    version: DRIVE_CAPABILITY_VERSION_02,
                },
                data: Capability::Drive,
            });
        }

        Self {
            padding: 0,
            num_capabilities: capabilities.len() as u16,
            capabilities,
        }
    }

    fn encode(&self) -> RdpResult<Vec<u8>> {
        let mut w = vec![];
        w.write_u16::<LittleEndian>(self.num_capabilities)?;
        w.write_u16::<LittleEndian>(self.padding)?;
        for cap in self.capabilities.iter() {
            w.extend_from_slice(&cap.encode()?);
        }
        Ok(w)
    }

    fn decode(payload: &mut Payload) -> RdpResult<Self> {
        let num_capabilities = payload.read_u16::<LittleEndian>()?;
        let padding = payload.read_u16::<LittleEndian>()?;
        let mut capabilities = vec![];
        for _ in 0..num_capabilities {
            capabilities.push(CapabilitySet::decode(payload)?);
        }

        Ok(Self {
            num_capabilities,
            padding,
            capabilities,
        })
    }
}

#[derive(Debug)]
struct CapabilitySet {
    header: CapabilityHeader,
    data: Capability,
}

impl CapabilitySet {
    fn encode(&self) -> RdpResult<Vec<u8>> {
        let mut w = self.header.encode()?;
        w.extend_from_slice(&self.data.encode()?);
        Ok(w)
    }
    fn decode(payload: &mut Payload) -> RdpResult<Self> {
        let header = CapabilityHeader::decode(payload)?;
        let data = Capability::decode(payload, &header)?;

        Ok(Self { header, data })
    }
}

#[derive(Debug)]
struct CapabilityHeader {
    cap_type: CapabilityType,
    length: u16,
    version: u32,
}

impl CapabilityHeader {
    fn encode(&self) -> RdpResult<Vec<u8>> {
        let mut w = vec![];
        w.write_u16::<LittleEndian>(self.cap_type.to_u16().unwrap())?;
        w.write_u16::<LittleEndian>(self.length)?;
        w.write_u32::<LittleEndian>(self.version)?;
        Ok(w)
    }
    fn decode(payload: &mut Payload) -> RdpResult<Self> {
        let cap_type = payload.read_u16::<LittleEndian>()?;
        Ok(Self {
            cap_type: CapabilityType::from_u16(cap_type).ok_or_else(|| {
                invalid_data_error(&format!("invalid capability type {:#06x}", cap_type))
            })?,
            length: payload.read_u16::<LittleEndian>()?,
            version: payload.read_u32::<LittleEndian>()?,
        })
    }
}

#[derive(Debug)]
enum Capability {
    General(GeneralCapabilitySet),
    Printer,
    Port,
    Drive,
    Smartcard,
}

impl Capability {
    fn encode(&self) -> RdpResult<Vec<u8>> {
        match self {
            Capability::General(general) => Ok(general.encode()?),
            _ => Ok(vec![]),
        }
    }

    fn decode(payload: &mut Payload, header: &CapabilityHeader) -> RdpResult<Self> {
        match header.cap_type {
            CapabilityType::CAP_GENERAL_TYPE => Ok(Capability::General(
                GeneralCapabilitySet::decode(payload, header.version)?,
            )),
            CapabilityType::CAP_PRINTER_TYPE => Ok(Capability::Printer),
            CapabilityType::CAP_PORT_TYPE => Ok(Capability::Port),
            CapabilityType::CAP_DRIVE_TYPE => Ok(Capability::Drive),
            CapabilityType::CAP_SMARTCARD_TYPE => Ok(Capability::Smartcard),
        }
    }
}

#[derive(Debug)]
struct GeneralCapabilitySet {
    os_type: u32,
    os_version: u32,
    protocol_major_version: u16,
    protocol_minor_version: u16,
    io_code_1: u32,
    io_code_2: u32,
    extended_pdu: u32,
    extra_flags_1: u32,
    extra_flags_2: u32,
    special_type_device_cap: u32,
}

impl GeneralCapabilitySet {
    fn encode(&self) -> RdpResult<Vec<u8>> {
        let mut w = vec![];
        w.write_u32::<LittleEndian>(self.os_type)?;
        w.write_u32::<LittleEndian>(self.os_version)?;
        w.write_u16::<LittleEndian>(self.protocol_major_version)?;
        w.write_u16::<LittleEndian>(self.protocol_minor_version)?;
        w.write_u32::<LittleEndian>(self.io_code_1)?;
        w.write_u32::<LittleEndian>(self.io_code_2)?;
        w.write_u32::<LittleEndian>(self.extended_pdu)?;
        w.write_u32::<LittleEndian>(self.extra_flags_1)?;
        w.write_u32::<LittleEndian>(self.extra_flags_2)?;
        w.write_u32::<LittleEndian>(self.special_type_device_cap)?;
        Ok(w)
    }

    fn decode(payload: &mut Payload, version: u32) -> RdpResult<Self> {
        Ok(Self {
            os_type: payload.read_u32::<LittleEndian>()?,
            os_version: payload.read_u32::<LittleEndian>()?,
            protocol_major_version: payload.read_u16::<LittleEndian>()?,
            protocol_minor_version: payload.read_u16::<LittleEndian>()?,
            io_code_1: payload.read_u32::<LittleEndian>()?,
            io_code_2: payload.read_u32::<LittleEndian>()?,
            extended_pdu: payload.read_u32::<LittleEndian>()?,
            extra_flags_1: payload.read_u32::<LittleEndian>()?,
            extra_flags_2: payload.read_u32::<LittleEndian>()?,
            special_type_device_cap: if version == GENERAL_CAPABILITY_VERSION_02 {
                payload.read_u32::<LittleEndian>()?
            } else {
                0
            },
        })
    }
}

type ClientCoreCapabilityResponse = ServerCoreCapabilityRequest;

#[derive(Debug)]
pub struct ClientDeviceListAnnounceRequest {
    device_count: u32,
    device_list: Vec<DeviceAnnounceHeader>,
}

pub type ClientDeviceListAnnounce = ClientDeviceListAnnounceRequest;

impl ClientDeviceListAnnounceRequest {
    // We only need to announce the smartcard in this Client Device List Announce Request.
    // Drives (directories) can be announced at any time with a Client Drive Device List Announce.
    fn new_smartcard(device_id: u32) -> Self {
        Self {
            device_count: 1,
            device_list: vec![DeviceAnnounceHeader {
                device_type: DeviceType::RDPDR_DTYP_SMARTCARD,
                device_id,
                // This name is a constant defined by the spec.
                preferred_dos_name: "SCARD".to_string(),
                device_data_length: 0,
                device_data: vec![],
            }],
        }
    }

    /// Creates a ClientDeviceListAnnounceRequest for announcing a new shared drive (directory).
    /// A new drive can be announced at any time during RDP's operation. It is up to the caller
    /// to ensure that the passed device_id is unique from that of any previously shared devices.
    pub fn new_drive(device_id: u32, drive_name: String) -> Self {
        // According to the spec:
        //
        // If the client supports DRIVE_CAPABILITY_VERSION_02 in the Drive Capability Set,
        // then the full name MUST also be specified in the DeviceData field, as a null-terminated
        // Unicode string. If the DeviceDataLength field is nonzero, the content of the
        // PreferredDosName field is ignored.
        //
        // In the RDP spec, Unicode typically means null-terminated UTF-16LE, however empirically it
        // appears that this field expects null-terminated UTF-8.
        let device_data = util::to_utf8(&drive_name);

        Self {
            device_count: 1,
            device_list: vec![DeviceAnnounceHeader {
                device_type: DeviceType::RDPDR_DTYP_FILESYSTEM,
                device_id,
                preferred_dos_name: drive_name,
                device_data_length: device_data.len() as u32,
                device_data,
            }],
        }
    }

    fn new_empty() -> Self {
        Self {
            device_count: 0,
            device_list: vec![],
        }
    }

    fn encode(&self) -> RdpResult<Vec<u8>> {
        let mut w = vec![];
        w.write_u32::<LittleEndian>(self.device_count)?;
        for dev in self.device_list.iter() {
            w.extend_from_slice(&dev.encode()?);
        }
        Ok(w)
    }
}

/// 2.2.1.3 Device Announce Header (DEVICE_ANNOUNCE)
/// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpefs/32e34332-774b-4ead-8c9d-5d64720d6bf9
#[derive(Debug)]
struct DeviceAnnounceHeader {
    device_type: DeviceType,
    device_id: u32,
    preferred_dos_name: String,
    device_data_length: u32,
    device_data: Vec<u8>,
}

impl DeviceAnnounceHeader {
    fn encode(&self) -> RdpResult<Vec<u8>> {
        let mut w = vec![];
        w.write_u32::<LittleEndian>(self.device_type.to_u32().unwrap())?;
        w.write_u32::<LittleEndian>(self.device_id)?;
        let mut name: &str = &self.preferred_dos_name;
        // See "PreferredDosName" at
        // https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpefs/32e34332-774b-4ead-8c9d-5d64720d6bf9
        if name.len() > 7 {
            name = &name[..7];
        }
        w.extend_from_slice(&format!("{:\x00<8}", name).into_bytes());
        w.write_u32::<LittleEndian>(self.device_data_length)?;
        w.extend_from_slice(&self.device_data);
        Ok(w)
    }
}

#[derive(Debug)]
struct ServerDeviceAnnounceResponse {
    device_id: u32,
    result_code: u32,
}

impl ServerDeviceAnnounceResponse {
    fn decode(payload: &mut Payload) -> RdpResult<Self> {
        Ok(Self {
            device_id: payload.read_u32::<LittleEndian>()?,
            result_code: payload.read_u32::<LittleEndian>()?,
        })
    }
}

/// 2.2.1.4 Device I/O Request (DR_DEVICE_IOREQUEST)
/// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpefs/a087ffa8-d0d5-4874-ac7b-0494f63e2d5d
#[derive(Debug, Clone)]
#[allow(dead_code)]
pub struct DeviceIoRequest {
    pub device_id: u32,
    file_id: u32,
    pub completion_id: u32,
    major_function: MajorFunction,
    minor_function: MinorFunction,
}

impl DeviceIoRequest {
    fn decode(payload: &mut Payload) -> RdpResult<Self> {
        let device_id = payload.read_u32::<LittleEndian>()?;
        let file_id = payload.read_u32::<LittleEndian>()?;
        let completion_id = payload.read_u32::<LittleEndian>()?;
        let major_function = payload.read_u32::<LittleEndian>()?;
        let major_function = MajorFunction::from_u32(major_function).ok_or_else(|| {
            invalid_data_error(&format!(
                "invalid major function value {:#010x}",
                major_function
            ))
        })?;
        let minor_function = payload.read_u32::<LittleEndian>()?;
        // From the spec (2.2.1.4 Device I/O Request (DR_DEVICE_IOREQUEST)):
        // "This field [MinorFunction] is valid only when the MajorFunction field
        // is set to IRP_MJ_DIRECTORY_CONTROL. If the MajorFunction field is set
        // to another value, the MinorFunction field value SHOULD be 0x00000000.""
        //
        // SHOULD means implementations are not guaranteed to give us 0x00000000,
        // so handle that possibility here.
        let minor_function = if major_function == MajorFunction::IRP_MJ_DIRECTORY_CONTROL {
            minor_function
        } else {
            0x00000000
        };
        let minor_function = MinorFunction::from_u32(minor_function).ok_or_else(|| {
            invalid_data_error(&format!(
                "invalid minor function value {:#010x}",
                minor_function
            ))
        })?;

        Ok(Self {
            device_id,
            file_id,
            completion_id,
            major_function,
            minor_function,
        })
    }
}

/// 2.2.1.4.5 Device Control Request (DR_CONTROL_REQ)
/// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpefs/30662c80-ec6e-4ed1-9004-2e6e367bb59f
#[derive(Debug)]
#[allow(dead_code)]
struct DeviceControlRequest {
    header: DeviceIoRequest,
    output_buffer_length: u32,
    input_buffer_length: u32,
    io_control_code: u32,
    padding: [u8; 20],
}

impl DeviceControlRequest {
    fn decode(header: DeviceIoRequest, payload: &mut Payload) -> RdpResult<Self> {
        let output_buffer_length = payload.read_u32::<LittleEndian>()?;
        let input_buffer_length = payload.read_u32::<LittleEndian>()?;
        let io_control_code = payload.read_u32::<LittleEndian>()?;
        let mut padding: [u8; 20] = [0; 20];
        payload.read_exact(&mut padding)?;
        Ok(Self {
            header,
            output_buffer_length,
            input_buffer_length,
            io_control_code,
            padding,
        })
    }
}

/// 2.2.1.5 Device I/O Response (DR_DEVICE_IOCOMPLETION)
/// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpefs/1c412a84-0776-4984-b35c-3f0445fcae65
#[derive(Debug)]
struct DeviceIoResponse {
    device_id: u32,
    completion_id: u32,
    io_status: u32,
}

impl DeviceIoResponse {
    fn new(req: &DeviceIoRequest, io_status: u32) -> Self {
        Self {
            device_id: req.device_id,
            completion_id: req.completion_id,
            io_status,
        }
    }

    fn encode(&self) -> RdpResult<Vec<u8>> {
        let mut w = vec![];
        w.write_u32::<LittleEndian>(self.device_id)?;
        w.write_u32::<LittleEndian>(self.completion_id)?;
        w.write_u32::<LittleEndian>(self.io_status)?;
        Ok(w)
    }
}

#[derive(Debug)]
struct DeviceControlResponse {
    header: DeviceIoResponse,
    output_buffer_length: u32,
    output_buffer: Vec<u8>,
}

impl DeviceControlResponse {
    fn new(req: &DeviceControlRequest, io_status: u32, output: Vec<u8>) -> Self {
        Self {
            header: DeviceIoResponse::new(&req.header, io_status),
            output_buffer_length: output.length() as u32,
            output_buffer: output,
        }
    }

    fn encode(&self) -> RdpResult<Vec<u8>> {
        let mut w = vec![];
        w.extend_from_slice(&self.header.encode()?);
        w.write_u32::<LittleEndian>(self.output_buffer_length)?;
        w.extend_from_slice(&self.output_buffer);
        Ok(w)
    }
}

/// 2.2.3.3.1 Server Create Drive Request (DR_DRIVE_CREATE_REQ)
/// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpefs/95b16fd0-d530-407c-a310-adedc85e9897
pub type ServerCreateDriveRequest = DeviceCreateRequest;

/// 2.2.1.4.1 Device Create Request (DR_CREATE_REQ)
/// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpefs/5f71f6d2-d9ff-40c2-bdb5-a739447d3c3e
#[derive(Debug, Clone)]
#[allow(dead_code)]
pub struct DeviceCreateRequest {
    /// The MajorFunction field in this header MUST be set to IRP_MJ_CREATE.
    pub device_io_request: DeviceIoRequest,
    desired_access: flags::DesiredAccess,
    allocation_size: u64,
    file_attributes: flags::FileAttributes,
    shared_access: flags::SharedAccess,
    create_disposition: flags::CreateDisposition,
    create_options: flags::CreateOptions,
    path_length: u32,
    pub path: String,
}

#[allow(dead_code)]
impl DeviceCreateRequest {
    fn decode(device_io_request: DeviceIoRequest, payload: &mut Payload) -> RdpResult<Self> {
        let invalid_flags = || invalid_data_error("invalid flags in Device Create Request");

        let desired_access = flags::DesiredAccess::from_bits(payload.read_u32::<LittleEndian>()?)
            .ok_or_else(invalid_flags)?;
        let allocation_size = payload.read_u64::<LittleEndian>()?;
        let file_attributes = flags::FileAttributes::from_bits(payload.read_u32::<LittleEndian>()?)
            .ok_or_else(invalid_flags)?;
        let shared_access = flags::SharedAccess::from_bits(payload.read_u32::<LittleEndian>()?)
            .ok_or_else(invalid_flags)?;
        let create_disposition =
            flags::CreateDisposition::from_bits(payload.read_u32::<LittleEndian>()?)
                .ok_or_else(invalid_flags)?;
        let create_options = flags::CreateOptions::from_bits(payload.read_u32::<LittleEndian>()?)
            .ok_or_else(invalid_flags)?;
        let path_length = payload.read_u32::<LittleEndian>()?;

        // usize is 32 bits on a 32 bit target and 64 on a 64, so we can safely say try_into().unwrap()
        // for a u32 will never panic on the machines that run teleport.
        let mut path = vec![0u8; path_length.try_into().unwrap()];
        payload.read_exact(&mut path)?;
        let path = util::from_unicode(path)?;

        Ok(Self {
            device_io_request,
            desired_access,
            allocation_size,
            file_attributes,
            shared_access,
            create_disposition,
            create_options,
            path_length,
            path,
        })
    }
}

/// 2.2.1.5.1 Device Create Response (DR_CREATE_RSP)
/// A message with this header describes a response to a Device Create Request (section 2.2.1.4.1).
/// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpefs/99e5fca5-b37a-41e4-bc69-8d7da7860f76
#[derive(Debug)]
#[allow(dead_code)]
struct DeviceCreateResponse {
    device_io_reply: DeviceIoResponse,
    file_id: u32,
    /// The values of the CreateDisposition field in the Device Create Request (section 2.2.1.4.1) that determine the value
    /// of the Information field are associated as follows:
    /// +---------------------+--------------------+
    /// | CreateDisposition   |   Information      |
    /// +---------------------+--------------------+
    /// | FILE_SUPERSEDE      |   FILE_SUPERSEDED  |
    /// | FILE_OPEN           |                    |
    /// | FILE_CREATE         |                    |
    /// | FILE_OVERWRITE      |                    |
    /// +---------------------+--------------------+
    /// | FILE_OPEN_IF        |   FILE_OPENED      |
    /// +---------------------+--------------------+
    /// | FILE_OVERWRITE_IF   |   FILE_OVERWRITTEN |
    /// +---------------------+--------------------+
    information: flags::Information,
}

#[allow(dead_code)]
impl DeviceCreateResponse {
    fn new(device_create_request: &DeviceCreateRequest, io_status: NTSTATUS) -> Self {
        let device_io_request = &device_create_request.device_io_request;

        let information: flags::Information;
        if device_create_request.create_disposition.intersects(
            flags::CreateDisposition::FILE_SUPERSEDE
                | flags::CreateDisposition::FILE_OPEN
                | flags::CreateDisposition::FILE_CREATE
                | flags::CreateDisposition::FILE_OVERWRITE,
        ) {
            information = flags::Information::FILE_SUPERSEDED;
        } else if device_create_request.create_disposition == flags::CreateDisposition::FILE_OPEN_IF
        {
            information = flags::Information::FILE_OPENED;
        } else if device_create_request.create_disposition
            == flags::CreateDisposition::FILE_OVERWRITE_IF
        {
            information = flags::Information::FILE_OVERWRITTEN;
        } else {
            panic!("program error, CreateDispositionFlags check should be exhaustive");
        }

        Self {
            device_io_reply: DeviceIoResponse::new(
                device_io_request,
                NTSTATUS::to_u32(&io_status).unwrap(),
            ),
            file_id: device_io_request.file_id, // TODO(isaiah): this is false, the client should be generating the file_id here
            information,
        }
    }

    fn encode(&self) -> RdpResult<Vec<u8>> {
        let mut w = vec![];
        w.extend_from_slice(&self.device_io_reply.encode()?);
        w.write_u32::<LittleEndian>(self.file_id)?;
        w.write_u8(self.information.bits())?;
        Ok(w)
    }
}

/// 2.2.3.3.8 Server Drive Query Information Request (DR_DRIVE_QUERY_INFORMATION_REQ)
/// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpefs/e43dcd68-2980-40a9-9238-344b6cf94946
#[derive(Debug)]
#[allow(dead_code)]
struct ServerDriveQueryInformationRequest {
    /// A DR_DEVICE_IOREQUEST (section 2.2.1.4) header. The MajorFunction field in the DR_DEVICE_IOREQUEST header MUST be set to IRP_MJ_QUERY_INFORMATION.
    device_io_request: DeviceIoRequest,
    /// A 32-bit unsigned integer.
    /// This field MUST contain one of the following values:
    /// FileBasicInformation
    /// This information class is used to query a file for the times of creation, last access, last write, and change, in addition to file attribute information. The Reserved field of the FileBasicInformation structure ([MS-FSCC] section 2.4.7) MUST NOT be present.
    ///
    /// FileStandardInformation
    /// This information class is used to query for file information such as allocation size, end-of-file position, and number of links. The Reserved field of the FileStandardInformation structure ([MS-FSCC] section 2.4.41) MUST NOT be present.
    ///
    /// FileAttributeTagInformation
    /// This information class is used to query for file attribute and reparse tag information.
    fs_information_class_lvl: FsInformationClassLevel,
    // Length, Padding, and QueryBuffer appear to be vestigial fields and can safely be ignored. Their description
    // is provided below for documentation purposes.
    //
    // Length (4 bytes): A 32-bit unsigned integer that specifies the number of bytes in the QueryBuffer field.
    //
    // Padding (24 bytes): An array of 24 bytes. This field is unused and MUST be ignored.
    //
    // QueryBuffer (variable): A variable-length array of bytes. The size of the array is specified by the Length field.
    // The content of this field is based on the value of the FsInformationClass field, which determines the different
    // structures that MUST be contained in the QueryBuffer field. For a complete list of these structures, see [MS-FSCC]
    // section 2.4. The "File information class" table defines all the possible values for the FsInformationClass field.
}

#[allow(dead_code)]
impl ServerDriveQueryInformationRequest {
    fn decode(device_io_request: DeviceIoRequest, payload: &mut Payload) -> RdpResult<Self> {
        if let Some(fs_information_class_lvl) =
            FsInformationClassLevel::from_u32(payload.read_u32::<LittleEndian>()?)
        {
            Ok(Self {
                device_io_request,
                fs_information_class_lvl,
            })
        } else {
            Err(invalid_data_error(
                "received invalid FsInformationClass in ServerDriveQueryInformationRequest",
            ))
        }
    }
}

/// 2.4 File Information Classes [MS-FSCC]
/// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-fscc/4718fc40-e539-4014-8e33-b675af74e3e1
#[derive(Debug)]
#[allow(dead_code, clippy::enum_variant_names)]
enum FsInformationClass {
    FileBasicInformation(FileBasicInformation),
    FileStandardInformation(FileStandardInformation),
    FileBothDirectoryInformation(FileBothDirectoryInformation),
}

#[allow(dead_code)]
impl FsInformationClass {
    fn encode(&self) -> RdpResult<Vec<u8>> {
        match self {
            Self::FileBasicInformation(file_basic_info) => file_basic_info.encode(),
            Self::FileStandardInformation(file_standard_info) => file_standard_info.encode(),
            Self::FileBothDirectoryInformation(fil_both_dir_info) => fil_both_dir_info.encode(), // TODO(isaiah)
        }
    }
}

/// 2.4.7 FileBasicInformation [MS-FSCC]
/// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-fscc/16023025-8a78-492f-8b96-c873b042ac50
#[derive(Debug)]
struct FileBasicInformation {
    creation_time: i64,
    last_access_time: i64,
    last_write_time: i64,
    change_time: i64,
    file_attributes: flags::FileAttributes,
    // NOTE: The `reserved` field in the spec MUST not be serialized and sent over RDP, or it will break the server implementation.
    // FreeRDP does the same: https://github.com/FreeRDP/FreeRDP/blob/1adb263813ca2e76a893ef729a04db8f94b5d757/channels/drive/client/drive_file.c#L508
    //reserved: u32,
}

#[allow(dead_code)]
/// 4 i64's and 1 u32's = (4 * 8) + 4
const FILE_BASIC_INFORMATION_SIZE: u32 = (4 * 8) + 4;

impl FileBasicInformation {
    fn encode(&self) -> RdpResult<Vec<u8>> {
        let mut w = vec![];
        w.write_i64::<LittleEndian>(self.creation_time)?;
        w.write_i64::<LittleEndian>(self.last_access_time)?;
        w.write_i64::<LittleEndian>(self.last_write_time)?;
        w.write_i64::<LittleEndian>(self.change_time)?;
        w.write_u32::<LittleEndian>(self.file_attributes.bits())?;
        Ok(w)
    }
}

/// 2.4.41 FileStandardInformation [MS-FSCC]
/// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-fscc/5afa7f66-619c-48f3-955f-68c4ece704ae
#[derive(Debug)]
struct FileStandardInformation {
    /// A 64-bit signed integer that contains the file allocation size, in bytes. The value of this field MUST be an
    /// integer multiple of the cluster size.
    /// Cluster size is the size of the logical minimal unit of disk space used by the operating system. FreeRDP
    /// doesn't give the actual size here, but rather just gives the file size itself, which we will mimic.
    /// (ttps://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_file.c#L518-L519).
    ///
    /// When FileStandardInformation is requested for a directory, its not entirely clear what "file size" means.
    /// FreeRDP derives this value from the st_size field of a stat struct (https://linux.die.net/man/2/lstat), which says
    /// "The st_size field gives the size of the file (if it is a regular file or a symbolic link) in bytes. The size of
    /// a symbolic link is the length of the pathname it contains, without a terminating null byte." Since it's not
    /// entirely clear what is offered here in the case of a directory, we will just use 0.
    allocation_size: i64,
    /// A 64-bit signed integer that contains the absolute end-of-file position as a byte offset from the start of the
    /// file. EndOfFile specifies the offset to the byte immediately following the last valid byte in the file. Because
    /// this value is zero-based, it actually refers to the first free byte in the file. That is, it is the offset from
    /// the beginning of the file at which new bytes appended to the file will be written. The value of this field MUST
    /// be greater than or equal to 0.
    end_of_file: i64,
    /// A 32-bit unsigned integer that contains the number of non-deleted [hard] links to this file.
    /// NOTE: this information is not available to us in the browser, so we will simply set this field to 0.
    number_of_links: u32,
    /// Set to TRUE to indicate that a file deletion has been requested; set to FALSE
    /// otherwise.
    delete_pending: Boolean,
    /// Set to TRUE to indicate that the file is a directory; set to FALSE otherwise.
    directory: Boolean,
    // NOTE: `reserved` field omitted, see NOTE in FileBasicInformation struct.
    // reserved: u16,
}

impl FileStandardInformation {
    fn encode(&self) -> RdpResult<Vec<u8>> {
        let mut w = vec![];
        w.write_i64::<LittleEndian>(self.allocation_size)?;
        w.write_i64::<LittleEndian>(self.end_of_file)?;
        w.write_u32::<LittleEndian>(self.number_of_links)?;
        w.write_u8(Boolean::to_u8(&self.delete_pending).unwrap())?;
        w.write_u8(Boolean::to_u8(&self.directory).unwrap())?;
        Ok(w)
    }
}

#[allow(dead_code)]
// 2 i64's + 1 u32 + 2 Boolean (u8) = (2 * 8) + 4 + 2
const FILE_STANDARD_INFORMATION_SIZE: u32 = (2 * 8) + 4 + 2;

/// 2.1.8 Boolean
/// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-fscc/8ce7b38c-d3cc-415d-ab39-944000ea77ff
#[derive(Debug, ToPrimitive)]
#[repr(u8)]
#[allow(dead_code)]
enum Boolean {
    True = 1,
    False = 0,
}

/// 2.4.8 FileBothDirectoryInformation
/// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-fscc/270df317-9ba5-4ccb-ba00-8d22be139bc5
/// Fields are omitted based on those omitted by FreeRDP: https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_file.c#L871
#[derive(Debug)]
struct FileBothDirectoryInformation {
    // next_entry_offset: u32,
    // file_index: u32,
    creation_time: i64,
    last_access_time: i64,
    last_write_time: i64,
    change_time: i64,
    end_of_file: i64,
    allocation_size: i64,
    file_attributes: flags::FileAttributes,
    file_name_length: u32,
    // ea_size: u32,
    // short_name_length: i8,
    // reserved: u8: MUST NOT be added,
    // see https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_file.c#L907
    // short_name: String, // 24 bytes
    file_name: String,
}

#[allow(dead_code)]
/// Base size of the FileBothDirectoryInformation, not accounting for variably sized file_name.
/// Note that file_name's size should be calculated as if it were a Unicode string.
/// 5 u32's (including FileAttributesFlags) + 6 i64's + 1 i8 + 24 bytes
const FILE_BOTH_DIRECTORY_INFORMATION_BASE_SIZE: u32 = (5 * 4) + (6 * 8) + 1 + 24; // 93

#[allow(dead_code)]
impl FileBothDirectoryInformation {
    fn new(
        creation_time: i64,
        last_access_time: i64,
        last_write_time: i64,
        change_time: i64,
        file_size: i64,
        file_attributes: flags::FileAttributes,
        file_name: String,
    ) -> Self {
        Self {
            creation_time,
            last_access_time,
            last_write_time,
            change_time,
            end_of_file: file_size,
            allocation_size: file_size,
            file_attributes,
            file_name_length: u32::try_from(util::to_unicode(&file_name, false).len()).unwrap(),
            file_name,
        }
    }

    fn encode(&self) -> RdpResult<Vec<u8>> {
        let mut w = vec![];
        // next_entry_offset
        w.write_u32::<LittleEndian>(0)?;
        // file_index
        w.write_u32::<LittleEndian>(0)?;
        w.write_i64::<LittleEndian>(self.creation_time)?;
        w.write_i64::<LittleEndian>(self.last_access_time)?;
        w.write_i64::<LittleEndian>(self.last_write_time)?;
        w.write_i64::<LittleEndian>(self.change_time)?;
        w.write_i64::<LittleEndian>(self.end_of_file)?;
        w.write_i64::<LittleEndian>(self.allocation_size)?;
        w.write_u32::<LittleEndian>(self.file_attributes.bits())?;
        w.write_u32::<LittleEndian>(self.file_name_length)?;
        // ea_size
        w.write_u32::<LittleEndian>(0)?;
        // short_name_length
        w.write_i8(0)?;
        // reserved u8, MUST NOT be added!
        // short_name
        w.extend_from_slice(&[0; 24]);
        // When working with this field, use file_name_length to determine the length of the file name rather
        // than assuming the presence of a trailing null delimiter. Dot directory names are valid for this field.
        w.extend_from_slice(&util::to_unicode(&self.file_name, false));
        Ok(w)
    }
}

/// 2.2.3.4.8 Client Drive Query Information Response (DR_DRIVE_QUERY_INFORMATION_RSP)
/// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpefs/37ef4fb1-6a95-4200-9fbf-515464f034a4
#[derive(Debug)]
#[allow(dead_code)]

struct ClientDriveQueryInformationResponse {
    device_io_response: DeviceIoResponse,
    length: u32,
    buffer: FsInformationClass,
}

#[allow(dead_code)]
impl ClientDriveQueryInformationResponse {
    /// Constructs a ClientDriveQueryInformationResponse from a ServerDriveQueryInformationRequest and an NTSTATUS.
    /// If the ServerDriveQueryInformationRequest.fs_information_class_lvl is currently unsupported, the program will panic.
    /// TODO(isaiah): We will pass some sort of file structure into here.
    fn new(req: &ServerDriveQueryInformationRequest, io_status: NTSTATUS) -> RdpResult<Self> {
        let (length, buffer) = match req.fs_information_class_lvl {
            FsInformationClassLevel::FileBasicInformation => (
                FILE_BASIC_INFORMATION_SIZE,
                FsInformationClass::FileBasicInformation(FileBasicInformation {
                    creation_time: 1,
                    last_access_time: 2,
                    last_write_time: 3,
                    change_time: 4,
                    file_attributes: flags::FileAttributes::FILE_ATTRIBUTE_DIRECTORY,
                }),
            ),
            FsInformationClassLevel::FileStandardInformation => (
                FILE_STANDARD_INFORMATION_SIZE,
                FsInformationClass::FileStandardInformation(FileStandardInformation {
                    allocation_size: 0,
                    end_of_file: 0,
                    number_of_links: 0,
                    delete_pending: Boolean::False,
                    directory: Boolean::True,
                }),
            ),
            _ => {
                return Err(not_implemented_error(&format!(
                    "received unsupported NTSTATUS: {:?}",
                    io_status
                )))
            }
        };

        Ok(Self {
            device_io_response: DeviceIoResponse::new(
                &req.device_io_request,
                NTSTATUS::to_u32(&io_status).unwrap(),
            ),
            length,
            buffer,
        })
    }

    fn encode(&self) -> RdpResult<Vec<u8>> {
        let mut w = vec![];
        w.extend_from_slice(&self.device_io_response.encode()?);
        w.write_u32::<LittleEndian>(self.length)?;
        w.extend_from_slice(&self.buffer.encode()?);
        Ok(w)
    }
}

/// 2.2.1.4.2 Device Close Request (DR_CLOSE_REQ)
/// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpefs/3ec6627f-9e0f-4941-a828-3fc6ed63d9e7
#[derive(Debug)]
#[allow(dead_code)]
struct DeviceCloseRequest {
    device_io_request: DeviceIoRequest,
    // Padding (32 bytes):  An array of 32 bytes. Reserved. This field can be set to any value, and MUST be ignored.
}

#[allow(dead_code)]
impl DeviceCloseRequest {
    fn decode(device_io_request: DeviceIoRequest) -> Self {
        Self { device_io_request }
    }
}

/// 2.2.1.5.2 Device Close Response (DR_CLOSE_RSP)
/// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpefs/0dae7031-cfd8-4f14-908c-ec06e14997b5
#[derive(Debug)]
#[allow(dead_code)]
struct DeviceCloseResponse {
    /// The CompletionId field of this header MUST match a Device I/O Request (section 2.2.1.4) message that had the MajorFunction field set to IRP_MJ_CLOSE.
    device_io_response: DeviceIoResponse,
    /// This field can be set to any value and MUST be ignored.
    padding: u32,
}
#[allow(dead_code)]
impl DeviceCloseResponse {
    fn new(device_close_request: DeviceCloseRequest, io_status: NTSTATUS) -> Self {
        Self {
            device_io_response: DeviceIoResponse::new(
                &device_close_request.device_io_request,
                NTSTATUS::to_u32(&io_status).unwrap(),
            ),
            padding: 0,
        }
    }

    fn encode(&self) -> RdpResult<Vec<u8>> {
        let mut w = vec![];
        w.extend_from_slice(&self.device_io_response.encode()?);
        w.write_u32::<LittleEndian>(self.padding)?;
        Ok(w)
    }
}

/// 2.2.3.3.11 Server Drive NotifyChange Directory Request (DR_DRIVE_NOTIFY_CHANGE_DIRECTORY_REQ)
/// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpefs/ed05e73d-e53e-4261-a1e1-365a70ba6512
#[derive(Debug)]
#[allow(dead_code)]
struct ServerDriveNotifyChangeDirectoryRequest {
    /// The MajorFunction field in the DR_DEVICE_IOREQUEST header MUST be set to IRP_MJ_DIRECTORY_CONTROL,
    /// and the MinorFunction field MUST be set to IRP_MN_NOTIFY_CHANGE_DIRECTORY.
    device_io_request: DeviceIoRequest,
    /// If nonzero, a change anywhere within the tree MUST trigger the notification response; otherwise, only a change in the root directory will do so.
    watch_tree: u8,
    completion_filter: flags::CompletionFilter,
    // Padding (27 bytes):  An array of 27 bytes. This field is unused and MUST be ignored.
}

#[allow(dead_code)]
impl ServerDriveNotifyChangeDirectoryRequest {
    fn decode(device_io_request: DeviceIoRequest, payload: &mut Payload) -> RdpResult<Self> {
        let invalid_flags =
            || invalid_data_error("invalid flags in Server Drive NotifyChange Directory Request");

        let watch_tree = payload.read_u8()?;
        let completion_filter =
            flags::CompletionFilter::from_bits(payload.read_u32::<LittleEndian>()?)
                .ok_or_else(invalid_flags)?;

        Ok(Self {
            device_io_request,
            watch_tree,
            completion_filter,
        })
    }
}

/// 2.2.1.4.3 Device Read Request (DR_READ_REQ)
/// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpefs/3192516d-36a6-47c5-987a-55c214aa0441
#[derive(Debug)]
#[allow(dead_code)]
struct DeviceReadRequest {
    /// The MajorFunction field in this header MUST be set to IRP_MJ_READ.
    device_io_request: DeviceIoRequest,
    /// This field specifies the maximum number of bytes to be read from the device.
    length: u32,
    /// This field specifies the file offset where the read operation is performed.
    offset: u64,
    // Padding (20 bytes):  An array of 20 bytes. Reserved. This field can be set to any value and MUST be ignored.
}

#[allow(dead_code)]
impl DeviceReadRequest {
    fn decode(device_io_request: DeviceIoRequest, payload: &mut Payload) -> RdpResult<Self> {
        Ok(Self {
            device_io_request,
            length: payload.read_u32::<LittleEndian>()?,
            offset: payload.read_u64::<LittleEndian>()?,
        })
    }
}

/// 2.2.1.5.3 Device Read Response (DR_READ_RSP)
/// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpefs/d35d3f91-fc5b-492b-80be-47f483ad1dc9
#[derive(Debug)]
#[allow(dead_code)]
struct DeviceReadResponse {
    /// The CompletionId field of this header MUST match a Device I/O Request (section 2.2.1.4) message that had the MajorFunction field set to IRP_MJ_READ.
    device_io_reply: DeviceIoResponse,
    /// Specifies the number of bytes in the ReadData field.
    length: u32,
    /// A variable-length array of bytes that specifies the output data from the read request.
    read_data: Vec<u8>,
}

#[allow(dead_code)]
impl DeviceReadResponse {
    fn new(
        device_read_request: &DeviceReadRequest,
        io_status: NTSTATUS,
        read_data: Vec<u8>,
    ) -> Self {
        let device_io_request = &device_read_request.device_io_request;

        Self {
            device_io_reply: DeviceIoResponse::new(
                device_io_request,
                NTSTATUS::to_u32(&io_status).unwrap(),
            ),
            length: u32::try_from(read_data.len()).unwrap(),
            read_data,
        }
    }

    fn encode(&self) -> RdpResult<Vec<u8>> {
        let mut w = vec![];
        w.extend_from_slice(&self.device_io_reply.encode()?);
        w.write_u32::<LittleEndian>(self.length)?;
        w.extend_from_slice(&self.read_data);
        Ok(w)
    }
}

/// 2.2.3.3.10 Server Drive Query Directory Request (DR_DRIVE_QUERY_DIRECTORY_REQ)
/// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpefs/458019d2-5d5a-4fd4-92ef-8c05f8d7acb1
#[derive(Debug)]
#[allow(dead_code)]
struct ServerDriveQueryDirectoryRequest {
    /// The MajorFunction field in the DR_DEVICE_IOREQUEST header MUST be set to IRP_MJ_DIRECTORY_CONTROL,
    /// and the MinorFunction field MUST be set to IRP_MN_QUERY_DIRECTORY.
    device_io_request: DeviceIoRequest,
    /// Must contain one of FileDirectoryInformation, FileFullDirectoryInformation, FileBothDirectoryInformation, FileNamesInformation
    fs_information_class_lvl: FsInformationClassLevel,
    /// If the value of this field is zero, the request is for the next file in the directory that was specified in a previous
    /// Server Drive Query Directory Request. If such a file does not exist, the client MUST complete this request with STATUS_NO_MORE_FILES
    /// in the IoStatus field of the Client Drive I/O Response packet (section 2.2.3.4).  If the value of this field is non-zero and such a
    /// file does not exist, the client MUST complete this request with STATUS_NO_SUCH_FILE in the IoStatus field of the Client Drive I/O Response.
    initial_query: u8,
    /// Specifies the number of bytes in the Path field, including the null-terminator.
    path_length: u32,
    // Padding (23 bytes): An array of 23 bytes. This field is unused and MUST be ignored.
    /// A variable-length array of Unicode characters (we will store this as a regular rust String) that specifies the directory
    /// on which this operation will be performed. The Path field MUST be null-terminated. If the value of the InitialQuery field
    /// is zero, then the contents of the Path field MUST be ignored, irrespective of the value specified in the PathLength field.
    path: String,
}

#[allow(dead_code)]
impl ServerDriveQueryDirectoryRequest {
    fn decode(device_io_request: DeviceIoRequest, payload: &mut Payload) -> RdpResult<Self> {
        let fs_information_class_lvl =
            FsInformationClassLevel::from_u32(payload.read_u32::<LittleEndian>()?)
                .ok_or_else(|| invalid_data_error("failed to read FsInformationClassLevel"))?;
        if fs_information_class_lvl != FsInformationClassLevel::FileDirectoryInformation
            && fs_information_class_lvl != FsInformationClassLevel::FileFullDirectoryInformation
            && fs_information_class_lvl != FsInformationClassLevel::FileBothDirectoryInformation
            && fs_information_class_lvl != FsInformationClassLevel::FileNamesInformation
        {
            return Err(invalid_data_error(&format!(
                "read invalid FsInformationClassLevel: {:?}, expected one of {:?}",
                fs_information_class_lvl,
                vec![
                    FsInformationClassLevel::FileDirectoryInformation,
                    FsInformationClassLevel::FileFullDirectoryInformation,
                    FsInformationClassLevel::FileBothDirectoryInformation,
                    FsInformationClassLevel::FileNamesInformation
                ]
            )));
        }
        let initial_query = payload.read_u8()?;
        let mut path_length: u32 = 0;
        let mut path = String::from("");
        if initial_query != 0 {
            path_length = payload.read_u32::<LittleEndian>()?;

            // TODO(isaiah): make a payload.skip(n)
            let mut padding: [u8; 23] = [0; 23];
            payload.read_exact(&mut padding)?;

            // TODO(isaiah): make a from_unicode_exact
            let mut path_as_vec = vec![0u8; path_length.try_into().unwrap()];
            payload.read_exact(&mut path_as_vec)?;
            path = util::from_unicode(path_as_vec)?;
        }

        Ok(Self {
            device_io_request,
            fs_information_class_lvl,
            initial_query,
            path_length,
            path,
        })
    }
}

/// 2.2.3.4.10 Client Drive Query Directory Response (DR_DRIVE_QUERY_DIRECTORY_RSP)
/// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpefs/9c929407-a833-4893-8f20-90c984756140
#[derive(Debug)]
#[allow(dead_code)]
struct ClientDriveQueryDirectoryResponse {
    /// The CompletionId field of the DR_DEVICE_IOCOMPLETION header MUST match a Device I/O Request (section 2.2.1.4) that
    /// has the MajorFunction field set to IRP_MJ_DIRECTORY_CONTROL and the MinorFunction field set to IRP_MN_QUERY_DIRECTORY.
    device_io_reply: DeviceIoResponse,
    /// Specifies the number of bytes in the Buffer field.
    length: u32,
    /// The content of this field is based on the value of the FsInformationClass field in the Server Drive Query Directory Request
    /// message, which determines the different structures that MUST be contained in the Buffer field.
    buffer: Option<FsInformationClass>,
    // Padding (1 byte): This field is unused and MUST be ignored.
}

#[allow(dead_code)]
impl ClientDriveQueryDirectoryResponse {
    fn new(
        req: &ServerDriveQueryDirectoryRequest,
        io_status: NTSTATUS,
        buffer: Option<FsInformationClass>,
    ) -> RdpResult<Self> {
        let device_io_request = &req.device_io_request;
        let length = match buffer {
            Some(ref fs_information_class) => match fs_information_class {
                FsInformationClass::FileBothDirectoryInformation(
                    file_both_directory_information,
                ) => {
                    FILE_BOTH_DIRECTORY_INFORMATION_BASE_SIZE
                        + file_both_directory_information.file_name_length
                }
                _ => {
                    return Err(not_implemented_error(&format!("ClientDriveQueryDirectoryResponse not implemented for fs_information_class {:?}", fs_information_class)));
                }
            },
            None => 0,
        };

        Ok(Self {
            device_io_reply: DeviceIoResponse::new(
                device_io_request,
                NTSTATUS::to_u32(&io_status).unwrap(),
            ),
            length,
            buffer,
        })
    }

    fn encode(&self) -> RdpResult<Vec<u8>> {
        let mut w = vec![];
        w.extend_from_slice(&self.device_io_reply.encode()?);

        if self.device_io_reply.io_status == NTSTATUS::to_u32(&NTSTATUS::STATUS_SUCCESS).unwrap() {
            w.write_u32::<LittleEndian>(self.length)?;
            w.extend_from_slice(
                &self
                    .buffer.as_ref()
                    .ok_or_else(|| invalid_data_error(
                        "ClientDriveQueryDirectoryResponse with NTSTATUS::STATUS_SUCCESS expects a FsInformationClass"
                    ))?
                    .encode()?,
            );
        } else if self.device_io_reply.io_status
            == NTSTATUS::to_u32(&NTSTATUS::STATUS_NO_MORE_FILES).unwrap()
        {
            // https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_file.c#L935-L937
            w.write_u32::<LittleEndian>(0)?;
            w.write_u8(0)?;
        } else {
            return Err(invalid_data_error(&format!(
                "Found ClientDriveQueryDirectoryResponse with invalid or unhandled NTSTATUS: {:?}",
                self.device_io_reply.io_status
            )));
        }

        Ok(w)
    }
}

type SharedDirectoryInfoResponseHandler =
    Box<dyn FnOnce(&mut Client, SharedDirectoryInfoResponse) -> RdpResult<Vec<Vec<u8>>>>;
