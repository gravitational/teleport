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

#include "authenticate.h"

#import <CoreFoundation/CoreFoundation.h>
#import <Foundation/Foundation.h>
#import <Security/Security.h>

#include "common.h"
#include "context.h"

int Authenticate(AuthContext *actx, AuthenticateRequest req, char **sigB64Out,
                 char **errOut) {
  NSData *appLabel = [NSData dataWithBytes:req.app_label
                                    length:strlen(req.app_label)];
  NSDictionary *query = @{
    (id)kSecClass : (id)kSecClassKey,
    (id)kSecAttrKeyType : (id)kSecAttrKeyTypeECSECPrimeRandom,
    (id)kSecMatchLimit : (id)kSecMatchLimitOne,
    (id)kSecReturnRef : @YES,
    (id)kSecAttrApplicationLabel : appLabel,
    // ctx takes effect in the SecKeyCreateSignature call below.
    (id)kSecUseAuthenticationContext : (id)GetLAContextFromAuth(actx),
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
