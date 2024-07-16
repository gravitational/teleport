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

#include "client_darwin.h"
#include "common_darwin.h"
#include "protocol_darwin.h"

#import <Foundation/Foundation.h>
#import <ServiceManagement/ServiceManagement.h>
#include <dispatch/dispatch.h>

#include <string.h>

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

@interface VNEDaemonClient ()

@property(nonatomic, strong, readwrite) NSXPCConnection *connection;
@property(nonatomic, strong, readonly) NSString *bundlePath;

@end

@implementation VNEDaemonClient

- (id)initWithBundlePath:(NSString *)bundlePath {
  self = [super init];
  if (self) {
    _bundlePath = bundlePath;
  }
  return self;
}

- (NSXPCConnection *)connection {
  // Create the XPC Connection on demand.
  if (_connection == nil) {
    _connection = [[NSXPCConnection alloc] initWithMachServiceName:DaemonLabel(_bundlePath)
                                                           options:NSXPCConnectionPrivileged];
    _connection.remoteObjectInterface =
        [NSXPCInterface interfaceWithProtocol:@protocol(VNEDaemonProtocol)];
    _connection.invalidationHandler = ^{
      self->_connection = nil;
    };

    // New connections always start in a suspended state.
    [_connection resume];
  }
  return _connection;
}

- (void)startVnet:(VnetConfig *)vnetConfig completion:(void (^)(NSError *))completion {
  // This way of calling the XPC proxy ensures either the error handler or
  // the reply block gets called.
  // https://forums.developer.apple.com/forums/thread/713429
  id proxy = [self.connection remoteObjectProxyWithErrorHandler:^(NSError *error) {
    completion(error);
  }];

  [(id<VNEDaemonProtocol>)proxy startVnet:vnetConfig
                               completion:^(void) {
                                 completion(nil);
                               }];
}

- (void)invalidate {
  if (_connection) {
    [_connection invalidate];
  }
}

@end

static VNEDaemonClient *daemonClient = NULL;

void StartVnet(StartVnetRequest *request, StartVnetResult *outResult) {
  if (!daemonClient) {
    daemonClient = [[VNEDaemonClient alloc] initWithBundlePath:@(request->bundle_path)];
  }

  dispatch_semaphore_t sema = dispatch_semaphore_create(0);

  [daemonClient startVnet:request->vnet_config
               completion:^(NSError *error) {
                 if (error) {
                   outResult->error_domain = VNECopyNSString([error domain]);
                   outResult->error_description = VNECopyNSString([error description]);
                   dispatch_semaphore_signal(sema);
                   return;
                 }

                 outResult->ok = true;
                 dispatch_semaphore_signal(sema);
               }];

  dispatch_semaphore_wait(sema, DISPATCH_TIME_FOREVER);
}

void InvalidateDaemonClient(void) {
  if (daemonClient) {
    [daemonClient invalidate];
  }
}
