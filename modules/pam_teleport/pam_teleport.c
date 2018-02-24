#include <stdio.h>
#include <stdlib.h>

#ifdef __APPLE__
  #include <security/pam_appl.h>
  #include <security/pam_modules.h>
#else
  #include <security/pam_appl.h>
  #include <security/pam_modules.h>
  #include <security/pam_ext.h>
#endif

int pam_sm_acct_mgmt(pam_handle_t *pamh, int flags, int argc, const char **argv)
{
    if (argc > 0 && argv[0][0] == '0') {
        return PAM_ACCT_EXPIRED;
    }

    pam_info(pamh, "Account opened successfully.");
    return PAM_SUCCESS;
}

int pam_sm_open_session(pam_handle_t *pamh, int flags, int argc, const char **argv)
{
    if (argc > 0 && argv[0][0] == '0') {
        return PAM_SESSION_ERR;
    }

    pam_info(pamh, "Session open successfully.");
    return PAM_SUCCESS;
}

int pam_sm_close_session (pam_handle_t *pamh, int flags, int argc, const char **argv)
{
    if (argc > 0 && argv[0][0] == '0') {
        return PAM_SESSION_ERR;
    }

    pam_info(pamh, "Session closed successfully.");
    return PAM_SUCCESS;
}
