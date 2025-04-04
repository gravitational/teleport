#ifndef TELEPORT_LIB_UTILS_LOG_OSLOG_DARWIN_H
#define TELEPORT_LIB_UTILS_LOG_OSLOG_DARWIN_H

#import <os/log.h>

// TELCreateLog creates a new os_log_t object. The caller is expected to manually CFRelease it when
// the object is no longer needed. This function returns void* rather than os_log_t so that Go code
// can operate on the pointer as unsafe.Pointer.
void* TELCreateLog(const char *subsystem, const char *category);

// TELLog logs the message as public on the given log with the given type (see os_log_type_t).
// log is expected to be a pointer to os_log_t.
void TELLog(void *log, uint type, const char *message);

// TELReleaseLog releases the object which log points to.
void TELReleaseLog(void *log);

#endif /* TELEPORT_LIB_UTILS_LOG_OSLOG_DARWIN_H */
