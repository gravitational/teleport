#ifndef TELEPORT_LIB_VNET_DAEMON_SERVICE_DARWIN_H_
#define TELEPORT_LIB_VNET_DAEMON_SERVICE_DARWIN_H_

#import <Foundation/Foundation.h>

@interface VNEDaemonService : NSObject

- (id)initWithBundlePath:(NSString *)bundlePath;

// start begins listening for incoming XPC connections.
- (void)start;

// stop stops listening for incoming XPC connections.
- (void)stop;

// waitForVnetConfig blocks until either startVnet signals that a VNet config was received
// or until stop signals that the daemon has stopped listening for incoming connections.
- (void)waitForVnetConfig;

@end

// DaemonStart initializes the XPC service and starts listening for new connections.
void DaemonStart(const char *bundle_path);
// DaemonStop stops the XPC service. Noop if DaemonStart wasn't called.
void DaemonStop(void);

typedef struct VnetConfigResult {
  bool ok;
  const char *error_description;
  const char *socket_path;
  const char *ipv6_prefix;
  const char *dns_addr;
  const char *home_path;
} VnetConfigResult;

// WaitForVnetConfig blocks until a client calls the daemon with a config necessary to start VNet.
// It can be interrupted by calling DaemonStop.
//
// The caller is expected to check outResult.ok to see if the call succeeded and to free strings
// in VnetConfigResult.
void WaitForVnetConfig(VnetConfigResult *outResult);

#endif /* TELEPORT_LIB_VNET_DAEMON_SERVICE_DARWIN_H_ */
