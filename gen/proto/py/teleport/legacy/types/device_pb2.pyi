from gogoproto import gogo_pb2 as _gogo_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from teleport.legacy.types import types_pb2 as _types_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class DeviceV1(_message.Message):
    __slots__ = ["Header", "spec"]
    HEADER_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Header: _types_pb2.ResourceHeader
    spec: DeviceSpec
    def __init__(self, Header: _Optional[_Union[_types_pb2.ResourceHeader, _Mapping]] = ..., spec: _Optional[_Union[DeviceSpec, _Mapping]] = ...) -> None: ...

class DeviceSpec(_message.Message):
    __slots__ = ["os_type", "asset_tag", "create_time", "update_time", "enroll_status", "credential", "collected_data", "source", "profile", "owner"]
    OS_TYPE_FIELD_NUMBER: _ClassVar[int]
    ASSET_TAG_FIELD_NUMBER: _ClassVar[int]
    CREATE_TIME_FIELD_NUMBER: _ClassVar[int]
    UPDATE_TIME_FIELD_NUMBER: _ClassVar[int]
    ENROLL_STATUS_FIELD_NUMBER: _ClassVar[int]
    CREDENTIAL_FIELD_NUMBER: _ClassVar[int]
    COLLECTED_DATA_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    PROFILE_FIELD_NUMBER: _ClassVar[int]
    OWNER_FIELD_NUMBER: _ClassVar[int]
    os_type: str
    asset_tag: str
    create_time: _timestamp_pb2.Timestamp
    update_time: _timestamp_pb2.Timestamp
    enroll_status: str
    credential: DeviceCredential
    collected_data: _containers.RepeatedCompositeFieldContainer[DeviceCollectedData]
    source: DeviceSource
    profile: DeviceProfile
    owner: str
    def __init__(self, os_type: _Optional[str] = ..., asset_tag: _Optional[str] = ..., create_time: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., update_time: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., enroll_status: _Optional[str] = ..., credential: _Optional[_Union[DeviceCredential, _Mapping]] = ..., collected_data: _Optional[_Iterable[_Union[DeviceCollectedData, _Mapping]]] = ..., source: _Optional[_Union[DeviceSource, _Mapping]] = ..., profile: _Optional[_Union[DeviceProfile, _Mapping]] = ..., owner: _Optional[str] = ...) -> None: ...

class DeviceCredential(_message.Message):
    __slots__ = ["id", "public_key_der", "device_attestation_type", "tpm_ekcert_serial", "tpm_ak_public"]
    ID_FIELD_NUMBER: _ClassVar[int]
    PUBLIC_KEY_DER_FIELD_NUMBER: _ClassVar[int]
    DEVICE_ATTESTATION_TYPE_FIELD_NUMBER: _ClassVar[int]
    TPM_EKCERT_SERIAL_FIELD_NUMBER: _ClassVar[int]
    TPM_AK_PUBLIC_FIELD_NUMBER: _ClassVar[int]
    id: str
    public_key_der: bytes
    device_attestation_type: str
    tpm_ekcert_serial: str
    tpm_ak_public: bytes
    def __init__(self, id: _Optional[str] = ..., public_key_der: _Optional[bytes] = ..., device_attestation_type: _Optional[str] = ..., tpm_ekcert_serial: _Optional[str] = ..., tpm_ak_public: _Optional[bytes] = ...) -> None: ...

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
    os_type: str
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
    tpm_platform_attestation: TPMPlatformAttestation
    def __init__(self, collect_time: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., record_time: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., os_type: _Optional[str] = ..., serial_number: _Optional[str] = ..., model_identifier: _Optional[str] = ..., os_version: _Optional[str] = ..., os_build: _Optional[str] = ..., os_username: _Optional[str] = ..., jamf_binary_version: _Optional[str] = ..., macos_enrollment_profiles: _Optional[str] = ..., reported_asset_tag: _Optional[str] = ..., system_serial_number: _Optional[str] = ..., base_board_serial_number: _Optional[str] = ..., tpm_platform_attestation: _Optional[_Union[TPMPlatformAttestation, _Mapping]] = ...) -> None: ...

