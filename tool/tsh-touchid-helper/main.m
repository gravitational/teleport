// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

// tsh-touchid-helper is a signed helper binary that provides Touch ID
// operations (Secure Enclave, Keychain) over a JSON-RPC protocol on
// stdin/stdout. It is used by unsigned dev builds of tsh to access
// Touch ID functionality.

#import <Foundation/Foundation.h>

#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>

#include "authenticate.h"
#include "context.h"
#include "credential_info.h"
#include "credentials.h"
#include "diag.h"
#include "register.h"

// runGoFuncHandle is declared extern in context.m (normally provided by CGO).
// In the helper binary, AuthContextGuard calls this after a successful biometric
// prompt. We provide a no-op since the actual callback runs on the tsh side.
void runGoFuncHandle(uintptr_t handle) {
  (void)handle;
}

static const char *kPromptReason = "authenticate user";

// Auth context storage.
static NSMutableDictionary<NSNumber *, NSValue *> *authContexts;
static int nextContextID = 0;

#pragma mark - JSON Helpers

static NSDictionary *errorResponse(int reqID, NSString *msg) {
  return @{@"id" : @(reqID), @"error" : msg};
}

static NSDictionary *resultResponse(int reqID, NSDictionary *result) {
  return @{@"id" : @(reqID), @"result" : result};
}

static NSDictionary *emptyResultResponse(int reqID) {
  return @{@"id" : @(reqID), @"result" : @{}};
}

#pragma mark - Credential Info Conversion

// Convert a C CredentialInfo to a JSON-compatible dictionary.
// Frees the C strings in the CredentialInfo.
static NSDictionary *credentialInfoToDict(CredentialInfo *info) {
  NSString *credID =
      info->app_label ? [NSString stringWithUTF8String:info->app_label] : @"";
  NSString *label =
      info->label ? [NSString stringWithUTF8String:info->label] : @"";
  NSString *appTag =
      info->app_tag ? [NSString stringWithUTF8String:info->app_tag] : @"";
  NSString *pubKeyB64 =
      info->pub_key_b64 ? [NSString stringWithUTF8String:info->pub_key_b64]
                        : @"";
  NSString *createDate =
      info->creation_date ? [NSString stringWithUTF8String:info->creation_date]
                          : @"";

  // Parse label to extract RPID and username.
  // Label format: "t01/{rpid} {user}"
  NSString *rpid = @"";
  NSString *userName = @"";
  NSString *prefix = @"t01/";
  if ([label hasPrefix:prefix]) {
    NSString *rest = [label substringFromIndex:prefix.length];
    NSRange spaceRange = [rest rangeOfString:@" "];
    if (spaceRange.location != NSNotFound) {
      rpid = [rest substringToIndex:spaceRange.location];
      userName = [rest substringFromIndex:spaceRange.location + 1];
    }
  }

  // Encode user_handle (appTag is already base64url-encoded, convert to
  // standard base64 for JSON).
  NSData *userHandleData =
      [[NSData alloc] initWithBase64EncodedString:appTag options:0];
  NSString *userHandleB64 =
      userHandleData
          ? [userHandleData base64EncodedStringWithOptions:0]
          : @"";

  // pub_key_b64 is already standard base64.

  // Free C strings.
  free((void *)info->label);
  free((void *)info->app_label);
  free((void *)info->app_tag);
  free((void *)info->pub_key_b64);
  free((void *)info->creation_date);

  return @{
    @"credential_id" : credID,
    @"rpid" : rpid,
    @"user_name" : userName,
    @"user_handle" : userHandleB64,
    @"pub_key_raw" : pubKeyB64,
    @"create_time" : createDate,
  };
}

#pragma mark - Handlers

static NSDictionary *handleDiag(int reqID) {
  DiagResult diag = {0};
  RunDiag(&diag);

  BOOL isAvailable = diag.has_signature && diag.has_entitlements &&
                     diag.passed_la_policy_test &&
                     diag.passed_secure_enclave_test;
  NSDictionary *result = @{
    @"has_signature" : @(diag.has_signature),
    @"has_entitlements" : @(diag.has_entitlements),
    @"passed_la_policy_test" : @(diag.passed_la_policy_test),
    @"passed_secure_enclave_test" : @(diag.passed_secure_enclave_test),
    @"is_available" : @(isAvailable),
  };

  free((void *)diag.la_error_domain);
  free((void *)diag.la_error_description);

  return resultResponse(reqID, result);
}

