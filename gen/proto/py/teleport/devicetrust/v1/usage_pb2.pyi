from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class AccountUsageType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    ACCOUNT_USAGE_TYPE_UNSPECIFIED: _ClassVar[AccountUsageType]
    ACCOUNT_USAGE_TYPE_UNLIMITED: _ClassVar[AccountUsageType]
    ACCOUNT_USAGE_TYPE_USAGE_BASED: _ClassVar[AccountUsageType]
ACCOUNT_USAGE_TYPE_UNSPECIFIED: AccountUsageType
ACCOUNT_USAGE_TYPE_UNLIMITED: AccountUsageType
ACCOUNT_USAGE_TYPE_USAGE_BASED: AccountUsageType

class DevicesUsage(_message.Message):
    __slots__ = ["account_usage_type", "devices_usage_limit", "devices_in_use"]
    ACCOUNT_USAGE_TYPE_FIELD_NUMBER: _ClassVar[int]
    DEVICES_USAGE_LIMIT_FIELD_NUMBER: _ClassVar[int]
    DEVICES_IN_USE_FIELD_NUMBER: _ClassVar[int]
    account_usage_type: AccountUsageType
    devices_usage_limit: int
    devices_in_use: int
    def __init__(self, account_usage_type: _Optional[_Union[AccountUsageType, str]] = ..., devices_usage_limit: _Optional[int] = ..., devices_in_use: _Optional[int] = ...) -> None: ...
