//! Shared MFA shapes for both wire modes. Field names and the JSON contract
//! mirror `web/packages/shared/libs/tdp/codec.ts` (`MfaResponse`, `MfaJson`,
//! `toMfaWebauthnChallenge`, `toMfaSsoChallenge`); the TDPB encoder bridges
//! these types to the structured proto internally.

use serde::{Deserialize, Serialize};

/// Client → server. Exactly one of `webauthn_response` / `sso_response` must
/// be set; the encoder returns `EncodeError::MissingInput` otherwise.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct MfaResponse {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub totp_code: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub webauthn_response: Option<WebauthnResponse>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub sso_response: Option<SsoResponse>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WebauthnResponse {
    pub id: String,
    #[serde(rename = "type")]
    pub kind: String,
    pub extensions: WebauthnExtensions,
    #[serde(rename = "rawId")]
    pub raw_id: String,
    pub response: WebauthnAssertion,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WebauthnExtensions {
    pub appid: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WebauthnAssertion {
    #[serde(rename = "authenticatorData")]
    pub authenticator_data: String,
    #[serde(rename = "clientDataJSON")]
    pub client_data_json: String,
    pub signature: String,
    #[serde(rename = "userHandle")]
    pub user_handle: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SsoResponse {
    #[serde(rename = "requestId")]
    pub request_id: String,
    pub token: String,
}

/// Server → client. After decode, exactly one of `webauthn_challenge` /
/// `sso_challenge` is `Some`; the variant is determined by `MfaChallenge.kind`.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct MfaChallengeJson {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub webauthn_challenge: Option<WebauthnChallenge>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub sso_challenge: Option<SsoChallenge>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WebauthnChallenge {
    #[serde(rename = "publicKey")]
    pub public_key: PublicKeyRequest,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PublicKeyRequest {
    /// Base64-encoded raw challenge bytes.
    pub challenge: String,
    #[serde(rename = "rpId")]
    pub rp_id: String,
    /// Milliseconds.
    pub timeout: i64,
    #[serde(rename = "userVerification")]
    pub user_verification: String,
    pub extensions: ExtensionsInput,
    #[serde(rename = "allowCredentials")]
    pub allow_credentials: Vec<AllowedCredential>,
}

/// Mirrors `AuthenticationExtensionsClientInputs` from the proto, serialised
/// in camelCase to match the frontend's `protobuf-ts` output.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ExtensionsInput {
    #[serde(rename = "appId")]
    pub app_id: String,
    #[serde(rename = "credProps")]
    pub cred_props: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AllowedCredential {
    /// Base64-encoded raw credential id.
    pub id: String,
    #[serde(rename = "type")]
    pub kind: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SsoChallenge {
    #[serde(rename = "channelId")]
    pub channel_id: String,
    #[serde(rename = "redirectUrl")]
    pub redirect_url: String,
    #[serde(rename = "requestId")]
    pub request_id: String,
    pub device: SsoDevice,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SsoDevice {
    #[serde(rename = "connectorId")]
    pub connector_id: String,
    #[serde(rename = "connectorType")]
    pub connector_type: String,
    #[serde(rename = "displayName")]
    pub display_name: String,
}
