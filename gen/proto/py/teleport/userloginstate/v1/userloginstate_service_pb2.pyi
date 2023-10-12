from google.protobuf import empty_pb2 as _empty_pb2
from teleport.userloginstate.v1 import userloginstate_pb2 as _userloginstate_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class GetUserLoginStatesRequest(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class GetUserLoginStatesResponse(_message.Message):
    __slots__ = ["user_login_states"]
    USER_LOGIN_STATES_FIELD_NUMBER: _ClassVar[int]
    user_login_states: _containers.RepeatedCompositeFieldContainer[_userloginstate_pb2.UserLoginState]
    def __init__(self, user_login_states: _Optional[_Iterable[_Union[_userloginstate_pb2.UserLoginState, _Mapping]]] = ...) -> None: ...

class GetUserLoginStateRequest(_message.Message):
    __slots__ = ["name"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class UpsertUserLoginStateRequest(_message.Message):
    __slots__ = ["user_login_state"]
    USER_LOGIN_STATE_FIELD_NUMBER: _ClassVar[int]
    user_login_state: _userloginstate_pb2.UserLoginState
    def __init__(self, user_login_state: _Optional[_Union[_userloginstate_pb2.UserLoginState, _Mapping]] = ...) -> None: ...

class DeleteUserLoginStateRequest(_message.Message):
    __slots__ = ["name"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class DeleteAllUserLoginStatesRequest(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...
