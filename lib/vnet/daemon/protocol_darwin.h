#ifndef TELEPORT_LIB_VNET_DAEMON_PROTOCOL_DARWIN_H_
#define TELEPORT_LIB_VNET_DAEMON_PROTOCOL_DARWIN_H_

#import <Foundation/Foundation.h>

static NSString *const VNEErrorDomain = @"com.Gravitational.Vnet.ErrorDomain";

typedef enum VNEErrorCode {
  // VNEAlreadyRunningError indicates that the daemon already received a VNet config.
  // It won't accept a new one during its lifetime, instead it's expected to stop, after
  // which the client might spawn a new instance of the daemon.
  VNEAlreadyRunningError,
} VNEErrorCode;

typedef struct VnetConfig {
  const char *socket_path;
  const char *ipv6_prefix;
  const char *dns_addr;
  const char *home_path;
} VnetConfig;

@protocol VNEDaemonProtocol
// startVnet passes the config back to Go code (which then starts VNet in a separate thread)
// and returns immediately.
//
// Only the first call to this method starts VNet. Subsequent calls return VNEAlreadyRunningError.
// The daemon process exits after VNet is stopped, after which it can be spawned again by calling
// this method.
- (void)startVnet:(VnetConfig *)vnetConfig completion:(void (^)(NSError *error))completion;
@end

#endif /* TELEPORT_LIB_VNET_DAEMON_PROTOCOL_DARWIN_H_ */
