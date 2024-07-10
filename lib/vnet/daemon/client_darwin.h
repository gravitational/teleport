#ifndef TELEPORT_LIB_VNET_DAEMON_CLIENT_DARWIN_H_
#define TELEPORT_LIB_VNET_DAEMON_CLIENT_DARWIN_H_

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

#endif /* TELEPORT_LIB_VNET_DAEMON_CLIENT_DARWIN_H_ */