static NSDictionary *handleRegister(int reqID, NSDictionary *params) {
  NSString *rpid = params[@"rpid"];
  NSString *user = params[@"user"];
  NSString *userHandleB64 = params[@"user_handle"];

  if (!rpid || !user || !userHandleB64) {
    return errorResponse(reqID, @"missing required params: rpid, user, "
                                @"user_handle");
  }

  // Generate a credential ID (UUID).
  NSString *credentialID =
      [[NSUUID UUID] UUIDString].lowercaseString;

  // Decode user_handle from base64.
  NSData *userHandleData =
      [[NSData alloc] initWithBase64EncodedString:userHandleB64 options:0];
  if (!userHandleData) {
    userHandleData = [NSData data];
  }

  // Re-encode as base64url (no padding) for the app_tag, matching Go's
  // base64.RawURLEncoding.
  NSString *userHandleB64URL =
      [[userHandleData base64EncodedStringWithOptions:0]
          stringByReplacingOccurrencesOfString:@"+"
                                   withString:@"-"];
  userHandleB64URL = [userHandleB64URL
      stringByReplacingOccurrencesOfString:@"/"
                                withString:@"_"];
  // Remove padding.
  userHandleB64URL = [userHandleB64URL
      stringByReplacingOccurrencesOfString:@"="
                                withString:@""];

  // Build label: "t01/{rpid} {user}"
  NSString *label =
      [NSString stringWithFormat:@"t01/%@ %@", rpid, user];

  CredentialInfo req = {0};
  req.label = [label UTF8String];
  req.app_label = [credentialID UTF8String];
  req.app_tag = [userHandleB64URL UTF8String];

  char *pubKeyB64 = NULL;
  char *errMsg = NULL;
  int res = Register(req, &pubKeyB64, &errMsg);
  if (res != 0) {
    NSString *err = errMsg ? [NSString stringWithUTF8String:errMsg]
                           : @"register failed";
    free(errMsg);
    return errorResponse(reqID, err);
  }

  // Decode the base64 public key and re-encode for JSON transport.
  NSString *pubKeyB64Str =
      pubKeyB64 ? [NSString stringWithUTF8String:pubKeyB64] : @"";
  free(pubKeyB64);

  return resultResponse(reqID, @{
    @"credential_id" : credentialID,
    @"pub_key_raw" : pubKeyB64Str,
  });
}

static NSDictionary *handleAuthenticate(int reqID, NSDictionary *params) {
  NSNumber *contextIDNum = params[@"context_id"];
  NSString *credentialID = params[@"credential_id"];
  NSString *digestB64 = params[@"digest"];

  if (!credentialID || !digestB64) {
    return errorResponse(reqID,
                         @"missing required params: credential_id, digest");
  }

  // Look up auth context.
  AuthContext *actx = NULL;
  if (contextIDNum && [contextIDNum intValue] != 0) {
    NSValue *val = authContexts[contextIDNum];
    if (val) {
      actx = [val pointerValue];
    } else {
      return errorResponse(
          reqID, [NSString stringWithFormat:@"auth context %@ not found",
                                           contextIDNum]);
    }
  }

  // Decode digest from base64.
  NSData *digestData =
      [[NSData alloc] initWithBase64EncodedString:digestB64 options:0];
  if (!digestData) {
    return errorResponse(reqID, @"invalid base64 digest");
  }

  AuthenticateRequest authReq = {0};
  authReq.app_label = [credentialID UTF8String];
  authReq.digest = (const char *)[digestData bytes];
  authReq.digest_len = [digestData length];

  char *sigB64 = NULL;
  char *errMsg = NULL;
  int res = Authenticate(actx, authReq, &sigB64, &errMsg);
  if (res != 0) {
    NSString *err = errMsg ? [NSString stringWithUTF8String:errMsg]
                           : @"authenticate failed";
    free(errMsg);
    return errorResponse(reqID, err);
  }

  NSString *sigB64Str =
      sigB64 ? [NSString stringWithUTF8String:sigB64] : @"";
  free(sigB64);

  return resultResponse(reqID, @{@"signature" : sigB64Str});
}

