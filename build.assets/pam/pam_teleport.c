#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#ifdef __APPLE__
  #include <security/pam_appl.h>
  #include <security/pam_modules.h>
#else
  #include <security/pam_appl.h>
  #include <security/pam_modules.h>
  #include <security/pam_ext.h>
  #include <sys/types.h>
#endif

// For all PAM hooks, argument "0" triggers failure, other arguments mean
// success. Certain special per-callback argument values have special meaning.

int pam_sm_acct_mgmt(pam_handle_t *pamh, int flags, int argc, const char **argv)
{
    // If the "echo" command is requested that will echo out the value of
    // the Teleport specific environment variables.
    if (argc > 0 && strcmp(argv[0], "echo") == 0) {
        pam_info(pamh, "%s", getenv("TELEPORT_USERNAME"));
        pam_info(pamh, "%s", getenv("TELEPORT_LOGIN"));
        pam_info(pamh, "%s", getenv("TELEPORT_ROLES"));
    } else if (argc > 0 && argv[0][0] == '0') {
        return PAM_ACCT_EXPIRED;
    } else if (argc > 0 && strcmp(argv[0], "test_custom_env") == 0) {
        // If the "test_custom_env" command is requested it will verify custom env inputs.
        if (strcmp(getenv("FIRST_NAME"), "JOHN") == 0
            && strcmp(getenv("LAST_NAME"), "DOE") == 0
            && strcmp(getenv("OTHER"), "integration") == 0) {
            pam_info(pamh, "pam_custom_envs OK");
        }
    }

    pam_info(pamh, "pam_sm_acct_mgmt OK");
    return PAM_SUCCESS;
}

int pam_sm_open_session(pam_handle_t *pamh, int flags, int argc, const char **argv)
{

    // If the "set_env" command is requested, set the PAM environment variable.
    if (argc > 0 && strcmp(argv[0], "set_env") == 0) {
        int pam_err;
        pam_err = pam_putenv(pamh, argv[1]);
        if (pam_err < 0) {
            return PAM_SYSTEM_ERR;
        }
    } else if (argc > 0 && argv[0][0] == '0') {
        return PAM_SESSION_ERR;
    }

    pam_info(pamh, "pam_sm_open_session OK");
    return PAM_SUCCESS;
}

int pam_sm_close_session(pam_handle_t *pamh, int flags, int argc, const char **argv)
{
    if (argc > 0 && argv[0][0] == '0') {
        return PAM_SESSION_ERR;
    }

    pam_info(pamh, "pam_sm_close_session OK");
    return PAM_SUCCESS;
}

int pam_sm_authenticate(pam_handle_t *pamh, int flags, int argc, const char **argv)
{
    if (argc > 0 && argv[0][0] == '0') {
        return PAM_AUTH_ERR;
    }

    pam_info(pamh, "pam_sm_authenticate OK");
    return PAM_SUCCESS;
}
