from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Optional as _Optional

DESCRIPTOR: _descriptor.FileDescriptor

class Embedding(_message.Message):
    __slots__ = ["embedded_kind", "embedded_id", "embedded_hash", "vector"]
    EMBEDDED_KIND_FIELD_NUMBER: _ClassVar[int]
    EMBEDDED_ID_FIELD_NUMBER: _ClassVar[int]
    EMBEDDED_HASH_FIELD_NUMBER: _ClassVar[int]
    VECTOR_FIELD_NUMBER: _ClassVar[int]
    embedded_kind: str
    embedded_id: str
    embedded_hash: bytes
    vector: _containers.RepeatedScalarFieldContainer[float]
    def __init__(self, embedded_kind: _Optional[str] = ..., embedded_id: _Optional[str] = ..., embedded_hash: _Optional[bytes] = ..., vector: _Optional[_Iterable[float]] = ...) -> None: ...
