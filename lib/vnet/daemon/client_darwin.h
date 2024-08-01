#ifndef TELEPORT_LIB_VNET_DAEMON_CLIENT_DARWIN_H_
#define TELEPORT_LIB_VNET_DAEMON_CLIENT_DARWIN_H_

#include "common_darwin.h"

#import <Foundation/Foundation.h>

typedef struct RegisterDaemonResult {
  bool ok;
  // service_status is fetched even if [service registerAndReturnError] fails,
  // for debugging purposes.
  int service_status;
  const char *error_description;
} RegisterDaemonResult;

// RegisterDaemon attempts to register the daemon. After the registration attempt,
// it fetches the daemon status.
// Pretty much a noop if the daemon is already registered and enabled.
//
// bundle_path must be an absolute path to the app bundle.
//
// The caller should check outResult.ok to see if the call succeeded.
void RegisterDaemon(const char *bundle_path, RegisterDaemonResult *outResult);

// DaemonStatus returns the current status of the daemon's service in SMAppService.
// Returns -1 if the given macOS version doesn't support SMAppService.
// The rest of values directly corresponds to values from SMAppServiceStatus enum.
// See client_darwin.go for a direct mapping.
// https://developer.apple.com/documentation/servicemanagement/smappservice/status-swift.enum?language=objc
//
// bundle_path must be an absolute path to the app bundle.
int DaemonStatus(const char *bundle_path);

// OpenSystemSettingsLoginItems opens the Login Items section of system settings.
// Should be used in conjunction with a message guiding the user towards enabling
// the login item for the daemon.
void OpenSystemSettingsLoginItems(void);

typedef struct StartVnetRequest {
  const char *bundle_path;

  const char *socket_path;
  const char *ipv6_prefix;
  const char *dns_addr;
  const char *home_path;
} StartVnetRequest;

typedef struct StartVnetResult {
  bool ok;
  // error_domain is either VNEErrorDomain, NSOSStatusErrorDomain, or NSCocoaErrorDomain.
  const char *error_domain;
  // If error_domain is set to VNEErrorDomain, error_code is one of the VNE codes from common_darwin.h.
  // If error_domain is NSOSStatusErrorDomain, error_code comes from OSStatus of Code Signing framework.
  // https://developer.apple.com/documentation/security/1574088-code_signing_services_result_cod?language=objc
  // If error_domain is NSCocoaErrorDomain, it's likely to be about XPC. It's best to inspect it
  // on https://osstatus.com in that case.
  int error_code;
  // error_description includes the full representation of the error, including domain and code.
  const char *error_description;
} StartVnetResult;

// StartVnet spawns the daemon process. Only the first call does that,
// subsequent calls are noops. The daemon process exits after the socket file
// in request.vnet_config.socket_path is removed. After that it can be spawned
// again by calling StartVnet.
//
// Blocks until the daemon receives the message or until the client gets
// invalidated.
//
// After calling StartVnet, the caller is expected to call InvalidateDaemonClient
// when a surrounding context in Go gets canceled, to check outResult.ok to see
// if the client was able to connect to the daemon, and to free strings in StartVnetResult.
void StartVnet(StartVnetRequest *request, StartVnetResult *outResult);

// InvalidateDaemonClient closes the connection to the daemon and unblocks
// any calls awaiting a reply from the daemon.
void InvalidateDaemonClient(void);

@interface VNEDaemonClient : NSObject
- (void)startVnet:(VNEConfig *)config completion:(void (^)(NSError *error))completion;
// invalidate executes all outstanding reply blocks, error handling blocks,
// and invalidation blocks and forbids from sending or receiving new messages.
- (void)invalidate;
@end

#endif /* TELEPORT_LIB_VNET_DAEMON_CLIENT_DARWIN_H_ */
