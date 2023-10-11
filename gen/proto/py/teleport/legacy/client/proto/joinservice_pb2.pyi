from teleport.legacy.client.proto import certs_pb2 as _certs_pb2
from teleport.legacy.types import types_pb2 as _types_pb2
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class RegisterUsingIAMMethodRequest(_message.Message):
    __slots__ = ["register_using_token_request", "sts_identity_request"]
    REGISTER_USING_TOKEN_REQUEST_FIELD_NUMBER: _ClassVar[int]
    STS_IDENTITY_REQUEST_FIELD_NUMBER: _ClassVar[int]
    register_using_token_request: _types_pb2.RegisterUsingTokenRequest
    sts_identity_request: bytes
    def __init__(self, register_using_token_request: _Optional[_Union[_types_pb2.RegisterUsingTokenRequest, _Mapping]] = ..., sts_identity_request: _Optional[bytes] = ...) -> None: ...

class RegisterUsingIAMMethodResponse(_message.Message):
    __slots__ = ["challenge", "certs"]
    CHALLENGE_FIELD_NUMBER: _ClassVar[int]
    CERTS_FIELD_NUMBER: _ClassVar[int]
    challenge: str
    certs: _certs_pb2.Certs
    def __init__(self, challenge: _Optional[str] = ..., certs: _Optional[_Union[_certs_pb2.Certs, _Mapping]] = ...) -> None: ...

class RegisterUsingAzureMethodRequest(_message.Message):
    __slots__ = ["register_using_token_request", "attested_data", "access_token"]
    REGISTER_USING_TOKEN_REQUEST_FIELD_NUMBER: _ClassVar[int]
    ATTESTED_DATA_FIELD_NUMBER: _ClassVar[int]
    ACCESS_TOKEN_FIELD_NUMBER: _ClassVar[int]
    register_using_token_request: _types_pb2.RegisterUsingTokenRequest
    attested_data: bytes
    access_token: str
    def __init__(self, register_using_token_request: _Optional[_Union[_types_pb2.RegisterUsingTokenRequest, _Mapping]] = ..., attested_data: _Optional[bytes] = ..., access_token: _Optional[str] = ...) -> None: ...

class RegisterUsingAzureMethodResponse(_message.Message):
    __slots__ = ["challenge", "certs"]
    CHALLENGE_FIELD_NUMBER: _ClassVar[int]
    CERTS_FIELD_NUMBER: _ClassVar[int]
    challenge: str
    certs: _certs_pb2.Certs
    def __init__(self, challenge: _Optional[str] = ..., certs: _Optional[_Union[_certs_pb2.Certs, _Mapping]] = ...) -> None: ...
