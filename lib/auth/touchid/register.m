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
