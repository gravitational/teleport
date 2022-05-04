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

#endif // CREDENTIALS_H_
