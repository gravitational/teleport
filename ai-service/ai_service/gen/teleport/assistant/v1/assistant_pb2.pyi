from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ChatCompletionMessage(_message.Message):
    __slots__ = ["content", "name", "role"]
    CONTENT_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    ROLE_FIELD_NUMBER: _ClassVar[int]
    content: str
    name: str
    role: str
    def __init__(self, role: _Optional[str] = ..., content: _Optional[str] = ..., name: _Optional[str] = ...) -> None: ...

class CompleteRequest(_message.Message):
    __slots__ = ["messages", "username"]
    MESSAGES_FIELD_NUMBER: _ClassVar[int]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    messages: _containers.RepeatedCompositeFieldContainer[ChatCompletionMessage]
    username: str
    def __init__(self, username: _Optional[str] = ..., messages: _Optional[_Iterable[_Union[ChatCompletionMessage, _Mapping]]] = ...) -> None: ...

class CompletionResponse(_message.Message):
    __slots__ = ["command", "content", "kind", "labels", "nodes"]
    COMMAND_FIELD_NUMBER: _ClassVar[int]
    CONTENT_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    LABELS_FIELD_NUMBER: _ClassVar[int]
    NODES_FIELD_NUMBER: _ClassVar[int]
    command: str
    content: str
    kind: str
    labels: _containers.RepeatedCompositeFieldContainer[Label]
    nodes: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, kind: _Optional[str] = ..., content: _Optional[str] = ..., command: _Optional[str] = ..., nodes: _Optional[_Iterable[str]] = ..., labels: _Optional[_Iterable[_Union[Label, _Mapping]]] = ...) -> None: ...

class Label(_message.Message):
    __slots__ = ["key", "value"]
    KEY_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    key: str
    value: str
    def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
