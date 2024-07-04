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
#import "common_darwin.h"
#import "protocol_darwin.h"
#import "service_darwin.h"

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

- (id)init {
    // Launch daemons must configure their listener with the machServiceName
    // initializer.
    _listener = [[NSXPCListener alloc] initWithMachServiceName:DaemonLabel()];
    _listener.delegate = self;
    
    _started = NO;
    _gotVnetConfigSema = dispatch_semaphore_create(0);
    
    return self;
}

- (void)start {
    assert(_started == NO);
    
    // Begin listening for incoming XPC connections.
    [_listener resume];
    
    _started = YES;
}

- (void)stop {
    assert(_started == YES);
    
    // Stop listening for incoming XPC connections.
    [_listener suspend];
    
    _started = NO;
    dispatch_semaphore_signal(_gotVnetConfigSema);
}

- (void)waitForVnetConfig {
    assert(_started == YES);
    dispatch_semaphore_wait(_gotVnetConfigSema, DISPATCH_TIME_FOREVER);
}

#pragma mark - VNEDaemonProtocol

- (void)startVnet:(VnetConfig *)vnetConfig
       completion:(void (^)(void))completion {
    _socketPath = @(vnetConfig->socket_path);
    _ipv6Prefix = @(vnetConfig->ipv6_prefix);
    _dnsAddr = @(vnetConfig->dns_addr);
    _homePath = @(vnetConfig->home_path);
    dispatch_semaphore_signal(_gotVnetConfigSema);
    completion();
}

#pragma mark - NSXPCListenerDelegate

- (BOOL)listener:(NSXPCListener *)listener
shouldAcceptNewConnection:(NSXPCConnection *)newConnection {
    assert(listener == _listener);
    assert(newConnection != nil);
    
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

void DaemonStart(void) {
    daemonService = [[VNEDaemonService alloc] init];
    [daemonService start];
}

void DaemonStop(void) {
    if (daemonService && [daemonService started]) {
        [daemonService stop];
    }
}

void WaitForVnetConfig(struct VnetConfigResult *result) {
    if (!daemonService) {
        result->error_description = strdup("daemon was not initialized yet");
        return;
    }
    
    [daemonService waitForVnetConfig];
    
    if (![daemonService started]) {
        result->error_description =
        strdup("daemon was stopped while waiting for VNet config");
        return;
    }
    
    result->socket_path = VNECopyNSString([daemonService socketPath]);
    result->ipv6_prefix = VNECopyNSString([daemonService ipv6Prefix]);
    result->dns_addr = VNECopyNSString([daemonService dnsAddr]);
    result->home_path = VNECopyNSString([daemonService homePath]);
    result->ok = true;
}

