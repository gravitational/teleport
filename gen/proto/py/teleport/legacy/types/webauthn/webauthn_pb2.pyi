from gogoproto import gogo_pb2 as _gogo_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class SessionData(_message.Message):
    __slots__ = ["challenge", "user_id", "allow_credentials", "resident_key", "user_verification"]
    CHALLENGE_FIELD_NUMBER: _ClassVar[int]
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    ALLOW_CREDENTIALS_FIELD_NUMBER: _ClassVar[int]
    RESIDENT_KEY_FIELD_NUMBER: _ClassVar[int]
    USER_VERIFICATION_FIELD_NUMBER: _ClassVar[int]
    challenge: bytes
    user_id: bytes
    allow_credentials: _containers.RepeatedScalarFieldContainer[bytes]
    resident_key: bool
    user_verification: str
    def __init__(self, challenge: _Optional[bytes] = ..., user_id: _Optional[bytes] = ..., allow_credentials: _Optional[_Iterable[bytes]] = ..., resident_key: bool = ..., user_verification: _Optional[str] = ...) -> None: ...

class User(_message.Message):
    __slots__ = ["teleport_user"]
    TELEPORT_USER_FIELD_NUMBER: _ClassVar[int]
    teleport_user: str
    def __init__(self, teleport_user: _Optional[str] = ...) -> None: ...

class CredentialAssertion(_message.Message):
    __slots__ = ["public_key"]
    PUBLIC_KEY_FIELD_NUMBER: _ClassVar[int]
    public_key: PublicKeyCredentialRequestOptions
    def __init__(self, public_key: _Optional[_Union[PublicKeyCredentialRequestOptions, _Mapping]] = ...) -> None: ...

class PublicKeyCredentialRequestOptions(_message.Message):
    __slots__ = ["challenge", "timeout_ms", "rp_id", "allow_credentials", "extensions", "user_verification"]
    CHALLENGE_FIELD_NUMBER: _ClassVar[int]
    TIMEOUT_MS_FIELD_NUMBER: _ClassVar[int]
    RP_ID_FIELD_NUMBER: _ClassVar[int]
    ALLOW_CREDENTIALS_FIELD_NUMBER: _ClassVar[int]
    EXTENSIONS_FIELD_NUMBER: _ClassVar[int]
    USER_VERIFICATION_FIELD_NUMBER: _ClassVar[int]
    challenge: bytes
    timeout_ms: int
    rp_id: str
    allow_credentials: _containers.RepeatedCompositeFieldContainer[CredentialDescriptor]
    extensions: AuthenticationExtensionsClientInputs
    user_verification: str
    def __init__(self, challenge: _Optional[bytes] = ..., timeout_ms: _Optional[int] = ..., rp_id: _Optional[str] = ..., allow_credentials: _Optional[_Iterable[_Union[CredentialDescriptor, _Mapping]]] = ..., extensions: _Optional[_Union[AuthenticationExtensionsClientInputs, _Mapping]] = ..., user_verification: _Optional[str] = ...) -> None: ...

class CredentialAssertionResponse(_message.Message):
    __slots__ = ["type", "raw_id", "response", "extensions"]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    RAW_ID_FIELD_NUMBER: _ClassVar[int]
    RESPONSE_FIELD_NUMBER: _ClassVar[int]
    EXTENSIONS_FIELD_NUMBER: _ClassVar[int]
    type: str
    raw_id: bytes
    response: AuthenticatorAssertionResponse
    extensions: AuthenticationExtensionsClientOutputs
    def __init__(self, type: _Optional[str] = ..., raw_id: _Optional[bytes] = ..., response: _Optional[_Union[AuthenticatorAssertionResponse, _Mapping]] = ..., extensions: _Optional[_Union[AuthenticationExtensionsClientOutputs, _Mapping]] = ...) -> None: ...

class AuthenticatorAssertionResponse(_message.Message):
    __slots__ = ["client_data_json", "authenticator_data", "signature", "user_handle"]
    CLIENT_DATA_JSON_FIELD_NUMBER: _ClassVar[int]
    AUTHENTICATOR_DATA_FIELD_NUMBER: _ClassVar[int]
    SIGNATURE_FIELD_NUMBER: _ClassVar[int]
    USER_HANDLE_FIELD_NUMBER: _ClassVar[int]
    client_data_json: bytes
    authenticator_data: bytes
    signature: bytes
    user_handle: bytes
    def __init__(self, client_data_json: _Optional[bytes] = ..., authenticator_data: _Optional[bytes] = ..., signature: _Optional[bytes] = ..., user_handle: _Optional[bytes] = ...) -> None: ...

class CredentialCreation(_message.Message):
    __slots__ = ["public_key"]
    PUBLIC_KEY_FIELD_NUMBER: _ClassVar[int]
    public_key: PublicKeyCredentialCreationOptions
    def __init__(self, public_key: _Optional[_Union[PublicKeyCredentialCreationOptions, _Mapping]] = ...) -> None: ...

