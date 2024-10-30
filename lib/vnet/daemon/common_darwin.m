//go:build vnetdaemon
// +build vnetdaemon

// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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
#include "common_darwin.h"

#import <CoreFoundation/CoreFoundation.h>
#import <Foundation/Foundation.h>
#import <Security/CodeSigning.h>

#include <string.h>

const char* const VNEErrorDomain = "com.Gravitational.Vnet.ErrorDomain";

const int VNEAlreadyRunningError = 1;
const int VNEMissingCodeSigningIdentifiersError = 2;

NSString *DaemonLabel(NSString *bundlePath) {
  NSBundle *main = [NSBundle bundleWithPath:bundlePath];
  if (!main) {
    return @"";
  }

  NSString *bundleIdentifier = [main bundleIdentifier];
  if (!bundleIdentifier || [bundleIdentifier length] == 0) {
    return @"";
  }

  return [NSString stringWithFormat:@"%@.vnetd", bundleIdentifier];
}

const char *VNECopyNSString(NSString *val) {
  if (val) {
    return strdup([val UTF8String]);
  }
  return strdup("");
}

bool getCodeSigningRequirement(NSString **outRequirement, NSError **outError) {
  SecCodeRef codeObj = nil;
  OSStatus status = SecCodeCopySelf(kSecCSDefaultFlags, &codeObj);
  if (status != errSecSuccess) {
    if (outError) {
      *outError = [NSError errorWithDomain:NSOSStatusErrorDomain code:status userInfo:nil];
    }
    return false;
  }

  CFDictionaryRef cfCodeSignInfo = nil;
  // kSecCSSigningInformation must be provided as a flag for the team identifier to be included
  // in the returned dictionary.
  status = SecCodeCopySigningInformation(codeObj, kSecCSSigningInformation, &cfCodeSignInfo);
  // codeObj is no longer needed. Manually release it, as we own it since we got it from a function
  // with "Copy" in its name.
  // https://developer.apple.com/library/archive/documentation/CoreFoundation/Conceptual/CFMemoryMgmt/Concepts/Ownership.html#//apple_ref/doc/writerid/cfCreateRule
  CFRelease(codeObj);
  if (status != errSecSuccess) {
    if (outError) {
      *outError = [NSError errorWithDomain:NSOSStatusErrorDomain code:status userInfo:nil];
    }
    return false;
  }

  // Transfer ownership of cfCodeSignInfo to Obj-C, which means we don't have to CFRelease it manually.
  // We can transfer the ownership of cfCodeSignInfo because we own it (we got it from a function
  // with "Copy" in its name).
  // https://developer.apple.com/documentation/foundation/1587932-cfbridgingrelease
  NSDictionary *codeSignInfo = (NSDictionary *)CFBridgingRelease(cfCodeSignInfo);
  // We don't own kSecCodeInfoIdentifier, so we cannot call CFBridgingRelease on it.
  // __bridge transfers a pointer between Obj-C and CoreFoundation with no transfer of ownership.
  // Values extracted out of codeSignInfo are cast to toll-free bridged Obj-C types. 
  // https://developer.apple.com/library/archive/documentation/CoreFoundation/Conceptual/CFDesignConcepts/Articles/tollFreeBridgedTypes.html#//apple_ref/doc/uid/TP40010677-SW2
  // https://stackoverflow.com/questions/18067108/when-should-you-use-bridge-vs-cfbridgingrelease-cfbridgingretain
  NSString *identifier = codeSignInfo[(__bridge NSString *)kSecCodeInfoIdentifier];
  NSString *teamIdentifier = codeSignInfo[(__bridge NSString *)kSecCodeInfoTeamIdentifier];

  if (!identifier || [identifier length] == 0 || !teamIdentifier || [teamIdentifier length] == 0) {
    if (outError) {
      *outError = [NSError errorWithDomain:@(VNEErrorDomain) code:VNEMissingCodeSigningIdentifiersError userInfo:nil];
    }
    return false;
  }

  // The requirement will be matched against the designated requirement of the application on
  // the other side of an XPC connection. It is based on the designated requirement of tsh.app.
  // To inspect the designated requirement of an app, use the following command:
  //
  //     codesign --display -r - <path to app>
  //
  // Breakdown of individual parts of the requirement:
  // * `identifier "foo"` is satisfied if the code signing identifier matches the provided one.
  //   It is not the same as the bundle identifier.
  // * `anchor apple generic` is satisfied by any code signed with any code signing identity issued
  //   by Apple.
  // * `certificate leaf[field(bunch of specific numbers)]` is satisfied by code signed with
  //   Developer ID Application certs.
  // * `certificate leaf[subject.OU]` is satisfied by certs with a specific Team ID.
  //
  // Read more at:
  // https://developer.apple.com/documentation/technotes/tn3127-inside-code-signing-requirements#Designated-requirement
  // https://developer.apple.com/documentation/technotes/tn3127-inside-code-signing-requirements#Xcode-designated-requirement-for-Developer-ID-code
  if (outRequirement) {
    *outRequirement = [NSString stringWithFormat:@"identifier \"%@\" and anchor apple generic and certificate leaf[field.1.2.840.113635.100.6.1.13] and certificate leaf[subject.OU] = %@", identifier, teamIdentifier];
  }

  return true;
}

@implementation VNEConfig
+ (BOOL)supportsSecureCoding {
  return YES;
}

- (void)encodeWithCoder:(nonnull NSCoder *)coder {
  [coder encodeObject:self.socketPath forKey:@"socketPath"];
  [coder encodeObject:self.ipv6Prefix forKey:@"ipv6Prefix"];
  [coder encodeObject:self.dnsAddr forKey:@"dnsAddr"];
  [coder encodeObject:self.homePath forKey:@"homePath"];
}

- (nullable instancetype)initWithCoder:(nonnull NSCoder *)coder {
  if (self = [super init]) {
    [self setSocketPath:[coder decodeObjectOfClass:[NSString class] forKey:@"socketPath"]];
    [self setIpv6Prefix:[coder decodeObjectOfClass:[NSString class] forKey:@"ipv6Prefix"]];
    [self setDnsAddr:[coder decodeObjectOfClass:[NSString class] forKey:@"dnsAddr"]];
    [self setHomePath:[coder decodeObjectOfClass:[NSString class] forKey:@"homePath"]];
  }
  return self;
}

@end
