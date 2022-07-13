//go:build touchid
// +build touchid

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

#include "context.h"

#import <LocalAuthentication/LocalAuthentication.h>

LAContext *GetLAContextFromAuth(AuthContext *ctx) {
  if (ctx == NULL) {
    return [[LAContext alloc] init];
  }
  if (ctx->la_ctx == NULL) {
    ctx->la_ctx = [[LAContext alloc] init];
    ctx->la_ctx.touchIDAuthenticationAllowableReuseDuration = 10; // seconds
  }
  return ctx->la_ctx;
}

void AuthContextClose(AuthContext *ctx) {
  if (ctx == NULL) {
    return;
  }
  ctx->la_ctx = NULL;
}
