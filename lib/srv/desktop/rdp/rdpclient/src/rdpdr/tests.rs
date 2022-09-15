// Copyright 2022 Gravitational, Inc
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

use super::{
    scard::{
        Context, EstablishContext_Call, GetDeviceTypeId_Call, IoctlCode, ListReaders_Call,
        ScardAccessStartedEvent_Call, Scope,
    },
    *,
};
use crate::{
    vchan::{ChannelPDUFlags, ChannelPDUHeader},
    Encode, Messages,
};

/// This function can be called at any point during a test, after which
/// all logs will print if the test fails. It is useful for debugging.
///
/// Tests must be called like `RUST_LOG=debug cargo test`.
///
/// https://docs.rs/env_logger/0.7.1/env_logger/#capturing-logs-in-tests
#[allow(dead_code)]
fn init_logger() {
    let _ = env_logger::builder().is_test(true).try_init();
}

#[test]
fn test_to_windows_time() {
    // Cross checked against
    // https://www.silisoftware.com/tools/date.php?inputdate=1655246166&inputformat=unix
    assert_eq!(to_windows_time(1655246166 * 1000), 132997197660000000);
    assert_eq!(to_windows_time(1000), 116444736010000000);
}

fn client() -> Client {
    Client::new(Config {
        cert_der: vec![],
        key_der: vec![],
        pin: "".to_string(),
        allow_directory_sharing: true,
        tdp_sd_acknowledge: Box::new(
            move |mut _ack: SharedDirectoryAcknowledge| -> RdpResult<()> { Ok(()) },
        ),
        tdp_sd_info_request: Box::new(move |_req: SharedDirectoryInfoRequest| -> RdpResult<()> {
            Ok(())
        }),
        tdp_sd_create_request: Box::new(
            move |_req: SharedDirectoryCreateRequest| -> RdpResult<()> { Ok(()) },
        ),
        tdp_sd_delete_request: Box::new(
            move |_req: SharedDirectoryDeleteRequest| -> RdpResult<()> { Ok(()) },
        ),
        tdp_sd_list_request: Box::new(move |_req: SharedDirectoryListRequest| -> RdpResult<()> {
            Ok(())
        }),
        tdp_sd_read_request: Box::new(move |_req: SharedDirectoryReadRequest| -> RdpResult<()> {
            Ok(())
        }),
        tdp_sd_write_request: Box::new(move |_req: SharedDirectoryWriteRequest| -> RdpResult<()> {
            Ok(())
        }),
        tdp_sd_move_request: Box::new(move |_req: SharedDirectoryMoveRequest| -> RdpResult<()> {
            Ok(())
        }),
    })
}

struct PayloadIn {
    channel_pdu_header: ChannelPDUHeader,
    shared_header: SharedHeader,
    request: Box<dyn Encode>,
    scard_ctl: Option<Box<dyn Encode>>,
}

type ResponseOut = Vec<(PacketId, Box<dyn Encode>)>;

fn create_payload(v: Vec<u8>, pos: u64) -> tpkt::Payload {
    let mut p = Payload::new(v);
    p.set_position(pos);
    tpkt::Payload::Raw(p)
}

fn test_payload_in_to_response_out(
    c: &mut Client,
    payload_in: PayloadIn,
    responses_out: ResponseOut,
) {
    // encode the incoming payload
    let mut encoded_payload = payload_in.channel_pdu_header.encode().unwrap();
    encoded_payload.extend(payload_in.shared_header.encode().unwrap());
    encoded_payload.extend(payload_in.request.encode().unwrap());
    if let Some(scard_ctl) = payload_in.scard_ctl {
        encoded_payload.extend(scard_ctl.encode().unwrap());
    }
    let encoded_payload = create_payload(encoded_payload, 0);

    // encode the outgoing responses
    let mut encoded_responses: Messages = vec![];
    for (packet_id, resp) in responses_out {
        encoded_responses.append(
            &mut c
                .add_headers_and_chunkify(packet_id, resp.encode().unwrap())
                .unwrap(),
        );
    }

    // check that the client processes the payload as expected
    assert_eq!(
        c.read_and_create_reply(encoded_payload).unwrap(),
        encoded_responses
    )
}

