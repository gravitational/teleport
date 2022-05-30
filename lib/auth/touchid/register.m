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

#include "register.h"

#import <CoreFoundation/CoreFoundation.h>
#import <Foundation/Foundation.h>
#import <Security/Security.h>

#include <string.h>

#include "common.h"

int Register(CredentialInfo req, char **pubKeyB64Out, char **errOut) {
  CFErrorRef error = NULL;
  SecAccessControlRef access = SecAccessControlCreateWithFlags(
      kCFAllocatorDefault, kSecAttrAccessibleWhenUnlockedThisDeviceOnly,
      kSecAccessControlPrivateKeyUsage | kSecAccessControlBiometryAny, &error);
  if (error) {
    NSError *nsError = CFBridgingRelease(error);
    *errOut = CopyNSString([nsError localizedDescription]);
    return -1;
  }

  NSDictionary *attributes = @{
    // Enclave requires EC/256 bits keys.
    (id)kSecAttrKeyType : (id)kSecAttrKeyTypeECSECPrimeRandom,
    (id)kSecAttrKeySizeInBits : @256,
    (id)kSecAttrTokenID : (id)kSecAttrTokenIDSecureEnclave,

    (id)kSecPrivateKeyAttrs : @{
      (id)kSecAttrIsPermanent : @YES,
      (id)kSecAttrAccessControl : (__bridge id)access,

      (id)kSecAttrLabel : [NSString stringWithUTF8String:req.label],
      (id)kSecAttrApplicationLabel :
          [NSData dataWithBytes:req.app_label length:strlen(req.app_label)],
      (id)kSecAttrApplicationTag : [NSData dataWithBytes:req.app_tag
                                                  length:strlen(req.app_tag)],
    },
  };
  SecKeyRef privateKey =
      SecKeyCreateRandomKey((__bridge CFDictionaryRef)(attributes), &error);
  if (error) {
    NSError *nsError = CFBridgingRelease(error);
    *errOut = CopyNSString([nsError localizedDescription]);
    CFRelease(access);
    return -1;
  }

  SecKeyRef publicKey = SecKeyCopyPublicKey(privateKey);
  if (!publicKey) {
    *errOut = CopyNSString(@"failed to copy public key");
    CFRelease(privateKey);
    CFRelease(access);
    return -1;
  }

  CFDataRef publicKeyRep = SecKeyCopyExternalRepresentation(publicKey, &error);
  if (error) {
    NSError *nsError = CFBridgingRelease(error);
    *errOut = CopyNSString([nsError localizedDescription]);
    CFRelease(publicKey);
    CFRelease(privateKey);
    CFRelease(access);
    return -1;
  }
  NSData *publicKeyData = CFBridgingRelease(publicKeyRep);
  *pubKeyB64Out =
      CopyNSString([publicKeyData base64EncodedStringWithOptions:0]);

  CFRelease(publicKey);
  CFRelease(privateKey);
  CFRelease(access);
  return 0;
}
