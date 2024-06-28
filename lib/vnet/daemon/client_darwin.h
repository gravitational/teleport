#ifndef client_darwin_h
#define client_darwin_h

#import <Foundation/Foundation.h>

typedef struct BundlePathResult {
    const char * bundlePath;
} BundlePathResult;

// BundlePath updates bundlePath of result with the path
// to the bundle directory that contains the current executable.
// bundlePath is an empty string if the bundle details could not be fetched.
// It might return a path even for executables that are not in a bundle.
// In that case, calling codesign --verify on that path will simply return with 1.
void BundlePath(struct BundlePathResult *result);

// VNECopyNSString duplicates an NSString into an UTF-8 encoded C string.
// The caller is expected to free the returned pointer.
char *VNECopyNSString(NSString *val);

#endif /* client_darwin_h */
