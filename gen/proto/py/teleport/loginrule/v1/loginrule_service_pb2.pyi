from google.protobuf import empty_pb2 as _empty_pb2
from teleport.legacy.types.wrappers import wrappers_pb2 as _wrappers_pb2
from teleport.loginrule.v1 import loginrule_pb2 as _loginrule_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class CreateLoginRuleRequest(_message.Message):
    __slots__ = ["login_rule"]
    LOGIN_RULE_FIELD_NUMBER: _ClassVar[int]
    login_rule: _loginrule_pb2.LoginRule
    def __init__(self, login_rule: _Optional[_Union[_loginrule_pb2.LoginRule, _Mapping]] = ...) -> None: ...

class UpsertLoginRuleRequest(_message.Message):
    __slots__ = ["login_rule"]
    LOGIN_RULE_FIELD_NUMBER: _ClassVar[int]
    login_rule: _loginrule_pb2.LoginRule
    def __init__(self, login_rule: _Optional[_Union[_loginrule_pb2.LoginRule, _Mapping]] = ...) -> None: ...

class GetLoginRuleRequest(_message.Message):
    __slots__ = ["name"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class ListLoginRulesRequest(_message.Message):
    __slots__ = ["page_size", "page_token"]
    PAGE_SIZE_FIELD_NUMBER: _ClassVar[int]
    PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    page_size: int
    page_token: str
    def __init__(self, page_size: _Optional[int] = ..., page_token: _Optional[str] = ...) -> None: ...

class ListLoginRulesResponse(_message.Message):
    __slots__ = ["login_rules", "next_page_token"]
    LOGIN_RULES_FIELD_NUMBER: _ClassVar[int]
    NEXT_PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    login_rules: _containers.RepeatedCompositeFieldContainer[_loginrule_pb2.LoginRule]
    next_page_token: str
    def __init__(self, login_rules: _Optional[_Iterable[_Union[_loginrule_pb2.LoginRule, _Mapping]]] = ..., next_page_token: _Optional[str] = ...) -> None: ...

class DeleteLoginRuleRequest(_message.Message):
    __slots__ = ["name"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class TestLoginRuleRequest(_message.Message):
    __slots__ = ["login_rules", "traits", "load_from_cluster"]
    class TraitsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: _wrappers_pb2.StringValues
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[_wrappers_pb2.StringValues, _Mapping]] = ...) -> None: ...
    LOGIN_RULES_FIELD_NUMBER: _ClassVar[int]
    TRAITS_FIELD_NUMBER: _ClassVar[int]
    LOAD_FROM_CLUSTER_FIELD_NUMBER: _ClassVar[int]
    login_rules: _containers.RepeatedCompositeFieldContainer[_loginrule_pb2.LoginRule]
    traits: _containers.MessageMap[str, _wrappers_pb2.StringValues]
    load_from_cluster: bool
    def __init__(self, login_rules: _Optional[_Iterable[_Union[_loginrule_pb2.LoginRule, _Mapping]]] = ..., traits: _Optional[_Mapping[str, _wrappers_pb2.StringValues]] = ..., load_from_cluster: bool = ...) -> None: ...

class TestLoginRuleResponse(_message.Message):
    __slots__ = ["traits"]
    class TraitsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: _wrappers_pb2.StringValues
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[_wrappers_pb2.StringValues, _Mapping]] = ...) -> None: ...
    TRAITS_FIELD_NUMBER: _ClassVar[int]
    traits: _containers.MessageMap[str, _wrappers_pb2.StringValues]
    def __init__(self, traits: _Optional[_Mapping[str, _wrappers_pb2.StringValues]] = ...) -> None: ...
