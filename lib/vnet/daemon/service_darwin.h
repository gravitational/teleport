#ifndef service_darwin_h
#define service_darwin_h

#import <Foundation/Foundation.h>

@interface VNEDaemonService : NSObject

-(id)init;

// start begins listening for incoming XPC connections.
-(void)start;

// stop stops listening for incoming XPC connections.
-(void)stop;

-(void)waitForVnetConfig;

@end

void DaemonStart(void);
void DaemonStop(void);

typedef struct VnetConfigResult {
    bool ok;
    const char * error_description;
    const char * socket_path;
    const char * ipv6_prefix;
    const char * dns_addr;
    const char * home_path;
} VnetConfigResult;

// WaitForVnetConfig blocks until a client calls the daemon
// with config necessary to start VNet.
void WaitForVnetConfig(struct VnetConfigResult *result);

#endif /* service_darwin_h */
