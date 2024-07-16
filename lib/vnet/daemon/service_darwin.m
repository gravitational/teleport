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
#include "protocol_darwin.h"
#include "service_darwin.h"

#import <Foundation/Foundation.h>
#include <dispatch/dispatch.h>

#include <string.h>

@interface VNEDaemonService () <NSXPCListenerDelegate, VNEDaemonProtocol>

@property(nonatomic, strong, readwrite) NSXPCListener *listener;
@property(nonatomic, readwrite) BOOL started;

@property(nonatomic, readwrite) NSString *socketPath;
@property(nonatomic, readwrite) NSString *ipv6Prefix;
@property(nonatomic, readwrite) NSString *dnsAddr;
@property(nonatomic, readwrite) NSString *homePath;
@property(nonatomic, readwrite) dispatch_semaphore_t gotVnetConfigSema;

@end

@implementation VNEDaemonService

- (id)initWithBundlePath:(NSString *)bundlePath {
  self = [super init];
  if (self) {
    // Launch daemons must configure their listener with the machServiceName
    // initializer.
    _listener = [[NSXPCListener alloc] initWithMachServiceName:DaemonLabel(bundlePath)];
    _listener.delegate = self;

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

- (void)startVnet:(VnetConfig *)vnetConfig completion:(void (^)(void))completion {
  @synchronized(self) {
    _socketPath = @(vnetConfig->socket_path);
    _ipv6Prefix = @(vnetConfig->ipv6_prefix);
    _dnsAddr = @(vnetConfig->dns_addr);
    _homePath = @(vnetConfig->home_path);
    dispatch_semaphore_signal(_gotVnetConfigSema);
    completion();
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

void DaemonStart(const char *bundle_path) {
  if (daemonService) {
    return;
  }
  daemonService = [[VNEDaemonService alloc] initWithBundlePath:@(bundle_path)];
  [daemonService start];
}

void DaemonStop(void) {
  if (daemonService && [daemonService started]) {
    [daemonService stop];
  }
}

void WaitForVnetConfig(VnetConfigResult *outResult) {
  if (!daemonService) {
    outResult->error_description = strdup("daemon was not initialized yet");
    return;
  }

  if (![daemonService started]) {
    outResult->error_description = strdup("daemon was not started yet");
  }

  [daemonService waitForVnetConfig];

  if (![daemonService started]) {
    outResult->error_description = strdup("daemon was stopped while waiting for VNet config");
    return;
  }

  @synchronized(daemonService) {
    outResult->socket_path = VNECopyNSString([daemonService socketPath]);
    outResult->ipv6_prefix = VNECopyNSString([daemonService ipv6Prefix]);
    outResult->dns_addr = VNECopyNSString([daemonService dnsAddr]);
    outResult->home_path = VNECopyNSString([daemonService homePath]);
    outResult->ok = true;
  }
}
