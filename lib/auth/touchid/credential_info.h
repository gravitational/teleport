#ifndef CREDENTIAL_INFO_H_
#define CREDENTIAL_INFO_H_

// CredentialInfo represents a credential stored in the Secure Enclave.
typedef struct CredentialInfo {
  // label is the label for the Keychain entry.
  // In practice, the label is a combination of RPID and username.
  const char *label;

  // app_label is the application label for the Keychain entry.
  // In practice, the app_label is the credential ID.
  const char *app_label;

  // app_tag is the application tag for the Keychain entry.
  // In practice, the app_tag is the WebAuthn user handle.
  const char *app_tag;

  // pub_key_b64 is the public key representation, encoded as a standard base64
  // string.
  // Refer to
  // https://developer.apple.com/documentation/security/1643698-seckeycopyexternalrepresentation?language=objc.
  const char *pub_key_b64;
} CredentialInfo;

#endif // CREDENTIAL_INFO_H_
