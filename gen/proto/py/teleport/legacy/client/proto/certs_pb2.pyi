from gogoproto import gogo_pb2 as _gogo_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Optional as _Optional

DESCRIPTOR: _descriptor.FileDescriptor

class Certs(_message.Message):
    __slots__ = ["SSH", "TLS", "TLSCACerts", "SSHCACerts"]
    SSH_FIELD_NUMBER: _ClassVar[int]
    TLS_FIELD_NUMBER: _ClassVar[int]
    TLSCACERTS_FIELD_NUMBER: _ClassVar[int]
    SSHCACERTS_FIELD_NUMBER: _ClassVar[int]
    SSH: bytes
    TLS: bytes
    TLSCACerts: _containers.RepeatedScalarFieldContainer[bytes]
    SSHCACerts: _containers.RepeatedScalarFieldContainer[bytes]
    def __init__(self, SSH: _Optional[bytes] = ..., TLS: _Optional[bytes] = ..., TLSCACerts: _Optional[_Iterable[bytes]] = ..., SSHCACerts: _Optional[_Iterable[bytes]] = ...) -> None: ...
