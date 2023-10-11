from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from typing import ClassVar as _ClassVar

DESCRIPTOR: _descriptor.FileDescriptor

class Theme(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    THEME_UNSPECIFIED: _ClassVar[Theme]
    THEME_LIGHT: _ClassVar[Theme]
    THEME_DARK: _ClassVar[Theme]
THEME_UNSPECIFIED: Theme
THEME_LIGHT: Theme
THEME_DARK: Theme
