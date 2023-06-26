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

#ifndef DIAG_H_
#define DIAG_H_

#include <stdbool.h>
#include <stdint.h>

typedef struct DiagResult {
  bool has_signature;
  bool has_entitlements;
  bool passed_la_policy_test;
  bool passed_secure_enclave_test;
  int64_t la_error_code;
  const char *la_error_domain;
  const char *la_error_description;
} DiagResult;

// RunDiag runs self-diagnostics to verify if Touch ID is supported.
void RunDiag(DiagResult *diagOut);

#endif // DIAG_H_
