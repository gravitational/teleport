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
        CardProtocol, CardStateFlags, Connect_Call, Connect_Common, Context, Context_Call,
        EstablishContext_Call, GetDeviceTypeId_Call, GetStatusChange_Call,
        HCardAndDisposition_Call, Handle, IoctlCode, ListReaders_Call, ReaderState,
        ReaderState_Common_Call, ScardAccessStartedEvent_Call, Scope,
    },
    *,
};
use crate::{
    rdpdr::scard::ContextInternal,
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

fn client(with_scard_id: bool, established_contexts: u32) -> Client {
    let mut c = Client::new(Config {
        cert_der: vec![
            48, 130, 4, 145, 48, 130, 3, 121, 160, 3, 2, 1, 2, 2, 16, 101, 91, 145, 220, 167, 255,
            174, 125, 129, 42, 229, 37, 240, 54, 206, 209, 48, 13, 6, 9, 42, 134, 72, 134, 247, 13,
            1, 1, 11, 5, 0, 48, 122, 49, 34, 48, 32, 6, 3, 85, 4, 10, 19, 25, 73, 115, 97, 105, 97,
            104, 115, 45, 77, 97, 99, 66, 111, 111, 107, 45, 80, 114, 111, 46, 108, 111, 99, 97,
            108, 49, 34, 48, 32, 6, 3, 85, 4, 3, 19, 25, 73, 115, 97, 105, 97, 104, 115, 45, 77,
            97, 99, 66, 111, 111, 107, 45, 80, 114, 111, 46, 108, 111, 99, 97, 108, 49, 48, 48, 46,
            6, 3, 85, 4, 5, 19, 39, 49, 56, 57, 50, 51, 56, 51, 48, 50, 52, 50, 50, 52, 48, 52, 56,
            56, 56, 48, 49, 48, 51, 50, 56, 57, 49, 53, 53, 55, 56, 55, 54, 55, 50, 52, 57, 50, 53,
            51, 48, 30, 23, 13, 50, 50, 48, 57, 49, 54, 50, 50, 52, 48, 50, 50, 90, 23, 13, 50, 50,
            48, 57, 49, 54, 50, 50, 52, 54, 50, 50, 90, 48, 24, 49, 22, 48, 20, 6, 3, 85, 4, 3, 19,
            13, 65, 100, 109, 105, 110, 105, 115, 116, 114, 97, 116, 111, 114, 48, 130, 1, 34, 48,
            13, 6, 9, 42, 134, 72, 134, 247, 13, 1, 1, 1, 5, 0, 3, 130, 1, 15, 0, 48, 130, 1, 10,
            2, 130, 1, 1, 0, 199, 156, 191, 93, 193, 211, 66, 72, 35, 172, 242, 26, 214, 215, 157,
            116, 92, 1, 15, 91, 90, 220, 8, 12, 222, 194, 144, 51, 150, 158, 80, 93, 180, 61, 44,
            203, 4, 79, 26, 241, 6, 39, 87, 146, 182, 216, 119, 78, 236, 182, 90, 87, 89, 91, 148,
            192, 248, 34, 71, 215, 209, 212, 223, 121, 117, 220, 88, 82, 208, 28, 4, 228, 98, 1,
            254, 210, 179, 41, 163, 85, 200, 242, 107, 250, 148, 170, 254, 65, 245, 52, 167, 153,
            176, 216, 157, 111, 133, 40, 74, 66, 214, 165, 219, 238, 119, 160, 44, 172, 24, 244,
            161, 186, 50, 80, 192, 98, 109, 242, 125, 246, 155, 191, 127, 126, 94, 149, 122, 75,
            143, 209, 24, 28, 170, 191, 207, 247, 245, 223, 7, 165, 126, 168, 33, 24, 154, 79, 53,
            63, 70, 153, 113, 212, 114, 95, 198, 238, 12, 101, 239, 217, 79, 184, 129, 146, 25, 34,
            172, 221, 29, 188, 120, 143, 128, 6, 55, 127, 156, 198, 193, 216, 9, 177, 212, 117,
            121, 70, 245, 64, 2, 118, 242, 9, 50, 211, 97, 107, 128, 24, 95, 25, 143, 128, 144, 17,
            183, 83, 111, 168, 30, 188, 34, 121, 150, 43, 58, 134, 200, 95, 34, 43, 219, 64, 97,
            114, 113, 47, 198, 117, 198, 51, 159, 145, 106, 183, 240, 112, 61, 97, 13, 88, 142,
            153, 168, 207, 148, 5, 109, 182, 37, 214, 11, 75, 142, 99, 35, 123, 2, 3, 1, 0, 1, 163,
            130, 1, 115, 48, 130, 1, 111, 48, 14, 6, 3, 85, 29, 15, 1, 1, 255, 4, 4, 3, 2, 7, 128,
            48, 12, 6, 3, 85, 29, 19, 1, 1, 255, 4, 2, 48, 0, 48, 31, 6, 3, 85, 29, 35, 4, 24, 48,
            22, 128, 20, 171, 253, 63, 101, 19, 143, 160, 188, 223, 135, 95, 0, 250, 242, 75, 243,
            201, 83, 122, 126, 48, 129, 213, 6, 3, 85, 29, 31, 4, 129, 205, 48, 129, 202, 48, 129,
            199, 160, 129, 196, 160, 129, 193, 134, 129, 190, 108, 100, 97, 112, 58, 47, 47, 47,
            67, 78, 61, 73, 115, 97, 105, 97, 104, 115, 45, 77, 97, 99, 66, 111, 111, 107, 45, 80,
            114, 111, 46, 108, 111, 99, 97, 108, 44, 67, 78, 61, 84, 101, 108, 101, 112, 111, 114,
            116, 44, 67, 78, 61, 67, 68, 80, 44, 67, 78, 61, 80, 117, 98, 108, 105, 99, 32, 75,
            101, 121, 32, 83, 101, 114, 118, 105, 99, 101, 115, 44, 67, 78, 61, 83, 101, 114, 118,
            105, 99, 101, 115, 44, 67, 78, 61, 67, 111, 110, 102, 105, 103, 117, 114, 97, 116, 105,
            111, 110, 44, 68, 67, 61, 116, 101, 108, 101, 112, 111, 114, 116, 44, 68, 67, 61, 100,
            101, 118, 63, 99, 101, 114, 116, 105, 102, 105, 99, 97, 116, 101, 82, 101, 118, 111,
            99, 97, 116, 105, 111, 110, 76, 105, 115, 116, 63, 98, 97, 115, 101, 63, 111, 98, 106,
            101, 99, 116, 67, 108, 97, 115, 115, 61, 99, 82, 76, 68, 105, 115, 116, 114, 105, 98,
            117, 116, 105, 111, 110, 80, 111, 105, 110, 116, 48, 31, 6, 3, 85, 29, 37, 4, 24, 48,
            22, 6, 8, 43, 6, 1, 5, 5, 7, 3, 2, 6, 10, 43, 6, 1, 4, 1, 130, 55, 20, 2, 2, 48, 53, 6,
            3, 85, 29, 17, 4, 46, 48, 44, 160, 42, 6, 10, 43, 6, 1, 4, 1, 130, 55, 20, 2, 3, 160,
            28, 12, 26, 65, 100, 109, 105, 110, 105, 115, 116, 114, 97, 116, 111, 114, 64, 116,
            101, 108, 101, 112, 111, 114, 116, 46, 100, 101, 118, 48, 13, 6, 9, 42, 134, 72, 134,
            247, 13, 1, 1, 11, 5, 0, 3, 130, 1, 1, 0, 234, 9, 25, 253, 27, 189, 163, 187, 130, 134,
            206, 82, 174, 9, 2, 161, 27, 9, 168, 70, 149, 101, 82, 114, 130, 214, 221, 36, 154,
            248, 94, 46, 133, 193, 52, 223, 80, 99, 111, 208, 95, 93, 86, 70, 215, 77, 176, 77,
            176, 139, 109, 98, 118, 72, 147, 247, 39, 170, 223, 195, 96, 149, 213, 252, 134, 78,
            53, 105, 136, 135, 150, 118, 100, 180, 51, 166, 202, 180, 104, 33, 244, 215, 60, 198,
            255, 142, 20, 228, 86, 30, 229, 181, 70, 19, 201, 97, 46, 139, 161, 90, 253, 178, 149,
            173, 238, 44, 8, 119, 116, 18, 106, 146, 82, 229, 234, 53, 24, 158, 13, 192, 196, 15,
            136, 167, 154, 88, 109, 103, 79, 49, 242, 231, 167, 248, 85, 80, 215, 236, 135, 135,
            129, 4, 192, 88, 150, 94, 60, 134, 224, 219, 176, 228, 200, 82, 101, 209, 195, 36, 181,
            64, 35, 233, 34, 93, 22, 221, 221, 202, 60, 69, 37, 129, 69, 17, 51, 125, 10, 175, 40,
            73, 120, 99, 246, 65, 133, 199, 61, 255, 72, 117, 121, 88, 227, 254, 219, 116, 240,
            248, 220, 146, 222, 241, 229, 53, 179, 146, 57, 149, 151, 113, 63, 122, 27, 14, 159,
            36, 153, 90, 7, 188, 13, 152, 106, 192, 191, 125, 153, 126, 84, 190, 48, 27, 29, 108,
            69, 195, 209, 202, 243, 113, 87, 244, 115, 95, 157, 188, 157, 255, 169, 30, 85, 52,
            175, 44, 118, 255,
        ],
        key_der: vec![
            48, 130, 4, 165, 2, 1, 0, 2, 130, 1, 1, 0, 199, 156, 191, 93, 193, 211, 66, 72, 35,
            172, 242, 26, 214, 215, 157, 116, 92, 1, 15, 91, 90, 220, 8, 12, 222, 194, 144, 51,
            150, 158, 80, 93, 180, 61, 44, 203, 4, 79, 26, 241, 6, 39, 87, 146, 182, 216, 119, 78,
            236, 182, 90, 87, 89, 91, 148, 192, 248, 34, 71, 215, 209, 212, 223, 121, 117, 220, 88,
            82, 208, 28, 4, 228, 98, 1, 254, 210, 179, 41, 163, 85, 200, 242, 107, 250, 148, 170,
            254, 65, 245, 52, 167, 153, 176, 216, 157, 111, 133, 40, 74, 66, 214, 165, 219, 238,
            119, 160, 44, 172, 24, 244, 161, 186, 50, 80, 192, 98, 109, 242, 125, 246, 155, 191,
            127, 126, 94, 149, 122, 75, 143, 209, 24, 28, 170, 191, 207, 247, 245, 223, 7, 165,
            126, 168, 33, 24, 154, 79, 53, 63, 70, 153, 113, 212, 114, 95, 198, 238, 12, 101, 239,
            217, 79, 184, 129, 146, 25, 34, 172, 221, 29, 188, 120, 143, 128, 6, 55, 127, 156, 198,
            193, 216, 9, 177, 212, 117, 121, 70, 245, 64, 2, 118, 242, 9, 50, 211, 97, 107, 128,
            24, 95, 25, 143, 128, 144, 17, 183, 83, 111, 168, 30, 188, 34, 121, 150, 43, 58, 134,
            200, 95, 34, 43, 219, 64, 97, 114, 113, 47, 198, 117, 198, 51, 159, 145, 106, 183, 240,
            112, 61, 97, 13, 88, 142, 153, 168, 207, 148, 5, 109, 182, 37, 214, 11, 75, 142, 99,
            35, 123, 2, 3, 1, 0, 1, 2, 130, 1, 0, 110, 65, 254, 210, 99, 5, 182, 78, 242, 165, 204,
            245, 86, 70, 179, 10, 90, 231, 154, 251, 243, 44, 38, 166, 53, 69, 115, 49, 139, 184,
            214, 219, 107, 123, 127, 10, 132, 206, 205, 42, 229, 35, 70, 20, 28, 59, 101, 107, 139,
            5, 14, 209, 192, 225, 253, 64, 185, 206, 245, 176, 24, 143, 101, 1, 74, 64, 243, 232,
            138, 91, 111, 184, 87, 10, 147, 30, 255, 39, 184, 184, 225, 206, 70, 38, 155, 135, 247,
            249, 166, 223, 246, 211, 198, 3, 96, 179, 0, 242, 72, 82, 179, 13, 218, 117, 214, 77,
            251, 94, 244, 73, 236, 43, 85, 47, 149, 148, 200, 246, 112, 237, 143, 10, 47, 250, 53,
            116, 139, 159, 198, 103, 154, 135, 111, 92, 88, 115, 126, 154, 95, 237, 229, 96, 23,
            57, 137, 244, 122, 61, 178, 14, 243, 187, 157, 7, 103, 183, 26, 252, 46, 33, 214, 70,
            187, 103, 70, 175, 8, 34, 119, 177, 105, 58, 131, 172, 220, 147, 29, 222, 182, 15, 9,
            99, 4, 59, 114, 31, 133, 68, 214, 132, 93, 42, 84, 102, 224, 196, 105, 204, 133, 142,
            228, 170, 112, 177, 23, 144, 68, 127, 16, 33, 156, 6, 131, 53, 143, 48, 142, 161, 218,
            114, 47, 106, 111, 203, 225, 32, 74, 142, 151, 150, 42, 70, 254, 190, 132, 198, 116,
            153, 195, 244, 132, 27, 211, 26, 12, 97, 150, 185, 120, 162, 209, 165, 129, 96, 5, 193,
            2, 129, 129, 0, 248, 205, 132, 65, 11, 44, 236, 203, 74, 170, 87, 190, 88, 35, 35, 66,
            144, 149, 29, 77, 230, 112, 36, 89, 97, 78, 177, 44, 104, 166, 98, 128, 151, 90, 192,
            94, 20, 146, 165, 104, 191, 118, 213, 211, 62, 92, 43, 230, 211, 68, 149, 3, 196, 58,
            77, 232, 81, 255, 185, 21, 42, 6, 52, 180, 196, 152, 138, 73, 200, 252, 187, 175, 87,
            233, 76, 70, 242, 190, 74, 118, 221, 173, 106, 140, 64, 38, 183, 242, 197, 181, 105,
            251, 39, 36, 145, 70, 1, 77, 182, 63, 237, 5, 8, 191, 110, 51, 98, 229, 246, 221, 240,
            151, 116, 57, 162, 23, 41, 194, 194, 224, 154, 134, 131, 191, 247, 150, 114, 143, 2,
            129, 129, 0, 205, 98, 244, 169, 239, 255, 0, 181, 34, 36, 2, 12, 28, 220, 6, 253, 89,
            87, 101, 212, 169, 114, 250, 119, 185, 47, 96, 27, 31, 29, 59, 83, 210, 214, 207, 251,
            182, 27, 10, 142, 57, 91, 162, 253, 144, 194, 138, 82, 244, 241, 225, 211, 196, 31,
            116, 175, 71, 143, 182, 28, 129, 93, 21, 7, 151, 89, 220, 253, 255, 161, 69, 165, 140,
            84, 134, 134, 108, 138, 253, 94, 94, 48, 75, 92, 14, 209, 104, 0, 93, 187, 35, 7, 244,
            233, 18, 67, 189, 63, 98, 165, 188, 220, 109, 254, 85, 105, 254, 152, 249, 160, 48, 14,
            242, 255, 45, 23, 205, 177, 92, 160, 94, 92, 47, 160, 70, 36, 70, 85, 2, 129, 129, 0,
            241, 30, 115, 18, 122, 27, 34, 172, 237, 130, 98, 32, 148, 216, 16, 190, 220, 209, 182,
            33, 157, 182, 134, 115, 156, 139, 31, 199, 50, 240, 52, 187, 252, 114, 181, 197, 55,
            88, 219, 54, 197, 127, 12, 64, 121, 201, 231, 189, 254, 119, 19, 151, 31, 223, 133, 75,
            37, 212, 151, 112, 252, 86, 33, 84, 34, 198, 214, 22, 37, 211, 80, 172, 224, 156, 183,
            16, 119, 5, 149, 178, 214, 168, 206, 126, 119, 89, 78, 161, 215, 155, 53, 199, 113,
            170, 205, 163, 51, 118, 53, 174, 132, 44, 129, 202, 203, 168, 191, 42, 176, 113, 108,
            77, 203, 20, 99, 146, 225, 36, 223, 169, 189, 247, 168, 205, 44, 203, 191, 223, 2, 129,
            129, 0, 146, 46, 77, 63, 42, 142, 207, 189, 28, 8, 142, 224, 122, 37, 236, 95, 163,
            151, 253, 229, 71, 153, 139, 69, 109, 43, 151, 246, 149, 197, 163, 117, 60, 202, 33,
            139, 225, 8, 12, 18, 64, 38, 197, 178, 61, 183, 8, 230, 148, 106, 24, 54, 54, 15, 193,
            104, 3, 193, 248, 118, 255, 103, 245, 208, 202, 91, 110, 91, 229, 246, 173, 240, 111,
            25, 182, 9, 180, 245, 147, 241, 247, 141, 222, 5, 46, 146, 194, 184, 7, 254, 106, 167,
            126, 27, 233, 33, 7, 112, 54, 209, 9, 195, 198, 17, 208, 79, 57, 163, 61, 128, 82, 212,
            65, 5, 119, 221, 202, 75, 227, 70, 77, 2, 197, 239, 8, 29, 71, 101, 2, 129, 129, 0,
            141, 75, 99, 52, 181, 42, 232, 22, 82, 120, 119, 201, 255, 122, 220, 146, 225, 193,
            162, 102, 82, 30, 94, 140, 197, 50, 59, 122, 2, 92, 75, 64, 178, 230, 209, 216, 171,
            40, 172, 143, 128, 77, 160, 241, 130, 40, 205, 123, 241, 181, 38, 13, 24, 215, 218, 53,
            217, 82, 125, 201, 153, 141, 149, 236, 191, 149, 137, 125, 208, 56, 69, 217, 228, 65,
            85, 148, 234, 30, 115, 31, 81, 234, 98, 250, 222, 165, 236, 164, 56, 19, 34, 29, 150,
            172, 118, 228, 179, 91, 26, 208, 186, 161, 49, 218, 225, 211, 204, 48, 207, 193, 226,
            158, 174, 105, 177, 227, 28, 132, 109, 252, 218, 102, 20, 175, 152, 91, 201, 168,
        ],
        pin: "68971585".to_string(),
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
    });

    if with_scard_id {
        c.push_active_device_id(SCARD_DEVICE_ID).unwrap();
    }

    for _ in 0..established_contexts {
        c.scard.contexts.establish();
    }

    c
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

#[test]
fn test_handle_server_announce() {
    let mut c = client(false, 0);
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

#[test]
fn test_handle_server_capability() {
    let mut c = client(false, 0);
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

#[test]
fn test_handle_client_id_confirm() {
    let mut c = client(false, 0);
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

    // Check that we added SCARD_DEVICE_ID to the device id cache
    assert_eq!(c.get_scard_device_id().unwrap(), SCARD_DEVICE_ID);
}

#[test]
fn test_handle_device_reply() {
    let mut c = client(true, 0);
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
                result_code: 0,
            }),
            scard_ctl: None,
        },
        vec![],
    );
}

