// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

#ifndef REGISTER_H_
#define REGISTER_H_

#include "credential_info.h"

// Register creates a new private key in the Secure Enclave.
// Creating new keys doesn't require user interaction, only attempting to use
// the key does.
// Returns zero if successful, non-zero otherwise.
int Register(CredentialInfo req, char **pubKeyB64Out, char **errOut);

#endif // REGISTER_H_
