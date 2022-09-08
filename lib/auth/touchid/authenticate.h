// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

#ifndef AUTHENTICATE_H_
#define AUTHENTICATE_H_

#include <stddef.h>

#include "context.h"
#include "credential_info.h"

typedef struct AuthenticateRequest {
  const char *app_label;
  const char *digest;
  size_t digest_len;
} AuthenticateRequest;

// Authenticate finds the key specified by app_label and signs the digest using
// it. The digest is expected to be in SHA256.
// Authenticate requires user interaction.
// Returns zero if successful, non-zero otherwise.
int Authenticate(AuthContext *actx, AuthenticateRequest req, char **sigB64Out,
                 char **errOut);

#endif // AUTHENTICATE_H_
