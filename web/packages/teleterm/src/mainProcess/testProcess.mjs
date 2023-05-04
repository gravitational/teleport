/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
