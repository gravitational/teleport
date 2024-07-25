#ifndef TELEPORT_LIB_VNET_DAEMON_COMMON_DARWIN_H_
#define TELEPORT_LIB_VNET_DAEMON_COMMON_DARWIN_H_

#import <Foundation/Foundation.h>

// VNEErrorDomain is a custom error domain used for Objective-C errors that pertain to VNet.
extern const char* const VNEErrorDomain;

// VNEAlreadyRunningError indicates that the daemon already received a VNet config.
// It won't accept a new one during its lifetime, instead it's expected to stop, after
// which the client might spawn a new instance of the daemon.
extern const int VNEAlreadyRunningError;

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

// Returns the label for the daemon by getting the identifier of the bundle
// this executable is shipped in and appending ".vnetd" to it.
//
// The returned string might be empty if the executable is not in a bundle.
//
// The filename and the value of the Label key in the plist file and the Mach
// service of of the daemon must match the string returned from this function.
NSString *DaemonLabel(NSString *bundlePath);

// VNECopyNSString duplicates an NSString into an UTF-8 encoded C string.
// The caller is expected to free the returned pointer.
const char *VNECopyNSString(NSString *val);

#endif /* TELEPORT_LIB_VNET_DAEMON_COMMON_DARWIN_H_ */
