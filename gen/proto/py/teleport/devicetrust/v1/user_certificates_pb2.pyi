from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Optional as _Optional

DESCRIPTOR: _descriptor.FileDescriptor

class UserCertificates(_message.Message):
    __slots__ = ["x509_der", "ssh_authorized_key"]
    X509_DER_FIELD_NUMBER: _ClassVar[int]
    SSH_AUTHORIZED_KEY_FIELD_NUMBER: _ClassVar[int]
    x509_der: bytes
    ssh_authorized_key: bytes
    def __init__(self, x509_der: _Optional[bytes] = ..., ssh_authorized_key: _Optional[bytes] = ...) -> None: ...
