// +build pam,cgo

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

#include "_cgo_export.h"
#include <stdio.h>
#include <stdlib.h>
#include <stdint.h>
#include <dlfcn.h>
#include <security/pam_appl.h>

// library_name returns the name of the library to load at runtime.
char *library_name()
{
#ifdef __APPLE__
    return "libpam.dylib";
#else
    return "libpam.so.0";
#endif
}

// converse is called by PAM to interact with the user. Interaction means
// either writing something to stdout and stderr or reading something from
// stdin.
int converse(int n, const struct pam_message **msg, struct pam_response **resp, void *data)
{
    int i;
    struct pam_response *aresp;

    // If no messages arrived, or the number of messages is greater than
    // allowed, something is wrong with the caller.
    if (n <= 0 || n > PAM_MAX_NUM_MSG) {
        return PAM_CONV_ERR;
    }

    // According to pam_conv(3): "It is the caller's responsibility to release
    // both, this array and the responses themselves, using free(3)." The
    // caller in this situation is the PAM module and the array and the
    // responses refer to aresp and aresp[i].resp.
    aresp = calloc(n, sizeof *aresp);
    if (aresp == NULL) {
        return PAM_BUF_ERR;
    }

    // Loop over all messages and process them.
    for (i = 0; i < n; ++i) {
        aresp[i].resp_retcode = 0;
        aresp[i].resp = NULL;

        switch (msg[i]->msg_style) {
        case PAM_PROMPT_ECHO_OFF:
            // Read back response from user. What the user writes should not
            // be echoed to the screen.
            aresp[i].resp = readCallback((uintptr_t)data, 0);
            if (aresp[i].resp == NULL) {
                goto fail;
            }
            break;
        case PAM_PROMPT_ECHO_ON:
            // First write the message to stderr.
            writeCallback((uintptr_t)data, STDERR_FILENO, (char *)(msg[i]->msg));

            // Read back response from user. What the user writes will be
            // echoed to the screen.
            aresp[i].resp = readCallback((uintptr_t)data, 1);
            if (aresp[i].resp == NULL) {
                goto fail;
            }
            break;
        case PAM_ERROR_MSG:
            // Write message to stderr.
            writeCallback((uintptr_t)data, STDERR_FILENO, (char *)(msg[i]->msg));
            if (strlen(msg[i]->msg) > 0 && msg[i]->msg[strlen(msg[i]->msg) - 1] != '\n') {
                writeCallback((uintptr_t)data, STDERR_FILENO, (char *)"\n");
            }
            break;
        case PAM_TEXT_INFO:
            // Write message to stdout.
            writeCallback((uintptr_t)data, STDOUT_FILENO, (char *)(msg[i]->msg));
            if (strlen(msg[i]->msg) > 0 && msg[i]->msg[strlen(msg[i]->msg) - 1] != '\n') {
                writeCallback((uintptr_t)data, STDOUT_FILENO, (char *)"\n");
            }

            break;
        default:
            goto fail;
        }
    }
    *resp = aresp;
    return PAM_SUCCESS;

 fail:
    for (i = 0; i < n; ++i) {
        if (aresp[i].resp != NULL) {
            memset(aresp[i].resp, 0, strlen(aresp[i].resp));
            free(aresp[i].resp);
        }
    }
    memset(aresp, 0, n * sizeof *aresp);
    free(aresp);
    *resp = NULL;
    return PAM_CONV_ERR;
}

// make_pam_conv creates a PAM conversation function used by PAM to interact
// with the user.
struct pam_conv *make_pam_conv(int n)
{
    // This memory allocation will be released in the Close function.
    struct pam_conv *conv = (struct pam_conv *)calloc(1, sizeof(struct pam_conv));

    // The converse is the actual callback function above.
    conv->conv = converse;

    // The callback handler index in Go code is stored as the value of the
    // pointer. This is done to avoid another memory allocation that needs a
    // free call later. According to the C standard this is okay.
    //
    // https://wiki.sei.cmu.edu/confluence/display/c/INT36-C.+Converting+a+pointer+to+integer+or+integer+to+pointer
    conv->appdata_ptr = (void *)(uintptr_t)n;

    return conv;
}

int _pam_start(void *handle, const char *service_name, const char *user, const struct pam_conv *pam_conversation, pam_handle_t **pamh)
{
    int (*f)(const char *, const char *, const struct pam_conv *, pam_handle_t **);

    f = dlsym(handle, "pam_start");
    if (f == NULL) {
        return PAM_ABORT;
    }

    return (f)(service_name, user, pam_conversation, pamh);
}

int _pam_end(void *handle, pam_handle_t *pamh, int pam_status)
{
    int (*f)(pam_handle_t *, int);

    f = dlsym(handle, "pam_end");
    if (f == NULL) {
        return PAM_ABORT;
    }

    return (f)(pamh, pam_status);
}

int _pam_putenv(void *handle, pam_handle_t *pamh, const char *name_value)
{
    int (*f)(pam_handle_t *, const char *);

    f = dlsym(handle, "pam_putenv");
    if (f == NULL) {
        return PAM_ABORT;
    }

    return (f)(pamh, name_value);
}

int _pam_authenticate(void *handle, pam_handle_t *pamh, int flags)
{
    int (*f)(pam_handle_t *, int);

    f = dlsym(handle, "pam_authenticate");
    if (f == NULL) {
        return PAM_ABORT;
    }

    return (f)(pamh, flags);
}

int _pam_acct_mgmt(void *handle, pam_handle_t *pamh, int flags)
{
    int (*f)(pam_handle_t *, int );

    f = dlsym(handle, "pam_acct_mgmt");
    if (f == NULL) {
        return PAM_ABORT;
    }

    return (f)(pamh, flags);
}

int _pam_open_session(void *handle, pam_handle_t *pamh, int flags)
{
    int (*f)(pam_handle_t *, int);

    f = dlsym(handle, "pam_open_session");
    if (f == NULL) {
        return PAM_ABORT;
    }

    return (f)(pamh, flags);
}

int _pam_close_session(void *handle, pam_handle_t *pamh, int flags)
{
    int (*f)(pam_handle_t *, int);

    f = dlsym(handle, "pam_close_session");
    if (f == NULL) {
        return PAM_ABORT;
    }

    return (f)(pamh, flags);
}

char **_pam_getenvlist(void *handle, pam_handle_t *pamh)
{
    char **(*f)(pam_handle_t *);

    f = dlsym(handle, "pam_getenvlist");
    if (f == NULL) {
        return NULL;
    }

    return (f)(pamh);
}

const char *_pam_strerror(void *handle, pam_handle_t *pamh, int errnum)
{
    const char *(*f)(pam_handle_t *, int);

    f = dlsym(handle, "pam_strerror");
    if (f == NULL) {
        return "Unable to find symbol pam_strerror";
    }

    return (f)(pamh, errnum);
}

// _pam_envlist_len loops over the passed in list and returns its length.
int _pam_envlist_len(char **pam_envlist)
{
    int n = 0;
    char **pam_env;

    for (pam_env = pam_envlist; *pam_env != NULL; ++pam_env) {
        n = n + 1;
    }

    return n;
}

// _pam_getenv returns an PAM environment variable by index.
char * _pam_getenv(char **pam_envlist, int index)
{
    return pam_envlist[index];
}
