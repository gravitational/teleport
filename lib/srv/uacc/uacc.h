/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

#ifndef UACC_C
#define UACC_C

#include <unistd.h>
#include <stdint.h>
#include <string.h>
#include <utmp.h>
#include <errno.h>
#include <stdbool.h>
#include <stdlib.h>
#include <limits.h>

// Sometimes the _UTMP_PATH and _WTMP_PATH macros from glibc are bad, this seems to depend on distro.
// I asked around on IRC, no one really knows why. I suspect it's another
// archaic remnant of old Unix days and that a cleanup is long overdue.
//
// In the meantime, we just try to resolve from these paths instead.
#define UACC_UTMP_PATH "/var/run/utmp"
#define UACC_WTMP_PATH "/var/run/wtmp"

int UACC_UTMP_MISSING_PERMISSIONS = 1;
int UACC_UTMP_WRITE_ERROR = 2;
int UACC_UTMP_READ_ERROR = 3;
int UACC_UTMP_FAILED_OPEN = 4;
int UACC_UTMP_ENTRY_DOES_NOT_EXIST = 5;
int UACC_UTMP_FAILED_TO_SELECT_FILE = 6;
int UACC_UTMP_OTHER_ERROR = 7;
int UACC_UTMP_PATH_DOES_NOT_EXIST = 8;

// At first glance this may seem unsafe.
// This pointer however is protected by the mutex lock that's enforced on all uacc logic on the Go side.
char* UACC_PATH_ERR;

// I initially attempted to use the login/logout BSD functions but ran into a string of unexpected behaviours such as
// errno being set to undocument values along with wierd return values in certain cases. They also modify the utmp database
// in a way we don't want. We want to insert a USER_PROCESS entry directly before we do PAM/cgroup setup and launch the shell
// without any middleman. I brought this information with my issues and requirements to the IRC and was told that I would be
// better off using the setutent/pututline low level manipulation functions - joel.

// `strncpy` is used a bit in this code and this comment attempts to briefly explain why. Back when C interfaces and libraries were
// being standardized a choice was to make the NUL terminator optional for strings occupying fixed-size buffers in certain interfaces.
// This decision is one of the origins of `strncpy` which has the special property that it will not write a NUL terminator if the
// source string excluding the NUL terminator is of equal or greater length in comparison to the limit parameter.

// get_absolute_path_with_fallback attempts to resolve the `supplied_path` path. If `supplied` is null it will
// resolve the `fallback_path` path. The resolved path is stored in `buffer` and will at most be
// `PATH_MAX` long including the null terminator.
static char* get_absolute_path_with_fallback(char* buffer, const char* supplied_path, const char* fallback_path) {
    const char* path;

    if (supplied_path != NULL) {
        path = supplied_path;
    } else {
        path = fallback_path;
    }

    return realpath(path, buffer);
}

static int check_abs_path_err(const char* buffer) {
    // check for errors
    if (buffer == NULL) {
        switch errno {
            case EACCES: return UACC_UTMP_MISSING_PERMISSIONS;
            case EINVAL: return UACC_UTMP_FAILED_OPEN;
            case EIO: return UACC_UTMP_READ_ERROR;
            case ELOOP: return UACC_UTMP_FAILED_OPEN;
            case ENAMETOOLONG: return UACC_UTMP_FAILED_OPEN;
            case ENOENT: return UACC_UTMP_PATH_DOES_NOT_EXIST;
            case ENOTDIR: return UACC_UTMP_FAILED_OPEN;
            default: return UACC_UTMP_OTHER_ERROR;
        }
    }

    // check for GNU extension errors
    if (errno == EACCES || errno == ENOENT) {
        UACC_PATH_ERR = (char*)malloc(PATH_MAX);
        strcpy(UACC_PATH_ERR, buffer);
        return UACC_UTMP_OTHER_ERROR;
    }

    // no error was found
    return 0;
}

// Allow the Go side to read errno.
static int get_errno() {
    return errno;
}

// The max byte length of the C string representing the TTY name.
static int max_len_tty_name() {
    return UT_LINESIZE;
}