static NSDictionary *handleFindCredentials(int reqID, NSDictionary *params) {
  NSString *rpid = params[@"rpid"];
  NSString *user = params[@"user"];

  if (!rpid) {
    return errorResponse(reqID, @"missing required param: rpid");
  }

  // Build label filter.
  NSString *label;
  LabelFilter filter = {0};
  if (user && user.length > 0) {
    label = [NSString stringWithFormat:@"t01/%@ %@", rpid, user];
    filter.kind = LABEL_EXACT;
  } else {
    label = [NSString stringWithFormat:@"t01/%@ ", rpid];
    filter.kind = LABEL_PREFIX;
  }
  filter.value = [label UTF8String];

  CredentialInfo *infos = NULL;
  int count = FindCredentials(filter, &infos);
  if (count < 0) {
    free(infos);
    return errorResponse(
        reqID,
        [NSString stringWithFormat:@"find credentials failed: status %d",
                                   count]);
  }

  NSMutableArray *creds = [NSMutableArray arrayWithCapacity:count];
  for (int i = 0; i < count; i++) {
    [creds addObject:credentialInfoToDict(&infos[i])];
  }
  free(infos);

  return resultResponse(reqID, @{@"credentials" : creds});
}

static NSDictionary *handleListCredentials(int reqID) {
  CredentialInfo *infos = NULL;
  char *errMsg = NULL;
  int count = ListCredentials(kPromptReason, &infos, &errMsg);
  if (count < 0) {
    NSString *err = errMsg ? [NSString stringWithUTF8String:errMsg]
                           : @"list credentials failed";
    free(errMsg);
    free(infos);
    return errorResponse(reqID, err);
  }

  NSMutableArray *creds = [NSMutableArray arrayWithCapacity:count];
  for (int i = 0; i < count; i++) {
    [creds addObject:credentialInfoToDict(&infos[i])];
  }
  free(infos);

  return resultResponse(reqID, @{@"credentials" : creds});
}

static NSDictionary *handleDeleteCredential(int reqID, NSDictionary *params) {
  NSString *credentialID = params[@"credential_id"];
  if (!credentialID) {
    return errorResponse(reqID, @"missing required param: credential_id");
  }

  char *errMsg = NULL;
  int res =
      DeleteCredential(kPromptReason, [credentialID UTF8String], &errMsg);
  if (res != 0) {
    NSString *err = errMsg ? [NSString stringWithUTF8String:errMsg]
                           : @"delete credential failed";
    free(errMsg);
    return errorResponse(reqID, err);
  }

  return emptyResultResponse(reqID);
}

static NSDictionary *handleDeleteNonInteractive(int reqID,
                                                NSDictionary *params) {
  NSString *credentialID = params[@"credential_id"];
  if (!credentialID) {
    return errorResponse(reqID, @"missing required param: credential_id");
  }

  int res = DeleteNonInteractive([credentialID UTF8String]);
  if (res != 0) {
    return errorResponse(
        reqID,
        [NSString
            stringWithFormat:@"non-interactive delete failed: status %d", res]);
  }

  return emptyResultResponse(reqID);
}

static NSDictionary *handleNewAuthContext(int reqID) {
  AuthContext *actx = calloc(1, sizeof(AuthContext));

  nextContextID++;
  NSNumber *key = @(nextContextID);
  authContexts[key] = [NSValue valueWithPointer:actx];

  return resultResponse(reqID, @{@"context_id" : key});
}

static NSDictionary *handleAuthContextGuard(int reqID, NSDictionary *params) {
  NSNumber *contextIDNum = params[@"context_id"];
  if (!contextIDNum) {
    return errorResponse(reqID, @"missing required param: context_id");
  }

  NSValue *val = authContexts[contextIDNum];
  if (!val) {
    return errorResponse(
        reqID, [NSString stringWithFormat:@"auth context %@ not found",
                                         contextIDNum]);
  }

  AuthContext *actx = [val pointerValue];
  char *errMsg = NULL;
  int res = AuthContextGuard(actx, kPromptReason, 0 /* handle */, &errMsg);
  if (res != 0) {
    NSString *err = errMsg ? [NSString stringWithUTF8String:errMsg]
                           : @"auth context guard failed";
    free(errMsg);
    return errorResponse(reqID, err);
  }

  return emptyResultResponse(reqID);
}

