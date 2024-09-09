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
#include "service_darwin.h"

#import <Foundation/Foundation.h>
#include <dispatch/dispatch.h>

#include <string.h>

@interface VNEClientCred : NSObject
@property BOOL valid;
@property gid_t egid;
@property uid_t euid;
@end

@implementation VNEClientCred
@end

@interface VNEDaemonService () <NSXPCListenerDelegate, VNEDaemonProtocol>

// started describes whether the XPC listener is listening for new connections.
@property(readonly) BOOL started;
@property(readonly) VNEConfig *config;
@property(readonly) VNEClientCred *clientCred;

@end

@implementation VNEDaemonService {
  NSXPCListener *_listener;
  dispatch_semaphore_t _gotVnetConfigSema;
}

- (id)initWithBundlePath:(NSString *)bundlePath codeSigningRequirement:(NSString *)codeSigningRequirement {
  self = [super init];
  if (self) {
    // Launch daemons must configure their listener with the machServiceName initializer.
    _listener = [[NSXPCListener alloc] initWithMachServiceName:DaemonLabel(bundlePath)];
    _listener.delegate = self;

    // The daemon won't even be started on macOS < 13.0, so we don't have to handle the else branch
    // of this condition.
    if (@available(macOS 13, *)) {
      [_listener setConnectionCodeSigningRequirement:codeSigningRequirement];
    }

    _started = NO;
    _gotVnetConfigSema = dispatch_semaphore_create(0);
  }
  return self;
}

- (void)start {
  // Begin listening for incoming XPC connections.
  [_listener resume];

  _started = YES;
}

- (void)stop {
  // Stop listening for incoming XPC connections.
  [_listener suspend];

  _started = NO;
  dispatch_semaphore_signal(_gotVnetConfigSema);
}

- (void)waitForVnetConfig {
  dispatch_semaphore_wait(_gotVnetConfigSema, DISPATCH_TIME_FOREVER);
}

#pragma mark - VNEDaemonProtocol

- (void)startVnet:(VNEConfig *)config completion:(void (^)(NSError *error))completion {
  @synchronized(self) {
    // startVnet is expected to be called only once per daemon's lifetime.
    // Between the process with the daemon client exiting and the admin process (which runs the
    // daemon) noticing this and exiting as well, a new client can be spawned and startVnet might
    // end up getting called again.
    //
    // In such scenarios, we want to return an error so that the client can wait for the daemon
    // to exit and retry the call.
    if (_config != nil) {
      NSError *error = [[NSError alloc] initWithDomain:@(VNEErrorDomain)
                                                  code:VNEAlreadyRunningError
                                              userInfo:nil];
      completion(error);
      return;
    }

    _config = config;

    NSXPCConnection *currentConn = [NSXPCConnection currentConnection];
    _clientCred = [[VNEClientCred alloc] init];
    [_clientCred setEgid:[currentConn effectiveGroupIdentifier]];
    [_clientCred setEuid:[currentConn effectiveUserIdentifier]];
    [_clientCred setValid:YES];

    dispatch_semaphore_signal(_gotVnetConfigSema);
    completion(nil);
  }
}

#pragma mark - NSXPCListenerDelegate

- (BOOL)listener:(NSXPCListener *)listener
    shouldAcceptNewConnection:(NSXPCConnection *)newConnection {
  // Configure the incoming connection.
  newConnection.exportedInterface =
      [NSXPCInterface interfaceWithProtocol:@protocol(VNEDaemonProtocol)];
  newConnection.exportedObject = self;

  // New connections always start in a suspended state.
  [newConnection resume];

  return YES;
}

@end

static VNEDaemonService *daemonService = NULL;

void DaemonStart(const char *bundle_path, DaemonStartResult *outResult) {
  if (daemonService) {
    outResult->ok = true;
    return;
  }

  NSString *requirement = nil;
  NSError *error = nil;
  bool ok = getCodeSigningRequirement(&requirement, &error);
  if (!ok) {
    outResult->ok = false;
    outResult->error_domain = VNECopyNSString([error domain]);
    outResult->error_code = (int)[error code];
    outResult->error_description = VNECopyNSString([error description]);
    return;
  }
  
  daemonService = [[VNEDaemonService alloc] initWithBundlePath:@(bundle_path) codeSigningRequirement:requirement];
  [daemonService start];
  outResult->ok = true;
}

void DaemonStop(void) {
  if (daemonService && daemonService.started) {
    [daemonService stop];
  }
}

void WaitForVnetConfig(VnetConfigResult *outResult, ClientCred *outClientCred) {
  if (!daemonService) {
    outResult->error_description = strdup("daemon was not initialized yet");
    return;
  }

  if (!daemonService.started) {
    outResult->error_description = strdup("daemon was not started yet");
  }

  [daemonService waitForVnetConfig];

  if (!daemonService.started) {
    outResult->error_description = strdup("daemon was stopped while waiting for VNet config");
    return;
  }

  @synchronized(daemonService) {
    outResult->socket_path = VNECopyNSString(daemonService.config.socketPath);
    outResult->ipv6_prefix = VNECopyNSString(daemonService.config.ipv6Prefix);
    outResult->dns_addr = VNECopyNSString(daemonService.config.dnsAddr);
    outResult->home_path = VNECopyNSString(daemonService.config.homePath);

    if (daemonService.clientCred && [daemonService.clientCred valid]) {
      outClientCred->egid = daemonService.clientCred.egid;
      outClientCred->euid = daemonService.clientCred.euid;
      outClientCred->valid = true;
    }
    
    outResult->ok = true;
  }
}
