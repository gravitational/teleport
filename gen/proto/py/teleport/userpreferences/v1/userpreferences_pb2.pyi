from google.protobuf import empty_pb2 as _empty_pb2
from teleport.userpreferences.v1 import assist_pb2 as _assist_pb2
from teleport.userpreferences.v1 import onboard_pb2 as _onboard_pb2
from teleport.userpreferences.v1 import theme_pb2 as _theme_pb2
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class UserPreferences(_message.Message):
    __slots__ = ["assist", "theme", "onboard"]
    ASSIST_FIELD_NUMBER: _ClassVar[int]
    THEME_FIELD_NUMBER: _ClassVar[int]
    ONBOARD_FIELD_NUMBER: _ClassVar[int]
    assist: _assist_pb2.AssistUserPreferences
    theme: _theme_pb2.Theme
    onboard: _onboard_pb2.OnboardUserPreferences
    def __init__(self, assist: _Optional[_Union[_assist_pb2.AssistUserPreferences, _Mapping]] = ..., theme: _Optional[_Union[_theme_pb2.Theme, str]] = ..., onboard: _Optional[_Union[_onboard_pb2.OnboardUserPreferences, _Mapping]] = ...) -> None: ...

class GetUserPreferencesRequest(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class GetUserPreferencesResponse(_message.Message):
    __slots__ = ["preferences"]
    PREFERENCES_FIELD_NUMBER: _ClassVar[int]
    preferences: UserPreferences
    def __init__(self, preferences: _Optional[_Union[UserPreferences, _Mapping]] = ...) -> None: ...

class UpsertUserPreferencesRequest(_message.Message):
    __slots__ = ["preferences"]
    PREFERENCES_FIELD_NUMBER: _ClassVar[int]
    preferences: UserPreferences
    def __init__(self, preferences: _Optional[_Union[UserPreferences, _Mapping]] = ...) -> None: ...
