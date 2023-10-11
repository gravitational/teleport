from google.protobuf import timestamp_pb2 as _timestamp_pb2
from teleport.devicetrust.v1 import device_collected_data_pb2 as _device_collected_data_pb2
from teleport.devicetrust.v1 import device_enroll_token_pb2 as _device_enroll_token_pb2
from teleport.devicetrust.v1 import device_profile_pb2 as _device_profile_pb2
from teleport.devicetrust.v1 import device_source_pb2 as _device_source_pb2
from teleport.devicetrust.v1 import os_type_pb2 as _os_type_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class DeviceAttestationType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    DEVICE_ATTESTATION_TYPE_UNSPECIFIED: _ClassVar[DeviceAttestationType]
    DEVICE_ATTESTATION_TYPE_TPM_EKPUB: _ClassVar[DeviceAttestationType]
    DEVICE_ATTESTATION_TYPE_TPM_EKCERT: _ClassVar[DeviceAttestationType]
    DEVICE_ATTESTATION_TYPE_TPM_EKCERT_TRUSTED: _ClassVar[DeviceAttestationType]

class DeviceEnrollStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    DEVICE_ENROLL_STATUS_UNSPECIFIED: _ClassVar[DeviceEnrollStatus]
    DEVICE_ENROLL_STATUS_NOT_ENROLLED: _ClassVar[DeviceEnrollStatus]
    DEVICE_ENROLL_STATUS_ENROLLED: _ClassVar[DeviceEnrollStatus]
DEVICE_ATTESTATION_TYPE_UNSPECIFIED: DeviceAttestationType
DEVICE_ATTESTATION_TYPE_TPM_EKPUB: DeviceAttestationType
DEVICE_ATTESTATION_TYPE_TPM_EKCERT: DeviceAttestationType
DEVICE_ATTESTATION_TYPE_TPM_EKCERT_TRUSTED: DeviceAttestationType
DEVICE_ENROLL_STATUS_UNSPECIFIED: DeviceEnrollStatus
DEVICE_ENROLL_STATUS_NOT_ENROLLED: DeviceEnrollStatus
DEVICE_ENROLL_STATUS_ENROLLED: DeviceEnrollStatus

class Device(_message.Message):
    __slots__ = ["api_version", "id", "os_type", "asset_tag", "create_time", "update_time", "enroll_token", "enroll_status", "credential", "collected_data", "source", "profile", "owner"]
    API_VERSION_FIELD_NUMBER: _ClassVar[int]
    ID_FIELD_NUMBER: _ClassVar[int]
    OS_TYPE_FIELD_NUMBER: _ClassVar[int]
    ASSET_TAG_FIELD_NUMBER: _ClassVar[int]
    CREATE_TIME_FIELD_NUMBER: _ClassVar[int]
    UPDATE_TIME_FIELD_NUMBER: _ClassVar[int]
    ENROLL_TOKEN_FIELD_NUMBER: _ClassVar[int]
    ENROLL_STATUS_FIELD_NUMBER: _ClassVar[int]
    CREDENTIAL_FIELD_NUMBER: _ClassVar[int]
    COLLECTED_DATA_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    PROFILE_FIELD_NUMBER: _ClassVar[int]
    OWNER_FIELD_NUMBER: _ClassVar[int]
    api_version: str
    id: str
    os_type: _os_type_pb2.OSType
    asset_tag: str
    create_time: _timestamp_pb2.Timestamp
    update_time: _timestamp_pb2.Timestamp
    enroll_token: _device_enroll_token_pb2.DeviceEnrollToken
    enroll_status: DeviceEnrollStatus
    credential: DeviceCredential
    collected_data: _containers.RepeatedCompositeFieldContainer[_device_collected_data_pb2.DeviceCollectedData]
    source: _device_source_pb2.DeviceSource
    profile: _device_profile_pb2.DeviceProfile
    owner: str
    def __init__(self, api_version: _Optional[str] = ..., id: _Optional[str] = ..., os_type: _Optional[_Union[_os_type_pb2.OSType, str]] = ..., asset_tag: _Optional[str] = ..., create_time: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., update_time: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., enroll_token: _Optional[_Union[_device_enroll_token_pb2.DeviceEnrollToken, _Mapping]] = ..., enroll_status: _Optional[_Union[DeviceEnrollStatus, str]] = ..., credential: _Optional[_Union[DeviceCredential, _Mapping]] = ..., collected_data: _Optional[_Iterable[_Union[_device_collected_data_pb2.DeviceCollectedData, _Mapping]]] = ..., source: _Optional[_Union[_device_source_pb2.DeviceSource, _Mapping]] = ..., profile: _Optional[_Union[_device_profile_pb2.DeviceProfile, _Mapping]] = ..., owner: _Optional[str] = ...) -> None: ...

class DeviceCredential(_message.Message):
    __slots__ = ["id", "public_key_der", "device_attestation_type", "tpm_ekcert_serial", "tpm_ak_public"]
    ID_FIELD_NUMBER: _ClassVar[int]
    PUBLIC_KEY_DER_FIELD_NUMBER: _ClassVar[int]
    DEVICE_ATTESTATION_TYPE_FIELD_NUMBER: _ClassVar[int]
    TPM_EKCERT_SERIAL_FIELD_NUMBER: _ClassVar[int]
    TPM_AK_PUBLIC_FIELD_NUMBER: _ClassVar[int]
    id: str
    public_key_der: bytes
    device_attestation_type: DeviceAttestationType
    tpm_ekcert_serial: str
    tpm_ak_public: bytes
    def __init__(self, id: _Optional[str] = ..., public_key_der: _Optional[bytes] = ..., device_attestation_type: _Optional[_Union[DeviceAttestationType, str]] = ..., tpm_ekcert_serial: _Optional[str] = ..., tpm_ak_public: _Optional[bytes] = ...) -> None: ...
