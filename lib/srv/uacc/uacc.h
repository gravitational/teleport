/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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

int UACC_UTMP_MISSING_PERMISSIONS = 1;
int UACC_UTMP_WRITE_ERROR = 2;
int UACC_UTMP_READ_ERROR = 3;
int UACC_UTMP_FAILED_OPEN = 4;
int UACC_UTMP_ENTRY_DOES_NOT_EXIST = 5;
int UACC_UTMP_FAILED_TO_SELECT_FILE = 6;
int UACC_UTMP_OTHER_ERROR = 7;
int UACC_UTMP_PATH_DOES_NOT_EXIST = 8;

// I initially attempted to use the login/logout BSD functions but ran into a string of unexpected behaviours such as
// errno being set to undocument values along with wierd return values in certain cases. They also modify the utmp database
// in a way we don't want. We want to insert a USER_PROCESS entry directly before we do PAM/cgroup setup and launch the shell
// without any middleman. I brought this information with my issues and requirements to the IRC and was told that I would be
// better off using the setutent/pututline low level manipulation functions - joel.

// `strncpy` is used a bit in this code and this comment attempts to briefly explain why. Back when C interfaces and libraries were
// being standardized a choice was to make the NUL terminator optional for strings occupying fixed-size buffers in certain interfaces.
// This decision is one of the origins of `strncpy` which has the special property that it will not write a NUL terminator if the
// source string excluding the NUL terminator is of equal or greater length in comparison to the limit parameter.

static int status_from_errno() {
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

static int check_abs_path_err(const char* buffer, char* uaccPathErr) {
    // check for errors
    if (buffer == NULL) {
        return status_from_errno();
    }

    // check for GNU extension errors
    if (errno == EACCES || errno == ENOENT) {
        strcpy(uaccPathErr, buffer);
        return UACC_UTMP_OTHER_ERROR;
    }

    // no error was found
    return 0;
}

// The max byte length of the C string representing the TTY name.
static int max_len_tty_name() {
    return UT_LINESIZE;
}

// Low level C function to add a new USER_PROCESS entry to the database.
// This function does not perform any argument validation.
static int uacc_add_utmp_entry(const char *utmp_path, const char *wtmp_path, const char *username,
  const char *hostname, const int32_t remote_addr_v6[4], const char *tty_name, const char *id,
  int32_t tv_sec, int32_t tv_usec, char* uaccPathErr) {

    if (utmp_path == NULL || wtmp_path == NULL) {
      // Return open failed error if any of the provided paths is NULL.
      return UACC_UTMP_FAILED_OPEN;
    }

    char resolved_utmp_buffer[PATH_MAX];
    const char* file = realpath(utmp_path, &resolved_utmp_buffer[0]);

    int status = check_abs_path_err(file, uaccPathErr);
    if (status != 0) {
        return status;
    }
    if (utmpname(file) < 0) {
        return UACC_UTMP_FAILED_TO_SELECT_FILE;
    }
    struct utmp entry = {
      .ut_type = USER_PROCESS,
      .ut_pid = getpid(),
      .ut_session = getsid(0),
      .ut_tv.tv_sec = tv_sec,
      .ut_tv.tv_usec = tv_usec
    };
    strncpy((char*) &entry.ut_line, tty_name, UT_LINESIZE);
    strncpy((char*) &entry.ut_id, id, sizeof(entry.ut_id));
    strncpy((char*) &entry.ut_host, hostname, sizeof(entry.ut_host));
    strncpy((char*) &entry.ut_user, username, sizeof(entry.ut_user));
    memcpy(&entry.ut_addr_v6, remote_addr_v6, sizeof(int32_t) * 4);

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
    const char* wtmp_file = realpath(wtmp_path, &resolved_wtmp_buffer[0]);
    status = check_abs_path_err(wtmp_file, uaccPathErr);
    if (status != 0) {
        return status;
    }
    updwtmp(wtmp_file, &entry);
    if (errno != 0) {
        return status_from_errno();
    }
    return 0;
}

// Low level C function to mark a database entry as DEAD_PROCESS.
// This function does not perform string argument validation.
static int uacc_mark_utmp_entry_dead(const char *utmp_path, const char *wtmp_path, const char *tty_name,
        int32_t tv_sec, int32_t tv_usec, char* uaccPathErr) {
    if (utmp_path == NULL || wtmp_path == NULL) {
      // Return open failed error if any of the provided paths is NULL.
      return UACC_UTMP_FAILED_OPEN;
    }

    char resolved_utmp_buffer[PATH_MAX];
    const char* file = realpath(utmp_path, &resolved_utmp_buffer[0]);
    int status = check_abs_path_err(file, uaccPathErr);
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
    const char* wtmp_file = realpath(wtmp_path, &resolved_wtmp_buffer[0]);
    status = check_abs_path_err(wtmp_file, uaccPathErr);
    if (status != 0) {
        return status;
    }
    updwtmp(wtmp_file, &log_entry);
    if (errno != 0) {
        return status_from_errno();
    }
    return 0;
}

// Low level C function to check the database for an entry for a given user.
// This function does not perform string argument validation.
static int uacc_has_entry_with_user(const char *utmp_path, const char *user, char *uaccPathErr) {
    if (utmp_path == NULL) {
        // Return open failed error if any of the provided paths is NULL.
        return UACC_UTMP_FAILED_OPEN;
    }

    char resolved_utmp_buffer[PATH_MAX];
    const char* file = realpath(utmp_path, &resolved_utmp_buffer[0]);
    int status = check_abs_path_err(file, uaccPathErr);
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

// Low level C function to add a new entry to the failed login log.
// This function does not perform any argument validation.
static int uacc_add_btmp_entry(const char *btmp_path, const char *username,
  const char *hostname, const int32_t remote_addr_v6[4],
  int32_t tv_sec, int32_t tv_usec, char *uaccPathErr) {

    if (btmp_path == NULL) {
      // Return open failed error if the provided path is NULL.
      return UACC_UTMP_FAILED_OPEN;
    }

    char resolved_btmp_buffer[PATH_MAX];
    const char* file = realpath(btmp_path, &resolved_btmp_buffer[0]);

    int status = check_abs_path_err(file, uaccPathErr);
    if (status != 0) {
        return status;
    }

    struct utmp entry = {
        .ut_type = USER_PROCESS,
        .ut_tv.tv_sec = tv_sec,
        .ut_tv.tv_usec = tv_usec
    };
    strncpy((char*) &entry.ut_host, hostname, sizeof(entry.ut_host));
    strncpy((char*) &entry.ut_user, username, sizeof(entry.ut_user));
    memcpy(&entry.ut_addr_v6, remote_addr_v6, sizeof(int32_t) * 4);

    updwtmp(file, &entry);
    if (errno != 0) {
        return status_from_errno();
    }
    return 0;
}

#endif
