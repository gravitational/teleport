from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

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
