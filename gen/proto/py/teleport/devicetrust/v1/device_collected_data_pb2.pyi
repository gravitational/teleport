from google.protobuf import timestamp_pb2 as _timestamp_pb2
from teleport.devicetrust.v1 import os_type_pb2 as _os_type_pb2
from teleport.devicetrust.v1 import tpm_pb2 as _tpm_pb2
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class DeviceCollectedData(_message.Message):
    __slots__ = ["collect_time", "record_time", "os_type", "serial_number", "model_identifier", "os_version", "os_build", "os_username", "jamf_binary_version", "macos_enrollment_profiles", "reported_asset_tag", "system_serial_number", "base_board_serial_number", "tpm_platform_attestation"]
    COLLECT_TIME_FIELD_NUMBER: _ClassVar[int]
    RECORD_TIME_FIELD_NUMBER: _ClassVar[int]
    OS_TYPE_FIELD_NUMBER: _ClassVar[int]
    SERIAL_NUMBER_FIELD_NUMBER: _ClassVar[int]
    MODEL_IDENTIFIER_FIELD_NUMBER: _ClassVar[int]
    OS_VERSION_FIELD_NUMBER: _ClassVar[int]
    OS_BUILD_FIELD_NUMBER: _ClassVar[int]
    OS_USERNAME_FIELD_NUMBER: _ClassVar[int]
    JAMF_BINARY_VERSION_FIELD_NUMBER: _ClassVar[int]
    MACOS_ENROLLMENT_PROFILES_FIELD_NUMBER: _ClassVar[int]
    REPORTED_ASSET_TAG_FIELD_NUMBER: _ClassVar[int]
    SYSTEM_SERIAL_NUMBER_FIELD_NUMBER: _ClassVar[int]
    BASE_BOARD_SERIAL_NUMBER_FIELD_NUMBER: _ClassVar[int]
    TPM_PLATFORM_ATTESTATION_FIELD_NUMBER: _ClassVar[int]
    collect_time: _timestamp_pb2.Timestamp
    record_time: _timestamp_pb2.Timestamp
    os_type: _os_type_pb2.OSType
    serial_number: str
    model_identifier: str
    os_version: str
    os_build: str
    os_username: str
    jamf_binary_version: str
    macos_enrollment_profiles: str
    reported_asset_tag: str
    system_serial_number: str
    base_board_serial_number: str
    tpm_platform_attestation: _tpm_pb2.TPMPlatformAttestation
    def __init__(self, collect_time: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., record_time: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., os_type: _Optional[_Union[_os_type_pb2.OSType, str]] = ..., serial_number: _Optional[str] = ..., model_identifier: _Optional[str] = ..., os_version: _Optional[str] = ..., os_build: _Optional[str] = ..., os_username: _Optional[str] = ..., jamf_binary_version: _Optional[str] = ..., macos_enrollment_profiles: _Optional[str] = ..., reported_asset_tag: _Optional[str] = ..., system_serial_number: _Optional[str] = ..., base_board_serial_number: _Optional[str] = ..., tpm_platform_attestation: _Optional[_Union[_tpm_pb2.TPMPlatformAttestation, _Mapping]] = ...) -> None: ...
