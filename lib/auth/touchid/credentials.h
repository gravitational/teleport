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

#ifndef CREDENTIALS_H_
#define CREDENTIALS_H_

#include "credential_info.h"

// LabelFilterKind is a way to filter by label.
typedef enum LabelFilterKind { LABEL_EXACT, LABEL_PREFIX } LabelFilterKind;

// LabelFilter specifies how to filter credentials by label.
typedef struct LabelFilter {
  LabelFilterKind kind;
  const char *value;
} LabelFilter;

// FindCredentials finds all credentials matching a certain label filter.
// Returns the numbers of credentials assigned to the infos array, or negative
// on failure (typically an OSStatus code). The caller is expected to free infos
// (and their contents!).
// User interaction is not required.
int FindCredentials(LabelFilter filter, CredentialInfo **infosOut);

// ListCredentials finds all registered credentials.
// Returns the numbers of credentials assigned to the infos array, or negative
// on failure (typically an OSStatus code). The caller is expected to free infos
// (and their contents!).
// Requires user interaction.
int ListCredentials(const char *reason, CredentialInfo **infosOut,
                    char **errOut);

// DeleteCredential deletes a credential by its app_label.
// Requires user interaction.
// Returns zero if successful, non-zero otherwise (typically an OSStatus).
int DeleteCredential(const char *reason, const char *appLabel, char **errOut);

// DeleteNonInteractive deletes a credential by its app_label, without user
// interaction.
// Returns zero if successful, non-zero otherwise (typically an OSStatus).
// Most callers should prefer DeleteCredential.
int DeleteNonInteractive(const char *appLabel);

#endif // CREDENTIALS_H_
