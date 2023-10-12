from google.protobuf import empty_pb2 as _empty_pb2
from teleport.legacy.types import types_pb2 as _types_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class GetCertAuthorityRequest(_message.Message):
    __slots__ = ["type", "domain", "include_key"]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    DOMAIN_FIELD_NUMBER: _ClassVar[int]
    INCLUDE_KEY_FIELD_NUMBER: _ClassVar[int]
    type: str
    domain: str
    include_key: bool
    def __init__(self, type: _Optional[str] = ..., domain: _Optional[str] = ..., include_key: bool = ...) -> None: ...

class GetCertAuthoritiesRequest(_message.Message):
    __slots__ = ["type", "include_key"]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    INCLUDE_KEY_FIELD_NUMBER: _ClassVar[int]
    type: str
    include_key: bool
    def __init__(self, type: _Optional[str] = ..., include_key: bool = ...) -> None: ...

class GetCertAuthoritiesResponse(_message.Message):
    __slots__ = ["cert_authorities_v2"]
    CERT_AUTHORITIES_V2_FIELD_NUMBER: _ClassVar[int]
    cert_authorities_v2: _containers.RepeatedCompositeFieldContainer[_types_pb2.CertAuthorityV2]
    def __init__(self, cert_authorities_v2: _Optional[_Iterable[_Union[_types_pb2.CertAuthorityV2, _Mapping]]] = ...) -> None: ...

class DeleteCertAuthorityRequest(_message.Message):
    __slots__ = ["type", "domain"]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    DOMAIN_FIELD_NUMBER: _ClassVar[int]
    type: str
    domain: str
    def __init__(self, type: _Optional[str] = ..., domain: _Optional[str] = ...) -> None: ...

class UpsertCertAuthorityRequest(_message.Message):
    __slots__ = ["cert_authority"]
    CERT_AUTHORITY_FIELD_NUMBER: _ClassVar[int]
    cert_authority: _types_pb2.CertAuthorityV2
    def __init__(self, cert_authority: _Optional[_Union[_types_pb2.CertAuthorityV2, _Mapping]] = ...) -> None: ...
