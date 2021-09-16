use crate::errors::{invalid_data_error, NTSTATUS_OK, SPECIAL_NO_RESPONSE};
use crate::scard;
use crate::Payload;
use bitflags::bitflags;
use byteorder::{LittleEndian, ReadBytesExt, WriteBytesExt};
use num_traits::{FromPrimitive, ToPrimitive};
use rdp::core::mcs;
use rdp::core::tpkt;
use rdp::model::data::Message;
use rdp::model::error::*;
use rdp::try_let;
use std::io::{Read, Write};

const CHANNEL_NAME: &str = "rdpdr";

pub struct Client {
    scard: scard::Client,
}

impl Client {
    pub fn new() -> Self {
        Client {
            scard: scard::Client::new(),
        }
    }
    pub fn read<S: Read + Write>(
        &self,
        payload: tpkt::Payload,
        mcs: &mut mcs::Client<S>,
    ) -> RdpResult<()> {
        let mut payload = try_let!(tpkt::Payload::Raw, payload)?;

        // Ignore this, we don't need anything from this header.
        let _pdu_header = ChannelPDUHeader::decode(&mut payload)?;

        let header = Header::decode(&mut payload)?;
        if let Component::RDPDR_CTYP_PRN = header.component {
            warn!("got {:?} RDPDR header from RDP server, ignoring because we're not redirecting any printers", header);
            return Ok(());
        }
        let resp = match header.packet_id {
            PacketId::PAKID_CORE_SERVER_ANNOUNCE => self.handle_server_announce(&mut payload)?,
            PacketId::PAKID_CORE_SERVER_CAPABILITY => {
                self.handle_server_capability(&mut payload)?
            }
            PacketId::PAKID_CORE_CLIENTID_CONFIRM => self.handle_client_id_confirm(&mut payload)?,
            PacketId::PAKID_CORE_DEVICE_REPLY => self.handle_device_reply(&mut payload)?,
            PacketId::PAKID_CORE_DEVICE_IOREQUEST => self.handle_device_io_request(&mut payload)?,
            _ => {
                // TODO(awly): return an error here once the entire protocol is implemented?
                error!(
                    "RDPDR packets {:?} are not implemented yet, ignoring",
                    header.packet_id
                );
                None
            }
        };

        if let Some(resp) = resp {
            Ok(mcs.write(&CHANNEL_NAME.to_string(), resp)?)
        } else {
            Ok(())
        }
    }

    fn handle_server_announce(&self, payload: &mut Payload) -> RdpResult<Option<Vec<u8>>> {
        let req = ServerAnnounceRequest::decode(payload)?;
        debug!("got ServerAnnounceRequest {:?}", req);

        let resp = encode_message(
            PacketId::PAKID_CORE_CLIENTID_CONFIRM,
            ClientAnnounceReply::new(req).encode()?,
        )?;
        debug!("sending client announce reply");
        Ok(Some(resp))
    }

    fn handle_server_capability(&self, payload: &mut Payload) -> RdpResult<Option<Vec<u8>>> {
        let req = ServerCoreCapabilityRequest::decode(payload)?;
        debug!("got {:?}", req);

        let resp = encode_message(
            PacketId::PAKID_CORE_CLIENT_CAPABILITY,
            ClientCoreCapabilityResponse::new_response().encode()?,
        )?;
        debug!("sending client core capability response");
        Ok(Some(resp))
    }

    fn handle_client_id_confirm(&self, payload: &mut Payload) -> RdpResult<Option<Vec<u8>>> {
        let req = ServerClientIdConfirm::decode(payload)?;
        debug!("got ServerClientIdConfirm {:?}", req);

        let resp = encode_message(
            PacketId::PAKID_CORE_DEVICELIST_ANNOUNCE,
            ClientDeviceListAnnounceRequest::new_smartcard().encode()?,
        )?;
        debug!("sending client device list announce request");
        Ok(Some(resp))
    }

    fn handle_device_reply(&self, payload: &mut Payload) -> RdpResult<Option<Vec<u8>>> {
        let req = ServerDeviceAnnounceResponse::decode(payload)?;
        debug!("got {:?}", req);

        if req.device_id != SCARD_DEVICE_ID {
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
            Ok(None)
        }
    }

