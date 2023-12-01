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

use super::flags;
use super::Boolean;

pub const CHANNEL_NAME: &str = "rdpdr";

// Each redirected device requires a unique ID. We only share
// one permanent smartcard device, so we can give it hardcoded ID 1.
pub const DIRECTORY_SHARE_CLIENT_NAME: &str = "teleport";

// Each redirected device requires a unique ID.
pub const SCARD_DEVICE_ID: u32 = 1;

pub const VERSION_MAJOR: u16 = 0x0001;
pub const VERSION_MINOR: u16 = 0x000c;

pub const SMARTCARD_CAPABILITY_VERSION_01: u32 = 0x00000001;
pub const DRIVE_CAPABILITY_VERSION_02: u32 = 0x00000002;
#[allow(dead_code)]
pub const GENERAL_CAPABILITY_VERSION_01: u32 = 0x00000001;
pub const GENERAL_CAPABILITY_VERSION_02: u32 = 0x00000002;

#[derive(Debug, FromPrimitive, ToPrimitive, PartialEq, Eq)]
#[allow(non_camel_case_types)]
pub enum Component {
    RDPDR_CTYP_CORE = 0x4472,
    RDPDR_CTYP_PRN = 0x5052,
}

#[derive(Debug, FromPrimitive, ToPrimitive, PartialEq, Eq)]
#[allow(non_camel_case_types)]
pub enum PacketId {
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

#[derive(Debug, FromPrimitive, ToPrimitive)]
#[allow(non_camel_case_types)]
pub enum CapabilityType {
    CAP_GENERAL_TYPE = 0x0001,
    CAP_PRINTER_TYPE = 0x0002,
    CAP_PORT_TYPE = 0x0003,
    CAP_DRIVE_TYPE = 0x0004,
    CAP_SMARTCARD_TYPE = 0x0005,
}

#[derive(Debug, FromPrimitive, ToPrimitive)]
#[allow(non_camel_case_types)]
pub enum DeviceType {
    RDPDR_DTYP_SERIAL = 0x00000001,
    RDPDR_DTYP_PARALLEL = 0x00000002,
    RDPDR_DTYP_PRINT = 0x00000004,
    RDPDR_DTYP_FILESYSTEM = 0x00000008,
    RDPDR_DTYP_SMARTCARD = 0x00000020,
}

/// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpefs/a087ffa8-d0d5-4874-ac7b-0494f63e2d5d
#[derive(Debug, FromPrimitive, ToPrimitive, PartialEq, Eq, Clone, Copy)]
#[allow(non_camel_case_types)]
pub enum MajorFunction {
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

#[derive(Debug, FromPrimitive, ToPrimitive, Clone, Copy)]
#[allow(non_camel_case_types)]
pub enum MinorFunction {
    IRP_MN_NONE = 0x00000000,
    IRP_MN_QUERY_DIRECTORY = 0x00000001,
    IRP_MN_NOTIFY_CHANGE_DIRECTORY = 0x00000002,
}

/// Windows defines an absolutely massive list of potential NTSTATUS values.
/// This enum includes the basic ones we support for communicating with the windows machine.
/// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-erref/596a1078-e883-4972-9bbc-49e60bebca55
#[derive(ToPrimitive, FromPrimitive, Debug, PartialEq, Eq, Copy, Clone)]
#[repr(u32)]
#[allow(non_camel_case_types)]
#[allow(clippy::upper_case_acronyms)]
#[allow(dead_code)]
pub enum NTSTATUS {
    STATUS_SUCCESS = 0x00000000,
    STATUS_UNSUCCESSFUL = 0xC0000001,
    STATUS_NOT_IMPLEMENTED = 0xC0000002,
    STATUS_NO_MORE_FILES = 0x80000006,
    STATUS_OBJECT_NAME_COLLISION = 0xC0000035,
    STATUS_ACCESS_DENIED = 0xC0000022,
    STATUS_NOT_A_DIRECTORY = 0xC0000103,
    STATUS_NO_SUCH_FILE = 0xC000000F,
    STATUS_NOT_SUPPORTED = 0xC00000BB,
    STATUS_DIRECTORY_NOT_EMPTY = 0xC0000101,
}

/// 2.4 File Information Classes [MS-FSCC]
/// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-fscc/4718fc40-e539-4014-8e33-b675af74e3e1
#[derive(FromPrimitive, Debug, PartialEq, Eq, Clone)]
#[repr(u32)]
#[allow(clippy::enum_variant_names)]
pub enum FileInformationClassLevel {
    FileAccessInformation = 8,
    FileAlignmentInformation = 17,
    FileAllInformation = 18,
    FileAllocationInformation = 19,
    FileAlternateNameInformation = 21,
    FileAttributeTagInformation = 35,
    FileBasicInformation = 4,
    FileBothDirectoryInformation = 3,
    FileCompressionInformation = 28,
    FileDirectoryInformation = 1,
    FileDispositionInformation = 13,
    FileEaInformation = 7,
    FileEndOfFileInformation = 20,
    FileFullDirectoryInformation = 2,
    FileFullEaInformation = 15,
    FileHardLinkInformation = 46,
    FileIdBothDirectoryInformation = 37,
    FileIdExtdDirectoryInformation = 60,
    FileIdFullDirectoryInformation = 38,
    FileIdGlobalTxDirectoryInformation = 50,
    FileIdInformation = 59,
    FileInternalInformation = 6,
    FileLinkInformation = 11,
    FileMailslo = 26,
    FileMailslotSetInformation = 27,
    FileModeInformation = 16,
    FileMoveClusterInformation = 31,
    FileNameInformation = 9,
    FileNamesInformation = 12,
    FileNetworkOpenInformation = 34,
    FileNormalizedNameInformation = 48,
    FileObjectIdInformation = 29,
    FilePipeInformation = 23,
    FilePipInformation = 24,
    FilePipeRemoteInformation = 25,
    FilePositionInformation = 14,
    FileQuotaInformation = 32,
    FileRenameInformation = 10,
    FileReparsePointInformation = 33,
    FileSfioReserveInformation = 44,
    FileSfioVolumeInformation = 45,
    FileShortNameInformation = 40,
    FileStandardInformation = 5,
    FileStandardLinkInformation = 54,
    FileStreamInformation = 22,
    FileTrackingInformation = 36,
    FileValidDataLengthInformation = 39,
}

/// 2.5 File System Information Classes [MS-FSCC]
/// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-fscc/ee12042a-9352-46e3-9f67-c094b75fe6c3
#[derive(FromPrimitive, Debug, PartialEq, Eq)]
#[repr(u32)]
#[allow(clippy::enum_variant_names)]
pub enum FileSystemInformationClassLevel {
    FileFsVolumeInformation = 1,
    FileFsLabelInformation = 2,
    FileFsSizeInformation = 3,
    FileFsDeviceInformation = 4,
    FileFsAttributeInformation = 5,
    FileFsControlInformation = 6,
    FileFsFullSizeInformation = 7,
    FileFsObjectIdInformation = 8,
    FileFsDriverPathInformation = 9,
    FileFsVolumeFlagsInformation = 10,
    FileFsSectorSizeInformation = 11,
}

const fn size_of<T>() -> u32 {
    std::mem::size_of::<T>() as u32
}

pub const U32_SIZE: u32 = size_of::<u32>();
pub const I64_SIZE: u32 = size_of::<i64>();
pub const I8_SIZE: u32 = size_of::<i8>();
pub const U8_SIZE: u32 = size_of::<u8>();
pub const FILE_ATTR_SIZE: u32 = size_of::<flags::FileAttributes>();
pub const BOOL_SIZE: u32 = size_of::<Boolean>();

pub const TDP_FALSE: u8 = 0;
#[allow(dead_code)]
pub const TDP_TRUE: u8 = 1;
