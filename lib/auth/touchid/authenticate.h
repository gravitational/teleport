#ifndef AUTHENTICATE_H_
#define AUTHENTICATE_H_

#include "credential_info.h"

typedef struct AuthenticateRequest {
  const char *app_label;
  const char *digest;
  int digest_len;
} AuthenticateRequest;

// Authenticate finds the key specified by app_label and signs the digest using
// it. The digest is expected to be in SHA256.
// Authenticate requires user interaction.
// Returns zero if successful, non-zero otherwise.
int Authenticate(AuthenticateRequest req, char **sigB64Out, char **errOut);

#endif // AUTHENTICATE_H_
