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

#import <Foundation/Foundation.h>

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
