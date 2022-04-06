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
use crate::util::{from_unicode, to_unicode};
use crate::Payload;
use crate::{scard, vchan};
use bitflags::bitflags;
use byteorder::{LittleEndian, ReadBytesExt, WriteBytesExt};
use num_traits::{FromPrimitive, ToPrimitive};
use rdp::core::mcs;
use rdp::core::tpkt;
use rdp::model::data::Message;
use rdp::model::error::*;
use std::convert::{TryFrom, TryInto};
use std::io::{Read, Write};
use utf16string::WString;

pub const CHANNEL_NAME: &str = "rdpdr";

/// Client implements a device redirection (RDPDR) client, as defined in
/// https://winprotocoldoc.blob.core.windows.net/productionwindowsarchives/MS-RDPEFS/%5bMS-RDPEFS%5d.pdf
///
/// This client only supports a single smartcard device.
pub struct Client {
    vchan: vchan::Client,
    scard: scard::Client,
}

impl Client {
    pub fn new(cert_der: Vec<u8>, key_der: Vec<u8>, pin: String) -> Self {
        Client {
            vchan: vchan::Client::new(),
            scard: scard::Client::new(cert_der, key_der, pin),
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
                // Device IO request is where communication with the smartcard actually happens.
                // Everything up to this point was negotiation and smartcard device registration.
                PacketId::PAKID_CORE_DEVICE_IOREQUEST => {
                    self.handle_device_io_request(&mut payload)?
                }
                _ => {
                    // We don't implement the full set of messages. Only the ones necessary for initial
                    // negotiation and registration of a smartcard device.
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
            ClientCoreCapabilityResponse::new_response().encode()?,
        )?;
        debug!("sending client core capability response");
        Ok(resp)
    }

    fn handle_client_id_confirm(&self, payload: &mut Payload) -> RdpResult<Vec<Vec<u8>>> {
        let req = ServerClientIdConfirm::decode(payload)?;
        debug!("got ServerClientIdConfirm {:?}", req);

        let resp = self.add_headers_and_chunkify(
            PacketId::PAKID_CORE_DEVICELIST_ANNOUNCE,
            ClientDeviceListAnnounceRequest::new_smartcard().encode()?,
        )?;
        debug!("sending client device list announce request");
        Ok(resp)
    }

    fn handle_device_reply(&self, payload: &mut Payload) -> RdpResult<Vec<Vec<u8>>> {
        let req = ServerDeviceAnnounceResponse::decode(payload)?;
        debug!("got ServerDeviceAnnounceResponse: {:?}", req);

        if req.device_id != SCARD_DEVICE_ID && req.device_id != DRIVE_DEVICE_ID {
            Err(invalid_data_error(&format!(
                "got ServerDeviceAnnounceResponse for unknown device_id {}",
                &req.device_id
            )))
        } else if req.result_code != NTSTATUS_OK {
            Err(invalid_data_error(&format!(
                "got unsuccessful ServerDeviceAnnounceResponse result code NTSTATUS({})",
                &req.result_code
            )))
        } else {
            debug!("ServerDeviceAnnounceResponse was valid");
            Ok(vec![])
        }
    }

    fn handle_device_io_request(&mut self, payload: &mut Payload) -> RdpResult<Vec<Vec<u8>>> {
        let device_io_request = DeviceIoRequest::decode(payload)?;
        debug!("got DeviceIORequest: {:?}", device_io_request);

        match device_io_request.major_function {
            // Used for smartcard control
            MajorFunction::IRP_MJ_DEVICE_CONTROL => {
                let ioctl = DeviceControlRequest::decode(device_io_request, payload)?;
                debug!("DeviceIORequest was a DeviceControlRequest: {:?}", ioctl);

                let (code, res) = self.scard.ioctl(ioctl.io_control_code, payload)?;
                if code == SPECIAL_NO_RESPONSE {
                    return Ok(vec![]);
                }
                let resp = self.add_headers_and_chunkify(
                    PacketId::PAKID_CORE_DEVICE_IOCOMPLETION,
                    DeviceControlResponse::new(&ioctl, code, res).encode()?,
                )?;
                debug!("sending device IO response");
                Ok(resp)
            }
            // Drive create request. This is sent to us by the server in response to
            // a ClientDeviceListAnnounce::new_drive, and TODO(isaiah).
            MajorFunction::IRP_MJ_CREATE => {
                let server_create_drive_request =
                    ServerCreateDriveRequest::decode(device_io_request, payload)?;
                debug!(
                    "DeviceIORequest was a ServerCreateDriveRequest: {:?}",
                    server_create_drive_request
                );
                // TODO(isaiah)
                Ok(vec![])
            }
            _ => Err(invalid_data_error(&format!(
                "got unsupported major_function in DeviceIoRequest: {:?}",
                &device_io_request.major_function
            ))),
        }
    }

    pub fn write_drive_announce<S: Read + Write>(
        &self,
        drive_name: String,
        mcs: &mut mcs::Client<S>,
    ) -> RdpResult<()> {
        let new_drive = ClientDeviceListAnnounce::new_drive(drive_name);
        debug!("sending new drive for redirection: {:?}", new_drive);
        let responses = self.add_headers_and_chunkify(
            PacketId::PAKID_CORE_DEVICELIST_ANNOUNCE,
            new_drive.encode()?,
        )?;
        let chan = &CHANNEL_NAME.to_string();
        for resp in responses {
            mcs.write(chan, resp)?;
        }

        Ok(())
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

#[derive(Debug, FromPrimitive, ToPrimitive)]
#[allow(non_camel_case_types)]
enum Component {
    RDPDR_CTYP_CORE = 0x4472,
    RDPDR_CTYP_PRN = 0x5052,
}

#[derive(Debug, FromPrimitive, ToPrimitive)]
#[allow(non_camel_case_types)]
enum PacketId {
    PAKID_CORE_SERVER_ANNOUNCE = 0x496E,
    PAKID_CORE_CLIENTID_CONFIRM = 0x4343,
    PAKID_CORE_CLIENT_NAME = 0x434E,
    PAKID_CORE_DEVICELIST_ANNOUNCE = 0x4441,
    PAKID_CORE_DEVICE_REPLY = 0x6472,
    PAKID_CORE_DEVICE_IOREQUEST = 0x4952,
    PAKID_CORE_DEVICE_IOCOMPLETION = 0x4943,
    PAKID_CORE_SERVER_CAPABILITY = 0x5350,
    PAKID_CORE_CLIENT_CAPABILITY = 0x4350,
    PAKID_CORE_DEVICELIST_REMOVE = 0x444D,
    PAKID_PRN_CACHE_DATA = 0x5043,
    PAKID_CORE_USER_LOGGEDON = 0x554C,
    PAKID_PRN_USING_XPS = 0x5543,
}

type ServerAnnounceRequest = ClientIdMessage;
type ClientAnnounceReply = ClientIdMessage;
type ServerClientIdConfirm = ClientIdMessage;

const VERSION_MAJOR: u16 = 0x0001;
const VERSION_MINOR: u16 = 0x000c;

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
    fn new_response() -> Self {
        // Clients are always required to send the "general" capability set.
        // In addition, we also send the optional smartcard capability (CAP_SMARTCARD_TYPE)
        // and drive capability (CAP_DRIVE_TYPE).
        let capabilities = vec![
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
            CapabilitySet {
                header: CapabilityHeader {
                    cap_type: CapabilityType::CAP_DRIVE_TYPE,
                    length: 8, // 8 byte header + empty capability descriptor
                    version: DRIVE_CAPABILITY_VERSION_02,
                },
                data: Capability::Drive,
            },
        ];

        Self {
            padding: 0,
            num_capabilities: u16::try_from(capabilities.len()).ok().unwrap(),
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

const SMARTCARD_CAPABILITY_VERSION_01: u32 = 0x00000001;
const DRIVE_CAPABILITY_VERSION_02: u32 = 0x00000002;
#[allow(dead_code)]
const GENERAL_CAPABILITY_VERSION_01: u32 = 0x00000001;
const GENERAL_CAPABILITY_VERSION_02: u32 = 0x00000002;

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

#[derive(Debug, FromPrimitive, ToPrimitive)]
#[allow(non_camel_case_types)]
enum CapabilityType {
    CAP_GENERAL_TYPE = 0x0001,
    CAP_PRINTER_TYPE = 0x0002,
    CAP_PORT_TYPE = 0x0003,
    CAP_DRIVE_TYPE = 0x0004,
    CAP_SMARTCARD_TYPE = 0x0005,
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

// Each redirected device requires a unique ID.
const SCARD_DEVICE_ID: u32 = 1;
const DRIVE_DEVICE_ID: u32 = 2;

#[derive(Debug)]
struct ClientDeviceListAnnounceRequest {
    device_count: u32,
    device_list: Vec<DeviceAnnounceHeader>,
}

type ClientDeviceListAnnounce = ClientDeviceListAnnounceRequest;

impl ClientDeviceListAnnounceRequest {
    // We only need to announce the smartcard in this Client Device List Announce Request.
    // Drives (directories) can be announced at any time with a Client Drive Device List Announce.
    fn new_smartcard() -> Self {
        Self {
            device_count: 1,
            device_list: vec![DeviceAnnounceHeader {
                device_type: DeviceType::RDPDR_DTYP_SMARTCARD,
                device_id: SCARD_DEVICE_ID,
                // This name is a constant defined by the spec.
                preferred_dos_name: "SCARD".to_string(),
                device_data_length: 0,
                device_data: vec![],
            }],
        }
    }

    fn new_drive(drive_name: String) -> Self {
        let device_data = to_unicode(&drive_name);

        let device_list = vec![DeviceAnnounceHeader {
            device_type: DeviceType::RDPDR_DTYP_FILESYSTEM,
            device_id: DRIVE_DEVICE_ID,
            preferred_dos_name: drive_name,
            // According to the spec:
            //
            // If the client supports DRIVE_CAPABILITY_VERSION_02 in the Drive Capability Set,
            // then the full name MUST also be specified in the DeviceData field, as a null-terminated
            // Unicode string. If the DeviceDataLength field is nonzero, the content of the
            // PreferredDosName field is ignored.
            device_data_length: device_data.len() as u32,
            device_data,
        }];

        Self {
            device_count: 1,
            device_list,
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
        if name.len() > 8 {
            name = &name[..8];
        }
        w.extend_from_slice(&format!("{:\x00<8}", name).into_bytes());
        w.write_u32::<LittleEndian>(self.device_data_length)?;
        w.extend_from_slice(&self.device_data);
        Ok(w)
    }
}

#[derive(Debug, FromPrimitive, ToPrimitive)]
#[allow(non_camel_case_types)]
enum DeviceType {
    RDPDR_DTYP_SERIAL = 0x00000001,
    RDPDR_DTYP_PARALLEL = 0x00000002,
    RDPDR_DTYP_PRINT = 0x00000004,
    RDPDR_DTYP_FILESYSTEM = 0x00000008,
    RDPDR_DTYP_SMARTCARD = 0x00000020,
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

#[derive(Debug)]
#[allow(dead_code)]
struct DeviceIoRequest {
    device_id: u32,
    file_id: u32,
    completion_id: u32,
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

#[derive(Debug, FromPrimitive, ToPrimitive, PartialEq)]
#[allow(non_camel_case_types)]
enum MajorFunction {
    IRP_MJ_CREATE = 0x00000000,
    IRP_MJ_CLOSE = 0x00000002,
    IRP_MJ_READ = 0x00000003,
    IRP_MJ_WRITE = 0x00000004,
    IRP_MJ_DEVICE_CONTROL = 0x0000000E,
    IRP_MJ_QUERY_VOLUME_INFORMATION = 0x0000000A,
    IRP_MJ_SET_VOLUME_INFORMATION = 0x0000000B,
    IRP_MJ_QUERY_INFORMATION = 0x00000005,
    IRP_MJ_SET_INFORMATION = 0x00000006,
    IRP_MJ_DIRECTORY_CONTROL = 0x0000000C,
    IRP_MJ_LOCK_CONTROL = 0x00000011,
}

#[derive(Debug, FromPrimitive, ToPrimitive)]
#[allow(non_camel_case_types)]
enum MinorFunction {
    IRP_MN_NONE = 0x00000000,
    IRP_MN_QUERY_DIRECTORY = 0x00000001,
    IRP_MN_NOTIFY_CHANGE_DIRECTORY = 0x00000002,
}

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

    fn encode(&mut self) -> RdpResult<Vec<u8>> {
        let mut w = vec![];
        w.extend_from_slice(&self.header.encode()?);
        w.write_u32::<LittleEndian>(self.output_buffer_length)?;
        w.extend_from_slice(&self.output_buffer);
        Ok(w)
    }
}

/// 2.2.3.3.1 Server Create Drive Request (DR_DRIVE_CREATE_REQ)
/// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpefs/95b16fd0-d530-407c-a310-adedc85e9897
type ServerCreateDriveRequest = DeviceCreateRequest;

/// 2.2.1.4.1 Device Create Request (DR_CREATE_REQ)
/// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpefs/5f71f6d2-d9ff-40c2-bdb5-a739447d3c3e
#[derive(Debug)]
struct DeviceCreateRequest {
    /// A DR_DEVICE_IOREQUEST header. The MajorFunction field in this header MUST be set to IRP_MJ_CREATE.
    device_io_request: DeviceIoRequest,
    /// A 32-bit unsigned integer that specifies the level of access. This field is specified in [MS-SMB2] section 2.2.13.
    desired_access: DesiredAccessFlags,
    /// A 64-bit unsigned integer that specifies the initial allocation size for the file.
    allocation_size: u64,
    /// A 32-bit unsigned integer that specifies the attributes for the file being created. This field is specified in [MS-SMB2] section 2.2.13.
    file_attributes: FileAttributesFlags,
    /// A 32-bit unsigned integer that specifies the sharing mode for the file being opened. This field is specified in [MS-SMB2] section 2.2.13.
    shared_access: SharedAccessFlags,
    /// A 32-bit unsigned integer that specifies the action for the client to take if the file already exists. This field is specified in [MS-SMB2] section 2.2.13. For ports and other devices, this field MUST be set to FILE_OPEN (0x00000001).
    create_disposition: CreateDispositionFlags,
    /// A 32-bit unsigned integer that specifies the options for creating the file. This field is specified in [MS-SMB2] section 2.2.13.
    create_options: CreateOptionsFlags,
    /// A 32-bit unsigned integer that specifies the number of bytes in the Path field, including the null-terminator.
    path_length: u32,
    /// A variable-length array of Unicode characters, including the null-terminator, whose size is specified by the PathLength field. The protocol imposes no limitations on the characters used in this field.
    path: String,
}

impl DeviceCreateRequest {
    fn decode(device_io_request: DeviceIoRequest, payload: &mut Payload) -> RdpResult<Self> {
        let invalid_flags = || invalid_data_error("invalid flags in Device Create Request");

        let desired_access = DesiredAccessFlags::from_bits(payload.read_u32::<LittleEndian>()?)
            .ok_or_else(invalid_flags)?;
        let allocation_size = payload.read_u64::<LittleEndian>()?;
        let file_attributes = FileAttributesFlags::from_bits(payload.read_u32::<LittleEndian>()?)
            .ok_or_else(invalid_flags)?;
        let shared_access = SharedAccessFlags::from_bits(payload.read_u32::<LittleEndian>()?)
            .ok_or_else(invalid_flags)?;
        let create_disposition =
            CreateDispositionFlags::from_bits(payload.read_u32::<LittleEndian>()?)
                .ok_or_else(invalid_flags)?;
        let create_options = CreateOptionsFlags::from_bits(payload.read_u32::<LittleEndian>()?)
            .ok_or_else(invalid_flags)?;
        let path_length = payload.read_u32::<LittleEndian>()?;

        // usize is 32 bits on a 32 bit target and 64 on a 64, so we can safely say try_into().unwrap()
        // for a u32 will never panic on the machines that run teleport.
        let mut path = vec![0u8; path_length.try_into().unwrap()];
        payload.read_exact(&mut path)?;
        let path = from_unicode(path)?;

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

bitflags! {
    /// DesiredAccess can be interpreted as either
    /// 2.2.13.1.1 File_Pipe_Printer_Access_Mask [MS-SMB2] (https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-smb2/77b36d0f-6016-458a-a7a0-0f4a72ae1534)
    /// or
    /// 2.2.13.1.2 Directory_Access_Mask [MS-SMB2] (https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-smb2/0a5934b1-80f1-4da0-b1bf-5e021c309b71)
    ///
    /// This implements the combination of the two. For flags where the names and/or functions are distinct between the two,
    /// the names are appended with an "_OR_", and the File_Pipe_Printer_Access_Mask functionality is described on the top line comment,
    /// and the Directory_Access_Mask functionality is described on the bottom (2nd) line comment.
    struct DesiredAccessFlags: u32 {
        /// This value indicates the right to read data from the file or named pipe.
        /// This value indicates the right to enumerate the contents of the directory.
        const FILE_READ_DATA_OR_FILE_LIST_DIRECTORY = 0x00000001;
        /// This value indicates the right to write data into the file or named pipe beyond the end of the file.
        /// This value indicates the right to create a file under the directory.
        const FILE_WRITE_DATA_OR_FILE_ADD_FILE = 0x00000002;
        /// This value indicates the right to append data into the file or named pipe.
        /// This value indicates the right to add a sub-directory under the directory.
        const FILE_APPEND_DATA_OR_FILE_ADD_SUBDIRECTORY = 0x00000004;
        /// This value indicates the right to read the extended attributes of the file or named pipe.
        const FILE_READ_EA = 0x00000008;
        /// This value indicates the right to write or change the extended attributes to the file or named pipe.
        const FILE_WRITE_EA = 0x00000010;
        /// This value indicates the right to traverse this directory if the server enforces traversal checking.
        const FILE_TRAVERSE = 0x00000020;
        /// This value indicates the right to delete entries within a directory.
        const FILE_DELETE_CHILD = 0x00000040;
        /// This value indicates the right to execute the file/directory.
        const FILE_EXECUTE = 0x00000020;
        /// This value indicates the right to read the attributes of the file/directory.
        const FILE_READ_ATTRIBUTES = 0x00000080;
        /// This value indicates the right to change the attributes of the file/directory.
        const FILE_WRITE_ATTRIBUTES = 0x00000100;
        /// This value indicates the right to delete the file/directory.
        const DELETE = 0x00010000;
        /// This value indicates the right to read the security descriptor for the file/directory or named pipe.
        const READ_CONTROL = 0x00020000;
        /// This value indicates the right to change the discretionary access control list (DACL) in the security descriptor for the file/directory or named pipe. For the DACL data structure, see ACL in [MS-DTYP].
        const WRITE_DAC = 0x00040000;
        /// This value indicates the right to change the owner in the security descriptor for the file/directory or named pipe.
        const WRITE_OWNER = 0x00080000;
        /// SMB2 clients set this flag to any value. SMB2 servers SHOULD ignore this flag.
        const SYNCHRONIZE = 0x00100000;
        /// This value indicates the right to read or change the system access control list (SACL) in the security descriptor for the file/directory or named pipe. For the SACL data structure, see ACL in [MS-DTYP].
        const ACCESS_SYSTEM_SECURITY = 0x01000000;
        /// This value indicates that the client is requesting an open to the file with the highest level of access the client has on this file. If no access is granted for the client on this file, the server MUST fail the open with STATUS_ACCESS_DENIED.
        const MAXIMUM_ALLOWED = 0x02000000;
        /// This value indicates a request for all the access flags that are previously listed except MAXIMUM_ALLOWED and ACCESS_SYSTEM_SECURITY.
        const GENERIC_ALL = 0x10000000;
        /// This value indicates a request for the following combination of access flags listed above: FILE_READ_ATTRIBUTES| FILE_EXECUTE| SYNCHRONIZE| READ_CONTROL.
        const GENERIC_EXECUTE = 0x20000000;
        /// This value indicates a request for the following combination of access flags listed above: FILE_WRITE_DATA| FILE_APPEND_DATA| FILE_WRITE_ATTRIBUTES| FILE_WRITE_EA| SYNCHRONIZE| READ_CONTROL.
        const GENERIC_WRITE = 0x40000000;
        /// This value indicates a request for the following combination of access flags listed above: FILE_READ_DATA| FILE_READ_ATTRIBUTES| FILE_READ_EA| SYNCHRONIZE| READ_CONTROL.
        const GENERIC_READ = 0x80000000;
    }
}

bitflags! {
    /// 2.6 File Attributes [MS-FSCC]
    /// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-fscc/ca28ec38-f155-4768-81d6-4bfeb8586fc9
    struct FileAttributesFlags: u32 {
        /// A file or directory that is read-only. For a file, applications can read the file but cannot write to it or delete it. For a directory, applications cannot delete it, but applications can create and delete files from that directory.
        const FILE_ATTRIBUTE_READONLY = 0x00000001;
        /// A file or directory that is hidden. Files and directories marked with this attribute do not appear in an ordinary directory listing.
        const FILE_ATTRIBUTE_HIDDEN = 0x00000002;
        /// A file or directory that the operating system uses a part of or uses exclusively.
        const FILE_ATTRIBUTE_SYSTEM = 0x00000004;
        /// This item is a directory.
        const FILE_ATTRIBUTE_DIRECTORY = 0x00000010;
        /// A file or directory that requires to be archived. Applications use this attribute to mark files for backup or removal.
        const FILE_ATTRIBUTE_ARCHIVE = 0x00000020;
        /// A file that does not have other attributes set. This flag is used to clear all other flags by specifying it with no other flags set. This flag MUST be ignored if other flags are set.<161>
        const FILE_ATTRIBUTE_NORMAL = 0x00000080;
        /// A file that is being used for temporary storage. The operating system can choose to store this file's data in memory rather than on mass storage, writing the data to mass storage only if data remains in the file when the file is closed.
        const FILE_ATTRIBUTE_TEMPORARY = 0x00000100;
        /// A file that is a sparse file.
        const FILE_ATTRIBUTE_SPARSE_FILE = 0x00000200;
        /// A file or directory that has an associated reparse point.
        const FILE_ATTRIBUTE_REPARSE_POINT = 0x00000400;
        /// A file or directory that is compressed. For a file, all of the data in the file is compressed. For a directory, compression is the default for newly created files and subdirectories.
        const FILE_ATTRIBUTE_COMPRESSED = 0x00000800;
        /// The data in this file is not available immediately. This attribute indicates that the file data is physically moved to offline storage. This attribute is used by Remote Storage, which is hierarchical storage management software.
        const FILE_ATTRIBUTE_OFFLINE = 0x00001000;
        /// A file or directory that is not indexed by the content indexing service.
        const FILE_ATTRIBUTE_NOT_CONTENT_INDEXED = 0x00002000;
        /// A file or directory that is encrypted. For a file, all data streams in the file are encrypted. For a directory, encryption is the default for newly created files and subdirectories.
        const FILE_ATTRIBUTE_ENCRYPTED = 0x00004000;
        /// A file or directory that is configured with integrity support. For a file, all data streams in the file have integrity support. For a directory, integrity support is the default for newly created files and subdirectories, unless the caller specifies otherwise.<162>
        const FILE_ATTRIBUTE_INTEGRITY_STREAM = 0x00008000;
        /// A file or directory that is configured to be excluded from the data integrity scan. For a directory configured with FILE_ATTRIBUTE_NO_SCRUB_DATA, the default for newly created files and subdirectories is to inherit the FILE_ATTRIBUTE_NO_SCRUB_DATA attribute.<163>
        const FILE_ATTRIBUTE_NO_SCRUB_DATA = 0x00020000;
        /// This attribute appears only in directory enumeration classes (FILE_DIRECTORY_INFORMATION, FILE_BOTH_DIR_INFORMATION, etc.). When this attribute is set, it means that the file or directory has no physical representation on the local system; the item is virtual. Opening the item will be more expensive than usual because it will cause at least some of the file or directory content to be fetched from a remote store. This attribute can only be set by kernel-mode components. This attribute is for use with hierarchical storage management software.<164>
        const FILE_ATTRIBUTE_RECALL_ON_OPEN = 0x00040000;
        /// This attribute indicates user intent that the file or directory should be kept fully present locally even when not being actively accessed. This attribute is for use with hierarchical storage management software.<165>
        const FILE_ATTRIBUTE_PINNED = 0x00080000;
        /// This attribute indicates that the file or directory should not be kept fully present locally except when being actively accessed. This attribute is for use with hierarchical storage management software.<166>
        const FILE_ATTRIBUTE_UNPINNED = 0x00100000;
        /// When this attribute is set, it means that the file or directory is not fully present locally. For a file this means that not all of its data is on local storage (for example, it may be sparse with some data still in remote storage). For a directory it means that some of the directory contents are being virtualized from another location. Reading the file or enumerating the directory will be more expensive than usual because it will cause at least some of the file or directory content to be fetched from a remote store. Only kernel-mode callers can set this attribute. This attribute is for use with hierarchical storage management software.<167>
        const FILE_ATTRIBUTE_RECALL_ON_DATA_ACCESS = 0x00400000;
    }
}

bitflags! {
    /// Specifies the sharing mode for the open. If ShareAccess values of FILE_SHARE_READ, FILE_SHARE_WRITE and FILE_SHARE_DELETE are set for a printer file or a named pipe, the server SHOULD<35> ignore these values. The field MUST be constructed using a combination of zero or more of the following bit values.
    /// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-smb2/e8fb45c1-a03d-44ca-b7ae-47385cfd7997
    struct SharedAccessFlags: u32 {
        /// When set, indicates that other opens are allowed to read this file while this open is present. This bit MUST NOT be set for a named pipe or a printer file. Each open creates a new instance of a named pipe. Likewise, opening a printer file always creates a new file.
        const FILE_SHARE_READ = 0x00000001;
        /// When set, indicates that other opens are allowed to write this file while this open is present. This bit MUST NOT be set for a named pipe or a printer file. Each open creates a new instance of a named pipe. Likewise, opening a printer file always creates a new file.
        const FILE_SHARE_WRITE = 0x00000002;
        /// When set, indicates that other opens are allowed to delete or rename this file while this open is present. This bit MUST NOT be set for a named pipe or a printer file. Each open creates a new instance of a named pipe. Likewise, opening a printer file always creates a new file.
        const FILE_SHARE_DELETE = 0x00000004;
    }
}

bitflags! {
    /// Defines the action the server MUST take if the file that is specified in the name field already exists. For opening named pipes, this field can be set to any value by the client and MUST be ignored by the server. For other files, this field MUST contain one of the following values.
    /// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-smb2/e8fb45c1-a03d-44ca-b7ae-47385cfd7997
    struct CreateDispositionFlags: u32 {
        /// If the file already exists, supersede it. Otherwise, create the file. This value SHOULD NOT be used for a printer object.<36>
        const FILE_SUPERSEDE = 0x00000000;
        /// If the file already exists, return success; otherwise, fail the operation. MUST NOT be used for a printer object.
        const FILE_OPEN = 0x00000001;
        /// If the file already exists, fail the operation; otherwise, create the file.
        const FILE_CREATE = 0x00000002;
        /// Open the file if it already exists; otherwise, create the file. This value SHOULD NOT be used for a printer object.<37>
        const FILE_OPEN_IF = 0x00000003;
        /// Overwrite the file if it already exists; otherwise, fail the operation. MUST NOT be used for a printer object.
        const FILE_OVERWRITE = 0x00000004;
        /// Overwrite the file if it already exists; otherwise, create the file. This value SHOULD NOT be used for a printer object.<38>
        const FILE_OVERWRITE_IF = 0x00000005;
    }
}

bitflags! {
    /// Specifies the options to be applied when creating or opening the file. Combinations of the bit positions listed below are valid, unless otherwise noted. This field MUST be constructed using the following values.
    /// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-smb2/e8fb45c1-a03d-44ca-b7ae-47385cfd7997
    struct CreateOptionsFlags: u32 {
        /// The file being created or opened is a directory file. With this flag, the CreateDisposition field MUST be set to FILE_CREATE, FILE_OPEN_IF, or FILE_OPEN. With this flag, only the following CreateOptions values are valid: FILE_WRITE_THROUGH, FILE_OPEN_FOR_BACKUP_INTENT, FILE_DELETE_ON_CLOSE, and FILE_OPEN_REPARSE_POINT. If the file being created or opened already exists and is not a directory file and FILE_CREATE is specified in the CreateDisposition field, then the server MUST fail the request with STATUS_OBJECT_NAME_COLLISION. If the file being created or opened already exists and is not a directory file and FILE_CREATE is not specified in the CreateDisposition field, then the server MUST fail the request with STATUS_NOT_A_DIRECTORY. The server MUST fail an invalid CreateDisposition field or an invalid combination of CreateOptions flags with STATUS_INVALID_PARAMETER.
        const FILE_DIRECTORY_FILE = 0x00000001;
        /// The server performs file write-through; file data is written to the underlying storage before completing the write operation on this open.
        const FILE_WRITE_THROUGH = 0x00000002;
        /// This indicates that the application intends to read or write at sequential offsets using this handle, so the server SHOULD optimize for sequential access. However, the server MUST accept any access pattern. This flag value is incompatible with the FILE_RANDOM_ACCESS value.
        const FILE_SEQUENTIAL_ONLY = 0x00000004;
        /// File buffering is not performed on this open; file data is not retained in memory upon writing it to, or reading it from, the underlying storage.
        const FILE_NO_INTERMEDIATE_BUFFERING = 0x00000008;
        /// This bit SHOULD be set to 0 and MUST be ignored by the server.<40>
        const FILE_SYNCHRONOUS_IO_ALERT = 0x00000010;
        /// This bit SHOULD be set to 0 and MUST be ignored by the server.<41>
        const FILE_SYNCHRONOUS_IO_NONALERT = 0x00000020;
        /// If the name of the file being created or opened matches with an existing directory file, the server MUST fail the request with STATUS_FILE_IS_A_DIRECTORY. This flag MUST NOT be used with FILE_DIRECTORY_FILE or the server MUST fail the request with STATUS_INVALID_PARAMETER.
        const FILE_NON_DIRECTORY_FILE = 0x00000040;
        /// This bit SHOULD be set to 0 and MUST be ignored by the server.<42>
        const FILE_COMPLETE_IF_OPLOCKED = 0x00000100;
        /// The caller does not understand how to handle extended attributes. If the request includes an SMB2_CREATE_EA_BUFFER create context, then the server MUST fail this request with STATUS_ACCESS_DENIED. If extended attributes with the FILE_NEED_EA flag (see [MS-FSCC] section 2.4.15) set are associated with the file being opened, then the server MUST fail this request with STATUS_ACCESS_DENIED.
        const FILE_NO_EA_KNOWLEDGE = 0x00000200;
        /// This indicates that the application intends to read or write at random offsets using this handle, so the server SHOULD optimize for random access. However, the server MUST accept any access pattern. This flag value is incompatible with the FILE_SEQUENTIAL_ONLY value. If both FILE_RANDOM_ACCESS and FILE_SEQUENTIAL_ONLY are set, then FILE_SEQUENTIAL_ONLY is ignored.
        const FILE_RANDOM_ACCESS = 0x00000800;
        /// The file MUST be automatically deleted when the last open request on this file is closed. When this option is set, the DesiredAccess field MUST include the DELETE flag. This option is often used for temporary files.
        const FILE_DELETE_ON_CLOSE = 0x00001000;
        /// This bit SHOULD be set to 0 and the server MUST fail the request with a STATUS_NOT_SUPPORTED error if this bit is set.<43>
        const FILE_OPEN_BY_FILE_ID = 0x00002000;
        /// The file is being opened for backup intent. That is, it is being opened or created for the purposes of either a backup or a restore operation. The server can check to ensure that the caller is capable of overriding whatever security checks have been placed on the file to allow a backup or restore operation to occur. The server can check for access rights to the file before checking the DesiredAccess field.
        const FILE_OPEN_FOR_BACKUP_INTENT = 0x00004000;
        /// The file cannot be compressed. This bit is ignored when FILE_DIRECTORY_FILE is set in CreateOptions.
        const FILE_NO_COMPRESSION = 0x00008000;
        /// This bit SHOULD be set to 0 and MUST be ignored by the server.
        const FILE_OPEN_REMOTE_INSTANCE = 0x00000400;
        /// This bit SHOULD be set to 0 and MUST be ignored by the server.
        const FILE_OPEN_REQUIRING_OPLOCK = 0x00010000;
        /// This bit SHOULD be set to 0 and MUST be ignored by the server.
        const FILE_DISALLOW_EXCLUSIVE = 0x00020000;
        /// This bit SHOULD be set to 0 and the server MUST fail the request with a STATUS_NOT_SUPPORTED error if this bit is set.<44>
        const FILE_RESERVE_OPFILTER = 0x00100000;
        /// If the file or directory being opened is a reparse point, open the reparse point itself rather than the target that the reparse point references.
        const FILE_OPEN_REPARSE_POINT = 0x00200000;
        /// In an HSM (Hierarchical Storage Management) environment, this flag means the file SHOULD NOT be recalled from tertiary storage such as tape. The recall can take several minutes. The caller can specify this flag to avoid those delays.
        const FILE_OPEN_NO_RECALL = 0x00400000;
        /// Open file to query for free space. The client SHOULD set this to 0 and the server MUST ignore it.<45>
        const FILE_OPEN_FOR_FREE_SPACE_QUERY = 0x00800000;
    }
}
