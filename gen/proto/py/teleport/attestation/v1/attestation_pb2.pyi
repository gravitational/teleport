from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class AttestationStatement(_message.Message):
    __slots__ = ["yubikey_attestation_statement"]
    YUBIKEY_ATTESTATION_STATEMENT_FIELD_NUMBER: _ClassVar[int]
    yubikey_attestation_statement: YubiKeyAttestationStatement
    def __init__(self, yubikey_attestation_statement: _Optional[_Union[YubiKeyAttestationStatement, _Mapping]] = ...) -> None: ...

class YubiKeyAttestationStatement(_message.Message):
    __slots__ = ["slot_cert", "attestation_cert"]
    SLOT_CERT_FIELD_NUMBER: _ClassVar[int]
    ATTESTATION_CERT_FIELD_NUMBER: _ClassVar[int]
    slot_cert: bytes
    attestation_cert: bytes
    def __init__(self, slot_cert: _Optional[bytes] = ..., attestation_cert: _Optional[bytes] = ...) -> None: ...
