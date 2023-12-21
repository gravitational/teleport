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

#ifndef CONTEXT_H_
#define CONTEXT_H_

#import <LocalAuthentication/LocalAuthentication.h>

#include <stdint.h>

// AuthContext is an optional, shared authentication context.
// Allows reusing a single authentication prompt/gesture between different
// functions, provided the functions are invoked in a short time interval.
typedef struct AuthContext {
  LAContext *la_ctx;
} AuthContext;

// GetLAContextFromAuth gets the LAContext from ctx, or returns a new LAContext
// instance.
LAContext *GetLAContextFromAuth(AuthContext *actx);

// AuthContextGuard guards the invocation of a Go function handle behind an
// authentication prompt.
// The expected Go function signature is `func ()`.
// Returns zero if successful, non-zero otherwise.
int AuthContextGuard(AuthContext *actx, const char *reason, uintptr_t handle,
                     char **errOut);

// AuthContextClose releases resources held by ctx.
void AuthContextClose(AuthContext *actx);

#endif // CONTEXT_H_
