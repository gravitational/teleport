from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class GetUsageRequest(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class GetUsageResponse(_message.Message):
    __slots__ = ["access_requests"]
    ACCESS_REQUESTS_FIELD_NUMBER: _ClassVar[int]
    access_requests: AccessRequestsUsage
    def __init__(self, access_requests: _Optional[_Union[AccessRequestsUsage, _Mapping]] = ...) -> None: ...

class AccessRequestsUsage(_message.Message):
    __slots__ = ["monthly_limit", "monthly_used"]
    MONTHLY_LIMIT_FIELD_NUMBER: _ClassVar[int]
    MONTHLY_USED_FIELD_NUMBER: _ClassVar[int]
    monthly_limit: int
    monthly_used: int
    def __init__(self, monthly_limit: _Optional[int] = ..., monthly_used: _Optional[int] = ...) -> None: ...
