from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ProcessSAMLIdPRequestRequest(_message.Message):
    __slots__ = ["destination", "request_id", "request_time", "metadata_url", "signature_method", "assertion", "service_provider_sso_descriptor"]
    DESTINATION_FIELD_NUMBER: _ClassVar[int]
    REQUEST_ID_FIELD_NUMBER: _ClassVar[int]
    REQUEST_TIME_FIELD_NUMBER: _ClassVar[int]
    METADATA_URL_FIELD_NUMBER: _ClassVar[int]
    SIGNATURE_METHOD_FIELD_NUMBER: _ClassVar[int]
    ASSERTION_FIELD_NUMBER: _ClassVar[int]
    SERVICE_PROVIDER_SSO_DESCRIPTOR_FIELD_NUMBER: _ClassVar[int]
    destination: str
    request_id: str
    request_time: _timestamp_pb2.Timestamp
    metadata_url: str
    signature_method: str
    assertion: bytes
    service_provider_sso_descriptor: bytes
    def __init__(self, destination: _Optional[str] = ..., request_id: _Optional[str] = ..., request_time: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., metadata_url: _Optional[str] = ..., signature_method: _Optional[str] = ..., assertion: _Optional[bytes] = ..., service_provider_sso_descriptor: _Optional[bytes] = ...) -> None: ...

class ProcessSAMLIdPRequestResponse(_message.Message):
    __slots__ = ["response"]
    RESPONSE_FIELD_NUMBER: _ClassVar[int]
    response: bytes
    def __init__(self, response: _Optional[bytes] = ...) -> None: ...
