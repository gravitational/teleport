#ifndef TELEPORT_LIB_UTILS_DARWINBUNDLE_DARWIN_H_
#define TELEPORT_LIB_UTILS_DARWINBUNDLE_DARWIN_H_

#import <Foundation/Foundation.h>

// TELBundleIdentifier returns the identifier of the bundle under the current path.
// Always returns a string, but it might be empty if there's no bundle under the given path or the bundle
// details couldn't have been read.
NSString *TELBundleIdentifier(NSString *bundlePath);

#endif /* TELEPORT_LIB_UTILS_DARWINBUNDLE_DARWIN_H_ */
