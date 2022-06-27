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

#include "credentials.h"

#import <CoreFoundation/CoreFoundation.h>
#import <Foundation/Foundation.h>
#import <LocalAuthentication/LocalAuthentication.h>
#import <Security/Security.h>

#include <limits.h>
#include <stdlib.h>

#include <dispatch/dispatch.h>

#include "common.h"

BOOL matchesLabelFilter(LabelFilterKind kind, NSString *filter,
                        NSString *label) {
  switch (kind) {
  case LABEL_EXACT:
    return [label isEqualToString:filter];
  case LABEL_PREFIX:
    return [label hasPrefix:filter];
  }
  return NO;
}

int findCredentials(BOOL applyFilter, LabelFilter filter,
                    CredentialInfo **infosOut) {
  NSDictionary *query = @{
    (id)kSecClass : (id)kSecClassKey,
    (id)kSecAttrKeyType : (id)kSecAttrKeyTypeECSECPrimeRandom,
    (id)kSecMatchLimit : (id)kSecMatchLimitAll,
    (id)kSecReturnRef : @YES,
    (id)kSecReturnAttributes : @YES,
  };
  CFArrayRef items = NULL;
  OSStatus status =
      SecItemCopyMatching((CFDictionaryRef)query, (CFTypeRef *)&items);
  switch (status) {
  case errSecSuccess:
    break; // continue below
  case errSecItemNotFound:
    return 0; // aka no items found
  default:
    // Not possible afaik, but let's make sure we keep up the method contract.
    if (status >= 0) {
      status = status * -1;
    }
    return status;
  }

  NSString *nsFilter = [NSString stringWithUTF8String:filter.value];

  CFIndex count = CFArrayGetCount(items);
  // Guard against overflows, just in case we ever get that many credentials.
  if (count > INT_MAX) {
    count = INT_MAX;
  }
  *infosOut = calloc(count, sizeof(CredentialInfo));
  int infosLen = 0;
  for (CFIndex i = 0; i < count; i++) {
    CFDictionaryRef attrs = CFArrayGetValueAtIndex(items, i);

    CFStringRef label = CFDictionaryGetValue(attrs, kSecAttrLabel);
    NSString *nsLabel = (__bridge NSString *)label;
    if (applyFilter && !matchesLabelFilter(filter.kind, nsFilter, nsLabel)) {
      continue;
    }

    CFDataRef appTag = CFDictionaryGetValue(attrs, kSecAttrApplicationTag);
    NSString *nsAppTag =
        [[NSString alloc] initWithData:(__bridge NSData *)appTag
                              encoding:NSUTF8StringEncoding];

    CFDataRef appLabel = CFDictionaryGetValue(attrs, kSecAttrApplicationLabel);
    NSString *nsAppLabel =
        [[NSString alloc] initWithData:(__bridge NSData *)appLabel
                              encoding:NSUTF8StringEncoding];

    // Copy public key representation.
    SecKeyRef privKey = (SecKeyRef)CFDictionaryGetValue(attrs, kSecValueRef);
    SecKeyRef pubKey = SecKeyCopyPublicKey(privKey);
    char *pubKeyB64 = NULL;
    if (pubKey) {
      CFDataRef pubKeyRep =
          SecKeyCopyExternalRepresentation(pubKey, NULL /*error*/);
      if (pubKeyRep) {
        NSData *pubKeyData = CFBridgingRelease(pubKeyRep);
        pubKeyB64 = CopyNSString([pubKeyData base64EncodedStringWithOptions:0]);
      }
      CFRelease(pubKey);
    }

    CFDateRef creationDate =
        (CFDateRef)CFDictionaryGetValue(attrs, kSecAttrCreationDate);
    NSDate *nsDate = (__bridge NSDate *)creationDate;
    NSISO8601DateFormatter *formatter = [[NSISO8601DateFormatter alloc] init];
    NSString *isoCreationDate = [formatter stringFromDate:nsDate];

    (*infosOut + infosLen)->label = CopyNSString(nsLabel);
    (*infosOut + infosLen)->app_label = CopyNSString(nsAppLabel);
    (*infosOut + infosLen)->app_tag = CopyNSString(nsAppTag);
    (*infosOut + infosLen)->pub_key_b64 = pubKeyB64;
    (*infosOut + infosLen)->creation_date = CopyNSString(isoCreationDate);
    infosLen++;
  }

  CFRelease(items);
  return infosLen;
}

int FindCredentials(LabelFilter filter, CredentialInfo **infosOut) {
  return findCredentials(YES /* applyFilter */, filter, infosOut);
}

int ListCredentials(const char *reason, CredentialInfo **infosOut,
                    char **errOut) {
  LAContext *ctx = [[LAContext alloc] init];

  __block LabelFilter filter;
  filter.kind = LABEL_PREFIX;
  filter.value = "";

  __block int res;
  __block NSString *nsError = NULL;

  // A semaphore is needed, otherwise we return before the prompt has a chance
  // to resolve.
  dispatch_semaphore_t sema = dispatch_semaphore_create(0);
  [ctx evaluatePolicy:LAPolicyDeviceOwnerAuthenticationWithBiometrics
      localizedReason:[NSString stringWithUTF8String:reason]
                reply:^void(BOOL success, NSError *_Nullable error) {
                  if (success) {
                    res =
                        findCredentials(NO /* applyFilter */, filter, infosOut);
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
  }

  return res;
}

OSStatus deleteCredential(const char *appLabel) {
  NSData *nsAppLabel = [NSData dataWithBytes:appLabel length:strlen(appLabel)];
  NSDictionary *query = @{
    (id)kSecClass : (id)kSecClassKey,
    (id)kSecAttrKeyType : (id)kSecAttrKeyTypeECSECPrimeRandom,
    (id)kSecMatchLimit : (id)kSecMatchLimitOne,
    (id)kSecAttrApplicationLabel : nsAppLabel,
  };
  return SecItemDelete((__bridge CFDictionaryRef)query);
}

int DeleteCredential(const char *reason, const char *appLabel, char **errOut) {
  LAContext *ctx = [[LAContext alloc] init];

  __block int res;
  __block NSString *nsError = NULL;

  // A semaphore is needed, otherwise we return before the prompt has a chance
  // to resolve.
  dispatch_semaphore_t sema = dispatch_semaphore_create(0);
  [ctx evaluatePolicy:LAPolicyDeviceOwnerAuthenticationWithBiometrics
      localizedReason:[NSString stringWithUTF8String:reason]
                reply:^void(BOOL success, NSError *_Nullable error) {
                  if (success) {
                    res = deleteCredential(appLabel);
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

int DeleteNonInteractive(const char *appLabel) {
  return deleteCredential(appLabel);
}
