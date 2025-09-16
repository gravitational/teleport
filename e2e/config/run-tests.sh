#!/bin/bash

#
# Teleport
# Copyright (C) 2023  Gravitational, Inc.
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU Affero General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU Affero General Public License for more details.
#
# You should have received a copy of the GNU Affero General Public License
# along with this program.  If not, see <http://www.gnu.org/licenses/>.

set -e

# Add generated certificates to system CA.
cp /etc/teleport/certs/rootCA.pem /usr/local/share/ca-certificates/teleport.crt
update-ca-certificates

yarn install

# Wait for the Teleport to be up and initialized.
sleep 5

npx playwright install chromium
npx playwright test --workers 1 --repeat-each 1 --timeout 15000 --project=chromium
