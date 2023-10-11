from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class AssistViewMode(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    ASSIST_VIEW_MODE_UNSPECIFIED: _ClassVar[AssistViewMode]
    ASSIST_VIEW_MODE_DOCKED: _ClassVar[AssistViewMode]
    ASSIST_VIEW_MODE_POPUP: _ClassVar[AssistViewMode]
    ASSIST_VIEW_MODE_POPUP_EXPANDED: _ClassVar[AssistViewMode]
    ASSIST_VIEW_MODE_POPUP_EXPANDED_SIDEBAR_VISIBLE: _ClassVar[AssistViewMode]
ASSIST_VIEW_MODE_UNSPECIFIED: AssistViewMode
ASSIST_VIEW_MODE_DOCKED: AssistViewMode
ASSIST_VIEW_MODE_POPUP: AssistViewMode
ASSIST_VIEW_MODE_POPUP_EXPANDED: AssistViewMode
ASSIST_VIEW_MODE_POPUP_EXPANDED_SIDEBAR_VISIBLE: AssistViewMode

class AssistUserPreferences(_message.Message):
    __slots__ = ["preferred_logins", "view_mode"]
    PREFERRED_LOGINS_FIELD_NUMBER: _ClassVar[int]
    VIEW_MODE_FIELD_NUMBER: _ClassVar[int]
    preferred_logins: _containers.RepeatedScalarFieldContainer[str]
    view_mode: AssistViewMode
    def __init__(self, preferred_logins: _Optional[_Iterable[str]] = ..., view_mode: _Optional[_Union[AssistViewMode, str]] = ...) -> None: ...
