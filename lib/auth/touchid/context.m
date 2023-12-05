//go:build touchid
// +build touchid

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
