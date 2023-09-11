#!/bin/bash

#
# Copyright 2023 Gravitational, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

set -e

# Add generated certificates to system CA.
cp /etc/teleport/certs/rootCA.pem /usr/local/share/ca-certificates/teleport.crt
update-ca-certificates

yarn install

# Wait for the Teleport to be up and initialized.
sleep 5

npx playwright install chromium
npx playwright test --workers 1 --repeat-each 1 --timeout 15000 --project=chromium