#[test]
fn test_scard_ioctl_accessstartedevent() {
    let mut c = client(true, 0);
    test_payload_in_to_response_out(
        &mut c,
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

#[test]
fn test_scard_ioctl_establishcontext() {
    let mut c = client(true, 0);

    test_payload_in_to_response_out(
        &mut c,
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
                output_buffer_length: 40,
                output_buffer: vec![
                    1, 16, 8, 0, 204, 204, 204, 204, 24, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 4, 0, 0,
                    0, 0, 0, 2, 0, 4, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0,
                ],
            }),
        )],
    );

    assert_eq!(c.scard.contexts.get(1), Some(&mut ContextInternal::new()));
}

#[test]
fn test_scard_ioctl_listreadersw() {
    let mut c = client(true, 1);
    test_payload_in_to_response_out(
        &mut c,
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

#[test]
fn test_scard_ioctl_getdevicetypeid() {
    let context_value = 2;
    let mut c = client(true, context_value);

    test_payload_in_to_response_out(
        &mut c,
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
                    value: context_value,
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
fn test_scard_ioctl_releasecontext() {
    let context_value = 2;
    let mut c = client(true, context_value);

    test_payload_in_to_response_out(
        &mut c,
        PayloadIn {
            channel_pdu_header: ChannelPDUHeader {
                length: 88,
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
                input_buffer_length: 32,
                io_control_code: IoctlCode::SCARD_IOCTL_RELEASECONTEXT,
            }),
            scard_ctl: Some(Box::new(Context_Call {
                context: Context {
                    length: 4,
                    value: context_value,
                },
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
                output_buffer_length: 24,
                output_buffer: vec![
                    1, 16, 8, 0, 204, 204, 204, 204, 8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
                ],
            }),
        )],
    );

    assert_eq!(c.scard.contexts.get(1), Some(&mut ContextInternal::new()));
    assert_eq!(c.scard.contexts.get(2), None);
}

#[test]
fn test_scard_ioctl_getstatuschangew() {
    let context_value = 1;
    let mut c = client(true, context_value);

    test_payload_in_to_response_out(
        &mut c,
        PayloadIn {
            channel_pdu_header: ChannelPDUHeader {
                length: 296,
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
                input_buffer_length: 240,
                io_control_code: IoctlCode::SCARD_IOCTL_GETSTATUSCHANGEW,
            }),
            scard_ctl: Some(Box::new(GetStatusChange_Call {
                context: Context {
                    length: 4,
                    value: context_value,
                },
                timeout: 4294967295,
                states_ptr_length: 2,
                states_ptr: 131076,
                states_length: 2,
                states: vec![
                    ReaderState {
                        reader: "\\\\?PnP?\\Notification".to_string(),
                        common: ReaderState_Common_Call {
                            current_state: CardStateFlags::SCARD_STATE_UNAWARE,
                            event_state: CardStateFlags::SCARD_STATE_UNAWARE,
                            atr_length: 0,
                            atr: [0; 36],
                        },
                    },
                    ReaderState {
                        reader: "Teleport".to_string(),
                        common: ReaderState_Common_Call {
                            current_state: CardStateFlags::SCARD_STATE_EMPTY,
                            event_state: CardStateFlags::SCARD_STATE_UNAWARE,
                            atr_length: 0,
                            atr: [0; 36],
                        },
                    },
                ],
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
                output_buffer_length: 128,
                output_buffer: vec![
                    1, 16, 8, 0, 204, 204, 204, 204, 112, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 0, 0,
                    0, 0, 0, 2, 0, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
                    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
                    0, 0, 0, 0, 0, 16, 0, 0, 0, 34, 0, 0, 0, 11, 0, 0, 0, 59, 149, 19, 129, 1, 128,
                    115, 255, 1, 0, 11, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
                    0, 0, 0, 0, 0,
                ],
            }),
        )],
    );
}

#[test]
fn test_scard_ioctl_connectw() {
    let context_value = 5;
    let mut c = client(true, context_value);

    test_payload_in_to_response_out(
        &mut c,
        PayloadIn {
            channel_pdu_header: ChannelPDUHeader {
                length: 136,
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
                    completion_id: 2,
                    major_function: MajorFunction::IRP_MJ_DEVICE_CONTROL,
                    minor_function: MinorFunction::IRP_MN_NONE,
                },
                output_buffer_length: 2048,
                input_buffer_length: 80,
                io_control_code: IoctlCode::SCARD_IOCTL_CONNECTW,
            }),
            scard_ctl: Some(Box::new(Connect_Call {
                reader: "Teleport".to_string(),
                common: Connect_Common {
                    context: Context {
                        length: 4,
                        value: context_value,
                    },
                    share_mode: 2,
                    preferred_protocols: CardProtocol::SCARD_PROTOCOL_T0
                        | CardProtocol::SCARD_PROTOCOL_T1
                        | CardProtocol::SCARD_PROTOCOL_TX,
                },
            })),
        },
        vec![(
            PacketId::PAKID_CORE_DEVICE_IOCOMPLETION,
            Box::new(DeviceControlResponse {
                header: DeviceIoResponse {
                    device_id: 1,
                    completion_id: 2,
                    io_status: 0,
                },
                output_buffer_length: 56,
                output_buffer: vec![
                    1, 16, 8, 0, 204, 204, 204, 204, 40, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 4, 0, 0,
                    0, 0, 0, 2, 0, 4, 0, 0, 0, 4, 0, 2, 0, 2, 0, 0, 0, 4, 0, 0, 0, 5, 0, 0, 0, 4,
                    0, 0, 0, 1, 0, 0, 0,
                ],
            }),
        )],
    );
}

#[test]
fn test_scard_ioctl_begintransaction() {
    let context_value = 5;
    let mut c = client(true, context_value);

    test_payload_in_to_response_out(
        &mut c,
        PayloadIn {
            channel_pdu_header: ChannelPDUHeader {
                length: 112,
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
                    completion_id: 2,
                    major_function: MajorFunction::IRP_MJ_DEVICE_CONTROL,
                    minor_function: MinorFunction::IRP_MN_NONE,
                },
                output_buffer_length: 2048,
                input_buffer_length: 56,
                io_control_code: IoctlCode::SCARD_IOCTL_BEGINTRANSACTION,
            }),
            scard_ctl: Some(Box::new(HCardAndDisposition_Call {
                handle: Handle {
                    context: Context {
                        length: 4,
                        value: 5,
                    },
                    length: 4,
                    value: 1,
                },
                disposition: 0,
            })),
        },
        vec![(
            PacketId::PAKID_CORE_DEVICE_IOCOMPLETION,
            Box::new(DeviceControlResponse {
                header: DeviceIoResponse {
                    device_id: 1,
                    completion_id: 2,
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
