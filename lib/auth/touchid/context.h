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

#ifndef CONTEXT_H_
#define CONTEXT_H_

#import <LocalAuthentication/LocalAuthentication.h>

// AuthContext is an optional, shared authentication context.
// Allows reusing a single authentication prompt/gesture between different
// functions, provided the functions are invoked in a short time interval.
typedef struct AuthContext {
  LAContext *la_ctx;
} AuthContext;

// GetLAContextFromAuth gets the LAContext from ctx, or returns a new LAContext
// instance.
LAContext *GetLAContextFromAuth(AuthContext *ctx);

// AuthContextClose releases resources held by ctx.
void AuthContextClose(AuthContext *ctx);

#endif // CONTEXT_H_