class TPMPCR(_message.Message):
    __slots__ = ["index", "digest", "digest_alg"]
    INDEX_FIELD_NUMBER: _ClassVar[int]
    DIGEST_FIELD_NUMBER: _ClassVar[int]
    DIGEST_ALG_FIELD_NUMBER: _ClassVar[int]
    index: int
    digest: bytes
    digest_alg: int
    def __init__(self, index: _Optional[int] = ..., digest: _Optional[bytes] = ..., digest_alg: _Optional[int] = ...) -> None: ...

class TPMQuote(_message.Message):
    __slots__ = ["quote", "signature"]
    QUOTE_FIELD_NUMBER: _ClassVar[int]
    SIGNATURE_FIELD_NUMBER: _ClassVar[int]
    quote: bytes
    signature: bytes
    def __init__(self, quote: _Optional[bytes] = ..., signature: _Optional[bytes] = ...) -> None: ...

class TPMPlatformParameters(_message.Message):
    __slots__ = ["quotes", "pcrs", "event_log"]
    QUOTES_FIELD_NUMBER: _ClassVar[int]
    PCRS_FIELD_NUMBER: _ClassVar[int]
    EVENT_LOG_FIELD_NUMBER: _ClassVar[int]
    quotes: _containers.RepeatedCompositeFieldContainer[TPMQuote]
    pcrs: _containers.RepeatedCompositeFieldContainer[TPMPCR]
    event_log: bytes
    def __init__(self, quotes: _Optional[_Iterable[_Union[TPMQuote, _Mapping]]] = ..., pcrs: _Optional[_Iterable[_Union[TPMPCR, _Mapping]]] = ..., event_log: _Optional[bytes] = ...) -> None: ...

class TPMPlatformAttestation(_message.Message):
    __slots__ = ["nonce", "platform_parameters"]
    NONCE_FIELD_NUMBER: _ClassVar[int]
    PLATFORM_PARAMETERS_FIELD_NUMBER: _ClassVar[int]
    nonce: bytes
    platform_parameters: TPMPlatformParameters
    def __init__(self, nonce: _Optional[bytes] = ..., platform_parameters: _Optional[_Union[TPMPlatformParameters, _Mapping]] = ...) -> None: ...

class DeviceSource(_message.Message):
    __slots__ = ["name", "origin"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    ORIGIN_FIELD_NUMBER: _ClassVar[int]
    name: str
    origin: str
    def __init__(self, name: _Optional[str] = ..., origin: _Optional[str] = ...) -> None: ...

class DeviceProfile(_message.Message):
    __slots__ = ["update_time", "model_identifier", "os_version", "os_build", "os_usernames", "jamf_binary_version", "external_id", "os_build_supplemental"]
    UPDATE_TIME_FIELD_NUMBER: _ClassVar[int]
    MODEL_IDENTIFIER_FIELD_NUMBER: _ClassVar[int]
    OS_VERSION_FIELD_NUMBER: _ClassVar[int]
    OS_BUILD_FIELD_NUMBER: _ClassVar[int]
    OS_USERNAMES_FIELD_NUMBER: _ClassVar[int]
    JAMF_BINARY_VERSION_FIELD_NUMBER: _ClassVar[int]
    EXTERNAL_ID_FIELD_NUMBER: _ClassVar[int]
    OS_BUILD_SUPPLEMENTAL_FIELD_NUMBER: _ClassVar[int]
    update_time: _timestamp_pb2.Timestamp
    model_identifier: str
    os_version: str
    os_build: str
    os_usernames: _containers.RepeatedScalarFieldContainer[str]
    jamf_binary_version: str
    external_id: str
    os_build_supplemental: str
    def __init__(self, update_time: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., model_identifier: _Optional[str] = ..., os_version: _Optional[str] = ..., os_build: _Optional[str] = ..., os_usernames: _Optional[_Iterable[str]] = ..., jamf_binary_version: _Optional[str] = ..., external_id: _Optional[str] = ..., os_build_supplemental: _Optional[str] = ...) -> None: ...
