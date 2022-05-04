#ifndef COMMON_H_
#define COMMON_H_

#import <Foundation/Foundation.h>

// CopyNSString duplicates an NSString into an UTF-8 encoded C string.
// The caller is expected to free the returned pointer.
char *CopyNSString(NSString *val);

#endif // COMMON_H_
