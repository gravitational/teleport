#ifndef TELEPORT_LIB_VNET_DAEMON_SERVICE_DARWIN_H_
#define TELEPORT_LIB_VNET_DAEMON_SERVICE_DARWIN_H_

#import <Foundation/Foundation.h>

@interface VNEDaemonService : NSObject

- (id)initWithBundlePath:(NSString *)bundlePath codeSigningRequirement:(NSString *)codeSigningRequirement;

// start begins listening for incoming XPC connections.
- (void)start;

// stop stops listening for incoming XPC connections.
- (void)stop;

// waitForVnetConfig blocks until either startVnet signals that a VNet config was received
// or until stop signals that the daemon has stopped listening for incoming connections.
- (void)waitForVnetConfig;

@end

typedef struct DaemonStartResult {
  bool ok;
  // error_domain is set to either VNEErrorDomain or NSOSStatusErrorDomain if ok is false.
  const char *error_domain;
  // If error_domain is set to VNEErrorDomain, error_code is one of the VNE codes from common_darwin.h.
  // If error_domain is NSOSStatusErrorDomain, error_code comes from OSStatus of Code Signing framework.
  // https://developer.apple.com/documentation/security/1574088-code_signing_services_result_cod?language=objc
  int error_code;
  // error_description includes the full representation of the error, including domain and code.
  const char *error_description;
} DaemonStartResult;

// DaemonStart initializes the XPC service and starts listening for new connections.
// It's expected to be called only once, noop if the daemon was already started.
// It might fail if it runs into problems with Code Signing APIs while calucating the code signing
// requirement. In such case, outResult.ok is set to false and the error fields are populated.
void DaemonStart(const char *bundle_path, DaemonStartResult *outResult);
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

typedef struct ClientCred {
  // valid is set if the euid and egid fields have been set.
  bool valid;
  // egid is the effective group ID of the process on the other side of the XPC connection.
  gid_t egid;
  // euid is the effective user ID of the process on the other side of the XPC connection.
  uid_t euid;
} ClientCred;

// WaitForVnetConfig blocks until a client calls the daemon with a config necessary to start VNet.
// It can be interrupted by calling DaemonStop.
//
// The caller is expected to check outResult.ok to see if the call succeeded and to free strings
// in VnetConfigResult.
void WaitForVnetConfig(VnetConfigResult *outResult, ClientCred *outClientCred);

#endif /* TELEPORT_LIB_VNET_DAEMON_SERVICE_DARWIN_H_ */
