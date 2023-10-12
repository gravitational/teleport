from teleport.header.v1 import metadata_pb2 as _metadata_pb2
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ResourceHeader(_message.Message):
    __slots__ = ["kind", "sub_kind", "version", "metadata"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUB_KIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    kind: str
    sub_kind: str
    version: str
    metadata: _metadata_pb2.Metadata
    def __init__(self, kind: _Optional[str] = ..., sub_kind: _Optional[str] = ..., version: _Optional[str] = ..., metadata: _Optional[_Union[_metadata_pb2.Metadata, _Mapping]] = ...) -> None: ...
