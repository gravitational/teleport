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

#include "diag.h"

#import <CoreFoundation/CoreFoundation.h>
#import <Foundation/Foundation.h>
#import <LocalAuthentication/LocalAuthentication.h>
#import <Security/Security.h>

#include "common.h"

void CheckSignatureAndEntitlements(DiagResult *diagOut) {
  // Get code object for running binary.
  SecCodeRef code = NULL;
  if (SecCodeCopySelf(kSecCSDefaultFlags, &code) != errSecSuccess) {
    return;
  }

  // Get signing information from code object.
  // Succeeds even for non-signed binaries.
  CFDictionaryRef info = NULL;
  if (SecCodeCopySigningInformation(code, kSecCSDefaultFlags, &info) !=
      errSecSuccess) {
    CFRelease(code);
    return;
  }

  // kSecCodeInfoIdentifier is present for signed code, absent otherwise.
  diagOut->has_signature =
      CFDictionaryContainsKey(info, kSecCodeInfoIdentifier);

  // kSecCodeInfoEntitlementsDict is only present in signed/entitled binaries.
  // We go a step further and check if keychain-access-groups are present.
  // Put together, this is a reasonable proxy for a proper-built binary.
  CFDictionaryRef entitlements =
      CFDictionaryGetValue(info, kSecCodeInfoEntitlementsDict);
  if (entitlements != NULL) {
    diagOut->has_entitlements =
        CFDictionaryContainsKey(entitlements, @"keychain-access-groups");
  }

  CFRelease(info);
  CFRelease(code);
}

void RunDiag(DiagResult *diagOut) {
  // Writes has_signature and has_entitlements to diagOut.
  CheckSignatureAndEntitlements(diagOut);

  // Attempt a simple LAPolicy check.
  // This fails if Touch ID is not available or cannot be used for various
  // reasons (no password set, device locked, lid is closed, etc).
  LAContext *ctx = [[LAContext alloc] init];
  NSError *laError = NULL;
  diagOut->passed_la_policy_test =
      [ctx canEvaluatePolicy:LAPolicyDeviceOwnerAuthenticationWithBiometrics
                       error:&laError];
  if (laError) {
    diagOut->la_error_code = [laError code];
    diagOut->la_error_domain = CopyNSString([laError domain]);
    diagOut->la_error_description = CopyNSString([laError description]);
  }

  // Attempt to write a non-permanent key to the enclave.
  NSDictionary *attributes = @{
    (id)kSecAttrKeyType : (id)kSecAttrKeyTypeECSECPrimeRandom,
    (id)kSecAttrKeySizeInBits : @256,
    (id)kSecAttrTokenID : (id)kSecAttrTokenIDSecureEnclave,
    (id)kSecAttrIsPermanent : @NO,
  };
  CFErrorRef error = NULL;
  SecKeyRef privateKey =
      SecKeyCreateRandomKey((__bridge CFDictionaryRef)(attributes), &error);
  if (privateKey) {
    diagOut->passed_secure_enclave_test = true;
    CFRelease(privateKey);
  }
  if (error) {
    CFRelease(error);
  }
}
