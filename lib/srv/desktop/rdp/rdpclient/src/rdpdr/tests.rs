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

use super::*;
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

fn client(with_scard_id: bool, dir_sharing_enabled: bool) -> Client {
    let mut c = Client::new(Config {
        cert_der: vec![],
        key_der: vec![],
        pin: "".to_string(),
        allow_directory_sharing: dir_sharing_enabled,
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
    });

    if with_scard_id {
        c.push_active_device_id(SCARD_DEVICE_ID).unwrap();
    }

    c
}

struct PayloadIn {
    channel_pdu_header: ChannelPDUHeader,
    shared_header: SharedHeader,
    request: Box<dyn Encode>,
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

#[test]
fn test_handle_server_announce() {
    let mut c = client(false, true);
    test_payload_in_to_response_out(
        &mut c,
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

#[test]
fn test_handle_server_capability() {
    let mut c = client(false, true);
    test_payload_in_to_response_out(
        &mut c,
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

#[test]
fn test_handle_client_id_confirm() {
    let mut c = client(false, true);
    test_payload_in_to_response_out(
        &mut c,
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
        },
        vec![(
            PacketId::PAKID_CORE_DEVICELIST_ANNOUNCE,
            Box::new(ClientDeviceListAnnounceRequest::new_smartcard(
                SCARD_DEVICE_ID,
            )),
        )],
    );

    // Check that we added SCARD_DEVICE_ID to the device id cache
    assert_eq!(c.get_scard_device_id().unwrap(), SCARD_DEVICE_ID);
}

#[test]
fn test_handle_device_reply() {
    let mut c = client(true, true);
    test_payload_in_to_response_out(
        &mut c,
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
                result_code: NTSTATUS::STATUS_SUCCESS,
            }),
        },
        vec![],
    );
}

/// Checks that any of the top level functions related to directory sharing fail with an error
/// if directory sharing is disabled on the client
#[test]
fn check_dir_sharing_methods_error_when_disabled() {
    let mut c = client(true, false);
    let mut results = vec![];

    results.push(
        c.handle_client_device_list_announce(ClientDeviceListAnnounce::new_drive(
            2,
            "test".to_string(),
        )),
    );
    results.push(c.handle_tdp_sd_info_response(SharedDirectoryInfoResponse {
        completion_id: 0,
        err_code: TdpErrCode::Nil,
        fso: FileSystemObject {
            last_modified: 1664500770191,
            size: 9999,
            file_type: FileType::File,
            is_empty: 1,
            path: UnixPath {
                path: "test_file.txt".to_string(),
            },
        },
    }));
    results.push(
        c.handle_tdp_sd_create_response(SharedDirectoryCreateResponse {
            completion_id: 0,
            err_code: TdpErrCode::Nil,
            fso: FileSystemObject {
                last_modified: 1664500770191,
                size: 9999,
                file_type: FileType::File,
                is_empty: 1,
                path: UnixPath {
                    path: "test_file.txt".to_string(),
                },
            },
        }),
    );
    results.push(
        c.handle_tdp_sd_delete_response(SharedDirectoryDeleteResponse {
            completion_id: 0,
            err_code: TdpErrCode::Nil,
        }),
    );
    results.push(c.handle_tdp_sd_list_response(SharedDirectoryListResponse {
        completion_id: 0,
        err_code: TdpErrCode::Nil,
        fso_list: vec![],
    }));
    results.push(c.handle_tdp_sd_read_response(SharedDirectoryReadResponse {
        completion_id: 0,
        err_code: TdpErrCode::Nil,
        read_data: vec![],
    }));
    results.push(
        c.handle_tdp_sd_write_response(SharedDirectoryWriteResponse {
            completion_id: 0,
            err_code: TdpErrCode::Nil,
            bytes_written: 0,
        }),
    );
    results.push(c.handle_tdp_sd_move_response(SharedDirectoryMoveResponse {
        completion_id: 0,
        err_code: TdpErrCode::Nil,
    }));

    for result in results {
        match result {
            Err(err) => match err {
                Error::TryError(s) => {
                    assert_eq!(s, "directory sharing disabled")
                }
                _ => panic!("unexpected error type"),
            },
            Ok(_) => panic!("unexpected success"),
        }
    }
}

/// Checks that we can encode a DeviceAnnounceHeader with a non-ascii name,
/// which was causing a panic in the past.
#[test]
fn test_device_announce_header_encode_with_non_ascii() {
    assert_eq!(
        DeviceAnnounceHeader::new_drive(2, "中文测试".to_string())
            .encode()
            .unwrap(),
        vec![
            8, 0, 0, 0, 2, 0, 0, 0, 70, 73, 76, 69, 0, 0, 0, 0, 13, 0, 0, 0, 228, 184, 173, 230,
            150, 135, 230, 181, 139, 232, 175, 149, 0
        ]
    )
}
