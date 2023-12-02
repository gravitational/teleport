/**
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

import process from 'process';

const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));

const waitTime = parseInt(process.argv[2]);
if (waitTime) {
  await sleep(waitTime);
}

const shouldExit = process.argv[3];
if (shouldExit) {
  process.exit(1);
}

console.log('Lorem ipsum dolor sit amet');
console.log('{CONNECT_GRPC_PORT: 1337}');
console.log('Lorem ipsum dolor sit amet');
