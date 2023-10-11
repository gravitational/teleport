from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class TPMPCR(_message.Message):
    __slots__ = ["index", "digest", "digest_alg"]
    INDEX_FIELD_NUMBER: _ClassVar[int]
    DIGEST_FIELD_NUMBER: _ClassVar[int]
    DIGEST_ALG_FIELD_NUMBER: _ClassVar[int]
    index: int
    digest: bytes
    digest_alg: int
    def __init__(self, index: _Optional[int] = ..., digest: _Optional[bytes] = ..., digest_alg: _Optional[int] = ...) -> None: ...

class TPMQuote(_message.Message):
    __slots__ = ["quote", "signature"]
    QUOTE_FIELD_NUMBER: _ClassVar[int]
    SIGNATURE_FIELD_NUMBER: _ClassVar[int]
    quote: bytes
    signature: bytes
    def __init__(self, quote: _Optional[bytes] = ..., signature: _Optional[bytes] = ...) -> None: ...

class TPMPlatformParameters(_message.Message):
    __slots__ = ["quotes", "pcrs", "event_log"]
    QUOTES_FIELD_NUMBER: _ClassVar[int]
    PCRS_FIELD_NUMBER: _ClassVar[int]
    EVENT_LOG_FIELD_NUMBER: _ClassVar[int]
    quotes: _containers.RepeatedCompositeFieldContainer[TPMQuote]
    pcrs: _containers.RepeatedCompositeFieldContainer[TPMPCR]
    event_log: bytes
    def __init__(self, quotes: _Optional[_Iterable[_Union[TPMQuote, _Mapping]]] = ..., pcrs: _Optional[_Iterable[_Union[TPMPCR, _Mapping]]] = ..., event_log: _Optional[bytes] = ...) -> None: ...

class TPMPlatformAttestation(_message.Message):
    __slots__ = ["nonce", "platform_parameters"]
    NONCE_FIELD_NUMBER: _ClassVar[int]
    PLATFORM_PARAMETERS_FIELD_NUMBER: _ClassVar[int]
    nonce: bytes
    platform_parameters: TPMPlatformParameters
    def __init__(self, nonce: _Optional[bytes] = ..., platform_parameters: _Optional[_Union[TPMPlatformParameters, _Mapping]] = ...) -> None: ...
