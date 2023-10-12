from google.protobuf import empty_pb2 as _empty_pb2
from google.protobuf import field_mask_pb2 as _field_mask_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.rpc import status_pb2 as _status_pb2
from teleport.devicetrust.v1 import device_pb2 as _device_pb2
from teleport.devicetrust.v1 import device_collected_data_pb2 as _device_collected_data_pb2
from teleport.devicetrust.v1 import device_enroll_token_pb2 as _device_enroll_token_pb2
from teleport.devicetrust.v1 import device_source_pb2 as _device_source_pb2
from teleport.devicetrust.v1 import tpm_pb2 as _tpm_pb2
from teleport.devicetrust.v1 import usage_pb2 as _usage_pb2
from teleport.devicetrust.v1 import user_certificates_pb2 as _user_certificates_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class DeviceView(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    DEVICE_VIEW_UNSPECIFIED: _ClassVar[DeviceView]
    DEVICE_VIEW_LIST: _ClassVar[DeviceView]
    DEVICE_VIEW_RESOURCE: _ClassVar[DeviceView]
DEVICE_VIEW_UNSPECIFIED: DeviceView
DEVICE_VIEW_LIST: DeviceView
DEVICE_VIEW_RESOURCE: DeviceView

class CreateDeviceRequest(_message.Message):
    __slots__ = ["device", "create_enroll_token", "create_as_resource", "enroll_token_expire_time"]
    DEVICE_FIELD_NUMBER: _ClassVar[int]
    CREATE_ENROLL_TOKEN_FIELD_NUMBER: _ClassVar[int]
    CREATE_AS_RESOURCE_FIELD_NUMBER: _ClassVar[int]
    ENROLL_TOKEN_EXPIRE_TIME_FIELD_NUMBER: _ClassVar[int]
    device: _device_pb2.Device
    create_enroll_token: bool
    create_as_resource: bool
    enroll_token_expire_time: _timestamp_pb2.Timestamp
    def __init__(self, device: _Optional[_Union[_device_pb2.Device, _Mapping]] = ..., create_enroll_token: bool = ..., create_as_resource: bool = ..., enroll_token_expire_time: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class UpdateDeviceRequest(_message.Message):
    __slots__ = ["device", "update_mask"]
    DEVICE_FIELD_NUMBER: _ClassVar[int]
    UPDATE_MASK_FIELD_NUMBER: _ClassVar[int]
    device: _device_pb2.Device
    update_mask: _field_mask_pb2.FieldMask
    def __init__(self, device: _Optional[_Union[_device_pb2.Device, _Mapping]] = ..., update_mask: _Optional[_Union[_field_mask_pb2.FieldMask, _Mapping]] = ...) -> None: ...

class UpsertDeviceRequest(_message.Message):
    __slots__ = ["device", "create_as_resource"]
    DEVICE_FIELD_NUMBER: _ClassVar[int]
    CREATE_AS_RESOURCE_FIELD_NUMBER: _ClassVar[int]
    device: _device_pb2.Device
    create_as_resource: bool
    def __init__(self, device: _Optional[_Union[_device_pb2.Device, _Mapping]] = ..., create_as_resource: bool = ...) -> None: ...

class DeleteDeviceRequest(_message.Message):
    __slots__ = ["device_id"]
    DEVICE_ID_FIELD_NUMBER: _ClassVar[int]
    device_id: str
    def __init__(self, device_id: _Optional[str] = ...) -> None: ...

class FindDevicesRequest(_message.Message):
    __slots__ = ["id_or_tag"]
    ID_OR_TAG_FIELD_NUMBER: _ClassVar[int]
    id_or_tag: str
    def __init__(self, id_or_tag: _Optional[str] = ...) -> None: ...

class FindDevicesResponse(_message.Message):
    __slots__ = ["devices"]
    DEVICES_FIELD_NUMBER: _ClassVar[int]
    devices: _containers.RepeatedCompositeFieldContainer[_device_pb2.Device]
    def __init__(self, devices: _Optional[_Iterable[_Union[_device_pb2.Device, _Mapping]]] = ...) -> None: ...

class GetDeviceRequest(_message.Message):
    __slots__ = ["device_id"]
    DEVICE_ID_FIELD_NUMBER: _ClassVar[int]
    device_id: str
    def __init__(self, device_id: _Optional[str] = ...) -> None: ...

class ListDevicesRequest(_message.Message):
    __slots__ = ["page_size", "page_token", "view"]
    PAGE_SIZE_FIELD_NUMBER: _ClassVar[int]
    PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    VIEW_FIELD_NUMBER: _ClassVar[int]
    page_size: int
    page_token: str
    view: DeviceView
    def __init__(self, page_size: _Optional[int] = ..., page_token: _Optional[str] = ..., view: _Optional[_Union[DeviceView, str]] = ...) -> None: ...

class ListDevicesResponse(_message.Message):
    __slots__ = ["devices", "next_page_token"]
    DEVICES_FIELD_NUMBER: _ClassVar[int]
    NEXT_PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    devices: _containers.RepeatedCompositeFieldContainer[_device_pb2.Device]
    next_page_token: str
    def __init__(self, devices: _Optional[_Iterable[_Union[_device_pb2.Device, _Mapping]]] = ..., next_page_token: _Optional[str] = ...) -> None: ...

class BulkCreateDevicesRequest(_message.Message):
    __slots__ = ["devices", "create_as_resource"]
    DEVICES_FIELD_NUMBER: _ClassVar[int]
    CREATE_AS_RESOURCE_FIELD_NUMBER: _ClassVar[int]
    devices: _containers.RepeatedCompositeFieldContainer[_device_pb2.Device]
    create_as_resource: bool
    def __init__(self, devices: _Optional[_Iterable[_Union[_device_pb2.Device, _Mapping]]] = ..., create_as_resource: bool = ...) -> None: ...

class BulkCreateDevicesResponse(_message.Message):
    __slots__ = ["devices"]
    DEVICES_FIELD_NUMBER: _ClassVar[int]
    devices: _containers.RepeatedCompositeFieldContainer[DeviceOrStatus]
    def __init__(self, devices: _Optional[_Iterable[_Union[DeviceOrStatus, _Mapping]]] = ...) -> None: ...

class DeviceOrStatus(_message.Message):
    __slots__ = ["status", "id", "deleted"]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    ID_FIELD_NUMBER: _ClassVar[int]
    DELETED_FIELD_NUMBER: _ClassVar[int]
    status: _status_pb2.Status
    id: str
    deleted: bool
    def __init__(self, status: _Optional[_Union[_status_pb2.Status, _Mapping]] = ..., id: _Optional[str] = ..., deleted: bool = ...) -> None: ...

class CreateDeviceEnrollTokenRequest(_message.Message):
    __slots__ = ["device_id", "device_data", "expire_time"]
    DEVICE_ID_FIELD_NUMBER: _ClassVar[int]
    DEVICE_DATA_FIELD_NUMBER: _ClassVar[int]
    EXPIRE_TIME_FIELD_NUMBER: _ClassVar[int]
    device_id: str
    device_data: _device_collected_data_pb2.DeviceCollectedData
    expire_time: _timestamp_pb2.Timestamp
    def __init__(self, device_id: _Optional[str] = ..., device_data: _Optional[_Union[_device_collected_data_pb2.DeviceCollectedData, _Mapping]] = ..., expire_time: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class EnrollDeviceRequest(_message.Message):
    __slots__ = ["init", "macos_challenge_response", "tpm_challenge_response"]
    INIT_FIELD_NUMBER: _ClassVar[int]
    MACOS_CHALLENGE_RESPONSE_FIELD_NUMBER: _ClassVar[int]
    TPM_CHALLENGE_RESPONSE_FIELD_NUMBER: _ClassVar[int]
    init: EnrollDeviceInit
    macos_challenge_response: MacOSEnrollChallengeResponse
    tpm_challenge_response: TPMEnrollChallengeResponse
    def __init__(self, init: _Optional[_Union[EnrollDeviceInit, _Mapping]] = ..., macos_challenge_response: _Optional[_Union[MacOSEnrollChallengeResponse, _Mapping]] = ..., tpm_challenge_response: _Optional[_Union[TPMEnrollChallengeResponse, _Mapping]] = ...) -> None: ...

class EnrollDeviceResponse(_message.Message):
    __slots__ = ["success", "macos_challenge", "tpm_challenge"]
    SUCCESS_FIELD_NUMBER: _ClassVar[int]
    MACOS_CHALLENGE_FIELD_NUMBER: _ClassVar[int]
    TPM_CHALLENGE_FIELD_NUMBER: _ClassVar[int]
    success: EnrollDeviceSuccess
    macos_challenge: MacOSEnrollChallenge
    tpm_challenge: TPMEnrollChallenge
    def __init__(self, success: _Optional[_Union[EnrollDeviceSuccess, _Mapping]] = ..., macos_challenge: _Optional[_Union[MacOSEnrollChallenge, _Mapping]] = ..., tpm_challenge: _Optional[_Union[TPMEnrollChallenge, _Mapping]] = ...) -> None: ...

class EnrollDeviceInit(_message.Message):
    __slots__ = ["token", "credential_id", "device_data", "macos", "tpm"]
    TOKEN_FIELD_NUMBER: _ClassVar[int]
    CREDENTIAL_ID_FIELD_NUMBER: _ClassVar[int]
    DEVICE_DATA_FIELD_NUMBER: _ClassVar[int]
    MACOS_FIELD_NUMBER: _ClassVar[int]
    TPM_FIELD_NUMBER: _ClassVar[int]
    token: str
    credential_id: str
    device_data: _device_collected_data_pb2.DeviceCollectedData
    macos: MacOSEnrollPayload
    tpm: TPMEnrollPayload
    def __init__(self, token: _Optional[str] = ..., credential_id: _Optional[str] = ..., device_data: _Optional[_Union[_device_collected_data_pb2.DeviceCollectedData, _Mapping]] = ..., macos: _Optional[_Union[MacOSEnrollPayload, _Mapping]] = ..., tpm: _Optional[_Union[TPMEnrollPayload, _Mapping]] = ...) -> None: ...

class EnrollDeviceSuccess(_message.Message):
    __slots__ = ["device"]
    DEVICE_FIELD_NUMBER: _ClassVar[int]
    device: _device_pb2.Device
    def __init__(self, device: _Optional[_Union[_device_pb2.Device, _Mapping]] = ...) -> None: ...

class MacOSEnrollPayload(_message.Message):
    __slots__ = ["public_key_der"]
    PUBLIC_KEY_DER_FIELD_NUMBER: _ClassVar[int]
    public_key_der: bytes
    def __init__(self, public_key_der: _Optional[bytes] = ...) -> None: ...

class MacOSEnrollChallenge(_message.Message):
    __slots__ = ["challenge"]
    CHALLENGE_FIELD_NUMBER: _ClassVar[int]
    challenge: bytes
    def __init__(self, challenge: _Optional[bytes] = ...) -> None: ...

class MacOSEnrollChallengeResponse(_message.Message):
    __slots__ = ["signature"]
    SIGNATURE_FIELD_NUMBER: _ClassVar[int]
    signature: bytes
    def __init__(self, signature: _Optional[bytes] = ...) -> None: ...

class TPMEnrollPayload(_message.Message):
    __slots__ = ["ek_cert", "ek_key", "attestation_parameters"]
    EK_CERT_FIELD_NUMBER: _ClassVar[int]
    EK_KEY_FIELD_NUMBER: _ClassVar[int]
    ATTESTATION_PARAMETERS_FIELD_NUMBER: _ClassVar[int]
    ek_cert: bytes
    ek_key: bytes
    attestation_parameters: TPMAttestationParameters
    def __init__(self, ek_cert: _Optional[bytes] = ..., ek_key: _Optional[bytes] = ..., attestation_parameters: _Optional[_Union[TPMAttestationParameters, _Mapping]] = ...) -> None: ...

class TPMAttestationParameters(_message.Message):
    __slots__ = ["public", "create_data", "create_attestation", "create_signature"]
    PUBLIC_FIELD_NUMBER: _ClassVar[int]
    CREATE_DATA_FIELD_NUMBER: _ClassVar[int]
    CREATE_ATTESTATION_FIELD_NUMBER: _ClassVar[int]
    CREATE_SIGNATURE_FIELD_NUMBER: _ClassVar[int]
    public: bytes
    create_data: bytes
    create_attestation: bytes
    create_signature: bytes
    def __init__(self, public: _Optional[bytes] = ..., create_data: _Optional[bytes] = ..., create_attestation: _Optional[bytes] = ..., create_signature: _Optional[bytes] = ...) -> None: ...

class TPMEnrollChallenge(_message.Message):
    __slots__ = ["encrypted_credential", "attestation_nonce"]
    ENCRYPTED_CREDENTIAL_FIELD_NUMBER: _ClassVar[int]
    ATTESTATION_NONCE_FIELD_NUMBER: _ClassVar[int]
    encrypted_credential: TPMEncryptedCredential
    attestation_nonce: bytes
    def __init__(self, encrypted_credential: _Optional[_Union[TPMEncryptedCredential, _Mapping]] = ..., attestation_nonce: _Optional[bytes] = ...) -> None: ...

class TPMEncryptedCredential(_message.Message):
    __slots__ = ["credential_blob", "secret"]
    CREDENTIAL_BLOB_FIELD_NUMBER: _ClassVar[int]
    SECRET_FIELD_NUMBER: _ClassVar[int]
    credential_blob: bytes
    secret: bytes
    def __init__(self, credential_blob: _Optional[bytes] = ..., secret: _Optional[bytes] = ...) -> None: ...

class TPMEnrollChallengeResponse(_message.Message):
    __slots__ = ["solution", "platform_parameters"]
    SOLUTION_FIELD_NUMBER: _ClassVar[int]
    PLATFORM_PARAMETERS_FIELD_NUMBER: _ClassVar[int]
    solution: bytes
    platform_parameters: _tpm_pb2.TPMPlatformParameters
    def __init__(self, solution: _Optional[bytes] = ..., platform_parameters: _Optional[_Union[_tpm_pb2.TPMPlatformParameters, _Mapping]] = ...) -> None: ...

class AuthenticateDeviceRequest(_message.Message):
    __slots__ = ["init", "challenge_response", "tpm_challenge_response"]
    INIT_FIELD_NUMBER: _ClassVar[int]
    CHALLENGE_RESPONSE_FIELD_NUMBER: _ClassVar[int]
    TPM_CHALLENGE_RESPONSE_FIELD_NUMBER: _ClassVar[int]
    init: AuthenticateDeviceInit
    challenge_response: AuthenticateDeviceChallengeResponse
    tpm_challenge_response: TPMAuthenticateDeviceChallengeResponse
    def __init__(self, init: _Optional[_Union[AuthenticateDeviceInit, _Mapping]] = ..., challenge_response: _Optional[_Union[AuthenticateDeviceChallengeResponse, _Mapping]] = ..., tpm_challenge_response: _Optional[_Union[TPMAuthenticateDeviceChallengeResponse, _Mapping]] = ...) -> None: ...

class AuthenticateDeviceResponse(_message.Message):
    __slots__ = ["challenge", "user_certificates", "tpm_challenge"]
    CHALLENGE_FIELD_NUMBER: _ClassVar[int]
    USER_CERTIFICATES_FIELD_NUMBER: _ClassVar[int]
    TPM_CHALLENGE_FIELD_NUMBER: _ClassVar[int]
    challenge: AuthenticateDeviceChallenge
    user_certificates: _user_certificates_pb2.UserCertificates
    tpm_challenge: TPMAuthenticateDeviceChallenge
    def __init__(self, challenge: _Optional[_Union[AuthenticateDeviceChallenge, _Mapping]] = ..., user_certificates: _Optional[_Union[_user_certificates_pb2.UserCertificates, _Mapping]] = ..., tpm_challenge: _Optional[_Union[TPMAuthenticateDeviceChallenge, _Mapping]] = ...) -> None: ...

class AuthenticateDeviceInit(_message.Message):
    __slots__ = ["user_certificates", "credential_id", "device_data"]
    USER_CERTIFICATES_FIELD_NUMBER: _ClassVar[int]
    CREDENTIAL_ID_FIELD_NUMBER: _ClassVar[int]
    DEVICE_DATA_FIELD_NUMBER: _ClassVar[int]
    user_certificates: _user_certificates_pb2.UserCertificates
    credential_id: str
    device_data: _device_collected_data_pb2.DeviceCollectedData
    def __init__(self, user_certificates: _Optional[_Union[_user_certificates_pb2.UserCertificates, _Mapping]] = ..., credential_id: _Optional[str] = ..., device_data: _Optional[_Union[_device_collected_data_pb2.DeviceCollectedData, _Mapping]] = ...) -> None: ...

class TPMAuthenticateDeviceChallenge(_message.Message):
    __slots__ = ["attestation_nonce"]
    ATTESTATION_NONCE_FIELD_NUMBER: _ClassVar[int]
    attestation_nonce: bytes
    def __init__(self, attestation_nonce: _Optional[bytes] = ...) -> None: ...

class TPMAuthenticateDeviceChallengeResponse(_message.Message):
    __slots__ = ["platform_parameters"]
    PLATFORM_PARAMETERS_FIELD_NUMBER: _ClassVar[int]
    platform_parameters: _tpm_pb2.TPMPlatformParameters
    def __init__(self, platform_parameters: _Optional[_Union[_tpm_pb2.TPMPlatformParameters, _Mapping]] = ...) -> None: ...

class AuthenticateDeviceChallenge(_message.Message):
    __slots__ = ["challenge"]
    CHALLENGE_FIELD_NUMBER: _ClassVar[int]
    challenge: bytes
    def __init__(self, challenge: _Optional[bytes] = ...) -> None: ...

class AuthenticateDeviceChallengeResponse(_message.Message):
    __slots__ = ["signature"]
    SIGNATURE_FIELD_NUMBER: _ClassVar[int]
    signature: bytes
    def __init__(self, signature: _Optional[bytes] = ...) -> None: ...

class SyncInventoryRequest(_message.Message):
    __slots__ = ["start", "end", "devices_to_upsert", "devices_to_remove"]
    START_FIELD_NUMBER: _ClassVar[int]
    END_FIELD_NUMBER: _ClassVar[int]
    DEVICES_TO_UPSERT_FIELD_NUMBER: _ClassVar[int]
    DEVICES_TO_REMOVE_FIELD_NUMBER: _ClassVar[int]
    start: SyncInventoryStart
    end: SyncInventoryEnd
    devices_to_upsert: SyncInventoryDevices
    devices_to_remove: SyncInventoryDevices
    def __init__(self, start: _Optional[_Union[SyncInventoryStart, _Mapping]] = ..., end: _Optional[_Union[SyncInventoryEnd, _Mapping]] = ..., devices_to_upsert: _Optional[_Union[SyncInventoryDevices, _Mapping]] = ..., devices_to_remove: _Optional[_Union[SyncInventoryDevices, _Mapping]] = ...) -> None: ...

class SyncInventoryResponse(_message.Message):
    __slots__ = ["ack", "result", "missing_devices"]
    ACK_FIELD_NUMBER: _ClassVar[int]
    RESULT_FIELD_NUMBER: _ClassVar[int]
    MISSING_DEVICES_FIELD_NUMBER: _ClassVar[int]
    ack: SyncInventoryAck
    result: SyncInventoryResult
    missing_devices: SyncInventoryMissingDevices
    def __init__(self, ack: _Optional[_Union[SyncInventoryAck, _Mapping]] = ..., result: _Optional[_Union[SyncInventoryResult, _Mapping]] = ..., missing_devices: _Optional[_Union[SyncInventoryMissingDevices, _Mapping]] = ...) -> None: ...

class SyncInventoryStart(_message.Message):
    __slots__ = ["source", "track_missing_devices"]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    TRACK_MISSING_DEVICES_FIELD_NUMBER: _ClassVar[int]
    source: _device_source_pb2.DeviceSource
    track_missing_devices: bool
    def __init__(self, source: _Optional[_Union[_device_source_pb2.DeviceSource, _Mapping]] = ..., track_missing_devices: bool = ...) -> None: ...

class SyncInventoryEnd(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class SyncInventoryDevices(_message.Message):
    __slots__ = ["devices"]
    DEVICES_FIELD_NUMBER: _ClassVar[int]
    devices: _containers.RepeatedCompositeFieldContainer[_device_pb2.Device]
    def __init__(self, devices: _Optional[_Iterable[_Union[_device_pb2.Device, _Mapping]]] = ...) -> None: ...

class SyncInventoryAck(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class SyncInventoryResult(_message.Message):
    __slots__ = ["devices"]
    DEVICES_FIELD_NUMBER: _ClassVar[int]
    devices: _containers.RepeatedCompositeFieldContainer[DeviceOrStatus]
    def __init__(self, devices: _Optional[_Iterable[_Union[DeviceOrStatus, _Mapping]]] = ...) -> None: ...

class SyncInventoryMissingDevices(_message.Message):
    __slots__ = ["devices"]
    DEVICES_FIELD_NUMBER: _ClassVar[int]
    devices: _containers.RepeatedCompositeFieldContainer[_device_pb2.Device]
    def __init__(self, devices: _Optional[_Iterable[_Union[_device_pb2.Device, _Mapping]]] = ...) -> None: ...

class GetDevicesUsageRequest(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...
