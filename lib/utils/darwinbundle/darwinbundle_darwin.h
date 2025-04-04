#ifndef TELEPORT_LIB_UTILS_DARWINBUNDLE_DARWIN_H_
#define TELEPORT_LIB_UTILS_DARWINBUNDLE_DARWIN_H_

#import <Foundation/Foundation.h>

// BundleIdentifier returns the identifier of the bundle under the given path.
// The caller is expected to free the returned pointer.
const char *BundleIdentifier(const char *bundlePath);

// TELBundleIdentifier returns the identifier of the bundle under the given path.
// Always returns a string, but it might be empty if there's no bundle under the given path or the bundle
// details couldn't have been read.
NSString *TELBundleIdentifier(NSString *bundlePath);

#endif /* TELEPORT_LIB_UTILS_DARWINBUNDLE_DARWIN_H_ */
