from gogoproto import gogo_pb2 as _gogo_pb2
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Frame(_message.Message):
    __slots__ = ["DialRequest", "ConnectionEstablished", "Data"]
    DIALREQUEST_FIELD_NUMBER: _ClassVar[int]
    CONNECTIONESTABLISHED_FIELD_NUMBER: _ClassVar[int]
    DATA_FIELD_NUMBER: _ClassVar[int]
    DialRequest: DialRequest
    ConnectionEstablished: ConnectionEstablished
    Data: Data
    def __init__(self, DialRequest: _Optional[_Union[DialRequest, _Mapping]] = ..., ConnectionEstablished: _Optional[_Union[ConnectionEstablished, _Mapping]] = ..., Data: _Optional[_Union[Data, _Mapping]] = ...) -> None: ...

class DialRequest(_message.Message):
    __slots__ = ["NodeID", "TunnelType", "Source", "Destination"]
    NODEID_FIELD_NUMBER: _ClassVar[int]
    TUNNELTYPE_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    DESTINATION_FIELD_NUMBER: _ClassVar[int]
    NodeID: str
    TunnelType: str
    Source: NetAddr
    Destination: NetAddr
    def __init__(self, NodeID: _Optional[str] = ..., TunnelType: _Optional[str] = ..., Source: _Optional[_Union[NetAddr, _Mapping]] = ..., Destination: _Optional[_Union[NetAddr, _Mapping]] = ...) -> None: ...

class NetAddr(_message.Message):
    __slots__ = ["Network", "Addr"]
    NETWORK_FIELD_NUMBER: _ClassVar[int]
    ADDR_FIELD_NUMBER: _ClassVar[int]
    Network: str
    Addr: str
    def __init__(self, Network: _Optional[str] = ..., Addr: _Optional[str] = ...) -> None: ...

class Data(_message.Message):
    __slots__ = ["Bytes"]
    BYTES_FIELD_NUMBER: _ClassVar[int]
    Bytes: bytes
    def __init__(self, Bytes: _Optional[bytes] = ...) -> None: ...

class ConnectionEstablished(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...
