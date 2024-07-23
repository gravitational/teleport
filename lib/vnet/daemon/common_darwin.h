#ifndef TELEPORT_LIB_VNET_DAEMON_COMMON_DARWIN_H_
#define TELEPORT_LIB_VNET_DAEMON_COMMON_DARWIN_H_

#import <Foundation/Foundation.h>

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
