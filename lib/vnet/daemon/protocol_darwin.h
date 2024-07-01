#ifndef TELEPORT_LIB_VNET_DAEMON_PROTOCOL_DARWIN_H_
#define TELEPORT_LIB_VNET_DAEMON_PROTOCOL_DARWIN_H_

#import <Foundation/Foundation.h>

typedef struct VnetParams {
  const char *socket_path;
  const char *ipv6_prefix;
  const char *dns_addr;
} VnetParams;

@protocol VNEDaemonProtocol
// startVnet starts VNet in a separate thread and returns immediately.
// Only the first call to this method starts VNet. Subsequent calls are noops.
// The daemon process exits after VNet is stopped, after which it can be
// spawned again by calling this method.
- (void)startVnet:(VnetParams *)vnetParams completion:(void (^)(void))completion;
@end

#endif /* TELEPORT_LIB_VNET_DAEMON_PROTOCOL_DARWIN_H_ */
