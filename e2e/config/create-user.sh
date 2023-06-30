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

# check if the user is already created
if [ -f /var/lib/teleport/user-create ]; then
    echo "User already created"
    exit 0
fi

echo "Creating user"

sleep 5
tctl -d users add bob --roles=access,editor --logins=root
sqlite3 /var/lib/teleport/backend/sqlite.db "UPDATE kv SET value='\$2a\$10\$w0K2pwK/cF8BG0kKZ7X1qe1uU7w3mwZ2S46PO6SlaiVjiTkbNGQp6' where key = '/web/users/bob/pwd';"

touch /var/lib/teleport/user-create

exit 0