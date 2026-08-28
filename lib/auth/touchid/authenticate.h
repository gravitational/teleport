/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

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
