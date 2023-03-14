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
