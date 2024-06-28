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
#import <ServiceManagement/ServiceManagement.h>

#include "client_darwin.h"

void BundlePath(struct BundlePathResult *result) {
    // From the docs:
    // > This method may return a valid bundle object even for unbundled apps.
    // > It may also return nil if the bundle object could not be created,
    // > so always check the return value.
    NSBundle *main = [NSBundle mainBundle];
    if (!main) {
        result->bundlePath = strdup("");
        return;
    }
    
    result->bundlePath = VNECopyNSString([main bundlePath]);
}

NSString * DaemonLabel(void) {
    NSBundle *main = [NSBundle mainBundle];
    if (!main) {
        return @"";
    }
    
    NSString *bundleIdentifier = [main bundleIdentifier];
    if ([bundleIdentifier length] == 0) {
        return bundleIdentifier;
    }
    
    return [NSString stringWithFormat:@"%@.vnetd", bundleIdentifier];
}

NSString * DaemonPlist(void) {
    NSString *label = DaemonLabel();
    if ([label length] == 0) {
        return label;
    }
    
    return [NSString stringWithFormat:@"%@.plist", label];
}

void RegisterDaemon(struct RegisterDaemonResult *result) {
    if (@available(macOS 13, *)) {
        SMAppService *service;
        NSError *error;
        
        service = [SMAppService daemonServiceWithPlistName:(DaemonPlist())];
        
        result->ok = [service registerAndReturnError:(&error)];
        if (error) {
            result->error_description = VNECopyNSString([error description]);
        }
        
        // Grabbing the service status for debugging purposes, no matter if
        // [service registerAndReturnError] succeeded or failed.
        result->service_status = (int) service.status;
    } else {
        result->error_description = strdup("Service Management APIs are available on macOS 13.0+");
    }
}

char *VNECopyNSString(NSString *val) {
    if (val) {
        return strdup([val UTF8String]);
    }
    return strdup("");
}
