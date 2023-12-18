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
