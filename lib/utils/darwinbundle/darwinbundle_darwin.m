// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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
#include "darwinbundle_darwin.h"

#import <Foundation/Foundation.h>

#include <string.h>

// TELCopyNSString converts and copies an Obj-C string to a C string which can be used with cgo.
// The caller is expected to free the returned pointer.
char *TELCopyNSString(NSString *val) {
  if (!val) {
    return strdup("");
  }
  const char *utf8String = [val UTF8String];
  if (!utf8String) {
    return strdup("");
  }
  return strdup(utf8String);
}

const char *BundleIdentifier(const char *bundlePath) {
  // Obj-C automatically marks objects for release, but it expects the program to mark points where
  // those objects can be released. Many Apple frameworks handle that automatically, but since this
  // code is not executed within a context of a framework, it must create those points.
  // BundleIdentifier is called from Go code, so it's a good candidate to be wrapped in
  // @autoreleasepool, which makes the compiler automatically insert such points and release Obj-C
  // objects that are no longer needed (such as bundleIdentifier).
  @autoreleasepool {
    NSString *bundleIdentifier = TELBundleIdentifier(@(bundlePath));
    return TELCopyNSString(bundleIdentifier);
  }
}

NSString *TELBundleIdentifier(NSString *bundlePath) {
  NSBundle *main = [NSBundle bundleWithPath:bundlePath];
  if (!main) {
    return @"";
  }

  NSString *bundleIdentifier = [main bundleIdentifier];
  if (!bundleIdentifier || [bundleIdentifier length] == 0) {
    return @"";
  }

  return bundleIdentifier;
}
