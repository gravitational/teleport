from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Optional as _Optional

DESCRIPTOR: _descriptor.FileDescriptor

class AthenaS3EventPayload(_message.Message):
    __slots__ = ["path", "version_id", "ckms"]
    PATH_FIELD_NUMBER: _ClassVar[int]
    VERSION_ID_FIELD_NUMBER: _ClassVar[int]
    CKMS_FIELD_NUMBER: _ClassVar[int]
    path: str
    version_id: str
    ckms: str
    def __init__(self, path: _Optional[str] = ..., version_id: _Optional[str] = ..., ckms: _Optional[str] = ...) -> None: ...