fn test_handle_server_announce(c: &mut Client) {
    test_payload_in_to_response_out(
        c,
        PayloadIn {
            channel_pdu_header: ChannelPDUHeader {
                length: 12,
                flags: ChannelPDUFlags::CHANNEL_FLAG_FIRST
                    | ChannelPDUFlags::CHANNEL_FLAG_LAST
                    | ChannelPDUFlags::CHANNEL_FLAG_ONLY,
            },
            shared_header: SharedHeader {
                component: Component::RDPDR_CTYP_CORE,
                packet_id: PacketId::PAKID_CORE_SERVER_ANNOUNCE,
            },
            request: Box::new(ServerAnnounceRequest {
                version_major: 1,
                version_minor: 13,
                client_id: 3,
            }),
            scard_ctl: None,
        },
        vec![
            (
                PacketId::PAKID_CORE_CLIENTID_CONFIRM,
                Box::new(ClientAnnounceReply {
                    version_major: 1,
                    version_minor: 12,
                    client_id: 3,
                }),
            ),
            (
                PacketId::PAKID_CORE_CLIENT_NAME,
                Box::new(ClientNameRequest {
                    unicode_flag: ClientNameRequestUnicodeFlag::Ascii,
                    computer_name: CString::new("teleport").unwrap(),
                }),
            ),
        ],
    );
}

fn test_handle_server_capability(c: &mut Client) {
    test_payload_in_to_response_out(
        c,
        PayloadIn {
            channel_pdu_header: ChannelPDUHeader {
                length: 84,
                flags: ChannelPDUFlags::CHANNEL_FLAG_FIRST
                    | ChannelPDUFlags::CHANNEL_FLAG_LAST
                    | ChannelPDUFlags::CHANNEL_FLAG_ONLY,
            },
            shared_header: SharedHeader {
                component: Component::RDPDR_CTYP_CORE,
                packet_id: PacketId::PAKID_CORE_SERVER_CAPABILITY,
            },
            request: Box::new(ServerCoreCapabilityRequest {
                num_capabilities: 5,
                padding: 0,
                capabilities: vec![
                    CapabilitySet {
                        header: CapabilityHeader {
                            cap_type: CapabilityType::CAP_GENERAL_TYPE,
                            length: 44,
                            version: 2,
                        },
                        data: Capability::General(GeneralCapabilitySet {
                            os_type: 2,
                            os_version: 0,
                            protocol_major_version: 1,
                            protocol_minor_version: 13,
                            io_code_1: 65535,
                            io_code_2: 0,
                            extended_pdu: 7,
                            extra_flags_1: 0,
                            extra_flags_2: 0,
                            special_type_device_cap: 2,
                        }),
                    },
                    CapabilitySet {
                        header: CapabilityHeader {
                            cap_type: CapabilityType::CAP_PRINTER_TYPE,
                            length: 8,
                            version: 1,
                        },
                        data: Capability::Printer,
                    },
                    CapabilitySet {
                        header: CapabilityHeader {
                            cap_type: CapabilityType::CAP_PORT_TYPE,
                            length: 8,
                            version: 1,
                        },
                        data: Capability::Port,
                    },
                    CapabilitySet {
                        header: CapabilityHeader {
                            cap_type: CapabilityType::CAP_DRIVE_TYPE,
                            length: 8,
                            version: 2,
                        },
                        data: Capability::Drive,
                    },
                    CapabilitySet {
                        header: CapabilityHeader {
                            cap_type: CapabilityType::CAP_SMARTCARD_TYPE,
                            length: 8,
                            version: 1,
                        },
                        data: Capability::Smartcard,
                    },
                ],
            }),
            scard_ctl: None,
        },
        vec![(
            PacketId::PAKID_CORE_CLIENT_CAPABILITY,
            Box::new(ClientCoreCapabilityResponse {
                num_capabilities: 3,
                padding: 0,
                capabilities: vec![
                    CapabilitySet {
                        header: CapabilityHeader {
                            cap_type: CapabilityType::CAP_GENERAL_TYPE,
                            length: 44,
                            version: 2,
                        },
                        data: Capability::General(GeneralCapabilitySet {
                            os_type: 0,
                            os_version: 0,
                            protocol_major_version: 1,
                            protocol_minor_version: 12,
                            io_code_1: 32767,
                            io_code_2: 0,
                            extended_pdu: 3,
                            extra_flags_1: 0,
                            extra_flags_2: 0,
                            special_type_device_cap: 1,
                        }),
                    },
                    // TODO(isaiah): These last two capabilities aren't actually getting encoded and sent back.
                    CapabilitySet {
                        header: CapabilityHeader {
                            cap_type: CapabilityType::CAP_SMARTCARD_TYPE,
                            length: 8,
                            version: 1,
                        },
                        data: Capability::Smartcard,
                    },
                    CapabilitySet {
                        header: CapabilityHeader {
                            cap_type: CapabilityType::CAP_DRIVE_TYPE,
                            length: 8,
                            version: 2,
                        },
                        data: Capability::Drive,
                    },
                ],
            }),
        )],
    );
}

