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

#include "authenticate.h"

#import <CoreFoundation/CoreFoundation.h>
#import <Foundation/Foundation.h>
#import <Security/Security.h>

#include "common.h"

int Authenticate(AuthenticateRequest req, char **sigB64Out, char **errOut) {
  NSData *appLabel = [NSData dataWithBytes:req.app_label
                                    length:strlen(req.app_label)];
  NSDictionary *query = @{
    (id)kSecClass : (id)kSecClassKey,
    (id)kSecAttrKeyType : (id)kSecAttrKeyTypeECSECPrimeRandom,
    (id)kSecMatchLimit : (id)kSecMatchLimitOne,
    (id)kSecReturnRef : @YES,
    (id)kSecAttrApplicationLabel : appLabel,
  };
  SecKeyRef privateKey = NULL;
  OSStatus status = SecItemCopyMatching((__bridge CFDictionaryRef)query,
                                        (CFTypeRef *)&privateKey);
  if (status != errSecSuccess) {
    CFStringRef err = SecCopyErrorMessageString(status, NULL);
    NSString *nsErr = (__bridge_transfer NSString *)err;
    *errOut = CopyNSString(nsErr);
    return -1;
  }

  NSData *digest = [NSData dataWithBytes:req.digest length:req.digest_len];
  CFErrorRef error = NULL;
  CFDataRef sig = SecKeyCreateSignature(
      privateKey, kSecKeyAlgorithmECDSASignatureDigestX962SHA256,
      (__bridge CFDataRef)digest, &error);
  if (error) {
    NSError *nsError = (__bridge_transfer NSError *)error;
    *errOut = CopyNSString([nsError localizedDescription]);
    CFRelease(privateKey);
    return -1;
  }
  NSData *nsSig = (__bridge_transfer NSData *)sig;
  *sigB64Out = CopyNSString([nsSig base64EncodedStringWithOptions:0]);

  CFRelease(privateKey);
  return 0;
}
