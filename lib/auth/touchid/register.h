#ifndef REGISTER_H_
#define REGISTER_H_

#include "credential_info.h"

// Register creates a new private key in the Secure Enclave.
// Creating new keys doesn't require user interaction, only attempting to use
// the key does.
// Returns zero if successful, non-zero otherwise.
int Register(CredentialInfo req, char **pubKeyB64Out, char **errOut);

#endif // REGISTER_H_
