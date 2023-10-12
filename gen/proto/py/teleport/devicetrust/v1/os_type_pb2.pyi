from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from typing import ClassVar as _ClassVar

DESCRIPTOR: _descriptor.FileDescriptor

class OSType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    OS_TYPE_UNSPECIFIED: _ClassVar[OSType]
    OS_TYPE_LINUX: _ClassVar[OSType]
    OS_TYPE_MACOS: _ClassVar[OSType]
    OS_TYPE_WINDOWS: _ClassVar[OSType]
OS_TYPE_UNSPECIFIED: OSType
OS_TYPE_LINUX: OSType
OS_TYPE_MACOS: OSType
OS_TYPE_WINDOWS: OSType
