from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class DeviceOrigin(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    DEVICE_ORIGIN_UNSPECIFIED: _ClassVar[DeviceOrigin]
    DEVICE_ORIGIN_API: _ClassVar[DeviceOrigin]
    DEVICE_ORIGIN_JAMF: _ClassVar[DeviceOrigin]
    DEVICE_ORIGIN_INTUNE: _ClassVar[DeviceOrigin]
DEVICE_ORIGIN_UNSPECIFIED: DeviceOrigin
DEVICE_ORIGIN_API: DeviceOrigin
DEVICE_ORIGIN_JAMF: DeviceOrigin
DEVICE_ORIGIN_INTUNE: DeviceOrigin

class DeviceSource(_message.Message):
    __slots__ = ["name", "origin"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    ORIGIN_FIELD_NUMBER: _ClassVar[int]
    name: str
    origin: DeviceOrigin
    def __init__(self, name: _Optional[str] = ..., origin: _Optional[_Union[DeviceOrigin, str]] = ...) -> None: ...