fn test_handle_client_id_confirm(c: &mut Client) {
    test_payload_in_to_response_out(
        c,
        PayloadIn {
            channel_pdu_header: ChannelPDUHeader {
                length: 12,
                flags: ChannelPDUFlags::CHANNEL_FLAG_FIRST
                    | ChannelPDUFlags::CHANNEL_FLAG_LAST
                    | ChannelPDUFlags::CHANNEL_FLAG_ONLY,
            },
            shared_header: SharedHeader {
                component: Component::RDPDR_CTYP_CORE,
                packet_id: PacketId::PAKID_CORE_CLIENTID_CONFIRM,
            },
            request: Box::new(ServerClientIdConfirm {
                version_major: 1,
                version_minor: 13,
                client_id: 3,
            }),
            scard_ctl: None,
        },
        vec![(
            PacketId::PAKID_CORE_DEVICELIST_ANNOUNCE,
            Box::new(ClientDeviceListAnnounceRequest {
                device_count: 1,
                device_list: vec![DeviceAnnounceHeader {
                    device_type: DeviceType::RDPDR_DTYP_SMARTCARD,
                    device_id: 1,
                    preferred_dos_name: "SCARD".to_string(),
                    device_data_length: 0,
                    device_data: vec![],
                }],
            }),
        )],
    );
}

fn test_handle_device_reply(c: &mut Client) {
    test_payload_in_to_response_out(
        c,
        PayloadIn {
            channel_pdu_header: ChannelPDUHeader {
                length: 12,
                flags: ChannelPDUFlags::CHANNEL_FLAG_FIRST
                    | ChannelPDUFlags::CHANNEL_FLAG_LAST
                    | ChannelPDUFlags::CHANNEL_FLAG_ONLY,
            },
            shared_header: SharedHeader {
                component: Component::RDPDR_CTYP_CORE,
                packet_id: PacketId::PAKID_CORE_DEVICE_REPLY,
            },
            request: Box::new(ServerDeviceAnnounceResponse {
                device_id: 1,
                result_code: 0,
            }),
            scard_ctl: None,
        },
        vec![],
    );
}

