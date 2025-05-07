/* Author: Trusted Computer Solutions, Inc. */
#include <selinux/selinux.h>

#define SETRANS_UNIX_SOCKET SELINUX_TRANS_DIR "/.setrans-unix"

#define RAW_TO_TRANS_CONTEXT		2
#define TRANS_TO_RAW_CONTEXT		3
#define RAW_CONTEXT_TO_COLOR		4
#define MAX_DATA_BUF			8192