    fn handle_device_io_request(&self, payload: &mut Payload) -> RdpResult<Option<Vec<u8>>> {
        let req = DeviceIoRequest::decode(payload)?;
        debug!("got {:?}", req);

        if let MajorFunction::IRP_MJ_DEVICE_CONTROL = req.major_function {
            let ioctl = DeviceControlRequest::decode(req, payload)?;
            debug!("got {:?}", ioctl);

            let (code, res) = self.scard.ioctl(ioctl.io_control_code, payload)?;
            if code == SPECIAL_NO_RESPONSE {
                return Ok(None);
            }
            let resp = encode_message(
                PacketId::PAKID_CORE_DEVICE_IOCOMPLETION,
                DeviceControlResponse::new(&ioctl, code, res).encode()?,
            )?;
            debug!("sending device IO response");
            Ok(Some(resp))
        } else {
            Err(invalid_data_error(&format!(
                "got unsupported major_function in DeviceIoRequest: {:?}",
                &req.major_function
            )))
        }
    }
}

impl Default for Client {
    fn default() -> Self {
        Self::new()
    }
}

fn encode_message(packet_id: PacketId, payload: Vec<u8>) -> RdpResult<Vec<u8>> {
    let mut inner = Header::new(Component::RDPDR_CTYP_CORE, packet_id).encode()?;
    inner.extend_from_slice(&payload);
    let mut outer = ChannelPDUHeader::new(inner.length() as u32).encode()?;
    outer.extend_from_slice(&inner);
    Ok(outer)
}

bitflags! {
    struct ChannelPDUFlags: u32 {
        const CHANNEL_FLAG_FIRST = 0x00000001;
        const CHANNEL_FLAG_LAST = 0x00000002;
        const CHANNEL_FLAG_ONLY = Self::CHANNEL_FLAG_FIRST.bits | Self::CHANNEL_FLAG_LAST.bits;
    }
}

#[derive(Debug)]
struct ChannelPDUHeader {
    length: u32,
    flags: ChannelPDUFlags,
}

impl ChannelPDUHeader {
    fn new(length: u32) -> Self {
        Self {
            length,
            flags: ChannelPDUFlags::CHANNEL_FLAG_ONLY,
        }
    }
    fn decode(payload: &mut Payload) -> RdpResult<Self> {
        Ok(Self {
            length: payload.read_u32::<LittleEndian>()?,
            flags: ChannelPDUFlags::from_bits(payload.read_u32::<LittleEndian>()?)
                .ok_or_else(|| invalid_data_error("invalid flags in ChannelPDUHeader"))?,
        })
    }
    fn encode(&self) -> RdpResult<Vec<u8>> {
        let mut w = vec![];
        w.write_u32::<LittleEndian>(self.length)?;
        w.write_u32::<LittleEndian>(self.flags.bits())?;
        Ok(w)
    }
}

#[derive(Debug)]
struct Header {
    component: Component,
    packet_id: PacketId,
}

impl Header {
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
        Self {
            num_capabilities: 2,
            padding: 0,
            capabilities: vec![
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
            ],
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

const SCARD_DEVICE_ID: u32 = 1;

#[derive(Debug)]
struct ClientDeviceListAnnounceRequest {
    count: u32,
    devices: Vec<DeviceAnnounceHeader>,
}

impl ClientDeviceListAnnounceRequest {
    fn new_smartcard() -> Self {
        Self {
            count: 1,
            devices: vec![DeviceAnnounceHeader {
                device_type: DeviceType::RDPDR_DTYP_SMARTCARD,
                device_id: SCARD_DEVICE_ID,
                preferred_dos_name: "SCARD".to_string(),
                device_data_length: 0,
                device_data: vec![],
            }],
        }
    }

    fn encode(&self) -> RdpResult<Vec<u8>> {
        let mut w = vec![];
        w.write_u32::<LittleEndian>(self.count)?;
        for dev in self.devices.iter() {
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
        let minor_function = payload.read_u32::<LittleEndian>()?;
        Ok(Self {
            device_id,
            file_id,
            completion_id,
            major_function: MajorFunction::from_u32(major_function).ok_or_else(|| {
                invalid_data_error(&format!(
                    "invalid major function value {:#010x}",
                    major_function
                ))
            })?,
            minor_function: MinorFunction::from_u32(minor_function).ok_or_else(|| {
                invalid_data_error(&format!(
                    "invalid minor function value {:#010x}",
                    minor_function
                ))
            })?,
        })
    }
}

#[derive(Debug, FromPrimitive, ToPrimitive)]
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