fn test_scard_ioctl_accessstartedevent(c: &mut Client) {
    test_payload_in_to_response_out(
        c,
        PayloadIn {
            channel_pdu_header: ChannelPDUHeader {
                length: 60,
                flags: ChannelPDUFlags::CHANNEL_FLAG_FIRST
                    | ChannelPDUFlags::CHANNEL_FLAG_LAST
                    | ChannelPDUFlags::CHANNEL_FLAG_ONLY,
            },
            shared_header: SharedHeader {
                component: Component::RDPDR_CTYP_CORE,
                packet_id: PacketId::PAKID_CORE_DEVICE_IOREQUEST,
            },
            request: Box::new(DeviceControlRequest {
                header: DeviceIoRequest {
                    device_id: 1,
                    file_id: 1,
                    completion_id: 1,
                    major_function: MajorFunction::IRP_MJ_DEVICE_CONTROL,
                    minor_function: MinorFunction::IRP_MN_NONE,
                },
                output_buffer_length: 256,
                input_buffer_length: 4,
                io_control_code: IoctlCode::SCARD_IOCTL_ACCESSSTARTEDEVENT,
            }),
            scard_ctl: Some(Box::new(ScardAccessStartedEvent_Call {
                _unused: 3234823568,
            })),
        },
        vec![(
            PacketId::PAKID_CORE_DEVICE_IOCOMPLETION,
            Box::new(DeviceControlResponse {
                header: DeviceIoResponse {
                    device_id: 1,
                    completion_id: 1,
                    io_status: 0,
                },
                output_buffer_length: 24,
                output_buffer: vec![
                    1, 16, 8, 0, 204, 204, 204, 204, 8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
                ],
            }),
        )],
    );
}

fn test_scard_ioctl_establishcontext(c: &mut Client, output_buffer: Message) {
    test_payload_in_to_response_out(
        c,
        PayloadIn {
            channel_pdu_header: ChannelPDUHeader {
                length: 80,
                flags: ChannelPDUFlags::CHANNEL_FLAG_FIRST
                    | ChannelPDUFlags::CHANNEL_FLAG_LAST
                    | ChannelPDUFlags::CHANNEL_FLAG_ONLY,
            },
            shared_header: SharedHeader {
                component: Component::RDPDR_CTYP_CORE,
                packet_id: PacketId::PAKID_CORE_DEVICE_IOREQUEST,
            },
            request: Box::new(DeviceControlRequest {
                header: DeviceIoRequest {
                    device_id: 1,
                    file_id: 1,
                    completion_id: 0,
                    major_function: MajorFunction::IRP_MJ_DEVICE_CONTROL,
                    minor_function: MinorFunction::IRP_MN_NONE,
                },
                output_buffer_length: 2048,
                input_buffer_length: 24,
                io_control_code: IoctlCode::SCARD_IOCTL_ESTABLISHCONTEXT,
            }),
            scard_ctl: Some(Box::new(EstablishContext_Call {
                scope: Scope::SCARD_SCOPE_SYSTEM,
            })),
        },
        vec![(
            PacketId::PAKID_CORE_DEVICE_IOCOMPLETION,
            Box::new(DeviceControlResponse {
                header: DeviceIoResponse {
                    device_id: 1,
                    completion_id: 0,
                    io_status: 0,
                },
                output_buffer_length: output_buffer.len() as u32,
                output_buffer,
            }),
        )],
    );
}

fn test_scard_ioctl_listreadersw(c: &mut Client) {
    test_payload_in_to_response_out(
        c,
        PayloadIn {
            channel_pdu_header: ChannelPDUHeader {
                length: 144,
                flags: ChannelPDUFlags::CHANNEL_FLAG_FIRST
                    | ChannelPDUFlags::CHANNEL_FLAG_LAST
                    | ChannelPDUFlags::CHANNEL_FLAG_ONLY,
            },
            shared_header: SharedHeader {
                component: Component::RDPDR_CTYP_CORE,
                packet_id: PacketId::PAKID_CORE_DEVICE_IOREQUEST,
            },
            request: Box::new(DeviceControlRequest {
                header: DeviceIoRequest {
                    device_id: 1,
                    file_id: 1,
                    completion_id: 1,
                    major_function: MajorFunction::IRP_MJ_DEVICE_CONTROL,
                    minor_function: MinorFunction::IRP_MN_NONE,
                },
                output_buffer_length: 2048,
                input_buffer_length: 88,
                io_control_code: IoctlCode::SCARD_IOCTL_LISTREADERSW,
            }),
            scard_ctl: Some(Box::new(ListReaders_Call {
                context: Context::new(1),
                groups_ptr_length: 36,
                groups_length: 36,
                groups_ptr: 131076,
                groups: vec!["SCard$AllReaders".to_string()],
                readers_is_null: false,
                readers_size: 4294967295,
            })),
        },
        vec![(
            PacketId::PAKID_CORE_DEVICE_IOCOMPLETION,
            Box::new(DeviceControlResponse {
                header: DeviceIoResponse {
                    device_id: 1,
                    completion_id: 1,
                    io_status: 0,
                },
                output_buffer_length: 56,
                output_buffer: vec![
                    1, 16, 8, 0, 204, 204, 204, 204, 40, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 20, 0, 0,
                    0, 0, 0, 2, 0, 20, 0, 0, 0, 84, 0, 101, 0, 108, 0, 101, 0, 112, 0, 111, 0, 114,
                    0, 116, 0, 0, 0, 0, 0, 0, 0, 0, 0,
                ],
            }),
        )],
    );
}