class PublicKeyCredentialCreationOptions(_message.Message):
    __slots__ = ["challenge", "rp", "user", "credential_parameters", "timeout_ms", "exclude_credentials", "attestation", "extensions", "authenticator_selection"]
    CHALLENGE_FIELD_NUMBER: _ClassVar[int]
    RP_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    CREDENTIAL_PARAMETERS_FIELD_NUMBER: _ClassVar[int]
    TIMEOUT_MS_FIELD_NUMBER: _ClassVar[int]
    EXCLUDE_CREDENTIALS_FIELD_NUMBER: _ClassVar[int]
    ATTESTATION_FIELD_NUMBER: _ClassVar[int]
    EXTENSIONS_FIELD_NUMBER: _ClassVar[int]
    AUTHENTICATOR_SELECTION_FIELD_NUMBER: _ClassVar[int]
    challenge: bytes
    rp: RelyingPartyEntity
    user: UserEntity
    credential_parameters: _containers.RepeatedCompositeFieldContainer[CredentialParameter]
    timeout_ms: int
    exclude_credentials: _containers.RepeatedCompositeFieldContainer[CredentialDescriptor]
    attestation: str
    extensions: AuthenticationExtensionsClientInputs
    authenticator_selection: AuthenticatorSelection
    def __init__(self, challenge: _Optional[bytes] = ..., rp: _Optional[_Union[RelyingPartyEntity, _Mapping]] = ..., user: _Optional[_Union[UserEntity, _Mapping]] = ..., credential_parameters: _Optional[_Iterable[_Union[CredentialParameter, _Mapping]]] = ..., timeout_ms: _Optional[int] = ..., exclude_credentials: _Optional[_Iterable[_Union[CredentialDescriptor, _Mapping]]] = ..., attestation: _Optional[str] = ..., extensions: _Optional[_Union[AuthenticationExtensionsClientInputs, _Mapping]] = ..., authenticator_selection: _Optional[_Union[AuthenticatorSelection, _Mapping]] = ...) -> None: ...

class CredentialCreationResponse(_message.Message):
    __slots__ = ["type", "raw_id", "response", "extensions"]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    RAW_ID_FIELD_NUMBER: _ClassVar[int]
    RESPONSE_FIELD_NUMBER: _ClassVar[int]
    EXTENSIONS_FIELD_NUMBER: _ClassVar[int]
    type: str
    raw_id: bytes
    response: AuthenticatorAttestationResponse
    extensions: AuthenticationExtensionsClientOutputs
    def __init__(self, type: _Optional[str] = ..., raw_id: _Optional[bytes] = ..., response: _Optional[_Union[AuthenticatorAttestationResponse, _Mapping]] = ..., extensions: _Optional[_Union[AuthenticationExtensionsClientOutputs, _Mapping]] = ...) -> None: ...

class AuthenticatorAttestationResponse(_message.Message):
    __slots__ = ["client_data_json", "attestation_object"]
    CLIENT_DATA_JSON_FIELD_NUMBER: _ClassVar[int]
    ATTESTATION_OBJECT_FIELD_NUMBER: _ClassVar[int]
    client_data_json: bytes
    attestation_object: bytes
    def __init__(self, client_data_json: _Optional[bytes] = ..., attestation_object: _Optional[bytes] = ...) -> None: ...

class AuthenticationExtensionsClientInputs(_message.Message):
    __slots__ = ["app_id"]
    APP_ID_FIELD_NUMBER: _ClassVar[int]
    app_id: str
    def __init__(self, app_id: _Optional[str] = ...) -> None: ...

class AuthenticationExtensionsClientOutputs(_message.Message):
    __slots__ = ["app_id"]
    APP_ID_FIELD_NUMBER: _ClassVar[int]
    app_id: bool
    def __init__(self, app_id: bool = ...) -> None: ...

class AuthenticatorSelection(_message.Message):
    __slots__ = ["authenticator_attachment", "require_resident_key", "user_verification"]
    AUTHENTICATOR_ATTACHMENT_FIELD_NUMBER: _ClassVar[int]
    REQUIRE_RESIDENT_KEY_FIELD_NUMBER: _ClassVar[int]
    USER_VERIFICATION_FIELD_NUMBER: _ClassVar[int]
    authenticator_attachment: str
    require_resident_key: bool
    user_verification: str
    def __init__(self, authenticator_attachment: _Optional[str] = ..., require_resident_key: bool = ..., user_verification: _Optional[str] = ...) -> None: ...

class CredentialDescriptor(_message.Message):
    __slots__ = ["type", "id"]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    ID_FIELD_NUMBER: _ClassVar[int]
    type: str
    id: bytes
    def __init__(self, type: _Optional[str] = ..., id: _Optional[bytes] = ...) -> None: ...

class CredentialParameter(_message.Message):
    __slots__ = ["type", "alg"]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    ALG_FIELD_NUMBER: _ClassVar[int]
    type: str
    alg: int
    def __init__(self, type: _Optional[str] = ..., alg: _Optional[int] = ...) -> None: ...

class RelyingPartyEntity(_message.Message):
    __slots__ = ["id", "name"]
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ...) -> None: ...

class UserEntity(_message.Message):
    __slots__ = ["id", "name", "display_name"]
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    DISPLAY_NAME_FIELD_NUMBER: _ClassVar[int]
    id: bytes
    name: str
    display_name: str
    def __init__(self, id: _Optional[bytes] = ..., name: _Optional[str] = ..., display_name: _Optional[str] = ...) -> None: ...
