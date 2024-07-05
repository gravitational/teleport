#ifndef client_darwin_h
#define client_darwin_h

#import <Foundation/Foundation.h>

// Returns the label for the daemon by getting the identifier of the bundle
// this executable is shipped in and appending ".vnetd" to it.
//
// The returned string might be empty if the executable is not in a bundle.
//
// The filename and the value of the Label key in the plist file and the Mach
// service of of the daemon must match the string returned from this function.
NSString *DaemonLabel(void);

// DaemonPlist takes the result of DaemonLabel and appends ".plist" to it
// if not empty.
NSString *DaemonPlist(void);

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
void RegisterDaemon(struct RegisterDaemonResult *result);

// DaemonStatus returns the current status of the daemon's service in SMAppService.
// Returns -1 if the given macOS version doesn't support SMAppService.
int DaemonStatus(void);

void OpenSystemSettingsLoginItems(void);

// VNECopyNSString duplicates an NSString into an UTF-8 encoded C string.
// The caller is expected to free the returned pointer.
char *VNECopyNSString(NSString *val);

#endif /* client_darwin_h */
