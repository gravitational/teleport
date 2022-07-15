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

#import <CoreFoundation/CoreFoundation.h>
#import <Foundation/Foundation.h>
#import <LocalAuthentication/LocalAuthentication.h>

#include <stdint.h>

#include <dispatch/dispatch.h>

#include "common.h"

// runGoFuncHandle is provided via CGO exports.
// (#include "_cgo_export.h" also works.)
extern void runGoFuncHandle(uintptr_t handle);

LAContext *GetLAContextFromAuth(AuthContext *actx) {
  if (actx == NULL) {
    return [[LAContext alloc] init];
  }
  if (actx->la_ctx == NULL) {
    actx->la_ctx = [[LAContext alloc] init];
    actx->la_ctx.touchIDAuthenticationAllowableReuseDuration = 10; // seconds
  }
  return actx->la_ctx;
}

int AuthContextGuard(AuthContext *actx, const char *reason, uintptr_t handle,
                     char **errOut) {
  LAContext *ctx = GetLAContextFromAuth(actx);

  __block int res = 0;
  __block NSString *nsError = NULL;

  // A semaphore is needed, otherwise we return before the prompt has a chance
  // to resolve.
  dispatch_semaphore_t sema = dispatch_semaphore_create(0);
  [ctx evaluatePolicy:LAPolicyDeviceOwnerAuthenticationWithBiometrics
      localizedReason:[NSString stringWithUTF8String:reason]
                reply:^void(BOOL success, NSError *_Nullable error) {
                  if (success) {
                    runGoFuncHandle(handle);
                  } else {
                    res = -1;
                    nsError = [error localizedDescription];
                  }
                  dispatch_semaphore_signal(sema);
                }];
  dispatch_semaphore_wait(sema, DISPATCH_TIME_FOREVER);
  // sema released by ARC.

  if (nsError) {
    *errOut = CopyNSString(nsError);
  } else if (res != errSecSuccess) {
    CFStringRef err = SecCopyErrorMessageString(res, NULL);
    NSString *nsErr = (__bridge_transfer NSString *)err;
    *errOut = CopyNSString(nsErr);
  }

  return res;
}

void AuthContextClose(AuthContext *actx) {
  if (actx == NULL) {
    return;
  }
  actx->la_ctx = NULL; // Let ARC collect the LAContext.
}