fn test_scard_ioctl_getdevicetypeid(c: &mut Client) {
    init_logger();
    test_payload_in_to_response_out(
        c,
        PayloadIn {
            channel_pdu_header: ChannelPDUHeader {
                length: 128,
                flags: ChannelPDUFlags::CHANNEL_FLAG_FIRST
                    | ChannelPDUFlags::CHANNEL_FLAG_LAST
                    | ChannelPDUFlags::CHANNEL_FLAG_ONLY,
            },
            shared_header: SharedHeader {
                component: Component::RDPDR_CTYP_CORE,
                packet_id: PacketId::PAKID_CORE_DEVICE_IOREQUEST,
            },
            request: Box::new(DeviceControlRequest {
                header: DeviceIoRequest {
                    device_id: 1,
                    file_id: 1,
                    completion_id: 1,
                    major_function: MajorFunction::IRP_MJ_DEVICE_CONTROL,
                    minor_function: MinorFunction::IRP_MN_NONE,
                },
                output_buffer_length: 2048,
                input_buffer_length: 72,
                io_control_code: IoctlCode::SCARD_IOCTL_GETDEVICETYPEID,
            }),
            scard_ctl: Some(Box::new(GetDeviceTypeId_Call {
                context: Context {
                    length: 4,
                    value: 2,
                },
                reader_ptr: 131076,
                reader_name: "Teleport".to_string(),
            })),
        },
        vec![(
            PacketId::PAKID_CORE_DEVICE_IOCOMPLETION,
            Box::new(DeviceControlResponse {
                header: DeviceIoResponse {
                    device_id: 1,
                    completion_id: 1,
                    io_status: 0,
                },
                output_buffer_length: 24,
                output_buffer: vec![
                    1, 16, 8, 0, 204, 204, 204, 204, 8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 240, 0, 0,
                    0,
                ],
            }),
        )],
    );
}

#[test]
fn test_smartcard_initialization() {
    let mut c = client();
    test_handle_server_announce(&mut c);
    test_handle_server_capability(&mut c);
    test_handle_client_id_confirm(&mut c);
    test_handle_device_reply(&mut c);
    test_scard_ioctl_accessstartedevent(&mut c);
    test_scard_ioctl_establishcontext(
        &mut c,
        vec![
            1, 16, 8, 0, 204, 204, 204, 204, 24, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 4, 0, 0, 0, 0, 0,
            2, 0, 4, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0,
        ],
    );
    test_scard_ioctl_listreadersw(&mut c);
    test_scard_ioctl_establishcontext(
        &mut c,
        vec![
            1, 16, 8, 0, 204, 204, 204, 204, 24, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 4, 0, 0, 0, 0, 0,
            2, 0, 4, 0, 0, 0, 2, 0, 0, 0, 0, 0, 0, 0,
        ],
    );
    test_scard_ioctl_getdevicetypeid(&mut c);
    // TODO(isaiah): the remainder of the initialization sequence
}
