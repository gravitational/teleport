#ifndef TELEPORT_LIB_UTILS_LOG_OSLOG_DARWIN_H
#define TELEPORT_LIB_UTILS_LOG_OSLOG_DARWIN_H

#import <os/log.h>

// TELCreateLog creates a new os_log_t object. This function returns void* rather than os_log_t so that Go code
// can operate on the pointer as unsafe.Pointer.
//
// The logging runtime maintains a global collection of all os_log_t objects, one per subsystem/category pair.
// These objects are never deallocated. See os_log_create(3) for more details.
void* TELCreateLog(const char *subsystem, const char *category);

// TELLog logs the message as public on the given log with the given type (see os_log_type_t).
// log is expected to be a pointer to os_log_t.
void TELLog(void *log, uint type, const char *message);

#endif /* TELEPORT_LIB_UTILS_LOG_OSLOG_DARWIN_H */