static NSDictionary *handleAuthContextClose(int reqID, NSDictionary *params) {
  NSNumber *contextIDNum = params[@"context_id"];
  if (!contextIDNum) {
    return errorResponse(reqID, @"missing required param: context_id");
  }

  NSValue *val = authContexts[contextIDNum];
  if (!val) {
    return errorResponse(
        reqID, [NSString stringWithFormat:@"auth context %@ not found",
                                         contextIDNum]);
  }

  AuthContext *actx = [val pointerValue];
  AuthContextClose(actx);
  free(actx);
  [authContexts removeObjectForKey:contextIDNum];

  return emptyResultResponse(reqID);
}

#pragma mark - Dispatch

static NSDictionary *dispatch(int reqID, NSString *method,
                              NSDictionary *params) {
  if ([method isEqualToString:@"Diag"]) {
    return handleDiag(reqID);
  } else if ([method isEqualToString:@"Register"]) {
    return handleRegister(reqID, params);
  } else if ([method isEqualToString:@"Authenticate"]) {
    return handleAuthenticate(reqID, params);
  } else if ([method isEqualToString:@"FindCredentials"]) {
    return handleFindCredentials(reqID, params);
  } else if ([method isEqualToString:@"ListCredentials"]) {
    return handleListCredentials(reqID);
  } else if ([method isEqualToString:@"DeleteCredential"]) {
    return handleDeleteCredential(reqID, params);
  } else if ([method isEqualToString:@"DeleteNonInteractive"]) {
    return handleDeleteNonInteractive(reqID, params);
  } else if ([method isEqualToString:@"NewAuthContext"]) {
    return handleNewAuthContext(reqID);
  } else if ([method isEqualToString:@"AuthContextGuard"]) {
    return handleAuthContextGuard(reqID, params);
  } else if ([method isEqualToString:@"AuthContextClose"]) {
    return handleAuthContextClose(reqID, params);
  }

  return errorResponse(
      reqID, [NSString stringWithFormat:@"unknown method: %@", method]);
}

#pragma mark - Main

int main(int argc, const char *argv[]) {
  @autoreleasepool {
    authContexts = [NSMutableDictionary dictionary];

    // Read stdin line by line.
    char buf[1024 * 1024]; // 1MB line buffer
    while (fgets(buf, sizeof(buf), stdin) != NULL) {
      @autoreleasepool {
        NSString *line =
            [[NSString stringWithUTF8String:buf]
                stringByTrimmingCharactersInSet:
                    [NSCharacterSet whitespaceAndNewlineCharacterSet]];
        if (line.length == 0) {
          continue;
        }

        // Parse JSON request.
        NSData *jsonData = [line dataUsingEncoding:NSUTF8StringEncoding];
        NSError *parseError = nil;
        NSDictionary *req =
            [NSJSONSerialization JSONObjectWithData:jsonData
                                           options:0
                                             error:&parseError];
        if (parseError || ![req isKindOfClass:[NSDictionary class]]) {
          fprintf(stderr, "ERROR: failed to parse request: %s\n",
                  parseError ? [[parseError description] UTF8String]
                             : "not a dictionary");
          continue;
        }

        int reqID = [req[@"id"] intValue];
        NSString *method = req[@"method"];
        NSDictionary *params = req[@"params"];
        if (!params || ![params isKindOfClass:[NSDictionary class]]) {
          params = @{};
        }

        // Dispatch and get response.
        NSDictionary *resp = dispatch(reqID, method, params);

        // Encode response as JSON.
        NSError *encodeError = nil;
        NSData *respData =
            [NSJSONSerialization dataWithJSONObject:resp
                                           options:0
                                             error:&encodeError];
        if (encodeError) {
          fprintf(stderr, "ERROR: failed to encode response: %s\n",
                  [[encodeError description] UTF8String]);
          continue;
        }

        // Write response + newline to stdout.
        fwrite([respData bytes], 1, [respData length], stdout);
        fputc('\n', stdout);
        fflush(stdout);
      }
    }

    // Cleanup auth contexts on exit.
    for (NSNumber *key in [authContexts allKeys]) {
      AuthContext *actx = [authContexts[key] pointerValue];
      AuthContextClose(actx);
      free(actx);
    }
    [authContexts removeAllObjects];
  }

  return 0;
}
