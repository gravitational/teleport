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
        
        NSLog(@"Registering daemon plist=%@", DaemonPlist());
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

int DaemonStatus(void) {
    if (@available(macOS 13, *)) {
        SMAppService *service;
        service = [SMAppService daemonServiceWithPlistName:(DaemonPlist())];
        return (int) service.status;
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

@property (nonatomic, strong, readwrite) NSXPCConnection *connection;

@end

@implementation VNEDaemonClient

-(NSXPCConnection *)connection {
    // Create the XPC Connection on demand.
    if (_connection == nil) {
        _connection = [[NSXPCConnection alloc] initWithMachServiceName:DaemonLabel() options:NSXPCConnectionPrivileged];
        _connection.remoteObjectInterface = [NSXPCInterface interfaceWithProtocol:@protocol(VNEDaemonProtocol)];
        _connection.invalidationHandler = ^{
            self->_connection = nil;
            NSLog(@"connection has been invalidated");
        };
        _connection.interruptionHandler = ^{
            NSLog(@"connection has been interrupted");
        };
        
        // New connections always start in a suspended state.
        [_connection resume];
    }
    return _connection;
}

-(void)startVnet:(VnetConfig *)vnetConfig completion:(void (^)(NSError *))completion {
    // This way of calling the XPC proxy ensures either the error handler or
    // the reply block gets called.
    // https://forums.developer.apple.com/forums/thread/713429
    id proxy = [self.connection remoteObjectProxyWithErrorHandler: ^(NSError * error) {
        completion(error);
    }];
    
    [(id<VNEDaemonProtocol>) proxy startVnet:vnetConfig completion:^(void) {
        completion(nil);
    }];
}

-(void)invalidate {
    if (_connection) {
        [_connection invalidate];
    }
}

@end

static VNEDaemonClient *daemonClient = NULL;

void StartVnet(struct StartVnetRequest *request, struct StartVnetResponse *response) {
    if (!daemonClient) {
        daemonClient = [[VNEDaemonClient alloc] init];
    }
    
    dispatch_semaphore_t sema = dispatch_semaphore_create(0);
    
    [daemonClient startVnet:request->vnet_config completion:^(NSError * error) {
        if (error) {
            response->error_domain = VNECopyNSString([error domain]);
            response->error_description = VNECopyNSString([error description]);
            dispatch_semaphore_signal(sema);
            return;
        }
        
        response->ok = true;
        dispatch_semaphore_signal(sema);
    }];
    
    dispatch_semaphore_wait(sema, DISPATCH_TIME_FOREVER);
}

void InvalidateDaemonClient(void) {
    if (daemonClient) {
        [daemonClient invalidate];
    }
}

