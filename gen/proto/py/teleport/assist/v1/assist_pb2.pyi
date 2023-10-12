from google.protobuf import empty_pb2 as _empty_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from teleport.legacy.client.proto import authservice_pb2 as _authservice_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class GetAssistantMessagesRequest(_message.Message):
    __slots__ = ["conversation_id", "username"]
    CONVERSATION_ID_FIELD_NUMBER: _ClassVar[int]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    conversation_id: str
    username: str
    def __init__(self, conversation_id: _Optional[str] = ..., username: _Optional[str] = ...) -> None: ...

class AssistantMessage(_message.Message):
    __slots__ = ["type", "created_time", "payload"]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    CREATED_TIME_FIELD_NUMBER: _ClassVar[int]
    PAYLOAD_FIELD_NUMBER: _ClassVar[int]
    type: str
    created_time: _timestamp_pb2.Timestamp
    payload: str
    def __init__(self, type: _Optional[str] = ..., created_time: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., payload: _Optional[str] = ...) -> None: ...

class CreateAssistantMessageRequest(_message.Message):
    __slots__ = ["message", "conversation_id", "username"]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    CONVERSATION_ID_FIELD_NUMBER: _ClassVar[int]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    message: AssistantMessage
    conversation_id: str
    username: str
    def __init__(self, message: _Optional[_Union[AssistantMessage, _Mapping]] = ..., conversation_id: _Optional[str] = ..., username: _Optional[str] = ...) -> None: ...

class GetAssistantMessagesResponse(_message.Message):
    __slots__ = ["messages"]
    MESSAGES_FIELD_NUMBER: _ClassVar[int]
    messages: _containers.RepeatedCompositeFieldContainer[AssistantMessage]
    def __init__(self, messages: _Optional[_Iterable[_Union[AssistantMessage, _Mapping]]] = ...) -> None: ...

class GetAssistantConversationsRequest(_message.Message):
    __slots__ = ["username"]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    username: str
    def __init__(self, username: _Optional[str] = ...) -> None: ...

class ConversationInfo(_message.Message):
    __slots__ = ["id", "title", "created_time"]
    ID_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    CREATED_TIME_FIELD_NUMBER: _ClassVar[int]
    id: str
    title: str
    created_time: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[str] = ..., title: _Optional[str] = ..., created_time: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class GetAssistantConversationsResponse(_message.Message):
    __slots__ = ["conversations"]
    CONVERSATIONS_FIELD_NUMBER: _ClassVar[int]
    conversations: _containers.RepeatedCompositeFieldContainer[ConversationInfo]
    def __init__(self, conversations: _Optional[_Iterable[_Union[ConversationInfo, _Mapping]]] = ...) -> None: ...

class CreateAssistantConversationRequest(_message.Message):
    __slots__ = ["username", "created_time"]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    CREATED_TIME_FIELD_NUMBER: _ClassVar[int]
    username: str
    created_time: _timestamp_pb2.Timestamp
    def __init__(self, username: _Optional[str] = ..., created_time: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class CreateAssistantConversationResponse(_message.Message):
    __slots__ = ["id"]
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class UpdateAssistantConversationInfoRequest(_message.Message):
    __slots__ = ["conversation_id", "username", "title"]
    CONVERSATION_ID_FIELD_NUMBER: _ClassVar[int]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    conversation_id: str
    username: str
    title: str
    def __init__(self, conversation_id: _Optional[str] = ..., username: _Optional[str] = ..., title: _Optional[str] = ...) -> None: ...

class IsAssistEnabledRequest(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class IsAssistEnabledResponse(_message.Message):
    __slots__ = ["enabled"]
    ENABLED_FIELD_NUMBER: _ClassVar[int]
    enabled: bool
    def __init__(self, enabled: bool = ...) -> None: ...

class DeleteAssistantConversationRequest(_message.Message):
    __slots__ = ["conversation_id", "username"]
    CONVERSATION_ID_FIELD_NUMBER: _ClassVar[int]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    conversation_id: str
    username: str
    def __init__(self, conversation_id: _Optional[str] = ..., username: _Optional[str] = ...) -> None: ...

class GetAssistantEmbeddingsRequest(_message.Message):
    __slots__ = ["username", "query", "limit", "kind"]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    QUERY_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    username: str
    query: str
    limit: int
    kind: str
    def __init__(self, username: _Optional[str] = ..., query: _Optional[str] = ..., limit: _Optional[int] = ..., kind: _Optional[str] = ...) -> None: ...

class EmbeddedDocument(_message.Message):
    __slots__ = ["id", "content", "similarity_score"]
    ID_FIELD_NUMBER: _ClassVar[int]
    CONTENT_FIELD_NUMBER: _ClassVar[int]
    SIMILARITY_SCORE_FIELD_NUMBER: _ClassVar[int]
    id: str
    content: str
    similarity_score: float
    def __init__(self, id: _Optional[str] = ..., content: _Optional[str] = ..., similarity_score: _Optional[float] = ...) -> None: ...

class GetAssistantEmbeddingsResponse(_message.Message):
    __slots__ = ["embeddings"]
    EMBEDDINGS_FIELD_NUMBER: _ClassVar[int]
    embeddings: _containers.RepeatedCompositeFieldContainer[EmbeddedDocument]
    def __init__(self, embeddings: _Optional[_Iterable[_Union[EmbeddedDocument, _Mapping]]] = ...) -> None: ...

class SearchUnifiedResourcesRequest(_message.Message):
    __slots__ = ["query", "limit", "kinds"]
    QUERY_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    KINDS_FIELD_NUMBER: _ClassVar[int]
    query: str
    limit: int
    kinds: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, query: _Optional[str] = ..., limit: _Optional[int] = ..., kinds: _Optional[_Iterable[str]] = ...) -> None: ...

class SearchUnifiedResourcesResponse(_message.Message):
    __slots__ = ["resources"]
    RESOURCES_FIELD_NUMBER: _ClassVar[int]
    resources: _containers.RepeatedCompositeFieldContainer[_authservice_pb2.PaginatedResource]
    def __init__(self, resources: _Optional[_Iterable[_Union[_authservice_pb2.PaginatedResource, _Mapping]]] = ...) -> None: ...
