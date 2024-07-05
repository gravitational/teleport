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

#include "client_darwin.h"

#import <Foundation/Foundation.h>
#import <ServiceManagement/ServiceManagement.h>

// VNECopyNSString duplicates an NSString into an UTF-8 encoded C string.
// The caller is expected to free the returned pointer.
char *VNECopyNSString(NSString *val) {
  if (val) {
    return strdup([val UTF8String]);
  }
  return strdup("");
}

// Returns the label for the daemon by getting the identifier of the bundle
// this executable is shipped in and appending ".vnetd" to it.
//
// The returned string might be empty if the executable is not in a bundle.
//
// The filename and the value of the Label key in the plist file and the Mach
// service of of the daemon must match the string returned from this function.
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

// DaemonPlist takes the result of DaemonLabel and appends ".plist" to it
// if not empty.
NSString *DaemonPlist(NSString *bundlePath) {
  NSString *label = DaemonLabel(bundlePath);
  if ([label length] == 0) {
    return label;
  }

  return [NSString stringWithFormat:@"%@.plist", label];
}

void RegisterDaemon(const char *bundle_path, RegisterDaemonResult *outResult) {
  if (@available(macOS 13, *)) {
    SMAppService *service;
    NSError *error;

    service = [SMAppService daemonServiceWithPlistName:(DaemonPlist(@(bundle_path)))];

    outResult->ok = [service registerAndReturnError:(&error)];
    if (error) {
      outResult->error_description = VNECopyNSString([error description]);
    }

    // Grabbing the service status for debugging purposes, no matter if
    // [service registerAndReturnError] succeeded or failed.
    outResult->service_status = (int)service.status;
  } else {
    outResult->error_description = strdup("Service Management APIs only available on macOS 13.0+");
  }
}

int DaemonStatus(const char *bundle_path) {
  if (@available(macOS 13, *)) {
    SMAppService *service;
    service = [SMAppService daemonServiceWithPlistName:(DaemonPlist(@(bundle_path)))];
    return (int)service.status;
  } else {
    return -1;
  }
}

void OpenSystemSettingsLoginItems(void) {
  if (@available(macOS 13, *)) {
    [SMAppService openSystemSettingsLoginItems];
  }
}
