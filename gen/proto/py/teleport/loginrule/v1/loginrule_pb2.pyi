from teleport.legacy.types import types_pb2 as _types_pb2
from teleport.legacy.types.wrappers import wrappers_pb2 as _wrappers_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class LoginRule(_message.Message):
    __slots__ = ["metadata", "version", "priority", "traits_map", "traits_expression"]
    class TraitsMapEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: _wrappers_pb2.StringValues
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[_wrappers_pb2.StringValues, _Mapping]] = ...) -> None: ...
    METADATA_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    PRIORITY_FIELD_NUMBER: _ClassVar[int]
    TRAITS_MAP_FIELD_NUMBER: _ClassVar[int]
    TRAITS_EXPRESSION_FIELD_NUMBER: _ClassVar[int]
    metadata: _types_pb2.Metadata
    version: str
    priority: int
    traits_map: _containers.MessageMap[str, _wrappers_pb2.StringValues]
    traits_expression: str
    def __init__(self, metadata: _Optional[_Union[_types_pb2.Metadata, _Mapping]] = ..., version: _Optional[str] = ..., priority: _Optional[int] = ..., traits_map: _Optional[_Mapping[str, _wrappers_pb2.StringValues]] = ..., traits_expression: _Optional[str] = ...) -> None: ...