// Low level C function to add a new USER_PROCESS entry to the database.
// This function does not perform any argument validation.
static int uacc_add_utmp_entry(const char *utmp_path, const char *wtmp_path, const char *username, const char *hostname, const int32_t remote_addr_v6[4], const char *tty_name, const char *id, int32_t tv_sec, int32_t tv_usec) {
    UACC_PATH_ERR = NULL;
    char resolved_utmp_buffer[PATH_MAX];
    const char* file = get_absolute_path_with_fallback(&resolved_utmp_buffer[0], utmp_path, UACC_UTMP_PATH);
    int status = check_abs_path_err(file);
    if (status != 0) {
        return status;
    }
    if (utmpname(file) < 0) {
        return UACC_UTMP_FAILED_TO_SELECT_FILE;
    }
    struct utmp entry;
    entry.ut_type = USER_PROCESS;
    strncpy((char*) &entry.ut_line, tty_name, UT_LINESIZE);
    strncpy((char*) &entry.ut_id, id, sizeof(entry.ut_id));
    entry.ut_pid = getpid();
    strncpy((char*) &entry.ut_host, hostname, sizeof(entry.ut_host));
    strncpy((char*) &entry.ut_user, username, sizeof(entry.ut_user));
    entry.ut_session = getsid(0);
    entry.ut_tv.tv_sec = tv_sec;
    entry.ut_tv.tv_usec = tv_usec;
    memcpy(&entry.ut_addr_v6, &remote_addr_v6, sizeof(int32_t) * 4);
    errno = 0;
    setutent();
    if (errno > 0) {
        endutent();
        return UACC_UTMP_FAILED_OPEN;
    }
    if (pututline(&entry) == NULL) {
        endutent();
        return errno == EPERM || errno == EACCES ? UACC_UTMP_MISSING_PERMISSIONS : UACC_UTMP_WRITE_ERROR;
    }
    endutent();
    char resolved_wtmp_buffer[PATH_MAX];
    const char* wtmp_file = get_absolute_path_with_fallback(&resolved_wtmp_buffer[0], wtmp_path, UACC_WTMP_PATH);
    status = check_abs_path_err(wtmp_file);
    if (status != 0) {
        return status;
    }
    updwtmp(wtmp_file, &entry);
    return 0;
}

// Low level C function to mark a database entry as DEAD_PROCESS.
// This function does not perform string argument validation.
static int uacc_mark_utmp_entry_dead(const char *utmp_path, const char *wtmp_path, const char *tty_name, int32_t tv_sec, int32_t tv_usec) {
    UACC_PATH_ERR = NULL;
    char resolved_utmp_buffer[PATH_MAX];
    const char* file = get_absolute_path_with_fallback(&resolved_utmp_buffer[0], utmp_path, UACC_UTMP_PATH);
    int status = check_abs_path_err(file);
    if (status != 0) {
        return status;
    }
    if (utmpname(file) < 0) {
        return UACC_UTMP_FAILED_TO_SELECT_FILE;
    }
    errno = 0;
    setutent();
    if (errno > 0) {
        return UACC_UTMP_FAILED_OPEN;
    }
    struct utmp line;
    strncpy((char*) &line.ut_line, tty_name, UT_LINESIZE);
    struct utmp *entry_t = getutline(&line);
    if (entry_t == NULL) {
        return UACC_UTMP_READ_ERROR;
    }
    struct utmp entry;
    memcpy(&entry, entry_t, sizeof(struct utmp));
    entry.ut_type = DEAD_PROCESS;
    memset(&entry.ut_user, 0, UT_NAMESIZE);
    struct utmp log_entry = entry;
    log_entry.ut_tv.tv_sec = tv_sec;
    log_entry.ut_tv.tv_usec = tv_usec;
    memset(&entry.ut_host, 0, UT_HOSTSIZE);
    memset(&entry.ut_line, 0, UT_LINESIZE);
    memset(&entry.ut_time, 0, 8);
    errno = 0;
    setutent();
    if (errno != 0) {
        endutent();
        return UACC_UTMP_FAILED_OPEN;
    }
    if (pututline(&entry) == NULL) {
        endutent();
        return errno == EPERM || errno == EACCES ? UACC_UTMP_MISSING_PERMISSIONS : UACC_UTMP_WRITE_ERROR;
    }
    endutent();
    char resolved_wtmp_buffer[PATH_MAX];
    const char* wtmp_file = get_absolute_path_with_fallback(&resolved_wtmp_buffer[0], wtmp_path, UACC_WTMP_PATH);
    status = check_abs_path_err(wtmp_file);
    if (status != 0) {
        return status;
    }
    updwtmp(wtmp_file, &log_entry);
    return 0;
}

// Low level C function to check the database for an entry for a given user.
// This function does not perform string argument validation.
static int uacc_has_entry_with_user(const char *utmp_path, const char *user) {
    UACC_PATH_ERR = NULL;
    char resolved_utmp_buffer[PATH_MAX];
    const char* file = get_absolute_path_with_fallback(&resolved_utmp_buffer[0], utmp_path, UACC_UTMP_PATH);
    int status = check_abs_path_err(file);
    if (status != 0) {
        return status;
    }
    if (utmpname(file) < 0) {
        return UACC_UTMP_FAILED_TO_SELECT_FILE;
    }
    errno = 0;
    setutent();
    if (errno != 0) {
        endutent();
        return UACC_UTMP_FAILED_OPEN;
    }
    struct utmp *entry = getutent();
    while (entry != NULL) {
        if (entry->ut_type == USER_PROCESS && strncmp(user, entry->ut_user, sizeof(entry->ut_user)) == 0) {
            endutent();
            return 0;
        }
        entry = getutent();
    }
    endutent();
    return errno == 0 ? UACC_UTMP_ENTRY_DOES_NOT_EXIST : errno;
}

#endif
